use serde::Serialize;
use std::env;
use std::io::Read;
use std::path::Path;
use std::process::{Child, Command, Stdio};
use std::thread;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};
use time::OffsetDateTime;
use time::format_description::well_known::Rfc3339;

const SPIKE_VERSION: &str = "0.1.0-spike";
const MAX_TIMEOUT_MS: u64 = 3_600_000;
const MAX_CAPTURED_OUTPUT: usize = 2_048;
const MAX_MESSAGE_CHARACTERS: usize = 2_048;
const SUPPORTED_GODOT_VERSION_PREFIX: &str = "4.7.2.stable";

#[derive(Serialize)]
struct CommandInfo {
    name: &'static str,
    arguments: serde_json::Value,
}

#[derive(Serialize)]
struct StructuredError {
    code: &'static str,
    category: &'static str,
    message: String,
    retryable: bool,
}

#[derive(Serialize)]
struct CommandResult {
    schema_version: &'static str,
    run_id: String,
    command: CommandInfo,
    outcome: &'static str,
    started_at: String,
    finished_at: String,
    duration_ms: u128,
    exit_code: i32,
    summary: &'static str,
    errors: Vec<StructuredError>,
    evidence: Vec<serde_json::Value>,
    data: serde_json::Value,
}

fn main() {
    std::process::exit(run(env::args().skip(1).collect()));
}

fn run(args: Vec<String>) -> i32 {
    if args.as_slice() == ["--version"] {
        println!("gameatelier-runtime-spike {SPIKE_VERSION} (rust)");
        return 0;
    }
    if args.first().map(String::as_str) != Some("doctor") {
        return emit_usage("expected --version or doctor");
    }

    let parsed = match parse_doctor_args(&args[1..]) {
        Ok(value) => value,
        Err(message) => return emit_usage(message),
    };
    let started_wall = OffsetDateTime::now_utc();
    let started = Instant::now();

    if !Path::new(&parsed.project).join("project.godot").is_file() {
        return emit_result(
            &parsed,
            started_wall,
            started,
            4,
            "BLOCKED",
            "Godot project was not found.",
            vec![structured_error(
                "GODOT_PROJECT_NOT_FOUND",
                "prerequisite",
                "project.godot was not found in the requested directory.",
                false,
            )],
            None,
        );
    }
    if !Path::new(&parsed.godot).is_file() {
        return emit_result(
            &parsed,
            started_wall,
            started,
            4,
            "BLOCKED",
            "Godot executable was not found.",
            vec![structured_error(
                "GODOT_NOT_FOUND",
                "prerequisite",
                "The configured Godot executable does not exist.",
                false,
            )],
            None,
        );
    }

    match run_with_timeout(
        &parsed.godot,
        &parsed.project,
        Duration::from_millis(parsed.timeout_ms),
    ) {
        ProcessOutcome::Success(version) => emit_result(
            &parsed,
            started_wall,
            started,
            0,
            "PASS",
            "Godot executable and project were detected.",
            vec![],
            Some(version.trim().to_owned()),
        ),
        ProcessOutcome::Timeout => emit_result(
            &parsed,
            started_wall,
            started,
            6,
            "FAIL",
            "Godot version check timed out.",
            vec![structured_error(
                "GODOT_TIMEOUT",
                "timeout",
                "Godot did not exit before the configured timeout.",
                true,
            )],
            None,
        ),
        ProcessOutcome::Failed(message) => emit_result(
            &parsed,
            started_wall,
            started,
            5,
            "FAIL",
            "Godot version check failed.",
            vec![structured_error(
                "GODOT_PROCESS_FAILED",
                "engine",
                &message,
                true,
            )],
            None,
        ),
        ProcessOutcome::UnsupportedVersion(message) => emit_result(
            &parsed,
            started_wall,
            started,
            4,
            "BLOCKED",
            "A supported Godot version was not detected.",
            vec![structured_error(
                "GODOT_VERSION_UNSUPPORTED",
                "prerequisite",
                &message,
                false,
            )],
            None,
        ),
    }
}

struct DoctorArgs {
    godot: String,
    project: String,
    timeout_ms: u64,
}

fn parse_doctor_args(args: &[String]) -> Result<DoctorArgs, &'static str> {
    let mut godot = None;
    let mut project = None;
    let mut timeout_ms = 5_000_u64;
    let mut index = 0;
    while index < args.len() {
        let value = args.get(index + 1).ok_or("doctor options require values")?;
        match args[index].as_str() {
            "--godot" => godot = Some(value.clone()),
            "--project" => project = Some(value.clone()),
            "--timeout-ms" => {
                timeout_ms = value
                    .parse()
                    .map_err(|_| "--timeout-ms must be a positive integer")?
            }
            _ => return Err("unknown doctor option"),
        }
        index += 2;
    }
    if timeout_ms == 0 || timeout_ms > MAX_TIMEOUT_MS {
        return Err("--timeout-ms must be from 1 to 3600000");
    }
    Ok(DoctorArgs {
        godot: godot.ok_or("doctor requires --godot")?,
        project: project.ok_or("doctor requires --project")?,
        timeout_ms,
    })
}

enum ProcessOutcome {
    Success(String),
    Failed(String),
    UnsupportedVersion(String),
    Timeout,
}

fn run_with_timeout(godot: &str, project: &str, timeout: Duration) -> ProcessOutcome {
    let mut command = Command::new(godot);
    command
        .arg("--version")
        .current_dir(project)
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());
    #[cfg(unix)]
    {
        use std::os::unix::process::CommandExt;
        command.process_group(0);
    }

    let mut child = match command.spawn() {
        Ok(child) => child,
        Err(error) => return ProcessOutcome::Failed(error.to_string()),
    };
    let stdout = child.stdout.take().map(read_in_thread);
    let stderr = child.stderr.take().map(read_in_thread);
    let deadline = Instant::now() + timeout;
    let status = loop {
        match child.try_wait() {
            Ok(Some(status)) => {
                #[cfg(unix)]
                let _ = terminate_process_group(child.id());
                if Instant::now() >= deadline {
                    break None;
                }
                break Some(status);
            }
            Ok(None) if Instant::now() < deadline => thread::sleep(Duration::from_millis(10)),
            Ok(None) => {
                terminate_process_group_and_wait(&mut child);
                break None;
            }
            Err(error) => {
                terminate_process_group_and_wait(&mut child);
                return ProcessOutcome::Failed(error.to_string());
            }
        }
    };
    if status.is_none() {
        return ProcessOutcome::Timeout;
    }
    while !outputs_finished(&stdout, &stderr) && Instant::now() < deadline {
        thread::sleep(Duration::from_millis(1));
    }
    if !outputs_finished(&stdout, &stderr) {
        #[cfg(unix)]
        let _ = terminate_process_group(child.id());
        return ProcessOutcome::Timeout;
    }
    if Instant::now() >= deadline {
        return ProcessOutcome::Timeout;
    }
    let stdout = join_output(stdout);
    let stderr = join_output(stderr);
    match status {
        None => ProcessOutcome::Timeout,
        Some(status) if status.success() => {
            let version = stdout.trim();
            if version.starts_with(SUPPORTED_GODOT_VERSION_PREFIX) {
                ProcessOutcome::Success(version.to_owned())
            } else {
                ProcessOutcome::UnsupportedVersion(bounded_message(
                    version,
                    "Godot returned an empty or unsupported version.",
                ))
            }
        }
        Some(status) => ProcessOutcome::Failed(format!("exit={status}; stderr={}", stderr.trim())),
    }
}

fn terminate_process_group_and_wait(child: &mut Child) {
    #[cfg(unix)]
    {
        let group_killed = terminate_process_group(child.id());
        if !group_killed {
            let _ = child.kill();
        }
    }
    #[cfg(not(unix))]
    {
        let _ = child.kill();
    }
    let _ = child.wait();
}

#[cfg(unix)]
fn terminate_process_group(process_id: u32) -> bool {
    const SIGKILL: i32 = 9;
    unsafe { killpg(process_id as i32, SIGKILL) == 0 }
}

#[cfg(unix)]
unsafe extern "C" {
    fn killpg(pgrp: i32, sig: i32) -> i32;
}

fn read_in_thread<R: Read + Send + 'static>(mut reader: R) -> thread::JoinHandle<Vec<u8>> {
    thread::spawn(move || {
        let mut bytes = Vec::with_capacity(MAX_CAPTURED_OUTPUT);
        let mut buffer = [0_u8; 4_096];
        while let Ok(count) = reader.read(&mut buffer) {
            if count == 0 {
                break;
            }
            let remaining = MAX_CAPTURED_OUTPUT.saturating_sub(bytes.len());
            bytes.extend_from_slice(&buffer[..count.min(remaining)]);
        }
        bytes
    })
}

fn outputs_finished(
    stdout: &Option<thread::JoinHandle<Vec<u8>>>,
    stderr: &Option<thread::JoinHandle<Vec<u8>>>,
) -> bool {
    stdout.as_ref().is_none_or(thread::JoinHandle::is_finished)
        && stderr.as_ref().is_none_or(thread::JoinHandle::is_finished)
}

fn join_output(handle: Option<thread::JoinHandle<Vec<u8>>>) -> String {
    let bytes = handle
        .and_then(|value| value.join().ok())
        .unwrap_or_default();
    String::from_utf8_lossy(&bytes).into_owned()
}

fn emit_usage(message: &'static str) -> i32 {
    let parsed = DoctorArgs {
        godot: String::new(),
        project: String::new(),
        timeout_ms: 0,
    };
    emit_result(
        &parsed,
        OffsetDateTime::now_utc(),
        Instant::now(),
        2,
        "FAIL",
        message,
        vec![structured_error(
            "INVALID_ARGUMENT",
            "usage",
            message,
            false,
        )],
        None,
    )
}

#[allow(clippy::too_many_arguments)]
fn emit_result(
    parsed: &DoctorArgs,
    started_wall: OffsetDateTime,
    started: Instant,
    exit_code: i32,
    outcome: &'static str,
    summary: &'static str,
    errors: Vec<StructuredError>,
    godot_version: Option<String>,
) -> i32 {
    let finished = OffsetDateTime::now_utc();
    let run_suffix = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_nanos();
    let result = CommandResult {
        schema_version: "1.0.0",
        run_id: format!("spike-rust-{run_suffix}"),
        command: CommandInfo {
            name: "doctor",
            arguments: serde_json::json!({"godot": parsed.godot, "project": parsed.project, "timeout_ms": parsed.timeout_ms}),
        },
        outcome,
        started_at: started_wall
            .format(&Rfc3339)
            .unwrap_or_else(|_| "1970-01-01T00:00:00Z".to_owned()),
        finished_at: finished
            .format(&Rfc3339)
            .unwrap_or_else(|_| "1970-01-01T00:00:00Z".to_owned()),
        duration_ms: started.elapsed().as_millis(),
        exit_code,
        summary,
        errors,
        evidence: vec![],
        data: serde_json::json!({"implementation": "rust", "spike_version": SPIKE_VERSION, "godot_version": godot_version}),
    };
    match serde_json::to_string(&result) {
        Ok(json) => println!("{json}"),
        Err(error) => {
            eprintln!("{error}");
            return 8;
        }
    }
    exit_code
}

fn structured_error(
    code: &'static str,
    category: &'static str,
    message: &str,
    retryable: bool,
) -> StructuredError {
    StructuredError {
        code,
        category,
        message: bounded_message(message, "An unspecified error occurred."),
        retryable,
    }
}

fn bounded_message(message: &str, fallback: &str) -> String {
    let message = message.trim();
    let message = if message.is_empty() {
        fallback
    } else {
        message
    };
    message.chars().take(MAX_MESSAGE_CHARACTERS).collect()
}

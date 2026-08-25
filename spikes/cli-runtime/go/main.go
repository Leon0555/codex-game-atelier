package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const spikeVersion = "0.1.0-spike"
const maxTimeoutMS int64 = 3_600_000
const maxCapturedOutput = 2_048
const maxMessageCharacters = 2_048
const supportedGodotVersionPrefix = "4.7.2.stable"

type command struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type structuredError struct {
	Code        string `json:"code"`
	Category    string `json:"category"`
	Message     string `json:"message"`
	Retryable   bool   `json:"retryable"`
	Remediation string `json:"remediation,omitempty"`
}

type commandResult struct {
	SchemaVersion string            `json:"schema_version"`
	RunID         string            `json:"run_id"`
	Command       command           `json:"command"`
	Outcome       string            `json:"outcome"`
	StartedAt     string            `json:"started_at"`
	FinishedAt    string            `json:"finished_at"`
	DurationMS    int64             `json:"duration_ms"`
	ExitCode      int               `json:"exit_code"`
	Summary       string            `json:"summary"`
	Errors        []structuredError `json:"errors"`
	Evidence      []map[string]any  `json:"evidence"`
	Data          map[string]any    `json:"data"`
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 1 && args[0] == "--version" {
		fmt.Printf("gamefoundry-runtime-spike %s (go)\n", spikeVersion)
		return 0
	}
	if len(args) == 0 || args[0] != "doctor" {
		return emitUsage("expected --version or doctor")
	}

	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	godot := flags.String("godot", "", "Godot executable")
	project := flags.String("project", "", "Godot project directory")
	timeoutMS := flags.Int64("timeout-ms", 5000, "timeout in milliseconds")
	if err := flags.Parse(args[1:]); err != nil || *godot == "" || *project == "" || *timeoutMS < 1 || *timeoutMS > maxTimeoutMS {
		return emitUsage("doctor requires --godot, --project, and --timeout-ms from 1 to 3600000")
	}

	started := time.Now().UTC()
	result := newResult(started, *godot, *project, *timeoutMS)
	if info, err := os.Stat(filepath.Join(*project, "project.godot")); err != nil || info.IsDir() {
		return finish(&result, started, 4, "BLOCKED", "Godot project was not found.", structuredError{
			Code: "GODOT_PROJECT_NOT_FOUND", Category: "prerequisite", Message: "project.godot was not found in the requested directory.", Retryable: false,
		})
	}
	if info, err := os.Stat(*godot); err != nil || info.IsDir() {
		return finish(&result, started, 4, "BLOCKED", "Godot executable was not found.", structuredError{
			Code: "GODOT_NOT_FOUND", Category: "prerequisite", Message: "The configured Godot executable does not exist.", Retryable: false,
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutMS)*time.Millisecond)
	defer cancel()
	cmd := exec.Command(*godot, "--version")
	cmd.Dir = *project
	configureProcessGroup(cmd)
	output, err, timedOut := runCommand(ctx, cmd)
	if timedOut {
		return finish(&result, started, 6, "FAIL", "Godot version check timed out.", structuredError{
			Code: "GODOT_TIMEOUT", Category: "timeout", Message: "Godot did not exit before the configured timeout.", Retryable: true,
		})
	}
	if err != nil {
		return finish(&result, started, 5, "FAIL", "Godot version check failed.", structuredError{
			Code: "GODOT_PROCESS_FAILED", Category: "engine", Message: boundedMessage(string(output), fmt.Sprintf("Godot process failed: %v", err)), Retryable: true,
		})
	}

	version := strings.TrimSpace(string(output))
	if !strings.HasPrefix(version, supportedGodotVersionPrefix) {
		return finish(&result, started, 4, "BLOCKED", "A supported Godot version was not detected.", structuredError{
			Code: "GODOT_VERSION_UNSUPPORTED", Category: "prerequisite", Message: boundedMessage(version, "Godot returned an empty or unsupported version."), Retryable: false,
		})
	}
	result.Data["godot_version"] = version
	return finish(&result, started, 0, "PASS", "Godot executable and project were detected.")
}

func runCommand(ctx context.Context, cmd *exec.Cmd) ([]byte, error, bool) {
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return nil, err, false
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		return nil, err, false
	}
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter
	if err := cmd.Start(); err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		_ = stderrReader.Close()
		_ = stderrWriter.Close()
		return nil, err, false
	}
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	stdout := readPipe(stdoutReader)
	stderr := readPipe(stderrReader)
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()

	var waitErr error
	timedOut := false
	select {
	case waitErr = <-waited:
		terminateProcessGroup(cmd)
	case <-ctx.Done():
		timedOut = true
		terminateProcessGroup(cmd)
		waitErr = <-waited
	}
	stdoutData, stderrData := drainPipes(ctx, stdoutReader, stderrReader, stdout, stderr, cmd, &timedOut)
	if ctx.Err() != nil {
		timedOut = true
	}
	output := append(stdoutData, stderrData...)
	return output, waitErr, timedOut
}

func readPipe(reader *os.File) <-chan []byte {
	output := make(chan []byte, 1)
	go func() {
		defer reader.Close()
		data := make([]byte, 0, maxCapturedOutput)
		buffer := make([]byte, 4_096)
		for {
			count, err := reader.Read(buffer)
			remaining := maxCapturedOutput - len(data)
			if remaining > 0 && count > 0 {
				if count < remaining {
					remaining = count
				}
				data = append(data, buffer[:remaining]...)
			}
			if err != nil {
				break
			}
		}
		output <- data
	}()
	return output
}

func drainPipes(ctx context.Context, stdoutReader, stderrReader *os.File, stdout, stderr <-chan []byte, cmd *exec.Cmd, timedOut *bool) ([]byte, []byte) {
	var stdoutData, stderrData []byte
	deadline := ctx.Done()
	for stdout != nil || stderr != nil {
		select {
		case stdoutData = <-stdout:
			stdout = nil
		case stderrData = <-stderr:
			stderr = nil
		case <-deadline:
			*timedOut = true
			terminateProcessGroup(cmd)
			_ = stdoutReader.Close()
			_ = stderrReader.Close()
			deadline = nil
		}
	}
	return stdoutData, stderrData
}

func boundedMessage(message, fallback string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		message = fallback
	}
	runes := []rune(message)
	if len(runes) > maxMessageCharacters {
		message = string(runes[:maxMessageCharacters])
	}
	return message
}

func newResult(started time.Time, godot, project string, timeoutMS int64) commandResult {
	return commandResult{
		SchemaVersion: "1.0.0",
		RunID:         fmt.Sprintf("spike-go-%d", started.UnixNano()),
		Command: command{Name: "doctor", Arguments: map[string]any{
			"godot": godot, "project": project, "timeout_ms": timeoutMS,
		}},
		StartedAt: started.Format(time.RFC3339Nano),
		Errors:    []structuredError{},
		Evidence:  []map[string]any{},
		Data:      map[string]any{"implementation": "go", "spike_version": spikeVersion},
	}
}

func finish(result *commandResult, started time.Time, code int, outcome, summary string, failures ...structuredError) int {
	finished := time.Now().UTC()
	result.Outcome = outcome
	result.FinishedAt = finished.Format(time.RFC3339Nano)
	result.DurationMS = finished.Sub(started).Milliseconds()
	result.ExitCode = code
	result.Summary = summary
	result.Errors = append(result.Errors, failures...)
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 8
	}
	return code
}

func emitUsage(message string) int {
	started := time.Now().UTC()
	result := newResult(started, "", "", 0)
	return finish(&result, started, 2, "FAIL", message, structuredError{
		Code: "INVALID_ARGUMENT", Category: "usage", Message: message, Retryable: false,
	})
}

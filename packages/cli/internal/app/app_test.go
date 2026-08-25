package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

func TestDetectDoesNotStartGodot(t *testing.T) {
	requireUnixShell(t)
	project := createProject(t, "测试 项目 🚀")
	marker := filepath.Join(t.TempDir(), "started")
	godot := createExecutable(t, "fake-godot", "#!/bin/sh\nprintf started > '"+marker+"'\nprintf '4.7.2.stable.official.ed1daf0bf\\n'\n")

	code, result, stdout, stderr := execute(t, context.Background(), "detect", "--project", project, "--godot", godot)
	if code != contract.ExitOK || result.Outcome != "PASS" {
		t.Fatalf("detect failed: code=%d result=%+v stdout=%s stderr=%s", code, result, stdout, stderr)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("detect started Godot; marker stat error=%v", err)
	}
	assertResultInvariant(t, result)
}

func TestDoctorAcceptsSupportedOfficialGodot(t *testing.T) {
	requireUnixShell(t)
	project := createProject(t, "中文 路径")
	godot := createExecutable(t, "godot", "#!/bin/sh\nprintf '4.7.2.stable.official.ed1daf0bf\\n'\n")

	code, result, stdout, stderr := execute(t, context.Background(), "doctor", "--project", project, "--godot", godot)
	if code != contract.ExitOK || result.Outcome != "PASS" {
		t.Fatalf("doctor failed: code=%d result=%+v stdout=%s stderr=%s", code, result, stdout, stderr)
	}
	assertResultInvariant(t, result)
}

func TestDoctorRejectsUnsupportedAndNonGodotExecutables(t *testing.T) {
	requireUnixShell(t)
	project := createProject(t, "project")
	tests := []struct {
		name   string
		output string
	}{
		{name: "wrong patch", output: "4.7.20.stable.official.deadbeef"},
		{name: "dotnet", output: "4.7.2.stable.mono.official.deadbeef"},
		{name: "not godot", output: "true"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			godot := createExecutable(t, "candidate", "#!/bin/sh\nprintf '"+test.output+"\\n'\n")
			code, result, _, _ := execute(t, context.Background(), "doctor", "--project", project, "--godot", godot)
			if code != contract.ExitPrerequisite || result.Outcome != "BLOCKED" || firstErrorCode(result) != "GODOT_VERSION_UNSUPPORTED" {
				t.Fatalf("unexpected result: code=%d result=%+v", code, result)
			}
			assertResultInvariant(t, result)
		})
	}
}

func TestDoctorDoesNotEchoUnsupportedProcessOutput(t *testing.T) {
	requireUnixShell(t)
	project := createProject(t, "project")
	secret := "tokenThatMustNotAppear"
	godot := createExecutable(t, "candidate", "#!/bin/sh\nprintf '"+secret+"\\n'\n")

	code, result, stdout, stderr := execute(t, context.Background(), "doctor", "--project", project, "--godot", godot)
	if code != contract.ExitPrerequisite || firstErrorCode(result) != "GODOT_VERSION_UNSUPPORTED" {
		t.Fatalf("unexpected process result: code=%d result=%+v", code, result)
	}
	if strings.Contains(stdout+stderr, secret) {
		t.Fatal("unsupported process output leaked into the command result")
	}
}

func TestDoctorBlocksUnreadableOrOversizedProjectBeforeStartingGodot(t *testing.T) {
	requireUnixShell(t)
	project := createProject(t, "oversized-project")
	if err := os.WriteFile(filepath.Join(project, "project.godot"), bytes.Repeat([]byte("x"), 1024*1024+1), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "started")
	godot := createExecutable(t, "godot", "#!/bin/sh\nprintf started > '"+marker+"'\nprintf '4.7.2.stable.official.ed1daf0bf\\n'\n")

	code, result, _, _ := execute(t, context.Background(), "doctor", "--project", project, "--godot", godot)
	if code != contract.ExitPrerequisite || firstErrorCode(result) != "GODOT_PROJECT_UNREADABLE" {
		t.Fatalf("unexpected oversized project result: code=%d result=%+v", code, result)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("doctor started Godot after project language check failed; marker stat=%v", err)
	}
	assertDoctorCheckIDs(t, result, "host", "project_file", "project_language", "godot_executable", "godot_version")
}

func TestDoctorAlwaysReportsAllFiveChecks(t *testing.T) {
	requireUnixShell(t)
	project := filepath.Join(t.TempDir(), "missing-project-file")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	godot := createExecutable(t, "godot", "#!/bin/sh\nprintf '4.7.2.stable.official.ed1daf0bf\\n'\n")

	code, result, _, _ := execute(t, context.Background(), "doctor", "--project", project, "--godot", godot)
	if code != contract.ExitPrerequisite {
		t.Fatalf("unexpected missing project result: code=%d result=%+v", code, result)
	}
	assertDoctorCheckIDs(t, result, "host", "project_file", "project_language", "godot_executable", "godot_version")
}

func TestProjectDotNetDetectionParsesConfigurationInsteadOfComments(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{name: "comment", content: "; This GDScript project does not use [dotnet]\nconfig_version=5\n", want: false},
		{name: "ordinary value", content: "config_version=5\n[application]\nconfig/name=\"mentions dotnet/project only\"\n", want: false},
		{name: "dotnet section", content: "config_version=5\n[dotnet]\nproject/assembly_name=\"Game\"\n", want: true},
		{name: "dotnet key", content: "config_version=5\ndotnet/project/assembly_name=\"Game\"\n", want: true},
		{name: "csharp feature", content: "config_version=5\nconfig/features=PackedStringArray(\"4.7\", \"C#\")\n", want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := createProject(t, test.name)
			if err := os.WriteFile(filepath.Join(project, "project.godot"), []byte(test.content), 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := projectUsesDotNet(project)
			if err != nil || got != test.want {
				t.Fatalf("projectUsesDotNet()=(%t,%v), want (%t,nil)", got, err, test.want)
			}
		})
	}
}

func TestDoctorCancelledBeforeStartStillReportsAllChecks(t *testing.T) {
	requireUnixShell(t)
	project := createProject(t, "cancelled")
	godot := createExecutable(t, "godot", "#!/bin/sh\nprintf '4.7.2.stable.official.ed1daf0bf\\n'\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	code, result, _, _ := execute(t, ctx, "doctor", "--project", project, "--godot", godot)
	if code != contract.ExitInterrupted || firstErrorCode(result) != "COMMAND_CANCELLED" {
		t.Fatalf("unexpected cancelled result: code=%d result=%+v", code, result)
	}
	assertDoctorCheckIDs(t, result, "host", "project_file", "project_language", "godot_executable", "godot_version")
}

func TestDoctorTimesOutAndBoundsOutput(t *testing.T) {
	requireUnixShell(t)
	project := createProject(t, "project")
	godot := createExecutable(t, "slow-godot", "#!/bin/sh\ntrap '' TERM\nsleep 5\n")

	started := time.Now()
	code, result, _, _ := execute(t, context.Background(), "doctor", "--project", project, "--godot", godot, "--timeout-ms", "50")
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("timeout took too long: %s", elapsed)
	}
	if code != contract.ExitInterrupted || result.Outcome != "FAIL" || firstErrorCode(result) != "GODOT_TIMEOUT" {
		t.Fatalf("unexpected timeout result: code=%d result=%+v", code, result)
	}
	assertResultInvariant(t, result)
}

func TestDoctorBoundsProcessOutputDeterministically(t *testing.T) {
	requireUnixShell(t)
	project := createProject(t, "project")
	largeOutput := strings.Repeat("x", 10000)
	godot := createExecutable(t, "noisy-godot", "#!/bin/sh\nprintf '"+largeOutput+"'\n")

	code, result, stdout, _ := execute(t, context.Background(), "doctor", "--project", project, "--godot", godot)
	if code != contract.ExitEngine || firstErrorCode(result) != "GODOT_OUTPUT_TRUNCATED" {
		t.Fatalf("unexpected noisy process result: code=%d result=%+v", code, result)
	}
	if len(stdout) > 16*1024 || !doctorOutputWasTruncated(t, result) || strings.Contains(stdout, largeOutput[:256]) {
		t.Fatalf("process output was not bounded and redacted: json_bytes=%d result=%+v", len(stdout), result)
	}
}

func TestDoctorNeverTrustsTruncatedVersionOutput(t *testing.T) {
	requireUnixShell(t)
	project := createProject(t, "project")
	tests := map[string]string{
		"truncated candidate line": "4.7.2.stable.official." + strings.Repeat("a", 6000),
		"valid line plus overflow": "4.7.2.stable.official.ed1daf0bf\\n" + strings.Repeat("x", 10000),
	}
	for name, output := range tests {
		t.Run(name, func(t *testing.T) {
			godot := createExecutable(t, "noisy-godot", "#!/bin/sh\nprintf '"+output+"'")
			code, result, stdout, _ := execute(t, context.Background(), "doctor", "--project", project, "--godot", godot)
			if code != contract.ExitEngine || firstErrorCode(result) != "GODOT_OUTPUT_TRUNCATED" {
				t.Fatalf("truncated output was trusted: code=%d result=%+v", code, result)
			}
			godotData, ok := resultDataMap(t, result)["godot"].(map[string]any)
			if !ok {
				t.Fatalf("godot data type=%T", resultDataMap(t, result)["godot"])
			}
			if _, exists := godotData["version"]; exists {
				t.Fatalf("untrusted version was exposed: %+v", godotData)
			}
			if len(stdout) > 16*1024 || strings.Contains(stdout, strings.Repeat("a", 256)) || strings.Contains(stdout, strings.Repeat("x", 256)) {
				t.Fatalf("truncated output leaked: json_bytes=%d", len(stdout))
			}
		})
	}
}

func TestDoctorDoesNotHangWhenChildKeepsPipesOpen(t *testing.T) {
	requireUnixShell(t)
	project := createProject(t, "project")
	godot := createExecutable(t, "detached-child-godot", "#!/bin/sh\nprintf '4.7.2.stable.official.ed1daf0bf\\n'\n(sleep 5) &\nexit 0\n")

	started := time.Now()
	code, result, _, _ := execute(t, context.Background(), "doctor", "--project", project, "--godot", godot, "--timeout-ms", "1500")
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("child-held pipes were not bounded: %s", elapsed)
	}
	if code != contract.ExitEngine || result.Outcome != "FAIL" || firstErrorCode(result) != "GODOT_PROCESS_FAILED" {
		t.Fatalf("unexpected detached child result: code=%d result=%+v", code, result)
	}
}

func TestDoctorReportsEngineFailureWithoutLeakingProcessOutput(t *testing.T) {
	requireUnixShell(t)
	project := createProject(t, "project")
	secret := "token-super-secret-value"
	godot := createExecutable(t, "failing-godot", "#!/bin/sh\nprintf '"+secret+"' >&2\nexit 23\n")

	code, result, stdout, stderr := execute(t, context.Background(), "doctor", "--project", project, "--godot", godot)
	if code != contract.ExitEngine || firstErrorCode(result) != "GODOT_PROCESS_FAILED" {
		t.Fatalf("unexpected process result: code=%d result=%+v", code, result)
	}
	if strings.Contains(stdout+stderr, secret) {
		t.Fatal("process output leaked into the command result")
	}
}

func TestStatusReadsStrictProjectState(t *testing.T) {
	project := createProject(t, "状态 项目")
	writeState(t, project, `{
  "schema_version":"1.0.0",
  "project_id":"sample-project",
  "revision":2,
  "mode":"standard",
  "engine":{"kind":"godot","requested_version":"4.7.2-stable","language":"gdscript"},
  "task_refs":[".gameatelier/tasks/task-1.json"],
  "active_run_refs":[],
  "last_command_result_ref":".gameatelier/runs/run-1/result.json",
  "updated_at":"2026-08-25T00:00:00Z"
}`)

	code, result, _, _ := execute(t, context.Background(), "status", "--project", project)
	if code != contract.ExitOK || result.Outcome != "PASS" {
		t.Fatalf("status failed: code=%d result=%+v", code, result)
	}
	assertResultInvariant(t, result)
}

func TestStatusAcceptsJSONSchemaMathematicalIntegerForms(t *testing.T) {
	for _, revision := range []string{"1.0", "1e0", "9223372036854775807"} {
		t.Run(revision, func(t *testing.T) {
			project := createProject(t, "revision-"+revision)
			writeState(t, project, fmt.Sprintf(`{
  "schema_version":"1.0.0","project_id":"p","revision":%s,"mode":"standard",
  "engine":{"kind":"godot","requested_version":"4.7.2-stable","language":"gdscript"},
  "task_refs":[],"active_run_refs":[],"updated_at":"2026-08-25T00:00:00Z"
}`, revision))
			code, result, _, _ := execute(t, context.Background(), "status", "--project", project)
			if code != contract.ExitOK || result.Outcome != "PASS" {
				t.Fatalf("revision %s should be accepted: code=%d result=%+v", revision, code, result)
			}
		})
	}
}

func TestStatusReportsUnsupportedSchemaBeforeV1FieldValidation(t *testing.T) {
	project := createProject(t, "future-schema")
	writeState(t, project, `{"schema_version":"2.0.0","future_shape":{"changed":true}}`)

	code, result, _, _ := execute(t, context.Background(), "status", "--project", project)
	if code != contract.ExitState || firstErrorCode(result) != "STATE_SCHEMA_UNSUPPORTED" {
		t.Fatalf("unexpected future schema result: code=%d result=%+v", code, result)
	}
	data := resultDataMap(t, result)
	if data["observed_schema_version"] != "2.0.0" {
		t.Fatalf("observed schema version=%v, want 2.0.0", data["observed_schema_version"])
	}
	if _, exists := data["schema_version"]; exists {
		t.Fatalf("unsupported schema must not be reported as the supported schema_version: %+v", data)
	}
}

func TestStatusRejectsMissingDuplicateAndUnsafeState(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		project := createProject(t, "missing")
		code, result, _, _ := execute(t, context.Background(), "status", "--project", project)
		if code != contract.ExitPrerequisite || firstErrorCode(result) != "PROJECT_NOT_INITIALIZED" {
			t.Fatalf("unexpected missing state result: code=%d result=%+v", code, result)
		}
	})

	tests := []struct {
		name  string
		state string
	}{
		{name: "duplicate key", state: `{"schema_version":"1.0.0","schema_version":"1.0.0"}`},
		{name: "schema version too long", state: `{"schema_version":"` + strings.Repeat("v", 129) + `"}`},
		{name: "missing required", state: `{
  "schema_version":"1.0.0","project_id":"p","mode":"standard",
  "engine":{"kind":"godot","requested_version":"4.7.2-stable","language":"gdscript"},
  "task_refs":[],"active_run_refs":[],"updated_at":"2026-08-25T00:00:00Z"
}`},
		{name: "null required", state: `{
  "schema_version":"1.0.0","project_id":"p","revision":null,"mode":"standard",
  "engine":{"kind":"godot","requested_version":"4.7.2-stable","language":"gdscript"},
  "task_refs":[],"active_run_refs":[],"updated_at":"2026-08-25T00:00:00Z"
}`},
		{name: "fractional revision", state: `{
  "schema_version":"1.0.0","project_id":"p","revision":1.5,"mode":"standard",
  "engine":{"kind":"godot","requested_version":"4.7.2-stable","language":"gdscript"},
  "task_refs":[],"active_run_refs":[],"updated_at":"2026-08-25T00:00:00Z"
}`},
		{name: "revision above int64", state: `{
  "schema_version":"1.0.0","project_id":"p","revision":9223372036854775808,"mode":"standard",
  "engine":{"kind":"godot","requested_version":"4.7.2-stable","language":"gdscript"},
  "task_refs":[],"active_run_refs":[],"updated_at":"2026-08-25T00:00:00Z"
}`},
		{name: "duplicate refs", state: `{
  "schema_version":"1.0.0","project_id":"p","revision":0,"mode":"standard",
  "engine":{"kind":"godot","requested_version":"4.7.2-stable","language":"gdscript"},
  "task_refs":[".gameatelier/tasks/a.json",".gameatelier/tasks/a.json"],"active_run_refs":[],"updated_at":"2026-08-25T00:00:00Z"
}`},
		{name: "unsafe ref", state: `{
  "schema_version":"1.0.0","project_id":"p","revision":0,"mode":"standard",
  "engine":{"kind":"godot","requested_version":"4.7.2-stable","language":"gdscript"},
  "task_refs":[".gameatelier/../secret"],"active_run_refs":[],"updated_at":"2026-08-25T00:00:00Z"
}`},
		{name: "unknown field", state: `{
  "schema_version":"1.0.0","project_id":"p","revision":0,"mode":"standard",
  "engine":{"kind":"godot","requested_version":"4.7.2-stable","language":"gdscript"},
  "task_refs":[],"active_run_refs":[],"updated_at":"2026-08-25T00:00:00Z","surprise":true
}`},
		{name: "case alias", state: `{
  "schema_version":"1.0.0","project_id":"p","Project_ID":"q","revision":0,"mode":"standard",
  "engine":{"kind":"godot","requested_version":"4.7.2-stable","language":"gdscript"},
  "task_refs":[],"active_run_refs":[],"updated_at":"2026-08-25T00:00:00Z"
}`},
		{name: "unknown engine alias", state: `{
  "schema_version":"1.0.0","project_id":"p","revision":0,"mode":"standard",
  "engine":{"kind":"godot","Kind":"other","requested_version":"4.7.2-stable","language":"gdscript"},
  "task_refs":[],"active_run_refs":[],"updated_at":"2026-08-25T00:00:00Z"
}`},
		{name: "version too long", state: `{
  "schema_version":"1.0.0","project_id":"p","revision":0,"mode":"standard",
  "engine":{"kind":"godot","requested_version":"` + strings.Repeat("v", 129) + `","language":"gdscript"},
  "task_refs":[],"active_run_refs":[],"updated_at":"2026-08-25T00:00:00Z"
}`},
		{name: "empty optional ref", state: `{
  "schema_version":"1.0.0","project_id":"p","revision":0,"mode":"standard",
  "engine":{"kind":"godot","requested_version":"4.7.2-stable","language":"gdscript"},
  "task_refs":[],"active_run_refs":[],"last_command_result_ref":"","updated_at":"2026-08-25T00:00:00Z"
}`},
		{name: "reserved ref", state: `{
  "schema_version":"1.0.0","project_id":"p","revision":0,"mode":"standard",
  "engine":{"kind":"godot","requested_version":"4.7.2-stable","language":"gdscript"},
  "task_refs":[".gameatelier/tasks/con.json"],"active_run_refs":[],"updated_at":"2026-08-25T00:00:00Z"
}`},
		{name: "lowercase timestamp", state: `{
  "schema_version":"1.0.0","project_id":"p","revision":0,"mode":"standard",
  "engine":{"kind":"godot","requested_version":"4.7.2-stable","language":"gdscript"},
  "task_refs":[],"active_run_refs":[],"updated_at":"2026-08-25t00:00:00z"
}`},
		{name: "comma timestamp", state: `{
  "schema_version":"1.0.0","project_id":"p","revision":0,"mode":"standard",
  "engine":{"kind":"godot","requested_version":"4.7.2-stable","language":"gdscript"},
  "task_refs":[],"active_run_refs":[],"updated_at":"2026-08-25T00:00:00,5Z"
}`},
		{name: "invalid offset hour", state: `{
  "schema_version":"1.0.0","project_id":"p","revision":0,"mode":"standard",
  "engine":{"kind":"godot","requested_version":"4.7.2-stable","language":"gdscript"},
  "task_refs":[],"active_run_refs":[],"updated_at":"2026-08-25T00:00:00+24:00"
}`},
		{name: "invalid offset minute", state: `{
  "schema_version":"1.0.0","project_id":"p","revision":0,"mode":"standard",
  "engine":{"kind":"godot","requested_version":"4.7.2-stable","language":"gdscript"},
  "task_refs":[],"active_run_refs":[],"updated_at":"2026-08-25T00:00:00+00:60"
}`},
		{name: "leap second", state: `{
  "schema_version":"1.0.0","project_id":"p","revision":0,"mode":"standard",
  "engine":{"kind":"godot","requested_version":"4.7.2-stable","language":"gdscript"},
  "task_refs":[],"active_run_refs":[],"updated_at":"2026-08-25T00:00:60Z"
}`},
	}
	t.Run("invalid utf8", func(t *testing.T) {
		project := createProject(t, "invalid-utf8")
		directory := filepath.Join(project, ".gameatelier")
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		content := append([]byte(`{"schema_version":"1.0.0","project_id":"p`), 0xff)
		content = append(content, []byte(`"}`)...)
		if err := os.WriteFile(filepath.Join(directory, "project.json"), content, 0o644); err != nil {
			t.Fatal(err)
		}
		code, result, _, _ := execute(t, context.Background(), "status", "--project", project)
		if code != contract.ExitState || firstErrorCode(result) != "STATE_INVALID" {
			t.Fatalf("unexpected invalid UTF-8 result: code=%d result=%+v", code, result)
		}
	})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := createProject(t, test.name)
			writeState(t, project, test.state)
			code, result, _, _ := execute(t, context.Background(), "status", "--project", project)
			if code != contract.ExitState || result.Outcome != "FAIL" || firstErrorCode(result) != "STATE_INVALID" {
				t.Fatalf("unexpected invalid state result: code=%d result=%+v", code, result)
			}
		})
	}
}

func TestArgumentsRejectUnknownTrailingDuplicateAndInvalidTimeout(t *testing.T) {
	tests := [][]string{
		{"detect", "--unknown"},
		{"detect", "--"},
		{"detect", "-project", "."},
		{"detect", "trailing"},
		{"detect", "--project", ".", "--project", "."},
		{"doctor", "--timeout-ms", "0"},
		{"doctor", "--timeout-ms", "3600001"},
		{"initialize", "--unknown"},
		{"status", "--project", ""},
	}
	for _, args := range tests {
		code, result, _, _ := execute(t, context.Background(), args...)
		if code != contract.ExitUsage || result.Outcome != "FAIL" || firstErrorCode(result) != "INVALID_ARGUMENT" {
			t.Fatalf("args %v: code=%d result=%+v", args, code, result)
		}
		assertResultInvariant(t, result)
	}
}

func TestCommandUsageErrorsUseSchemaNeutralData(t *testing.T) {
	for _, command := range []string{"detect", "doctor", "initialize", "status"} {
		code, result, _, _ := execute(t, context.Background(), command, "--unknown")
		if code != contract.ExitUsage {
			t.Fatalf("%s usage exit=%d, want %d", command, code, contract.ExitUsage)
		}
		if data := resultDataMap(t, result); len(data) != 0 {
			t.Fatalf("%s usage data=%v, want empty object", command, data)
		}
	}
}

func TestDuplicateLongFlagDoesNotLeakOrBreakResultLimits(t *testing.T) {
	flagName := strings.Repeat("x", 3000)
	code, result, stdout, _ := execute(t, context.Background(), "detect", "--"+flagName, "--"+flagName)
	if code != contract.ExitUsage || firstErrorCode(result) != "INVALID_ARGUMENT" {
		t.Fatalf("unexpected long duplicate result: code=%d result=%+v", code, result)
	}
	if strings.Contains(stdout, flagName) {
		t.Fatal("long user-controlled flag name leaked into stdout")
	}
	if len(result.Summary) > 2048 || len(result.Errors[0].Message) > 2048 {
		t.Fatalf("usage diagnostics exceed schema limits: summary=%d error=%d", len(result.Summary), len(result.Errors[0].Message))
	}
}

func TestSafeStateReferenceUsesPortableProjectRelativePaths(t *testing.T) {
	tests := map[string]bool{
		".gameatelier/tasks/task-1.json":   true,
		".gameatelier/runs/r/result.json":  true,
		"/absolute":                        false,
		"C:\\state.json":                   false,
		"\\\\server\\share\\state.json":    false,
		".gameatelier/../secret":           false,
		".gameatelier\\..\\secret":         false,
		".gameatelier\\tasks\\task.json":   false,
		".gameatelier/state.json:stream":   false,
		".gameatelier/tasks/a\n/../../x":   false,
		".gameatelier/tasks/a\x00.json":    false,
		".gameatelier/tasks/con.json":      false,
		".gameatelier/tasks/trailing.":     false,
		".gameatelier/tasks/foo./bar.json": false,
		".gameatelier/tasks/Upper.json":    false,
		"other/state.json":                 false,
	}
	for input, expected := range tests {
		if actual := safeStateReference(input); actual != expected {
			t.Errorf("safeStateReference(%q)=%t, want %t", input, actual, expected)
		}
	}
}

func TestVersion(t *testing.T) {
	var stdout bytes.Buffer
	code := Run(context.Background(), []string{"--version"}, &stdout, ioDiscard{})
	if code != contract.ExitOK || stdout.String() != "codex-game-atelier "+Version+"\n" {
		t.Fatalf("unexpected version output: code=%d output=%q", code, stdout.String())
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(input []byte) (int, error) { return len(input), nil }

func execute(t *testing.T, ctx context.Context, args ...string) (int, contract.Result, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(ctx, args, &stdout, &stderr)
	var result contract.Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not one result JSON: %v; stdout=%q stderr=%q", err, stdout.String(), stderr.String())
	}
	return code, result, stdout.String(), stderr.String()
}

func createProject(t *testing.T, name string) string {
	t.Helper()
	project := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("config_version=5\n[application]\nconfig/name=\"Test\"\n")
	if err := os.WriteFile(filepath.Join(project, "project.godot"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	return project
}

func createExecutable(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeState(t *testing.T, project, content string) {
	t.Helper()
	directory := filepath.Join(project, ".gameatelier")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "project.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func requireUnixShell(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is covered on Unix hosts; Windows requires native fixture validation")
	}
}

func firstErrorCode(result contract.Result) string {
	if len(result.Errors) == 0 {
		return ""
	}
	return result.Errors[0].Code
}

func resultDataMap(t *testing.T, result contract.Result) map[string]any {
	t.Helper()
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("result data type=%T, want JSON object", result.Data)
	}
	return data
}

func doctorOutputWasTruncated(t *testing.T, result contract.Result) bool {
	t.Helper()
	encoded, err := json.Marshal(result.Data)
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Godot struct {
			OutputTruncated bool `json:"output_truncated"`
		} `json:"godot"`
	}
	if err := json.Unmarshal(encoded, &data); err != nil {
		t.Fatal(err)
	}
	return data.Godot.OutputTruncated
}

func assertDoctorCheckIDs(t *testing.T, result contract.Result, expected ...string) {
	t.Helper()
	encoded, err := json.Marshal(result.Data)
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Checks []struct {
			ID string `json:"id"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(encoded, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Checks) != len(expected) {
		t.Fatalf("check count=%d, want %d: %+v", len(data.Checks), len(expected), data.Checks)
	}
	for index, id := range expected {
		if data.Checks[index].ID != id {
			t.Fatalf("check %d id=%q, want %q", index, data.Checks[index].ID, id)
		}
	}
}

func assertResultInvariant(t *testing.T, result contract.Result) {
	t.Helper()
	if result.SchemaVersion != contract.SchemaVersion || result.RunID == "" || result.Command.Name == "" || result.Summary == "" || result.StartedAt == "" || result.FinishedAt == "" {
		t.Fatalf("incomplete result envelope: %+v", result)
	}
	if result.Outcome == "PASS" && (result.ExitCode != 0 || len(result.Errors) != 0) {
		t.Fatalf("invalid PASS result: %+v", result)
	}
	if (result.Outcome == "FAIL" || result.Outcome == "BLOCKED") && (result.ExitCode == 0 || len(result.Errors) == 0) {
		t.Fatalf("invalid failure result: %+v", result)
	}
}

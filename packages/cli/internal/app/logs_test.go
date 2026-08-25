package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

func TestLogsProjectsVerifiedRunWithoutFreeTextOrWrites(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, state := createRunStoreProject(t, "logs 安全投影")
	privateText := "Q7zN4pL8vR2xM9cK6wT3"
	privateID := "q7zn4pl8vr2xm9ck6wt3"
	started := time.Now().UTC()
	source := contract.NewResult(started, contract.Command{Name: "validate", Arguments: map[string]any{"project": "."}})
	source.Finish(started, started.Add(10*time.Millisecond), "PASS", contract.ExitOK, privateText, map[string]any{"scope": "baseline", "check_count": 1})
	payloadBytes, err := marshalRunJSON(map[string]any{
		"schema_version": contract.SchemaVersion,
		"scope":          "baseline",
		"outcome":        "PASS",
		"checks": []map[string]any{
			{"id": privateID, "outcome": "PASS", "summary": privateText},
		},
	})
	if err != nil {
		stateRoot.Close()
		t.Fatal(err)
	}
	payload := runPayload{Kind: "validation-report", Outcome: "PASS", MediaType: "application/json", Content: payloadBytes}
	commit := finishRunForTest(stateRoot, state, source, []runPayload{payload}, nil)
	if err := stateRoot.Close(); err != nil || commit.Err != nil || !commit.Committed {
		t.Fatalf("run fixture did not commit: close=%v commit=%+v", err, commit)
	}

	before := snapshotTree(t, filepath.Join(project, ".gameatelier"))
	code, result, stdout, stderr := execute(t, context.Background(), "logs", "--project", project, "--run-id", source.RunID)
	after := snapshotTree(t, filepath.Join(project, ".gameatelier"))
	if code != contract.ExitOK || result.Outcome != "PASS" || len(result.Evidence) != 0 || stderr != "" {
		t.Fatalf("logs failed: code=%d result=%+v stderr=%q", code, result, stderr)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("logs modified the project state tree")
	}
	if strings.Contains(stdout, privateText) || strings.Contains(stdout, privateID) || strings.Contains(stdout, project) || strings.Contains(stdout, `"message":`) {
		t.Fatalf("logs exposed free text or an absolute project path: %s", stdout)
	}
	if result.Command.Arguments["project"] != "." || result.Command.Arguments["run_id"] != source.RunID {
		t.Fatalf("logs arguments were not normalized: %+v", result.Command.Arguments)
	}
	data := resultDataMap(t, result)
	if data["scope"] != "run" || data["target_run_id"] != source.RunID || data["source_command"] != "validate" || data["source_outcome"] != "PASS" || data["raw_output_included"] != false || data["evidence_kind"] != "validation-report" || data["producer_version"] != Version {
		t.Fatalf("unexpected logs data: %+v", data)
	}
	if hash, ok := data["evidence_sha256"].(string); !ok || len(hash) != 64 || data["evidence_byte_size"] != float64(len(payloadBytes)) {
		t.Fatalf("logs omitted verified integrity metadata: %+v", data)
	}
	events, ok := data["events"].([]any)
	if !ok || len(events) != 2 {
		t.Fatalf("unexpected structured events: %#v", data["events"])
	}
	first := events[0].(map[string]any)
	last := events[1].(map[string]any)
	if first["source"] != "validation-report" || first["kind"] != "check" || first["id"] != "check-0001" || first["outcome"] != "PASS" || first["level"] != "INFO" || last["source"] != "result" || last["kind"] != "result" || last["id"] != "command-finished" {
		t.Fatalf("unexpected event projection: %#v", events)
	}
}

func TestLogsRejectsInvalidMissingIncompleteCorruptAndFutureRuns(t *testing.T) {
	requireInitializePlatform(t)
	code, result, stdout, _ := execute(t, context.Background(), "logs", "--run-id", "../escape")
	if code != contract.ExitUsage || firstErrorCode(result) != "INVALID_ARGUMENT" || strings.Contains(stdout, "escape") {
		t.Fatalf("unsafe run ID was not rejected without reflection: code=%d result=%+v stdout=%q", code, result, stdout)
	}

	project, stateRoot, state := createRunStoreProject(t, "logs incomplete")
	missingID := "atelier-20260825t111111.000000000z-111111111111"
	code, result, _, _ = execute(t, context.Background(), "logs", "--project", project, "--run-id", missingID)
	if code != contract.ExitPrerequisite || firstErrorCode(result) != "RUN_NOT_FOUND" {
		stateRoot.Close()
		t.Fatalf("missing run was not blocked: code=%d result=%+v", code, result)
	}
	started := time.Now().UTC()
	incomplete := contract.NewResult(started, contract.Command{Name: "validate", Arguments: map[string]any{"project": "."}})
	transaction, err := beginRun(stateRoot, state, incomplete, nil)
	if err != nil {
		stateRoot.Close()
		t.Fatal(err)
	}
	if err := transaction.close(); err != nil {
		stateRoot.Close()
		t.Fatal(err)
	}
	if err := stateRoot.Close(); err != nil {
		t.Fatal(err)
	}
	code, result, _, _ = execute(t, context.Background(), "logs", "--project", project, "--run-id", incomplete.RunID)
	if code != contract.ExitPrerequisite || firstErrorCode(result) != "RUN_NOT_COMMITTED" {
		t.Fatalf("incomplete run was not blocked: code=%d result=%+v", code, result)
	}

	intentPath := filepath.Join(project, ".gameatelier", "runs", incomplete.RunID, "intent.json")
	intent, err := os.ReadFile(intentPath)
	if err != nil {
		t.Fatal(err)
	}
	intent = bytes.Replace(intent, []byte(`"schema_version":"1.0.0"`), []byte(`"schema_version":"2.0.0"`), 1)
	if err := os.WriteFile(intentPath, intent, 0o600); err != nil {
		t.Fatal(err)
	}
	code, result, _, _ = execute(t, context.Background(), "logs", "--project", project, "--run-id", incomplete.RunID)
	if code != contract.ExitState || firstErrorCode(result) != "RUN_SCHEMA_UNSUPPORTED" {
		t.Fatalf("future run schema was not protected: code=%d result=%+v", code, result)
	}

	corruptProject, corruptRoot, corruptState := createRunStoreProject(t, "logs corrupt")
	corrupt, corruptPayload := sampleRunTransaction(t)
	commit := finishRunForTest(corruptRoot, corruptState, corrupt, []runPayload{corruptPayload}, nil)
	if err := corruptRoot.Close(); err != nil || commit.Err != nil || !commit.Committed {
		t.Fatalf("corrupt fixture did not commit: close=%v commit=%+v", err, commit)
	}
	payloadPath := filepath.Join(corruptProject, ".gameatelier", "runs", corrupt.RunID, "payloads", "0001-validation-report.json")
	if err := os.WriteFile(payloadPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, result, _, _ = execute(t, context.Background(), "logs", "--project", corruptProject, "--run-id", corrupt.RunID)
	if code != contract.ExitState || firstErrorCode(result) != "RUN_LOGS_UNSAFE" {
		t.Fatalf("corrupt run was not rejected: code=%d result=%+v", code, result)
	}
}

func TestLogsProjectsFailureCodesWithoutFailureText(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, state := createRunStoreProject(t, "logs failure redaction")
	privateText := "R8mQ2zV7kL4nP9xC5tW6"
	started := time.Now().UTC()
	source := contract.NewResult(started, contract.Command{Name: "validate", Arguments: map[string]any{"project": "."}})
	source.Finish(started, started.Add(10*time.Millisecond), "FAIL", contract.ExitValidation, privateText, map[string]any{"scope": "baseline", "check_count": 1}, contract.Error{
		Code: "VALIDATION_FAILED", Category: "validation", Message: privateText, Retryable: false,
	})
	payloadBytes, err := marshalRunJSON(map[string]any{
		"schema_version": contract.SchemaVersion,
		"scope":          "baseline",
		"outcome":        "FAIL",
		"checks": []map[string]any{
			{"id": "failed-check", "outcome": "FAIL", "summary": privateText},
		},
	})
	if err != nil {
		stateRoot.Close()
		t.Fatal(err)
	}
	commit := finishRunForTest(stateRoot, state, source, []runPayload{{Kind: "validation-report", Outcome: "FAIL", MediaType: "application/json", Content: payloadBytes}}, nil)
	if err := stateRoot.Close(); err != nil || commit.Err != nil || !commit.Committed {
		t.Fatalf("failure fixture did not commit: close=%v commit=%+v", err, commit)
	}
	code, result, stdout, _ := execute(t, context.Background(), "logs", "--project", project, "--run-id", source.RunID)
	if code != contract.ExitOK || result.Outcome != "PASS" || strings.Contains(stdout, privateText) || strings.Contains(stdout, "failed-check") || strings.Contains(stdout, "VALIDATION_FAILED") {
		t.Fatalf("failed source run was not safely projected: code=%d result=%+v stdout=%q", code, result, stdout)
	}
	data := resultDataMap(t, result)
	events := data["events"].([]any)
	if len(events) != 3 || events[0].(map[string]any)["id"] != "check-0001" || events[1].(map[string]any)["id"] != "error-0001" || events[1].(map[string]any)["kind"] != "error" || events[1].(map[string]any)["level"] != "ERROR" || events[2].(map[string]any)["id"] != "command-finished" {
		t.Fatalf("failed source run events are incorrect: %#v", events)
	}
}

func TestLogsRejectsSymlinkRunAndHonorsCancellation(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, _ := createRunStoreProject(t, "logs symlink")
	if err := stateRoot.Close(); err != nil {
		t.Fatal(err)
	}
	runs := filepath.Join(project, ".gameatelier", "runs")
	if err := os.MkdirAll(runs, 0o700); err != nil {
		t.Fatal(err)
	}
	runID := "atelier-20260825t121212.000000000z-222222222222"
	if err := os.Symlink(t.TempDir(), filepath.Join(runs, runID)); err != nil {
		t.Fatal(err)
	}
	code, result, _, _ := execute(t, context.Background(), "logs", "--project", project, "--run-id", runID)
	if code != contract.ExitState || firstErrorCode(result) != "RUN_LOGS_UNSAFE" {
		t.Fatalf("symlink run was not rejected: code=%d result=%+v", code, result)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code, result, _, _ = execute(t, ctx, "logs", "--project", project, "--run-id", runID)
	if code != contract.ExitInterrupted || firstErrorCode(result) != "COMMAND_CANCELLED" {
		t.Fatalf("logs cancellation was not preserved: code=%d result=%+v", code, result)
	}
}

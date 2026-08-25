package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

func TestValidateCommitsBaselineEvidenceAndEmitsStoredBytes(t *testing.T) {
	requireInitializePlatform(t)
	project := createProject(t, "验证 项目 🚀")
	code, initialized, _, _ := execute(t, context.Background(), "initialize", "--project", project)
	if code != contract.ExitOK || initialized.Outcome != "PASS" {
		t.Fatalf("initialize failed: code=%d result=%+v", code, initialized)
	}

	code, result, stdout, stderr := execute(t, context.Background(), "validate", "--project", project)
	if code != contract.ExitOK || result.Outcome != "PASS" || len(result.Evidence) != 1 || stderr != "" {
		t.Fatalf("validate failed: code=%d result=%+v stderr=%q", code, result, stderr)
	}
	stored, err := os.ReadFile(filepath.Join(project, ".gameatelier", "runs", result.RunID, "result.json"))
	if err != nil || !bytes.Equal(stored, []byte(stdout)) {
		t.Fatalf("stdout differs from committed result: err=%v", err)
	}
	assertStoredRunClosure(t, project, result.RunID)
	assertResultInvariant(t, result)
}

func TestValidateRejectsSymlinkedProjectFileWithoutFollowingIt(t *testing.T) {
	requireInitializePlatform(t)
	if runtime.GOOS == "windows" {
		t.Skip("Windows reparse behavior requires its native matrix")
	}
	project := createProject(t, "symlinked-project-file")
	code, initialized, _, _ := execute(t, context.Background(), "initialize", "--project", project)
	if code != contract.ExitOK || initialized.Outcome != "PASS" {
		t.Fatalf("initialize failed: code=%d result=%+v", code, initialized)
	}
	external := filepath.Join(t.TempDir(), "external-project.godot")
	if err := os.WriteFile(external, []byte("config_version=5\n[application]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(project, "project.godot")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(project, "project.godot")); err != nil {
		t.Fatal(err)
	}

	code, result, _, _ := execute(t, context.Background(), "validate", "--project", project)
	if code != contract.ExitPrerequisite || result.Outcome != "BLOCKED" || firstErrorCode(result) != "GODOT_PROJECT_UNREADABLE" || len(result.Evidence) != 1 {
		t.Fatalf("symlinked project file was followed or not recorded: code=%d result=%+v", code, result)
	}
	assertStoredRunClosure(t, project, result.RunID)
}

func TestValidatePinsProjectRootAcrossPathReplacement(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, _ := createRunStoreProject(t, "pinned-project-root")
	stateRoot.Close()
	moved := project + "-moved"
	replaced := false
	execution := runValidateWithFault(context.Background(), time.Now().UTC(), []string{"--project", project}, func(stage string) error {
		if stage != "after-intent" || replaced {
			return nil
		}
		replaced = true
		if err := os.Rename(project, moved); err != nil {
			return err
		}
		if err := os.Mkdir(project, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(project, "project.godot"), []byte("config_version=5\n[dotnet]\nproject/assembly_name=\"Replacement\"\n"), 0o644)
	})
	var result contract.Result
	if err := json.Unmarshal(execution.resultBytes, &result); err != nil {
		t.Fatal(err)
	}
	if !replaced || execution.exitCode != contract.ExitOK || result.Outcome != "PASS" {
		t.Fatalf("validate mixed the replacement path with pinned state: replaced=%t execution=%+v result=%+v", replaced, execution, result)
	}
	stored, err := os.ReadFile(filepath.Join(moved, ".gameatelier", "runs", result.RunID, "result.json"))
	if err != nil || !bytes.Equal(stored, execution.resultBytes) {
		t.Fatalf("pinned root did not retain its run: err=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(project, ".gameatelier")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("validate wrote into replacement path: %v", err)
	}
	assertStoredRunClosure(t, moved, result.RunID)
}

func TestValidateBoundsPinnedProjectDirectoryEnumeration(t *testing.T) {
	requireInitializePlatform(t)
	project := createProject(t, "bounded-project-directory")
	code, initialized, _, _ := execute(t, context.Background(), "initialize", "--project", project)
	if code != contract.ExitOK || initialized.Outcome != "PASS" {
		t.Fatalf("initialize failed: code=%d result=%+v", code, initialized)
	}
	for index := 0; index < maxPinnedProjectEntries+1; index++ {
		name := filepath.Join(project, fmt.Sprintf("entry-%04d.txt", index))
		if err := os.WriteFile(name, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	code, result, _, _ := execute(t, context.Background(), "validate", "--project", project)
	if code != contract.ExitPrerequisite || result.Outcome != "BLOCKED" || firstErrorCode(result) != "GODOT_PROJECT_UNREADABLE" || len(result.Evidence) != 1 {
		t.Fatalf("oversized project directory was not bounded and recorded: code=%d result=%+v", code, result)
	}
	assertStoredRunClosure(t, project, result.RunID)
}

func TestValidateCommitFailureReturnsSingleUncommittedFailure(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, _ := createRunStoreProject(t, "validate-precommit-failure")
	stateRoot.Close()
	injected := errors.New("stop before result link")
	execution := runValidateWithFault(context.Background(), time.Now().UTC(), []string{"--project", project}, func(stage string) error {
		if stage == "result:before-link" {
			return injected
		}
		return nil
	})
	var stdout, stderr bytes.Buffer
	code := emitEncodedExecution(&stdout, &stderr, execution)
	var result contract.Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if code != contract.ExitState || result.ExitCode != contract.ExitState || result.Outcome != "FAIL" || firstErrorCode(result) != "RUN_COMMIT_FAILED" || len(result.Evidence) != 0 || stderr.Len() != 0 {
		t.Fatalf("unexpected commit failure: code=%d result=%+v stderr=%q", code, result, stderr.String())
	}
	runRoot := filepath.Join(project, ".gameatelier", "runs", result.RunID)
	if _, err := os.Stat(filepath.Join(runRoot, "intent.json")); err != nil {
		t.Fatalf("pre-result failure lost intent: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(runRoot, "result.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pre-result failure exposed result: %v", err)
	}
}

func TestValidateDistinguishesBeginFailurePhases(t *testing.T) {
	requireInitializePlatform(t)
	for _, test := range []struct {
		stage    string
		wantCode string
		intent   bool
	}{
		{stage: "before-run-directory", wantCode: "RUN_RECORDING_UNAVAILABLE", intent: false},
		{stage: "before-intent", wantCode: "RUN_PREPARE_FAILED", intent: false},
		{stage: "after-intent", wantCode: "RUN_COMMIT_FAILED", intent: true},
	} {
		t.Run(test.stage, func(t *testing.T) {
			project, stateRoot, _ := createRunStoreProject(t, "begin-phase-"+test.stage)
			stateRoot.Close()
			execution := runValidateWithFault(context.Background(), time.Now().UTC(), []string{"--project", project}, func(stage string) error {
				if stage == test.stage {
					return errors.New("injected begin failure")
				}
				return nil
			})
			var result contract.Result
			if err := json.Unmarshal(execution.resultBytes, &result); err != nil {
				t.Fatal(err)
			}
			if execution.exitCode != contract.ExitState || firstErrorCode(result) != test.wantCode || len(result.Evidence) != 0 {
				t.Fatalf("unexpected begin failure mapping: execution=%+v result=%+v", execution, result)
			}
			intentPath := filepath.Join(project, ".gameatelier", "runs", result.RunID, "intent.json")
			_, statErr := os.Stat(intentPath)
			if test.intent && statErr != nil {
				t.Fatalf("intent-published failure lost intent: %v", statErr)
			}
			if !test.intent && !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("pre-intent failure exposed intent: %v", statErr)
			}
		})
	}
}

func TestValidateCancellationBeforeAndDuringOperationIsCommitted(t *testing.T) {
	requireInitializePlatform(t)
	for _, test := range []struct {
		name string
		ctx  context.Context
	}{
		{name: "before operation", ctx: cancelledContext()},
		{name: "during operation", ctx: &countingCancelContext{cancelAfter: 2}},
	} {
		t.Run(test.name, func(t *testing.T) {
			project, stateRoot, _ := createRunStoreProject(t, "cancel-"+test.name)
			stateRoot.Close()
			execution := runValidateWithFault(test.ctx, time.Now().UTC(), []string{"--project", project}, nil)
			var result contract.Result
			if err := json.Unmarshal(execution.resultBytes, &result); err != nil {
				t.Fatal(err)
			}
			if execution.exitCode != contract.ExitInterrupted || result.Outcome != "FAIL" || firstErrorCode(result) != "COMMAND_CANCELLED" || len(result.Evidence) != 1 {
				t.Fatalf("cancellation was not committed consistently: execution=%+v result=%+v", execution, result)
			}
			assertStoredRunClosure(t, project, result.RunID)
		})
	}
}

func TestValidatePostLinkDurabilityFailureKeepsStoredResultAuthoritative(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, _ := createRunStoreProject(t, "validate-postlink-failure")
	stateRoot.Close()
	injected := errors.New("directory sync unavailable")
	execution := runValidateWithFault(context.Background(), time.Now().UTC(), []string{"--project", project}, func(stage string) error {
		if stage == "result:before-directory-sync" {
			return injected
		}
		return nil
	})
	var stdout, stderr bytes.Buffer
	code := emitEncodedExecution(&stdout, &stderr, execution)
	var result contract.Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if code != result.ExitCode || result.Outcome != "PASS" || stderr.String() != committedRunWarning {
		t.Fatalf("unexpected post-link semantics: code=%d result=%+v stderr=%q", code, result, stderr.String())
	}
	stored, err := os.ReadFile(filepath.Join(project, ".gameatelier", "runs", result.RunID, "result.json"))
	if err != nil || !bytes.Equal(stored, stdout.Bytes()) {
		t.Fatalf("post-link stdout differs from authority: err=%v", err)
	}
	assertStoredRunClosure(t, project, result.RunID)
}

func TestCommittedValidateShortWriterReturnsInternalWithoutChangingRun(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, _ := createRunStoreProject(t, "validate-short-writer")
	stateRoot.Close()
	execution := runValidateWithFault(context.Background(), time.Now().UTC(), []string{"--project", project}, nil)
	var result contract.Result
	if err := json.Unmarshal(execution.resultBytes, &result); err != nil {
		t.Fatal(err)
	}
	stdout := &failingWriter{limit: len(execution.resultBytes) / 2, err: io.ErrClosedPipe}
	var stderr bytes.Buffer
	code := emitEncodedExecution(stdout, &stderr, execution)
	if code != contract.ExitInternal || stderr.String() != "failed to write committed command result\n" || len(stdout.content) == 0 || len(stdout.content) >= len(execution.resultBytes) {
		t.Fatalf("short write was not surfaced: code=%d wrote=%d stderr=%q", code, len(stdout.content), stderr.String())
	}
	stored, err := os.ReadFile(filepath.Join(project, ".gameatelier", "runs", result.RunID, "result.json"))
	if err != nil || !bytes.Equal(stored, execution.resultBytes) {
		t.Fatalf("short stdout write changed committed run: err=%v", err)
	}
	assertStoredRunClosure(t, project, result.RunID)
}

type failingWriter struct {
	content []byte
	limit   int
	err     error
}

type countingCancelContext struct {
	calls       int
	cancelAfter int
}

func (ctx *countingCancelContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (ctx *countingCancelContext) Done() <-chan struct{}       { return nil }
func (ctx *countingCancelContext) Value(any) any               { return nil }
func (ctx *countingCancelContext) Err() error {
	ctx.calls++
	if ctx.calls >= ctx.cancelAfter {
		return context.Canceled
	}
	return nil
}

func cancelledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func (writer *failingWriter) Write(content []byte) (int, error) {
	remaining := writer.limit - len(writer.content)
	if remaining <= 0 {
		return 0, writer.err
	}
	if remaining > len(content) {
		remaining = len(content)
	}
	writer.content = append(writer.content, content[:remaining]...)
	if remaining < len(content) {
		return remaining, writer.err
	}
	return remaining, nil
}

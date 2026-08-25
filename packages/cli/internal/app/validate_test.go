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
	"strings"
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

func TestValidateHeadlessRequiresExplicitUserDataAuthorizationBeforeGodotStarts(t *testing.T) {
	requireInitializePlatform(t)
	project := createProject(t, "headless-policy")
	code, initialized, _, _ := execute(t, context.Background(), "initialize", "--project", project)
	if code != contract.ExitOK || initialized.Outcome != "PASS" {
		t.Fatalf("initialize failed: code=%d result=%+v", code, initialized)
	}
	marker := filepath.Join(t.TempDir(), "started")
	godot := createExecutable(t, "fake-godot", "#!/bin/sh\nprintf started > '"+marker+"'\n")

	code, result, _, _ := execute(t, context.Background(), "validate", "--project", project, "--headless", "--godot", godot)
	if code != contract.ExitPrerequisite || result.Outcome != "BLOCKED" || firstErrorCode(result) != "ENGINE_USER_DATA_NOT_AUTHORIZED" || len(result.Evidence) != 1 {
		t.Fatalf("unexpected policy result: code=%d result=%+v", code, result)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("Godot started without authorization: %v", err)
	}
	if result.Command.Arguments["engine_user_data"] != "not-authorized" || result.Command.Arguments["godot_source"] != "explicit" {
		t.Fatalf("headless policy was not normalized in the result: %+v", result.Command.Arguments)
	}
	assertStoredRunClosure(t, project, result.RunID)
}

func TestValidateHeadlessCommitsPassingFixedEngineRun(t *testing.T) {
	requireInitializePlatform(t)
	project := createProject(t, "验证 Headless 🚀")
	code, initialized, _, _ := execute(t, context.Background(), "initialize", "--project", project)
	if code != contract.ExitOK || initialized.Outcome != "PASS" {
		t.Fatalf("initialize failed: code=%d result=%+v", code, initialized)
	}
	godot := createExecutable(t, "fake-godot", "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then printf '4.7.2.stable.official.ed1daf0bf\\n'; exit 0; fi\nprintf 'scene ready\\n'\n")

	code, result, stdout, stderr := execute(t, context.Background(), "validate", "--project", project, "--headless", "--godot", godot, "--timeout-ms", "2000", "--allow-engine-user-data")
	if code != contract.ExitOK || result.Outcome != "PASS" || len(result.Evidence) != 1 || stderr != "" {
		t.Fatalf("headless validate failed: code=%d result=%+v stderr=%q", code, result, stderr)
	}
	if strings.Contains(stdout, godot) || result.Command.Arguments["engine_user_data"] != "standard-os-location" || result.Command.Arguments["timeout_ms"] != float64(2000) {
		t.Fatalf("result leaked an executable path or lost normalized arguments: %s", stdout)
	}
	data := resultDataMap(t, result)
	if data["scope"] != "headless" || data["check_count"] != float64(8) {
		t.Fatalf("unexpected headless data: %+v", data)
	}
	intentBytes, err := os.ReadFile(filepath.Join(project, ".gameatelier", "runs", result.RunID, "intent.json"))
	if err != nil {
		t.Fatal(err)
	}
	var intent runIntentRecord
	if err := json.Unmarshal(intentBytes, &intent); err != nil || len(intent.DeclaredExternal) != 1 || intent.DeclaredExternal[0] != "godot:user-data:standard-os-location" {
		t.Fatalf("headless external write was not declared: err=%v intent=%+v", err, intent)
	}
	assertStoredRunClosure(t, project, result.RunID)
}

func TestValidateHeadlessCommitsEngineErrorAndTimeout(t *testing.T) {
	requireInitializePlatform(t)
	for _, test := range []struct {
		name      string
		body      string
		timeoutMS string
		wantExit  int
		wantError string
	}{
		{name: "engine error", body: "printf 'ERROR: scene failed\\n' >&2", timeoutMS: "2000", wantExit: contract.ExitEngine, wantError: "GODOT_REPORTED_ERRORS"},
		{name: "timeout", body: "sleep 5", timeoutMS: "100", wantExit: contract.ExitInterrupted, wantError: "GODOT_TIMEOUT"},
	} {
		t.Run(test.name, func(t *testing.T) {
			project := createProject(t, "headless-"+test.name)
			code, initialized, _, _ := execute(t, context.Background(), "initialize", "--project", project)
			if code != contract.ExitOK || initialized.Outcome != "PASS" {
				t.Fatalf("initialize failed: code=%d result=%+v", code, initialized)
			}
			script := "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then printf '4.7.2.stable.official.ed1daf0bf\\n'; exit 0; fi\n" + test.body + "\n"
			godot := createExecutable(t, "fake-godot", script)

			code, result, _, _ := execute(t, context.Background(), "validate", "--project", project, "--headless", "--godot", godot, "--timeout-ms", test.timeoutMS, "--allow-engine-user-data")
			if code != test.wantExit || result.Outcome != "FAIL" || firstErrorCode(result) != test.wantError || len(result.Evidence) != 1 {
				t.Fatalf("unexpected headless failure: code=%d result=%+v", code, result)
			}
			assertStoredRunClosure(t, project, result.RunID)
		})
	}
}

func TestValidateRejectsHeadlessOnlyFlagsWithoutHeadless(t *testing.T) {
	project := createProject(t, "invalid-headless-flags")
	for _, args := range [][]string{
		{"validate", "--project", project, "--timeout-ms", "100"},
		{"validate", "--project", project, "--allow-engine-user-data"},
		{"validate", "--project", project, "--godot", "/tmp/godot"},
	} {
		code, result, _, _ := execute(t, context.Background(), args...)
		if code != contract.ExitUsage || firstErrorCode(result) != "INVALID_ARGUMENT" || len(result.Evidence) != 0 {
			t.Fatalf("unexpected invalid-flag result for %v: code=%d result=%+v", args, code, result)
		}
	}
}

func TestValidationRunCommitGateRejectsCleanupFailureAndTransientFiles(t *testing.T) {
	runPath := t.TempDir()
	runRoot, err := os.OpenRoot(runPath)
	if err != nil {
		t.Fatal(err)
	}
	defer runRoot.Close()
	result := contract.Result{Errors: []contract.Error{{Code: "GODOT_EXECUTABLE_SNAPSHOT_CLEANUP_FAILED"}}}
	if !validationRunMustRemainIncomplete(result, runRoot) {
		t.Fatal("cleanup failure did not block result publication")
	}
	result.Errors = nil
	if err := os.WriteFile(filepath.Join(runPath, ".atelier-scene-runner.cstemp"), []byte("transient"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !validationRunMustRemainIncomplete(result, runRoot) {
		t.Fatal("transient runner file did not block result publication")
	}
}

func TestValidateHeadlessKeepsScopeWhenProjectIsUnavailable(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-project")
	code, result, _, _ := execute(t, context.Background(), "validate", "--project", missing, "--headless", "--allow-engine-user-data")
	if code != contract.ExitPrerequisite || result.Outcome != "BLOCKED" || firstErrorCode(result) != "GODOT_PROJECT_NOT_FOUND" {
		t.Fatalf("unexpected unavailable-project result: code=%d result=%+v", code, result)
	}
	if result.Command.Arguments["headless"] != true || resultDataMap(t, result)["scope"] != "headless" {
		t.Fatalf("headless command lost its scope before evidence recording: %+v", result)
	}
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

func TestValidateHeadlessRejectsPathReplacementBeforeStartingGodot(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, _ := createRunStoreProject(t, "headless-pinned-project")
	stateRoot.Close()
	moved := project + "-moved"
	marker := filepath.Join(t.TempDir(), "godot-started")
	godot := createExecutable(t, "fake-godot", "#!/bin/sh\nprintf started > '"+marker+"'\nprintf '4.7.2.stable.official.ed1daf0bf\\n'\n")
	replaced := false
	execution := runValidateWithFault(context.Background(), time.Now().UTC(), []string{
		"--project", project,
		"--headless",
		"--godot", godot,
		"--allow-engine-user-data",
	}, func(stage string) error {
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
		return os.WriteFile(filepath.Join(project, "project.godot"), []byte("config_version=5\n[application]\nconfig/name=\"Replacement\"\n"), 0o644)
	})
	var result contract.Result
	if err := json.Unmarshal(execution.resultBytes, &result); err != nil {
		t.Fatal(err)
	}
	if !replaced || execution.exitCode != contract.ExitState || result.Outcome != "FAIL" || firstErrorCode(result) != "PROJECT_CHANGED_DURING_VALIDATION" {
		t.Fatalf("headless replacement was not rejected: replaced=%t execution=%+v result=%+v", replaced, execution, result)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("Godot started after project path replacement: %v", err)
	}
	assertStoredRunClosure(t, moved, result.RunID)
}

func TestValidateHeadlessDiscardsObservationsAfterPathReplacement(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, _ := createRunStoreProject(t, "headless-runtime-replacement")
	stateRoot.Close()
	moved := project + "-moved"
	marker := filepath.Join(t.TempDir(), "godot-ran")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ]; then printf '4.7.2.stable.official.ed1daf0bf\\n'; exit 0; fi\n" +
		"printf ran > '" + marker + "'\n" +
		"mv '" + project + "' '" + moved + "'\n" +
		"mkdir '" + project + "'\n" +
		"printf 'config_version=5\\n[application]\\nconfig/name=\"Replacement\"\\n' > '" + filepath.Join(project, "project.godot") + "'\n"
	godot := createExecutable(t, "fake-godot", script)

	execution := runValidateWithFault(context.Background(), time.Now().UTC(), []string{
		"--project", project,
		"--headless",
		"--godot", godot,
		"--allow-engine-user-data",
	}, nil)
	var result contract.Result
	if err := json.Unmarshal(execution.resultBytes, &result); err != nil {
		t.Fatal(err)
	}
	if execution.exitCode != contract.ExitState || result.Outcome != "FAIL" || firstErrorCode(result) != "PROJECT_CHANGED_DURING_VALIDATION" {
		t.Fatalf("runtime replacement observations were not discarded: execution=%+v result=%+v", execution, result)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("fake Godot did not reach the replacement step: %v", err)
	}
	if content, err := os.ReadFile(filepath.Join(moved, "project.godot")); err != nil || strings.Contains(string(content), "Replacement") {
		t.Fatalf("Godot was redirected to the replacement project: content=%q err=%v", content, err)
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

func TestValidateHeadlessKeepsScopeAcrossEvidenceFailures(t *testing.T) {
	requireInitializePlatform(t)
	for _, test := range []struct {
		name     string
		stage    string
		wantCode string
	}{
		{name: "recording unavailable", stage: "before-run-directory", wantCode: "RUN_RECORDING_UNAVAILABLE"},
		{name: "prepare failure", stage: "before-intent", wantCode: "RUN_PREPARE_FAILED"},
		{name: "incomplete intent", stage: "after-intent", wantCode: "RUN_COMMIT_FAILED"},
		{name: "result commit failure", stage: "result:before-link", wantCode: "RUN_COMMIT_FAILED"},
	} {
		t.Run(test.name, func(t *testing.T) {
			project, stateRoot, _ := createRunStoreProject(t, "headless-evidence-"+test.name)
			stateRoot.Close()
			godot := createExecutable(t, "fake-godot", "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then printf '4.7.2.stable.official.ed1daf0bf\\n'; fi\n")
			execution := runValidateWithFault(context.Background(), time.Now().UTC(), []string{
				"--project", project,
				"--headless",
				"--godot", godot,
				"--allow-engine-user-data",
			}, func(stage string) error {
				if stage == test.stage {
					return errors.New("injected evidence failure")
				}
				return nil
			})
			var result contract.Result
			if err := json.Unmarshal(execution.resultBytes, &result); err != nil {
				t.Fatal(err)
			}
			if execution.exitCode != contract.ExitState || firstErrorCode(result) != test.wantCode || result.Command.Arguments["headless"] != true || resultDataMap(t, result)["scope"] != "headless" {
				t.Fatalf("headless evidence failure lost semantic scope: execution=%+v result=%+v", execution, result)
			}
		})
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

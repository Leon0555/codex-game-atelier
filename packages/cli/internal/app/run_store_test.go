package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

func TestCommitRunPublishesCompleteClosureAndLeavesProjectStateUnchanged(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, state := createRunStoreProject(t, "完整 evidence 🚀")
	defer stateRoot.Close()
	statePath := filepath.Join(project, ".gameatelier", "project.json")
	stateBefore, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	result, payload := sampleRunTransaction(t)
	committed := finishRunForTest(stateRoot, state, result, []runPayload{payload}, nil)
	if committed.Err != nil || !committed.Committed {
		t.Fatalf("commitRun failed: %+v", committed)
	}
	runRoot := filepath.Join(project, ".gameatelier", "runs", result.RunID)
	storedResult, err := os.ReadFile(filepath.Join(runRoot, "result.json"))
	if err != nil || !bytes.Equal(storedResult, committed.ResultBytes) {
		t.Fatalf("stored/stdout result mismatch: err=%v", err)
	}
	var observed contract.Result
	if err := json.Unmarshal(storedResult, &observed); err != nil || len(observed.Evidence) != 1 {
		t.Fatalf("stored result invalid: err=%v result=%+v", err, observed)
	}
	recordBytes, err := os.ReadFile(filepath.Join(runRoot, "evidence", "0001-validation-report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record persistedEvidenceRecord
	if err := json.Unmarshal(recordBytes, &record); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload.Content)
	if record.RunID != result.RunID || record.ID != observed.Evidence[0].ID || record.Path != ".gameatelier/runs/"+result.RunID+"/payloads/0001-validation-report.json" || record.SHA256 != hex.EncodeToString(sum[:]) || record.ByteSize != int64(len(payload.Content)) {
		t.Fatalf("unexpected evidence record: %+v", record)
	}
	stateAfter, err := os.ReadFile(statePath)
	if err != nil || !bytes.Equal(stateBefore, stateAfter) {
		t.Fatalf("run commit changed project state: err=%v", err)
	}

	second := finishRunForTest(stateRoot, state, result, []runPayload{payload}, nil)
	if second.Err == nil || second.Committed {
		t.Fatalf("same run ID was overwritten or recommitted: %+v", second)
	}
	afterSecond, err := os.ReadFile(filepath.Join(runRoot, "result.json"))
	if err != nil || !bytes.Equal(afterSecond, storedResult) {
		t.Fatalf("duplicate commit changed result: err=%v", err)
	}
}

func TestBeginRunPublishesIntentBeforeOperationAndFinishPublishesResult(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, state := createRunStoreProject(t, "intent-before-operation")
	defer stateRoot.Close()
	started := time.Now().UTC()
	result := contract.NewResult(started, contract.Command{Name: "validate", Arguments: map[string]any{"project": "."}})
	transaction, err := beginRun(stateRoot, state, result, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.close()
	runRoot := filepath.Join(project, ".gameatelier", "runs", result.RunID)
	if _, err := os.Stat(filepath.Join(runRoot, "intent.json")); err != nil {
		t.Fatalf("begin did not durably publish intent: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(runRoot, "result.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("begin exposed a result before the operation: %v", err)
	}
	finishSampleResult(&result, started)
	payload := sampleValidationPayload(t)
	committed := transaction.finish(result, []runPayload{payload})
	if committed.Err != nil || !committed.Committed {
		t.Fatalf("finish failed: %+v", committed)
	}
	assertStoredRunClosure(t, project, result.RunID)
}

func TestBeginRunFaultBeforeIntentLeavesOrphanThatCannotBePass(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, state := createRunStoreProject(t, "orphan-before-intent")
	defer stateRoot.Close()
	result, _ := sampleRunTransaction(t)
	injected := errors.New("stop before intent")
	transaction, err := beginRun(stateRoot, state, result, func(stage string) error {
		if stage == "before-intent" {
			return injected
		}
		return nil
	})
	if transaction != nil || !errors.Is(err, injected) {
		t.Fatalf("before-intent fault unexpectedly opened transaction: transaction=%v err=%v", transaction, err)
	}
	runRoot := filepath.Join(project, ".gameatelier", "runs", result.RunID)
	if info, err := os.Stat(runRoot); err != nil || !info.IsDir() {
		t.Fatalf("expected explicit orphan run directory: info=%v err=%v", info, err)
	}
	for _, name := range []string{"intent.json", "result.json"} {
		if _, err := os.Lstat(filepath.Join(runRoot, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("orphan exposed %s: %v", name, err)
		}
	}
}

func TestCommitRunFaultsNeverExposeResultBeforeCommitPoint(t *testing.T) {
	requireInitializePlatform(t)
	stages := []string{
		"before-run-directory",
		"before-intent",
		"after-intent",
		"before-payload-0001",
		"before-evidence-0001",
		"before-result",
	}
	for _, component := range []string{"intent", "payload-0001", "evidence-0001", "result"} {
		for _, operation := range []string{"before-create", "before-write", "before-file-sync", "before-close", "before-link"} {
			stages = append(stages, component+":"+operation)
		}
	}
	for _, component := range []string{"intent", "payload-0001", "evidence-0001"} {
		for _, operation := range []string{"before-remove", "before-directory-sync"} {
			stages = append(stages, component+":"+operation)
		}
	}
	for _, stage := range stages {
		t.Run(stage, func(t *testing.T) {
			project, stateRoot, state := createRunStoreProject(t, stage)
			defer stateRoot.Close()
			result, payload := sampleRunTransaction(t)
			injected := errors.New("injected run-store fault")
			committed := finishRunForTest(stateRoot, state, result, []runPayload{payload}, func(observed string) error {
				if observed == stage {
					return injected
				}
				return nil
			})
			if !errors.Is(committed.Err, injected) || committed.Committed {
				t.Fatalf("fault did not stop before commit: %+v", committed)
			}
			resultPath := filepath.Join(project, ".gameatelier", "runs", result.RunID, "result.json")
			if _, err := os.Lstat(resultPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("pre-commit fault exposed result: %v", err)
			}
		})
	}
}

func TestCommitRunRejectsMultipleEvidenceRecordsBeforeWriting(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, state := createRunStoreProject(t, "multiple-evidence")
	defer stateRoot.Close()
	result, first := sampleRunTransaction(t)
	second := first
	committed := finishRunForTest(stateRoot, state, result, []runPayload{first, second}, nil)
	if committed.Err == nil || committed.Committed {
		t.Fatalf("multi-evidence first-slice transaction was accepted: %+v", committed)
	}
	if _, err := os.Lstat(filepath.Join(project, ".gameatelier", "runs", result.RunID, "result.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("multi-evidence preflight failure exposed result: %v", err)
	}
}

func TestFinishRejectsOversizedResultBeforeWritingPayloads(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, state := createRunStoreProject(t, "oversized-result")
	defer stateRoot.Close()
	result, payload := sampleRunTransaction(t)
	result.Outcome = "FAIL"
	result.ExitCode = contract.ExitState
	result.Summary = "Baseline project validation failed."
	result.Errors = make([]contract.Error, 64)
	for index := range result.Errors {
		result.Errors[index] = contract.Error{
			Code:      "STATE_INVALID",
			Category:  "state",
			Message:   "Project state is invalid.",
			Retryable: false,
			Details:   map[string]any{"reason": string(bytes.Repeat([]byte{'x'}, 65500))},
		}
	}
	payload = failingValidationPayload(t)
	transaction, err := beginRun(stateRoot, state, result, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.close()
	committed := transaction.finish(result, []runPayload{payload})
	if committed.Err == nil || committed.Committed {
		t.Fatalf("oversized result passed finish preflight: %+v", committed)
	}
	runRoot := filepath.Join(project, ".gameatelier", "runs", result.RunID)
	if _, err := os.Stat(filepath.Join(runRoot, "intent.json")); err != nil {
		t.Fatalf("oversized operation lost its intent: %v", err)
	}
	for _, name := range []string{"payloads", "evidence", "result.json"} {
		if _, err := os.Lstat(filepath.Join(runRoot, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("oversized result wrote %s before rejection: %v", name, err)
		}
	}
}

func TestCommitRunHardExitRespectsResultCommitPoint(t *testing.T) {
	requireInitializePlatform(t)
	for _, test := range []struct {
		stage      string
		wantResult bool
	}{
		{stage: "payload-0001:before-link", wantResult: false},
		{stage: "evidence-0001:before-link", wantResult: false},
		{stage: "result:before-link", wantResult: false},
		{stage: "after-intent", wantResult: false},
		{stage: "during-operation", wantResult: false},
		{stage: "result:before-remove", wantResult: true},
	} {
		t.Run(test.stage, func(t *testing.T) {
			project, stateRoot, _ := createRunStoreProject(t, "hard-exit-"+test.stage)
			stateRoot.Close()
			runID := "atelier-20260825t010203.000000000z-0123456789ab"
			command := exec.Command(os.Args[0], "-test.run=^TestRunStoreCrashHelper$")
			command.Env = append(os.Environ(),
				"ATELIER_RUN_STORE_CRASH_STAGE="+test.stage,
				"ATELIER_RUN_STORE_PROJECT="+project,
				"ATELIER_RUN_STORE_RUN_ID="+runID,
			)
			err := command.Run()
			var exitError *exec.ExitError
			if !errors.As(err, &exitError) || exitError.ExitCode() != 93 {
				t.Fatalf("crash helper exit=%v, want 93", err)
			}
			_, statErr := os.Lstat(filepath.Join(project, ".gameatelier", "runs", runID, "result.json"))
			if test.wantResult && statErr != nil {
				t.Fatalf("post-link hard exit lost result: %v", statErr)
			}
			if test.wantResult {
				assertStoredRunClosure(t, project, runID)
			}
			if !test.wantResult && !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("pre-link hard exit exposed result: %v", statErr)
			}
			if test.stage == "after-intent" || test.stage == "during-operation" {
				if _, err := os.Stat(filepath.Join(project, ".gameatelier", "runs", runID, "intent.json")); err != nil {
					t.Fatalf("operation-window hard exit lost intent: %v", err)
				}
			}
		})
	}
}

func TestRunStoreCrashHelper(t *testing.T) {
	stage := os.Getenv("ATELIER_RUN_STORE_CRASH_STAGE")
	if stage == "" {
		t.Skip("subprocess helper")
	}
	project := os.Getenv("ATELIER_RUN_STORE_PROJECT")
	runID := os.Getenv("ATELIER_RUN_STORE_RUN_ID")
	stateRoot, exists, err := openExistingStateRoot(project)
	if err != nil || !exists {
		os.Exit(91)
	}
	defer stateRoot.Close()
	state, exists, _, err := loadExistingState(stateRoot)
	if err != nil || !exists {
		os.Exit(92)
	}
	started := time.Now().UTC()
	result := contract.NewResult(started, contract.Command{Name: "validate", Arguments: map[string]any{"project": "."}})
	result.RunID = runID
	transaction, err := beginRun(stateRoot, state, result, func(observed string) error {
		if observed == stage {
			os.Exit(93)
		}
		return nil
	})
	if err != nil {
		os.Exit(95)
	}
	defer transaction.close()
	if stage == "during-operation" {
		os.Exit(93)
	}
	finishSampleResult(&result, started)
	payload := sampleValidationPayload(t)
	_ = transaction.finish(result, []runPayload{payload})
	os.Exit(94)
}

func TestCommitRunDurabilityFaultAfterResultLinkKeepsCommitPoint(t *testing.T) {
	requireInitializePlatform(t)
	for _, stage := range []string{"result:before-remove", "result:before-directory-sync"} {
		t.Run(stage, func(t *testing.T) {
			project, stateRoot, state := createRunStoreProject(t, stage)
			defer stateRoot.Close()
			result, payload := sampleRunTransaction(t)
			injected := errors.New("injected durability fault")
			committed := finishRunForTest(stateRoot, state, result, []runPayload{payload}, func(observed string) error {
				if observed == stage {
					return injected
				}
				return nil
			})
			if !committed.Committed || !errors.Is(committed.Err, injected) {
				t.Fatalf("post-link durability fault lost commit point: %+v", committed)
			}
			stored, err := os.ReadFile(filepath.Join(project, ".gameatelier", "runs", result.RunID, "result.json"))
			if err != nil || !bytes.Equal(stored, committed.ResultBytes) {
				t.Fatalf("result commit point missing: err=%v", err)
			}
		})
	}
}

func TestCommitRunAfterResultFailureKeepsAuthoritativeResult(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, state := createRunStoreProject(t, "after-result")
	defer stateRoot.Close()
	result, payload := sampleRunTransaction(t)
	injected := errors.New("stdout unavailable")
	committed := finishRunForTest(stateRoot, state, result, []runPayload{payload}, func(stage string) error {
		if stage == "after-result" {
			return injected
		}
		return nil
	})
	if !committed.Committed || !errors.Is(committed.Err, injected) || len(committed.ResultBytes) == 0 {
		t.Fatalf("post-result failure lost commit status: %+v", committed)
	}
	stored, err := os.ReadFile(filepath.Join(project, ".gameatelier", "runs", result.RunID, "result.json"))
	if err != nil || !bytes.Equal(stored, committed.ResultBytes) {
		t.Fatalf("authoritative result missing after post-commit failure: err=%v", err)
	}
}

func TestCommitRunRejectsSymlinkedRunsDirectory(t *testing.T) {
	requireInitializePlatform(t)
	if runtime.GOOS == "windows" {
		t.Skip("Windows reparse behavior requires its native matrix")
	}
	project, stateRoot, state := createRunStoreProject(t, "symlink-runs")
	defer stateRoot.Close()
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(project, ".gameatelier", "runs")); err != nil {
		t.Fatal(err)
	}
	result, payload := sampleRunTransaction(t)
	committed := finishRunForTest(stateRoot, state, result, []runPayload{payload}, nil)
	if committed.Err == nil || committed.Committed {
		t.Fatalf("symlinked runs directory was accepted: %+v", committed)
	}
	entries, err := os.ReadDir(external)
	if err != nil || len(entries) != 0 {
		t.Fatalf("run store escaped through symlink: err=%v entries=%v", err, entries)
	}
}

func TestCommitRunSupportsConcurrentUniqueRuns(t *testing.T) {
	requireInitializePlatform(t)
	project, initialRoot, state := createRunStoreProject(t, "concurrent-runs")
	initialRoot.Close()
	const workers = 12
	var group sync.WaitGroup
	errorsFound := make(chan error, workers)
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			root, exists, err := openExistingStateRoot(project)
			if err != nil || !exists {
				errorsFound <- errors.New("state root unavailable")
				return
			}
			defer root.Close()
			result, payload := sampleRunTransaction(t)
			committed := finishRunForTest(root, state, result, []runPayload{payload}, nil)
			if committed.Err != nil || !committed.Committed {
				errorsFound <- committed.Err
			}
		}()
	}
	group.Wait()
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(project, ".gameatelier", "runs"))
	if err != nil || len(entries) != workers {
		t.Fatalf("concurrent run count=%d err=%v, want %d", len(entries), err, workers)
	}
}

func TestCommitRunPreflightRejectsUnsafeOrSchemaInvalidContentBeforeWriting(t *testing.T) {
	requireInitializePlatform(t)
	tests := []struct {
		name   string
		mutate func(*projectState, *contract.Result, *runPayload)
	}{
		{name: "absolute project path", mutate: func(_ *projectState, result *contract.Result, _ *runPayload) {
			result.Command.Arguments["project"] = "/Users/example/private-project"
		}},
		{name: "non-scalar project path", mutate: func(_ *projectState, result *contract.Result, _ *runPayload) {
			result.Command.Arguments["project"] = map[string]any{"path": "."}
		}},
		{name: "missing project argument", mutate: func(_ *projectState, result *contract.Result, _ *runPayload) {
			result.Command.Arguments = map[string]any{"foo": "."}
		}},
		{name: "secret argument key", mutate: func(_ *projectState, result *contract.Result, _ *runPayload) {
			result.Command.Arguments["api_token"] = "redacted"
		}},
		{name: "credential-shaped data", mutate: func(_ *projectState, result *contract.Result, _ *runPayload) {
			result.Summary = "Project at /Users/example/private-project passed."
		}},
		{name: "unsupported command", mutate: func(_ *projectState, result *contract.Result, _ *runPayload) {
			result.Command.Name = "doctor"
		}},
		{name: "invalid result time", mutate: func(_ *projectState, result *contract.Result, _ *runPayload) {
			result.FinishedAt = "not-a-time"
		}},
		{name: "invalid exit invariant", mutate: func(_ *projectState, result *contract.Result, _ *runPayload) {
			result.ExitCode = contract.ExitEngine
		}},
		{name: "nil errors", mutate: func(_ *projectState, result *contract.Result, _ *runPayload) {
			result.Errors = nil
		}},
		{name: "unsupported evidence kind", mutate: func(_ *projectState, _ *contract.Result, payload *runPayload) {
			payload.Kind = "custom-report"
		}},
		{name: "overlong media type", mutate: func(_ *projectState, _ *contract.Result, payload *runPayload) {
			payload.MediaType = "application/" + string(bytes.Repeat([]byte{'x'}, 200))
		}},
		{name: "metadata not frozen", mutate: func(_ *projectState, _ *contract.Result, payload *runPayload) {
			payload.Metadata = map[string]any{"attempt": 1}
		}},
		{name: "conflicting evidence outcome", mutate: func(_ *projectState, _ *contract.Result, payload *runPayload) {
			payload.Outcome = "FAIL"
		}},
		{name: "duplicate payload key", mutate: func(_ *projectState, _ *contract.Result, payload *runPayload) {
			payload.Content = []byte(`{"scope":"baseline","scope":"other"}`)
		}},
		{name: "multiple payload values", mutate: func(_ *projectState, _ *contract.Result, payload *runPayload) {
			payload.Content = []byte(`{} {}`)
		}},
		{name: "empty validation report", mutate: func(_ *projectState, _ *contract.Result, payload *runPayload) {
			payload.Content = []byte(`{}`)
		}},
		{name: "mismatched check count", mutate: func(_ *projectState, result *contract.Result, _ *runPayload) {
			result.Data = map[string]any{"scope": "baseline", "check_count": 1}
		}},
		{name: "command and result scope mismatch", mutate: func(_ *projectState, result *contract.Result, _ *runPayload) {
			result.Command.Arguments = map[string]any{
				"project":          ".",
				"headless":         true,
				"timeout_ms":       int64(5000),
				"engine_user_data": "not-authorized",
				"godot_source":     "explicit",
			}
		}},
		{name: "invalid project state", mutate: func(state *projectState, _ *contract.Result, _ *runPayload) {
			state.Mode = "unbounded"
		}},
		{name: "non-generated project identity", mutate: func(state *projectState, _ *contract.Result, _ *runPayload) {
			state.ProjectID = "ghp_not-a-project-identity"
		}},
		{name: "unsafe engine version", mutate: func(state *projectState, _ *contract.Result, _ *runPayload) {
			state.Engine.RequestedVersion = "token=hidden-value"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project, stateRoot, state := createRunStoreProject(t, test.name)
			defer stateRoot.Close()
			result, payload := sampleRunTransaction(t)
			test.mutate(&state, &result, &payload)
			committed := finishRunForTest(stateRoot, state, result, []runPayload{payload}, nil)
			if committed.Err == nil || committed.Committed {
				t.Fatalf("unsafe transaction was committed: %+v", committed)
			}
			if _, err := os.Lstat(filepath.Join(project, ".gameatelier", "runs", result.RunID, "result.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("preflight failure exposed result: %v", err)
			}
		})
	}
}

func TestPersistedJSONRejectsUnsafeKeysAndCumulativeKeyBudget(t *testing.T) {
	if err := validatePersistedJSON(map[string]any{"/Users/private-project": true}); err == nil {
		t.Fatal("absolute path in a JSON field name passed persistence validation")
	}
	large := make(map[string]any, 3)
	for group := 0; group < 3; group++ {
		nested := make(map[string]any, 256)
		for index := 0; index < 256; index++ {
			key := fmt.Sprintf("field-%03d-%s", index, string(bytes.Repeat([]byte{'x'}, 112)))
			nested[key] = true
		}
		large[fmt.Sprintf("group-%d", group)] = nested
	}
	if err := validatePersistedJSON(large); err == nil {
		t.Fatal("cumulative JSON field names exceeded the string budget without rejection")
	}
}

func TestCommitRunBindsStateSnapshotToPinnedRoot(t *testing.T) {
	requireInitializePlatform(t)
	_, rootA, stateA := createRunStoreProject(t, "project-a")
	defer rootA.Close()
	stateA.ProjectID = "project-ffffffffffffffffffffffffffffffff"
	projectB, rootB, _ := createRunStoreProject(t, "project-b")
	defer rootB.Close()
	result, payload := sampleRunTransaction(t)
	committed := finishRunForTest(rootB, stateA, result, []runPayload{payload}, nil)
	if committed.Err == nil || committed.Committed {
		t.Fatalf("state from another root was accepted: %+v", committed)
	}
	if _, err := os.Lstat(filepath.Join(projectB, ".gameatelier", "runs")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("mismatched state/root wrote a runs directory: %v", err)
	}
}

func TestFinishRejectsResultThatDiffersFromPublishedIntent(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, state := createRunStoreProject(t, "immutable-intent")
	defer stateRoot.Close()
	result, payload := sampleRunTransaction(t)
	transaction, err := beginRun(stateRoot, state, result, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.close()
	result.Command.Arguments = map[string]any{"project": "different"}
	committed := transaction.finish(result, []runPayload{payload})
	if committed.Err == nil || committed.Committed {
		t.Fatalf("finish accepted a result that differs from intent: %+v", committed)
	}
	if _, err := os.Lstat(filepath.Join(project, ".gameatelier", "runs", transaction.runID, "result.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("intent mismatch exposed result: %v", err)
	}
}

func TestRunTransactionFreezesProducerVersionAtBegin(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, state := createRunStoreProject(t, "frozen-producer-version")
	defer stateRoot.Close()
	result, payload := sampleRunTransaction(t)
	originalVersion := Version
	transaction, err := beginRun(stateRoot, state, result, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.close()
	Version = "invalid version after begin"
	defer func() { Version = originalVersion }()
	committed := transaction.finish(result, []runPayload{payload})
	if committed.Err != nil || !committed.Committed {
		t.Fatalf("finish did not use the producer version frozen by intent: %+v", committed)
	}
	recordBytes, err := os.ReadFile(filepath.Join(project, ".gameatelier", "runs", result.RunID, "evidence", "0001-validation-report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record persistedEvidenceRecord
	if err := json.Unmarshal(recordBytes, &record); err != nil || record.Producer.Version != originalVersion {
		t.Fatalf("producer version drifted after begin: err=%v record=%+v", err, record)
	}
}

func TestCommitRunUnverifiedHostWritesNothing(t *testing.T) {
	if initializePlatformReady() {
		t.Skip("this host has an enabled native transaction implementation")
	}
	project, stateRoot, state := createRunStoreProject(t, "unverified-run-host")
	defer stateRoot.Close()
	result, payload := sampleRunTransaction(t)
	committed := finishRunForTest(stateRoot, state, result, []runPayload{payload}, nil)
	if committed.Err == nil || committed.Committed {
		t.Fatalf("unverified host accepted run persistence: %+v", committed)
	}
	if _, err := os.Lstat(filepath.Join(project, ".gameatelier", "runs")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unverified host wrote a runs directory: %v", err)
	}
}

func TestRunStoreRejectsFilesystemIdentityChangesAtNestedDirectories(t *testing.T) {
	requireInitializePlatform(t)
	for _, test := range []struct {
		name       string
		mismatchAt int
		wantPhase  runBeginPhase
		beginOK    bool
	}{
		{name: "runs mount", mismatchAt: 2, wantPhase: runBeginBeforeRoot},
		{name: "run mount", mismatchAt: 3, wantPhase: runBeginOrphan},
		{name: "payload mount", mismatchAt: 4, beginOK: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			project, stateRoot, state := createRunStoreProject(t, "identity-"+test.name)
			defer stateRoot.Close()
			result, payload := sampleRunTransaction(t)
			calls := 0
			reader := func(_ *os.Root) (runPersistenceIdentity, bool) {
				calls++
				if calls == test.mismatchAt {
					return runPersistenceIdentity{fsid: [2]int64{9, 9}}, true
				}
				return runPersistenceIdentity{fsid: [2]int64{1, 2}}, true
			}
			transaction, err := beginRunWithIdentity(stateRoot, state, result, nil, reader)
			if !test.beginOK {
				var failure *runBeginError
				if transaction != nil || !errors.As(err, &failure) || failure.phase != test.wantPhase {
					t.Fatalf("nested filesystem mismatch was misclassified: transaction=%v err=%v", transaction, err)
				}
			} else {
				if err != nil || transaction == nil {
					t.Fatalf("begin failed before injected payload mismatch: transaction=%v err=%v", transaction, err)
				}
				defer transaction.close()
				committed := transaction.finish(result, []runPayload{payload})
				if committed.Err == nil || committed.Committed {
					t.Fatalf("payload filesystem mismatch committed: %+v", committed)
				}
			}
			if _, err := os.Lstat(filepath.Join(project, ".gameatelier", "runs", result.RunID, "result.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("filesystem mismatch exposed result: %v", err)
			}
		})
	}
}

func createRunStoreProject(t *testing.T, name string) (string, *os.Root, projectState) {
	t.Helper()
	project := createProject(t, name)
	writeState(t, project, `{
  "schema_version":"1.0.0","project_id":"project-0123456789abcdef0123456789abcdef","revision":0,"mode":"standard",
  "engine":{"kind":"godot","requested_version":"4.7.2-stable","language":"gdscript"},
  "task_refs":[],"active_run_refs":[],"updated_at":"2026-08-25T00:00:00Z"
}`)
	root, exists, err := openExistingStateRoot(project)
	if err != nil || !exists {
		t.Fatalf("openExistingStateRoot()=(%v,%t,%v)", root, exists, err)
	}
	state, exists, _, err := loadExistingState(root)
	if err != nil || !exists {
		root.Close()
		t.Fatalf("loadExistingState()=(%+v,%t,%v)", state, exists, err)
	}
	return project, root, state
}

func sampleRunTransaction(t *testing.T) (contract.Result, runPayload) {
	t.Helper()
	started := time.Now().UTC()
	result := contract.NewResult(started, contract.Command{Name: "validate", Arguments: map[string]any{"project": "."}})
	finishSampleResult(&result, started)
	return result, sampleValidationPayload(t)
}

func finishSampleResult(result *contract.Result, started time.Time) {
	result.Finish(started, started.Add(10*time.Millisecond), "PASS", contract.ExitOK, "Baseline project validation passed.", map[string]any{"scope": "baseline", "check_count": 2})
}

func sampleValidationPayload(t *testing.T) runPayload {
	t.Helper()
	payloadBytes, err := marshalRunJSON(map[string]any{
		"schema_version": contract.SchemaVersion,
		"scope":          "baseline",
		"outcome":        "PASS",
		"checks": []map[string]any{
			{"id": "project-state", "outcome": "PASS", "summary": "Project state is valid."},
			{"id": "project-file", "outcome": "PASS", "summary": "Godot project file is present."},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return runPayload{Kind: "validation-report", Outcome: "PASS", MediaType: "application/json", Content: payloadBytes}
}

func failingValidationPayload(t *testing.T) runPayload {
	t.Helper()
	payloadBytes, err := marshalRunJSON(map[string]any{
		"schema_version": contract.SchemaVersion,
		"scope":          "baseline",
		"outcome":        "FAIL",
		"checks": []map[string]any{
			{"id": "project-state", "outcome": "FAIL", "summary": "Project state is invalid."},
			{"id": "project-file", "outcome": "PASS", "summary": "Godot project file is present."},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return runPayload{Kind: "validation-report", Outcome: "FAIL", MediaType: "application/json", Content: payloadBytes}
}

func finishRunForTest(stateRoot *os.Root, state projectState, result contract.Result, payloads []runPayload, fault runFault) runCommit {
	transaction, err := beginRun(stateRoot, state, result, fault)
	if err != nil {
		return runCommit{Err: err}
	}
	defer transaction.close()
	return transaction.finish(result, payloads)
}

func assertStoredRunClosure(t *testing.T, project, runID string) {
	t.Helper()
	runRoot := filepath.Join(project, ".gameatelier", "runs", runID)
	resultBytes, err := os.ReadFile(filepath.Join(runRoot, "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result contract.Result
	if err := json.Unmarshal(resultBytes, &result); err != nil || result.RunID != runID || len(result.Evidence) != 1 {
		t.Fatalf("invalid stored result closure: err=%v result=%+v", err, result)
	}
	evidencePrefix := ".gameatelier/runs/" + runID + "/evidence/"
	evidenceName := strings.TrimPrefix(result.Evidence[0].Path, evidencePrefix)
	if evidenceName == result.Evidence[0].Path || evidenceName == "" || filepath.Base(evidenceName) != evidenceName {
		t.Fatalf("stored result has an unsafe evidence reference: %+v", result.Evidence[0])
	}
	recordBytes, err := os.ReadFile(filepath.Join(runRoot, "evidence", evidenceName))
	if err != nil {
		t.Fatal(err)
	}
	var record persistedEvidenceRecord
	if err := json.Unmarshal(recordBytes, &record); err != nil {
		t.Fatal(err)
	}
	payloadPrefix := ".gameatelier/runs/" + runID + "/payloads/"
	payloadName := strings.TrimPrefix(record.Path, payloadPrefix)
	if payloadName == record.Path || payloadName == "" || filepath.Base(payloadName) != payloadName {
		t.Fatalf("stored evidence has an unsafe payload reference: %+v", record)
	}
	payloadBytes, err := os.ReadFile(filepath.Join(runRoot, "payloads", payloadName))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payloadBytes)
	if result.Evidence[0].ID != record.ID || record.RunID != runID || record.SHA256 != hex.EncodeToString(sum[:]) || record.ByteSize != int64(len(payloadBytes)) {
		t.Fatalf("stored run closure is inconsistent: result=%+v record=%+v", result, record)
	}
}

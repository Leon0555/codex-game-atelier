package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

func TestScanRunsClassifiesCommittedIncompleteOrphanAndCorrupt(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, state := createRunStoreProject(t, "扫描 分类 中文")
	defer stateRoot.Close()

	committed, payload := sampleRunTransaction(t)
	if result := finishRunForTest(stateRoot, state, committed, []runPayload{payload}, nil); result.Err != nil || !result.Committed {
		t.Fatalf("committed fixture failed: %+v", result)
	}

	incompleteStarted := time.Now().UTC().Add(time.Second)
	incomplete := contract.NewResult(incompleteStarted, contract.Command{Name: "validate", Arguments: map[string]any{"project": "."}})
	transaction, err := beginRun(stateRoot, state, incomplete, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := transaction.close(); err != nil {
		t.Fatal(err)
	}

	orphanID := "atelier-20260825t111111.000000000z-111111111111"
	if err := os.Mkdir(filepath.Join(project, ".gameatelier", "runs", orphanID), 0o700); err != nil {
		t.Fatal(err)
	}

	corrupt, corruptPayload := sampleRunTransaction(t)
	if result := finishRunForTest(stateRoot, state, corrupt, []runPayload{corruptPayload}, nil); result.Err != nil || !result.Committed {
		t.Fatalf("corrupt fixture setup failed: %+v", result)
	}
	corruptPayloadPath := filepath.Join(project, ".gameatelier", "runs", corrupt.RunID, "payloads", "0001-validation-report.json")
	if err := os.WriteFile(corruptPayloadPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	scan, err := scanRuns(context.Background(), stateRoot, state)
	if err != nil {
		t.Fatal(err)
	}
	if scan.Counts != (cleanRunCounts{Committed: 1, Incomplete: 1, Orphan: 1, Corrupt: 1}) {
		t.Fatalf("unexpected counts: %+v", scan.Counts)
	}
	states := map[string]string{}
	for _, item := range scan.Candidates {
		states[item.RunID] = item.State + ":" + item.Reason
	}
	if states[incomplete.RunID] != "incomplete:RESULT_MISSING" || states[orphanID] != "orphan:INTENT_AND_RESULT_MISSING" {
		t.Fatalf("unexpected cleanup candidates: %+v", scan.Candidates)
	}
	if len(scan.Protected) != 1 || scan.Protected[0].RunID != corrupt.RunID || scan.Protected[0].State != "corrupt" {
		t.Fatalf("corrupt run was not protected: %+v", scan.Protected)
	}
	for _, item := range append(append([]cleanRunEntry{}, scan.Candidates...), scan.Protected...) {
		if item.Path != ".gameatelier/runs/"+item.RunID {
			t.Fatalf("scanner exposed a non-canonical path: %+v", item)
		}
	}
	if _, exists := states[committed.RunID]; exists {
		t.Fatalf("committed run became a cleanup candidate: %+v", scan.Candidates)
	}
}

func TestCleanListIsReadOnlyAndReturnsStableCandidateShape(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, _ := createRunStoreProject(t, "clean list 只读")
	orphanID := "atelier-20260825t121212.000000000z-222222222222"
	runsRoot, err := openOrCreateVerifiedDirectory(stateRoot, "runs", false)
	if err != nil {
		stateRoot.Close()
		t.Fatal(err)
	}
	if err := runsRoot.Close(); err != nil {
		stateRoot.Close()
		t.Fatal(err)
	}
	stateRoot.Close()
	if err := os.Mkdir(filepath.Join(project, ".gameatelier", "runs", orphanID), 0o700); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, filepath.Join(project, ".gameatelier"))
	code, result, _, stderr := execute(t, context.Background(), "clean", "--list", "--project", project)
	after := snapshotTree(t, filepath.Join(project, ".gameatelier"))
	if code != contract.ExitOK || result.Outcome != "PASS" || len(result.Evidence) != 0 || stderr != "" {
		t.Fatalf("clean --list failed: code=%d result=%+v stderr=%q", code, result, stderr)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("clean --list modified the state tree:\nbefore=%+v\nafter=%+v", before, after)
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data["scope"] != "runs" || data["scanned"] != true {
		t.Fatalf("unexpected clean data: %#v", result.Data)
	}
	candidates, ok := data["candidates"].([]any)
	if !ok || len(candidates) != 1 {
		t.Fatalf("unexpected candidates: %#v", data["candidates"])
	}
	candidate, ok := candidates[0].(map[string]any)
	if !ok || candidate["run_id"] != orphanID || candidate["path"] != ".gameatelier/runs/"+orphanID || candidate["state"] != "orphan" {
		t.Fatalf("unexpected candidate: %#v", candidates[0])
	}
}

func TestCleanListWithoutRunsDirectoryIsReadOnlyAndEmpty(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, _ := createRunStoreProject(t, "没有 runs 目录")
	stateRoot.Close()
	before := snapshotTree(t, filepath.Join(project, ".gameatelier"))
	code, result, _, stderr := execute(t, context.Background(), "clean", "--list", "--project", project)
	after := snapshotTree(t, filepath.Join(project, ".gameatelier"))
	if code != contract.ExitOK || result.Outcome != "PASS" || stderr != "" || !reflect.DeepEqual(before, after) {
		t.Fatalf("empty clean scan was not a read-only PASS: code=%d result=%+v stderr=%q", code, result, stderr)
	}
	data := result.Data.(map[string]any)
	counts := data["counts"].(map[string]any)
	if data["scanned"] != true || len(data["candidates"].([]any)) != 0 || len(data["protected"].([]any)) != 0 || counts["committed"] != float64(0) || counts["incomplete"] != float64(0) || counts["orphan"] != float64(0) || counts["corrupt"] != float64(0) {
		t.Fatalf("unexpected empty scan data: %#v", data)
	}
}

func TestScanRunsTreatsCommittedFailureAsCommittedAndAcceptsHistoricalRevision(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, state := createRunStoreProject(t, "历史 revision failure")
	defer stateRoot.Close()
	started := time.Now().UTC()
	result := contract.NewResult(started, contract.Command{Name: "validate", Arguments: map[string]any{"project": "."}})
	result.Finish(started, started.Add(10*time.Millisecond), "FAIL", contract.ExitValidation, "Baseline project validation failed.", map[string]any{"scope": "baseline", "check_count": 2}, contract.Error{
		Code:      "VALIDATION_FAILED",
		Category:  "validation",
		Message:   "The baseline validation checks did not all pass.",
		Retryable: false,
	})
	if committed := finishRunForTest(stateRoot, state, result, []runPayload{failingValidationPayload(t)}, nil); committed.Err != nil || !committed.Committed {
		t.Fatalf("failed operation did not commit its evidence: %+v", committed)
	}
	intentBytes, err := os.ReadFile(filepath.Join(project, ".gameatelier", "runs", result.RunID, "intent.json"))
	if err != nil {
		t.Fatal(err)
	}
	var intent runIntentRecord
	if err := decodeStrictRunJSON(intentBytes, &intent); err != nil {
		t.Fatal(err)
	}
	intent.ProjectRevision = 1
	if err := validateScannedIntent(intent, state, result.RunID); err == nil {
		t.Fatal("intent from a future project revision was accepted")
	}
	state.Revision = 1
	scan, err := scanRuns(context.Background(), stateRoot, state)
	if err != nil || scan.Counts != (cleanRunCounts{Committed: 1}) || len(scan.Candidates) != 0 || len(scan.Protected) != 0 {
		t.Fatalf("historical committed FAIL was misclassified: scan=%+v err=%v", scan, err)
	}
}

func TestCleanListRejectsRunCountAboveBound(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, _ := createRunStoreProject(t, "bounded scan")
	runsRoot, err := openOrCreateVerifiedDirectory(stateRoot, "runs", false)
	if err != nil {
		stateRoot.Close()
		t.Fatal(err)
	}
	runsRoot.Close()
	stateRoot.Close()
	for index := 0; index <= maxRunScanEntries; index++ {
		runID := fmt.Sprintf("atelier-20260825t151515.%09dz-%012x", index, index)
		if err := os.Mkdir(filepath.Join(project, ".gameatelier", "runs", runID), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	code, result, _, _ := execute(t, context.Background(), "clean", "--list", "--project", project)
	if code != contract.ExitState || firstErrorCode(result) != "RUN_SCAN_UNSAFE" {
		t.Fatalf("unbounded run store was not rejected: code=%d result=%+v", code, result)
	}
	data := result.Data.(map[string]any)
	if data["scanned"] != false || len(data["candidates"].([]any)) != 0 || len(data["protected"].([]any)) != 0 {
		t.Fatalf("unbounded scan returned partial decisions: %#v", data)
	}
}

func TestCleanListPreCancelledContextReturnsNoPartialDecisions(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, _ := createRunStoreProject(t, "pre-cancelled clean")
	stateRoot.Close()
	before := snapshotTree(t, filepath.Join(project, ".gameatelier"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code, result, _, _ := execute(t, ctx, "clean", "--list", "--project", project)
	after := snapshotTree(t, filepath.Join(project, ".gameatelier"))
	if code != contract.ExitInterrupted || result.Outcome != "FAIL" || firstErrorCode(result) != "COMMAND_CANCELLED" || !reflect.DeepEqual(before, after) {
		t.Fatalf("pre-cancelled clean did not fail read-only: code=%d result=%+v", code, result)
	}
	assertEmptyFailedCleanData(t, result)
}

func TestScanRunsMidScanCancellationReturnsNoPartialDecisions(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, state := createRunStoreProject(t, "mid-scan cancel")
	defer stateRoot.Close()
	runsRoot, err := openOrCreateVerifiedDirectory(stateRoot, "runs", false)
	if err != nil {
		t.Fatal(err)
	}
	runsRoot.Close()
	for _, runID := range []string{
		"atelier-20260825t161616.000000000z-111111111111",
		"atelier-20260825t161617.000000000z-222222222222",
	} {
		if err := os.Mkdir(filepath.Join(project, ".gameatelier", "runs", runID), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	ctx := newSteppedCancelContext(6)
	scan, err := scanRuns(ctx, stateRoot, state)
	if !errors.Is(err, context.Canceled) || !emptyRunScanResult(scan) {
		t.Fatalf("mid-scan cancellation leaked partial decisions: scan=%+v err=%v", scan, err)
	}
}

func TestScanRunsAggregateBudgetReturnsNoPartialDecisions(t *testing.T) {
	requireInitializePlatform(t)
	_, stateRoot, state := createRunStoreProject(t, "scan byte budget")
	defer stateRoot.Close()
	result, payload := sampleRunTransaction(t)
	if committed := finishRunForTest(stateRoot, state, result, []runPayload{payload}, nil); committed.Err != nil || !committed.Committed {
		t.Fatalf("budget fixture did not commit: %+v", committed)
	}
	scan, err := scanRunsWithBudget(context.Background(), stateRoot, state, newRunScanBudget(1, maxRunScanFiles))
	if !errors.Is(err, errRunScanBudgetExceeded) || !emptyRunScanResult(scan) {
		t.Fatalf("budget exhaustion leaked partial decisions: scan=%+v err=%v", scan, err)
	}
}

func TestCleanListRejectsUnsafeRunDirectoryWithoutPartialCandidates(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, _ := createRunStoreProject(t, "unsafe scan")
	runsRoot, err := openOrCreateVerifiedDirectory(stateRoot, "runs", false)
	if err != nil {
		stateRoot.Close()
		t.Fatal(err)
	}
	runsRoot.Close()
	stateRoot.Close()
	validOrphan := "atelier-20260825t131313.000000000z-333333333333"
	if err := os.Mkdir(filepath.Join(project, ".gameatelier", "runs", validOrphan), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(project, ".gameatelier", "runs", "not-a-run"), 0o700); err != nil {
		t.Fatal(err)
	}

	code, result, _, _ := execute(t, context.Background(), "clean", "--list", "--project", project)
	if code != contract.ExitState || result.Outcome != "FAIL" || firstErrorCode(result) != "RUN_SCAN_UNSAFE" {
		t.Fatalf("unsafe scan was not rejected: code=%d result=%+v", code, result)
	}
	data := result.Data.(map[string]any)
	if data["scanned"] != false || len(data["candidates"].([]any)) != 0 || len(data["protected"].([]any)) != 0 {
		t.Fatalf("unsafe scan leaked partial cleanup decisions: %#v", data)
	}
}

func TestCleanListRejectsRunDirectorySymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ordinary Windows test accounts may not create symlinks")
	}
	requireInitializePlatform(t)
	project, stateRoot, _ := createRunStoreProject(t, "run symlink")
	runsRoot, err := openOrCreateVerifiedDirectory(stateRoot, "runs", false)
	if err != nil {
		stateRoot.Close()
		t.Fatal(err)
	}
	runsRoot.Close()
	stateRoot.Close()
	target := t.TempDir()
	runID := "atelier-20260825t141414.000000000z-444444444444"
	if err := os.Symlink(target, filepath.Join(project, ".gameatelier", "runs", runID)); err != nil {
		t.Fatal(err)
	}
	code, result, _, _ := execute(t, context.Background(), "clean", "--list", "--project", project)
	if code != contract.ExitState || firstErrorCode(result) != "RUN_SCAN_UNSAFE" {
		t.Fatalf("run symlink was not rejected: code=%d result=%+v", code, result)
	}
}

func TestCleanRequiresExplicitListMode(t *testing.T) {
	code, result, _, _ := execute(t, context.Background(), "clean", "--project", ".")
	if code != contract.ExitUsage || result.Outcome != "FAIL" || firstErrorCode(result) != "INVALID_ARGUMENT" {
		t.Fatalf("clean without --list was not rejected: code=%d result=%+v", code, result)
	}
}

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case entry.Type()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			snapshot[relative] = "symlink:" + target
		case entry.IsDir():
			snapshot[relative] = "dir:" + info.Mode().String()
		case info.Mode().IsRegular():
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			snapshot[relative] = "file:" + info.Mode().String() + ":" + string(content)
		default:
			snapshot[relative] = "special:" + info.Mode().String()
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(snapshot))
	for key := range snapshot {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := make(map[string]string, len(snapshot))
	for _, key := range keys {
		ordered[key] = snapshot[key]
	}
	return ordered
}

func TestDecodeStrictRunJSONRejectsDuplicateKeysAndTrailingValues(t *testing.T) {
	var result contract.Result
	for _, content := range [][]byte{
		[]byte(`{"schema_version":"1.0.0","schema_version":"1.0.0"}`),
		[]byte(`{} {}`),
		bytes.Repeat([]byte{'x'}, int(maxRunPayloadBytes)+1),
	} {
		if err := decodeStrictRunJSON(content, &result); err == nil {
			t.Fatalf("unsafe JSON was accepted: %.80q", content)
		}
	}
}

func TestReadOptionalRunFileRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ordinary Windows test accounts may not create symlinks")
	}
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "target"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target", filepath.Join(directory, "intent.json")); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	_, exists, err := readOptionalRunFile(context.Background(), newRunScanBudget(maxRunScanTotalBytes, maxRunScanFiles), root, "intent.json", maxRunIntentBytes)
	if !exists || err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatalf("symlink intent was not rejected: exists=%t err=%v", exists, err)
	}
}

func TestReadScanRegularFileCancelsBetweenChunks(t *testing.T) {
	directory := t.TempDir()
	content := bytes.Repeat([]byte{'x'}, runScanReadChunk*3)
	if err := os.WriteFile(filepath.Join(directory, "result.json"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	ctx := newSteppedCancelContext(3)
	budget := newRunScanBudget(maxRunScanTotalBytes, maxRunScanFiles)
	observed, err := readScanRegularFile(ctx, budget, root, "result.json", maxRunPayloadBytes)
	if !errors.Is(err, context.Canceled) || observed != nil || budget.readBytes != runScanReadChunk {
		t.Fatalf("chunk cancellation was not bounded: bytes=%d content=%d err=%v", budget.readBytes, len(observed), err)
	}
}

func TestReadScanRegularFileHonorsExactAggregateByteLimit(t *testing.T) {
	directory := t.TempDir()
	content := bytes.Repeat([]byte{'b'}, runScanReadChunk*2+17)
	if err := os.WriteFile(filepath.Join(directory, "payload.json"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	exact := newRunScanBudget(int64(len(content)), 1)
	observed, err := readScanRegularFile(context.Background(), exact, root, "payload.json", maxRunPayloadBytes)
	if err != nil || !bytes.Equal(observed, content) || exact.readBytes != int64(len(content)) || exact.readFiles != 1 {
		t.Fatalf("exact byte limit failed: bytes=%d files=%d content=%d err=%v", exact.readBytes, exact.readFiles, len(observed), err)
	}

	short := newRunScanBudget(int64(len(content)-1), 1)
	observed, err = readScanRegularFile(context.Background(), short, root, "payload.json", maxRunPayloadBytes)
	if !errors.Is(err, errRunScanBudgetExceeded) || observed != nil || short.readBytes != 0 || short.readFiles != 0 {
		t.Fatalf("limit-minus-one performed content reads: bytes=%d files=%d content=%d err=%v", short.readBytes, short.readFiles, len(observed), err)
	}
}

func assertEmptyFailedCleanData(t *testing.T, result contract.Result) {
	t.Helper()
	data := result.Data.(map[string]any)
	counts := data["counts"].(map[string]any)
	if data["scanned"] != false || len(data["candidates"].([]any)) != 0 || len(data["protected"].([]any)) != 0 || counts["committed"] != float64(0) || counts["incomplete"] != float64(0) || counts["orphan"] != float64(0) || counts["corrupt"] != float64(0) {
		t.Fatalf("failed clean returned partial decisions: %#v", data)
	}
}

func emptyRunScanResult(scan runScanResult) bool {
	return scan.Counts == (cleanRunCounts{}) && len(scan.Candidates) == 0 && len(scan.Protected) == 0
}

type steppedCancelContext struct {
	cancelAfter int
	checks      int
	done        chan struct{}
	closed      bool
}

func newSteppedCancelContext(cancelAfter int) *steppedCancelContext {
	return &steppedCancelContext{cancelAfter: cancelAfter, done: make(chan struct{})}
}

func (ctx *steppedCancelContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (ctx *steppedCancelContext) Done() <-chan struct{}       { return ctx.done }
func (ctx *steppedCancelContext) Value(any) any               { return nil }
func (ctx *steppedCancelContext) Err() error {
	ctx.checks++
	if ctx.checks < ctx.cancelAfter {
		return nil
	}
	if !ctx.closed {
		close(ctx.done)
		ctx.closed = true
	}
	return context.Canceled
}

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

func TestInitializeCreatesStrictStateAndIsIdempotent(t *testing.T) {
	requireInitializePlatform(t)
	project := createProject(t, "初始化 项目 🚀")
	code, result, _, _ := execute(t, context.Background(), "initialize", "--project", project)
	if code != contract.ExitOK || result.Outcome != "PASS" || len(result.Evidence) != 0 {
		t.Fatalf("initialize failed: code=%d result=%+v", code, result)
	}
	data := resultDataMap(t, result)
	if data["created"] != true || data["initialized"] != true {
		t.Fatalf("unexpected initialize data: %+v", data)
	}
	projectID, _ := data["project_id"].(string)
	if !regexp.MustCompile(`^project-[a-f0-9]{32}$`).MatchString(projectID) {
		t.Fatalf("generated project_id=%q", projectID)
	}

	statePath := filepath.Join(project, ".gameatelier", "project.json")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	beforeInfo, err := os.Stat(statePath)
	if err != nil {
		t.Fatal(err)
	}
	state, err := decodeProjectState(before)
	if err != nil || validateProjectState(state) != nil {
		t.Fatalf("published state is invalid: state=%+v decode=%v validate=%v", state, err, validateProjectState(state))
	}
	if state.ProjectID != projectID || state.Revision != 0 || state.Mode != "standard" || state.LastCommandResultRef != "" || len(state.TaskRefs) != 0 || len(state.ActiveRunRefs) != 0 {
		t.Fatalf("unexpected initial state: %+v", state)
	}
	if runtime.GOOS != "windows" && beforeInfo.Mode().Perm() != 0o600 {
		t.Fatalf("project state mode=%o, want 600", beforeInfo.Mode().Perm())
	}

	time.Sleep(20 * time.Millisecond)
	code, result, _, _ = execute(t, context.Background(), "initialize", "--project", project)
	if code != contract.ExitOK || resultDataMap(t, result)["created"] != false || len(result.Evidence) != 0 {
		t.Fatalf("idempotent initialize failed: code=%d result=%+v", code, result)
	}
	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	afterInfo, err := os.Stat(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || !beforeInfo.ModTime().Equal(afterInfo.ModTime()) {
		t.Fatal("idempotent initialize changed project state bytes or mtime")
	}

	code, statusResult, _, _ := execute(t, context.Background(), "status", "--project", project)
	if code != contract.ExitOK || statusResult.Outcome != "PASS" {
		t.Fatalf("status did not accept initialized state: code=%d result=%+v", code, statusResult)
	}
}

func TestInitializeChecksPrerequisitesBeforeWriting(t *testing.T) {
	requireInitializePlatform(t)
	t.Run("missing project", func(t *testing.T) {
		project := filepath.Join(t.TempDir(), "missing")
		if err := os.Mkdir(project, 0o755); err != nil {
			t.Fatal(err)
		}
		code, result, _, _ := execute(t, context.Background(), "initialize", "--project", project)
		if code != contract.ExitPrerequisite || firstErrorCode(result) != "GODOT_PROJECT_NOT_FOUND" {
			t.Fatalf("unexpected missing-project result: code=%d result=%+v", code, result)
		}
		if _, err := os.Lstat(filepath.Join(project, ".gameatelier")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("initialization wrote state before prerequisites passed: %v", err)
		}
	})

	t.Run("dotnet project", func(t *testing.T) {
		project := createProject(t, "dotnet")
		if err := os.WriteFile(filepath.Join(project, "Game.csproj"), []byte("<Project />"), 0o644); err != nil {
			t.Fatal(err)
		}
		code, result, _, _ := execute(t, context.Background(), "initialize", "--project", project)
		if code != contract.ExitPrerequisite || firstErrorCode(result) != "GODOT_DOTNET_UNSUPPORTED" {
			t.Fatalf("unexpected .NET result: code=%d result=%+v", code, result)
		}
		if _, err := os.Lstat(filepath.Join(project, ".gameatelier")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf(".NET rejection wrote state: %v", err)
		}
	})
}

func TestInitializeNeverOverwritesInvalidOrUnsafeState(t *testing.T) {
	requireInitializePlatform(t)
	t.Run("invalid existing state", func(t *testing.T) {
		project := createProject(t, "invalid-state")
		original := []byte(`{"schema_version":"2.0.0","future":true}`)
		writeState(t, project, string(original))
		statePath := filepath.Join(project, ".gameatelier", "project.json")
		code, result, _, _ := execute(t, context.Background(), "initialize", "--project", project)
		if code != contract.ExitState || firstErrorCode(result) != "STATE_CONFLICT" {
			t.Fatalf("unexpected state conflict: code=%d result=%+v", code, result)
		}
		after, err := os.ReadFile(statePath)
		if err != nil || !bytes.Equal(after, original) {
			t.Fatalf("invalid state was modified: err=%v content=%q", err, after)
		}
		if _, err := os.Lstat(filepath.Join(project, ".gameatelier", projectStateLockName)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("invalid existing state caused a lock-file write: %v", err)
		}
	})

	t.Run("state-root symlink", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("native Windows reparse-point coverage remains NOT RUN")
		}
		project := createProject(t, "symlink-state")
		external := t.TempDir()
		sentinel := filepath.Join(external, "sentinel")
		if err := os.WriteFile(sentinel, []byte("preserve"), 0o644); err != nil {
			t.Fatal(err)
		}
		stateRoot := filepath.Join(project, ".gameatelier")
		if err := os.Symlink(external, stateRoot); err != nil {
			t.Fatal(err)
		}
		code, result, _, _ := execute(t, context.Background(), "initialize", "--project", project)
		if code != contract.ExitState || firstErrorCode(result) != "STATE_CONFLICT" {
			t.Fatalf("unexpected symlink result: code=%d result=%+v", code, result)
		}
		content, err := os.ReadFile(sentinel)
		if err != nil || string(content) != "preserve" {
			t.Fatalf("external symlink target changed: err=%v content=%q", err, content)
		}
		info, err := os.Lstat(stateRoot)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("state-root symlink was replaced: info=%v err=%v", info, err)
		}
	})
}

func TestInitializeAcceptsValidPreexistingStateWithoutCreatingLock(t *testing.T) {
	requireInitializePlatform(t)
	project := createProject(t, "preexisting")
	writeState(t, project, `{
  "schema_version":"1.0.0","project_id":"existing-project","revision":7,"mode":"manual",
  "engine":{"kind":"godot","requested_version":"4.8.0-stable","language":"gdscript"},
  "task_refs":[],"active_run_refs":[],"updated_at":"2026-08-25T00:00:00Z"
}`)
	statePath := filepath.Join(project, ".gameatelier", "project.json")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	beforeInfo, err := os.Stat(statePath)
	if err != nil {
		t.Fatal(err)
	}
	code, result, _, _ := execute(t, context.Background(), "initialize", "--project", project)
	if code != contract.ExitOK || resultDataMap(t, result)["created"] != false {
		t.Fatalf("preexisting state was not idempotent: code=%d result=%+v", code, result)
	}
	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	afterInfo, err := os.Stat(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || !beforeInfo.ModTime().Equal(afterInfo.ModTime()) {
		t.Fatal("preexisting state bytes or mtime changed")
	}
	if _, err := os.Lstat(filepath.Join(project, ".gameatelier", projectStateLockName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("idempotent read created a lock file: %v", err)
	}
}

func TestInitializeConcurrentWritersNeverReplaceState(t *testing.T) {
	requireInitializePlatform(t)
	project := createProject(t, "concurrent")
	const workers = 16
	start := make(chan struct{})
	type outcome struct {
		code   int
		result contract.Result
		err    error
	}
	outcomes := make(chan outcome, workers)
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			code, result, err := executeInitializeRaw(project)
			outcomes <- outcome{code: code, result: result, err: err}
		}()
	}
	close(start)
	group.Wait()
	close(outcomes)

	created := 0
	projectIDs := map[string]struct{}{}
	for item := range outcomes {
		if item.err != nil {
			t.Fatal(item.err)
		}
		if item.code == contract.ExitState && firstErrorCode(item.result) == "STATE_LOCKED" {
			continue
		}
		if item.code != contract.ExitOK || item.result.Outcome != "PASS" {
			t.Fatalf("unexpected concurrent result: code=%d result=%+v", item.code, item.result)
		}
		data, ok := item.result.Data.(map[string]any)
		if !ok {
			t.Fatalf("data type=%T", item.result.Data)
		}
		if data["created"] == true {
			created++
		}
		if id, ok := data["project_id"].(string); ok {
			projectIDs[id] = struct{}{}
		}
	}
	if created != 1 || len(projectIDs) != 1 {
		t.Fatalf("concurrent initialization created=%d ids=%v", created, projectIDs)
	}
	content, err := os.ReadFile(filepath.Join(project, ".gameatelier", "project.json"))
	if err != nil {
		t.Fatal(err)
	}
	state, err := decodeProjectState(content)
	if err != nil || validateProjectState(state) != nil {
		t.Fatalf("concurrent state invalid: state=%+v err=%v", state, err)
	}
	entries, err := os.ReadDir(filepath.Join(project, ".gameatelier"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if regexp.MustCompile(`^\.project\.json\.tmp-`).MatchString(entry.Name()) {
			t.Fatalf("successful initialize left temporary file %q", entry.Name())
		}
	}
}

func TestInitializeLockContentionIsRetryableAndRecovers(t *testing.T) {
	requireInitializePlatform(t)
	project := createProject(t, "locked")
	stateRoot, err := openOrCreateStateRoot(project)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := acquireProjectStateLock(stateRoot)
	if err != nil {
		stateRoot.Close()
		t.Fatal(err)
	}
	code, result, _, _ := execute(t, context.Background(), "initialize", "--project", project)
	if code != contract.ExitState || firstErrorCode(result) != "STATE_LOCKED" || !result.Errors[0].Retryable {
		lock.release()
		stateRoot.Close()
		t.Fatalf("unexpected locked result: code=%d result=%+v", code, result)
	}
	lock.release()
	stateRoot.Close()

	code, result, _, _ = execute(t, context.Background(), "initialize", "--project", project)
	if code != contract.ExitOK || resultDataMap(t, result)["created"] != true {
		t.Fatalf("initialize did not recover after lock release: code=%d result=%+v", code, result)
	}
}

func TestPublishProjectStateNeverOverwritesExistingFinal(t *testing.T) {
	requireInitializePlatform(t)
	project := createProject(t, "no-replace")
	stateRoot, err := openOrCreateStateRoot(project)
	if err != nil {
		t.Fatal(err)
	}
	defer stateRoot.Close()
	original := []byte("existing-state")
	if err := writeRootTestFile(stateRoot, "project.json", original); err != nil {
		t.Fatal(err)
	}
	publish := publishProjectState(stateRoot, "atelier-test-no-replace", []byte("replacement"))
	if publish.err == nil || !publish.targetExists {
		t.Fatalf("publish did not report existing target: %+v", publish)
	}
	after, err := os.ReadFile(filepath.Join(stateRoot.Name(), "project.json"))
	if err != nil || !bytes.Equal(after, original) {
		t.Fatalf("existing final was overwritten: err=%v content=%q", err, after)
	}
	if _, err := stateRoot.Lstat(".project.json.tmp-atelier-test-no-replace"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed no-replace publish left its temp: %v", err)
	}
}

func TestInitializeDoesNotDeleteForeignCrashTemp(t *testing.T) {
	requireInitializePlatform(t)
	project := createProject(t, "crash-temp")
	stateRoot, err := openOrCreateStateRoot(project)
	if err != nil {
		t.Fatal(err)
	}
	foreignTemp := ".project.json.tmp-atelier-previous-crash"
	if err := writeRootTestFile(stateRoot, foreignTemp, []byte("partial")); err != nil {
		stateRoot.Close()
		t.Fatal(err)
	}
	stateRoot.Close()

	code, result, _, _ := execute(t, context.Background(), "initialize", "--project", project)
	if code != contract.ExitOK || resultDataMap(t, result)["created"] != true {
		t.Fatalf("initialize did not recover around unrelated crash temp: code=%d result=%+v", code, result)
	}
	content, err := os.ReadFile(filepath.Join(project, ".gameatelier", foreignTemp))
	if err != nil || string(content) != "partial" {
		t.Fatalf("initialize deleted or changed another run's temp: err=%v content=%q", err, content)
	}
}

func TestInitializeUnverifiedHostBlocksBeforeStateWrite(t *testing.T) {
	if initializePlatformReady() {
		t.Skip("this host has an enabled native initialize implementation")
	}
	project := createProject(t, "unverified-host")
	code, result, _, _ := execute(t, context.Background(), "initialize", "--project", project)
	if code != contract.ExitPrerequisite || result.Outcome != "BLOCKED" || firstErrorCode(result) != "INITIALIZE_HOST_NOT_VERIFIED" {
		t.Fatalf("unexpected unverified-host result: code=%d result=%+v", code, result)
	}
	if _, err := os.Lstat(filepath.Join(project, ".gameatelier")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unverified host wrote state before returning its gate: %v", err)
	}
}

func requireInitializePlatform(t *testing.T) {
	t.Helper()
	if !initializePlatformReady() {
		t.Skip("native initialize transaction is not enabled on this host")
	}
}

func writeRootTestFile(root *os.Root, name string, content []byte) error {
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func executeInitializeRaw(project string) (int, contract.Result, error) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"initialize", "--project", project}, &stdout, &stderr)
	var result contract.Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return code, result, errors.New("initialize stdout is not valid result JSON: " + stderr.String())
	}
	return code, result, nil
}

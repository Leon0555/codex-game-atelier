package app

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

func TestHooksPlanInstallStatusAndUninstallAreExplicitAndReversible(t *testing.T) {
	requireUnixShell(t)
	project := createHookProject(t, "atelier's 中文 hook")
	marker := filepath.Join(t.TempDir(), "arguments")
	fakeCLI := createExecutable(t, "atelier cli", "#!/bin/sh\nprintf '%s\\n' \"$@\" > '"+marker+"'\n")
	stubHookExecutable(t, fakeCLI)
	hooksPath := filepath.Join(project, ".git", "hooks")
	before := snapshotTree(t, hooksPath)

	for _, action := range []string{"list", "status", "plan"} {
		code, result, _, stderr := execute(t, context.Background(), "hooks", action, "--project", project)
		if code != contract.ExitOK || result.Outcome != "PASS" || stderr != "" || len(result.Evidence) != 0 {
			t.Fatalf("hooks %s failed: code=%d result=%+v stderr=%q", action, code, result, stderr)
		}
		data := resultDataMap(t, result)
		if data["status"] != "absent" || data["changed"] != false || data["path"] != ".git/hooks/pre-commit" {
			t.Fatalf("unexpected hooks %s data: %#v", action, data)
		}
	}
	if after := snapshotTree(t, hooksPath); !equalSnapshots(before, after) {
		t.Fatal("read-only hook commands changed the Git hooks directory")
	}

	code, installed, _, stderr := execute(t, context.Background(), "hooks", "install", "--project", project)
	if code != contract.ExitOK || installed.Outcome != "PASS" || stderr != "" {
		t.Fatalf("hooks install failed: code=%d result=%+v stderr=%q", code, installed, stderr)
	}
	installedData := resultDataMap(t, installed)
	if installedData["status"] != "installed" || installedData["changed"] != true {
		t.Fatalf("unexpected install data: %#v", installedData)
	}
	hookPath := filepath.Join(hooksPath, hookFileName)
	info, err := os.Stat(hookPath)
	if err != nil || info.Mode()&0o100 == 0 {
		t.Fatalf("installed hook is missing or not executable: info=%v err=%v", info, err)
	}
	command := exec.Command(hookPath)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("installed hook did not execute safely: err=%v output=%s", err, output)
	}
	arguments, err := os.ReadFile(marker)
	canonicalProject, canonicalErr := filepath.EvalSymlinks(project)
	if err != nil || canonicalErr != nil || string(arguments) != "release\ncheck\n--project\n"+filepath.ToSlash(canonicalProject)+"\n--mode\nmanual\n" {
		t.Fatalf("hook arguments were not quoted deterministically: err=%v args=%q", err, arguments)
	}

	code, idempotent, _, _ := execute(t, context.Background(), "hooks", "install", "--project", project)
	if code != contract.ExitOK || resultDataMap(t, idempotent)["changed"] != false {
		t.Fatalf("repeat install was not idempotent: code=%d result=%+v", code, idempotent)
	}
	code, removed, _, _ := execute(t, context.Background(), "hooks", "uninstall", "--project", project)
	if code != contract.ExitOK || resultDataMap(t, removed)["status"] != "absent" || resultDataMap(t, removed)["changed"] != true {
		t.Fatalf("hooks uninstall failed: code=%d result=%+v", code, removed)
	}
	for _, name := range []string{hookFileName, hookManifestName} {
		if _, err := os.Lstat(filepath.Join(hooksPath, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("managed hook member survived uninstall: %s err=%v", name, err)
		}
	}
}

func TestHooksNeverOverwriteOrDeleteUnownedPreCommit(t *testing.T) {
	project := createHookProject(t, "existing-hook")
	stubHookExecutable(t, createExecutable(t, "atelier", "#!/bin/sh\nexit 0\n"))
	hookPath := filepath.Join(project, ".git", "hooks", hookFileName)
	original := []byte("#!/bin/sh\nprintf 'user hook\\n'\n")
	if err := os.WriteFile(hookPath, original, 0o700); err != nil {
		t.Fatal(err)
	}

	for _, action := range []string{"plan", "install", "uninstall"} {
		code, result, _, _ := execute(t, context.Background(), "hooks", action, "--project", project)
		if code != contract.ExitPrerequisite || result.Outcome != "BLOCKED" {
			t.Fatalf("hooks %s did not protect the existing hook: code=%d result=%+v", action, code, result)
		}
		observed, err := os.ReadFile(hookPath)
		if err != nil || string(observed) != string(original) {
			t.Fatalf("hooks %s changed the user hook: err=%v content=%q", action, err, observed)
		}
	}
}

func TestHooksUninstallAcceptsVerifiedOwnedHookFromPriorCLIPath(t *testing.T) {
	project := createHookProject(t, "stale-hook")
	first := createExecutable(t, "atelier-old", "#!/bin/sh\nexit 0\n")
	stubHookExecutable(t, first)
	code, result, _, _ := execute(t, context.Background(), "hooks", "install", "--project", project)
	if code != contract.ExitOK || result.Outcome != "PASS" {
		t.Fatalf("initial install failed: code=%d result=%+v", code, result)
	}
	second := createExecutable(t, "atelier-new", "#!/bin/sh\nexit 0\n")
	stubHookExecutable(t, second)
	code, status, _, _ := execute(t, context.Background(), "hooks", "status", "--project", project)
	if code != contract.ExitOK || resultDataMap(t, status)["status"] != "stale" {
		t.Fatalf("prior installation was not reported stale: code=%d result=%+v", code, status)
	}
	code, removed, _, _ := execute(t, context.Background(), "hooks", "uninstall", "--project", project)
	if code != contract.ExitOK || resultDataMap(t, removed)["status"] != "absent" {
		t.Fatalf("verified stale hook could not be removed: code=%d result=%+v", code, removed)
	}
}

func TestHooksBlockEffectiveCoreHooksPathOverride(t *testing.T) {
	project := createHookProject(t, "custom-hooks-path")
	stubHookExecutable(t, createExecutable(t, "atelier", "#!/bin/sh\nexit 0\n"))
	command := exec.Command("git", "-C", project, "config", "core.hooksPath", "custom-hooks")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git config failed: err=%v output=%s", err, output)
	}
	code, result, _, _ := execute(t, context.Background(), "hooks", "install", "--project", project)
	if code != contract.ExitPrerequisite || firstErrorCode(result) != "GIT_HOOKS_PATH_UNSUPPORTED" {
		t.Fatalf("custom hooks path was not blocked: code=%d result=%+v", code, result)
	}
	if _, err := os.Lstat(filepath.Join(project, ".git", "hooks", hookFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("default hook path was modified despite core.hooksPath: %v", err)
	}
}

func TestHooksBlockLinkedWorktreeGitFileWithoutChangingIt(t *testing.T) {
	project, stateRoot, _ := createRunStoreProject(t, "linked-worktree-layout")
	stateRoot.Close()
	gitFile := filepath.Join(project, ".git")
	original := []byte("gitdir: ../elsewhere\n")
	if err := os.WriteFile(gitFile, original, 0o600); err != nil {
		t.Fatal(err)
	}
	code, result, _, _ := execute(t, context.Background(), "hooks", "install", "--project", project)
	if code != contract.ExitPrerequisite || firstErrorCode(result) != "GIT_LAYOUT_UNSUPPORTED" {
		t.Fatalf("linked worktree layout was not blocked: code=%d result=%+v", code, result)
	}
	observed, err := os.ReadFile(gitFile)
	if err != nil || string(observed) != string(original) {
		t.Fatalf("linked worktree .git file changed: err=%v content=%q", err, observed)
	}
}

func TestHooksRejectInvalidSyntaxAndCancellation(t *testing.T) {
	project := createHookProject(t, "hook-errors")
	stubHookExecutable(t, createExecutable(t, "atelier", "#!/bin/sh\nexit 0\n"))
	code, result, _, _ := execute(t, context.Background(), "hooks", "install", "--unknown")
	if code != contract.ExitUsage || firstErrorCode(result) != "INVALID_ARGUMENT" {
		t.Fatalf("invalid hook syntax was accepted: code=%d result=%+v", code, result)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code, result, _, _ = execute(t, ctx, "hooks", "status", "--project", project)
	if code != contract.ExitInterrupted || firstErrorCode(result) != "COMMAND_CANCELLED" {
		t.Fatalf("cancelled hook inspection was misclassified: code=%d result=%+v", code, result)
	}
}

func createHookProject(t *testing.T, name string) string {
	t.Helper()
	project, stateRoot, _ := createRunStoreProject(t, name)
	stateRoot.Close()
	command := exec.Command("git", "init", "--quiet", project)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: err=%v output=%s", err, output)
	}
	command = exec.Command("git", "-C", project, "config", "--local", "core.hooksPath", "")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git test isolation config failed: err=%v output=%s", err, output)
	}
	return project
}

func stubHookExecutable(t *testing.T, executable string) {
	t.Helper()
	prior := resolveHookExecutable
	resolveHookExecutable = func() (string, error) { return executable, nil }
	t.Cleanup(func() { resolveHookExecutable = prior })
}

func equalSnapshots(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func TestShellSingleQuoteDoesNotExposeMetacharacters(t *testing.T) {
	quoted := shellSingleQuote("a'b $HOME; touch nope")
	if !strings.HasPrefix(quoted, "'") || !strings.HasSuffix(quoted, "'") || !strings.Contains(quoted, "'\\''") {
		t.Fatalf("unsafe shell quote: %q", quoted)
	}
}

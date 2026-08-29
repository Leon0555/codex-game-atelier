package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

const hookOwnerMarker = "# codex-game-atelier-hook:v1"
const hookManifestName = "codex-game-atelier.manifest.json"
const hookFileName = "pre-commit"
const maxHookFileBytes int64 = 64 * 1024

type hookManifest struct {
	SchemaVersion string `json:"schema_version"`
	Owner         string `json:"owner"`
	Hook          string `json:"hook"`
	HookSHA256    string `json:"hook_sha256"`
	CLIVersion    string `json:"cli_version"`
	Check         string `json:"check"`
}

type hooksData struct {
	Scope        string `json:"scope"`
	Action       string `json:"action"`
	Hook         string `json:"hook"`
	Path         string `json:"path"`
	ManifestPath string `json:"manifest_path"`
	Check        string `json:"check"`
	Status       string `json:"status"`
	Changed      bool   `json:"changed"`
}

type hookInspection struct {
	status         string
	removable      bool
	current        bool
	hookExists     bool
	manifestExists bool
}

var resolveHookExecutable = func() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return "", err
	}
	executable, err = filepath.Abs(executable)
	if err != nil || strings.ContainsAny(executable, "\x00\r\n") {
		return "", errors.New("CLI executable path is unsafe for a Git hook")
	}
	info, err := os.Stat(executable)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("CLI executable is not a regular file")
	}
	return executable, nil
}

func runHooks(ctx context.Context, started time.Time, args []string) contract.Result {
	if len(args) == 0 {
		return parseError(started, "hooks", "hooks requires list, plan, status, install, or uninstall", map[string]any{})
	}
	action := args[0]
	if action != "list" && action != "plan" && action != "status" && action != "install" && action != "uninstall" {
		return parseError(started, "hooks", "hooks requires list, plan, status, install, or uninstall", map[string]any{})
	}
	set := newFlagSet("hooks " + action)
	project := set.String("project", ".", "initialized project at a regular Git repository root")
	if err := rejectDuplicateFlags(args[1:]); err != nil {
		return parseError(started, "hooks "+action, err.Error(), map[string]any{})
	}
	if err := set.Parse(args[1:]); err != nil || set.NArg() != 0 || *project == "" {
		return parseError(started, "hooks "+action, "hooks "+action+" accepts --project only", map[string]any{})
	}
	result := contract.NewResult(started, contract.Command{Name: "hooks " + action, Arguments: map[string]any{"project": "."}})
	data := hooksData{
		Scope: "git-hooks", Action: action, Hook: hookFileName,
		Path: ".git/hooks/pre-commit", ManifestPath: ".git/hooks/" + hookManifestName,
		Check: "release-check-manual", Status: "absent",
	}
	if err := ctx.Err(); err != nil {
		return finishHooksCancelled(started, result, data)
	}
	root, err := canonicalProjectRoot(*project)
	if err != nil {
		failure := prerequisiteError("GIT_PROJECT_NOT_FOUND", "The requested project directory cannot be resolved safely.", "Select an initialized project at a regular Git repository root.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Git hook state could not be inspected.", data, failure)
		return result
	}
	if strings.ContainsAny(root, "\x00\r\n") {
		failure := prerequisiteError("GIT_PROJECT_PATH_UNSUPPORTED", "The project path contains characters that cannot be represented safely in the portable hook.", "Use a project path without line breaks.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Git hook state could not be inspected.", data, failure)
		return result
	}
	projectRoot, err := os.OpenRoot(root)
	if err != nil {
		return finishHooksUnsafe(started, result, data, "The project root could not be pinned safely.")
	}
	defer projectRoot.Close()
	stateRoot, stateExists, err := openExistingStateRootFromProjectRoot(projectRoot)
	if err != nil {
		return finishHooksUnsafe(started, result, data, "The project state root is unsafe.")
	}
	if !stateExists {
		failure := prerequisiteError("PROJECT_NOT_INITIALIZED", "The project has no .gameatelier state directory.", "Run initialize before installing the optional hook.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Codex Game Atelier project state is not initialized.", data, failure)
		return result
	}
	state, initialized, _, stateErr := loadExistingState(stateRoot)
	stateRoot.Close()
	if stateErr != nil || !initialized || validateProjectState(state) != nil {
		return finishHooksUnsafe(started, result, data, "The project state is invalid or unsafe.")
	}
	gitInfo, gitInfoErr := projectRoot.Lstat(".git")
	if errors.Is(gitInfoErr, os.ErrNotExist) {
		failure := prerequisiteError("GIT_REPOSITORY_REQUIRED", "The project root is not a regular Git repository root.", "Initialize Git at the project root; linked worktree .git files are not supported by the v1 hook helper.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "The optional Git hook requires a regular repository root.", data, failure)
		return result
	}
	if gitInfoErr != nil || gitInfo.Mode()&os.ModeSymlink != 0 {
		return finishHooksUnsafe(started, result, data, "The .git entry is unsafe.")
	}
	if !gitInfo.IsDir() {
		failure := prerequisiteError("GIT_LAYOUT_UNSUPPORTED", "The project uses a .git file or another unsupported repository layout.", "Use hooks manually for linked worktrees; the v1 helper manages only a real project-root .git directory.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "The optional Git hook helper does not support this repository layout.", data, failure)
		return result
	}
	gitRoot, gitExists, err := openExistingVerifiedDirectory(projectRoot, ".git")
	if err != nil || !gitExists {
		return finishHooksUnsafe(started, result, data, "The .git directory changed while it was being pinned.")
	}
	defer gitRoot.Close()
	custom, gitFailure := hasCustomGitHooksPath(ctx, root)
	if gitFailure != nil {
		failure := prerequisiteError("GIT_CONFIGURATION_UNAVAILABLE", "Git configuration could not be checked deterministically.", "Ensure Git is installed and the repository configuration is readable.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "The effective Git hooks path could not be verified.", data, failure)
		return result
	}
	if custom {
		failure := prerequisiteError("GIT_HOOKS_PATH_UNSUPPORTED", "An effective core.hooksPath override is active.", "Remove the override or install the documented hook manually in the configured hooks directory.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "The v1 hook helper only manages the repository default hooks directory.", data, failure)
		return result
	}
	hooksRoot, hooksExists, err := openExistingVerifiedDirectory(gitRoot, "hooks")
	if err != nil {
		return finishHooksUnsafe(started, result, data, "The Git hooks directory is unsafe.")
	}
	if !hooksExists {
		if action != "install" {
			return finishHooksObservation(started, result, data, hookInspection{status: "absent"})
		}
		hooksRoot, err = openOrCreateVerifiedDirectory(gitRoot, "hooks", false)
		if err != nil {
			return finishHooksUnsafe(started, result, data, "The Git hooks directory could not be created safely.")
		}
	}
	defer hooksRoot.Close()
	executable, err := resolveHookExecutable()
	if err != nil {
		failure := prerequisiteError("HOOK_CLI_UNAVAILABLE", "The running CLI executable cannot be pinned for the hook.", "Run the command from an installed Codex Game Atelier CLI binary.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "The optional hook could not bind to this CLI installation.", data, failure)
		return result
	}
	hookContent := renderPreCommitHook(executable, root)
	inspection, err := inspectManagedHook(hooksRoot, hookContent)
	if err != nil {
		return finishHooksUnsafe(started, result, data, "Existing Git hook state is unsafe or exceeds its bounds.")
	}
	data.Status = inspection.status
	switch action {
	case "list", "status":
		return finishHooksObservation(started, result, data, inspection)
	case "plan":
		if inspection.status == "conflict" || inspection.status == "stale" {
			failure := prerequisiteError("GIT_HOOK_CONFLICT", "The pre-commit hook path is already occupied or belongs to a different CLI installation.", "Run hooks uninstall only for a verified managed hook, or resolve the existing hook manually without overwriting it.")
			result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "The optional hook cannot be installed without resolving a conflict.", data, failure)
			return result
		}
		result.Finish(started, time.Now().UTC(), "PASS", contract.ExitOK, "The optional hook installation plan is safe and read-only.", data)
		return result
	case "install":
		if inspection.current {
			result.Finish(started, time.Now().UTC(), "PASS", contract.ExitOK, "The optional hook is already installed by this CLI.", data)
			return result
		}
		if inspection.hookExists || inspection.manifestExists {
			failure := prerequisiteError("GIT_HOOK_CONFLICT", "The pre-commit hook path or ownership manifest already exists.", "Resolve the existing files manually; this command never overwrites them.")
			result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "The optional hook was not installed because existing Git state is protected.", data, failure)
			return result
		}
		manifestBytes, err := managedHookManifest(hookContent)
		if err != nil || installManagedHook(hooksRoot, hookContent, manifestBytes) != nil {
			return finishHooksUnsafe(started, result, data, "The optional hook could not be installed atomically.")
		}
		data.Status, data.Changed = "installed", true
		result.Finish(started, time.Now().UTC(), "PASS", contract.ExitOK, "The optional pre-commit hook was installed explicitly.", data)
		return result
	case "uninstall":
		if inspection.status == "absent" {
			result.Finish(started, time.Now().UTC(), "PASS", contract.ExitOK, "No managed optional hook is installed.", data)
			return result
		}
		if !inspection.removable {
			failure := prerequisiteError("GIT_HOOK_NOT_OWNED", "The existing hook cannot be proven to match its Codex Game Atelier ownership manifest.", "Inspect and remove the conflicting hook manually; this command will not delete it.")
			result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "The optional hook was preserved because ownership could not be verified.", data, failure)
			return result
		}
		if err := uninstallManagedHook(hooksRoot); err != nil {
			return finishHooksUnsafe(started, result, data, "The managed optional hook could not be removed safely.")
		}
		data.Status, data.Changed = "absent", true
		result.Finish(started, time.Now().UTC(), "PASS", contract.ExitOK, "The managed optional pre-commit hook was removed.", data)
		return result
	}
	panic("unreachable hook action")
}

func renderPreCommitHook(executable, project string) []byte {
	return []byte("#!/bin/sh\n" + hookOwnerMarker + "\n# Managed by Codex Game Atelier; remove with hooks uninstall.\nexec " + shellSingleQuote(filepath.ToSlash(executable)) + " release check --project " + shellSingleQuote(filepath.ToSlash(project)) + " --mode manual\n")
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func managedHookManifest(hook []byte) ([]byte, error) {
	digest := sha256.Sum256(hook)
	return marshalRunJSON(hookManifest{SchemaVersion: contract.SchemaVersion, Owner: "codex-game-atelier", Hook: hookFileName, HookSHA256: hex.EncodeToString(digest[:]), CLIVersion: Version, Check: "release-check-manual"})
}

func inspectManagedHook(root *os.Root, expected []byte) (hookInspection, error) {
	hook, hookExists, err := readOptionalBoundedHookFile(root, hookFileName)
	if err != nil {
		return hookInspection{}, err
	}
	manifestBytes, manifestExists, err := readOptionalBoundedHookFile(root, hookManifestName)
	if err != nil {
		return hookInspection{}, err
	}
	inspection := hookInspection{status: "absent", hookExists: hookExists, manifestExists: manifestExists}
	if !hookExists && !manifestExists {
		return inspection, nil
	}
	if !hookExists || !manifestExists {
		inspection.status = "conflict"
		return inspection, nil
	}
	var manifest hookManifest
	decoder := json.NewDecoder(bytes.NewReader(manifestBytes))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&manifest) != nil || decoder.Decode(&struct{}{}) != io.EOF || manifest.SchemaVersion != contract.SchemaVersion || manifest.Owner != "codex-game-atelier" || manifest.Hook != hookFileName || manifest.Check != "release-check-manual" || !producerVersionPattern.MatchString(manifest.CLIVersion) {
		inspection.status = "conflict"
		return inspection, nil
	}
	digest := sha256.Sum256(hook)
	if manifest.HookSHA256 != hex.EncodeToString(digest[:]) || !bytes.Contains(hook, []byte(hookOwnerMarker)) {
		inspection.status = "conflict"
		return inspection, nil
	}
	inspection.removable = true
	if bytes.Equal(hook, expected) && manifest.CLIVersion == Version {
		inspection.status, inspection.current = "installed", true
	} else {
		inspection.status = "stale"
	}
	return inspection, nil
}

func readOptionalBoundedHookFile(root *os.Root, name string) ([]byte, bool, error) {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maxHookFileBytes {
		return nil, true, errors.New("hook member is not a bounded regular file")
	}
	content, err := readBoundedRegularFile(root, name, maxHookFileBytes)
	return content, true, err
}

func installManagedHook(root *os.Root, hook, manifest []byte) error {
	// Publish ownership first so a crash cannot leave an executable hook without
	// the manifest required for inspection and removal. A manifest-only partial
	// state is inert and is conservatively reported as a conflict.
	if err := publishHookFile(root, ".manifest.atelier.tmp", hookManifestName, manifest, 0o600); err != nil {
		return err
	}
	if err := publishHookFile(root, ".pre-commit.atelier.tmp", hookFileName, hook, 0o700); err != nil {
		_ = root.Remove(hookManifestName)
		_ = syncStateDirectory(root)
		return err
	}
	return nil
}

func publishHookFile(root *os.Root, temporary, final string, content []byte, mode os.FileMode) error {
	published := publishImmutableFile(root, temporary, final, content)
	if err := firstRunStoreError(published); err != nil {
		return err
	}
	if mode&0o100 != 0 {
		file, err := root.Open(final)
		if err != nil {
			_ = root.Remove(final)
			_ = syncStateDirectory(root)
			return err
		}
		opened, statErr := file.Stat()
		chmodErr := file.Chmod(mode)
		updated, updatedErr := file.Stat()
		_, seekErr := file.Seek(0, io.SeekStart)
		observed, readErr := io.ReadAll(io.LimitReader(file, maxHookFileBytes+1))
		closeErr := file.Close()
		current, currentErr := root.Lstat(final)
		if statErr != nil || chmodErr != nil || updatedErr != nil || seekErr != nil || readErr != nil || closeErr != nil || currentErr != nil || !opened.Mode().IsRegular() || !updated.Mode().IsRegular() || updated.Mode()&0o100 == 0 || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(opened, current) || !bytes.Equal(observed, content) {
			_ = root.Remove(final)
			_ = syncStateDirectory(root)
			return errors.New("published hook did not retain its executable owned content")
		}
		return syncStateDirectory(root)
	}
	return nil
}

func uninstallManagedHook(root *os.Root) error {
	if err := root.Remove(hookFileName); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := root.Remove(hookManifestName); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return syncStateDirectory(root)
}

func hasCustomGitHooksPath(ctx context.Context, project string) (bool, error) {
	execution := runManagedProcess(ctx, 2*time.Second, "git", project, "config", "--get", "core.hooksPath")
	if execution.StdoutTruncated || execution.StderrTruncated || execution.Cancelled || execution.TimedOut {
		return false, errors.New("git configuration inspection did not complete")
	}
	if execution.ExitCode == nil {
		return false, execution.Err
	}
	switch *execution.ExitCode {
	case 0:
		return strings.TrimSpace(string(execution.Stdout)) != "", nil
	case 1:
		return false, nil
	default:
		return false, fmt.Errorf("git config exited with %d", *execution.ExitCode)
	}
}

func finishHooksObservation(started time.Time, result contract.Result, data hooksData, inspection hookInspection) contract.Result {
	data.Status = inspection.status
	result.Finish(started, time.Now().UTC(), "PASS", contract.ExitOK, "Optional Git hook state was inspected without modification.", data)
	return result
}

func finishHooksUnsafe(started time.Time, result contract.Result, data hooksData, message string) contract.Result {
	failure := contract.Error{Code: "GIT_HOOKS_UNSAFE", Category: "state", Message: message, Retryable: false}
	result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Optional Git hook state could not be handled safely.", data, failure)
	return result
}

func finishHooksCancelled(started time.Time, result contract.Result, data hooksData) contract.Result {
	failure := contract.Error{Code: "COMMAND_CANCELLED", Category: "cancelled", Message: "Git hook inspection was cancelled.", Retryable: true}
	result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitInterrupted, "Optional Git hook handling was cancelled.", data, failure)
	return result
}

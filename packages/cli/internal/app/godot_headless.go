package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"
)

const maxHeadlessOutputBytes = 256 * 1024

type godotHeadlessFailure string

const (
	headlessFailureNone                godotHeadlessFailure = ""
	headlessFailureCancelled           godotHeadlessFailure = "cancelled"
	headlessFailureTimeout             godotHeadlessFailure = "timeout"
	headlessFailureProcess             godotHeadlessFailure = "process-failed"
	headlessFailureEngineErrors        godotHeadlessFailure = "engine-errors"
	headlessFailureOutputTruncated     godotHeadlessFailure = "output-truncated"
	headlessFailureUnsupportedVersion  godotHeadlessFailure = "unsupported-version"
	headlessFailureSnapshotUnavailable godotHeadlessFailure = "snapshot-unavailable"
	headlessFailureExecutableChanged   godotHeadlessFailure = "executable-changed"
	headlessFailureSnapshotCleanup     godotHeadlessFailure = "snapshot-cleanup"
)

type godotHeadlessExecution struct {
	Version        string
	Failure        godotHeadlessFailure
	FailureStage   string
	VersionProcess processResult
	ActionProcess  processResult
	SceneProcess   processResult
}

type godotFixedActionOptions struct {
	Action    string
	Preset    string
	Output    string
	Templates exportTemplateInspection
}

// runGodotHeadless executes only the fixed Atelier validation sequence. It does
// not accept caller-provided Godot arguments. On the supported Unix hosts, the
// paired runner changes directory through the already-open project descriptor
// and executes a private engine snapshot. Version and scene use distinct
// snapshots from the same open source descriptor, so public-path replacement
// cannot redirect either stage or mutate the other stage's executable.
// The caller remains responsible for authorizing Godot's documented user://
// side effect before invoking this function.
func runGodotHeadless(parent context.Context, timeout time.Duration, runnerSource, source *os.File, runRoot *os.Root, projectDirectory *os.File) godotHeadlessExecution {
	return runGodotFixedAction(parent, timeout, runnerSource, source, runRoot, projectDirectory, "scene")
}

// runGodotFixedAction executes the fixed version check followed by one
// allow-listed headless action. The private runner, rather than a public argv,
// owns the exact engine arguments for each action.
func runGodotFixedAction(parent context.Context, timeout time.Duration, runnerSource, source *os.File, runRoot *os.Root, projectDirectory *os.File, action string) godotHeadlessExecution {
	return runGodotFixedActionWithOptions(parent, timeout, runnerSource, source, runRoot, projectDirectory, godotFixedActionOptions{Action: action})
}

func runGodotExportAction(parent context.Context, timeout time.Duration, runnerSource, source *os.File, runRoot *os.Root, projectDirectory *os.File, profile, preset, output string, templates exportTemplateInspection) godotHeadlessExecution {
	return runGodotFixedActionWithOptions(parent, timeout, runnerSource, source, runRoot, projectDirectory, godotFixedActionOptions{
		Action:    "export-" + profile,
		Preset:    preset,
		Output:    output,
		Templates: templates,
	})
}

func runGodotFixedActionWithOptions(parent context.Context, timeout time.Duration, runnerSource, source *os.File, runRoot *os.Root, projectDirectory *os.File, options godotFixedActionOptions) godotHeadlessExecution {
	operation, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	action := options.Action
	if action != "scene" && action != "test" && action != "export-debug" && action != "export-release" {
		return godotHeadlessExecution{Failure: headlessFailureProcess, FailureStage: action}
	}
	if (action == "export-debug" || action == "export-release") && (options.Preset != defaultMacOSExportPreset || !exportOutputPattern.MatchString(options.Output) || options.Templates.Root == "" || len(options.Templates.Files) == 0) {
		return godotHeadlessExecution{Failure: headlessFailureProcess, FailureStage: action}
	}
	execution := godotHeadlessExecution{FailureStage: "version"}
	sourceDigest, err := digestGodotExecutable(operation, source)
	if err != nil {
		execution.Failure = classifySnapshotCreationFailure(operation, err)
		return execution
	}
	runnerSourceDigest, err := digestGodotExecutable(operation, runnerSource)
	if err != nil {
		execution.Failure = classifySnapshotCreationFailure(operation, err)
		return execution
	}
	versionRunner, err := createPinnedRunnerSnapshot(operation, remainingDuration(operation, timeout), runRoot, runnerSource, ".atelier-version-runner")
	if err != nil {
		execution.Failure = classifySnapshotCreationFailure(operation, err)
		return execution
	}
	versionRunnerDigest, runnerDigestErr := versionRunner.digest(operation)
	runnerSourceDigestAfterSnapshot, runnerSourceVerifyErr := digestGodotExecutable(operation, runnerSource)
	if runnerDigestErr != nil || runnerSourceVerifyErr != nil || runnerSourceDigestAfterSnapshot != runnerSourceDigest {
		if cleanupErr := removeGodotSnapshots(runRoot, versionRunner); cleanupErr != nil {
			execution.Failure = headlessFailureSnapshotCleanup
			return execution
		}
		if operation.Err() != nil {
			execution.Failure = classifyHeadlessContext(operation.Err())
			return execution
		}
		execution.Failure = headlessFailureExecutableChanged
		return execution
	}
	versionSnapshot, err := createGodotEngineSnapshot(operation, remainingDuration(operation, timeout), runRoot, source, ".godot-version-snapshot")
	if err != nil {
		if cleanupErr := removeGodotSnapshots(runRoot, versionRunner); cleanupErr != nil {
			execution.Failure = headlessFailureSnapshotCleanup
			return execution
		}
		execution.Failure = classifySnapshotCreationFailure(operation, err)
		return execution
	}
	versionDigest, err := versionSnapshot.digest(operation)
	sourceDigestAfterVersionSnapshot, sourceVerifyErr := digestGodotExecutable(operation, source)
	if err != nil || sourceVerifyErr != nil || sourceDigestAfterVersionSnapshot != sourceDigest {
		if cleanupErr := removeGodotSnapshots(runRoot, versionSnapshot, versionRunner); cleanupErr != nil {
			execution.Failure = headlessFailureSnapshotCleanup
			return execution
		}
		if operation.Err() != nil {
			execution.Failure = classifyHeadlessContext(operation.Err())
			return execution
		}
		execution.Failure = headlessFailureExecutableChanged
		return execution
	}
	execution.VersionProcess = runPinnedGodotStage(operation, remainingDuration(operation, timeout), versionRunner.file, projectDirectory, versionSnapshot.file, "version")
	versionDigestAfter, versionVerifyErr := versionSnapshot.digest(operation)
	versionRunnerDigestAfter, versionRunnerVerifyErr := versionRunner.digest(operation)
	sourceDigestAfterVersion, sourceAfterVersionErr := digestGodotExecutable(operation, source)
	runnerSourceDigestAfterVersion, runnerSourceAfterVersionErr := digestGodotExecutable(operation, runnerSource)
	versionCleanupErr := removeGodotSnapshots(runRoot, versionSnapshot, versionRunner)
	if versionCleanupErr != nil {
		execution.Failure = headlessFailureSnapshotCleanup
		return execution
	}
	if versionVerifyErr != nil || versionRunnerVerifyErr != nil || sourceAfterVersionErr != nil || runnerSourceAfterVersionErr != nil || versionDigestAfter != versionDigest || versionRunnerDigestAfter != versionRunnerDigest || sourceDigestAfterVersion != sourceDigest || runnerSourceDigestAfterVersion != runnerSourceDigest {
		if operation.Err() != nil {
			execution.Failure = classifyHeadlessContext(operation.Err())
			return execution
		}
		execution.Failure = headlessFailureExecutableChanged
		return execution
	}
	if failure := classifyHeadlessResult(execution.VersionProcess); failure != headlessFailureNone {
		execution.Failure = failure
		return execution
	}
	if containsGodotError(execution.VersionProcess.Stderr) {
		execution.Failure = headlessFailureEngineErrors
		return execution
	}
	execution.Version = findGodotVersion(execution.VersionProcess.Stdout, execution.VersionProcess.Stderr)
	if execution.Version == "" {
		execution.Failure = headlessFailureUnsupportedVersion
		return execution
	}
	if err := operation.Err(); err != nil {
		execution.FailureStage = action
		execution.Failure = classifyHeadlessContext(err)
		return execution
	}

	execution.FailureStage = action
	actionRunner, err := createPinnedRunnerSnapshot(operation, remainingDuration(operation, timeout), runRoot, runnerSource, ".atelier-"+action+"-runner")
	if err != nil {
		execution.Failure = classifySnapshotCreationFailure(operation, err)
		return execution
	}
	actionRunnerDigest, runnerDigestErr := actionRunner.digest(operation)
	runnerSourceDigestAfterSceneSnapshot, runnerSourceVerifyErr := digestGodotExecutable(operation, runnerSource)
	if runnerDigestErr != nil || runnerSourceVerifyErr != nil || actionRunnerDigest != versionRunnerDigest || runnerSourceDigestAfterSceneSnapshot != runnerSourceDigest {
		if cleanupErr := removeGodotSnapshots(runRoot, actionRunner); cleanupErr != nil {
			execution.Failure = headlessFailureSnapshotCleanup
			return execution
		}
		if operation.Err() != nil {
			execution.Failure = classifyHeadlessContext(operation.Err())
			return execution
		}
		execution.Failure = headlessFailureExecutableChanged
		return execution
	}
	var exportRuntime *godotExportRuntime
	var actionSnapshot *godotEngineSnapshot
	if action == "export-debug" || action == "export-release" {
		exportRuntime, err = createGodotExportRuntime(operation, remainingDuration(operation, timeout), runRoot, source, options.Templates)
		if exportRuntime != nil {
			actionSnapshot = exportRuntime.snapshot
		}
	} else {
		actionSnapshot, err = createGodotEngineSnapshot(operation, remainingDuration(operation, timeout), runRoot, source, ".godot-"+action+"-snapshot")
	}
	if err != nil {
		if cleanupErr := removeGodotSnapshots(runRoot, actionRunner); cleanupErr != nil {
			execution.Failure = headlessFailureSnapshotCleanup
			return execution
		}
		execution.Failure = classifySnapshotCreationFailure(operation, err)
		return execution
	}
	actionDigest, err := actionSnapshot.digest(operation)
	sourceDigestAfterSceneSnapshot, sourceVerifyErr := digestGodotExecutable(operation, source)
	if err != nil || sourceVerifyErr != nil || sourceDigestAfterSceneSnapshot != sourceDigest || actionDigest != versionDigest {
		cleanupErr := error(nil)
		if exportRuntime != nil {
			cleanupErr = exportRuntime.remove(runRoot)
		} else {
			cleanupErr = removeGodotSnapshots(runRoot, actionSnapshot)
		}
		if runnerCleanupErr := removeGodotSnapshots(runRoot, actionRunner); cleanupErr == nil {
			cleanupErr = runnerCleanupErr
		}
		if cleanupErr != nil {
			execution.Failure = headlessFailureSnapshotCleanup
			return execution
		}
		if operation.Err() != nil {
			execution.Failure = classifyHeadlessContext(operation.Err())
			return execution
		}
		execution.Failure = headlessFailureExecutableChanged
		return execution
	}
	execution.ActionProcess = runPinnedGodotStageWithControl(operation, remainingDuration(operation, timeout), actionRunner.file, projectDirectory, actionSnapshot.file, pinnedRunnerControl{Stage: action, Preset: options.Preset, Output: options.Output})
	if action == "scene" {
		execution.SceneProcess = execution.ActionProcess
	}
	actionDigestAfter, actionVerifyErr := actionSnapshot.digest(operation)
	exportRuntimeVerifyErr := error(nil)
	if exportRuntime != nil {
		exportRuntimeVerifyErr = exportRuntime.verify(operation)
	}
	actionRunnerDigestAfter, actionRunnerVerifyErr := actionRunner.digest(operation)
	sourceDigestAfterScene, sourceAfterSceneErr := digestGodotExecutable(operation, source)
	runnerSourceDigestAfterScene, runnerSourceAfterSceneErr := digestGodotExecutable(operation, runnerSource)
	actionCleanupErr := error(nil)
	if exportRuntime != nil {
		actionCleanupErr = exportRuntime.remove(runRoot)
	} else {
		actionCleanupErr = removeGodotSnapshots(runRoot, actionSnapshot)
	}
	if runnerCleanupErr := removeGodotSnapshots(runRoot, actionRunner); actionCleanupErr == nil {
		actionCleanupErr = runnerCleanupErr
	}
	if actionCleanupErr != nil {
		execution.Failure = headlessFailureSnapshotCleanup
		return execution
	}
	if actionVerifyErr != nil || exportRuntimeVerifyErr != nil || actionRunnerVerifyErr != nil || sourceAfterSceneErr != nil || runnerSourceAfterSceneErr != nil || actionDigestAfter != actionDigest || actionRunnerDigestAfter != actionRunnerDigest || sourceDigestAfterScene != sourceDigest || runnerSourceDigestAfterScene != runnerSourceDigest {
		if operation.Err() != nil {
			execution.Failure = classifyHeadlessContext(operation.Err())
			return execution
		}
		execution.Failure = headlessFailureExecutableChanged
		return execution
	}
	if failure := classifyHeadlessResult(execution.ActionProcess); failure != headlessFailureNone {
		execution.Failure = failure
		return execution
	}
	if containsGodotError(execution.ActionProcess.Stderr) {
		execution.Failure = headlessFailureEngineErrors
		return execution
	}
	execution.Failure = headlessFailureNone
	execution.FailureStage = ""
	return execution
}

func runPinnedGodotStage(parent context.Context, timeout time.Duration, runnerExecutable, projectDirectory, engineExecutable *os.File, stage string) processResult {
	return runPinnedGodotStageWithControl(parent, timeout, runnerExecutable, projectDirectory, engineExecutable, pinnedRunnerControl{Stage: stage})
}

func runPinnedGodotStageWithControl(parent context.Context, timeout time.Duration, runnerExecutable, projectDirectory, engineExecutable *os.File, control pinnedRunnerControl) processResult {
	runner, err := pinnedExecutablePath(runnerExecutable)
	if err != nil {
		return processResult{Err: err}
	}
	runnerInfo, err := runnerExecutable.Stat()
	pathInfo, pathErr := os.Stat(runner)
	if err != nil || pathErr != nil || !runnerInfo.Mode().IsRegular() || !os.SameFile(runnerInfo, pathInfo) {
		return processResult{Err: errors.New("pinned runner path changed")}
	}
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return processResult{Err: err}
	}
	nonce := hex.EncodeToString(nonceBytes)
	control.Nonce = nonce
	controlBytes, err := json.Marshal(control)
	if err != nil {
		return processResult{Err: err}
	}
	controlReader, controlWriter, err := os.Pipe()
	if err != nil {
		return processResult{Err: err}
	}
	if _, err := controlWriter.Write(controlBytes); err != nil {
		controlReader.Close()
		controlWriter.Close()
		return processResult{Err: err}
	}
	if err := controlWriter.Close(); err != nil {
		controlReader.Close()
		return processResult{Err: err}
	}
	defer controlReader.Close()
	prefix := pinnedGodotHelperEnvironment + "="
	environment := make([]string, 0, len(os.Environ())+1)
	for _, item := range os.Environ() {
		if !strings.HasPrefix(item, prefix) {
			environment = append(environment, item)
		}
	}
	environment = append(environment, prefix+nonce)
	return runManagedProcessWithLimitFilesEnv(
		parent,
		timeout,
		maxHeadlessOutputBytes,
		[]*os.File{projectDirectory, engineExecutable, controlReader},
		environment,
		runner,
		"",
	)
}

func removeGodotSnapshots(runRoot *os.Root, snapshots ...*godotEngineSnapshot) error {
	var first error
	for _, snapshot := range snapshots {
		if snapshot == nil {
			continue
		}
		if err := snapshot.remove(runRoot); first == nil && err != nil {
			first = err
		}
	}
	return first
}

func containsGodotError(output []byte) bool {
	for _, line := range bytes.Split(output, []byte{'\n'}) {
		trimmed := bytes.TrimSpace(line)
		if bytes.HasPrefix(trimmed, []byte("ERROR:")) || bytes.HasPrefix(trimmed, []byte("SCRIPT ERROR:")) || bytes.HasPrefix(trimmed, []byte("USER ERROR:")) {
			return true
		}
	}
	return false
}

func classifyHeadlessResult(result processResult) godotHeadlessFailure {
	if result.Cancelled {
		return headlessFailureCancelled
	}
	if result.TimedOut {
		return headlessFailureTimeout
	}
	if result.StdoutTruncated || result.StderrTruncated {
		return headlessFailureOutputTruncated
	}
	if result.Err != nil {
		return headlessFailureProcess
	}
	return headlessFailureNone
}

func classifyHeadlessContext(err error) godotHeadlessFailure {
	if errors.Is(err, context.Canceled) {
		return headlessFailureCancelled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return headlessFailureTimeout
	}
	return headlessFailureProcess
}

func classifySnapshotCreationFailure(ctx context.Context, failures ...error) godotHeadlessFailure {
	for _, err := range failures {
		var cleanupFailure *godotSnapshotCleanupError
		if errors.As(err, &cleanupFailure) {
			return headlessFailureSnapshotCleanup
		}
	}
	if err := ctx.Err(); err != nil {
		return classifyHeadlessContext(err)
	}
	return headlessFailureSnapshotUnavailable
}

func remainingDuration(ctx context.Context, fallback time.Duration) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return fallback
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return time.Nanosecond
	}
	return remaining
}

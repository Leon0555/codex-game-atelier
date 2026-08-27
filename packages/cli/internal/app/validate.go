package app

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

const committedRunWarning = "RUN_COMMITTED_DURABILITY_UNCONFIRMED: result.json is authoritative, but final store cleanup or durability confirmation failed.\n"
const maxPinnedProjectEntries = 4096
const maxPinnedProjectNameBytes = 512 * 1024
const defaultHeadlessTimeoutMS int64 = 30_000

type encodedExecution struct {
	resultBytes []byte
	exitCode    int
	warning     string
}

type validateOptions struct {
	project             string
	headless            bool
	godot               string
	explicitGodot       bool
	timeoutMS           int64
	allowEngineUserData bool
}

func (options validateOptions) scope() string {
	if options.headless {
		return "headless"
	}
	return "baseline"
}

func (options validateOptions) persistedArguments() map[string]any {
	arguments := map[string]any{"project": "."}
	if !options.headless {
		return arguments
	}
	policy := "not-authorized"
	if options.allowEngineUserData {
		policy = "standard-os-location"
	}
	source := "discovery"
	if options.explicitGodot {
		source = "explicit"
	}
	arguments["headless"] = true
	arguments["timeout_ms"] = options.timeoutMS
	arguments["engine_user_data"] = policy
	arguments["godot_source"] = source
	return arguments
}

func runValidate(ctx context.Context, started time.Time, args []string) encodedExecution {
	return runValidateWithFault(ctx, started, args, nil)
}

func runValidateWithFault(ctx context.Context, started time.Time, args []string, fault runFault) encodedExecution {
	set := newFlagSet("validate")
	project := set.String("project", ".", "Godot project directory")
	headless := set.Bool("headless", false, "run the fixed Godot headless scene validation")
	godot := set.String("godot", "", "Godot executable")
	timeoutMS := set.Int64("timeout-ms", defaultHeadlessTimeoutMS, "headless timeout in milliseconds")
	allowEngineUserData := set.Bool("allow-engine-user-data", false, "allow Godot's documented standard user-data directory")
	if err := rejectDuplicateFlags(args); err != nil {
		return encodeUncommittedResult(parseError(started, "validate", err.Error(), map[string]any{}))
	}
	if err := set.Parse(args); err != nil || set.NArg() != 0 || *project == "" || *timeoutMS < 1 || *timeoutMS > maxTimeoutMS {
		return encodeUncommittedResult(parseError(started, "validate", "validate accepts --project and optional --headless, --godot, --timeout-ms, and --allow-engine-user-data", map[string]any{}))
	}
	explicitGodot := flagWasProvided(args, "godot")
	headlessOnlyFlag := explicitGodot || flagWasProvided(args, "timeout-ms") || flagWasProvided(args, "allow-engine-user-data")
	if explicitGodot && *godot == "" {
		return encodeUncommittedResult(parseError(started, "validate", "--godot requires a non-empty path", map[string]any{}))
	}
	if !*headless && headlessOnlyFlag {
		return encodeUncommittedResult(parseError(started, "validate", "--godot, --timeout-ms, and --allow-engine-user-data require --headless", map[string]any{}))
	}
	options := validateOptions{project: *project, headless: *headless, godot: *godot, explicitGodot: explicitGodot, timeoutMS: *timeoutMS, allowEngineUserData: *allowEngineUserData}
	command := contract.Command{Name: "validate", Arguments: options.persistedArguments()}

	projectRoot, err := canonicalProjectRoot(*project)
	if err != nil {
		failure := prerequisiteError("GODOT_PROJECT_NOT_FOUND", "The requested project directory does not exist or cannot be resolved.", "Select an initialized Godot project directory, then run validate again.")
		return encodeUncommittedResult(finishUncommittedValidate(started, command, options.scope(), "BLOCKED", contract.ExitPrerequisite, "Project validation could not locate the project.", failure))
	}
	pinnedProjectRoot, err := os.OpenRoot(projectRoot)
	if err != nil {
		failure := contract.Error{Code: "STATE_READ_FAILED", Category: "state", Message: "The project directory could not be pinned safely.", Retryable: false}
		return encodeUncommittedResult(finishUncommittedValidate(started, command, options.scope(), "FAIL", contract.ExitState, "Project validation could not pin the project directory.", failure))
	}
	defer pinnedProjectRoot.Close()
	stateRoot, exists, err := openExistingStateRootFromProjectRoot(pinnedProjectRoot)
	if err != nil {
		failure := contract.Error{Code: "STATE_READ_FAILED", Category: "state", Message: "The project state directory could not be opened safely.", Retryable: false}
		return encodeUncommittedResult(finishUncommittedValidate(started, command, options.scope(), "FAIL", contract.ExitState, "Project validation could not read project state.", failure))
	}
	if !exists {
		failure := prerequisiteError("PROJECT_NOT_INITIALIZED", "The project has no .gameatelier state directory.", "Run initialize before validate.")
		return encodeUncommittedResult(finishUncommittedValidate(started, command, options.scope(), "BLOCKED", contract.ExitPrerequisite, "Codex Game Atelier project state is not initialized.", failure))
	}
	defer stateRoot.Close()
	state, stateExists, _, err := loadExistingState(stateRoot)
	if err != nil || !stateExists {
		failure := contract.Error{Code: "STATE_READ_FAILED", Category: "state", Message: "The project state file could not be read safely.", Retryable: false}
		return encodeUncommittedResult(finishUncommittedValidate(started, command, options.scope(), "FAIL", contract.ExitState, "Project validation could not read project state.", failure))
	}

	initial := contract.NewResult(started, command)
	transaction, err := beginRun(stateRoot, state, initial, fault)
	if err != nil {
		return encodeRunBeginFailure(started, initial, err)
	}
	defer transaction.close()
	result, payload := executeValidation(ctx, started, initial, transaction.runRoot, pinnedProjectRoot, projectRoot, options)
	if validationRunMustRemainIncomplete(result, transaction.runRoot) {
		_ = transaction.close()
		return encodeRunCommitFailure(started, initial)
	}
	committed := transaction.finish(result, []runPayload{payload})
	closeErr := transaction.close()
	if !committed.Committed {
		return encodeRunCommitFailure(started, initial)
	}
	execution := encodedExecution{resultBytes: committed.ResultBytes, exitCode: result.ExitCode}
	if committed.Err != nil || closeErr != nil {
		execution.warning = committedRunWarning
	}
	return execution
}

func validationRunMustRemainIncomplete(result contract.Result, runRoot *os.Root) bool {
	return fixedActionRunMustRemainIncomplete(result, runRoot, "scene")
}

func executeValidation(ctx context.Context, started time.Time, result contract.Result, runRoot, projectRoot *os.Root, projectPath string, options validateOptions) (contract.Result, runPayload) {
	scope := options.scope()
	checks := []baselineValidationCheck{
		{ID: "project-state", Outcome: "PASS", Summary: "Project state is valid."},
		{ID: "persistence-platform", Outcome: "PASS", Summary: "Run evidence persistence is enabled on this host and filesystem."},
	}
	if err := ctx.Err(); err != nil {
		return finishCancelledValidation(started, result, scope)
	}

	projectContent, err := readPinnedProjectFile(projectRoot)
	if err != nil {
		checks = append(checks,
			baselineValidationCheck{ID: "project-file", Outcome: "BLOCKED", Summary: "Godot project file is missing, unsafe, or unreadable."},
			baselineValidationCheck{ID: "project-language", Outcome: "SKIPPED", Summary: "Project language validation requires a readable project file."},
		)
		failure := prerequisiteError("GODOT_PROJECT_UNREADABLE", "project.godot must be a bounded regular file inside the pinned project directory.", "Replace symlinks or special files with a regular project.godot at or below 1 MiB.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Baseline project validation is blocked by the project file.", map[string]any{"scope": scope, "check_count": len(checks)}, failure)
		return result, makeValidationPayload(scope, result.Outcome, checks)
	}
	if err := ctx.Err(); err != nil {
		return finishCancelledValidation(started, result, scope)
	}
	checks = append(checks, baselineValidationCheck{ID: "project-file", Outcome: "PASS", Summary: "Godot project file is present."})

	usesDotNet, err := pinnedProjectUsesDotNet(ctx, projectRoot, projectContent)
	if ctx.Err() != nil {
		return finishCancelledValidation(started, result, scope)
	}
	if err != nil {
		checks = append(checks, baselineValidationCheck{ID: "project-language", Outcome: "BLOCKED", Summary: "Project language could not be inspected safely."})
		failure := prerequisiteError("GODOT_PROJECT_UNREADABLE", "project.godot or its directory could not be read within safety bounds.", "Check permissions and keep project.godot at or below 1 MiB.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Baseline project validation could not inspect the project language.", map[string]any{"scope": scope, "check_count": len(checks)}, failure)
		return result, makeValidationPayload(scope, result.Outcome, checks)
	}
	if usesDotNet {
		checks = append(checks, baselineValidationCheck{ID: "project-language", Outcome: "BLOCKED", Summary: "Godot .NET/C# is outside the v1.0 support matrix."})
		failure := prerequisiteError("GODOT_DOTNET_UNSUPPORTED", "This project appears to use Godot .NET/C#, which is outside the v1.0 support matrix.", "Use a standard Godot GDScript project for v1.0.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Baseline project validation only supports GDScript projects.", map[string]any{"scope": scope, "check_count": len(checks)}, failure)
		return result, makeValidationPayload(scope, result.Outcome, checks)
	}
	checks = append(checks, baselineValidationCheck{ID: "project-language", Outcome: "PASS", Summary: "Project is within the standard Godot GDScript scope."})
	if err := ctx.Err(); err != nil {
		return finishCancelledValidation(started, result, scope)
	}
	if !options.headless {
		result.Finish(started, time.Now().UTC(), "PASS", contract.ExitOK, "Baseline project validation passed.", map[string]any{"scope": scope, "check_count": len(checks)})
		return result, makeValidationPayload(scope, result.Outcome, checks)
	}
	return executeHeadlessValidation(ctx, started, result, checks, runRoot, projectRoot, projectPath, options)
}

func finishCancelledValidation(started time.Time, result contract.Result, scope string) (contract.Result, runPayload) {
	checks := []baselineValidationCheck{
		{ID: "project-state", Outcome: "PASS", Summary: "Project state is valid."},
		{ID: "persistence-platform", Outcome: "PASS", Summary: "Run evidence persistence is enabled on this host and filesystem."},
		{ID: "project-file", Outcome: "FAIL", Summary: "Project checks were interrupted before completion."},
		{ID: "project-language", Outcome: "SKIPPED", Summary: "Project language validation did not complete after cancellation."},
	}
	failure := contract.Error{Code: "COMMAND_CANCELLED", Category: "cancelled", Message: "Project validation was cancelled.", Retryable: true}
	result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitInterrupted, "Project validation was cancelled.", map[string]any{"scope": scope, "check_count": len(checks)}, failure)
	return result, makeValidationPayload(scope, result.Outcome, checks)
}

func executeHeadlessValidation(ctx context.Context, started time.Time, result contract.Result, checks []baselineValidationCheck, runRoot, pinnedProjectRoot *os.Root, projectPath string, options validateOptions) (contract.Result, runPayload) {
	const scope = "headless"
	if !options.allowEngineUserData {
		checks = append(checks,
			baselineValidationCheck{ID: "engine-user-data", Outcome: "BLOCKED", Summary: "Godot's documented standard user-data directory was not explicitly authorized."},
			baselineValidationCheck{ID: "godot-version", Outcome: "SKIPPED", Summary: "Godot was not started without engine user-data authorization."},
			baselineValidationCheck{ID: "headless-scene", Outcome: "SKIPPED", Summary: "Headless scene validation was not started."},
		)
		failure := contract.Error{Code: "ENGINE_USER_DATA_NOT_AUTHORIZED", Category: "policy", Message: "Headless Godot may create its standard user:// directory outside the project.", Retryable: true, Remediation: "Review the documented side effect, then rerun with --allow-engine-user-data."}
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Headless validation requires explicit authorization for Godot user data.", map[string]any{"scope": scope, "check_count": len(checks)}, failure)
		return result, makeValidationPayload(scope, result.Outcome, checks)
	}
	checks = append(checks, baselineValidationCheck{ID: "engine-user-data", Outcome: "PASS", Summary: "The caller explicitly authorized Godot's standard OS user-data location for this run."})
	projectDirectory, err := openPinnedProjectDirectory(pinnedProjectRoot, projectPath)
	if err != nil {
		checks = append(checks,
			baselineValidationCheck{ID: "project-identity", Outcome: "FAIL", Summary: "The project path no longer identifies the pinned project."},
			baselineValidationCheck{ID: "godot-version", Outcome: "SKIPPED", Summary: "Godot was not started after the project identity changed."},
			baselineValidationCheck{ID: "headless-scene", Outcome: "SKIPPED", Summary: "Headless scene validation was not started."},
		)
		failure := contract.Error{Code: "PROJECT_CHANGED_DURING_VALIDATION", Category: "state", Message: "The project path changed after validation intent was committed.", Retryable: true, Remediation: "Stop concurrent moves or replacements of the project directory, then retry."}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Headless validation stopped because the project identity changed.", map[string]any{"scope": scope, "check_count": len(checks)}, failure)
		return result, makeValidationPayload(scope, result.Outcome, checks)
	}
	defer projectDirectory.Close()

	discovery, discoveryFailure := discoverGodot(options.godot, options.explicitGodot)
	if discoveryFailure != nil {
		checks = append(checks,
			baselineValidationCheck{ID: "project-identity", Outcome: "PASS", Summary: "The engine execution directory is pinned to the validated project."},
			baselineValidationCheck{ID: "godot-version", Outcome: "BLOCKED", Summary: "A supported Godot executable was not available for version verification."},
			baselineValidationCheck{ID: "headless-scene", Outcome: "SKIPPED", Summary: "Headless scene validation was not started."},
		)
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Headless validation could not locate a runnable Godot executable.", map[string]any{"scope": scope, "check_count": len(checks)}, *discoveryFailure)
		return result, makeValidationPayload(scope, result.Outcome, checks)
	}
	runner, err := discoverPinnedGodotRunner()
	if err != nil {
		checks = append(checks,
			baselineValidationCheck{ID: "project-identity", Outcome: "PASS", Summary: "The engine execution directory is pinned to the validated project."},
			baselineValidationCheck{ID: "godot-version", Outcome: "BLOCKED", Summary: "The private pinned-engine runner is unavailable."},
			baselineValidationCheck{ID: "headless-scene", Outcome: "SKIPPED", Summary: "Headless scene validation was not started."},
		)
		failure := prerequisiteError("GODOT_RUNNER_UNAVAILABLE", "The paired internal runner executable is missing or not runnable.", "Install the complete Codex Game Atelier CLI artifact pair, then retry.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Headless validation could not start its pinned engine runner.", map[string]any{"scope": scope, "check_count": len(checks)}, failure)
		return result, makeValidationPayload(scope, result.Outcome, checks)
	}
	runnerSource, err := os.Open(runner)
	if err != nil {
		checks = append(checks,
			baselineValidationCheck{ID: "project-identity", Outcome: "PASS", Summary: "The engine execution directory is pinned to the validated project."},
			baselineValidationCheck{ID: "godot-version", Outcome: "BLOCKED", Summary: "The private runner could not be pinned for snapshotting."},
			baselineValidationCheck{ID: "headless-scene", Outcome: "SKIPPED", Summary: "Headless scene validation was not started."},
		)
		failure := prerequisiteError("GODOT_RUNNER_PIN_UNAVAILABLE", "The paired internal runner could not be opened as a fixed file identity.", "Install the complete Codex Game Atelier CLI artifact pair, then retry.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Headless validation could not pin its private runner.", map[string]any{"scope": scope, "check_count": len(checks)}, failure)
		return result, makeValidationPayload(scope, result.Outcome, checks)
	}
	defer runnerSource.Close()
	openedRunner, runnerStatErr := runnerSource.Stat()
	currentRunner, currentRunnerStatErr := os.Stat(runner)
	if runnerStatErr != nil || currentRunnerStatErr != nil || !openedRunner.Mode().IsRegular() || !os.SameFile(openedRunner, currentRunner) {
		checks = append(checks,
			baselineValidationCheck{ID: "project-identity", Outcome: "PASS", Summary: "The engine execution directory is pinned to the validated project."},
			baselineValidationCheck{ID: "godot-version", Outcome: "BLOCKED", Summary: "The private runner changed while its identity was opened."},
			baselineValidationCheck{ID: "headless-scene", Outcome: "SKIPPED", Summary: "Headless scene validation was not started."},
		)
		failure := prerequisiteError("GODOT_RUNNER_PIN_UNAVAILABLE", "The paired internal runner path changed before snapshot creation.", "Stop concurrent CLI updates, then retry.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Headless validation could not stabilize its private runner.", map[string]any{"scope": scope, "check_count": len(checks)}, failure)
		return result, makeValidationPayload(scope, result.Outcome, checks)
	}
	engineSource, err := os.Open(discovery.Executable)
	if err != nil {
		checks = append(checks,
			baselineValidationCheck{ID: "project-identity", Outcome: "PASS", Summary: "The engine execution directory is pinned to the validated project."},
			baselineValidationCheck{ID: "godot-version", Outcome: "BLOCKED", Summary: "The selected executable could not be pinned for snapshotting."},
			baselineValidationCheck{ID: "headless-scene", Outcome: "SKIPPED", Summary: "Headless scene validation was not started."},
		)
		failure := prerequisiteError("GODOT_EXECUTABLE_PIN_UNAVAILABLE", "The selected executable could not be opened as a fixed file identity.", "Check the Godot executable and project filesystem, then retry.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Headless validation could not pin the selected executable.", map[string]any{"scope": scope, "check_count": len(checks)}, failure)
		return result, makeValidationPayload(scope, result.Outcome, checks)
	}
	defer engineSource.Close()
	openedEngine, engineStatErr := engineSource.Stat()
	currentEngine, currentStatErr := os.Stat(discovery.Executable)
	if engineStatErr != nil || currentStatErr != nil || !openedEngine.Mode().IsRegular() || !os.SameFile(openedEngine, currentEngine) {
		checks = append(checks,
			baselineValidationCheck{ID: "project-identity", Outcome: "PASS", Summary: "The engine execution directory is pinned to the validated project."},
			baselineValidationCheck{ID: "godot-version", Outcome: "BLOCKED", Summary: "The selected executable changed while its identity was opened."},
			baselineValidationCheck{ID: "headless-scene", Outcome: "SKIPPED", Summary: "Headless scene validation was not started."},
		)
		failure := prerequisiteError("GODOT_EXECUTABLE_PIN_UNAVAILABLE", "The selected executable path changed before snapshot creation.", "Stop concurrent engine updates, then retry.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Headless validation could not stabilize the selected executable.", map[string]any{"scope": scope, "check_count": len(checks)}, failure)
		return result, makeValidationPayload(scope, result.Outcome, checks)
	}

	execution := runGodotHeadless(ctx, time.Duration(options.timeoutMS)*time.Millisecond, runnerSource, engineSource, runRoot, projectDirectory)
	if !pinnedProjectPathMatches(pinnedProjectRoot, projectPath) {
		checks = append(checks,
			baselineValidationCheck{ID: "project-identity", Outcome: "FAIL", Summary: "The project path changed while the pinned engine run was active."},
			baselineValidationCheck{ID: "godot-version", Outcome: "SKIPPED", Summary: "Engine observations were discarded after the project identity changed."},
			baselineValidationCheck{ID: "headless-scene", Outcome: "SKIPPED", Summary: "Headless scene observations were discarded after the project identity changed."},
		)
		failure := contract.Error{Code: "PROJECT_CHANGED_DURING_VALIDATION", Category: "state", Message: "The project path changed while Godot headless validation was running.", Retryable: true, Remediation: "Stop concurrent moves or replacements of the project directory, then retry."}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Headless validation discarded engine observations after a project identity change.", map[string]any{"scope": scope, "check_count": len(checks)}, failure)
		return result, makeValidationPayload(scope, result.Outcome, checks)
	}
	checks = append(checks, baselineValidationCheck{ID: "project-identity", Outcome: "PASS", Summary: "The engine execution directory remained pinned to the validated project."})
	if execution.Version != "" {
		checks = append(checks, baselineValidationCheck{ID: "godot-version", Outcome: "PASS", Summary: "The selected executable self-reported the supported Godot 4.7.2-stable official standard build identifier."})
	}
	if execution.Failure == headlessFailureNone {
		checks = append(checks, baselineValidationCheck{ID: "headless-scene", Outcome: "PASS", Summary: "The main scene completed one headless frame with exit 0 and no bounded Godot ERROR output."})
		result.Finish(started, time.Now().UTC(), "PASS", contract.ExitOK, "Godot headless project validation passed.", map[string]any{"scope": scope, "check_count": len(checks)})
		return result, makeValidationPayload(scope, result.Outcome, checks)
	}

	outcome, exitCode, failure := mapHeadlessFailure(execution)
	if execution.FailureStage == "version" {
		checkOutcome := outcome
		checks = append(checks,
			baselineValidationCheck{ID: "godot-version", Outcome: checkOutcome, Summary: "Godot version verification did not complete successfully."},
			baselineValidationCheck{ID: "headless-scene", Outcome: "SKIPPED", Summary: "Headless scene validation was not started."},
		)
	} else {
		checks = append(checks, baselineValidationCheck{ID: "headless-scene", Outcome: outcome, Summary: "Godot headless scene execution did not complete cleanly."})
	}
	result.Finish(started, time.Now().UTC(), outcome, exitCode, "Godot headless project validation did not pass.", map[string]any{"scope": scope, "check_count": len(checks)}, failure)
	return result, makeValidationPayload(scope, result.Outcome, checks)
}

func openPinnedProjectDirectory(root *os.Root, projectPath string) (*os.File, error) {
	if root == nil {
		return nil, errors.New("project root is not pinned")
	}
	expected, err := root.Stat(".")
	if err != nil {
		return nil, err
	}
	directory, err := os.Open(projectPath)
	if err != nil {
		return nil, err
	}
	opened, err := directory.Stat()
	if err != nil || !opened.IsDir() || !os.SameFile(expected, opened) {
		directory.Close()
		return nil, errors.New("project path does not match pinned root")
	}
	return directory, nil
}

func pinnedProjectPathMatches(root *os.Root, projectPath string) bool {
	if root == nil {
		return false
	}
	expected, err := root.Stat(".")
	if err != nil {
		return false
	}
	current, err := os.Stat(projectPath)
	return err == nil && current.IsDir() && os.SameFile(expected, current)
}

func mapHeadlessFailure(execution godotHeadlessExecution) (string, int, contract.Error) {
	details := map[string]any{"stage": execution.FailureStage}
	process := execution.VersionProcess
	if execution.FailureStage == "scene" {
		process = execution.SceneProcess
	}
	if process.ExitCode != nil {
		details["process_exit_code"] = *process.ExitCode
	}
	switch execution.Failure {
	case headlessFailureCancelled:
		return "FAIL", contract.ExitInterrupted, contract.Error{Code: "COMMAND_CANCELLED", Category: "cancelled", Message: "Godot headless validation was cancelled.", Retryable: true, Details: details}
	case headlessFailureTimeout:
		return "FAIL", contract.ExitInterrupted, contract.Error{Code: "GODOT_TIMEOUT", Category: "timeout", Message: "Godot headless validation exceeded its total timeout.", Retryable: true, Details: details}
	case headlessFailureOutputTruncated:
		return "FAIL", contract.ExitEngine, contract.Error{Code: "GODOT_OUTPUT_TRUNCATED", Category: "engine", Message: "Godot output exceeded the bounded capture limit and was not trusted.", Retryable: false, Details: details}
	case headlessFailureUnsupportedVersion:
		return "BLOCKED", contract.ExitPrerequisite, prerequisiteError("GODOT_VERSION_UNSUPPORTED", "The selected executable did not report the supported Godot 4.7.2-stable official standard build identifier.", "Install or select the official standard Godot 4.7.2-stable executable.")
	case headlessFailureSnapshotUnavailable:
		return "BLOCKED", contract.ExitPrerequisite, prerequisiteError("GODOT_EXECUTABLE_SNAPSHOT_UNAVAILABLE", "The paired private runner or selected Godot executable could not be snapshotted into the run evidence filesystem.", "Install the complete CLI artifact pair and place the project and Godot executable on supported filesystems, then retry.")
	case headlessFailureExecutableChanged:
		return "FAIL", contract.ExitEngine, contract.Error{Code: "GODOT_EXECUTABLE_CHANGED", Category: "engine", Message: "A pinned private-runner or Godot executable snapshot changed during validation.", Retryable: true, Details: details}
	case headlessFailureSnapshotCleanup:
		return "FAIL", contract.ExitState, contract.Error{Code: "GODOT_EXECUTABLE_SNAPSHOT_CLEANUP_FAILED", Category: "state", Message: "A transient private-runner or Godot executable snapshot could not be removed safely.", Retryable: true, Details: details}
	case headlessFailureEngineErrors:
		return "FAIL", contract.ExitEngine, contract.Error{Code: "GODOT_REPORTED_ERRORS", Category: "engine", Message: "Godot emitted one or more ERROR records during headless validation.", Retryable: true, Details: details}
	default:
		return "FAIL", contract.ExitEngine, contract.Error{Code: "GODOT_PROCESS_FAILED", Category: "engine", Message: "Godot failed during headless validation.", Retryable: true, Details: details}
	}
}

func readPinnedProjectFile(root *os.Root) ([]byte, error) {
	if root == nil {
		return nil, errors.New("project root is not open")
	}
	info, err := root.Lstat("project.godot")
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, errors.New("project file is not a regular file")
	}
	file, err := root.Open("project.godot")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil || !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return nil, errors.New("project file changed while opening")
	}
	content, err := io.ReadAll(io.LimitReader(file, 1024*1024+1))
	if err != nil || len(content) > 1024*1024 {
		return nil, errors.New("project file exceeds its read bound")
	}
	return content, nil
}

func pinnedProjectUsesDotNet(ctx context.Context, root *os.Root, projectContent []byte) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	usesDotNet, err := projectContentUsesDotNet(projectContent)
	if err != nil || usesDotNet {
		return usesDotNet, err
	}
	expected, err := root.Stat(".")
	if err != nil {
		return false, err
	}
	directory, err := root.Open(".")
	if err != nil {
		return false, err
	}
	defer directory.Close()
	opened, err := directory.Stat()
	if err != nil || !opened.IsDir() || !os.SameFile(expected, opened) {
		return false, errors.New("project directory changed while opening")
	}
	entryCount := 0
	nameBytes := 0
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		entries, readErr := directory.ReadDir(256)
		for _, entry := range entries {
			entryCount++
			nameBytes += len(entry.Name())
			if entryCount > maxPinnedProjectEntries || nameBytes > maxPinnedProjectNameBytes {
				return false, errors.New("project directory exceeds its entry bound")
			}
			if directoryEntryUsesDotNet(entry) {
				return true, nil
			}
		}
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if errors.Is(readErr, io.EOF) {
			return false, nil
		}
		if readErr != nil {
			return false, readErr
		}
	}
}

func makeValidationPayload(scope, outcome string, checks []baselineValidationCheck) runPayload {
	content, err := marshalRunJSON(baselineValidationReport{SchemaVersion: contract.SchemaVersion, Scope: scope, Outcome: outcome, Checks: checks})
	if err != nil {
		content = []byte("{}\n")
	}
	return runPayload{Kind: "validation-report", Outcome: outcome, MediaType: "application/json", Content: content}
}

func finishUncommittedValidate(started time.Time, command contract.Command, scope, outcome string, exitCode int, summary string, failure contract.Error) contract.Result {
	result := contract.NewResult(started, command)
	result.Finish(started, time.Now().UTC(), outcome, exitCode, summary, map[string]any{"scope": scope, "recorded": false}, failure)
	return result
}

func encodeRunCommitFailure(started time.Time, initial contract.Result) encodedExecution {
	result := contract.NewResult(started, initial.Command)
	result.RunID = initial.RunID
	failure := contract.Error{Code: "RUN_COMMIT_FAILED", Category: "state", Message: "The command run could not be committed atomically.", Retryable: true, Remediation: "Inspect incomplete run state, then retry as a new run."}
	result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "The command did not produce a committed result.", map[string]any{"scope": validateCommandScope(initial.Command), "recorded": false}, failure)
	return encodeUncommittedResult(result)
}

func encodeRunBeginFailure(started time.Time, initial contract.Result, err error) encodedExecution {
	var failure *runBeginError
	if !errors.As(err, &failure) {
		return encodeRunUnavailable(started, initial)
	}
	switch failure.phase {
	case runBeginOrphan:
		result := contract.NewResult(started, initial.Command)
		result.RunID = initial.RunID
		problem := contract.Error{Code: "RUN_PREPARE_FAILED", Category: "state", Message: "The run directory was created, but its immutable intent was not committed.", Retryable: true, Remediation: "Inspect the orphan run directory, then retry as a new run."}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "The command did not start with a committed intent.", map[string]any{"scope": validateCommandScope(initial.Command), "recorded": false}, problem)
		return encodeUncommittedResult(result)
	case runBeginIncomplete:
		return encodeRunCommitFailure(started, initial)
	default:
		return encodeRunUnavailable(started, initial)
	}
}

func encodeRunUnavailable(started time.Time, initial contract.Result) encodedExecution {
	result := contract.NewResult(started, initial.Command)
	result.RunID = initial.RunID
	failure := contract.Error{Code: "RUN_RECORDING_UNAVAILABLE", Category: "state", Message: "Run evidence persistence was unavailable before a run root was created.", Retryable: true, Remediation: "Check project state, host, and filesystem support, then retry."}
	result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "The command could not start its evidence transaction.", map[string]any{"scope": validateCommandScope(initial.Command), "recorded": false}, failure)
	return encodeUncommittedResult(result)
}

func validateCommandScope(command contract.Command) string {
	if command.Name == "test" {
		return "gdscript"
	}
	if command.Name == "export" || command.Name == "build" {
		return "godot-export"
	}
	if command.Arguments != nil && command.Arguments["headless"] == true {
		return "headless"
	}
	return "baseline"
}

func encodeUncommittedResult(result contract.Result) encodedExecution {
	content, err := marshalRunJSON(result)
	if err != nil {
		return encodedExecution{exitCode: contract.ExitInternal}
	}
	return encodedExecution{resultBytes: content, exitCode: result.ExitCode}
}

func emitEncodedExecution(stdout, stderr interface{ Write([]byte) (int, error) }, execution encodedExecution) int {
	if len(execution.resultBytes) == 0 {
		_, _ = stderr.Write([]byte("failed to encode command result\n"))
		return contract.ExitInternal
	}
	if err := writeAll(stdout, execution.resultBytes); err != nil {
		_, _ = stderr.Write([]byte("failed to write committed command result\n"))
		return contract.ExitInternal
	}
	if execution.warning != "" {
		_ = writeAll(stderr, []byte(execution.warning))
	}
	return execution.exitCode
}

func writeAll(writer interface{ Write([]byte) (int, error) }, content []byte) error {
	for len(content) > 0 {
		written, err := writer.Write(content)
		if written < 0 || written > len(content) {
			return errors.New("writer returned an invalid byte count")
		}
		content = content[written:]
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}

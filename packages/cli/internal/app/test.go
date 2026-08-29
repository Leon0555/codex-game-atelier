package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

const gdscriptTestRunner = "tests/atelier_test_runner.gd"
const gdscriptTestRunnerResource = "res://tests/atelier_test_runner.gd"
const gdscriptTestReportPrefix = "CODEX_GAME_ATELIER_TEST_REPORT "
const maxGDScriptTestRunnerBytes = 1024 * 1024
const maxGDScriptTestCases = 256

type testOptions struct {
	project             string
	godot               string
	explicitGodot       bool
	timeoutMS           int64
	allowEngineUserData bool
}

func (options testOptions) persistedArguments() map[string]any {
	policy := "not-authorized"
	if options.allowEngineUserData {
		policy = "standard-os-location"
	}
	source := "discovery"
	if options.explicitGodot {
		source = "explicit"
	}
	return map[string]any{
		"project":          ".",
		"timeout_ms":       options.timeoutMS,
		"engine_user_data": policy,
		"godot_source":     source,
		"test_runner":      gdscriptTestRunnerResource,
	}
}

type gdscriptTestCase struct {
	ID      string `json:"id"`
	Outcome string `json:"outcome"`
	Summary string `json:"summary"`
}

type gdscriptRunnerReport struct {
	SchemaVersion string             `json:"schema_version"`
	Outcome       string             `json:"outcome"`
	Tests         []gdscriptTestCase `json:"tests"`
}

type gdscriptTestReport struct {
	SchemaVersion string             `json:"schema_version"`
	Scope         string             `json:"scope"`
	Outcome       string             `json:"outcome"`
	EngineVersion string             `json:"engine_version,omitempty"`
	Tests         []gdscriptTestCase `json:"tests"`
}

type pinnedGodotExecutionSources struct {
	projectDirectory *os.File
	runnerSource     *os.File
	engineSource     *os.File
	enginePath       string
}

func (sources *pinnedGodotExecutionSources) close() {
	if sources == nil {
		return
	}
	if sources.engineSource != nil {
		_ = sources.engineSource.Close()
	}
	if sources.runnerSource != nil {
		_ = sources.runnerSource.Close()
	}
	if sources.projectDirectory != nil {
		_ = sources.projectDirectory.Close()
	}
}

func runTest(ctx context.Context, started time.Time, args []string) encodedExecution {
	return runTestWithFault(ctx, started, args, nil)
}

func runTestWithFault(ctx context.Context, started time.Time, args []string, fault runFault) encodedExecution {
	return runTestWithPolicy(ctx, started, args, "", fault)
}

func runTestWithPolicy(ctx context.Context, started time.Time, args []string, policyMode string, fault runFault) encodedExecution {
	set := newFlagSet("test")
	project := set.String("project", ".", "Godot project directory")
	godot := set.String("godot", "", "Godot executable")
	timeoutMS := set.Int64("timeout-ms", defaultHeadlessTimeoutMS, "test timeout in milliseconds")
	allowEngineUserData := set.Bool("allow-engine-user-data", false, "allow Godot's documented standard user-data directory")
	if err := rejectDuplicateFlags(args); err != nil {
		return encodeUncommittedResult(parseError(started, "test", err.Error(), map[string]any{}))
	}
	if err := set.Parse(args); err != nil || set.NArg() != 0 || *project == "" || *timeoutMS < 1 || *timeoutMS > maxTimeoutMS {
		return encodeUncommittedResult(parseError(started, "test", "test accepts --project and optional --godot, --timeout-ms, and --allow-engine-user-data", map[string]any{}))
	}
	explicitGodot := flagWasProvided(args, "godot")
	if explicitGodot && *godot == "" {
		return encodeUncommittedResult(parseError(started, "test", "--godot requires a non-empty path", map[string]any{}))
	}
	options := testOptions{project: *project, godot: *godot, explicitGodot: explicitGodot, timeoutMS: *timeoutMS, allowEngineUserData: *allowEngineUserData}
	command := contract.Command{Name: "test", Arguments: options.persistedArguments()}

	projectRoot, err := canonicalProjectRoot(*project)
	if err != nil {
		failure := prerequisiteError("GODOT_PROJECT_NOT_FOUND", "The requested project directory does not exist or cannot be resolved.", "Select an initialized Godot project directory, then run test again.")
		return encodeUncommittedResult(finishUncommittedTest(started, command, "BLOCKED", contract.ExitPrerequisite, "GDScript tests could not locate the project.", failure))
	}
	pinnedProjectRoot, err := os.OpenRoot(projectRoot)
	if err != nil {
		failure := contract.Error{Code: "STATE_READ_FAILED", Category: "state", Message: "The project directory could not be pinned safely.", Retryable: false}
		return encodeUncommittedResult(finishUncommittedTest(started, command, "FAIL", contract.ExitState, "GDScript tests could not pin the project directory.", failure))
	}
	defer pinnedProjectRoot.Close()
	stateRoot, exists, err := openExistingStateRootFromProjectRoot(pinnedProjectRoot)
	if err != nil {
		failure := contract.Error{Code: "STATE_READ_FAILED", Category: "state", Message: "The project state directory could not be opened safely.", Retryable: false}
		return encodeUncommittedResult(finishUncommittedTest(started, command, "FAIL", contract.ExitState, "GDScript tests could not read project state.", failure))
	}
	if !exists {
		failure := prerequisiteError("PROJECT_NOT_INITIALIZED", "The project has no .gameatelier state directory.", "Run initialize before test.")
		return encodeUncommittedResult(finishUncommittedTest(started, command, "BLOCKED", contract.ExitPrerequisite, "Codex Game Atelier project state is not initialized.", failure))
	}
	defer stateRoot.Close()
	state, stateExists, _, err := loadExistingState(stateRoot)
	if err != nil || !stateExists {
		failure := contract.Error{Code: "STATE_READ_FAILED", Category: "state", Message: "The project state file could not be read safely.", Retryable: false}
		return encodeUncommittedResult(finishUncommittedTest(started, command, "FAIL", contract.ExitState, "GDScript tests could not read project state.", failure))
	}

	initial := contract.NewResult(started, command)
	if policyMode == "" {
		policyMode = state.Mode
	}
	if !validOptionalGateMode(policyMode) {
		return encodeUncommittedResult(finishUncommittedTest(started, command, "FAIL", contract.ExitState, "GDScript tests received an invalid internal policy mode.", contract.Error{Code: "POLICY_MODE_INVALID", Category: "state", Message: "The internal policy mode is outside the supported contract.", Retryable: false}))
	}
	transaction, err := beginRunWithPolicy(stateRoot, state, initial, policyMode, fault)
	if err != nil {
		return encodeRunBeginFailure(started, initial, err)
	}
	defer transaction.close()
	result, payload := executeGDScriptTests(ctx, started, initial, transaction.runRoot, pinnedProjectRoot, projectRoot, options)
	if fixedActionRunMustRemainIncomplete(result, transaction.runRoot, "test") {
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

func executeGDScriptTests(ctx context.Context, started time.Time, result contract.Result, runRoot, pinnedProjectRoot *os.Root, projectPath string, options testOptions) (contract.Result, runPayload) {
	if err := ctx.Err(); err != nil {
		failure := contract.Error{Code: "COMMAND_CANCELLED", Category: "cancelled", Message: "GDScript tests were cancelled before project inspection.", Retryable: true}
		return finishTestResult(started, result, "FAIL", contract.ExitInterrupted, "GDScript tests were cancelled.", "", nil, failure)
	}
	if !options.allowEngineUserData {
		failure := contract.Error{Code: "ENGINE_USER_DATA_NOT_AUTHORIZED", Category: "policy", Message: "Headless Godot may create its standard user:// directory outside the project.", Retryable: true, Remediation: "Review the documented side effect, then rerun with --allow-engine-user-data."}
		return finishTestResult(started, result, "BLOCKED", contract.ExitPrerequisite, "GDScript tests require explicit authorization for Godot user data.", "", nil, failure)
	}
	projectContent, err := readPinnedProjectFile(pinnedProjectRoot)
	if err != nil {
		failure := prerequisiteError("GODOT_PROJECT_UNREADABLE", "project.godot must be a bounded regular file inside the pinned project directory.", "Replace symlinks or special files with a regular project.godot at or below 1 MiB.")
		return finishTestResult(started, result, "BLOCKED", contract.ExitPrerequisite, "GDScript tests could not read the Godot project.", "", nil, failure)
	}
	usesDotNet, err := pinnedProjectUsesDotNet(ctx, pinnedProjectRoot, projectContent)
	if err != nil {
		if ctx.Err() != nil {
			failure := contract.Error{Code: "COMMAND_CANCELLED", Category: "cancelled", Message: "GDScript tests were cancelled during project inspection.", Retryable: true}
			return finishTestResult(started, result, "FAIL", contract.ExitInterrupted, "GDScript tests were cancelled.", "", nil, failure)
		}
		failure := prerequisiteError("GODOT_PROJECT_UNREADABLE", "The project language could not be checked safely.", "Check project permissions and keep project.godot at or below 1 MiB.")
		return finishTestResult(started, result, "BLOCKED", contract.ExitPrerequisite, "GDScript tests could not inspect the project language.", "", nil, failure)
	}
	if usesDotNet {
		failure := prerequisiteError("GODOT_DOTNET_UNSUPPORTED", "This project appears to use Godot .NET/C#, which is outside the v1.0 support matrix.", "Use a standard Godot GDScript project for v1.0.")
		return finishTestResult(started, result, "BLOCKED", contract.ExitPrerequisite, "GDScript tests only support standard Godot projects.", "", nil, failure)
	}
	runnerBefore, err := readPinnedTestRunner(pinnedProjectRoot)
	if err != nil {
		failure := prerequisiteError("GDSCRIPT_TEST_RUNNER_NOT_FOUND", "The fixed res://tests/atelier_test_runner.gd entry is missing, unsafe, or unreadable.", "Add the bounded Atelier GDScript test runner at the documented fixed path.")
		return finishTestResult(started, result, "BLOCKED", contract.ExitPrerequisite, "The fixed GDScript test runner is unavailable.", "", nil, failure)
	}

	sources, sourceFailure := openPinnedGodotExecutionSources(pinnedProjectRoot, projectPath, options.godot, options.explicitGodot)
	if sourceFailure != nil {
		return finishTestResult(started, result, "BLOCKED", contract.ExitPrerequisite, "GDScript tests could not pin the Godot execution sources.", "", nil, *sourceFailure)
	}
	defer sources.close()
	execution := runGodotFixedAction(ctx, time.Duration(options.timeoutMS)*time.Millisecond, sources.runnerSource, sources.engineSource, runRoot, sources.projectDirectory, "test")
	if !pinnedProjectPathMatches(pinnedProjectRoot, projectPath) {
		failure := contract.Error{Code: "PROJECT_CHANGED_DURING_TEST", Category: "state", Message: "The project path changed while Godot tests were running.", Retryable: true, Remediation: "Stop concurrent moves or replacements of the project directory, then retry."}
		return finishTestResult(started, result, "FAIL", contract.ExitState, "GDScript test observations were discarded after a project identity change.", execution.Version, nil, failure)
	}
	runnerAfter, err := readPinnedTestRunner(pinnedProjectRoot)
	if err != nil || sha256.Sum256(runnerBefore) != sha256.Sum256(runnerAfter) {
		failure := contract.Error{Code: "GDSCRIPT_TEST_RUNNER_CHANGED", Category: "state", Message: "The fixed GDScript test runner changed while tests were running.", Retryable: true, Remediation: "Stop concurrent edits to the test runner, then retry."}
		return finishTestResult(started, result, "FAIL", contract.ExitState, "GDScript test observations were discarded after the runner changed.", execution.Version, nil, failure)
	}

	if execution.FailureStage == "test" && execution.Failure == headlessFailureProcess && execution.ActionProcess.ExitCode != nil && *execution.ActionProcess.ExitCode == 1 && !execution.ActionProcess.StdoutTruncated && !execution.ActionProcess.StderrTruncated && !containsGodotError(execution.ActionProcess.Stderr) {
		report, reportErr := parseGDScriptRunnerReport(execution.ActionProcess.Stdout)
		if reportErr == nil && report.Outcome == "FAIL" {
			failure := contract.Error{Code: "GDSCRIPT_TESTS_FAILED", Category: "validation", Message: "One or more GDScript tests failed.", Retryable: false, Details: map[string]any{"failed_count": countTestOutcomes(report.Tests, "FAIL")}}
			return finishTestResult(started, result, "FAIL", contract.ExitValidation, "GDScript tests completed with assertion failures.", execution.Version, report.Tests, failure)
		}
	}
	if execution.Failure != headlessFailureNone {
		outcome, exitCode, failure := mapTestExecutionFailure(execution)
		return finishTestResult(started, result, outcome, exitCode, "GDScript test execution did not complete successfully.", execution.Version, nil, failure)
	}
	report, err := parseGDScriptRunnerReport(execution.ActionProcess.Stdout)
	if err != nil || report.Outcome != "PASS" {
		failure := contract.Error{Code: "GDSCRIPT_TEST_REPORT_INVALID", Category: "engine", Message: "The fixed test runner did not emit one valid passing report.", Retryable: false}
		return finishTestResult(started, result, "FAIL", contract.ExitEngine, "GDScript test output could not be trusted.", execution.Version, nil, failure)
	}
	return finishTestResult(started, result, "PASS", contract.ExitOK, "GDScript tests passed.", execution.Version, report.Tests)
}

func openPinnedGodotExecutionSources(root *os.Root, projectPath, requestedGodot string, explicitGodot bool) (*pinnedGodotExecutionSources, *contract.Error) {
	projectDirectory, err := openPinnedProjectDirectory(root, projectPath)
	if err != nil {
		failure := prerequisiteError("PROJECT_IDENTITY_UNAVAILABLE", "The project path no longer identifies the pinned project.", "Stop concurrent project moves or replacements, then retry.")
		return nil, &failure
	}
	discovery, discoveryFailure := discoverGodot(requestedGodot, explicitGodot)
	if discoveryFailure != nil {
		projectDirectory.Close()
		return nil, discoveryFailure
	}
	runner, err := discoverPinnedGodotRunner()
	if err != nil {
		projectDirectory.Close()
		failure := prerequisiteError("GODOT_RUNNER_UNAVAILABLE", "The paired internal runner executable is missing or not runnable.", "Install the complete Codex Game Atelier CLI artifact pair, then retry.")
		return nil, &failure
	}
	runnerSource, err := openStableRegularFile(runner)
	if err != nil {
		projectDirectory.Close()
		failure := prerequisiteError("GODOT_RUNNER_PIN_UNAVAILABLE", "The paired internal runner could not be opened as a fixed file identity.", "Stop concurrent CLI updates, then retry.")
		return nil, &failure
	}
	engineSource, err := openStableRegularFile(discovery.Executable)
	if err != nil {
		runnerSource.Close()
		projectDirectory.Close()
		failure := prerequisiteError("GODOT_EXECUTABLE_PIN_UNAVAILABLE", "The selected executable could not be opened as a fixed file identity.", "Stop concurrent engine updates, then retry.")
		return nil, &failure
	}
	return &pinnedGodotExecutionSources{projectDirectory: projectDirectory, runnerSource: runnerSource, engineSource: engineSource, enginePath: discovery.Executable}, nil
}

func openStableRegularFile(path string) (*os.File, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	opened, openedErr := file.Stat()
	current, currentErr := os.Stat(path)
	if openedErr != nil || currentErr != nil || !opened.Mode().IsRegular() || !os.SameFile(opened, current) {
		file.Close()
		return nil, errors.New("file identity changed while opening")
	}
	return file, nil
}

func readPinnedTestRunner(root *os.Root) ([]byte, error) {
	if root == nil {
		return nil, errors.New("project root is not open")
	}
	for _, directory := range []string{"tests"} {
		info, err := root.Lstat(directory)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return nil, errors.New("test runner parent is not a regular directory")
		}
	}
	info, err := root.Lstat(gdscriptTestRunner)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, errors.New("test runner is not a regular file")
	}
	file, err := root.Open(gdscriptTestRunner)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, errors.New("test runner changed while opening")
	}
	content, err := io.ReadAll(io.LimitReader(file, maxGDScriptTestRunnerBytes+1))
	if err != nil || len(content) == 0 || len(content) > maxGDScriptTestRunnerBytes {
		return nil, errors.New("test runner is empty or exceeds its read bound")
	}
	return content, nil
}

func parseGDScriptRunnerReport(stdout []byte) (gdscriptRunnerReport, error) {
	var encoded []byte
	for _, line := range bytes.Split(stdout, []byte{'\n'}) {
		trimmed := bytes.TrimSpace(line)
		if !bytes.HasPrefix(trimmed, []byte(gdscriptTestReportPrefix)) {
			continue
		}
		if encoded != nil {
			return gdscriptRunnerReport{}, errors.New("multiple GDScript test reports")
		}
		encoded = bytes.TrimSpace(bytes.TrimPrefix(trimmed, []byte(gdscriptTestReportPrefix)))
	}
	if len(encoded) == 0 {
		return gdscriptRunnerReport{}, errors.New("GDScript test report is missing")
	}
	if !utf8.Valid(encoded) {
		return gdscriptRunnerReport{}, errors.New("GDScript test report is not valid UTF-8")
	}
	duplicateDecoder := json.NewDecoder(bytes.NewReader(encoded))
	duplicateDecoder.UseNumber()
	if err := rejectDuplicateObjectKeysWithin(duplicateDecoder, maxPersistedJSONDepth, maxPersistedJSONNodes); err != nil {
		return gdscriptRunnerReport{}, errors.New("GDScript test report has duplicate keys or invalid JSON")
	}
	if token, err := duplicateDecoder.Token(); err != io.EOF || token != nil {
		return gdscriptRunnerReport{}, errors.New("GDScript test report contains multiple JSON values")
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var report gdscriptRunnerReport
	if err := decoder.Decode(&report); err != nil {
		return gdscriptRunnerReport{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return gdscriptRunnerReport{}, errors.New("GDScript test report contains trailing data")
	}
	if report.SchemaVersion != contract.SchemaVersion || report.Outcome != "PASS" && report.Outcome != "FAIL" || len(report.Tests) < 1 || len(report.Tests) > maxGDScriptTestCases {
		return gdscriptRunnerReport{}, errors.New("GDScript test report envelope is invalid")
	}
	seen := make(map[string]struct{}, len(report.Tests))
	failed := 0
	for _, test := range report.Tests {
		if !evidenceKindPattern.MatchString(test.ID) || test.Outcome != "PASS" && test.Outcome != "FAIL" || !validGDScriptTestSummary(test.Summary) {
			return gdscriptRunnerReport{}, errors.New("GDScript test case is invalid")
		}
		if _, exists := seen[test.ID]; exists {
			return gdscriptRunnerReport{}, errors.New("GDScript test IDs must be unique")
		}
		seen[test.ID] = struct{}{}
		if test.Outcome == "FAIL" {
			failed++
		}
	}
	if report.Outcome == "PASS" && failed != 0 || report.Outcome == "FAIL" && failed == 0 {
		return gdscriptRunnerReport{}, errors.New("GDScript test aggregate outcome is inconsistent")
	}
	return report, nil
}

func validGDScriptTestSummary(summary string) bool {
	if utf8.RuneCountInString(summary) < 1 || utf8.RuneCountInString(summary) > 512 || strings.TrimSpace(summary) == "" || !safePersistedString(summary) {
		return false
	}
	for _, character := range summary {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func finishTestResult(started time.Time, result contract.Result, outcome string, exitCode int, summary, engineVersion string, tests []gdscriptTestCase, failures ...contract.Error) (contract.Result, runPayload) {
	if tests == nil {
		tests = []gdscriptTestCase{}
	}
	passed := countTestOutcomes(tests, "PASS")
	failed := countTestOutcomes(tests, "FAIL")
	data := map[string]any{
		"scope":          "gdscript",
		"test_count":     len(tests),
		"passed_count":   passed,
		"failed_count":   failed,
		"engine_version": engineVersion,
	}
	result.Finish(started, time.Now().UTC(), outcome, exitCode, summary, data, failures...)
	report := gdscriptTestReport{SchemaVersion: contract.SchemaVersion, Scope: "gdscript", Outcome: outcome, EngineVersion: engineVersion, Tests: tests}
	content, err := marshalRunJSON(report)
	if err != nil {
		content = []byte("{}\n")
	}
	return result, runPayload{Kind: "test-report", Outcome: outcome, MediaType: "application/json", Content: content}
}

func countTestOutcomes(tests []gdscriptTestCase, outcome string) int {
	count := 0
	for _, test := range tests {
		if test.Outcome == outcome {
			count++
		}
	}
	return count
}

func finishUncommittedTest(started time.Time, command contract.Command, outcome string, exitCode int, summary string, failure contract.Error) contract.Result {
	result := contract.NewResult(started, command)
	result.Finish(started, time.Now().UTC(), outcome, exitCode, summary, map[string]any{"scope": "gdscript", "recorded": false}, failure)
	return result
}

func mapTestExecutionFailure(execution godotHeadlessExecution) (string, int, contract.Error) {
	details := map[string]any{"stage": execution.FailureStage}
	process := execution.VersionProcess
	if execution.FailureStage == "test" {
		process = execution.ActionProcess
	}
	if process.ExitCode != nil {
		details["process_exit_code"] = *process.ExitCode
	}
	switch execution.Failure {
	case headlessFailureCancelled:
		return "FAIL", contract.ExitInterrupted, contract.Error{Code: "COMMAND_CANCELLED", Category: "cancelled", Message: "GDScript tests were cancelled.", Retryable: true, Details: details}
	case headlessFailureTimeout:
		return "FAIL", contract.ExitInterrupted, contract.Error{Code: "GODOT_TIMEOUT", Category: "timeout", Message: "GDScript tests exceeded their total timeout.", Retryable: true, Details: details}
	case headlessFailureOutputTruncated:
		return "FAIL", contract.ExitEngine, contract.Error{Code: "GODOT_OUTPUT_TRUNCATED", Category: "engine", Message: "Godot test output exceeded the bounded capture limit and was not trusted.", Retryable: false, Details: details}
	case headlessFailureUnsupportedVersion:
		return "BLOCKED", contract.ExitPrerequisite, prerequisiteError("GODOT_VERSION_UNSUPPORTED", "The selected executable did not report the supported Godot 4.7.2-stable official standard build identifier.", "Install or select the official standard Godot 4.7.2-stable executable.")
	case headlessFailureSnapshotUnavailable:
		return "BLOCKED", contract.ExitPrerequisite, prerequisiteError("GODOT_EXECUTABLE_SNAPSHOT_UNAVAILABLE", "The paired private runner or selected Godot executable could not be snapshotted into the run evidence filesystem.", "Install the complete CLI artifact pair and place the project and Godot executable on supported filesystems, then retry.")
	case headlessFailureExecutableChanged:
		return "FAIL", contract.ExitEngine, contract.Error{Code: "GODOT_EXECUTABLE_CHANGED", Category: "engine", Message: "A pinned private-runner or Godot executable snapshot changed during tests.", Retryable: true, Details: details}
	case headlessFailureSnapshotCleanup:
		return "FAIL", contract.ExitState, contract.Error{Code: "GODOT_EXECUTABLE_SNAPSHOT_CLEANUP_FAILED", Category: "state", Message: "A transient private-runner or Godot executable snapshot could not be removed safely.", Retryable: true, Details: details}
	case headlessFailureEngineErrors:
		return "FAIL", contract.ExitEngine, contract.Error{Code: "GODOT_REPORTED_ERRORS", Category: "engine", Message: "Godot emitted one or more ERROR records during tests.", Retryable: true, Details: details}
	default:
		return "FAIL", contract.ExitEngine, contract.Error{Code: "GODOT_PROCESS_FAILED", Category: "engine", Message: "Godot failed while running GDScript tests.", Retryable: true, Details: details}
	}
}

func fixedActionRunMustRemainIncomplete(result contract.Result, runRoot *os.Root, action string) bool {
	for _, failure := range result.Errors {
		if failure.Code == "GODOT_EXECUTABLE_SNAPSHOT_CLEANUP_FAILED" {
			return true
		}
	}
	for _, name := range []string{
		".atelier-version-runner",
		".atelier-version-runner.cstemp",
		".godot-version-snapshot",
		".godot-version-snapshot.cstemp",
		".atelier-" + action + "-runner",
		".atelier-" + action + "-runner.cstemp",
		".godot-" + action + "-snapshot",
		".godot-" + action + "-snapshot.cstemp",
		exportRuntimeDirectory,
		godotProjectSnapshotDirectory,
		godotTargetSmokeRunner,
		godotTargetSmokeRunner + ".cstemp",
	} {
		if _, err := runRoot.Lstat(name); err == nil || !errors.Is(err, os.ErrNotExist) {
			return true
		}
	}
	return false
}

func validatePersistedTestArguments(arguments map[string]any) error {
	if len(arguments) != 5 || arguments["project"] != "." || arguments["test_runner"] != gdscriptTestRunnerResource {
		return errors.New("persisted test arguments violate the fixed runner contract")
	}
	timeout, ok := persistedInteger(arguments["timeout_ms"])
	if !ok || timeout < 1 || timeout > maxTimeoutMS {
		return errors.New("persisted test timeout is invalid")
	}
	policy, ok := arguments["engine_user_data"].(string)
	if !ok || policy != "not-authorized" && policy != "standard-os-location" {
		return errors.New("persisted test user-data policy is invalid")
	}
	source, ok := arguments["godot_source"].(string)
	if !ok || source != "explicit" && source != "discovery" {
		return errors.New("persisted test Godot source is invalid")
	}
	return nil
}

func preflightTestRunFinish(result contract.Result, payloads []runPayload) error {
	data, ok := result.Data.(map[string]any)
	if !ok || len(data) != 5 || data["scope"] != "gdscript" {
		return errors.New("test result data violates its bounded contract")
	}
	testCount, testCountOK := persistedInteger(data["test_count"])
	passedCount, passedCountOK := persistedInteger(data["passed_count"])
	failedCount, failedCountOK := persistedInteger(data["failed_count"])
	engineVersion, versionOK := data["engine_version"].(string)
	if !testCountOK || !passedCountOK || !failedCountOK || testCount < 0 || testCount > maxGDScriptTestCases || passedCount < 0 || failedCount < 0 || passedCount+failedCount != testCount || !versionOK || engineVersion != "" && !supportedGodotVersion.MatchString(engineVersion) {
		return errors.New("test result counts or engine version are invalid")
	}
	if testCount > 0 {
		if engineVersion == "" {
			return errors.New("executed tests require a verified engine version")
		}
		if result.Outcome == "FAIL" {
			if result.ExitCode != contract.ExitValidation || len(result.Errors) != 1 {
				return errors.New("test assertion failure has an invalid result mapping")
			}
			failure := result.Errors[0]
			reportedFailed, reported := persistedInteger(failure.Details["failed_count"])
			if failure.Code != "GDSCRIPT_TESTS_FAILED" || failure.Category != "validation" || failure.Retryable || len(failure.Details) != 1 || !reported || reportedFailed != failedCount {
				return errors.New("test assertion failure has an invalid structured error")
			}
		} else if result.Outcome != "PASS" {
			return errors.New("executed tests have an invalid aggregate outcome")
		}
	} else if err := validateEmptyTestFailureMapping(result); err != nil {
		return err
	}
	if len(payloads) != 1 {
		return errors.New("test run must contain one report payload")
	}
	payload := payloads[0]
	if payload.Kind != "test-report" || payload.Outcome != result.Outcome || payload.MediaType != "application/json" || len(payload.Metadata) != 0 || len(payload.Content) == 0 || !utf8.Valid(payload.Content) || int64(len(payload.Content)) > maxRunPayloadBytes {
		return errors.New("test report payload violates its bounded contract")
	}
	duplicateDecoder := json.NewDecoder(bytes.NewReader(payload.Content))
	duplicateDecoder.UseNumber()
	if err := rejectDuplicateObjectKeysWithin(duplicateDecoder, maxPersistedJSONDepth, maxPersistedJSONNodes); err != nil {
		return errors.New("test report payload has duplicate keys or invalid JSON")
	}
	if token, err := duplicateDecoder.Token(); err != io.EOF || token != nil {
		return errors.New("test report payload contains multiple JSON values")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload.Content))
	decoder.DisallowUnknownFields()
	var report gdscriptTestReport
	if err := decoder.Decode(&report); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("test report payload violates its strict shape")
	}
	if report.SchemaVersion != contract.SchemaVersion || report.Scope != "gdscript" || report.Outcome != result.Outcome || report.EngineVersion != engineVersion || len(report.Tests) != int(testCount) {
		return errors.New("test report conflicts with its result")
	}
	if len(report.Tests) == 0 {
		if result.Outcome == "PASS" || passedCount != 0 || failedCount != 0 {
			return errors.New("empty test report cannot pass")
		}
		return nil
	}
	runnerReport := gdscriptRunnerReport{SchemaVersion: report.SchemaVersion, Outcome: report.Outcome, Tests: report.Tests}
	encoded, err := json.Marshal(runnerReport)
	if err != nil {
		return err
	}
	parsed, err := parseGDScriptRunnerReport([]byte(gdscriptTestReportPrefix + string(encoded)))
	if err != nil || !reflect.DeepEqual(parsed.Tests, report.Tests) || int64(countTestOutcomes(report.Tests, "PASS")) != passedCount || int64(countTestOutcomes(report.Tests, "FAIL")) != failedCount {
		return errors.New("test report cases violate their aggregate contract")
	}
	return nil
}

func validateEmptyTestFailureMapping(result contract.Result) error {
	if result.Outcome == "PASS" || len(result.Errors) != 1 {
		return errors.New("empty test report requires one mapped failure")
	}
	failure := result.Errors[0]
	if result.Outcome == "BLOCKED" && result.ExitCode == contract.ExitPrerequisite {
		if failure.Code == "ENGINE_USER_DATA_NOT_AUTHORIZED" && failure.Category == "policy" {
			return nil
		}
		if failure.Category != "prerequisite" {
			return errors.New("blocked test result has an invalid error category")
		}
		switch failure.Code {
		case "GODOT_PROJECT_UNREADABLE", "GODOT_DOTNET_UNSUPPORTED", "GDSCRIPT_TEST_RUNNER_NOT_FOUND", "PROJECT_IDENTITY_UNAVAILABLE", "GODOT_NOT_FOUND", "GODOT_NOT_EXECUTABLE", "GODOT_RUNNER_UNAVAILABLE", "GODOT_RUNNER_PIN_UNAVAILABLE", "GODOT_EXECUTABLE_PIN_UNAVAILABLE", "GODOT_VERSION_UNSUPPORTED", "GODOT_EXECUTABLE_SNAPSHOT_UNAVAILABLE":
			return nil
		}
		return errors.New("blocked test result has an unknown prerequisite code")
	}
	if result.Outcome != "FAIL" {
		return errors.New("empty test report has an invalid outcome")
	}
	switch result.ExitCode {
	case contract.ExitEngine:
		if failure.Category != "engine" {
			break
		}
		switch failure.Code {
		case "GODOT_OUTPUT_TRUNCATED", "GODOT_EXECUTABLE_CHANGED", "GODOT_REPORTED_ERRORS", "GODOT_PROCESS_FAILED", "GDSCRIPT_TEST_REPORT_INVALID":
			return nil
		}
	case contract.ExitInterrupted:
		if failure.Code == "COMMAND_CANCELLED" && failure.Category == "cancelled" || failure.Code == "GODOT_TIMEOUT" && failure.Category == "timeout" {
			return nil
		}
	case contract.ExitState:
		if failure.Category == "state" && (failure.Code == "PROJECT_CHANGED_DURING_TEST" || failure.Code == "GDSCRIPT_TEST_RUNNER_CHANGED") {
			return nil
		}
	}
	return errors.New("empty test report has an invalid failure mapping")
}

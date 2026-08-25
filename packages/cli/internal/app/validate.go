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

type encodedExecution struct {
	resultBytes []byte
	exitCode    int
	warning     string
}

func runValidate(ctx context.Context, started time.Time, args []string) encodedExecution {
	return runValidateWithFault(ctx, started, args, nil)
}

func runValidateWithFault(ctx context.Context, started time.Time, args []string, fault runFault) encodedExecution {
	set := newFlagSet("validate")
	project := set.String("project", ".", "Godot project directory")
	if err := rejectDuplicateFlags(args); err != nil {
		return encodeUncommittedResult(parseError(started, "validate", err.Error(), map[string]any{}))
	}
	if err := set.Parse(args); err != nil || set.NArg() != 0 || *project == "" {
		return encodeUncommittedResult(parseError(started, "validate", "validate accepts --project only", map[string]any{}))
	}

	projectRoot, err := canonicalProjectRoot(*project)
	if err != nil {
		failure := prerequisiteError("GODOT_PROJECT_NOT_FOUND", "The requested project directory does not exist or cannot be resolved.", "Select an initialized Godot project directory, then run validate again.")
		return encodeUncommittedResult(finishUncommittedValidate(started, "BLOCKED", contract.ExitPrerequisite, "Baseline validation could not locate the project.", failure))
	}
	pinnedProjectRoot, err := os.OpenRoot(projectRoot)
	if err != nil {
		failure := contract.Error{Code: "STATE_READ_FAILED", Category: "state", Message: "The project directory could not be pinned safely.", Retryable: false}
		return encodeUncommittedResult(finishUncommittedValidate(started, "FAIL", contract.ExitState, "Baseline validation could not pin the project directory.", failure))
	}
	defer pinnedProjectRoot.Close()
	stateRoot, exists, err := openExistingStateRootFromProjectRoot(pinnedProjectRoot)
	if err != nil {
		failure := contract.Error{Code: "STATE_READ_FAILED", Category: "state", Message: "The project state directory could not be opened safely.", Retryable: false}
		return encodeUncommittedResult(finishUncommittedValidate(started, "FAIL", contract.ExitState, "Baseline validation could not read project state.", failure))
	}
	if !exists {
		failure := prerequisiteError("PROJECT_NOT_INITIALIZED", "The project has no .gameatelier state directory.", "Run initialize before validate.")
		return encodeUncommittedResult(finishUncommittedValidate(started, "BLOCKED", contract.ExitPrerequisite, "Codex Game Atelier project state is not initialized.", failure))
	}
	defer stateRoot.Close()
	state, stateExists, _, err := loadExistingState(stateRoot)
	if err != nil || !stateExists {
		failure := contract.Error{Code: "STATE_READ_FAILED", Category: "state", Message: "The project state file could not be read safely.", Retryable: false}
		return encodeUncommittedResult(finishUncommittedValidate(started, "FAIL", contract.ExitState, "Baseline validation could not read project state.", failure))
	}

	initial := contract.NewResult(started, contract.Command{Name: "validate", Arguments: map[string]any{"project": "."}})
	transaction, err := beginRun(stateRoot, state, initial, fault)
	if err != nil {
		return encodeRunBeginFailure(started, initial, err)
	}
	defer transaction.close()
	result, payload := executeBaselineValidation(ctx, started, initial, pinnedProjectRoot)
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

func executeBaselineValidation(ctx context.Context, started time.Time, result contract.Result, projectRoot *os.Root) (contract.Result, runPayload) {
	checks := []baselineValidationCheck{
		{ID: "project-state", Outcome: "PASS", Summary: "Project state is valid."},
		{ID: "persistence-platform", Outcome: "PASS", Summary: "Run evidence persistence is enabled on this host and filesystem."},
	}
	if err := ctx.Err(); err != nil {
		return finishCancelledValidation(started, result)
	}

	projectContent, err := readPinnedProjectFile(projectRoot)
	if err != nil {
		checks = append(checks,
			baselineValidationCheck{ID: "project-file", Outcome: "BLOCKED", Summary: "Godot project file is missing, unsafe, or unreadable."},
			baselineValidationCheck{ID: "project-language", Outcome: "SKIPPED", Summary: "Project language validation requires a readable project file."},
		)
		failure := prerequisiteError("GODOT_PROJECT_UNREADABLE", "project.godot must be a bounded regular file inside the pinned project directory.", "Replace symlinks or special files with a regular project.godot at or below 1 MiB.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Baseline project validation is blocked by the project file.", map[string]any{"scope": "baseline", "check_count": len(checks)}, failure)
		return result, makeValidationPayload(result.Outcome, checks)
	}
	if err := ctx.Err(); err != nil {
		return finishCancelledValidation(started, result)
	}
	checks = append(checks, baselineValidationCheck{ID: "project-file", Outcome: "PASS", Summary: "Godot project file is present."})

	usesDotNet, err := pinnedProjectUsesDotNet(ctx, projectRoot, projectContent)
	if ctx.Err() != nil {
		return finishCancelledValidation(started, result)
	}
	if err != nil {
		checks = append(checks, baselineValidationCheck{ID: "project-language", Outcome: "BLOCKED", Summary: "Project language could not be inspected safely."})
		failure := prerequisiteError("GODOT_PROJECT_UNREADABLE", "project.godot or its directory could not be read within safety bounds.", "Check permissions and keep project.godot at or below 1 MiB.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Baseline project validation could not inspect the project language.", map[string]any{"scope": "baseline", "check_count": len(checks)}, failure)
		return result, makeValidationPayload(result.Outcome, checks)
	}
	if usesDotNet {
		checks = append(checks, baselineValidationCheck{ID: "project-language", Outcome: "BLOCKED", Summary: "Godot .NET/C# is outside the v1.0 support matrix."})
		failure := prerequisiteError("GODOT_DOTNET_UNSUPPORTED", "This project appears to use Godot .NET/C#, which is outside the v1.0 support matrix.", "Use a standard Godot GDScript project for v1.0.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Baseline project validation only supports GDScript projects.", map[string]any{"scope": "baseline", "check_count": len(checks)}, failure)
		return result, makeValidationPayload(result.Outcome, checks)
	}
	checks = append(checks, baselineValidationCheck{ID: "project-language", Outcome: "PASS", Summary: "Project is within the standard Godot GDScript scope."})
	if err := ctx.Err(); err != nil {
		return finishCancelledValidation(started, result)
	}
	result.Finish(started, time.Now().UTC(), "PASS", contract.ExitOK, "Baseline project validation passed.", map[string]any{"scope": "baseline", "check_count": len(checks)})
	return result, makeValidationPayload(result.Outcome, checks)
}

func finishCancelledValidation(started time.Time, result contract.Result) (contract.Result, runPayload) {
	checks := []baselineValidationCheck{
		{ID: "project-state", Outcome: "PASS", Summary: "Project state is valid."},
		{ID: "persistence-platform", Outcome: "PASS", Summary: "Run evidence persistence is enabled on this host and filesystem."},
		{ID: "project-file", Outcome: "FAIL", Summary: "Project checks were interrupted before completion."},
		{ID: "project-language", Outcome: "SKIPPED", Summary: "Project language validation did not complete after cancellation."},
	}
	failure := contract.Error{Code: "COMMAND_CANCELLED", Category: "cancelled", Message: "Baseline validation was cancelled.", Retryable: true}
	result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitInterrupted, "Baseline project validation was cancelled.", map[string]any{"scope": "baseline", "check_count": len(checks)}, failure)
	return result, makeValidationPayload(result.Outcome, checks)
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

func makeValidationPayload(outcome string, checks []baselineValidationCheck) runPayload {
	content, err := marshalRunJSON(baselineValidationReport{SchemaVersion: contract.SchemaVersion, Scope: "baseline", Outcome: outcome, Checks: checks})
	if err != nil {
		content = []byte("{}\n")
	}
	return runPayload{Kind: "validation-report", Outcome: outcome, MediaType: "application/json", Content: content}
}

func finishUncommittedValidate(started time.Time, outcome string, exitCode int, summary string, failure contract.Error) contract.Result {
	result := contract.NewResult(started, contract.Command{Name: "validate", Arguments: map[string]any{"project": "."}})
	result.Finish(started, time.Now().UTC(), outcome, exitCode, summary, map[string]any{"scope": "baseline", "recorded": false}, failure)
	return result
}

func encodeRunCommitFailure(started time.Time, initial contract.Result) encodedExecution {
	result := contract.NewResult(started, initial.Command)
	result.RunID = initial.RunID
	failure := contract.Error{Code: "RUN_COMMIT_FAILED", Category: "state", Message: "The baseline validation run could not be committed atomically.", Retryable: true, Remediation: "Inspect incomplete run state, then retry as a new run."}
	result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Baseline validation did not produce a committed result.", map[string]any{"scope": "baseline", "recorded": false}, failure)
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
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Baseline validation did not start with a committed intent.", map[string]any{"scope": "baseline", "recorded": false}, problem)
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
	result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Baseline validation could not start its evidence transaction.", map[string]any{"scope": "baseline", "recorded": false}, failure)
	return encodeUncommittedResult(result)
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

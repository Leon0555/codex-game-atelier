package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

const maxStructuredLogEvents = maxGDScriptTestCases + 64 + 1
const maxSingleRunLogBytes = maxRunIntentBytes + 3*maxRunPayloadBytes

type structuredLogEvent struct {
	Source  string `json:"source"`
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Level   string `json:"level"`
	Outcome string `json:"outcome"`
}

type logsData struct {
	Scope             string               `json:"scope"`
	TargetRunID       string               `json:"target_run_id"`
	SourceCommand     string               `json:"source_command,omitempty"`
	SourceOutcome     string               `json:"source_outcome,omitempty"`
	SourceStartedAt   string               `json:"source_started_at,omitempty"`
	SourceFinishedAt  string               `json:"source_finished_at,omitempty"`
	SourceExitCode    *int                 `json:"source_exit_code,omitempty"`
	EvidenceKind      string               `json:"evidence_kind,omitempty"`
	EvidenceSHA256    string               `json:"evidence_sha256,omitempty"`
	EvidenceByteSize  int64                `json:"evidence_byte_size,omitempty"`
	ProducerVersion   string               `json:"producer_version,omitempty"`
	RawOutputIncluded bool                 `json:"raw_output_included"`
	Events            []structuredLogEvent `json:"events"`
}

func emptyLogsData(runID string) logsData {
	return logsData{Scope: "run", TargetRunID: runID, Events: []structuredLogEvent{}}
}

func runLogs(ctx context.Context, started time.Time, args []string) contract.Result {
	set := newFlagSet("logs")
	project := set.String("project", ".", "project directory")
	runID := set.String("run-id", "", "committed run identifier")
	if err := rejectDuplicateFlags(args); err != nil {
		return parseError(started, "logs", err.Error(), map[string]any{})
	}
	if err := set.Parse(args); err != nil || set.NArg() != 0 || *project == "" || !runIDPattern.MatchString(*runID) {
		return parseError(started, "logs", "logs requires one strict --run-id and accepts --project only", map[string]any{})
	}

	result := contract.NewResult(started, contract.Command{Name: "logs", Arguments: map[string]any{"project": ".", "run_id": *runID}})
	data := emptyLogsData(*runID)
	if err := ctx.Err(); err != nil {
		return logsCancelled(started, result, data)
	}
	root, err := canonicalProjectRoot(*project)
	if err != nil {
		failure := contract.Error{Code: "RUN_LOGS_FAILED", Category: "state", Message: "The requested project directory cannot be resolved safely.", Retryable: false}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Structured run logs could not be read.", data, failure)
		return result
	}
	stateRoot, exists, err := openExistingStateRoot(root)
	if err != nil {
		failure := contract.Error{Code: "RUN_LOGS_UNSAFE", Category: "state", Message: "The project state root could not be opened safely.", Retryable: false}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Structured run logs could not be read safely.", data, failure)
		return result
	}
	if !exists {
		failure := prerequisiteError("PROJECT_NOT_INITIALIZED", "The project has no .gameatelier state directory.", "Run initialize before reading structured run logs.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Codex Game Atelier project state is not initialized.", data, failure)
		return result
	}
	defer stateRoot.Close()

	state, initialized, _, err := loadExistingState(stateRoot)
	if err != nil {
		failure := contract.Error{Code: "RUN_LOGS_UNSAFE", Category: "state", Message: "The project state is invalid or unsafe.", Retryable: false}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Structured run logs could not be read safely.", data, failure)
		return result
	}
	if !initialized {
		failure := prerequisiteError("PROJECT_NOT_INITIALIZED", "The project has no .gameatelier/project.json state file.", "Run initialize before reading structured run logs.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Codex Game Atelier project state is not initialized.", data, failure)
		return result
	}

	runsRoot, exists, err := openExistingVerifiedDirectory(stateRoot, "runs")
	if err != nil {
		failure := contract.Error{Code: "RUN_LOGS_UNSAFE", Category: "state", Message: "The run store could not be opened safely.", Retryable: false}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Structured run logs could not be read safely.", data, failure)
		return result
	}
	if !exists {
		return logsRunNotFound(started, result, data)
	}
	defer runsRoot.Close()
	runRoot, exists, err := openExistingVerifiedDirectory(runsRoot, *runID)
	if err != nil {
		failure := contract.Error{Code: "RUN_LOGS_UNSAFE", Category: "state", Message: "The selected run directory is unsafe.", Retryable: false}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Structured run logs could not be read safely.", data, failure)
		return result
	}
	if !exists {
		return logsRunNotFound(started, result, data)
	}
	defer runRoot.Close()

	var run verifiedRun
	stateName, reason, err := classifyRun(ctx, newRunScanBudget(maxSingleRunLogBytes, 4), runRoot, state, *runID, &run)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return logsCancelled(started, result, data)
		}
		code := "RUN_LOGS_UNSAFE"
		message := "The selected run closure is unsafe or invalid."
		if errors.Is(err, errRunScanBudgetExceeded) {
			code = "RUN_LOGS_LIMIT_EXCEEDED"
			message = "The selected run exceeds the bounded read budget."
		}
		failure := contract.Error{Code: code, Category: "state", Message: message, Retryable: false}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Structured run logs could not be read safely.", data, failure)
		return result
	}
	if stateName != "committed" {
		if stateName == "incomplete" || stateName == "orphan" {
			failure := prerequisiteError("RUN_NOT_COMMITTED", "The selected run has no verified committed result.", "Complete the operation or select a committed run.")
			result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Structured logs are available only for committed runs.", data, failure)
			return result
		}
		if reason == "SCHEMA_UNSUPPORTED" {
			failure := contract.Error{Code: "RUN_SCHEMA_UNSUPPORTED", Category: "state", Message: "The selected run uses an unsupported schema version.", Retryable: false}
			result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Structured logs cannot read this run schema.", data, failure)
			return result
		}
		failure := contract.Error{Code: "RUN_LOGS_UNSAFE", Category: "state", Message: "The selected run closure is invalid.", Retryable: false, Details: map[string]any{"reason": reason}}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Structured run logs could not be read safely.", data, failure)
		return result
	}

	events, err := projectStructuredLogEvents(run)
	if err != nil || len(events) > maxStructuredLogEvents {
		failure := contract.Error{Code: "RUN_LOGS_UNSAFE", Category: "state", Message: "The verified run could not be projected into bounded structured logs.", Retryable: false}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Structured run logs could not be produced safely.", data, failure)
		return result
	}
	data.SourceCommand = run.Result.Command.Name
	data.SourceOutcome = run.Result.Outcome
	data.SourceStartedAt = run.Result.StartedAt
	data.SourceFinishedAt = run.Result.FinishedAt
	data.SourceExitCode = &run.Result.ExitCode
	data.EvidenceKind = run.Record.Kind
	data.EvidenceSHA256 = run.Record.SHA256
	data.EvidenceByteSize = run.Record.ByteSize
	data.ProducerVersion = run.Record.Producer.Version
	data.Events = events
	result.Finish(started, time.Now().UTC(), "PASS", contract.ExitOK, "Verified structured run logs were read without modification.", data)
	return result
}

func projectStructuredLogEvents(run verifiedRun) ([]structuredLogEvent, error) {
	events := make([]structuredLogEvent, 0, maxStructuredLogEvents)
	appendEvent := func(source, kind, id, outcome string) {
		events = append(events, structuredLogEvent{Source: source, Kind: kind, ID: id, Level: structuredLogLevel(outcome), Outcome: outcome})
	}
	switch run.PayloadKind {
	case "validation-report":
		var report baselineValidationReport
		if err := decodeStrictRunJSON(run.Payload, &report); err != nil {
			return nil, err
		}
		for index, check := range report.Checks {
			appendEvent(run.PayloadKind, "check", fmt.Sprintf("check-%04d", index+1), check.Outcome)
		}
	case "test-report":
		var report gdscriptTestReport
		if err := decodeStrictRunJSON(run.Payload, &report); err != nil {
			return nil, err
		}
		for index, test := range report.Tests {
			appendEvent(run.PayloadKind, "test", fmt.Sprintf("test-%04d", index+1), test.Outcome)
		}
	default:
		return nil, errors.New("unsupported verified payload kind")
	}
	for index := range run.Result.Errors {
		appendEvent("result", "error", fmt.Sprintf("error-%04d", index+1), run.Result.Outcome)
	}
	appendEvent("result", "result", "command-finished", run.Result.Outcome)
	return events, nil
}

func structuredLogLevel(outcome string) string {
	switch outcome {
	case "FAIL":
		return "ERROR"
	case "BLOCKED":
		return "WARNING"
	default:
		return "INFO"
	}
}

func logsRunNotFound(started time.Time, result contract.Result, data logsData) contract.Result {
	failure := prerequisiteError("RUN_NOT_FOUND", "The selected run does not exist in this project.", "Select an existing committed run ID.")
	result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "The selected run was not found.", data, failure)
	return result
}

func logsCancelled(started time.Time, result contract.Result, data logsData) contract.Result {
	failure := contract.Error{Code: "COMMAND_CANCELLED", Category: "cancelled", Message: "Structured run log reading was cancelled.", Retryable: true}
	result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitInterrupted, "Structured run log reading was cancelled.", data, failure)
	return result
}

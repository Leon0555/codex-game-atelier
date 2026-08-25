package app

import (
	"context"
	"errors"
	"time"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

type cleanRunCounts struct {
	Committed  int `json:"committed"`
	Incomplete int `json:"incomplete"`
	Orphan     int `json:"orphan"`
	Corrupt    int `json:"corrupt"`
}

type cleanRunEntry struct {
	RunID  string `json:"run_id"`
	State  string `json:"state"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type cleanData struct {
	Scope      string          `json:"scope"`
	StatePath  string          `json:"state_path"`
	Scanned    bool            `json:"scanned"`
	Counts     cleanRunCounts  `json:"counts"`
	Candidates []cleanRunEntry `json:"candidates"`
	Protected  []cleanRunEntry `json:"protected"`
}

func emptyCleanData() cleanData {
	return cleanData{
		Scope:      "runs",
		StatePath:  ".gameatelier/runs",
		Candidates: []cleanRunEntry{},
		Protected:  []cleanRunEntry{},
	}
}

func runClean(ctx context.Context, started time.Time, args []string) contract.Result {
	set := newFlagSet("clean")
	project := set.String("project", ".", "project directory")
	list := set.Bool("list", false, "list cleanup candidates without deleting them")
	if err := rejectDuplicateFlags(args); err != nil {
		return parseError(started, "clean", err.Error(), map[string]any{})
	}
	if err := set.Parse(args); err != nil || set.NArg() != 0 || *project == "" || !*list {
		return parseError(started, "clean", "clean currently requires --list and accepts --project only", map[string]any{})
	}

	result := contract.NewResult(started, contract.Command{Name: "clean", Arguments: map[string]any{"project": *project, "list": true}})
	data := emptyCleanData()
	if err := ctx.Err(); err != nil {
		return cleanCancelled(started, result, data)
	}
	root, err := canonicalProjectRoot(*project)
	if err != nil {
		failure := contract.Error{Code: "RUN_SCAN_FAILED", Category: "state", Message: "The requested project directory cannot be resolved safely.", Retryable: false}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Run records could not be scanned.", data, failure)
		return result
	}
	stateRoot, exists, err := openExistingStateRoot(root)
	if err != nil {
		failure := contract.Error{Code: "RUN_SCAN_UNSAFE", Category: "state", Message: "The project state root could not be opened safely.", Retryable: false}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Run records could not be scanned safely.", data, failure)
		return result
	}
	if !exists {
		failure := prerequisiteError("PROJECT_NOT_INITIALIZED", "The project has no .gameatelier state directory.", "Run initialize before listing run cleanup candidates.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Codex Game Atelier project state is not initialized.", data, failure)
		return result
	}
	defer stateRoot.Close()

	state, initialized, _, err := loadExistingState(stateRoot)
	if err != nil {
		failure := contract.Error{Code: "RUN_SCAN_UNSAFE", Category: "state", Message: "The project state is invalid or unsafe.", Retryable: false}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Run records could not be scanned safely.", data, failure)
		return result
	}
	if !initialized {
		failure := prerequisiteError("PROJECT_NOT_INITIALIZED", "The project has no .gameatelier/project.json state file.", "Run initialize before listing run cleanup candidates.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Codex Game Atelier project state is not initialized.", data, failure)
		return result
	}

	scan, err := scanRuns(ctx, stateRoot, state)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return cleanCancelled(started, result, data)
		}
		if errors.Is(err, errRunScanBudgetExceeded) {
			failure := contract.Error{Code: "RUN_SCAN_LIMIT_EXCEEDED", Category: "state", Message: "The run store exceeds the bounded scan work budget.", Retryable: false}
			result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Run records exceed the bounded scan budget.", data, failure)
			return result
		}
		failure := contract.Error{Code: "RUN_SCAN_UNSAFE", Category: "state", Message: "The run store contains an unsafe or unbounded directory structure.", Retryable: false}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Run records could not be scanned safely.", data, failure)
		return result
	}
	data.Scanned = true
	data.Counts = scan.Counts
	data.Candidates = scan.Candidates
	data.Protected = scan.Protected
	result.Finish(started, time.Now().UTC(), "PASS", contract.ExitOK, "Run cleanup candidates were listed without modifying the project.", data)
	return result
}

func cleanCancelled(started time.Time, result contract.Result, data cleanData) contract.Result {
	failure := contract.Error{Code: "COMMAND_CANCELLED", Category: "cancelled", Message: "The run scan was cancelled.", Retryable: true}
	result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitInterrupted, "Run cleanup candidate scanning was cancelled.", data, failure)
	return result
}

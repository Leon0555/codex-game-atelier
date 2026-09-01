package app

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

type releaseCheck struct {
	ID      string `json:"id"`
	Outcome string `json:"outcome"`
	Summary string `json:"summary"`
}

type releaseCheckCounts struct {
	Passed  int `json:"passed"`
	Blocked int `json:"blocked"`
	NotRun  int `json:"not_run"`
}

type releaseCheckData struct {
	Scope           string             `json:"scope"`
	SelectedMode    string             `json:"selected_mode"`
	ProjectMode     string             `json:"project_mode,omitempty"`
	ProjectRevision *int64             `json:"project_revision,omitempty"`
	ReleaseReady    bool               `json:"release_ready"`
	Checks          []releaseCheck     `json:"checks"`
	Counts          releaseCheckCounts `json:"counts"`
}

func runRelease(ctx context.Context, started time.Time, args []string) contract.Result {
	if len(args) == 0 || args[0] != "check" {
		return parseError(started, "release check", "release requires the check subcommand", map[string]any{})
	}
	return runReleaseCheck(ctx, started, args[1:])
}

func runReleaseCheck(ctx context.Context, started time.Time, args []string) contract.Result {
	set := newFlagSet("release check")
	project := set.String("project", ".", "project directory")
	mode := set.String("mode", "", "gate mode override")
	distributionCandidate := set.String("distribution-candidate", "", "verified local distribution candidate")
	releaseEvidence := set.String("release-evidence", "", "bound external release evidence")
	if err := rejectDuplicateFlags(args); err != nil {
		return parseError(started, "release check", err.Error(), map[string]any{})
	}
	if err := set.Parse(args); err != nil || set.NArg() != 0 || *project == "" || !validOptionalGateMode(*mode) || *distributionCandidate != "" && *mode != "strict" || *releaseEvidence != "" && (*mode != "strict" || *distributionCandidate == "") {
		return parseError(started, "release check", "release check accepts --project, --mode manual|standard|strict, and --distribution-candidate plus optional --release-evidence with explicit strict mode only", map[string]any{})
	}

	selectedMode := *mode
	commandArguments := map[string]any{"project": "."}
	if selectedMode != "" {
		commandArguments["mode"] = selectedMode
	}
	if *distributionCandidate != "" {
		commandArguments["distribution_candidate"] = "provided"
	}
	if *releaseEvidence != "" {
		commandArguments["release_evidence"] = "provided"
	}
	result := contract.NewResult(started, contract.Command{Name: "release check", Arguments: commandArguments})
	data := emptyReleaseCheckData(selectedMode)
	if err := ctx.Err(); err != nil {
		return releaseCheckCancelled(started, result, data)
	}
	root, err := canonicalProjectRoot(*project)
	if err != nil {
		failure := contract.Error{Code: "RELEASE_CHECK_FAILED", Category: "state", Message: "The requested project directory cannot be resolved safely.", Retryable: false}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Release readiness could not be checked.", data, failure)
		return result
	}
	stateRoot, exists, err := openExistingStateRoot(root)
	if err != nil {
		failure := contract.Error{Code: "RELEASE_CHECK_UNSAFE", Category: "state", Message: "The project state root could not be opened safely.", Retryable: false}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Release readiness could not be checked safely.", data, failure)
		return result
	}
	if !exists {
		failure := prerequisiteError("PROJECT_NOT_INITIALIZED", "The project has no .gameatelier state directory.", "Run initialize before checking release readiness.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Codex Game Atelier project state is not initialized.", data, failure)
		return result
	}
	defer stateRoot.Close()

	state, initialized, _, err := loadExistingState(stateRoot)
	if err != nil {
		failure := contract.Error{Code: "RELEASE_CHECK_UNSAFE", Category: "state", Message: "The project state is invalid or unsafe.", Retryable: false}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Release readiness could not be checked safely.", data, failure)
		return result
	}
	if !initialized {
		failure := prerequisiteError("PROJECT_NOT_INITIALIZED", "The project has no .gameatelier/project.json state file.", "Run initialize before checking release readiness.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Codex Game Atelier project state is not initialized.", data, failure)
		return result
	}
	if selectedMode == "" {
		selectedMode = state.Mode
		result.Command.Arguments["mode"] = selectedMode
		data.SelectedMode = selectedMode
	}
	revision := int64(state.Revision)
	data.ProjectMode = state.Mode
	data.ProjectRevision = &revision
	data.Checks = append(data.Checks,
		releaseCheck{ID: "project-state", Outcome: "PASS", Summary: "The project state satisfies the v1 contract."},
		releaseCheck{ID: "support-scope", Outcome: "PASS", Summary: "The project targets Godot 4.7.2-stable standard/GDScript within the v1 scope."},
	)
	if selectedMode == "manual" {
		return finishReleaseCheck(started, result, data)
	}

	scan, err := scanRunsVerified(ctx, stateRoot, state)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return releaseCheckCancelled(started, result, data)
		}
		code := "RUN_SCAN_UNSAFE"
		message := "The run store contains an unsafe or unbounded directory structure."
		if errors.Is(err, errRunScanBudgetExceeded) {
			code = "RUN_SCAN_LIMIT_EXCEEDED"
			message = "The run store exceeds the bounded scan work budget."
		}
		failure := contract.Error{Code: code, Category: "state", Message: message, Retryable: false}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Release evidence could not be scanned safely.", data, failure)
		return result
	}
	storeOutcome := "PASS"
	storeSummary := "Every scanned run has a committed and verified closure."
	if scan.Counts.Incomplete+scan.Counts.Orphan+scan.Counts.Corrupt > 0 {
		storeOutcome = "BLOCKED"
		storeSummary = "The run store contains incomplete, orphan, or corrupt records."
	}
	data.Checks = append(data.Checks, releaseCheck{ID: "run-store-integrity", Outcome: storeOutcome, Summary: storeSummary})
	evidenceChecks, releaseRun := currentEvidenceChecks(scan.Verified, revision)
	if releaseRun != nil {
		verified, verifyErr := verifyCurrentReleaseArtifact(ctx, stateRoot, *releaseRun)
		if ctx.Err() != nil {
			return releaseCheckCancelled(started, result, data)
		}
		if verifyErr != nil || !verified {
			evidenceChecks[len(evidenceChecks)-1] = releaseCheck{ID: "latest-release-export", Outcome: "BLOCKED", Summary: "The latest PASS release export artifact is missing, unsafe, or no longer matches its committed manifest."}
		}
	}
	data.Checks = append(data.Checks, evidenceChecks...)

	if selectedMode == "strict" {
		distributionChecks, verifyErr := strictDistributionChecks(ctx, *distributionCandidate)
		if verifyErr != nil && ctx.Err() != nil {
			return releaseCheckCancelled(started, result, data)
		}
		data.Checks = append(data.Checks, distributionChecks...)
		externalChecks, externalErr := externalReleaseChecks(ctx, *releaseEvidence, *distributionCandidate)
		if externalErr != nil && ctx.Err() != nil {
			return releaseCheckCancelled(started, result, data)
		}
		data.Checks = append(data.Checks, externalChecks...)
	}
	return finishReleaseCheck(started, result, data)
}

func strictDistributionChecks(ctx context.Context, candidate string) ([]releaseCheck, error) {
	ids := []string{"clean-source-policy", "plugin-bundle", "starter-package", "license-and-provenance"}
	if candidate == "" {
		summaries := []string{
			"No explicit local distribution candidate was provided for strict clean-source verification.",
			"No explicit local distribution candidate was provided for strict Plugin verification.",
			"No explicit local distribution candidate was provided for strict embedded Starter verification.",
			"No explicit local distribution candidate was provided for strict license and provenance verification.",
		}
		checks := make([]releaseCheck, len(ids))
		for index := range ids {
			checks[index] = releaseCheck{ID: ids[index], Outcome: "NOT_RUN", Summary: summaries[index]}
		}
		return checks, nil
	}
	err := verifyReleaseDistributionCandidate(ctx, candidate)
	outcome := "PASS"
	summaries := []string{
		"The candidate contains verified clean Git and Go build provenance.",
		"The bounded Plugin archive, inventory, targets, and build records passed.",
		"The embedded Starter inventory and Plugin version pairing passed.",
		"Project and Go notices, checksums, and cross-manifest provenance passed.",
	}
	if err != nil {
		outcome = "BLOCKED"
		for index := range summaries {
			summaries[index] = "The provided local distribution candidate did not pass the fixed strict verification contract."
		}
	}
	checks := make([]releaseCheck, len(ids))
	for index := range ids {
		checks[index] = releaseCheck{ID: ids[index], Outcome: outcome, Summary: summaries[index]}
	}
	return checks, err
}

func emptyReleaseCheckData(mode string) releaseCheckData {
	return releaseCheckData{Scope: "project-release", SelectedMode: mode, Checks: []releaseCheck{}}
}

func validOptionalGateMode(mode string) bool {
	return mode == "" || mode == "manual" || mode == "standard" || mode == "strict"
}

func currentEvidenceChecks(runs []verifiedRun, revision int64) ([]releaseCheck, *verifiedRun) {
	requirements := []struct {
		id      string
		summary string
		match   func(contract.Command) bool
	}{
		{id: "latest-headless-validation", summary: "A PASS headless validation exists for the current project revision.", match: func(command contract.Command) bool {
			return command.Name == "validate" && command.Arguments["headless"] == true
		}},
		{id: "latest-fixed-gdscript-tests", summary: "A PASS fixed GDScript test run exists for the current project revision.", match: func(command contract.Command) bool {
			return command.Name == "test"
		}},
		{id: "latest-release-export", summary: "A PASS release export exists for the current project revision.", match: func(command contract.Command) bool {
			return command.Name == "export" && command.Arguments["profile"] == "release"
		}},
	}
	checks := make([]releaseCheck, 0, len(requirements))
	var releaseRun *verifiedRun
	for _, requirement := range requirements {
		outcome := "BLOCKED"
		summary := "No verified PASS evidence exists for this gate at the current project revision."
		var latest *verifiedRun
		var latestFinished time.Time
		for index := range runs {
			run := &runs[index]
			if run.ProjectRevision != revision || !requirement.match(run.Result.Command) {
				continue
			}
			finished, err := time.Parse(time.RFC3339Nano, run.Result.FinishedAt)
			if err == nil && (latest == nil || finished.After(latestFinished) || finished.Equal(latestFinished) && run.Result.RunID > latest.Result.RunID) {
				latest = run
				latestFinished = finished
			}
		}
		if latest != nil && latest.Result.Outcome == "PASS" {
			outcome = "PASS"
			summary = requirement.summary
			if requirement.id == "latest-release-export" {
				releaseRun = latest
			}
		} else if latest != nil {
			summary = "The latest verified evidence for this gate at the current project revision did not pass."
		}
		checks = append(checks, releaseCheck{ID: requirement.id, Outcome: outcome, Summary: summary})
	}
	return checks, releaseRun
}

func verifyCurrentReleaseArtifact(ctx context.Context, stateRoot *os.Root, run verifiedRun) (bool, error) {
	if stateRoot == nil || run.Result.Command.Name != "export" || run.Result.Command.Arguments["profile"] != "release" || run.PayloadKind != "export-artifact" {
		return false, nil
	}
	var manifest exportArtifactManifest
	if err := decodeStrictRunJSON(run.Payload, &manifest); err != nil || manifest.Artifact == nil {
		return false, nil
	}
	artifacts, exists, err := openExistingVerifiedDirectory(stateRoot, "artifacts")
	if err != nil || !exists {
		return false, err
	}
	defer artifacts.Close()
	artifactRoot, exists, err := openExistingVerifiedDirectory(artifacts, run.Result.RunID)
	if err != nil || !exists {
		return false, err
	}
	defer artifactRoot.Close()
	observed, err := inspectExportArtifact(ctx, artifactRoot, "game-release.zip", manifest.Artifact.Path)
	if err != nil {
		return false, err
	}
	return observed.Path == manifest.Artifact.Path && observed.MediaType == manifest.Artifact.MediaType && observed.SHA256 == manifest.Artifact.SHA256 && observed.ByteSize == manifest.Artifact.ByteSize, nil
}

func finishReleaseCheck(started time.Time, result contract.Result, data releaseCheckData) contract.Result {
	for _, check := range data.Checks {
		switch check.Outcome {
		case "PASS":
			data.Counts.Passed++
		case "BLOCKED":
			data.Counts.Blocked++
		default:
			data.Counts.NotRun++
		}
	}
	data.ReleaseReady = data.SelectedMode == "strict" && data.Counts.Blocked == 0 && data.Counts.NotRun == 0
	if data.Counts.Blocked == 0 && data.Counts.NotRun == 0 {
		summary := "Selected release checks passed without modifying the project."
		if !data.ReleaseReady {
			summary = "Selected release checks passed; strict release readiness was not requested."
		}
		result.Finish(started, time.Now().UTC(), "PASS", contract.ExitOK, summary, data)
		return result
	}
	failure := prerequisiteError("RELEASE_CHECK_INCOMPLETE", "One or more required release gates are blocked or not implemented.", "Resolve every reported gate and rerun release check.")
	result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Release readiness checks are incomplete.", data, failure)
	return result
}

func releaseCheckCancelled(started time.Time, result contract.Result, data releaseCheckData) contract.Result {
	failure := contract.Error{Code: "COMMAND_CANCELLED", Category: "cancelled", Message: "The release check was cancelled.", Retryable: true}
	result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitInterrupted, "Release readiness checking was cancelled.", data, failure)
	return result
}

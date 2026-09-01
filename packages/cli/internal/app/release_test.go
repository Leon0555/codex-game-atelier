package app

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

func TestReleaseCheckManualIsReadOnlyAndNotReleaseReady(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, _ := createRunStoreProject(t, "release manual 只读")
	stateRoot.Close()
	before := snapshotTree(t, filepath.Join(project, ".gameatelier"))
	code, result, _, stderr := execute(t, context.Background(), "release", "check", "--project", project, "--mode", "manual")
	after := snapshotTree(t, filepath.Join(project, ".gameatelier"))
	if code != contract.ExitOK || result.Outcome != "PASS" || len(result.Evidence) != 0 || stderr != "" {
		t.Fatalf("manual release check failed: code=%d result=%+v stderr=%q", code, result, stderr)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("manual release check modified the project:\nbefore=%+v\nafter=%+v", before, after)
	}
	data := resultDataMap(t, result)
	if data["scope"] != "project-release" || data["selected_mode"] != "manual" || data["project_mode"] != "standard" || data["release_ready"] != false {
		t.Fatalf("unexpected manual release data: %#v", data)
	}
	assertReleaseCheckIDs(t, result, "project-state", "support-scope")
}

func TestReleaseCheckDefaultsToProjectModeAndBlocksMissingEvidence(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, _ := createRunStoreProject(t, "release standard")
	stateRoot.Close()
	before := snapshotTree(t, filepath.Join(project, ".gameatelier"))
	code, result, _, _ := execute(t, context.Background(), "release", "check", "--project", project)
	after := snapshotTree(t, filepath.Join(project, ".gameatelier"))
	if code != contract.ExitPrerequisite || result.Outcome != "BLOCKED" || firstErrorCode(result) != "RELEASE_CHECK_INCOMPLETE" || !reflect.DeepEqual(before, after) {
		t.Fatalf("standard release check was not a read-only block: code=%d result=%+v", code, result)
	}
	if result.Command.Arguments["mode"] != "standard" {
		t.Fatalf("project mode was not resolved into the result: %+v", result.Command.Arguments)
	}
	assertReleaseCheckIDs(t, result, "project-state", "support-scope", "run-store-integrity", "latest-headless-validation", "latest-fixed-gdscript-tests", "latest-release-export")
	data := resultDataMap(t, result)
	counts := data["counts"].(map[string]any)
	if data["release_ready"] != false || counts["passed"] != float64(3) || counts["blocked"] != float64(3) || counts["not_run"] != float64(0) {
		t.Fatalf("unexpected standard release counts: %#v", data)
	}
}

func TestReleaseCheckStrictReportsDeferredDistributionGates(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, _ := createRunStoreProject(t, "release strict")
	stateRoot.Close()
	code, result, _, _ := execute(t, context.Background(), "release", "check", "--project", project, "--mode", "strict")
	if code != contract.ExitPrerequisite || result.Outcome != "BLOCKED" || firstErrorCode(result) != "RELEASE_CHECK_INCOMPLETE" {
		t.Fatalf("strict release check did not block honestly: code=%d result=%+v", code, result)
	}
	assertReleaseCheckIDs(t, result, "project-state", "support-scope", "run-store-integrity", "latest-headless-validation", "latest-fixed-gdscript-tests", "latest-release-export", "clean-source-policy", "plugin-bundle", "starter-package", "license-and-provenance", "remote-plugin-install", "required-ci")
	data := resultDataMap(t, result)
	counts := data["counts"].(map[string]any)
	if counts["not_run"] != float64(6) || data["release_ready"] != false {
		t.Fatalf("strict deferred gates were not explicit: %#v", data)
	}
}

func TestReleaseCheckStandardPassesOnlyWithCurrentVerifiedClosuresAndArtifact(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, state := createRunStoreProject(t, "release standard pass")
	commitReleaseCheckPrerequisites(t, project, stateRoot, state)
	stateRoot.Close()
	before := snapshotTree(t, filepath.Join(project, ".gameatelier"))
	code, result, _, stderr := execute(t, context.Background(), "release", "check", "--project", project, "--mode", "standard")
	after := snapshotTree(t, filepath.Join(project, ".gameatelier"))
	if code != contract.ExitOK || result.Outcome != "PASS" || stderr != "" || !reflect.DeepEqual(before, after) {
		t.Fatalf("standard release check did not pass read-only: code=%d result=%+v stderr=%q", code, result, stderr)
	}
	data := resultDataMap(t, result)
	counts := data["counts"].(map[string]any)
	if data["release_ready"] != false || counts["passed"] != float64(6) || counts["blocked"] != float64(0) || counts["not_run"] != float64(0) {
		t.Fatalf("unexpected standard PASS data: %#v", data)
	}

	exportRunID := latestReleaseExportRunID(t, project)
	artifact := filepath.Join(project, ".gameatelier", "artifacts", exportRunID, "game-release.zip")
	if err := os.WriteFile(artifact, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, result, _, _ = execute(t, context.Background(), "release", "check", "--project", project, "--mode", "standard")
	if code != contract.ExitPrerequisite || result.Outcome != "BLOCKED" || firstErrorCode(result) != "RELEASE_CHECK_INCOMPLETE" {
		t.Fatalf("tampered release artifact was trusted: code=%d result=%+v", code, result)
	}
	checks := resultDataMap(t, result)["checks"].([]any)
	last := checks[len(checks)-1].(map[string]any)
	if last["id"] != "latest-release-export" || last["outcome"] != "BLOCKED" {
		t.Fatalf("artifact mismatch was not attributed to the release export gate: %#v", last)
	}
}

func TestReleaseCheckStrictConsumesCandidateButKeepsRequiredCIBlocked(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, state := createRunStoreProject(t, "release strict candidate")
	commitReleaseCheckPrerequisites(t, project, stateRoot, state)
	stateRoot.Close()
	original := verifyReleaseDistributionCandidate
	verifyReleaseDistributionCandidate = func(context.Context, string) error { return nil }
	defer func() { verifyReleaseDistributionCandidate = original }()

	before := snapshotTree(t, filepath.Join(project, ".gameatelier"))
	code, result, _, stderr := execute(t, context.Background(), "release", "check", "--project", project, "--mode", "strict", "--distribution-candidate", "/private/candidate/path")
	after := snapshotTree(t, filepath.Join(project, ".gameatelier"))
	if code != contract.ExitPrerequisite || result.Outcome != "BLOCKED" || stderr != "" || !reflect.DeepEqual(before, after) {
		t.Fatalf("strict candidate check did not remain a read-only required-CI block: code=%d result=%+v stderr=%q", code, result, stderr)
	}
	if result.Command.Arguments["distribution_candidate"] != "provided" {
		t.Fatalf("candidate presence was not recorded without disclosing its path: %+v", result.Command.Arguments)
	}
	data := resultDataMap(t, result)
	counts := data["counts"].(map[string]any)
	if data["release_ready"] != false || counts["passed"] != float64(10) || counts["blocked"] != float64(0) || counts["not_run"] != float64(2) {
		t.Fatalf("strict local candidate bypassed required CI or lost a PASS gate: %#v", data)
	}
	checks := data["checks"].([]any)
	for _, index := range []int{6, 7, 8, 9} {
		if checks[index].(map[string]any)["outcome"] != "PASS" {
			t.Fatalf("local distribution gate %d did not pass: %#v", index, checks)
		}
	}
	if last := checks[len(checks)-1].(map[string]any); last["id"] != "required-ci" || last["outcome"] != "NOT_RUN" {
		t.Fatalf("required CI gate changed unexpectedly: %#v", last)
	}
}

func TestReleaseCheckStrictConsumesBoundExternalEvidenceReadOnly(t *testing.T) {
	requireInitializePlatform(t)
	project, stateRoot, state := createRunStoreProject(t, "release strict external evidence")
	commitReleaseCheckPrerequisites(t, project, stateRoot, state)
	stateRoot.Close()
	originalCandidate := verifyReleaseDistributionCandidate
	originalEvidence := verifyReleaseEvidenceManifest
	verifyReleaseDistributionCandidate = func(context.Context, string) error { return nil }
	verifyReleaseEvidenceManifest = func(context.Context, string, string) error { return nil }
	defer func() {
		verifyReleaseDistributionCandidate = originalCandidate
		verifyReleaseEvidenceManifest = originalEvidence
	}()

	before := snapshotTree(t, filepath.Join(project, ".gameatelier"))
	code, result, stdout, stderr := execute(t, context.Background(), "release", "check", "--project", project, "--mode", "strict", "--distribution-candidate", "/private/candidate/path", "--release-evidence", "/private/evidence/path")
	after := snapshotTree(t, filepath.Join(project, ".gameatelier"))
	if code != contract.ExitOK || result.Outcome != "PASS" || stderr != "" || !reflect.DeepEqual(before, after) {
		t.Fatalf("strict bound evidence check failed or wrote state: code=%d result=%+v stderr=%q", code, result, stderr)
	}
	if result.Command.Arguments["distribution_candidate"] != "provided" || result.Command.Arguments["release_evidence"] != "provided" || result.Command.Arguments["mode"] != "strict" {
		t.Fatalf("strict input presence was not recorded symbolically: %+v", result.Command.Arguments)
	}
	if strings.Contains(stdout, "/private/candidate/path") || strings.Contains(stdout, "/private/evidence/path") {
		t.Fatalf("strict result disclosed an input path: %s", stdout)
	}
	data := resultDataMap(t, result)
	counts := data["counts"].(map[string]any)
	if data["release_ready"] != true || counts["passed"] != float64(12) || counts["blocked"] != float64(0) || counts["not_run"] != float64(0) {
		t.Fatalf("strict bound evidence did not close all release gates: %#v", data)
	}
}

func TestCurrentReleaseEvidenceRequiresCurrentRevisionAndExactCommands(t *testing.T) {
	runs := []verifiedRun{
		{ProjectRevision: 4, Result: contract.Result{Outcome: "PASS", FinishedAt: "2026-08-29T01:00:00Z", Command: contract.Command{Name: "validate", Arguments: map[string]any{"headless": true}}}},
		{ProjectRevision: 4, Result: contract.Result{Outcome: "PASS", FinishedAt: "2026-08-29T01:00:01Z", Command: contract.Command{Name: "test", Arguments: map[string]any{}}}},
		{ProjectRevision: 4, Result: contract.Result{Outcome: "PASS", FinishedAt: "2026-08-29T01:00:02Z", Command: contract.Command{Name: "export", Arguments: map[string]any{"profile": "release"}}}},
		{ProjectRevision: 3, Result: contract.Result{Outcome: "PASS", FinishedAt: "2026-08-29T02:00:00Z", Command: contract.Command{Name: "export", Arguments: map[string]any{"profile": "release"}}}},
		{ProjectRevision: 4, Result: contract.Result{Outcome: "FAIL", FinishedAt: "2026-08-29T00:59:00Z", Command: contract.Command{Name: "validate", Arguments: map[string]any{"headless": true}}}},
	}
	checks, releaseRun := currentEvidenceChecks(runs, 4)
	for _, check := range checks {
		if check.Outcome != "PASS" {
			t.Fatalf("current exact evidence was not accepted: %+v", checks)
		}
	}
	if releaseRun == nil || releaseRun.Result.Command.Name != "export" {
		t.Fatalf("latest release export was not selected: %+v", releaseRun)
	}
	checks, releaseRun = currentEvidenceChecks(runs, 5)
	for _, check := range checks {
		if check.Outcome != "BLOCKED" {
			t.Fatalf("historical evidence was accepted for a future revision: %+v", checks)
		}
	}
	if releaseRun != nil {
		t.Fatalf("future revision selected an export: %+v", releaseRun)
	}
}

func TestCurrentReleaseEvidenceUsesLatestResultNotAnyHistoricalPass(t *testing.T) {
	runs := []verifiedRun{
		{ProjectRevision: 0, Result: contract.Result{Outcome: "PASS", FinishedAt: "2026-08-29T01:00:00Z", Command: contract.Command{Name: "test", Arguments: map[string]any{}}}},
		{ProjectRevision: 0, Result: contract.Result{Outcome: "FAIL", FinishedAt: "2026-08-29T02:00:00Z", Command: contract.Command{Name: "test", Arguments: map[string]any{}}}},
	}
	checks, _ := currentEvidenceChecks(runs, 0)
	if checks[1].Outcome != "BLOCKED" {
		t.Fatalf("older PASS overrode the latest failure: %+v", checks)
	}
}

func TestReleaseCheckRejectsInvalidInvocationAndCancellation(t *testing.T) {
	code, result, _, _ := execute(t, context.Background(), "release")
	if code != contract.ExitUsage || firstErrorCode(result) != "INVALID_ARGUMENT" {
		t.Fatalf("release without check was accepted: code=%d result=%+v", code, result)
	}
	code, result, _, _ = execute(t, context.Background(), "release", "check", "--mode", "unsafe")
	if code != contract.ExitUsage || firstErrorCode(result) != "INVALID_ARGUMENT" {
		t.Fatalf("invalid release mode was accepted: code=%d result=%+v", code, result)
	}
	code, result, _, _ = execute(t, context.Background(), "release", "check", "--mode", "standard", "--distribution-candidate", "candidate")
	if code != contract.ExitUsage || firstErrorCode(result) != "INVALID_ARGUMENT" {
		t.Fatalf("distribution candidate was accepted outside explicit strict mode: code=%d result=%+v", code, result)
	}
	code, result, _, _ = execute(t, context.Background(), "release", "check", "--mode", "strict", "--release-evidence", "evidence.json")
	if code != contract.ExitUsage || firstErrorCode(result) != "INVALID_ARGUMENT" {
		t.Fatalf("release evidence was accepted without a distribution candidate: code=%d result=%+v", code, result)
	}
	code, result, _, _ = execute(t, context.Background(), "release", "check", "--mode", "standard", "--distribution-candidate", "candidate", "--release-evidence", "evidence.json")
	if code != contract.ExitUsage || firstErrorCode(result) != "INVALID_ARGUMENT" {
		t.Fatalf("release evidence was accepted outside explicit strict mode: code=%d result=%+v", code, result)
	}

	requireInitializePlatform(t)
	project, stateRoot, _ := createRunStoreProject(t, "release cancelled")
	stateRoot.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code, result, _, _ = execute(t, ctx, "release", "check", "--project", project)
	if code != contract.ExitInterrupted || firstErrorCode(result) != "COMMAND_CANCELLED" || len(result.Evidence) != 0 {
		t.Fatalf("cancelled release check returned an unexpected result: code=%d result=%+v", code, result)
	}
}

func assertReleaseCheckIDs(t *testing.T, result contract.Result, expected ...string) {
	t.Helper()
	data := resultDataMap(t, result)
	items, ok := data["checks"].([]any)
	if !ok || len(items) != len(expected) {
		t.Fatalf("unexpected release checks: %#v", data["checks"])
	}
	for index, id := range expected {
		item, ok := items[index].(map[string]any)
		if !ok || item["id"] != id {
			t.Fatalf("release check %d=%#v, want %q", index, items[index], id)
		}
	}
}

func commitReleaseCheckPrerequisites(t *testing.T, project string, stateRoot *os.Root, state projectState) {
	t.Helper()
	started := time.Now().UTC().Add(-3 * time.Second)
	validateCommand := contract.Command{Name: "validate", Arguments: validateOptions{headless: true, explicitGodot: true, timeoutMS: defaultHeadlessTimeoutMS, allowEngineUserData: true}.persistedArguments()}
	validateResult := contract.NewResult(started, validateCommand)
	validateChecks := []baselineValidationCheck{{ID: "headless-scene", Outcome: "PASS", Summary: "The main scene completed one headless frame."}}
	validateResult.Finish(started, started.Add(10*time.Millisecond), "PASS", contract.ExitOK, "Godot headless project validation passed.", map[string]any{"scope": "headless", "check_count": 1})
	if commit := finishRunForTest(stateRoot, state, validateResult, []runPayload{makeValidationPayload("headless", "PASS", validateChecks)}, nil); commit.Err != nil || !commit.Committed {
		t.Fatalf("headless prerequisite did not commit: %+v", commit)
	}

	started = time.Now().UTC().Add(-2 * time.Second)
	testCommand := contract.Command{Name: "test", Arguments: testOptions{explicitGodot: true, timeoutMS: defaultHeadlessTimeoutMS, allowEngineUserData: true}.persistedArguments()}
	testResult := contract.NewResult(started, testCommand)
	testResult, testPayload := finishTestResult(started, testResult, "PASS", contract.ExitOK, "GDScript tests passed.", "4.7.2.stable.official.ed1daf0bf", []gdscriptTestCase{{ID: "release-check", Outcome: "PASS", Summary: "The fixed test passed."}})
	if commit := finishRunForTest(stateRoot, state, testResult, []runPayload{testPayload}, nil); commit.Err != nil || !commit.Committed {
		t.Fatalf("test prerequisite did not commit: %+v", commit)
	}

	started = time.Now().UTC().Add(-time.Second)
	exportCommand := contract.Command{Name: "export", Arguments: exportOptions{profile: "release", preset: defaultMacOSExportPreset, explicitGodot: true, timeoutMS: defaultExportTimeoutMS, allowEngineUserData: true}.persistedArguments()}
	exportResult := contract.NewResult(started, exportCommand)
	artifactDirectory := filepath.Join(project, ".gameatelier", "artifacts", exportResult.RunID)
	if err := os.MkdirAll(artifactDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	archiveBytes, err := os.ReadFile(createMacOSExportArchive(t))
	if err != nil {
		t.Fatal(err)
	}
	artifactPath := filepath.Join(artifactDirectory, "game-release.zip")
	if err := os.WriteFile(artifactPath, archiveBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	artifactRoot, err := os.OpenRoot(artifactDirectory)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := inspectExportArtifact(context.Background(), artifactRoot, "game-release.zip", ".gameatelier/artifacts/"+exportResult.RunID+"/game-release.zip")
	artifactRoot.Close()
	if err != nil {
		t.Fatal(err)
	}
	artifact.TargetSmoke = &exportTargetSmoke{Host: "macos", Arch: "arm64", Mode: "headless-one-frame", ExitCode: 0}
	exportResult, exportPayload := finishExportResult(started, exportResult, "PASS", contract.ExitOK, "Godot macOS technical export passed.", "4.7.2.stable.official.ed1daf0bf", artifact)
	if commit := finishRunForTest(stateRoot, state, exportResult, []runPayload{exportPayload}, nil); commit.Err != nil || !commit.Committed {
		t.Fatalf("export prerequisite did not commit: %+v", commit)
	}
}

func latestReleaseExportRunID(t *testing.T, project string) string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(project, ".gameatelier", "artifacts"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("unexpected artifact directories: entries=%v err=%v", entries, err)
	}
	return entries[0].Name()
}

func setProjectMode(t *testing.T, project, mode string) {
	t.Helper()
	path := filepath.Join(project, ".gameatelier", "project.json")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	state, err := decodeProjectState(content)
	if err != nil {
		t.Fatal(err)
	}
	state.Mode = mode
	content, err = marshalState(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
}

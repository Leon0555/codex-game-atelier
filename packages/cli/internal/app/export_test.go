package app

import (
	"archive/zip"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

func TestExportCommitsVerifiedArtifactAndScannerClosure(t *testing.T) {
	requireMacOSAppleSilicon(t)
	stubSuccessfulTargetSmoke(t)
	project, stateRoot, state := createRunStoreProject(t, "导出 项目 🚀")
	setProjectMode(t, project, "manual")
	writeMacOSExportPreset(t, project)
	archive := createMacOSExportArchive(t)
	godot := createExportGodot(t, archive, "printf 'uid://generated\\n' > generated.gd.uid")
	writeExportTemplateFixture(t, godot, true)

	code, result, _, stderr := execute(t, context.Background(), "export", "--project", project, "--profile", "release", "--godot", godot, "--allow-engine-user-data", "--timeout-ms", "10000")
	if code != contract.ExitOK || result.Outcome != "PASS" || stderr != "" || len(result.Evidence) != 1 {
		t.Fatalf("export failed: code=%d result=%+v stderr=%q", code, result, stderr)
	}
	data := resultDataMap(t, result)
	if data["scope"] != "godot-export" || data["target"] != "macos-universal2" || data["profile"] != "release" || data["artifact_count"] != float64(1) {
		t.Fatalf("unexpected export data: %+v", data)
	}
	artifactPath := filepath.Join(project, ".gameatelier", "artifacts", result.RunID, "game-release.zip")
	if info, err := os.Stat(artifactPath); err != nil || !info.Mode().IsRegular() || info.Size() < 1 {
		t.Fatalf("verified export artifact is missing: info=%v err=%v", info, err)
	}
	if _, err := os.Stat(filepath.Join(project, "generated.gd.uid")); !os.IsNotExist(err) {
		t.Fatalf("Godot wrote through the isolated project snapshot: %v", err)
	}
	scan, err := scanRuns(context.Background(), stateRoot, state)
	if err != nil || scan.Counts.Committed != 1 || scan.Counts.Corrupt != 0 || len(scan.Candidates) != 0 || len(scan.Protected) != 0 {
		t.Fatalf("export run closure did not verify: scan=%+v err=%v", scan, err)
	}
}

func TestBuildUsesTheExportPipelineWithoutAcceptingAPreset(t *testing.T) {
	requireMacOSAppleSilicon(t)
	stubSuccessfulTargetSmoke(t)
	project, stateRoot, state := createRunStoreProject(t, "build-wrapper")
	setProjectMode(t, project, "manual")
	writeMacOSExportPreset(t, project)
	godot := createExportGodot(t, createMacOSExportArchive(t), "")
	writeExportTemplateFixture(t, godot, true)

	code, result, _, stderr := execute(t, context.Background(), "build", "--project", project, "--profile", "debug", "--godot", godot, "--allow-engine-user-data", "--timeout-ms", "10000")
	if code != contract.ExitOK || result.Outcome != "PASS" || result.Command.Name != "build" || stderr != "" {
		t.Fatalf("build wrapper failed: code=%d result=%+v stderr=%q", code, result, stderr)
	}
	if info, err := os.Stat(filepath.Join(project, ".gameatelier", "artifacts", result.RunID, "game-debug.zip")); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("build artifact is missing: info=%v err=%v", info, err)
	}
	scan, err := scanRuns(context.Background(), stateRoot, state)
	if err != nil || scan.Counts.Committed != 1 || scan.Counts.Corrupt != 0 {
		t.Fatalf("build run closure did not verify: scan=%+v err=%v", scan, err)
	}

	code, invalid, _, _ := execute(t, context.Background(), "build", "--project", project, "--preset", defaultMacOSExportPreset)
	if code != contract.ExitUsage || firstErrorCode(invalid) != "INVALID_ARGUMENT" {
		t.Fatalf("build accepted the direct export preset control: code=%d result=%+v", code, invalid)
	}
}

func TestStandardExportRunsHeadlessAndFixedTestsBeforeExport(t *testing.T) {
	requireMacOSAppleSilicon(t)
	stubSuccessfulTargetSmoke(t)
	project, stateRoot, state := createRunStoreProject(t, "standard-gates")
	writeMacOSExportPreset(t, project)
	writeFixedTestRunner(t, project)
	exportMarker := filepath.Join(t.TempDir(), "export-started")
	godot := createPolicyAwareExportGodot(t, createMacOSExportArchive(t), exportMarker, true)
	writeExportTemplateFixture(t, godot, true)

	code, result, _, stderr := execute(t, context.Background(), "export", "--project", project, "--profile", "release", "--godot", godot, "--allow-engine-user-data", "--timeout-ms", "15000")
	if code != contract.ExitOK || result.Outcome != "PASS" || stderr != "" {
		t.Fatalf("standard gated export failed: code=%d result=%+v stderr=%q", code, result, stderr)
	}
	if _, err := os.Stat(exportMarker); err != nil {
		t.Fatalf("export action was not reached after passing gates: %v", err)
	}
	scan, err := scanRunsVerified(context.Background(), stateRoot, state)
	if err != nil || scan.Counts != (cleanRunCounts{Committed: 3}) {
		t.Fatalf("standard gate run closures were incomplete: scan=%+v err=%v", scan, err)
	}
	commands := map[string]int{}
	for _, run := range scan.Verified {
		commands[run.Result.Command.Name]++
	}
	if commands["validate"] != 1 || commands["test"] != 1 || commands["export"] != 1 {
		t.Fatalf("standard gates did not record exactly one workflow: %+v", commands)
	}
}

func TestArtifactModeOverrideIsRecordedWithoutMutatingProjectMode(t *testing.T) {
	requireMacOSAppleSilicon(t)
	stubSuccessfulTargetSmoke(t)
	project, stateRoot, state := createRunStoreProject(t, "manual-override")
	writeMacOSExportPreset(t, project)
	godot := createExportGodot(t, createMacOSExportArchive(t), "")
	writeExportTemplateFixture(t, godot, true)
	statePath := filepath.Join(project, ".gameatelier", "project.json")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}

	code, result, _, _ := execute(t, context.Background(), "export", "--project", project, "--profile", "release", "--mode", "manual", "--godot", godot, "--allow-engine-user-data", "--timeout-ms", "10000")
	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if code != contract.ExitOK || result.Outcome != "PASS" || result.Command.Arguments["mode"] != "manual" || string(before) != string(after) {
		t.Fatalf("manual override was not isolated to one command: code=%d result=%+v state_changed=%t", code, result, string(before) != string(after))
	}
	scan, err := scanRunsVerified(context.Background(), stateRoot, state)
	if err != nil || len(scan.Verified) != 1 || scan.Verified[0].PolicyMode != "manual" {
		t.Fatalf("manual override was not bound into run intent: scan=%+v err=%v", scan, err)
	}

	code, invalid, _, _ := execute(t, context.Background(), "build", "--project", project, "--mode", "unsafe")
	if code != contract.ExitUsage || firstErrorCode(invalid) != "INVALID_ARGUMENT" {
		t.Fatalf("invalid build/export mode was accepted: code=%d result=%+v", code, invalid)
	}
}

func TestStandardExportStopsAfterFailingFixedTests(t *testing.T) {
	requireMacOSAppleSilicon(t)
	project, stateRoot, state := createRunStoreProject(t, "standard-gate-failure")
	writeMacOSExportPreset(t, project)
	writeFixedTestRunner(t, project)
	exportMarker := filepath.Join(t.TempDir(), "export-started")
	godot := createPolicyAwareExportGodot(t, createMacOSExportArchive(t), exportMarker, false)
	writeExportTemplateFixture(t, godot, true)

	code, result, _, _ := execute(t, context.Background(), "export", "--project", project, "--profile", "release", "--godot", godot, "--allow-engine-user-data", "--timeout-ms", "15000")
	if code != contract.ExitValidation || result.Outcome != "FAIL" || firstErrorCode(result) != "POLICY_GATE_FAILED" || len(result.Evidence) != 1 {
		t.Fatalf("failing test gate was not propagated: code=%d result=%+v", code, result)
	}
	if _, err := os.Stat(exportMarker); !os.IsNotExist(err) {
		t.Fatalf("export action started after a failing test gate: %v", err)
	}
	scan, err := scanRunsVerified(context.Background(), stateRoot, state)
	if err != nil || scan.Counts != (cleanRunCounts{Committed: 3}) {
		t.Fatalf("failed gate workflow did not retain three committed facts: scan=%+v err=%v", scan, err)
	}
}

func TestStrictExportRunsStandardGatesThenBlocksDeferredGates(t *testing.T) {
	requireMacOSAppleSilicon(t)
	project, stateRoot, state := createRunStoreProject(t, "strict-gates")
	writeMacOSExportPreset(t, project)
	writeFixedTestRunner(t, project)
	exportMarker := filepath.Join(t.TempDir(), "export-started")
	godot := createPolicyAwareExportGodot(t, createMacOSExportArchive(t), exportMarker, true)
	writeExportTemplateFixture(t, godot, true)

	code, result, _, _ := execute(t, context.Background(), "export", "--project", project, "--profile", "release", "--mode", "strict", "--godot", godot, "--allow-engine-user-data", "--timeout-ms", "15000")
	if code != contract.ExitPrerequisite || result.Outcome != "BLOCKED" || firstErrorCode(result) != "STRICT_GATES_UNAVAILABLE" || len(result.Evidence) != 1 {
		t.Fatalf("strict deferred gates were not enforced: code=%d result=%+v", code, result)
	}
	if _, err := os.Stat(exportMarker); !os.IsNotExist(err) {
		t.Fatalf("strict export action started before strict gates were available: %v", err)
	}
	scan, err := scanRunsVerified(context.Background(), stateRoot, state)
	if err != nil || scan.Counts != (cleanRunCounts{Committed: 3}) {
		t.Fatalf("strict gate workflow did not retain three committed facts: scan=%+v err=%v", scan, err)
	}
	for _, run := range scan.Verified {
		if run.PolicyMode != "strict" {
			t.Fatalf("strict override was not bound to every nested run: %+v", scan.Verified)
		}
	}
}

func TestExportBlocksBeforeGodotWithoutAuthorizationOrSafePreset(t *testing.T) {
	requireMacOSAppleSilicon(t)
	for _, test := range []struct {
		name        string
		writePreset bool
		allow       bool
		wantCode    string
	}{
		{name: "authorization", writePreset: true, wantCode: "ENGINE_USER_DATA_NOT_AUTHORIZED"},
		{name: "preset", allow: true, wantCode: "GODOT_EXPORT_PRESET_INVALID"},
	} {
		t.Run(test.name, func(t *testing.T) {
			project, _, _ := createRunStoreProject(t, "blocked-"+test.name)
			setProjectMode(t, project, "manual")
			if test.writePreset {
				writeMacOSExportPreset(t, project)
			}
			marker := filepath.Join(t.TempDir(), "started")
			godot := createExecutable(t, "godot", "#!/bin/sh\nprintf started > '"+marker+"'\n")
			args := []string{"export", "--project", project, "--godot", godot}
			if test.allow {
				args = append(args, "--allow-engine-user-data")
			}
			code, result, _, _ := execute(t, context.Background(), args...)
			if code != contract.ExitPrerequisite || result.Outcome != "BLOCKED" || firstErrorCode(result) != test.wantCode {
				t.Fatalf("unexpected blocked export: code=%d result=%+v", code, result)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("Godot started before export preflight completed: %v", err)
			}
		})
	}
}

func TestExportRejectsInvalidArtifactDespiteZeroEngineExit(t *testing.T) {
	requireMacOSAppleSilicon(t)
	project, stateRoot, state := createRunStoreProject(t, "invalid-artifact")
	setProjectMode(t, project, "manual")
	writeMacOSExportPreset(t, project)
	plain := filepath.Join(t.TempDir(), "not-a-zip")
	if err := os.WriteFile(plain, []byte("not a zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	godot := createExportGodot(t, plain, "")
	writeExportTemplateFixture(t, godot, true)

	code, result, _, _ := execute(t, context.Background(), "export", "--project", project, "--godot", godot, "--allow-engine-user-data", "--timeout-ms", "10000")
	if code != contract.ExitEngine || result.Outcome != "FAIL" || firstErrorCode(result) != "EXPORT_ARTIFACT_INVALID" {
		t.Fatalf("invalid artifact was trusted: code=%d result=%+v", code, result)
	}
	scan, err := scanRuns(context.Background(), stateRoot, state)
	if err != nil || scan.Counts.Committed != 1 || scan.Counts.Corrupt != 0 {
		t.Fatalf("failing export evidence did not remain verifiable: scan=%+v err=%v", scan, err)
	}
}

func TestExportRejectsSingleArchitectureMachO(t *testing.T) {
	requireMacOSAppleSilicon(t)
	project, _, _ := createRunStoreProject(t, "single-architecture")
	setProjectMode(t, project, "manual")
	writeMacOSExportPreset(t, project)
	thin := make([]byte, 8)
	binary.LittleEndian.PutUint32(thin[0:4], 0xfeedfacf)
	binary.LittleEndian.PutUint32(thin[4:8], machCPUTypeARM64)
	godot := createExportGodot(t, createMacOSExportArchiveWithExecutable(t, thin), "")
	writeExportTemplateFixture(t, godot, true)

	code, result, _, _ := execute(t, context.Background(), "export", "--project", project, "--godot", godot, "--allow-engine-user-data", "--timeout-ms", "10000")
	if code != contract.ExitEngine || result.Outcome != "FAIL" || firstErrorCode(result) != "EXPORT_ARTIFACT_INVALID" {
		t.Fatalf("single-architecture export was trusted: code=%d result=%+v", code, result)
	}
}

func TestExportRejectsFailedTargetSmoke(t *testing.T) {
	requireMacOSAppleSilicon(t)
	project, stateRoot, state := createRunStoreProject(t, "failed-target-smoke")
	setProjectMode(t, project, "manual")
	writeMacOSExportPreset(t, project)
	godot := createExportGodot(t, createMacOSExportArchive(t), "")
	writeExportTemplateFixture(t, godot, true)
	original := runExportTargetSmoke
	runExportTargetSmoke = func(context.Context, time.Duration, *os.Root, *os.File, *godotProjectSnapshot, string) targetSmokeExecution {
		exitCode := 23
		return targetSmokeExecution{Process: processResult{ExitCode: &exitCode, Err: errors.New("target exited nonzero")}}
	}
	t.Cleanup(func() { runExportTargetSmoke = original })

	code, result, _, _ := execute(t, context.Background(), "export", "--project", project, "--godot", godot, "--allow-engine-user-data", "--timeout-ms", "10000")
	if code != contract.ExitEngine || result.Outcome != "FAIL" || firstErrorCode(result) != "TARGET_SMOKE_FAILED" {
		t.Fatalf("failed target smoke was trusted: code=%d result=%+v", code, result)
	}
	if _, err := os.Stat(filepath.Join(project, ".gameatelier", "artifacts", result.RunID, "game-release.zip")); !os.IsNotExist(err) {
		t.Fatalf("failed target smoke published an artifact: %v", err)
	}
	scan, err := scanRuns(context.Background(), stateRoot, state)
	if err != nil || scan.Counts.Committed != 1 || scan.Counts.Corrupt != 0 {
		t.Fatalf("failed target-smoke evidence did not remain verifiable: scan=%+v err=%v", scan, err)
	}
}

func writeMacOSExportPreset(t *testing.T, project string) {
	t.Helper()
	content := `[preset.0]

name="macOS Technical"
platform="macOS"
script_export_mode=2

[preset.0.options]

binary_format/architecture="universal"
application/bundle_identifier="io.github.leon0555.atelier-test"
codesign/codesign=0
notarization/notarization=0
`
	if err := os.WriteFile(filepath.Join(project, "export_presets.cfg"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func createMacOSExportArchive(t *testing.T) string {
	t.Helper()
	return createMacOSExportArchiveWithExecutable(t, minimalUniversal2MachO())
}

func createMacOSExportArchiveWithExecutable(t *testing.T, executable []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "game.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	entry, err := archive.Create("Game.app/Contents/MacOS/Game")
	if err != nil {
		file.Close()
		t.Fatal(err)
	}
	if _, err := entry.Write(executable); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func minimalUniversal2MachO() []byte {
	const firstOffset = 4096
	const secondOffset = 8192
	content := make([]byte, secondOffset+8)
	binary.BigEndian.PutUint32(content[0:4], 0xcafebabe)
	binary.BigEndian.PutUint32(content[4:8], 2)
	binary.BigEndian.PutUint32(content[8:12], machCPUTypeX8664)
	binary.BigEndian.PutUint32(content[16:20], firstOffset)
	binary.BigEndian.PutUint32(content[20:24], 8)
	binary.BigEndian.PutUint32(content[24:28], 12)
	binary.BigEndian.PutUint32(content[28:32], machCPUTypeARM64)
	binary.BigEndian.PutUint32(content[36:40], secondOffset)
	binary.BigEndian.PutUint32(content[40:44], 8)
	binary.BigEndian.PutUint32(content[44:48], 12)
	for offset, cpu := range map[int]uint32{firstOffset: machCPUTypeX8664, secondOffset: machCPUTypeARM64} {
		binary.LittleEndian.PutUint32(content[offset:offset+4], 0xfeedfacf)
		binary.LittleEndian.PutUint32(content[offset+4:offset+8], cpu)
	}
	return content
}

func createExportGodot(t *testing.T, artifact, extra string) string {
	t.Helper()
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ]; then printf '4.7.2.stable.official.ed1daf0bf\\n'; exit 0; fi\n" +
		extra + "\n" +
		"cp '" + artifact + "' \"$7\"\n"
	return createExecutable(t, "fake-godot", script)
}

func createPolicyAwareExportGodot(t *testing.T, artifact, exportMarker string, testsPass bool) string {
	t.Helper()
	outcome := "PASS"
	testOutcome := "PASS"
	exitCode := "0"
	if !testsPass {
		outcome = "FAIL"
		testOutcome = "FAIL"
		exitCode = "1"
	}
	report := `{"schema_version":"1.0.0","outcome":"` + outcome + `","tests":[{"id":"policy-gate","outcome":"` + testOutcome + `","summary":"Policy gate test completed."}]}`
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ]; then printf '4.7.2.stable.official.ed1daf0bf\\n'; exit 0; fi\n" +
		"case \" $* \" in\n" +
		"  *\" --script \"*) printf '%s\\n' '" + gdscriptTestReportPrefix + report + "'; exit " + exitCode + ";;\n" +
		"  *\" --quit-after \"*) exit 0;;\n" +
		"  *\" --export-release \"*|*\" --export-debug \"*) printf started > '" + exportMarker + "'; cp '" + artifact + "' \"$7\"; exit 0;;\n" +
		"esac\n" +
		"exit 91\n"
	return createExecutable(t, "policy-aware-godot", script)
}

func writeFixedTestRunner(t *testing.T, project string) {
	t.Helper()
	directory := filepath.Join(project, "tests")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "atelier_test_runner.gd"), []byte("extends SceneTree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func requireMacOSAppleSilicon(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("the first production export slice is enabled only on macOS Apple Silicon")
	}
}

func stubSuccessfulTargetSmoke(t *testing.T) {
	t.Helper()
	original := runExportTargetSmoke
	runExportTargetSmoke = func(context.Context, time.Duration, *os.Root, *os.File, *godotProjectSnapshot, string) targetSmokeExecution {
		exitCode := 0
		return targetSmokeExecution{Process: processResult{ExitCode: &exitCode}}
	}
	t.Cleanup(func() { runExportTargetSmoke = original })
}

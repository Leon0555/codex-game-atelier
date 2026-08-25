package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

func TestGDScriptRunnerReportParserIsStrict(t *testing.T) {
	validPass := gdscriptTestReportPrefix + `{"schema_version":"1.0.0","outcome":"PASS","tests":[{"id":"resource-load","outcome":"PASS","summary":"Resource loaded."}]}`
	validFail := gdscriptTestReportPrefix + `{"schema_version":"1.0.0","outcome":"FAIL","tests":[{"id":"resource-load","outcome":"FAIL","summary":"Resource did not load."}]}`
	for _, input := range []string{validPass, "noise\n" + validPass + "\nnoise", validFail} {
		if _, err := parseGDScriptRunnerReport([]byte(input)); err != nil {
			t.Fatalf("valid report rejected: %v", err)
		}
	}
	invalid := []string{
		"",
		validPass + "\n" + validPass,
		gdscriptTestReportPrefix + `{"schema_version":"1.0.0","schema_version":"1.0.0","outcome":"PASS","tests":[{"id":"a","outcome":"PASS","summary":"ok"}]}`,
		gdscriptTestReportPrefix + `{"schema_version":"1.0.0","outcome":"PASS","tests":[{"id":"a","outcome":"FAIL","summary":"bad"}]}`,
		gdscriptTestReportPrefix + `{"schema_version":"1.0.0","outcome":"FAIL","tests":[{"id":"a","outcome":"PASS","summary":"ok"}]}`,
		gdscriptTestReportPrefix + `{"schema_version":"1.0.0","outcome":"PASS","tests":[]}`,
		gdscriptTestReportPrefix + `{"schema_version":"1.0.0","outcome":"PASS","tests":[{"id":"a","outcome":"PASS","summary":"ok","extra":true}]}`,
	}
	for _, input := range invalid {
		if _, err := parseGDScriptRunnerReport([]byte(input)); err == nil {
			t.Fatalf("invalid report accepted: %q", input)
		}
	}
}

func TestGDScriptRunnerReportMeasuresUnicodeSummariesByCharacters(t *testing.T) {
	accepted := gdscriptTestReportPrefix + `{"schema_version":"1.0.0","outcome":"PASS","tests":[{"id":"unicode-summary","outcome":"PASS","summary":"` + strings.Repeat("中", 200) + `"}]}`
	if _, err := parseGDScriptRunnerReport([]byte(accepted)); err != nil {
		t.Fatalf("schema-valid Unicode summary was rejected by byte length: %v", err)
	}
	rejected := gdscriptTestReportPrefix + `{"schema_version":"1.0.0","outcome":"PASS","tests":[{"id":"unicode-summary","outcome":"PASS","summary":"` + strings.Repeat("中", 513) + `"}]}`
	if _, err := parseGDScriptRunnerReport([]byte(rejected)); err == nil {
		t.Fatal("summary above 512 Unicode characters was accepted")
	}
}

func TestGDScriptRunnerReportRejectsInvalidUTF8AndControlSummaries(t *testing.T) {
	invalidUTF8 := append([]byte(gdscriptTestReportPrefix+`{"schema_version":"1.0.0","outcome":"PASS","tests":[{"id":"invalid-utf8","outcome":"PASS","summary":"`), 0xff)
	invalidUTF8 = append(invalidUTF8, []byte(`"}]}`)...)
	if _, err := parseGDScriptRunnerReport(invalidUTF8); err == nil {
		t.Fatal("report with invalid UTF-8 was accepted")
	}
	for _, summary := range []string{`\u0001`, `\n`, `   `} {
		input := gdscriptTestReportPrefix + `{"schema_version":"1.0.0","outcome":"PASS","tests":[{"id":"unsafe-summary","outcome":"PASS","summary":"` + summary + `"}]}`
		if _, err := parseGDScriptRunnerReport([]byte(input)); err == nil {
			t.Fatalf("unsafe summary was accepted: %q", summary)
		}
	}
}

func TestTestPreflightRejectsRemappedAssertionFailure(t *testing.T) {
	started := time.Now().UTC()
	command := contract.Command{Name: "test", Arguments: testOptions{timeoutMS: 30_000, allowEngineUserData: true}.persistedArguments()}
	result := contract.NewResult(started, command)
	tests := []gdscriptTestCase{{ID: "assertion", Outcome: "FAIL", Summary: "Assertion failed."}}
	result, payload := finishTestResult(started, result, "FAIL", contract.ExitValidation, "GDScript tests completed with assertion failures.", "4.7.2.stable.official.ed1daf0bf", tests, contract.Error{Code: "GDSCRIPT_TESTS_FAILED", Category: "validation", Message: "One or more GDScript tests failed.", Retryable: false, Details: map[string]any{"failed_count": 1}})
	if err := preflightTestRunFinish(result, []runPayload{payload}); err != nil {
		t.Fatalf("valid assertion failure rejected: %v", err)
	}
	result.ExitCode = contract.ExitEngine
	result.Errors = []contract.Error{{Code: "GODOT_PROCESS_FAILED", Category: "engine", Message: "Godot failed.", Retryable: true}}
	if err := preflightTestRunFinish(result, []runPayload{payload}); err == nil {
		t.Fatal("assertion report was accepted after remapping to a generic engine failure")
	}

	emptyResult := contract.NewResult(started, command)
	emptyResult, emptyPayload := finishTestResult(started, emptyResult, "FAIL", contract.ExitEngine, "GDScript test execution did not complete successfully.", "4.7.2.stable.official.ed1daf0bf", nil, contract.Error{Code: "GODOT_PROCESS_FAILED", Category: "engine", Message: "Godot failed while running GDScript tests.", Retryable: true})
	if err := preflightTestRunFinish(emptyResult, []runPayload{emptyPayload}); err != nil {
		t.Fatalf("valid empty engine failure rejected: %v", err)
	}
	emptyResult.Errors = []contract.Error{{Code: "GDSCRIPT_TESTS_FAILED", Category: "validation", Message: "No assertions were recorded.", Retryable: false, Details: map[string]any{"failed_count": 0}}}
	if err := preflightTestRunFinish(emptyResult, []runPayload{emptyPayload}); err == nil {
		t.Fatal("empty report was accepted after remapping to an assertion failure")
	}

	timeoutResult := contract.NewResult(started, command)
	timeoutResult, timeoutPayload := finishTestResult(started, timeoutResult, "FAIL", contract.ExitInterrupted, "GDScript test execution did not complete successfully.", "4.7.2.stable.official.ed1daf0bf", nil, contract.Error{Code: "GODOT_TIMEOUT", Category: "timeout", Message: "GDScript tests exceeded their total timeout.", Retryable: true})
	if err := preflightTestRunFinish(timeoutResult, []runPayload{timeoutPayload}); err != nil {
		t.Fatalf("valid timeout failure rejected: %v", err)
	}
	timeoutResult.Errors = []contract.Error{{Code: "GODOT_PROCESS_FAILED", Category: "engine", Message: "Godot failed.", Retryable: true}}
	if err := preflightTestRunFinish(timeoutResult, []runPayload{timeoutPayload}); err == nil {
		t.Fatal("timeout report was accepted after remapping to an engine failure")
	}
}

func TestTestRequiresExplicitUserDataAuthorizationBeforeGodotStarts(t *testing.T) {
	requireInitializePlatform(t)
	project := createInitializedTestProject(t, "test-policy")
	marker := filepath.Join(t.TempDir(), "started")
	godot := createExecutable(t, "fake-godot", "#!/bin/sh\nprintf started > '"+marker+"'\n")

	code, result, _, _ := execute(t, context.Background(), "test", "--project", project, "--godot", godot)
	if code != contract.ExitPrerequisite || result.Outcome != "BLOCKED" || firstErrorCode(result) != "ENGINE_USER_DATA_NOT_AUTHORIZED" || len(result.Evidence) != 1 {
		t.Fatalf("unexpected test policy result: code=%d result=%+v", code, result)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("Godot started without authorization: %v", err)
	}
	assertStoredRunClosure(t, project, result.RunID)
}

func TestTestCommitsPassingAndFailingReports(t *testing.T) {
	requireInitializePlatform(t)
	for _, test := range []struct {
		name       string
		report     string
		process    int
		wantExit   int
		wantCode   string
		wantResult string
	}{
		{name: "pass", report: `{"schema_version":"1.0.0","outcome":"PASS","tests":[{"id":"localized-resource","outcome":"PASS","summary":"Resource loaded."},{"id":"signal-flow","outcome":"PASS","summary":"Signal emitted."}]}`, process: 0, wantExit: contract.ExitOK, wantResult: "PASS"},
		{name: "assertion failure", report: `{"schema_version":"1.0.0","outcome":"FAIL","tests":[{"id":"localized-resource","outcome":"PASS","summary":"Resource loaded."},{"id":"signal-flow","outcome":"FAIL","summary":"Signal did not emit."}]}`, process: 1, wantExit: contract.ExitValidation, wantCode: "GDSCRIPT_TESTS_FAILED", wantResult: "FAIL"},
	} {
		t.Run(test.name, func(t *testing.T) {
			project := createInitializedTestProject(t, "test-"+strings.ReplaceAll(test.name, " ", "-"))
			script := "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then printf '4.7.2.stable.official.ed1daf0bf\\n'; exit 0; fi\nprintf '%s\\n' '" + gdscriptTestReportPrefix + test.report + "'\nexit " + string(rune('0'+test.process)) + "\n"
			godot := createExecutable(t, "fake-godot", script)

			code, result, stdout, stderr := execute(t, context.Background(), "test", "--project", project, "--godot", godot, "--timeout-ms", "5000", "--allow-engine-user-data")
			if code != test.wantExit || result.Outcome != test.wantResult || firstErrorCode(result) != test.wantCode || len(result.Evidence) != 1 || stderr != "" {
				t.Fatalf("unexpected test result: code=%d result=%+v stderr=%q", code, result, stderr)
			}
			if strings.Contains(stdout, godot) || result.Command.Arguments["test_runner"] != gdscriptTestRunnerResource || result.Command.Arguments["engine_user_data"] != "standard-os-location" {
				t.Fatalf("test result leaked a path or lost normalized arguments: %s", stdout)
			}
			data := resultDataMap(t, result)
			if data["scope"] != "gdscript" || data["test_count"] != float64(2) || data["passed_count"] != float64(2-test.process) || data["failed_count"] != float64(test.process) {
				t.Fatalf("unexpected test counts: %+v", data)
			}
			intentBytes, err := os.ReadFile(filepath.Join(project, ".gameatelier", "runs", result.RunID, "intent.json"))
			if err != nil {
				t.Fatal(err)
			}
			var intent runIntentRecord
			if err := json.Unmarshal(intentBytes, &intent); err != nil || len(intent.DeclaredExternal) != 1 || intent.DeclaredExternal[0] != "godot:user-data:standard-os-location" {
				t.Fatalf("test external write was not declared: err=%v intent=%+v", err, intent)
			}
			assertStoredRunClosure(t, project, result.RunID)
			if test.wantResult == "PASS" {
				cleanCode, cleanResult, _, _ := execute(t, context.Background(), "clean", "--list", "--project", project)
				cleanData := resultDataMap(t, cleanResult)
				counts, _ := cleanData["counts"].(map[string]any)
				if cleanCode != contract.ExitOK || cleanResult.Outcome != "PASS" || counts["committed"] != float64(1) || counts["corrupt"] != float64(0) {
					t.Fatalf("scanner did not accept the committed test closure: code=%d result=%+v", cleanCode, cleanResult)
				}
			}
		})
	}
}

func TestTestRejectsInvalidReportAndFixedRunnerAbsence(t *testing.T) {
	requireInitializePlatform(t)
	project := createInitializedTestProject(t, "invalid-report")
	godot := createExecutable(t, "fake-godot", "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then printf '4.7.2.stable.official.ed1daf0bf\\n'; exit 0; fi\nprintf 'not a report\\n'\n")
	code, result, _, _ := execute(t, context.Background(), "test", "--project", project, "--godot", godot, "--allow-engine-user-data")
	if code != contract.ExitEngine || result.Outcome != "FAIL" || firstErrorCode(result) != "GDSCRIPT_TEST_REPORT_INVALID" || len(result.Evidence) != 1 {
		t.Fatalf("invalid report was not rejected: code=%d result=%+v", code, result)
	}
	assertStoredRunClosure(t, project, result.RunID)

	missing := createProject(t, "missing-runner")
	initializedCode, initialized, _, _ := execute(t, context.Background(), "initialize", "--project", missing)
	if initializedCode != contract.ExitOK || initialized.Outcome != "PASS" {
		t.Fatalf("initialize failed: %+v", initialized)
	}
	code, result, _, _ = execute(t, context.Background(), "test", "--project", missing, "--godot", godot, "--allow-engine-user-data")
	if code != contract.ExitPrerequisite || result.Outcome != "BLOCKED" || firstErrorCode(result) != "GDSCRIPT_TEST_RUNNER_NOT_FOUND" || len(result.Evidence) != 1 {
		t.Fatalf("missing runner was not blocked: code=%d result=%+v", code, result)
	}
	assertStoredRunClosure(t, missing, result.RunID)
}

func TestTestMapsTimeoutAndCancellationToInterruptedEvidence(t *testing.T) {
	requireInitializePlatform(t)
	project := createInitializedTestProject(t, "test-timeout")
	godot := createExecutable(t, "fake-godot", "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then printf '4.7.2.stable.official.ed1daf0bf\\n'; exit 0; fi\nsleep 5\n")
	code, result, _, _ := execute(t, context.Background(), "test", "--project", project, "--godot", godot, "--timeout-ms", "100", "--allow-engine-user-data")
	if code != contract.ExitInterrupted || result.Outcome != "FAIL" || firstErrorCode(result) != "GODOT_TIMEOUT" || len(result.Evidence) != 1 {
		t.Fatalf("test timeout was not committed correctly: code=%d result=%+v", code, result)
	}
	assertStoredRunClosure(t, project, result.RunID)

	cancelledProject := createInitializedTestProject(t, "test-cancelled")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	code, result, _, _ = execute(t, ctx, "test", "--project", cancelledProject, "--godot", godot, "--allow-engine-user-data")
	if time.Since(started) > 2*time.Second || code != contract.ExitInterrupted || result.Outcome != "FAIL" || firstErrorCode(result) != "COMMAND_CANCELLED" || len(result.Evidence) != 1 {
		t.Fatalf("test cancellation was not committed promptly: code=%d result=%+v", code, result)
	}
	assertStoredRunClosure(t, cancelledProject, result.RunID)
}

func TestPinnedTestActionUsesOnlyFixedArguments(t *testing.T) {
	requireInitializePlatform(t)
	project := createInitializedTestProject(t, "fixed-arguments")
	observation := filepath.Join(t.TempDir(), "arguments")
	script := "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then printf '4.7.2.stable.official.ed1daf0bf\\n'; exit 0; fi\nprintf '%s\\n' \"$@\" > '" + observation + "'\nprintf '%s\\n' '" + gdscriptTestReportPrefix + `{"schema_version":"1.0.0","outcome":"PASS","tests":[{"id":"fixed-entry","outcome":"PASS","summary":"Fixed entry ran."}]}` + "'\n"
	godot := createExecutable(t, "fake-godot", script)

	code, result, _, _ := execute(t, context.Background(), "test", "--project", project, "--godot", godot, "--allow-engine-user-data")
	if code != contract.ExitOK || result.Outcome != "PASS" {
		t.Fatalf("fixed test action failed: code=%d result=%+v", code, result)
	}
	content, err := os.ReadFile(observation)
	want := strings.Join([]string{"--headless", "--path", ".", "--script", gdscriptTestRunnerResource, "--no-header", ""}, "\n")
	if err != nil || string(content) != want {
		t.Fatalf("unexpected fixed arguments: err=%v got=%q want=%q", err, content, want)
	}
}

func createInitializedTestProject(t *testing.T, name string) string {
	t.Helper()
	project := createProject(t, name)
	if err := os.MkdirAll(filepath.Join(project, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, gdscriptTestRunner), []byte("extends SceneTree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, result, _, _ := execute(t, context.Background(), "initialize", "--project", project)
	if code != contract.ExitOK || result.Outcome != "PASS" {
		t.Fatalf("initialize failed: code=%d result=%+v", code, result)
	}
	return project
}

//go:build darwin || linux

package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if handled, code := RunPinnedGodotHelper(os.Stderr); handled {
		os.Exit(code)
	}
	runner := filepath.Join(filepath.Dir(os.Args[0]), "codex-game-atelier-runner")
	if err := os.Symlink(os.Args[0], runner); err != nil && !os.IsExist(err) {
		_, _ = os.Stderr.WriteString("could not prepare the test runner: " + err.Error() + "\n")
		os.Exit(125)
	}
	code := m.Run()
	_ = os.Remove(runner)
	os.Exit(code)
}

func TestPinnedGodotHelperRejectsInvalidInvocation(t *testing.T) {
	t.Setenv(pinnedGodotHelperEnvironment, "invalid")
	var stderr bytes.Buffer
	handled, code := RunPinnedGodotHelper(&stderr)
	if !handled || code != 125 || stderr.Len() == 0 {
		t.Fatalf("invalid helper invocation was not rejected: handled=%t code=%d stderr=%q", handled, code, stderr.String())
	}
}

func TestPinnedRunnerDiscoveryResolvesPublicCLISymlinkFirst(t *testing.T) {
	realDirectory := t.TempDir()
	launcherDirectory := t.TempDir()
	cli := filepath.Join(realDirectory, "codex-game-atelier")
	runner := filepath.Join(realDirectory, "codex-game-atelier-runner")
	for _, path := range []string{cli, runner} {
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	launcher := filepath.Join(launcherDirectory, "atelier")
	if err := os.Symlink(cli, launcher); err != nil {
		t.Fatal(err)
	}
	resolved, err := resolvePinnedGodotRunner(launcher)
	expected, expectedErr := filepath.EvalSymlinks(runner)
	if err != nil || expectedErr != nil || resolved != expected {
		t.Fatalf("runner discovery did not follow the public CLI symlink first: resolved=%q err=%v", resolved, err)
	}
}

func TestGodotHeadlessUsesFixedArgumentsAndPinnedProjectDescriptor(t *testing.T) {
	project := createProject(t, "中文 空格 🚀")
	observation := filepath.Join(t.TempDir(), "observation.txt")
	workingDirectory := filepath.Join(t.TempDir(), "working-directory.txt")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ]; then\n" +
		"  printf '4.7.2.stable.official.ed1daf0bf\\n'\n" +
		"  exit 0\n" +
		"fi\n" +
		"pwd -P > '" + workingDirectory + "'\n" +
		"printf '%s\\n' \"$@\" > '" + observation + "'\n" +
		"printf '{\"event\":\"ready\",\"status\":\"PASS\"}\\n'\n"
	godot := createExecutable(t, "fake-godot", script)

	execution := runTestGodotHeadless(t, context.Background(), 10*time.Second, godot, project)
	if execution.Failure != headlessFailureNone || execution.Version != "4.7.2.stable.official.ed1daf0bf" {
		t.Fatalf("unexpected headless execution: %+v", execution)
	}
	content, err := os.ReadFile(observation)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{"--headless", "--path", ".", "--quit-after", "1", "--no-header", ""}, "\n")
	if string(content) != want {
		t.Fatalf("unexpected working directory or arguments:\n%s\nwant:\n%s", content, want)
	}
	directoryContent, err := os.ReadFile(workingDirectory)
	wantDirectory, canonicalErr := canonicalProjectRoot(project)
	if err != nil || canonicalErr != nil || strings.TrimSpace(string(directoryContent)) != wantDirectory {
		t.Fatalf("scene did not execute from the pinned project: content=%q err=%v", directoryContent, err)
	}
}

func TestGodotExportUsesFixedArgumentsPinnedTemplatesAndCleansRuntime(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("the first production export action is enabled only on macOS Apple Silicon")
	}
	project := createProject(t, "导出 项目 🚀")
	if err := os.MkdirAll(filepath.Join(project, godotProjectOutputDirectory), 0o700); err != nil {
		t.Fatal(err)
	}
	output := godotProjectOutputDirectory + "/game-release.zip"
	observation := filepath.Join(t.TempDir(), "export-arguments.txt")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ]; then printf '4.7.2.stable.official.ed1daf0bf\\n'; exit 0; fi\n" +
		"runtime_dir=$(dirname \"$0\")\n" +
		"test -f \"$runtime_dir/_sc_\" || exit 41\n" +
		"test -f \"$runtime_dir/editor_data/export_templates/4.7.2.stable/macos.zip\" || exit 42\n" +
		"printf '%s\\n' \"$@\" > '" + observation + "'\n" +
		"printf 'artifact' > \"$7\"\n"
	godot := createExecutable(t, "fake-godot", script)
	writeExportTemplateFixture(t, godot, true)
	templates, err := locateGodotExportTemplates(godot, []string{"icudt_godot.dat", "macos.zip"})
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(godot)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	runRootPath := t.TempDir()
	runRoot, err := os.OpenRoot(runRootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer runRoot.Close()
	runnerSource, err := os.Open(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	defer runnerSource.Close()

	execution := runGodotExportAction(context.Background(), 10*time.Second, runnerSource, source, runRoot, openProjectDirectory(t, project), "release", defaultMacOSExportPreset, output, templates)
	if execution.Failure != headlessFailureNone || execution.Version != "4.7.2.stable.official.ed1daf0bf" {
		t.Fatalf("unexpected export execution: %+v", execution)
	}
	wantArguments := strings.Join([]string{"--headless", "--path", ".", "--no-header", "--export-release", defaultMacOSExportPreset, output, ""}, "\n")
	arguments, err := os.ReadFile(observation)
	if err != nil || string(arguments) != wantArguments {
		t.Fatalf("unexpected export arguments: %q err=%v", arguments, err)
	}
	if artifact, err := os.ReadFile(filepath.Join(project, filepath.FromSlash(output))); err != nil || string(artifact) != "artifact" {
		t.Fatalf("export artifact was not created at the fixed path: content=%q err=%v", artifact, err)
	}
	entries, err := os.ReadDir(runRootPath)
	if err != nil || len(entries) != 0 {
		t.Fatalf("export execution left transient runtime state: entries=%v err=%v", entries, err)
	}
}

func TestGodotHeadlessRejectsUnsupportedVersionBeforeScene(t *testing.T) {
	project := createProject(t, "unsupported")
	marker := filepath.Join(t.TempDir(), "scene-started")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ]; then printf '4.8.stable.official.deadbeef\\n'; exit 0; fi\n" +
		"printf started > '" + marker + "'\n"
	godot := createExecutable(t, "fake-godot", script)

	execution := runTestGodotHeadless(t, context.Background(), 10*time.Second, godot, project)
	if execution.Failure != headlessFailureUnsupportedVersion || execution.FailureStage != "version" {
		t.Fatalf("unexpected unsupported-version result: %+v", execution)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("scene started after version rejection: %v", err)
	}
}

func TestGodotHeadlessRejectsVersionErrorsBeforeScene(t *testing.T) {
	project := createProject(t, "version-error")
	marker := filepath.Join(t.TempDir(), "scene-started")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ]; then printf '4.7.2.stable.official.ed1daf0bf\\n'; printf 'ERROR: broken runtime\\n' >&2; exit 0; fi\n" +
		"printf started > '" + marker + "'\n"
	godot := createExecutable(t, "fake-godot", script)

	execution := runTestGodotHeadless(t, context.Background(), 10*time.Second, godot, project)
	if execution.Failure != headlessFailureEngineErrors || execution.FailureStage != "version" {
		t.Fatalf("unexpected version-error result: %+v", execution)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("scene started after version error: %v", err)
	}
}

func TestGodotHeadlessMapsSceneExitTimeoutAndCancellation(t *testing.T) {
	project := createProject(t, "failures")
	for _, test := range []struct {
		name    string
		body    string
		ctx     func() context.Context
		timeout time.Duration
		want    godotHeadlessFailure
	}{
		{name: "nonzero", body: "exit 23", ctx: context.Background, timeout: 10 * time.Second, want: headlessFailureProcess},
		{name: "timeout", body: "sleep 5", ctx: context.Background, timeout: 250 * time.Millisecond, want: headlessFailureTimeout},
		{name: "cancelled", body: "sleep 5", ctx: cancelledContext, timeout: 10 * time.Second, want: headlessFailureCancelled},
	} {
		t.Run(test.name, func(t *testing.T) {
			script := "#!/bin/sh\n" +
				"if [ \"$1\" = \"--version\" ]; then printf '4.7.2.stable.official.ed1daf0bf\\n'; exit 0; fi\n" +
				test.body + "\n"
			godot := createExecutable(t, "fake-godot", script)
			execution := runTestGodotHeadless(t, test.ctx(), test.timeout, godot, project)
			if execution.Failure != test.want {
				t.Fatalf("failure=%q, want %q: %+v", execution.Failure, test.want, execution)
			}
			if execution.FailureStage != "version" && execution.FailureStage != "scene" {
				t.Fatalf("unexpected failure stage=%q", execution.FailureStage)
			}
		})
	}
}

func TestGodotHeadlessRejectsTruncatedOutputFromEveryStageAndStream(t *testing.T) {
	large := strings.Repeat("x", maxHeadlessOutputBytes+1)
	for _, test := range []struct {
		name        string
		versionBody string
		sceneBody   string
		wantStage   string
		wantStdout  bool
		wantStderr  bool
	}{
		{name: "version stdout", versionBody: "printf '" + large + "'; exit 23", wantStage: "version", wantStdout: true},
		{name: "version stderr", versionBody: "printf '4.7.2.stable.official.ed1daf0bf\\n'; printf '" + large + "' >&2; exit 23", wantStage: "version", wantStderr: true},
		{name: "scene stdout", versionBody: "printf '4.7.2.stable.official.ed1daf0bf\\n'", sceneBody: "printf '" + large + "'; exit 23", wantStage: "scene", wantStdout: true},
		{name: "scene stderr", versionBody: "printf '4.7.2.stable.official.ed1daf0bf\\n'", sceneBody: "printf '" + large + "' >&2; exit 23", wantStage: "scene", wantStderr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			project := createProject(t, "noisy-"+test.name)
			script := "#!/bin/sh\n" +
				"if [ \"$1\" = \"--version\" ]; then " + test.versionBody + "; exit $?; fi\n" +
				test.sceneBody + "\n"
			godot := createExecutable(t, "fake-godot", script)

			execution := runTestGodotHeadless(t, context.Background(), 10*time.Second, godot, project)
			process := execution.VersionProcess
			if test.wantStage == "scene" {
				process = execution.SceneProcess
			}
			if execution.Failure != headlessFailureOutputTruncated || execution.FailureStage != test.wantStage || process.StdoutTruncated != test.wantStdout || process.StderrTruncated != test.wantStderr {
				t.Fatalf("unexpected truncation result: %+v", execution)
			}
		})
	}
}

func TestClassifyHeadlessResultUsesStableFailurePriority(t *testing.T) {
	processFailure := errors.New("process exited nonzero")
	for _, test := range []struct {
		name   string
		result processResult
		want   godotHeadlessFailure
	}{
		{name: "process", result: processResult{Err: processFailure}, want: headlessFailureProcess},
		{name: "truncation before process", result: processResult{Err: processFailure, StdoutTruncated: true}, want: headlessFailureOutputTruncated},
		{name: "timeout before truncation", result: processResult{Err: processFailure, TimedOut: true, StderrTruncated: true}, want: headlessFailureTimeout},
		{name: "cancellation first", result: processResult{Err: processFailure, Cancelled: true, TimedOut: true, StdoutTruncated: true}, want: headlessFailureCancelled},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := classifyHeadlessResult(test.result); got != test.want {
				t.Fatalf("classifyHeadlessResult()=%q, want %q", got, test.want)
			}
		})
	}
}

func TestGodotHeadlessRejectsEngineErrorsDespiteZeroExit(t *testing.T) {
	for _, prefix := range []string{"ERROR:", "SCRIPT ERROR:", "USER ERROR:"} {
		t.Run(prefix, func(t *testing.T) {
			project := createProject(t, "engine-error")
			script := "#!/bin/sh\n" +
				"if [ \"$1\" = \"--version\" ]; then printf '4.7.2.stable.official.ed1daf0bf\\n'; exit 0; fi\n" +
				"printf '" + prefix + " user data directory was not writable\\n' >&2\n" +
				"exit 0\n"
			godot := createExecutable(t, "fake-godot", script)

			execution := runTestGodotHeadless(t, context.Background(), 10*time.Second, godot, project)
			if execution.Failure != headlessFailureEngineErrors || execution.FailureStage != "scene" {
				t.Fatalf("engine error was ignored: %+v", execution)
			}
		})
	}
}

func TestGodotHeadlessUsesPinnedSourceAcrossPublicExecutableReplacement(t *testing.T) {
	project := createProject(t, "engine-replacement")
	temporary := t.TempDir()
	originalMarker := filepath.Join(temporary, "original-scene")
	replacementMarker := filepath.Join(temporary, "replacement-scene")
	replacement := filepath.Join(temporary, "replacement-godot")
	original := filepath.Join(temporary, "fake-godot")
	replacementScript := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ]; then printf '4.7.2.stable.official.ed1daf0bf\\n'; exit 0; fi\n" +
		"printf replacement > '" + replacementMarker + "'\n"
	if err := os.WriteFile(replacement, []byte(replacementScript), 0o755); err != nil {
		t.Fatal(err)
	}
	originalScript := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ]; then\n" +
		"  mv '" + original + "' '" + original + ".old'\n" +
		"  mv '" + replacement + "' '" + original + "'\n" +
		"  printf '4.7.2.stable.official.ed1daf0bf\\n'\n" +
		"  exit 0\n" +
		"fi\n" +
		"printf original > '" + originalMarker + "'\n"
	if err := os.WriteFile(original, []byte(originalScript), 0o755); err != nil {
		t.Fatal(err)
	}

	execution := runTestGodotHeadless(t, context.Background(), 10*time.Second, original, project)
	if execution.Failure != headlessFailureNone {
		t.Fatalf("public executable replacement redirected the pinned run: %+v", execution)
	}
	if _, err := os.Stat(originalMarker); err != nil {
		t.Fatalf("the original pinned source did not run the scene stage: %v", err)
	}
	if _, err := os.Stat(replacementMarker); !os.IsNotExist(err) {
		t.Fatalf("the replacement executable ran unexpectedly: %v", err)
	}
}

func TestGodotHeadlessUsesPinnedRunnerAcrossPublicRunnerReplacement(t *testing.T) {
	project := createProject(t, "runner-replacement")
	temporary := t.TempDir()
	runner := filepath.Join(temporary, "runner-source")
	if err := os.Link(os.Args[0], runner); err != nil {
		t.Fatal(err)
	}
	replacementMarker := filepath.Join(temporary, "replacement-runner-started")
	replacement := filepath.Join(temporary, "replacement-runner")
	if err := os.WriteFile(replacement, []byte("#!/bin/sh\nprintf replacement > '"+replacementMarker+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ]; then\n" +
		"  mv '" + runner + "' '" + runner + ".old'\n" +
		"  mv '" + replacement + "' '" + runner + "'\n" +
		"  printf '4.7.2.stable.official.ed1daf0bf\\n'\n" +
		"  exit 0\n" +
		"fi\n" +
		"printf scene-ready\n"
	godot := createExecutable(t, "fake-godot", script)

	execution := runTestGodotHeadlessWithRunner(t, context.Background(), 10*time.Second, runner, godot, project)
	if execution.Failure != headlessFailureNone {
		t.Fatalf("public runner replacement redirected the pinned run: %+v", execution)
	}
	if _, err := os.Stat(replacementMarker); !os.IsNotExist(err) {
		t.Fatalf("the replacement runner executed unexpectedly: %v", err)
	}
}

func TestGodotHeadlessCancellationAfterSceneStartRemovesChild(t *testing.T) {
	project := createProject(t, "scene-cancel-child")
	temporary := t.TempDir()
	pidFile := filepath.Join(temporary, "child.pid")
	started := filepath.Join(temporary, "scene-started")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ]; then printf '4.7.2.stable.official.ed1daf0bf\\n'; exit 0; fi\n" +
		"sleep 30 &\n" +
		"child=$!\n" +
		"printf '%s' \"$child\" > '" + pidFile + "'\n" +
		"printf started > '" + started + "'\n" +
		"wait \"$child\"\n"
	godot := createExecutable(t, "fake-godot", script)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(started); err == nil {
				cancel()
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		cancel()
	}()

	execution := runTestGodotHeadless(t, ctx, 10*time.Second, godot, project)
	<-done
	if execution.Failure != headlessFailureCancelled || execution.FailureStage != "scene" {
		t.Fatalf("scene cancellation was not classified correctly: %+v", execution)
	}
	assertPIDDisappears(t, pidFile)
}

func TestGodotHeadlessTimeoutBoundsLargeSourceDigest(t *testing.T) {
	project := createProject(t, "large-source-timeout")
	godot := filepath.Join(t.TempDir(), "large-fake-godot")
	file, err := os.OpenFile(godot, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("#!/bin/sh\nexit 0\n"); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Truncate(256 * 1024 * 1024); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	execution := runTestGodotHeadless(t, context.Background(), time.Millisecond, godot, project)
	if execution.Failure != headlessFailureTimeout || execution.FailureStage != "version" {
		t.Fatalf("large source timeout was not classified correctly: %+v", execution)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("large source ignored the timeout for %s", elapsed)
	}
}

func openProjectDirectory(t *testing.T, path string) *os.File {
	t.Helper()
	directory, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = directory.Close() })
	return directory
}

func runTestGodotHeadless(t *testing.T, ctx context.Context, timeout time.Duration, godot, project string) godotHeadlessExecution {
	t.Helper()
	return runTestGodotHeadlessWithRunner(t, ctx, timeout, os.Args[0], godot, project)
}

func runTestGodotHeadlessWithRunner(t *testing.T, ctx context.Context, timeout time.Duration, runner, godot, project string) godotHeadlessExecution {
	t.Helper()
	source, err := os.Open(godot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = source.Close() })
	runRootPath := t.TempDir()
	runRoot, err := os.OpenRoot(runRootPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runRoot.Close() })
	runnerSource, err := os.Open(runner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runnerSource.Close() })
	execution := runGodotHeadless(ctx, timeout, runnerSource, source, runRoot, openProjectDirectory(t, project))
	entries, err := os.ReadDir(runRootPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("headless execution left transient engine snapshots: %v", entries)
	}
	return execution
}

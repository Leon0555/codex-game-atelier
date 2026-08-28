//go:build darwin || linux

package app

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestGodotTargetSmokeExtractsRunsAndCleansPinnedRunner(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("the first target-smoke slice is enabled only on macOS Apple Silicon")
	}
	snapshotPath := t.TempDir()
	if err := os.Mkdir(filepath.Join(snapshotPath, godotProjectOutputDirectory), 0o700); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(snapshotPath, godotProjectOutputDirectory, "game-release.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("Game.app/Contents/MacOS/Game")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("#!/bin/sh\nexit 0\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	snapshotRoot, err := os.OpenRoot(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	defer snapshotRoot.Close()
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

	execution := runGodotExportTargetSmoke(context.Background(), 10*time.Second, runRoot, runnerSource, &godotProjectSnapshot{root: snapshotRoot}, godotProjectOutputDirectory+"/game-release.zip")
	if execution.Err != nil || classifyHeadlessResult(execution.Process) != headlessFailureNone {
		t.Fatalf("target smoke failed: %+v", execution)
	}
	if _, err := os.Stat(filepath.Join(runRootPath, godotTargetSmokeRunner)); !os.IsNotExist(err) {
		t.Fatalf("target smoke left its pinned runner: %v", err)
	}
}

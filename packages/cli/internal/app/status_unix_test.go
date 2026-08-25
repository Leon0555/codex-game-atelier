//go:build darwin || linux

package app

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

func TestStatusRejectsFIFOWithoutBlocking(t *testing.T) {
	project := createProject(t, "fifo-state")
	stateDir := filepath.Join(project, ".gameatelier")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(stateDir, "project.json"), 0o600); err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	code, result, _, _ := execute(t, context.Background(), "status", "--project", project)
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("FIFO state read blocked for %s", elapsed)
	}
	if code != contract.ExitState || firstErrorCode(result) != "STATE_READ_FAILED" {
		t.Fatalf("unexpected FIFO result: code=%d result=%+v", code, result)
	}
}

func TestStatusRejectsSymlinkedStatePath(t *testing.T) {
	project := createProject(t, "symlink-state")
	stateDir := filepath.Join(project, ".gameatelier")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(stateDir, "project.json")); err != nil {
		t.Fatal(err)
	}

	code, result, _, _ := execute(t, context.Background(), "status", "--project", project)
	if code != contract.ExitState || firstErrorCode(result) != "STATE_READ_FAILED" {
		t.Fatalf("unexpected symlink result: code=%d result=%+v", code, result)
	}
}

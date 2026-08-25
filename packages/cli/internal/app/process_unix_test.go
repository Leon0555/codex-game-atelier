//go:build darwin || linux

package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

func TestDoctorRemovesChildThatKeepsPipesOpen(t *testing.T) {
	project := createProject(t, "child-cleanup")
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	script := "#!/bin/sh\n" +
		"sleep 30 &\n" +
		"child=$!\n" +
		"printf '%s' \"$child\" > '" + pidFile + "'\n" +
		"printf '4.7.2.stable.official.ed1daf0bf\\n'\n" +
		"exit 0\n"
	godot := createExecutable(t, "child-godot", script)

	code, result, _, _ := execute(t, context.Background(), "doctor", "--project", project, "--godot", godot, "--timeout-ms", "2000")
	if code != contract.ExitEngine || firstErrorCode(result) != "GODOT_PROCESS_FAILED" {
		t.Fatalf("unexpected child-held-pipe result: code=%d result=%+v", code, result)
	}
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		err = syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("child process %d remained after doctor returned; kill(0)=%v", pid, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestDoctorRemovesSameGroupChildThatClosesPipes(t *testing.T) {
	project := createProject(t, "child-closed-pipes-cleanup")
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	script := "#!/bin/sh\n" +
		"sleep 30 >/dev/null 2>&1 &\n" +
		"child=$!\n" +
		"printf '%s' \"$child\" > '" + pidFile + "'\n" +
		"printf '4.7.2.stable.official.ed1daf0bf\\n'\n" +
		"exit 0\n"
	godot := createExecutable(t, "child-closed-pipes-godot", script)

	code, result, _, _ := execute(t, context.Background(), "doctor", "--project", project, "--godot", godot, "--timeout-ms", "2000")
	if code != contract.ExitOK || result.Outcome != "PASS" {
		t.Fatalf("unexpected normal child result: code=%d result=%+v", code, result)
	}
	assertPIDDisappears(t, pidFile)
}

func assertPIDDisappears(t *testing.T, pidFile string) {
	t.Helper()
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		err = syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("child process %d remained after doctor returned; kill(0)=%v", pid, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

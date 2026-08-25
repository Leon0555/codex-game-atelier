package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
	"time"
)

const maxProcessOutputBytes = 4096

type processResult struct {
	Stdout          []byte
	Stderr          []byte
	StdoutTruncated bool
	StderrTruncated bool
	ExitCode        *int
	Err             error
	TimedOut        bool
	Cancelled       bool
}

type processController interface {
	terminate() error
	cleanup()
}

type limitedBuffer struct {
	mu        sync.Mutex
	data      bytes.Buffer
	limit     int
	truncated bool
}

func (buffer *limitedBuffer) Write(input []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	remaining := buffer.limit - buffer.data.Len()
	if remaining > 0 {
		writeLength := len(input)
		if writeLength > remaining {
			writeLength = remaining
		}
		_, _ = buffer.data.Write(input[:writeLength])
	}
	if len(input) > remaining {
		buffer.truncated = true
	}
	return len(input), nil
}

func (buffer *limitedBuffer) snapshot() ([]byte, bool) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return append([]byte(nil), buffer.data.Bytes()...), buffer.truncated
}

func runManagedProcess(parent context.Context, timeout time.Duration, executable, directory string, arguments ...string) processResult {
	return runManagedProcessWithLimit(parent, timeout, maxProcessOutputBytes, executable, directory, arguments...)
}

func runManagedProcessWithLimit(parent context.Context, timeout time.Duration, outputLimit int, executable, directory string, arguments ...string) processResult {
	return runManagedProcessWithLimitAndFiles(parent, timeout, outputLimit, nil, executable, directory, arguments...)
}

func runManagedProcessWithLimitAndFiles(parent context.Context, timeout time.Duration, outputLimit int, extraFiles []*os.File, executable, directory string, arguments ...string) processResult {
	return runManagedProcessWithLimitFilesEnv(parent, timeout, outputLimit, extraFiles, nil, executable, directory, arguments...)
}

func runManagedProcessWithLimitFilesEnv(parent context.Context, timeout time.Duration, outputLimit int, extraFiles []*os.File, environment []string, executable, directory string, arguments ...string) processResult {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, executable, arguments...)
	cmd.Dir = directory
	cmd.ExtraFiles = extraFiles
	if environment != nil {
		cmd.Env = environment
	}
	cmd.WaitDelay = 500 * time.Millisecond
	preparePlatformCommand(cmd)

	stdout := &limitedBuffer{limit: outputLimit}
	stderr := &limitedBuffer{limit: outputLimit}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	var controllerMu sync.Mutex
	var controller processController
	cmd.Cancel = func() error {
		controllerMu.Lock()
		defer controllerMu.Unlock()
		if controller != nil {
			return controller.terminate()
		}
		if cmd.Process != nil {
			return cmd.Process.Kill()
		}
		return os.ErrProcessDone
	}

	if err := cmd.Start(); err != nil {
		result := processResult{Err: err}
		if errors.Is(parent.Err(), context.Canceled) {
			result.Cancelled = true
		} else if errors.Is(parent.Err(), context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			result.TimedOut = true
		}
		return result
	}
	attached, err := attachPlatformProcess(cmd)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return processResult{Err: err}
	}
	controllerMu.Lock()
	controller = attached
	controllerMu.Unlock()

	waitErr := cmd.Wait()
	// A successful process can leave descendants behind after closing or
	// redirecting their pipes. Tear down the containment boundary synchronously
	// on every exit; platform implementations treat an already-empty boundary as
	// a harmless best-effort cleanup.
	controllerMu.Lock()
	if controller != nil {
		_ = controller.terminate()
		controller = nil
	}
	controllerMu.Unlock()
	attached.cleanup()
	stdoutData, stdoutTruncated := stdout.snapshot()
	stderrData, stderrTruncated := stderr.snapshot()
	result := processResult{
		Stdout:          stdoutData,
		Stderr:          stderrData,
		StdoutTruncated: stdoutTruncated,
		StderrTruncated: stderrTruncated,
		Err:             waitErr,
	}
	if cmd.ProcessState != nil {
		exitCode := cmd.ProcessState.ExitCode()
		result.ExitCode = &exitCode
	}
	if errors.Is(parent.Err(), context.Canceled) {
		result.Cancelled = true
	} else if errors.Is(parent.Err(), context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.TimedOut = true
	}
	return result
}

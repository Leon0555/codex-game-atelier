//go:build !unix

package main

import "os/exec"

func configureProcessGroup(_ *exec.Cmd) {
	// The bounded Phase 1 spike only validates process-group termination on Unix.
	// Windows requires a separately tested Job Object implementation.
}

func terminateProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

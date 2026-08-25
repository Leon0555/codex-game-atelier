//go:build !darwin && !linux && !windows

package app

import (
	"os"
	"os/exec"
)

type fallbackProcessController struct {
	process *os.Process
}

func preparePlatformCommand(_ *exec.Cmd) {}

func attachPlatformProcess(cmd *exec.Cmd) (processController, error) {
	return &fallbackProcessController{process: cmd.Process}, nil
}

func (controller *fallbackProcessController) terminate() error {
	return controller.process.Kill()
}

func (controller *fallbackProcessController) cleanup() {}

//go:build darwin || linux

package app

import (
	"os/exec"
	"syscall"
)

type unixProcessController struct {
	pid int
}

func preparePlatformCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func attachPlatformProcess(cmd *exec.Cmd) (processController, error) {
	return &unixProcessController{pid: cmd.Process.Pid}, nil
}

func (controller *unixProcessController) terminate() error {
	return syscall.Kill(-controller.pid, syscall.SIGKILL)
}

func (controller *unixProcessController) cleanup() {}

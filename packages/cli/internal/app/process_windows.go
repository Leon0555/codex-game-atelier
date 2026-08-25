//go:build windows

package app

import (
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"
)

const (
	createNewProcessGroup        = 0x00000200
	jobObjectExtendedLimitInfo   = 9
	jobObjectLimitKillOnJobClose = 0x00002000
	processSetQuota              = 0x0100
	processTerminate             = 0x0001
	processQueryInformation      = 0x0400
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	createJobObjectW         = kernel32.NewProc("CreateJobObjectW")
	setInformationJobObject  = kernel32.NewProc("SetInformationJobObject")
	assignProcessToJobObject = kernel32.NewProc("AssignProcessToJobObject")
	terminateJobObject       = kernel32.NewProc("TerminateJobObject")
)

type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobObjectExtendedLimitInformation struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

type windowsProcessController struct {
	job syscall.Handle
}

func preparePlatformCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup}
}

func attachPlatformProcess(cmd *exec.Cmd) (processController, error) {
	jobValue, _, createErr := createJobObjectW.Call(0, 0)
	if jobValue == 0 {
		return nil, fmt.Errorf("create Windows Job Object: %w", createErr)
	}
	job := syscall.Handle(jobValue)
	info := jobObjectExtendedLimitInformation{}
	info.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose
	setOK, _, setErr := setInformationJobObject.Call(
		uintptr(job),
		jobObjectExtendedLimitInfo,
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
	)
	if setOK == 0 {
		_ = syscall.CloseHandle(job)
		return nil, fmt.Errorf("configure Windows Job Object: %w", setErr)
	}
	process, err := syscall.OpenProcess(processSetQuota|processTerminate|processQueryInformation, false, uint32(cmd.Process.Pid))
	if err != nil {
		_ = syscall.CloseHandle(job)
		return nil, fmt.Errorf("open child process for Job Object: %w", err)
	}
	assignOK, _, assignErr := assignProcessToJobObject.Call(uintptr(job), uintptr(process))
	_ = syscall.CloseHandle(process)
	if assignOK == 0 {
		_ = syscall.CloseHandle(job)
		return nil, fmt.Errorf("assign child process to Windows Job Object: %w", assignErr)
	}
	return &windowsProcessController{job: job}, nil
}

func (controller *windowsProcessController) terminate() error {
	ok, _, err := terminateJobObject.Call(uintptr(controller.job), 1)
	if ok == 0 {
		return fmt.Errorf("terminate Windows Job Object: %w", err)
	}
	return nil
}

func (controller *windowsProcessController) cleanup() {
	_ = syscall.CloseHandle(controller.job)
}

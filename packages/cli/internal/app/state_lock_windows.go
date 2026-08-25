//go:build windows

package app

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

const (
	lockfileFailImmediately = 0x00000001
	lockfileExclusiveLock   = 0x00000002
)

var (
	lockFileEx   = kernel32.NewProc("LockFileEx")
	unlockFileEx = kernel32.NewProc("UnlockFileEx")
)

type windowsProjectStateLock struct {
	file       *os.File
	overlapped syscall.Overlapped
}

func lockProjectStateFile(file *os.File) (projectStateLock, error) {
	lock := &windowsProjectStateLock{file: file}
	ok, _, callErr := lockFileEx.Call(
		file.Fd(),
		lockfileFailImmediately|lockfileExclusiveLock,
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&lock.overlapped)),
	)
	if ok == 0 {
		if errors.Is(callErr, syscall.Errno(33)) {
			return nil, errStateLocked
		}
		return nil, callErr
	}
	return lock, nil
}

func (lock *windowsProjectStateLock) release() {
	_, _, _ = unlockFileEx.Call(lock.file.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&lock.overlapped)))
	_ = lock.file.Close()
}

func syncStateDirectory(_ *os.Root) error {
	return errAtomicPublishUnsupported
}

func atomicLinkUnsupported(err error) bool {
	return errors.Is(err, syscall.Errno(1)) || errors.Is(err, syscall.Errno(50))
}

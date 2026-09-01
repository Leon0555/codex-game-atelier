//go:build darwin

package app

import (
	"errors"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	sysRenameAtXNP  = 488
	renameExclusive = 0x00000004
)

var errStarterAtomicPublishUnsupported = errors.New("atomic no-replace Starter publication is unsupported")

func starterCreatePlatformReady() bool { return runtime.GOARCH == "arm64" }

func publishStarterDirectoryNoReplace(root *os.Root, oldName, newName string) error {
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	oldPointer, err := syscall.BytePtrFromString(oldName)
	if err != nil {
		return err
	}
	newPointer, err := syscall.BytePtrFromString(newName)
	if err != nil {
		return err
	}
	_, _, callErr := syscall.Syscall6(
		sysRenameAtXNP,
		directory.Fd(),
		uintptr(unsafe.Pointer(oldPointer)),
		directory.Fd(),
		uintptr(unsafe.Pointer(newPointer)),
		renameExclusive,
		0,
	)
	if callErr != 0 {
		if callErr == syscall.ENOTSUP {
			return errStarterAtomicPublishUnsupported
		}
		return callErr
	}
	return nil
}

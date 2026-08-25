//go:build linux

package app

import (
	"os"
	"syscall"
	"unsafe"
)

func linkStateFileNoReplace(root *os.Root, existingName, newName string) error {
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	existingPointer, err := syscall.BytePtrFromString(existingName)
	if err != nil {
		return err
	}
	newPointer, err := syscall.BytePtrFromString(newName)
	if err != nil {
		return err
	}
	_, _, callErr := syscall.Syscall6(
		syscall.SYS_LINKAT,
		directory.Fd(),
		uintptr(unsafe.Pointer(existingPointer)),
		directory.Fd(),
		uintptr(unsafe.Pointer(newPointer)),
		0,
		0,
	)
	if callErr != 0 {
		return callErr
	}
	return nil
}

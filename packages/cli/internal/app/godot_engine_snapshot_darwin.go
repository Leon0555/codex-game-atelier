//go:build darwin

package app

import (
	"context"
	"os"
	"syscall"
	"unsafe"
)

const darwinSYSFclonefileat = 517

func cloneGodotExecutable(ctx context.Context, runRoot *os.Root, source *os.File, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	directory, err := runRoot.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	namePointer, err := syscall.BytePtrFromString(name)
	if err != nil {
		return err
	}
	_, _, failure := syscall.Syscall6(
		darwinSYSFclonefileat,
		source.Fd(),
		directory.Fd(),
		uintptr(unsafe.Pointer(namePointer)),
		0,
		0,
		0,
	)
	if failure != 0 {
		return failure
	}
	if err := ctx.Err(); err != nil {
		if cleanupErr := removeUnopenedGodotSnapshot(runRoot, name); cleanupErr != nil {
			return cleanupErr
		}
		return err
	}
	if err := syncStateDirectory(runRoot); err != nil {
		if cleanupErr := removeUnopenedGodotSnapshot(runRoot, name); cleanupErr != nil {
			return cleanupErr
		}
		return err
	}
	return nil
}

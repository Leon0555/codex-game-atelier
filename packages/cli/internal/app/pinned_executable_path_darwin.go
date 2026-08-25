//go:build darwin

package app

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

func pinnedExecutablePath(file *os.File) (string, error) {
	buffer := make([]byte, 4096)
	_, _, failure := syscall.Syscall(syscall.SYS_FCNTL, file.Fd(), uintptr(syscall.F_GETPATH), uintptr(unsafe.Pointer(&buffer[0])))
	if failure != 0 {
		return "", failure
	}
	for index, value := range buffer {
		if value == 0 {
			if index == 0 {
				return "", errors.New("pinned executable path is empty")
			}
			return string(buffer[:index]), nil
		}
	}
	return "", errors.New("pinned executable path exceeds its bound")
}

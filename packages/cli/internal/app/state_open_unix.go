//go:build darwin || linux

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

func openStateFile(root string) (*os.File, error) {
	rootFD, err := syscall.Open(root, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("open project root safely: %w", err)
	}
	defer syscall.Close(rootFD)

	stateDirFD, err := openAt(rootFD, ".gameatelier", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.Close(stateDirFD)

	stateFD, err := openAt(stateDirFD, "project.json", syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	var stat syscall.Stat_t
	if err := syscall.Fstat(stateFD, &stat); err != nil {
		_ = syscall.Close(stateFD)
		return nil, err
	}
	if stat.Mode&syscall.S_IFMT != syscall.S_IFREG {
		_ = syscall.Close(stateFD)
		return nil, fmt.Errorf("project state is not a regular file")
	}
	return os.NewFile(uintptr(stateFD), filepath.Join(root, ".gameatelier", "project.json")), nil
}

func openAt(directoryFD int, path string, flags int, mode uint32) (int, error) {
	pathPointer, err := syscall.BytePtrFromString(path)
	if err != nil {
		return -1, err
	}
	fd, _, callErr := syscall.Syscall6(
		sysOpenAt,
		uintptr(directoryFD),
		uintptr(unsafe.Pointer(pathPointer)),
		uintptr(flags),
		uintptr(mode),
		0,
		0,
	)
	if callErr != 0 {
		return -1, callErr
	}
	return int(fd), nil
}

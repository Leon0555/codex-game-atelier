//go:build darwin || linux

package app

import (
	"errors"
	"os"
	"syscall"
)

type unixProjectStateLock struct {
	file *os.File
}

func lockProjectStateFile(file *os.File) (projectStateLock, error) {
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, errStateLocked
		}
		return nil, err
	}
	return &unixProjectStateLock{file: file}, nil
}

func (lock *unixProjectStateLock) release() {
	_ = syscall.Flock(int(lock.file.Fd()), syscall.LOCK_UN)
	_ = lock.file.Close()
}

func syncStateDirectory(root *os.Root) error {
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func atomicLinkUnsupported(err error) bool {
	return errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, syscall.EXDEV) || errors.Is(err, syscall.EPERM)
}

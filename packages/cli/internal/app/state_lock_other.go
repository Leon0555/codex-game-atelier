//go:build !darwin && !linux && !windows

package app

import (
	"errors"
	"os"
)

func lockProjectStateFile(_ *os.File) (projectStateLock, error) {
	return nil, errors.New("project-state locking is unsupported on this host")
}

func syncStateDirectory(_ *os.Root) error {
	return errors.New("state directory synchronization is unsupported on this host")
}

func atomicLinkUnsupported(_ error) bool {
	return true
}

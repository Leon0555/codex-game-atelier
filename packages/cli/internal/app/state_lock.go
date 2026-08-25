package app

import (
	"errors"
	"os"
	"runtime"
)

const projectStateLockName = ".project-state.lock"

func acquireProjectStateLock(root *os.Root) (projectStateLock, error) {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		lock, retry, err := acquireProjectStateLockOnce(root)
		if !retry {
			return lock, err
		}
		lastErr = err
		runtime.Gosched()
	}
	return nil, lastErr
}

func acquireProjectStateLockOnce(root *os.Root) (projectStateLock, bool, error) {
	priorInfo, priorErr := root.Lstat(projectStateLockName)
	if priorErr != nil && !errors.Is(priorErr, os.ErrNotExist) {
		return nil, false, priorErr
	}
	if priorErr == nil && (priorInfo.Mode()&os.ModeSymlink != 0 || !priorInfo.Mode().IsRegular()) {
		return nil, false, errors.New("project-state lock path is not a regular file")
	}
	file, err := root.OpenFile(projectStateLockName, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, errors.Is(err, os.ErrNotExist), err
	}
	openedInfo, err := file.Stat()
	if err != nil || !openedInfo.Mode().IsRegular() {
		_ = file.Close()
		return nil, false, errors.New("project-state lock is not a regular file")
	}
	currentInfo, err := root.Lstat(projectStateLockName)
	if errors.Is(err, os.ErrNotExist) {
		_ = file.Close()
		return nil, true, err
	}
	if err != nil || currentInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(openedInfo, currentInfo) {
		_ = file.Close()
		return nil, false, errors.New("project-state lock changed while opening")
	}
	lock, err := lockProjectStateFile(file)
	if err != nil {
		_ = file.Close()
		return nil, false, err
	}
	lockedInfo, err := root.Lstat(projectStateLockName)
	if err != nil || lockedInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(openedInfo, lockedInfo) {
		lock.release()
		return nil, true, errors.New("project-state lock changed while acquiring")
	}
	return lock, false, nil
}

package app

import (
	"bytes"
	"errors"
	"io"
	"os"
)

var errStateLocked = errors.New("project state is locked")
var errAtomicPublishUnsupported = errors.New("atomic no-replace state publication is unsupported")

type projectStateLock interface {
	release()
}

type statePublishResult struct {
	err               error
	durabilityErr     error
	targetExists      bool
	atomicUnsupported bool
}

func openOrCreateStateRoot(projectRoot string) (*os.Root, error) {
	root, err := os.OpenRoot(projectRoot)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	if err := root.Mkdir(".gameatelier", 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	return openVerifiedStateRoot(root)
}

func openExistingStateRoot(projectRoot string) (*os.Root, bool, error) {
	root, err := os.OpenRoot(projectRoot)
	if err != nil {
		return nil, false, err
	}
	defer root.Close()
	_, err = root.Lstat(".gameatelier")
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	stateRoot, err := openVerifiedStateRoot(root)
	if err != nil {
		return nil, true, err
	}
	return stateRoot, true, nil
}

func openVerifiedStateRoot(root *os.Root) (*os.Root, error) {
	info, err := root.Lstat(".gameatelier")
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, errors.New("state root must be a real directory")
	}
	stateRoot, err := root.OpenRoot(".gameatelier")
	if err != nil {
		return nil, err
	}
	openedInfo, err := stateRoot.Stat(".")
	if err != nil || !openedInfo.IsDir() || !os.SameFile(info, openedInfo) {
		stateRoot.Close()
		return nil, errors.New("state root changed while opening")
	}
	return stateRoot, nil
}

func readStateFromRootSafely(root *os.Root, expected os.FileInfo) ([]byte, error) {
	file, err := root.Open("project.json")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	openedInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(expected, openedInfo) {
		return nil, errors.New("project state changed while opening")
	}

	content, err := io.ReadAll(io.LimitReader(file, maxProjectStateBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > maxProjectStateBytes {
		return nil, errors.New("project state exceeds the supported size")
	}
	return content, nil
}

func publishProjectState(root *os.Root, runID string, content []byte) statePublishResult {
	tempName := ".project.json.tmp-" + runID
	file, err := root.OpenFile(tempName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return statePublishResult{err: err}
	}
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = root.Remove(tempName)
		}
	}()
	if _, err := io.Copy(file, bytes.NewReader(content)); err != nil {
		_ = file.Close()
		return statePublishResult{err: err}
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return statePublishResult{err: err}
	}
	if err := file.Close(); err != nil {
		return statePublishResult{err: err}
	}
	if err := linkStateFileNoReplace(root, tempName, "project.json"); err != nil {
		return classifyStatePublishError(err)
	}
	cleanupErr := root.Remove(tempName)
	// Once final is visible, never let the deferred pre-publish cleanup mutate
	// the directory after its durability sync. A failed exact-temp removal is
	// reported and left for explicit recovery rather than retried by a glob.
	removeTemp = false
	durabilityErr := syncStateDirectory(root)
	if durabilityErr == nil && cleanupErr != nil {
		durabilityErr = cleanupErr
	}
	return statePublishResult{durabilityErr: durabilityErr}
}

func classifyStatePublishError(err error) statePublishResult {
	result := statePublishResult{err: err}
	if errors.Is(err, os.ErrExist) {
		result.targetExists = true
	}
	if errors.Is(err, errAtomicPublishUnsupported) || atomicLinkUnsupported(err) {
		result.atomicUnsupported = true
	}
	return result
}

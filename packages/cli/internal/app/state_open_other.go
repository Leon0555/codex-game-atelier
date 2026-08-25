//go:build !darwin && !linux

package app

import (
	"errors"
	"os"
)

func openStateFile(root string) (*os.File, error) {
	projectRoot, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer projectRoot.Close()
	directoryInfo, err := projectRoot.Lstat(".gameatelier")
	if err != nil {
		return nil, err
	}
	if directoryInfo.Mode()&os.ModeSymlink != 0 || !directoryInfo.IsDir() {
		return nil, errors.New("state directory must be a real directory")
	}
	pathInfo, err := projectRoot.Lstat(".gameatelier/project.json")
	if err != nil {
		return nil, err
	}
	if pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() {
		return nil, errors.New("project state must be a regular file")
	}
	file, err := projectRoot.Open(".gameatelier/project.json")
	if err != nil {
		return nil, err
	}
	openedInfo, err := file.Stat()
	if err != nil || !openedInfo.Mode().IsRegular() || !os.SameFile(pathInfo, openedInfo) {
		_ = file.Close()
		return nil, errors.New("project state changed while opening")
	}
	return file, nil
}

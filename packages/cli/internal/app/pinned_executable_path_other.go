//go:build !darwin && !linux

package app

import (
	"errors"
	"os"
)

func pinnedExecutablePath(*os.File) (string, error) {
	return "", errors.New("pinned executable paths are not implemented on this host")
}

//go:build linux

package app

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

func pinnedExecutablePath(file *os.File) (string, error) {
	path, err := os.Readlink("/proc/self/fd/" + strconv.FormatUint(uint64(file.Fd()), 10))
	if err != nil || path == "" || strings.HasSuffix(path, " (deleted)") {
		return "", errors.New("pinned executable path is unavailable")
	}
	return path, nil
}

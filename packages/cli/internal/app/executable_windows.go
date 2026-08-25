//go:build windows

package app

import (
	"os"
	"path/filepath"
	"strings"
)

func platformExecutable(_ os.FileMode, path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".exe")
}

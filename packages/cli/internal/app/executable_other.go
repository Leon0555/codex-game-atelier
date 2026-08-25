//go:build !darwin && !linux && !windows

package app

import "os"

func platformExecutable(mode os.FileMode, _ string) bool {
	return mode.IsRegular()
}

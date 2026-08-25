//go:build darwin || linux

package app

import "os"

func platformExecutable(mode os.FileMode, _ string) bool {
	return mode.Perm()&0o111 != 0
}

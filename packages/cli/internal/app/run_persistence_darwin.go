//go:build darwin

package app

import (
	"os"
	"syscall"
)

func readRunPersistenceIdentity(root *os.Root) (runPersistenceIdentity, bool) {
	if !initializePlatformReady() || root == nil {
		return runPersistenceIdentity{}, false
	}
	directory, err := root.Open(".")
	if err != nil {
		return runPersistenceIdentity{}, false
	}
	defer directory.Close()
	var stats syscall.Statfs_t
	if err := syscall.Fstatfs(int(directory.Fd()), &stats); err != nil {
		return runPersistenceIdentity{}, false
	}
	name := make([]byte, 0, len(stats.Fstypename))
	for _, character := range stats.Fstypename {
		if character == 0 {
			break
		}
		name = append(name, byte(character))
	}
	if string(name) != "apfs" {
		return runPersistenceIdentity{}, false
	}
	return runPersistenceIdentity{fsid: [2]int64{int64(stats.Fsid.Val[0]), int64(stats.Fsid.Val[1])}}, true
}

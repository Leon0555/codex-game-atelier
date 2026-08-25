//go:build !darwin

package app

import "os"

func readRunPersistenceIdentity(_ *os.Root) (runPersistenceIdentity, bool) {
	return runPersistenceIdentity{}, false
}

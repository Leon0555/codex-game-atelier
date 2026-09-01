//go:build !darwin

package app

import (
	"errors"
	"os"
)

var errStarterAtomicPublishUnsupported = errors.New("atomic no-replace Starter publication is unsupported")

func starterCreatePlatformReady() bool { return false }

func publishStarterDirectoryNoReplace(_ *os.Root, _, _ string) error {
	return errStarterAtomicPublishUnsupported
}

//go:build windows

package app

import (
	"os"
)

func linkStateFileNoReplace(_ *os.Root, _, _ string) error {
	return errAtomicPublishUnsupported
}

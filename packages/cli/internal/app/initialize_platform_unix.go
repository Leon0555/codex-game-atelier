//go:build darwin

package app

import "runtime"

func initializePlatformReady() bool {
	return runtime.GOARCH == "arm64"
}

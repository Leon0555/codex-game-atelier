//go:build !darwin

package app

func initializePlatformReady() bool {
	return false
}

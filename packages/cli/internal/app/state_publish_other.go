//go:build !darwin && !linux && !windows

package app

import (
	"os"
	"path/filepath"
)

func linkStateFileNoReplace(root *os.Root, existingName, newName string) error {
	return os.Link(filepath.Join(root.Name(), existingName), filepath.Join(root.Name(), newName))
}

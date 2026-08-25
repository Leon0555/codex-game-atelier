//go:build !darwin && !linux

package app

import "errors"

func execPinnedGodot(string) error {
	return errors.New("pinned Godot execution is not implemented on this host")
}

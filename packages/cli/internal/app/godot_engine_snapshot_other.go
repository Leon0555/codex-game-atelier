//go:build !darwin && !linux

package app

import (
	"context"
	"errors"
	"os"
)

func cloneGodotExecutable(context.Context, *os.Root, *os.File, string) error {
	return errors.New("engine snapshots are not implemented on this host")
}

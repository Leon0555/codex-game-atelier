//go:build linux

package app

import (
	"context"
	"os"
)

func cloneGodotExecutable(ctx context.Context, runRoot *os.Root, source *os.File, name string) error {
	return copyExecutableToRunRoot(ctx, runRoot, source, name, maxGodotExecutableBytes)
}

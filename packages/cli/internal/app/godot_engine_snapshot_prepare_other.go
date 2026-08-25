//go:build !darwin

package app

import (
	"context"
	"os"
	"time"
)

func prepareGodotExecutableSnapshot(context.Context, time.Duration, *os.Root, string) error {
	return nil
}

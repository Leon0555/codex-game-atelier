package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(app.Run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

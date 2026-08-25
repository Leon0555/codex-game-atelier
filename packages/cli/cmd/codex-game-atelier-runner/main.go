package main

import (
	"fmt"
	"os"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/app"
)

func main() {
	if handled, code := app.RunPinnedGodotHelper(os.Stderr); handled {
		os.Exit(code)
	}
	_, _ = fmt.Fprintln(os.Stderr, "codex-game-atelier-runner is an internal fd-only component")
	os.Exit(125)
}

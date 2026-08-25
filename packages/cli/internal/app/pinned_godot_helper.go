package app

import (
	"fmt"
	"io"
	"os"
)

const pinnedGodotHelperEnvironment = "CODEX_GAME_ATELIER_INTERNAL_RUNNER_NONCE"

type pinnedRunnerControl struct {
	Nonce string `json:"nonce"`
	Stage string `json:"stage"`
}

// RunPinnedGodotHelper handles the private fd-only protocol of the separate
// runner executable. The public CLI never calls this from its argv dispatcher.
func RunPinnedGodotHelper(stderr io.Writer) (bool, int) {
	nonce := os.Getenv(pinnedGodotHelperEnvironment)
	if nonce == "" {
		return false, 0
	}
	if err := execPinnedGodot(nonce); err != nil {
		_, _ = fmt.Fprintln(stderr, "pinned Godot helper could not start the fixed engine command")
		return true, 125
	}
	return true, 0
}

package app

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

func discoverPinnedGodotRunner() (string, error) {
	current, err := os.Executable()
	if err != nil {
		return "", err
	}
	return resolvePinnedGodotRunner(current)
}

func resolvePinnedGodotRunner(current string) (string, error) {
	resolvedCurrent, err := filepath.EvalSymlinks(current)
	if err != nil {
		return "", err
	}
	name := "codex-game-atelier-runner"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	candidate := filepath.Join(filepath.Dir(resolvedCurrent), name)
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() || !platformExecutable(info.Mode(), resolved) {
		return "", errors.New("pinned Godot runner is not executable")
	}
	return filepath.Clean(resolved), nil
}

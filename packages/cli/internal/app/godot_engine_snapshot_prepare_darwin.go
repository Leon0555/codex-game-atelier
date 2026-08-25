//go:build darwin

package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const godotSnapshotSigningIdentifier = "org.codex-game-atelier.godot-snapshot"

func prepareGodotExecutableSnapshot(ctx context.Context, timeout time.Duration, runRoot *os.Root, name string) error {
	needsSigning, err := godotSnapshotNeedsAdhocSigning(runRoot, name)
	if err != nil || !needsSigning {
		return err
	}
	directory, err := runRoot.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	directoryPath, err := pinnedExecutablePath(directory)
	if err != nil {
		return err
	}
	result := runManagedProcessWithLimit(
		ctx,
		timeout,
		maxProcessOutputBytes,
		"/usr/bin/codesign",
		"",
		"--force",
		"--sign",
		"-",
		"--identifier",
		godotSnapshotSigningIdentifier,
		filepath.Join(directoryPath, name),
	)
	temporaryName := name + ".cstemp"
	if err := runRoot.Remove(temporaryName); err != nil && !errors.Is(err, os.ErrNotExist) {
		return &godotSnapshotCleanupError{err: errors.New("engine snapshot signing temporary file cleanup failed")}
	}
	if err := syncStateDirectory(runRoot); err != nil {
		return &godotSnapshotCleanupError{err: err}
	}
	if result.Err != nil || result.StdoutTruncated || result.StderrTruncated {
		return errors.New("engine snapshot ad-hoc signing failed")
	}
	return nil
}

func godotSnapshotNeedsAdhocSigning(runRoot *os.Root, name string) (bool, error) {
	file, err := runRoot.Open(name)
	if err != nil {
		return false, err
	}
	defer file.Close()
	var magic [4]byte
	if _, err := file.Read(magic[:]); err != nil {
		return false, err
	}
	switch magic {
	case [4]byte{0xfe, 0xed, 0xfa, 0xce},
		[4]byte{0xce, 0xfa, 0xed, 0xfe},
		[4]byte{0xfe, 0xed, 0xfa, 0xcf},
		[4]byte{0xcf, 0xfa, 0xed, 0xfe},
		[4]byte{0xca, 0xfe, 0xba, 0xbe},
		[4]byte{0xbe, 0xba, 0xfe, 0xca},
		[4]byte{0xca, 0xfe, 0xba, 0xbf},
		[4]byte{0xbf, 0xba, 0xfe, 0xca}:
		return true, nil
	default:
		return false, nil
	}
}

package app

import (
	"context"
	"errors"
	"io"
	"os"
	"sort"
)

const godotProjectSnapshotDirectory = ".godot-project-snapshot"
const godotProjectOutputDirectory = ".atelier-output"
const maxGodotProjectSnapshotEntries = 4096
const maxGodotProjectSnapshotBytes int64 = 1024 * 1024 * 1024
const maxGodotProjectSnapshotFileBytes int64 = 512 * 1024 * 1024
const maxGodotProjectSnapshotDepth = 64

type godotProjectSnapshot struct {
	root      *os.Root
	directory *os.File
}

type godotProjectSnapshotBudget struct {
	entries int
	bytes   int64
}

func createGodotProjectSnapshot(ctx context.Context, runRoot, source *os.Root) (*godotProjectSnapshot, error) {
	if runRoot == nil || source == nil {
		return nil, errors.New("project snapshot roots are unavailable")
	}
	destination, err := openOrCreateVerifiedDirectory(runRoot, godotProjectSnapshotDirectory, true)
	if err != nil {
		return nil, err
	}
	cleanupFailure := func(cause error) (*godotProjectSnapshot, error) {
		_ = destination.Close()
		budget := maxGodotProjectSnapshotEntries + 2
		if cleanupErr := removeTransientTree(runRoot, godotProjectSnapshotDirectory, &budget); cleanupErr != nil {
			return nil, errors.Join(cause, errors.New("project snapshot cleanup failed"), cleanupErr)
		}
		return nil, cause
	}
	budget := &godotProjectSnapshotBudget{}
	if err := copyGodotProjectDirectory(ctx, source, destination, budget, 0); err != nil {
		return cleanupFailure(err)
	}
	if err := destination.Mkdir(godotProjectOutputDirectory, 0o700); err != nil {
		return cleanupFailure(err)
	}
	directory, err := destination.Open(".")
	if err != nil {
		return cleanupFailure(err)
	}
	return &godotProjectSnapshot{root: destination, directory: directory}, nil
}

func copyGodotProjectDirectory(ctx context.Context, source, destination *os.Root, budget *godotProjectSnapshotBudget, depth int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if depth > maxGodotProjectSnapshotDepth {
		return errors.New("project snapshot exceeds its directory-depth bound")
	}
	directory, err := source.Open(".")
	if err != nil {
		return err
	}
	entries, readErr := directory.ReadDir(maxGodotProjectSnapshotEntries + 1)
	closeErr := directory.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].Name() < entries[right].Name() })
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := entry.Name()
		if depth == 0 && (name == ".gameatelier" || name == ".godot" || name == ".git") {
			continue
		}
		if name == "" || name == "." || name == ".." || name == godotProjectOutputDirectory {
			return errors.New("project contains a reserved or unsafe snapshot entry")
		}
		budget.entries++
		if budget.entries > maxGodotProjectSnapshotEntries {
			return errors.New("project snapshot exceeds its entry bound")
		}
		info, err := source.Lstat(name)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("project snapshot rejects symlinks and unstable entries")
		}
		if info.IsDir() {
			if err := destination.Mkdir(name, 0o700); err != nil {
				return err
			}
			sourceChild, sourceExists, err := openExistingVerifiedDirectory(source, name)
			if err != nil || !sourceExists {
				return errors.New("project directory changed during snapshot")
			}
			destinationChild, destinationExists, err := openExistingVerifiedDirectory(destination, name)
			if err != nil || !destinationExists {
				sourceChild.Close()
				return errors.New("project snapshot destination could not be pinned")
			}
			err = copyGodotProjectDirectory(ctx, sourceChild, destinationChild, budget, depth+1)
			sourceCloseErr := sourceChild.Close()
			destinationCloseErr := destinationChild.Close()
			if err != nil {
				return err
			}
			if sourceCloseErr != nil || destinationCloseErr != nil {
				return errors.New("project snapshot directory could not be closed")
			}
			continue
		}
		if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > maxGodotProjectSnapshotFileBytes || budget.bytes > maxGodotProjectSnapshotBytes-info.Size() {
			return errors.New("project snapshot contains an unsupported or oversized file")
		}
		budget.bytes += info.Size()
		if err := copyGodotProjectFile(ctx, source, destination, name, info); err != nil {
			return err
		}
	}
	return nil
}

func copyGodotProjectFile(ctx context.Context, source, destination *os.Root, name string, before os.FileInfo) error {
	input, err := source.Open(name)
	if err != nil {
		return err
	}
	opened, err := input.Stat()
	if err != nil || !os.SameFile(before, opened) || !opened.Mode().IsRegular() {
		input.Close()
		return errors.New("project file changed while opening snapshot source")
	}
	output, err := destination.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		input.Close()
		return err
	}
	written, copyErr := copyBoundedWithContext(ctx, output, input, maxGodotProjectSnapshotFileBytes)
	inputAfter, inputStatErr := input.Stat()
	outputSyncErr := output.Sync()
	outputCloseErr := output.Close()
	inputCloseErr := input.Close()
	if copyErr != nil || inputStatErr != nil || !os.SameFile(opened, inputAfter) || inputAfter.Size() != opened.Size() || inputAfter.ModTime() != opened.ModTime() || written != opened.Size() || outputSyncErr != nil || outputCloseErr != nil || inputCloseErr != nil {
		return errors.New("project file changed or failed during snapshot copy")
	}
	return nil
}

func (snapshot *godotProjectSnapshot) remove(runRoot *os.Root) error {
	if snapshot == nil {
		return nil
	}
	var closeErr error
	if snapshot.directory != nil {
		closeErr = snapshot.directory.Close()
		snapshot.directory = nil
	}
	if snapshot.root != nil {
		if err := snapshot.root.Close(); closeErr == nil {
			closeErr = err
		}
		snapshot.root = nil
	}
	budget := maxGodotProjectSnapshotEntries + 2
	removeErr := removeTransientTree(runRoot, godotProjectSnapshotDirectory, &budget)
	return errors.Join(closeErr, removeErr)
}

func copyGodotSnapshotArtifact(ctx context.Context, snapshot *godotProjectSnapshot, destination *os.Root, profile string) error {
	if snapshot == nil || snapshot.root == nil || destination == nil || profile != "debug" && profile != "release" {
		return errors.New("snapshot artifact copy inputs are invalid")
	}
	name := "game-" + profile + ".zip"
	sourceName := godotProjectOutputDirectory + "/" + name
	before, err := snapshot.root.Lstat(sourceName)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() || before.Size() < 1 || before.Size() > maxExportArtifactBytes {
		return errors.New("snapshot export artifact is not a bounded regular file")
	}
	input, err := snapshot.root.Open(sourceName)
	if err != nil {
		return err
	}
	opened, err := input.Stat()
	if err != nil || !os.SameFile(before, opened) {
		input.Close()
		return errors.New("snapshot export artifact changed while opening")
	}
	output, err := destination.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		input.Close()
		return err
	}
	written, copyErr := copyBoundedWithContext(ctx, output, input, maxExportArtifactBytes)
	inputAfter, inputStatErr := input.Stat()
	outputSyncErr := output.Sync()
	outputCloseErr := output.Close()
	inputCloseErr := input.Close()
	if copyErr != nil || inputStatErr != nil || !os.SameFile(opened, inputAfter) || inputAfter.Size() != opened.Size() || inputAfter.ModTime() != opened.ModTime() || written != opened.Size() || outputSyncErr != nil || outputCloseErr != nil || inputCloseErr != nil {
		_ = destination.Remove(name)
		return errors.New("snapshot export artifact copy was not stable")
	}
	return nil
}

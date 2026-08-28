package app

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

const godotTargetSmokeDirectory = ".atelier-target-smoke"
const godotTargetSmokeRunner = ".atelier-target-smoke-runner"

type exportTargetSmoke struct {
	Host     string `json:"host"`
	Arch     string `json:"arch"`
	Mode     string `json:"mode"`
	ExitCode int    `json:"exit_code"`
}

type targetSmokeExecution struct {
	Process processResult
	Err     error
}

var runExportTargetSmoke = runGodotExportTargetSmoke

func runGodotExportTargetSmoke(ctx context.Context, timeout time.Duration, runRoot *os.Root, runnerSource *os.File, snapshot *godotProjectSnapshot, archiveName string) targetSmokeExecution {
	if timeout <= 0 || runRoot == nil || runnerSource == nil || snapshot == nil || snapshot.root == nil {
		return targetSmokeExecution{Err: errors.New("target smoke inputs are invalid")}
	}
	smokeRoot, executablePath, err := extractGodotTargetForSmoke(ctx, snapshot.root, archiveName)
	if err != nil {
		return targetSmokeExecution{Err: err}
	}
	defer smokeRoot.Close()
	directory, err := smokeRoot.Open(".")
	if err != nil {
		return targetSmokeExecution{Err: err}
	}
	defer directory.Close()
	executable, err := smokeRoot.Open(executablePath)
	if err != nil {
		return targetSmokeExecution{Err: err}
	}
	defer executable.Close()

	runnerSourceDigest, err := digestGodotExecutable(ctx, runnerSource)
	if err != nil {
		return targetSmokeExecution{Err: err}
	}
	runner, err := createPinnedRunnerSnapshot(ctx, timeout, runRoot, runnerSource, godotTargetSmokeRunner)
	if err != nil {
		return targetSmokeExecution{Err: err}
	}
	runnerDigest, digestErr := runner.digest(ctx)
	runnerSourceAfterSnapshot, sourceErr := digestGodotExecutable(ctx, runnerSource)
	if digestErr != nil || sourceErr != nil || runnerSourceAfterSnapshot != runnerSourceDigest {
		cleanupErr := removeGodotSnapshots(runRoot, runner)
		return targetSmokeExecution{Err: errors.Join(errors.New("target smoke runner changed during snapshot"), cleanupErr)}
	}
	process := runPinnedGodotStage(ctx, timeout, runner.file, directory, executable, "target-smoke")
	runnerDigestAfter, verifyErr := runner.digest(ctx)
	runnerSourceAfter, sourceVerifyErr := digestGodotExecutable(ctx, runnerSource)
	cleanupErr := removeGodotSnapshots(runRoot, runner)
	if verifyErr != nil || sourceVerifyErr != nil || runnerDigestAfter != runnerDigest || runnerSourceAfter != runnerSourceDigest || cleanupErr != nil {
		return targetSmokeExecution{Process: process, Err: errors.Join(errors.New("target smoke runner verification or cleanup failed"), verifyErr, sourceVerifyErr, cleanupErr)}
	}
	return targetSmokeExecution{Process: process}
}

func extractGodotTargetForSmoke(ctx context.Context, snapshotRoot *os.Root, archiveName string) (*os.Root, string, error) {
	archiveFile, err := snapshotRoot.Open(archiveName)
	if err != nil {
		return nil, "", err
	}
	defer archiveFile.Close()
	info, err := archiveFile.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maxExportArtifactBytes {
		return nil, "", errors.New("target smoke archive is invalid")
	}
	archive, err := zip.NewReader(archiveFile, info.Size())
	if err != nil || len(archive.File) < 1 || len(archive.File) > maxExportArchiveEntries {
		return nil, "", errors.New("target smoke archive cannot be read safely")
	}
	destination, err := openOrCreateVerifiedDirectory(snapshotRoot, godotTargetSmokeDirectory, true)
	if err != nil {
		return nil, "", err
	}
	failed := true
	defer func() {
		if failed {
			destination.Close()
		}
	}()

	entries := append([]*zip.File(nil), archive.File...)
	sort.Slice(entries, func(left, right int) bool { return entries[left].Name < entries[right].Name })
	seen := make(map[string]struct{}, len(entries))
	executablePath := ""
	var expanded uint64
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
		cleaned := path.Clean(entry.Name)
		if cleaned == "." || strings.HasPrefix(cleaned, "/") || cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") || strings.Contains(cleaned, `\`) {
			return nil, "", errors.New("target smoke archive contains an unsafe path")
		}
		folded := strings.ToLower(cleaned)
		if _, duplicate := seen[folded]; duplicate {
			return nil, "", errors.New("target smoke archive contains a duplicate or case-colliding path")
		}
		seen[folded] = struct{}{}
		if entry.Mode()&os.ModeSymlink != 0 || entry.UncompressedSize64 > maxExportArchiveEntryBytes || expanded > maxExportArchiveExpandedBytes-entry.UncompressedSize64 {
			return nil, "", errors.New("target smoke archive exceeds its type or size bounds")
		}
		expanded += entry.UncompressedSize64
		if strings.HasSuffix(entry.Name, "/") {
			if err := ensureRootDirectoryPath(destination, cleaned); err != nil {
				return nil, "", err
			}
			continue
		}
		parent := path.Dir(cleaned)
		if parent != "." {
			if err := ensureRootDirectoryPath(destination, parent); err != nil {
				return nil, "", err
			}
		}
		input, err := entry.Open()
		if err != nil {
			return nil, "", err
		}
		mode := os.FileMode(0o600)
		if strings.Contains(cleaned, ".app/Contents/MacOS/") {
			if executablePath != "" {
				input.Close()
				return nil, "", errors.New("target smoke archive contains multiple app executables")
			}
			executablePath = cleaned
			mode = 0o700
		}
		output, err := destination.OpenFile(cleaned, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			input.Close()
			return nil, "", err
		}
		written, copyErr := copyBoundedWithContext(ctx, output, input, int64(entry.UncompressedSize64))
		syncErr := output.Sync()
		outputCloseErr := output.Close()
		inputCloseErr := input.Close()
		if copyErr != nil || written != int64(entry.UncompressedSize64) || syncErr != nil || outputCloseErr != nil || inputCloseErr != nil {
			return nil, "", errors.New("target smoke archive member extraction failed")
		}
	}
	if executablePath == "" {
		return nil, "", errors.New("target smoke archive has no app executable")
	}
	failed = false
	return destination, executablePath, nil
}

func ensureRootDirectoryPath(root *os.Root, relative string) error {
	current := ""
	for _, segment := range strings.Split(relative, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return errors.New("target smoke directory path is invalid")
		}
		if current == "" {
			current = segment
		} else {
			current += "/" + segment
		}
		if err := root.Mkdir(current, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		info, err := root.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errors.New("target smoke directory is unsafe")
		}
	}
	return nil
}

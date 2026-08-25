package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"time"
)

const maxGodotExecutableBytes int64 = 1024 * 1024 * 1024
const maxPinnedRunnerBytes int64 = 64 * 1024 * 1024

type godotEngineSnapshot struct {
	file *os.File
	name string
}

type godotSnapshotCleanupError struct {
	err error
}

func (failure *godotSnapshotCleanupError) Error() string {
	return "engine snapshot cleanup failed: " + failure.err.Error()
}

func (failure *godotSnapshotCleanupError) Unwrap() error {
	return failure.err
}

func createGodotEngineSnapshot(ctx context.Context, timeout time.Duration, runRoot *os.Root, source *os.File, name string) (*godotEngineSnapshot, error) {
	return createExecutableSnapshot(ctx, timeout, runRoot, source, name, cloneGodotExecutable)
}

func createPinnedRunnerSnapshot(ctx context.Context, timeout time.Duration, runRoot *os.Root, source *os.File, name string) (*godotEngineSnapshot, error) {
	return createExecutableSnapshot(ctx, timeout, runRoot, source, name, func(copyCtx context.Context, root *os.Root, file *os.File, snapshotName string) error {
		return copyExecutableToRunRoot(copyCtx, root, file, snapshotName, maxPinnedRunnerBytes)
	})
}

func createExecutableSnapshot(ctx context.Context, timeout time.Duration, runRoot *os.Root, source *os.File, name string, clone func(context.Context, *os.Root, *os.File, string) error) (*godotEngineSnapshot, error) {
	if runRoot == nil || source == nil || name == "" {
		return nil, errors.New("engine snapshot inputs are invalid")
	}
	if err := clone(ctx, runRoot, source, name); err != nil {
		return nil, err
	}
	if err := prepareGodotExecutableSnapshot(ctx, timeout, runRoot, name); err != nil {
		if cleanupErr := removeUnopenedGodotSnapshot(runRoot, name); cleanupErr != nil {
			return nil, cleanupErr
		}
		return nil, err
	}
	file, err := runRoot.OpenFile(name, os.O_RDONLY, 0)
	if err != nil {
		if cleanupErr := removeUnopenedGodotSnapshot(runRoot, name); cleanupErr != nil {
			return nil, cleanupErr
		}
		return nil, err
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		if cleanupErr := (&godotEngineSnapshot{file: file, name: name}).remove(runRoot); cleanupErr != nil {
			return nil, &godotSnapshotCleanupError{err: cleanupErr}
		}
		return nil, errors.New("engine snapshot is not a regular file")
	}
	if err := file.Chmod(0o500); err != nil {
		if cleanupErr := (&godotEngineSnapshot{file: file, name: name}).remove(runRoot); cleanupErr != nil {
			return nil, &godotSnapshotCleanupError{err: cleanupErr}
		}
		return nil, err
	}
	return &godotEngineSnapshot{file: file, name: name}, nil
}

func copyExecutableToRunRoot(ctx context.Context, runRoot *os.Root, source *os.File, name string, limit int64) (returnErr error) {
	destination, err := runRoot.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o500)
	if err != nil {
		return err
	}
	complete := false
	closed := false
	defer func() {
		if !complete {
			var cleanupErr error
			if !closed {
				cleanupErr = destination.Close()
			}
			if err := runRoot.Remove(name); cleanupErr == nil && err != nil {
				cleanupErr = err
			}
			if err := syncStateDirectory(runRoot); cleanupErr == nil && err != nil {
				cleanupErr = err
			}
			if cleanupErr != nil {
				returnErr = &godotSnapshotCleanupError{err: cleanupErr}
			}
		}
	}()
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return err
	}
	written, err := copyBoundedWithContext(ctx, destination, source, limit)
	if err != nil || written < 1 || written > limit {
		return errors.New("executable snapshot copy failed or exceeded its bound")
	}
	if err := destination.Sync(); err != nil {
		return err
	}
	closeErr := destination.Close()
	closed = true
	if closeErr != nil {
		return closeErr
	}
	if err := syncStateDirectory(runRoot); err != nil {
		return err
	}
	complete = true
	return nil
}

func removeUnopenedGodotSnapshot(runRoot *os.Root, name string) error {
	if err := runRoot.Remove(name); err != nil {
		return &godotSnapshotCleanupError{err: err}
	}
	if err := syncStateDirectory(runRoot); err != nil {
		return &godotSnapshotCleanupError{err: err}
	}
	return nil
}

func (snapshot *godotEngineSnapshot) digest(ctx context.Context) (string, error) {
	if snapshot == nil || snapshot.file == nil {
		return "", errors.New("engine snapshot is closed")
	}
	return digestGodotExecutable(ctx, snapshot.file)
}

func (snapshot *godotEngineSnapshot) remove(runRoot *os.Root) error {
	if snapshot == nil || runRoot == nil {
		return errors.New("engine snapshot cleanup is invalid")
	}
	var closeErr error
	if snapshot.file != nil {
		closeErr = snapshot.file.Close()
		snapshot.file = nil
	}
	removeErr := runRoot.Remove(snapshot.name)
	syncErr := syncStateDirectory(runRoot)
	if closeErr != nil {
		return closeErr
	}
	if removeErr != nil {
		return removeErr
	}
	return syncErr
}

func digestGodotExecutable(ctx context.Context, file *os.File) (string, error) {
	if file == nil {
		return "", errors.New("engine file is missing")
	}
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() || before.Size() < 1 || before.Size() > maxGodotExecutableBytes {
		return "", errors.New("engine file is outside the executable size bound")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	hash := sha256.New()
	written, err := copyBoundedWithContext(ctx, hash, file, maxGodotExecutableBytes)
	if err != nil || written != before.Size() || written > maxGodotExecutableBytes {
		return "", errors.New("engine file changed or exceeded its hash bound")
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) || before.Size() != after.Size() || before.ModTime() != after.ModTime() {
		return "", errors.New("engine file changed while hashing")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func copyBoundedWithContext(ctx context.Context, destination io.Writer, source io.Reader, limit int64) (int64, error) {
	buffer := make([]byte, 1024*1024)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		read, readErr := source.Read(buffer)
		if read > 0 {
			if total+int64(read) > limit {
				return total, errors.New("executable exceeds its copy bound")
			}
			written := 0
			for written < read {
				if err := ctx.Err(); err != nil {
					return total, err
				}
				count, writeErr := destination.Write(buffer[written:read])
				if count < 0 || count > read-written {
					return total, errors.New("executable copy returned an invalid byte count")
				}
				written += count
				total += int64(count)
				if writeErr != nil {
					return total, writeErr
				}
				if count == 0 {
					return total, io.ErrShortWrite
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			return total, nil
		}
		if readErr != nil {
			return total, readErr
		}
	}
}

package app

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

const exportRuntimeDirectory = ".godot-export-runtime"
const maxExportRuntimeEntries = 4096

type godotExportRuntime struct {
	root              *os.Root
	snapshot          *godotEngineSnapshot
	templateSources   []*os.File
	templateSnapshots []*os.File
	templateDigests   []string
}

func createGodotExportRuntime(ctx context.Context, timeout time.Duration, runRoot *os.Root, engineSource *os.File, templates exportTemplateInspection) (*godotExportRuntime, error) {
	if runRoot == nil || engineSource == nil || templates.Root == "" || len(templates.Files) == 0 {
		return nil, errors.New("export runtime inputs are invalid")
	}
	runtimeRoot, err := openOrCreateVerifiedDirectory(runRoot, exportRuntimeDirectory, true)
	if err != nil {
		return nil, err
	}
	runtime := &godotExportRuntime{root: runtimeRoot}
	cleanupFailure := func(failure error) (*godotExportRuntime, error) {
		if cleanupErr := runtime.remove(runRoot); cleanupErr != nil {
			return nil, &godotSnapshotCleanupError{err: cleanupErr}
		}
		return nil, failure
	}
	snapshot, err := createGodotEngineSnapshot(ctx, timeout, runtimeRoot, engineSource, "Godot")
	if err != nil {
		return cleanupFailure(err)
	}
	runtime.snapshot = snapshot
	marker, err := runtimeRoot.OpenFile("_sc_", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o400)
	if err != nil {
		return cleanupFailure(err)
	}
	if err := marker.Sync(); err != nil {
		marker.Close()
		return cleanupFailure(err)
	}
	if err := marker.Close(); err != nil {
		return cleanupFailure(err)
	}
	if err := syncStateDirectory(runtimeRoot); err != nil {
		return cleanupFailure(err)
	}

	editorData, err := openOrCreateVerifiedDirectory(runtimeRoot, "editor_data", true)
	if err != nil {
		return cleanupFailure(err)
	}
	exportTemplates, err := openOrCreateVerifiedDirectory(editorData, "export_templates", true)
	if err != nil {
		editorData.Close()
		return cleanupFailure(err)
	}
	versionRoot, err := openOrCreateVerifiedDirectory(exportTemplates, supportedExportTemplateVersion, true)
	if err != nil {
		exportTemplates.Close()
		editorData.Close()
		return cleanupFailure(err)
	}

	for _, name := range append([]string{"version.txt"}, templates.Files...) {
		if err := ctx.Err(); err != nil {
			versionRoot.Close()
			exportTemplates.Close()
			editorData.Close()
			return cleanupFailure(err)
		}
		source, err := openStableRegularFile(filepath.Join(templates.Root, name))
		if err != nil {
			versionRoot.Close()
			exportTemplates.Close()
			editorData.Close()
			return cleanupFailure(err)
		}
		runtime.templateSources = append(runtime.templateSources, source)
		digest, err := digestGodotExecutable(ctx, source)
		if err != nil {
			versionRoot.Close()
			exportTemplates.Close()
			editorData.Close()
			return cleanupFailure(err)
		}
		runtime.templateDigests = append(runtime.templateDigests, digest)
		if err := copyExportTemplate(ctx, versionRoot, source, name); err != nil {
			versionRoot.Close()
			exportTemplates.Close()
			editorData.Close()
			return cleanupFailure(err)
		}
		destination, err := versionRoot.Open(name)
		if err != nil {
			versionRoot.Close()
			exportTemplates.Close()
			editorData.Close()
			return cleanupFailure(err)
		}
		runtime.templateSnapshots = append(runtime.templateSnapshots, destination)
		destinationDigest, err := digestGodotExecutable(ctx, destination)
		if err != nil || destinationDigest != digest {
			versionRoot.Close()
			exportTemplates.Close()
			editorData.Close()
			return cleanupFailure(errors.New("export template snapshot digest mismatch"))
		}
	}
	if err := versionRoot.Close(); err != nil {
		exportTemplates.Close()
		editorData.Close()
		return cleanupFailure(err)
	}
	if err := exportTemplates.Close(); err != nil {
		editorData.Close()
		return cleanupFailure(err)
	}
	if err := editorData.Close(); err != nil {
		return cleanupFailure(err)
	}
	return runtime, nil
}

func copyExportTemplate(ctx context.Context, destinationRoot *os.Root, source *os.File, name string) (returnErr error) {
	destination, err := destinationRoot.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o400)
	if err != nil {
		return err
	}
	complete := false
	defer func() {
		if !complete {
			_ = destination.Close()
			_ = destinationRoot.Remove(name)
		}
	}()
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return err
	}
	written, err := copyBoundedWithContext(ctx, destination, source, maxGodotExecutableBytes)
	if err != nil || written < 1 {
		return errors.New("export template snapshot copy failed")
	}
	if err := destination.Sync(); err != nil {
		return err
	}
	if err := destination.Close(); err != nil {
		return err
	}
	if err := syncStateDirectory(destinationRoot); err != nil {
		return err
	}
	complete = true
	return nil
}

func (runtime *godotExportRuntime) verify(ctx context.Context) error {
	if runtime == nil || runtime.snapshot == nil || len(runtime.templateSources) != len(runtime.templateSnapshots) || len(runtime.templateSources) != len(runtime.templateDigests) {
		return errors.New("export runtime verification state is invalid")
	}
	for index := range runtime.templateSources {
		sourceDigest, sourceErr := digestGodotExecutable(ctx, runtime.templateSources[index])
		snapshotDigest, snapshotErr := digestGodotExecutable(ctx, runtime.templateSnapshots[index])
		if sourceErr != nil || snapshotErr != nil || sourceDigest != runtime.templateDigests[index] || snapshotDigest != runtime.templateDigests[index] {
			return errors.New("export template changed during execution")
		}
	}
	return nil
}

func (runtime *godotExportRuntime) remove(runRoot *os.Root) error {
	if runtime == nil || runRoot == nil {
		return errors.New("export runtime cleanup is invalid")
	}
	var first error
	for _, file := range runtime.templateSnapshots {
		if file != nil {
			if err := file.Close(); first == nil && err != nil {
				first = err
			}
		}
	}
	for _, file := range runtime.templateSources {
		if file != nil {
			if err := file.Close(); first == nil && err != nil {
				first = err
			}
		}
	}
	if runtime.snapshot != nil && runtime.snapshot.file != nil {
		if err := runtime.snapshot.file.Close(); first == nil && err != nil {
			first = err
		}
		runtime.snapshot.file = nil
	}
	if runtime.root != nil {
		if err := runtime.root.Close(); first == nil && err != nil {
			first = err
		}
		runtime.root = nil
	}
	budget := maxExportRuntimeEntries
	if err := removeTransientTree(runRoot, exportRuntimeDirectory, &budget); first == nil && err != nil {
		first = err
	}
	return first
}

func removeTransientTree(parent *os.Root, name string, budget *int) error {
	if parent == nil || budget == nil || *budget < 1 || name == "" || name == "." || name == ".." {
		return errors.New("transient cleanup exceeded its safety bound")
	}
	*budget--
	info, err := parent.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		if err := parent.Remove(name); err != nil {
			return err
		}
		return syncStateDirectory(parent)
	}
	child, err := parent.OpenRoot(name)
	if err != nil {
		return err
	}
	directory, err := child.Open(".")
	if err != nil {
		child.Close()
		return err
	}
	entries, readErr := directory.ReadDir(maxExportRuntimeEntries + 1)
	closeErr := directory.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		child.Close()
		return readErr
	}
	if closeErr != nil || len(entries) > maxExportRuntimeEntries {
		child.Close()
		if closeErr != nil {
			return closeErr
		}
		return errors.New("transient cleanup directory exceeds its bound")
	}
	for _, entry := range entries {
		if err := removeTransientTree(child, entry.Name(), budget); err != nil {
			child.Close()
			return err
		}
	}
	if err := child.Close(); err != nil {
		return err
	}
	if err := parent.Remove(name); err != nil {
		return err
	}
	return syncStateDirectory(parent)
}

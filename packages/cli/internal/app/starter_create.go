package app

import (
	"context"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

const (
	maxEmbeddedStarterBytes     int64 = 8 * 1024 * 1024
	maxEmbeddedStarterFileBytes int64 = 1024 * 1024
)

type starterCreateData struct {
	Created          bool   `json:"created"`
	Initialized      bool   `json:"initialized"`
	TemplateVersion  string `json:"template_version,omitempty"`
	FileCount        int    `json:"file_count,omitempty"`
	ExpandedByteSize int64  `json:"expanded_byte_size,omitempty"`
	NextCommand      string `json:"next_command,omitempty"`
}

type embeddedStarterSnapshot struct {
	version string
	files   map[string][]byte
	size    int64
}

var resolveEmbeddedStarterTemplate = func() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return "", err
	}
	pluginRoot := filepath.Clean(filepath.Join(filepath.Dir(executable), "..", ".."))
	candidate := filepath.Join(pluginRoot, "starter-template")
	info, err := os.Lstat(candidate)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", errors.New("embedded Starter Template is unavailable")
	}
	return candidate, nil
}

func runStarter(ctx context.Context, started time.Time, args []string) contract.Result {
	if len(args) == 0 || args[0] != "create" {
		return parseError(started, "starter create", "starter requires the create subcommand", map[string]any{})
	}
	set := newFlagSet("starter create")
	project := set.String("project", "", "new Godot project directory")
	if err := rejectDuplicateFlags(args[1:]); err != nil {
		return parseError(started, "starter create", err.Error(), map[string]any{})
	}
	if err := set.Parse(args[1:]); err != nil || set.NArg() != 0 || strings.TrimSpace(*project) == "" {
		return parseError(started, "starter create", "starter create accepts one required --project directory", map[string]any{})
	}

	result := contract.NewResult(started, contract.Command{Name: "starter create", Arguments: map[string]any{"project": "provided"}})
	data := starterCreateData{Initialized: false}
	if err := ctx.Err(); err != nil {
		return finishStarterCancelled(started, result, data)
	}
	if !starterCreatePlatformReady() {
		failure := prerequisiteError("STARTER_CREATE_HOST_NOT_VERIFIED", "Atomic Starter publication is not verified on this host.", "Use the verified macOS Apple Silicon Plugin path or wait for native target-host validation.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Starter project creation is unavailable on this host.", data, failure)
		return result
	}

	templateRoot, err := resolveEmbeddedStarterTemplate()
	if err != nil {
		failure := prerequisiteError("STARTER_TEMPLATE_UNAVAILABLE", "The complete embedded Starter Template could not be located beside this Plugin CLI.", "Reinstall the complete Codex Game Atelier Plugin, then retry.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Starter project creation requires the complete Plugin bundle.", data, failure)
		return result
	}
	snapshot, err := loadEmbeddedStarterSnapshot(ctx, templateRoot)
	if err != nil {
		if ctx.Err() != nil {
			return finishStarterCancelled(started, result, data)
		}
		failure := prerequisiteError("STARTER_TEMPLATE_INVALID", "The embedded Starter Template failed its fixed integrity and version contract.", "Reinstall the supported Codex Game Atelier Plugin; do not copy files from an unverified bundle.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Starter project creation rejected an invalid Plugin payload.", data, failure)
		return result
	}
	data.TemplateVersion = snapshot.version
	data.FileCount = len(snapshot.files)
	data.ExpandedByteSize = snapshot.size
	data.NextCommand = "initialize --project <created-project>"

	created, durabilityUnconfirmed, err := createStarterProject(ctx, *project, result.RunID, snapshot)
	data.Created = created
	if err != nil {
		if ctx.Err() != nil && !created {
			return finishStarterCancelled(started, result, data)
		}
		code := "STARTER_CREATE_FAILED"
		message := "The Starter project could not be created without overwriting or exposing a partial target."
		if errors.Is(err, os.ErrExist) {
			code = "STARTER_TARGET_EXISTS"
			message = "The requested project path already exists and was not modified."
		} else if errors.Is(err, errStarterAtomicPublishUnsupported) {
			code = "STARTER_ATOMIC_PUBLISH_UNSUPPORTED"
			message = "This filesystem or host cannot publish the Starter directory atomically without replacement."
		} else if durabilityUnconfirmed {
			code = "STARTER_DURABILITY_UNCONFIRMED"
			message = "The Starter project is visible, but parent-directory durability could not be confirmed."
		}
		failure := contract.Error{Code: code, Category: "state", Message: message, Retryable: !created, Details: map[string]any{"reason": safeStateFailureReason(err)}}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Starter project creation did not complete cleanly.", data, failure)
		return result
	}

	result.Finish(started, time.Now().UTC(), "PASS", contract.ExitOK, "A verified Starter project was created atomically without initializing project state.", data)
	return result
}

func loadEmbeddedStarterSnapshot(ctx context.Context, directory string) (embeddedStarterSnapshot, error) {
	info, err := os.Lstat(directory)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return embeddedStarterSnapshot{}, errors.New("embedded Starter root is unsafe")
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return embeddedStarterSnapshot{}, err
	}
	defer root.Close()

	expectedFiles := expectedStarterArchiveFiles()
	if err := verifyEmbeddedStarterClosure(root, expectedFiles); err != nil {
		return embeddedStarterSnapshot{}, err
	}
	contents := make(map[string][]byte, len(expectedFiles))
	inventory := make([]distributionFileRecord, 0, len(expectedFiles)-1)
	var total int64
	for name := range expectedFiles {
		data, fileInfo, readErr := readDistributionRootFile(ctx, root, name, maxEmbeddedStarterFileBytes)
		if readErr != nil {
			return embeddedStarterSnapshot{}, readErr
		}
		contents[name] = data
		if name != starterManifestName {
			record := makeDistributionFileRecord(name, data, fileInfo.Mode())
			if runtime.GOOS == "windows" {
				record.Mode = 0o644
			}
			inventory = append(inventory, record)
			total += int64(len(data))
			if total > maxEmbeddedStarterBytes {
				return embeddedStarterSnapshot{}, errors.New("embedded Starter exceeds its aggregate bound")
			}
		}
	}
	sort.Slice(inventory, func(i, j int) bool { return inventory[i].Path < inventory[j].Path })

	var manifest starterPackageManifest
	if err := decodeStrictDistributionJSON(contents[starterManifestName], &manifest); err != nil {
		return embeddedStarterSnapshot{}, errors.New("embedded Starter manifest is invalid")
	}
	if manifest.SchemaVersion != "1.0.0" || manifest.Template.Name != "codex-game-atelier-starter" || manifest.Template.Version != Version || manifest.Pairing.Kind != "codex-plugin" || manifest.Pairing.Name != "codex-game-atelier" || manifest.Pairing.VerifiedPluginVersion != Version || manifest.Pairing.Embedded != distributionBool(true) || manifest.TelemetryEnabled != distributionBool(false) {
		return embeddedStarterSnapshot{}, errors.New("embedded Starter identity or Plugin pairing is invalid")
	}
	if manifest.Engine != (distributionEngine{Kind: "godot", Version: "4.7.2-stable", Edition: "standard", Language: "gdscript"}) || !reflect.DeepEqual(manifest.Files, inventory) || manifest.FileCount != len(inventory) || manifest.ExpandedByteSize != total {
		return embeddedStarterSnapshot{}, errors.New("embedded Starter inventory or engine contract is invalid")
	}
	for name, data := range contents {
		if name != starterManifestName && distributedConcreteModelPattern.Match(data) {
			return embeddedStarterSnapshot{}, errors.New("embedded Starter violates the model boundary")
		}
	}
	delete(contents, starterManifestName)
	return embeddedStarterSnapshot{version: Version, files: contents, size: total}, nil
}

func verifyEmbeddedStarterClosure(root *os.Root, expectedFiles map[string]bool) error {
	expectedDirectories := archiveDirectories(expectedFiles)
	children := make(map[string]map[string]bool, len(expectedDirectories))
	for directory := range expectedDirectories {
		children[directory] = make(map[string]bool)
	}
	for directory := range expectedDirectories {
		if directory == "" {
			continue
		}
		parent := path.Dir(directory)
		if parent == "." {
			parent = ""
		}
		children[parent][path.Base(directory)] = true
	}
	for name := range expectedFiles {
		parent := path.Dir(name)
		if parent == "." {
			parent = ""
		}
		children[parent][path.Base(name)] = false
	}

	for directory, expectedChildren := range children {
		name := directory
		if name == "" {
			name = "."
		}
		info, err := root.Lstat(name)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || runtime.GOOS != "windows" && info.Mode().Perm() != 0o755 {
			return errors.New("embedded Starter directory closure is invalid")
		}
		opened, err := root.Open(name)
		if err != nil {
			return err
		}
		entries, readErr := opened.ReadDir(len(expectedChildren) + 1)
		closeErr := opened.Close()
		if readErr != nil && !errors.Is(readErr, io.EOF) || closeErr != nil || len(entries) != len(expectedChildren) {
			return errors.New("embedded Starter directory entries are invalid")
		}
		for _, entry := range entries {
			directoryExpected, exists := expectedChildren[entry.Name()]
			if !exists || entry.Type()&os.ModeSymlink != 0 || entry.IsDir() != directoryExpected {
				return errors.New("embedded Starter contains an unknown or unsafe path")
			}
		}
	}
	return nil
}

func createStarterProject(ctx context.Context, requested, runID string, snapshot embeddedStarterSnapshot) (created bool, durabilityUnconfirmed bool, resultErr error) {
	absolute, err := filepath.Abs(requested)
	if err != nil {
		return false, false, err
	}
	absolute = filepath.Clean(absolute)
	targetName := filepath.Base(absolute)
	if targetName == "." || targetName == string(filepath.Separator) || targetName == "" || strings.ContainsAny(targetName, "\x00\r\n") {
		return false, false, errors.New("Starter target name is unsafe")
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		return false, false, err
	}
	parentInfo, err := os.Lstat(parent)
	if err != nil || parentInfo.Mode()&os.ModeSymlink != 0 || !parentInfo.IsDir() {
		return false, false, errors.New("Starter target parent is unsafe")
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return false, false, err
	}
	defer root.Close()
	if _, err := root.Lstat(targetName); err == nil {
		return false, false, os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, false, err
	}
	if err := ctx.Err(); err != nil {
		return false, false, err
	}

	stage := ".atelier-starter-" + strings.TrimPrefix(runID, "atelier-")
	if err := root.Mkdir(stage, 0o700); err != nil {
		return false, false, err
	}
	directories := starterSnapshotDirectories(snapshot.files)
	staged := true
	defer func() {
		if staged {
			removeStarterStage(root, stage, snapshot.files, directories)
		}
	}()

	for _, directory := range directories {
		if err := root.Mkdir(path.Join(stage, directory), 0o755); err != nil {
			return false, false, err
		}
		if err := chmodStarterPath(root, path.Join(stage, directory), 0o755); err != nil {
			return false, false, err
		}
	}
	fileNames := make([]string, 0, len(snapshot.files))
	for name := range snapshot.files {
		fileNames = append(fileNames, name)
	}
	sort.Strings(fileNames)
	for _, name := range fileNames {
		if err := ctx.Err(); err != nil {
			return false, false, err
		}
		file, err := root.OpenFile(path.Join(stage, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return false, false, err
		}
		content := snapshot.files[name]
		count, writeErr := file.Write(content)
		if writeErr == nil && count != len(content) {
			writeErr = io.ErrShortWrite
		}
		if writeErr == nil {
			writeErr = file.Sync()
		}
		if writeErr == nil {
			writeErr = file.Chmod(0o644)
		}
		if closeErr := file.Close(); writeErr == nil {
			writeErr = closeErr
		}
		if writeErr != nil {
			return false, false, writeErr
		}
	}
	for index := len(directories) - 1; index >= 0; index-- {
		if err := syncStarterDirectory(root, path.Join(stage, directories[index])); err != nil {
			return false, false, err
		}
	}
	if err := chmodStarterPath(root, stage, 0o755); err != nil {
		return false, false, err
	}
	if err := syncStarterDirectory(root, stage); err != nil {
		return false, false, err
	}
	if err := ctx.Err(); err != nil {
		return false, false, err
	}
	if err := publishStarterDirectoryNoReplace(root, stage, targetName); err != nil {
		return false, false, err
	}
	staged = false
	if err := syncStarterDirectory(root, "."); err != nil {
		return true, true, err
	}
	return true, false, nil
}

func starterSnapshotDirectories(files map[string][]byte) []string {
	set := make(map[string]bool)
	for name := range files {
		for directory := path.Dir(name); directory != "."; directory = path.Dir(directory) {
			set[directory] = true
		}
	}
	directories := make([]string, 0, len(set))
	for directory := range set {
		directories = append(directories, directory)
	}
	sort.Slice(directories, func(i, j int) bool {
		leftDepth := strings.Count(directories[i], "/")
		rightDepth := strings.Count(directories[j], "/")
		if leftDepth == rightDepth {
			return directories[i] < directories[j]
		}
		return leftDepth < rightDepth
	})
	return directories
}

func syncStarterDirectory(root *os.Root, name string) error {
	directory, err := root.Open(name)
	if err != nil {
		return err
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return err
	}
	return directory.Close()
}

func chmodStarterPath(root *os.Root, name string, mode os.FileMode) error {
	file, err := root.Open(name)
	if err != nil {
		return err
	}
	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func removeStarterStage(root *os.Root, stage string, files map[string][]byte, directories []string) {
	fileNames := make([]string, 0, len(files))
	for name := range files {
		fileNames = append(fileNames, name)
	}
	sort.Strings(fileNames)
	for _, name := range fileNames {
		_ = root.Remove(path.Join(stage, name))
	}
	for index := len(directories) - 1; index >= 0; index-- {
		_ = root.Remove(path.Join(stage, directories[index]))
	}
	_ = root.Remove(stage)
}

func finishStarterCancelled(started time.Time, result contract.Result, data starterCreateData) contract.Result {
	failure := contract.Error{Code: "COMMAND_CANCELLED", Category: "interrupted", Message: "Starter project creation was cancelled before publication.", Retryable: true}
	result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitInterrupted, "Starter project creation was cancelled.", data, failure)
	return result
}

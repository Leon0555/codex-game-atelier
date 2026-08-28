package app

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

const defaultExportTimeoutMS int64 = 180_000
const maxExportPresetBytes int64 = 1024 * 1024
const maxExportArtifactBytes int64 = 4 * 1024 * 1024 * 1024
const maxExportArchiveEntries = 4096
const maxExportArchiveEntryBytes uint64 = 512 * 1024 * 1024
const maxExportArchiveExpandedBytes uint64 = 1024 * 1024 * 1024

const machCPUTypeX8664 uint32 = 0x01000007
const machCPUTypeARM64 uint32 = 0x0100000c

type exportOptions struct {
	project             string
	profile             string
	preset              string
	godot               string
	explicitGodot       bool
	timeoutMS           int64
	allowEngineUserData bool
}

func (options exportOptions) persistedArguments() map[string]any {
	policy := "not-authorized"
	if options.allowEngineUserData {
		policy = "standard-os-location"
	}
	source := "discovery"
	if options.explicitGodot {
		source = "explicit"
	}
	return map[string]any{
		"project":          ".",
		"profile":          options.profile,
		"preset":           options.preset,
		"target":           "macos-universal2",
		"timeout_ms":       options.timeoutMS,
		"engine_user_data": policy,
		"godot_source":     source,
	}
}

type exportArtifact struct {
	Path                    string             `json:"path"`
	MediaType               string             `json:"media_type"`
	SHA256                  string             `json:"sha256"`
	ByteSize                int64              `json:"byte_size"`
	Unsigned                bool               `json:"unsigned"`
	NotNotarized            bool               `json:"not_notarized"`
	PublicDistributionReady bool               `json:"public_distribution_ready"`
	TargetSmoke             *exportTargetSmoke `json:"target_smoke,omitempty"`
}

type exportArtifactManifest struct {
	SchemaVersion string          `json:"schema_version"`
	Scope         string          `json:"scope"`
	Outcome       string          `json:"outcome"`
	Target        string          `json:"target"`
	Profile       string          `json:"profile"`
	Preset        string          `json:"preset"`
	EngineVersion string          `json:"engine_version,omitempty"`
	Artifact      *exportArtifact `json:"artifact,omitempty"`
}

func runExport(ctx context.Context, started time.Time, args []string) encodedExecution {
	return runGodotArtifactCommand(ctx, started, "export", args, true)
}

func runBuild(ctx context.Context, started time.Time, args []string) encodedExecution {
	return runGodotArtifactCommand(ctx, started, "build", args, false)
}

func runGodotArtifactCommand(ctx context.Context, started time.Time, commandName string, args []string, acceptsPreset bool) encodedExecution {
	set := newFlagSet(commandName)
	project := set.String("project", ".", "Godot project directory")
	profile := set.String("profile", "release", "export profile: debug or release")
	preset := defaultMacOSExportPreset
	if acceptsPreset {
		set.StringVar(&preset, "preset", defaultMacOSExportPreset, "fixed macOS export preset")
	}
	godot := set.String("godot", "", "Godot executable")
	timeoutMS := set.Int64("timeout-ms", defaultExportTimeoutMS, "export timeout in milliseconds")
	allowEngineUserData := set.Bool("allow-engine-user-data", false, "allow Godot's documented standard user-data directory")
	if err := rejectDuplicateFlags(args); err != nil {
		return encodeUncommittedResult(parseError(started, commandName, err.Error(), map[string]any{}))
	}
	if err := set.Parse(args); err != nil || set.NArg() != 0 || *project == "" || (*profile != "debug" && *profile != "release") || preset != defaultMacOSExportPreset || *timeoutMS < 1 || *timeoutMS > maxTimeoutMS {
		usage := commandName + " accepts --project, --profile debug|release, optional --godot, --timeout-ms, and --allow-engine-user-data"
		if acceptsPreset {
			usage = "export accepts --project, --profile debug|release, the fixed --preset, optional --godot, --timeout-ms, and --allow-engine-user-data"
		}
		return encodeUncommittedResult(parseError(started, commandName, usage, map[string]any{}))
	}
	explicitGodot := flagWasProvided(args, "godot")
	if explicitGodot && *godot == "" {
		return encodeUncommittedResult(parseError(started, commandName, "--godot requires a non-empty path", map[string]any{}))
	}
	options := exportOptions{project: *project, profile: *profile, preset: preset, godot: *godot, explicitGodot: explicitGodot, timeoutMS: *timeoutMS, allowEngineUserData: *allowEngineUserData}
	command := contract.Command{Name: commandName, Arguments: options.persistedArguments()}
	projectRoot, err := canonicalProjectRoot(*project)
	if err != nil {
		failure := prerequisiteError("GODOT_PROJECT_NOT_FOUND", "The requested project directory does not exist or cannot be resolved.", "Select an initialized Godot project directory, then run export again.")
		return encodeUncommittedResult(finishUncommittedExport(started, command, "BLOCKED", contract.ExitPrerequisite, "Godot export could not locate the project.", failure))
	}
	pinnedProjectRoot, err := os.OpenRoot(projectRoot)
	if err != nil {
		failure := contract.Error{Code: "STATE_READ_FAILED", Category: "state", Message: "The project directory could not be pinned safely.", Retryable: false}
		return encodeUncommittedResult(finishUncommittedExport(started, command, "FAIL", contract.ExitState, "Godot export could not pin the project directory.", failure))
	}
	defer pinnedProjectRoot.Close()
	stateRoot, exists, err := openExistingStateRootFromProjectRoot(pinnedProjectRoot)
	if err != nil {
		failure := contract.Error{Code: "STATE_READ_FAILED", Category: "state", Message: "The project state directory could not be opened safely.", Retryable: false}
		return encodeUncommittedResult(finishUncommittedExport(started, command, "FAIL", contract.ExitState, "Godot export could not read project state.", failure))
	}
	if !exists {
		failure := prerequisiteError("PROJECT_NOT_INITIALIZED", "The project has no .gameatelier state directory.", "Run initialize before export.")
		return encodeUncommittedResult(finishUncommittedExport(started, command, "BLOCKED", contract.ExitPrerequisite, "Codex Game Atelier project state is not initialized.", failure))
	}
	defer stateRoot.Close()
	state, stateExists, _, err := loadExistingState(stateRoot)
	if err != nil || !stateExists {
		failure := contract.Error{Code: "STATE_READ_FAILED", Category: "state", Message: "The project state file could not be read safely.", Retryable: false}
		return encodeUncommittedResult(finishUncommittedExport(started, command, "FAIL", contract.ExitState, "Godot export could not read project state.", failure))
	}

	initial := contract.NewResult(started, command)
	transaction, err := beginRun(stateRoot, state, initial, nil)
	if err != nil {
		return encodeRunBeginFailure(started, initial, err)
	}
	defer transaction.close()
	result, payload := executeGodotExport(ctx, started, initial, transaction.runRoot, stateRoot, pinnedProjectRoot, projectRoot, options)
	if fixedActionRunMustRemainIncomplete(result, transaction.runRoot, "export-"+options.profile) {
		_ = transaction.close()
		return encodeRunCommitFailure(started, initial)
	}
	committed := transaction.finish(result, []runPayload{payload})
	closeErr := transaction.close()
	if !committed.Committed {
		return encodeRunCommitFailure(started, initial)
	}
	execution := encodedExecution{resultBytes: committed.ResultBytes, exitCode: result.ExitCode}
	if committed.Err != nil || closeErr != nil {
		execution.warning = committedRunWarning
	}
	return execution
}

func executeGodotExport(ctx context.Context, started time.Time, result contract.Result, runRoot, stateRoot, pinnedProjectRoot *os.Root, projectPath string, options exportOptions) (contract.Result, runPayload) {
	if err := ctx.Err(); err != nil {
		failure := contract.Error{Code: "COMMAND_CANCELLED", Category: "cancelled", Message: "Godot export was cancelled before project inspection.", Retryable: true}
		return finishExportResult(started, result, "FAIL", contract.ExitInterrupted, "Godot export was cancelled.", "", nil, failure)
	}
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		failure := prerequisiteError("EXPORT_HOST_NOT_VERIFIED", "The first production export slice is enabled only on macOS Apple Silicon.", "Use macOS Apple Silicon or wait for native validation of the requested host.")
		return finishExportResult(started, result, "BLOCKED", contract.ExitPrerequisite, "This host is not enabled for production export.", "", nil, failure)
	}
	if !options.allowEngineUserData {
		failure := contract.Error{Code: "ENGINE_USER_DATA_NOT_AUTHORIZED", Category: "policy", Message: "Godot export may create its standard user:// directory outside the project.", Retryable: true, Remediation: "Review the documented side effect, then rerun with --allow-engine-user-data."}
		return finishExportResult(started, result, "BLOCKED", contract.ExitPrerequisite, "Godot export requires explicit authorization for Godot user data.", "", nil, failure)
	}
	projectContent, err := readPinnedProjectFile(pinnedProjectRoot)
	if err != nil {
		failure := prerequisiteError("GODOT_PROJECT_UNREADABLE", "project.godot must be a bounded regular file inside the pinned project directory.", "Replace symlinks or special files with a regular project.godot at or below 1 MiB.")
		return finishExportResult(started, result, "BLOCKED", contract.ExitPrerequisite, "Godot export could not read the project.", "", nil, failure)
	}
	usesDotNet, err := pinnedProjectUsesDotNet(ctx, pinnedProjectRoot, projectContent)
	if err != nil || usesDotNet {
		failure := prerequisiteError("GODOT_DOTNET_UNSUPPORTED", "Godot export supports only the v1.0 standard/GDScript project contract.", "Use a standard Godot GDScript project for v1.0.")
		return finishExportResult(started, result, "BLOCKED", contract.ExitPrerequisite, "Godot export only supports standard Godot projects.", "", nil, failure)
	}
	presetBefore, err := readPinnedExportPresets(pinnedProjectRoot)
	if err != nil || !validateMacOSExportPreset(presetBefore, options.preset) {
		failure := prerequisiteError("GODOT_EXPORT_PRESET_INVALID", "The fixed macOS Technical export preset is missing, unsafe, or outside the unsigned Universal 2 contract.", "Add the documented unsigned macOS Technical preset and rerun export.")
		return finishExportResult(started, result, "BLOCKED", contract.ExitPrerequisite, "Godot export preset validation did not pass.", "", nil, failure)
	}
	sources, sourceFailure := openPinnedGodotExecutionSources(pinnedProjectRoot, projectPath, options.godot, options.explicitGodot)
	if sourceFailure != nil {
		return finishExportResult(started, result, "BLOCKED", contract.ExitPrerequisite, "Godot export could not pin its execution sources.", "", nil, *sourceFailure)
	}
	defer sources.close()
	requiredTemplates, _ := requiredExportTemplateFiles(runtime.GOOS, runtime.GOARCH)
	templates, err := locateGodotExportTemplates(sources.enginePath, requiredTemplates)
	if err != nil {
		failure := prerequisiteError("GODOT_EXPORT_TEMPLATES_MISSING", "Matching Godot 4.7.2-stable macOS export templates were not found as bounded regular files.", "Install the official export templates, then rerun export.")
		return finishExportResult(started, result, "BLOCKED", contract.ExitPrerequisite, "Godot export templates are incomplete.", "", nil, failure)
	}
	artifactRoot, output, err := createExportArtifactRoot(stateRoot, result.RunID, options.profile)
	if err != nil {
		failure := contract.Error{Code: "EXPORT_ARTIFACT_PREPARE_FAILED", Category: "state", Message: "The exclusive export artifact directory could not be created safely.", Retryable: true}
		return finishExportResult(started, result, "FAIL", contract.ExitState, "Godot export could not prepare its artifact directory.", "", nil, failure)
	}
	defer artifactRoot.Close()
	projectSnapshot, err := createGodotProjectSnapshot(ctx, runRoot, pinnedProjectRoot)
	if err != nil {
		failure := prerequisiteError("GODOT_PROJECT_SNAPSHOT_UNAVAILABLE", "The project could not be copied into the bounded export snapshot without symlinks, special files, or hidden source writes.", "Keep the project within the documented snapshot bounds and replace project symlinks or special files with regular entries.")
		return finishExportResult(started, result, "BLOCKED", contract.ExitPrerequisite, "Godot export could not create its isolated project snapshot.", "", nil, failure)
	}
	internalOutput := godotProjectOutputDirectory + "/game-" + options.profile + ".zip"
	execution := runGodotExportAction(ctx, time.Duration(options.timeoutMS)*time.Millisecond, sources.runnerSource, sources.engineSource, runRoot, projectSnapshot.directory, options.profile, options.preset, internalOutput, templates)
	var snapshotArtifact *exportArtifact
	artifactValidationErr := error(nil)
	targetSmoke := targetSmokeExecution{}
	artifactCopyErr := error(nil)
	if execution.Failure == headlessFailureNone {
		snapshotArtifact, artifactValidationErr = inspectExportArtifact(ctx, projectSnapshot.root, internalOutput, output)
		if artifactValidationErr == nil {
			remaining := time.Until(started.Add(time.Duration(options.timeoutMS) * time.Millisecond))
			if remaining <= 0 {
				targetSmoke.Process = processResult{TimedOut: true, Err: context.DeadlineExceeded}
			} else {
				targetSmoke = runExportTargetSmoke(ctx, remaining, runRoot, sources.runnerSource, projectSnapshot, internalOutput)
			}
			if targetSmoke.Err == nil && classifyHeadlessResult(targetSmoke.Process) == headlessFailureNone && !containsGodotError(targetSmoke.Process.Stderr) {
				snapshotArtifact.TargetSmoke = &exportTargetSmoke{Host: "macos", Arch: "arm64", Mode: "headless-one-frame", ExitCode: 0}
				artifactCopyErr = copyGodotSnapshotArtifact(ctx, projectSnapshot, artifactRoot, options.profile)
			}
		}
	}
	projectSnapshotCleanupErr := projectSnapshot.remove(runRoot)
	if !pinnedProjectPathMatches(pinnedProjectRoot, projectPath) {
		failure := contract.Error{Code: "PROJECT_CHANGED_DURING_EXPORT", Category: "state", Message: "The project path changed while Godot export was running.", Retryable: true}
		return finishExportResult(started, result, "FAIL", contract.ExitState, "Godot export observations were discarded after the project changed.", execution.Version, nil, failure)
	}
	presetAfter, err := readPinnedExportPresets(pinnedProjectRoot)
	if err != nil || sha256.Sum256(presetBefore) != sha256.Sum256(presetAfter) {
		failure := contract.Error{Code: "GODOT_EXPORT_PRESET_CHANGED", Category: "state", Message: "export_presets.cfg changed while Godot export was running.", Retryable: true}
		return finishExportResult(started, result, "FAIL", contract.ExitState, "Godot export observations were discarded after the preset changed.", execution.Version, nil, failure)
	}
	if projectSnapshotCleanupErr != nil {
		failure := contract.Error{Code: "GODOT_PROJECT_SNAPSHOT_CLEANUP_FAILED", Category: "state", Message: "The transient project snapshot could not be removed safely.", Retryable: true}
		return finishExportResult(started, result, "FAIL", contract.ExitState, "Godot export could not close its isolated project snapshot.", execution.Version, nil, failure)
	}
	if execution.Failure != headlessFailureNone {
		outcome, exitCode, failure := mapExportExecutionFailure(execution)
		return finishExportResult(started, result, outcome, exitCode, "Godot export execution did not complete successfully.", execution.Version, nil, failure)
	}
	if artifactValidationErr != nil {
		failure := contract.Error{Code: "EXPORT_ARTIFACT_INVALID", Category: "engine", Message: "Godot exited successfully but the snapshot export artifact was missing, unsafe, or invalid.", Retryable: true}
		return finishExportResult(started, result, "FAIL", contract.ExitEngine, "Godot export artifact validation did not pass.", execution.Version, nil, failure)
	}
	if targetSmoke.Err != nil {
		failure := contract.Error{Code: "TARGET_SMOKE_PREPARE_FAILED", Category: "state", Message: "The exported app could not be staged or pinned for target smoke.", Retryable: true}
		return finishExportResult(started, result, "FAIL", contract.ExitState, "Godot target smoke could not start safely.", execution.Version, nil, failure)
	}
	if smokeFailure := classifyHeadlessResult(targetSmoke.Process); smokeFailure != headlessFailureNone || containsGodotError(targetSmoke.Process.Stderr) {
		outcome, exitCode, failure := mapTargetSmokeFailure(targetSmoke.Process, smokeFailure)
		return finishExportResult(started, result, outcome, exitCode, "Godot exported-app target smoke did not pass.", execution.Version, nil, failure)
	}
	if artifactCopyErr != nil {
		failure := contract.Error{Code: "EXPORT_ARTIFACT_COPY_FAILED", Category: "state", Message: "The snapshot export artifact could not be copied into its declared project-state path.", Retryable: true}
		return finishExportResult(started, result, "FAIL", contract.ExitState, "Godot export artifact could not be published from the isolated snapshot.", execution.Version, nil, failure)
	}
	artifact, err := inspectExportArtifact(ctx, artifactRoot, "game-"+options.profile+".zip", output)
	if err != nil {
		failure := contract.Error{Code: "EXPORT_ARTIFACT_INVALID", Category: "engine", Message: "Godot exited successfully but the fixed export artifact was missing, unsafe, or invalid.", Retryable: true}
		return finishExportResult(started, result, "FAIL", contract.ExitEngine, "Godot export artifact validation did not pass.", execution.Version, nil, failure)
	}
	artifact.TargetSmoke = snapshotArtifact.TargetSmoke
	summary := "Godot macOS technical export passed."
	if result.Command.Name == "build" {
		summary = "Godot macOS technical build passed."
	}
	return finishExportResult(started, result, "PASS", contract.ExitOK, summary, execution.Version, artifact)
}

func mapTargetSmokeFailure(process processResult, failure godotHeadlessFailure) (string, int, contract.Error) {
	details := map[string]any{"stage": "target-smoke"}
	if process.ExitCode != nil {
		details["process_exit_code"] = *process.ExitCode
	}
	switch failure {
	case headlessFailureCancelled:
		return "FAIL", contract.ExitInterrupted, contract.Error{Code: "COMMAND_CANCELLED", Category: "cancelled", Message: "Exported-app target smoke was cancelled.", Retryable: true, Details: details}
	case headlessFailureTimeout:
		return "FAIL", contract.ExitInterrupted, contract.Error{Code: "TARGET_SMOKE_TIMEOUT", Category: "timeout", Message: "Exported-app target smoke exceeded the remaining command timeout.", Retryable: true, Details: details}
	case headlessFailureOutputTruncated:
		return "FAIL", contract.ExitEngine, contract.Error{Code: "TARGET_SMOKE_OUTPUT_TRUNCATED", Category: "engine", Message: "Exported-app target smoke output exceeded its bounded capture limit.", Retryable: false, Details: details}
	case headlessFailureEngineErrors:
		return "FAIL", contract.ExitEngine, contract.Error{Code: "TARGET_SMOKE_REPORTED_ERRORS", Category: "engine", Message: "Exported app emitted one or more ERROR records during target smoke.", Retryable: true, Details: details}
	default:
		if containsGodotError(process.Stderr) {
			return "FAIL", contract.ExitEngine, contract.Error{Code: "TARGET_SMOKE_REPORTED_ERRORS", Category: "engine", Message: "Exported app emitted one or more ERROR records during target smoke.", Retryable: true, Details: details}
		}
		return "FAIL", contract.ExitEngine, contract.Error{Code: "TARGET_SMOKE_FAILED", Category: "engine", Message: "Exported app failed its Apple Silicon headless startup and exit smoke.", Retryable: true, Details: details}
	}
}

func createExportArtifactRoot(stateRoot *os.Root, runID, profile string) (*os.Root, string, error) {
	if stateRoot == nil || !runIDPattern.MatchString(runID) || (profile != "debug" && profile != "release") {
		return nil, "", errors.New("artifact root inputs are invalid")
	}
	stateIdentity, ready := readRunPersistenceIdentity(stateRoot)
	if !ready {
		return nil, "", errors.New("artifact persistence identity is unavailable")
	}
	artifacts, err := openOrCreateVerifiedDirectory(stateRoot, "artifacts", false)
	if err != nil {
		return nil, "", err
	}
	defer artifacts.Close()
	if identity, ok := readRunPersistenceIdentity(artifacts); !ok || identity != stateIdentity {
		return nil, "", errors.New("artifact root is outside the state filesystem")
	}
	artifactRoot, err := openOrCreateVerifiedDirectory(artifacts, runID, true)
	if err != nil {
		return nil, "", err
	}
	if identity, ok := readRunPersistenceIdentity(artifactRoot); !ok || identity != stateIdentity {
		artifactRoot.Close()
		return nil, "", errors.New("artifact run root is outside the state filesystem")
	}
	output := ".gameatelier/artifacts/" + runID + "/game-" + profile + ".zip"
	return artifactRoot, output, nil
}

func readPinnedExportPresets(root *os.Root) ([]byte, error) {
	if root == nil {
		return nil, errors.New("project root is not open")
	}
	info, err := root.Lstat("export_presets.cfg")
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, errors.New("export preset file is not regular")
	}
	file, err := root.Open("export_presets.cfg")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) || opened.Size() < 1 || opened.Size() > maxExportPresetBytes {
		return nil, errors.New("export preset file exceeds its bound")
	}
	content, err := io.ReadAll(io.LimitReader(file, maxExportPresetBytes+1))
	if err != nil || int64(len(content)) > maxExportPresetBytes || !utf8.Valid(content) {
		return nil, errors.New("export preset file is unreadable")
	}
	return content, nil
}

func validateMacOSExportPreset(content []byte, preset string) bool {
	sections := make(map[string]map[string]string)
	section := ""
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	scanner.Buffer(make([]byte, 4096), int(maxExportPresetBytes))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			if _, duplicate := sections[section]; duplicate {
				return false
			}
			sections[section] = make(map[string]string)
			continue
		}
		separator := strings.IndexByte(line, '=')
		if section == "" || separator < 1 {
			return false
		}
		key := strings.TrimSpace(line[:separator])
		value := strings.TrimSpace(line[separator+1:])
		if key == "" {
			return false
		}
		if _, duplicate := sections[section][key]; duplicate {
			return false
		}
		sections[section][key] = value
	}
	if scanner.Err() != nil {
		return false
	}
	for name, values := range sections {
		if !strings.HasPrefix(name, "preset.") || strings.HasSuffix(name, ".options") || godotQuotedValue(values["name"]) != preset {
			continue
		}
		options, exists := sections[name+".options"]
		if !exists {
			return false
		}
		identifier := godotQuotedValue(options["application/bundle_identifier"])
		return godotQuotedValue(values["platform"]) == "macOS" && values["script_export_mode"] == "2" && godotQuotedValue(options["binary_format/architecture"]) == "universal" && options["codesign/codesign"] == "0" && options["notarization/notarization"] == "0" && identifier != "" && len(identifier) <= 255 && safePersistedString(identifier)
	}
	return false
}

func godotQuotedValue(value string) string {
	decoded, err := strconv.Unquote(value)
	if err != nil {
		return ""
	}
	return decoded
}

func inspectExportArtifact(ctx context.Context, root *os.Root, name, relativePath string) (*exportArtifact, error) {
	info, err := root.Lstat(name)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maxExportArtifactBytes {
		return nil, errors.New("artifact is not a bounded regular file")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) || opened.Size() != info.Size() {
		return nil, errors.New("artifact changed while opening")
	}
	hash := sha256.New()
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	written, err := copyBoundedWithContext(ctx, hash, file, maxExportArtifactBytes)
	if err != nil || written != opened.Size() {
		return nil, errors.New("artifact hash did not cover the fixed file")
	}
	archive, err := zip.NewReader(file, opened.Size())
	if err != nil || len(archive.File) < 1 || len(archive.File) > maxExportArchiveEntries {
		return nil, errors.New("artifact is not a bounded ZIP archive")
	}
	appExecutableCount := 0
	var expandedBytes uint64
	for _, entry := range archive.File {
		cleaned := filepath.ToSlash(filepath.Clean(entry.Name))
		if strings.HasPrefix(cleaned, "/") || cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
			return nil, errors.New("artifact archive contains an unsafe path")
		}
		if entry.UncompressedSize64 > maxExportArchiveEntryBytes || expandedBytes > maxExportArchiveExpandedBytes-entry.UncompressedSize64 {
			return nil, errors.New("artifact archive exceeds its expanded-size bound")
		}
		expandedBytes += entry.UncompressedSize64
		if strings.Contains(cleaned, ".app/Contents/MacOS/") && !strings.HasSuffix(cleaned, "/") {
			appExecutableCount++
			if err := validateUniversal2MachO(entry); err != nil {
				return nil, err
			}
		}
	}
	if appExecutableCount != 1 {
		return nil, errors.New("artifact archive must contain exactly one Universal 2 app executable")
	}
	finalInfo, err := file.Stat()
	if err != nil || !os.SameFile(opened, finalInfo) || finalInfo.Size() != opened.Size() || finalInfo.ModTime() != opened.ModTime() {
		return nil, errors.New("artifact changed during inspection")
	}
	return &exportArtifact{Path: relativePath, MediaType: "application/zip", SHA256: hex.EncodeToString(hash.Sum(nil)), ByteSize: opened.Size(), Unsigned: true, NotNotarized: true, PublicDistributionReady: false}, nil
}

type fatMachSlice struct {
	cpu    uint32
	offset uint64
	size   uint64
}

func validateUniversal2MachO(entry *zip.File) error {
	if entry == nil || entry.UncompressedSize64 < 48 || entry.UncompressedSize64 > maxExportArchiveEntryBytes {
		return errors.New("macOS app executable is outside the Universal 2 size contract")
	}
	stream, err := entry.Open()
	if err != nil {
		return errors.New("macOS app executable could not be opened")
	}
	defer stream.Close()
	header := make([]byte, 8)
	if _, err := io.ReadFull(stream, header); err != nil {
		return errors.New("macOS app executable has a truncated fat header")
	}
	var order binary.ByteOrder
	entryBytes := 0
	switch string(header[:4]) {
	case "\xca\xfe\xba\xbe":
		order, entryBytes = binary.BigEndian, 20
	case "\xbe\xba\xfe\xca":
		order, entryBytes = binary.LittleEndian, 20
	case "\xca\xfe\xba\xbf":
		order, entryBytes = binary.BigEndian, 32
	case "\xbf\xba\xfe\xca":
		order, entryBytes = binary.LittleEndian, 32
	default:
		return errors.New("macOS app executable is not a fat Mach-O binary")
	}
	if architectures := order.Uint32(header[4:8]); architectures != 2 {
		return errors.New("macOS app executable does not contain exactly two Universal 2 architectures")
	}
	table := make([]byte, entryBytes*2)
	if _, err := io.ReadFull(stream, table); err != nil {
		return errors.New("macOS app executable has a truncated architecture table")
	}
	slices := make([]fatMachSlice, 0, 2)
	seen := make(map[uint32]bool, 2)
	for index := 0; index < 2; index++ {
		record := table[index*entryBytes : (index+1)*entryBytes]
		cpu := order.Uint32(record[:4])
		var offset, size uint64
		if entryBytes == 20 {
			offset, size = uint64(order.Uint32(record[8:12])), uint64(order.Uint32(record[12:16]))
		} else {
			offset, size = order.Uint64(record[8:16]), order.Uint64(record[16:24])
		}
		if cpu != machCPUTypeX8664 && cpu != machCPUTypeARM64 || seen[cpu] || size < 8 || offset < uint64(8+len(table)) || offset > entry.UncompressedSize64-size {
			return errors.New("macOS app executable has an invalid Universal 2 architecture table")
		}
		seen[cpu] = true
		slices = append(slices, fatMachSlice{cpu: cpu, offset: offset, size: size})
	}
	if !seen[machCPUTypeX8664] || !seen[machCPUTypeARM64] {
		return errors.New("macOS app executable lacks x86_64 or arm64")
	}
	sort.Slice(slices, func(left, right int) bool { return slices[left].offset < slices[right].offset })
	position := uint64(8 + len(table))
	var previousEnd uint64
	for _, slice := range slices {
		if previousEnd > slice.offset {
			return errors.New("macOS app executable architecture slices overlap")
		}
		if _, err := io.CopyN(io.Discard, stream, int64(slice.offset-position)); err != nil {
			return errors.New("macOS app executable architecture slice is truncated")
		}
		thinHeader := make([]byte, 8)
		if _, err := io.ReadFull(stream, thinHeader); err != nil {
			return errors.New("macOS app executable architecture header is truncated")
		}
		var thinOrder binary.ByteOrder
		switch string(thinHeader[:4]) {
		case "\xfe\xed\xfa\xcf":
			thinOrder = binary.BigEndian
		case "\xcf\xfa\xed\xfe":
			thinOrder = binary.LittleEndian
		default:
			return errors.New("macOS app executable contains a non-64-bit architecture slice")
		}
		if thinOrder.Uint32(thinHeader[4:8]) != slice.cpu {
			return errors.New("macOS app executable fat and thin CPU types differ")
		}
		position = slice.offset + 8
		previousEnd = slice.offset + slice.size
	}
	return nil
}

func finishExportResult(started time.Time, result contract.Result, outcome string, exitCode int, summary, engineVersion string, artifact *exportArtifact, failures ...contract.Error) (contract.Result, runPayload) {
	artifactCount := 0
	if artifact != nil {
		artifactCount = 1
	}
	profile, _ := result.Command.Arguments["profile"].(string)
	preset, _ := result.Command.Arguments["preset"].(string)
	data := map[string]any{"scope": "godot-export", "target": "macos-universal2", "profile": profile, "preset": preset, "artifact_count": artifactCount, "engine_version": engineVersion}
	result.Finish(started, time.Now().UTC(), outcome, exitCode, summary, data, failures...)
	report := exportArtifactManifest{SchemaVersion: contract.SchemaVersion, Scope: "godot-export", Outcome: outcome, Target: "macos-universal2", Profile: profile, Preset: preset, EngineVersion: engineVersion, Artifact: artifact}
	content, err := marshalRunJSON(report)
	if err != nil {
		content = []byte("{}\n")
	}
	return result, runPayload{Kind: "export-artifact", Outcome: outcome, MediaType: "application/json", Content: content}
}

func finishUncommittedExport(started time.Time, command contract.Command, outcome string, exitCode int, summary string, failure contract.Error) contract.Result {
	result := contract.NewResult(started, command)
	result.Finish(started, time.Now().UTC(), outcome, exitCode, summary, map[string]any{"scope": "godot-export", "recorded": false}, failure)
	return result
}

func mapExportExecutionFailure(execution godotHeadlessExecution) (string, int, contract.Error) {
	details := map[string]any{"stage": execution.FailureStage}
	process := execution.VersionProcess
	if strings.HasPrefix(execution.FailureStage, "export-") {
		process = execution.ActionProcess
	}
	if process.ExitCode != nil {
		details["process_exit_code"] = *process.ExitCode
	}
	switch execution.Failure {
	case headlessFailureCancelled:
		return "FAIL", contract.ExitInterrupted, contract.Error{Code: "COMMAND_CANCELLED", Category: "cancelled", Message: "Godot export was cancelled.", Retryable: true, Details: details}
	case headlessFailureTimeout:
		return "FAIL", contract.ExitInterrupted, contract.Error{Code: "GODOT_TIMEOUT", Category: "timeout", Message: "Godot export exceeded its total timeout.", Retryable: true, Details: details}
	case headlessFailureOutputTruncated:
		return "FAIL", contract.ExitEngine, contract.Error{Code: "GODOT_OUTPUT_TRUNCATED", Category: "engine", Message: "Godot export output exceeded the bounded capture limit and was not trusted.", Retryable: false, Details: details}
	case headlessFailureUnsupportedVersion:
		return "BLOCKED", contract.ExitPrerequisite, prerequisiteError("GODOT_VERSION_UNSUPPORTED", "The selected executable did not report the supported Godot 4.7.2-stable official standard build identifier.", "Install or select the official standard Godot 4.7.2-stable executable.")
	case headlessFailureSnapshotUnavailable:
		return "BLOCKED", contract.ExitPrerequisite, prerequisiteError("GODOT_EXPORT_RUNTIME_UNAVAILABLE", "The fixed export runtime could not be snapshotted safely.", "Install the complete CLI pair and official matching export templates, then retry.")
	case headlessFailureExecutableChanged:
		return "FAIL", contract.ExitEngine, contract.Error{Code: "GODOT_EXPORT_SOURCE_CHANGED", Category: "engine", Message: "A pinned runner, engine, or export-template source changed during export.", Retryable: true, Details: details}
	case headlessFailureSnapshotCleanup:
		return "FAIL", contract.ExitState, contract.Error{Code: "GODOT_EXPORT_RUNTIME_CLEANUP_FAILED", Category: "state", Message: "The transient export runtime could not be removed safely.", Retryable: true, Details: details}
	case headlessFailureEngineErrors:
		return "FAIL", contract.ExitEngine, contract.Error{Code: "GODOT_REPORTED_ERRORS", Category: "engine", Message: "Godot emitted one or more ERROR records during export.", Retryable: true, Details: details}
	default:
		return "FAIL", contract.ExitEngine, contract.Error{Code: "GODOT_PROCESS_FAILED", Category: "engine", Message: "Godot failed during export.", Retryable: true, Details: details}
	}
}

func validatePersistedExportArguments(arguments map[string]any) error {
	if len(arguments) != 7 || arguments["project"] != "." || arguments["preset"] != defaultMacOSExportPreset || arguments["target"] != "macos-universal2" {
		return errors.New("persisted export arguments violate the fixed macOS contract")
	}
	profile, ok := arguments["profile"].(string)
	if !ok || profile != "debug" && profile != "release" {
		return errors.New("persisted export profile is invalid")
	}
	timeout, ok := persistedInteger(arguments["timeout_ms"])
	if !ok || timeout < 1 || timeout > maxTimeoutMS {
		return errors.New("persisted export timeout is invalid")
	}
	policy, ok := arguments["engine_user_data"].(string)
	if !ok || policy != "not-authorized" && policy != "standard-os-location" {
		return errors.New("persisted export user-data policy is invalid")
	}
	source, ok := arguments["godot_source"].(string)
	if !ok || source != "explicit" && source != "discovery" {
		return errors.New("persisted export Godot source is invalid")
	}
	return nil
}

func preflightExportRunFinish(result contract.Result, payloads []runPayload) error {
	data, ok := result.Data.(map[string]any)
	if !ok || len(data) != 6 || data["scope"] != "godot-export" || data["target"] != "macos-universal2" || data["preset"] != defaultMacOSExportPreset {
		return errors.New("export result data violates its bounded contract")
	}
	profile, profileOK := data["profile"].(string)
	artifactCount, countOK := persistedInteger(data["artifact_count"])
	engineVersion, versionOK := data["engine_version"].(string)
	if !profileOK || profile != "debug" && profile != "release" || !countOK || artifactCount < 0 || artifactCount > 1 || !versionOK || engineVersion != "" && !supportedGodotVersion.MatchString(engineVersion) {
		return errors.New("export result fields are invalid")
	}
	if len(payloads) != 1 || payloads[0].Kind != "export-artifact" || payloads[0].Outcome != result.Outcome || payloads[0].MediaType != "application/json" || len(payloads[0].Metadata) != 0 || int64(len(payloads[0].Content)) > maxRunPayloadBytes {
		return errors.New("export evidence payload violates its closure contract")
	}
	var manifest exportArtifactManifest
	if err := decodeStrictRunJSON(payloads[0].Content, &manifest); err != nil {
		return errors.New("export artifact manifest is invalid")
	}
	if manifest.SchemaVersion != contract.SchemaVersion || manifest.Scope != "godot-export" || manifest.Outcome != result.Outcome || manifest.Target != "macos-universal2" || manifest.Profile != profile || manifest.Preset != defaultMacOSExportPreset || manifest.EngineVersion != engineVersion {
		return errors.New("export artifact manifest conflicts with its result")
	}
	if result.Outcome == "PASS" {
		if artifactCount != 1 || engineVersion == "" || manifest.Artifact == nil {
			return errors.New("passing export lacks its verified artifact")
		}
		artifact := manifest.Artifact
		expectedPath := ".gameatelier/artifacts/" + result.RunID + "/game-" + profile + ".zip"
		if artifact.Path != expectedPath || artifact.MediaType != "application/zip" || len(artifact.SHA256) != 64 || artifact.ByteSize < 1 || artifact.ByteSize > maxExportArtifactBytes || !artifact.Unsigned || !artifact.NotNotarized || artifact.PublicDistributionReady || artifact.TargetSmoke == nil {
			return errors.New("passing export artifact metadata is invalid")
		}
		if smoke := artifact.TargetSmoke; smoke.Host != "macos" || smoke.Arch != "arm64" || smoke.Mode != "headless-one-frame" || smoke.ExitCode != 0 {
			return errors.New("passing export target smoke metadata is invalid")
		}
		if _, err := hex.DecodeString(artifact.SHA256); err != nil {
			return errors.New("passing export artifact hash is invalid")
		}
	} else if artifactCount != 0 || manifest.Artifact != nil {
		return errors.New("non-passing export must not publish an artifact")
	}
	return nil
}

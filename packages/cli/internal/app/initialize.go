package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"time"
	"unicode/utf8"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

type initializeData struct {
	Initialized   bool                `json:"initialized"`
	Created       bool                `json:"created"`
	StatePath     string              `json:"state_path"`
	ProjectID     string              `json:"project_id,omitempty"`
	SchemaVersion string              `json:"schema_version,omitempty"`
	Revision      *int64              `json:"revision,omitempty"`
	Mode          string              `json:"mode,omitempty"`
	Engine        *projectStateEngine `json:"engine,omitempty"`
	UpdatedAt     string              `json:"updated_at,omitempty"`
}

func runInitialize(started time.Time, args []string) contract.Result {
	set := newFlagSet("initialize")
	project := set.String("project", ".", "Godot project directory")
	if err := rejectDuplicateFlags(args); err != nil {
		return parseError(started, "initialize", err.Error(), map[string]any{})
	}
	if err := set.Parse(args); err != nil || set.NArg() != 0 || *project == "" {
		return parseError(started, "initialize", "initialize accepts --project only", map[string]any{})
	}

	result := contract.NewResult(started, contract.Command{Name: "initialize", Arguments: map[string]any{}})
	data := initializeData{StatePath: ".gameatelier/project.json"}
	projectData, projectFailure := discoverProject(*project)
	if projectFailure != nil {
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Codex Game Atelier initialization requires an existing Godot project.", data, *projectFailure)
		return result
	}
	if !currentHostData().Supported {
		failure := prerequisiteError("HOST_UNSUPPORTED", "This host is outside the v1.0 Tier 1 support matrix.", "Initialize on macOS Apple Silicon, Windows x64, or Linux x64.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Codex Game Atelier initialization is unavailable on this host.", data, failure)
		return result
	}
	if !initializePlatformReady() {
		failure := prerequisiteError("INITIALIZE_HOST_NOT_VERIFIED", "Atomic state initialization is not yet enabled on this host.", "Use the currently verified macOS Apple Silicon implementation or wait for native target-host validation.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Atomic initialization is not yet verified on this host.", data, failure)
		return result
	}
	usesDotNet, err := projectUsesDotNet(projectData.Root)
	if err != nil {
		failure := prerequisiteError("GODOT_PROJECT_UNREADABLE", "project.godot or its project directory could not be read within the initialization safety bounds.", "Check project permissions and keep project.godot at or below 1 MiB.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "The Godot project could not be inspected safely.", data, failure)
		return result
	}
	if usesDotNet {
		failure := prerequisiteError("GODOT_DOTNET_UNSUPPORTED", "This project appears to use Godot .NET/C#, which is outside the v1.0 support matrix.", "Use a standard Godot GDScript project for v1.0.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Godot .NET/C# projects are not initialized by v1.0.", data, failure)
		return result
	}

	existingRoot, rootExists, err := openExistingStateRoot(projectData.Root)
	if err != nil {
		return initializeStateFailure(started, result, data, "STATE_CONFLICT", "Existing project state is invalid or unsafe and was not modified.", err, false)
	}
	if rootExists {
		existingState, stateExists, stateInfo, readErr := loadExistingState(existingRoot)
		if readErr != nil {
			_ = existingRoot.Close()
			return initializeStateFailure(started, result, data, "STATE_CONFLICT", "Existing project state is invalid or unsafe and was not modified.", readErr, false)
		}
		if stateExists {
			populateInitializeData(&data, existingState, false)
			if syncErr := confirmExistingStateDurability(existingRoot, stateInfo); syncErr != nil {
				_ = existingRoot.Close()
				return initializeStateFailureWithData(started, result, data, "STATE_DURABILITY_UNCONFIRMED", "Existing project state is valid, but its durability could not be confirmed.", syncErr, true)
			}
			_ = existingRoot.Close()
			result.Finish(started, time.Now().UTC(), "PASS", contract.ExitOK, "Codex Game Atelier project state is already initialized; no files were changed.", data)
			return result
		}
		_ = existingRoot.Close()
	}

	stateRoot, err := openOrCreateStateRoot(projectData.Root)
	if err != nil {
		return initializeStateFailure(started, result, data, "STATE_WRITE_FAILED", "The state directory could not be created or opened safely.", err, false)
	}
	defer stateRoot.Close()

	lock, err := acquireProjectStateLock(stateRoot)
	if err != nil {
		if errors.Is(err, errStateLocked) {
			return initializeStateFailure(started, result, data, "STATE_LOCKED", "Another process currently owns the project-state write lock.", err, true)
		}
		return initializeStateFailure(started, result, data, "STATE_WRITE_FAILED", "The project-state lock could not be acquired safely.", err, false)
	}
	defer lock.release()

	state, exists, stateInfo, err := loadExistingState(stateRoot)
	if err != nil {
		return initializeStateFailure(started, result, data, "STATE_CONFLICT", "Existing project state is invalid or unsafe and was not modified.", err, false)
	}
	if exists {
		populateInitializeData(&data, state, false)
		if syncErr := confirmExistingStateDurability(stateRoot, stateInfo); syncErr != nil {
			return initializeStateFailureWithData(started, result, data, "STATE_DURABILITY_UNCONFIRMED", "Existing project state is valid, but its durability could not be confirmed.", syncErr, true)
		}
		result.Finish(started, time.Now().UTC(), "PASS", contract.ExitOK, "Codex Game Atelier project state is already initialized; no files were changed.", data)
		return result
	}

	projectID, err := newProjectID()
	if err != nil {
		failure := contract.Error{Code: "PROJECT_ID_GENERATION_FAILED", Category: "internal", Message: "A secure project identifier could not be generated.", Retryable: false}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitInternal, "Project initialization could not generate an identifier.", data, failure)
		return result
	}
	finished := time.Now().UTC()
	state = projectState{
		SchemaVersion: contract.SchemaVersion,
		ProjectID:     projectID,
		Revision:      0,
		Mode:          "standard",
		Engine: projectStateEngine{
			Kind:             "godot",
			RequestedVersion: "4.7.2-stable",
			Language:         "gdscript",
		},
		TaskRefs:      []string{},
		ActiveRunRefs: []string{},
		UpdatedAt:     finished.Format(time.RFC3339Nano),
	}
	stateBytes, err := marshalState(state)
	if err != nil {
		return initializeStateFailure(started, result, data, "STATE_WRITE_FAILED", "The initial project state could not be encoded.", err, false)
	}
	publishResult := publishProjectState(stateRoot, result.RunID, stateBytes)
	if publishResult.err != nil {
		if publishResult.targetExists {
			concurrentState, concurrentExists, concurrentInfo, readErr := loadExistingState(stateRoot)
			if readErr == nil && concurrentExists {
				populateInitializeData(&data, concurrentState, false)
				if syncErr := confirmExistingStateDurability(stateRoot, concurrentInfo); syncErr != nil {
					return initializeStateFailureWithData(started, result, data, "STATE_DURABILITY_UNCONFIRMED", "Concurrently initialized project state is valid, but its durability could not be confirmed.", syncErr, true)
				}
				result.Finish(started, time.Now().UTC(), "PASS", contract.ExitOK, "Codex Game Atelier project state was initialized concurrently; no existing state was overwritten.", data)
				return result
			}
		}
		code := "STATE_WRITE_FAILED"
		message := "The initial project state could not be published atomically."
		if publishResult.atomicUnsupported {
			code = "STATE_ATOMIC_WRITE_UNSUPPORTED"
			message = "This filesystem does not support the required no-replace atomic state publication."
		}
		return initializeStateFailure(started, result, data, code, message, publishResult.err, false)
	}

	populateInitializeData(&data, state, true)
	if publishResult.durabilityErr != nil {
		return initializeStateFailureWithData(started, result, data, "STATE_DURABILITY_UNCONFIRMED", "Project state is complete and visible, but directory durability could not be confirmed.", publishResult.durabilityErr, true)
	}
	result.Finish(started, time.Now().UTC(), "PASS", contract.ExitOK, "Codex Game Atelier project state was initialized atomically.", data)
	return result
}

func newProjectID() (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return "project-" + hex.EncodeToString(random), nil
}

func marshalState(state projectState) ([]byte, error) {
	content, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return nil, err
	}
	content = append(content, '\n')
	if !utf8.Valid(content) || int64(len(content)) > maxProjectStateBytes {
		return nil, errors.New("encoded project state is outside the supported bounds")
	}
	return content, nil
}

func loadExistingState(stateRoot *os.Root) (projectState, bool, os.FileInfo, error) {
	info, err := stateRoot.Lstat("project.json")
	if errors.Is(err, os.ErrNotExist) {
		return projectState{}, false, nil, nil
	}
	if err != nil {
		return projectState{}, false, nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return projectState{}, false, nil, errors.New("project state target is not a regular file")
	}
	content, err := readStateFromRootSafely(stateRoot, info)
	if err != nil {
		return projectState{}, false, nil, err
	}
	version, err := inspectSchemaVersion(content)
	if err != nil {
		return projectState{}, false, nil, err
	}
	if version != contract.SchemaVersion {
		return projectState{}, false, nil, errors.New("project state schema version is unsupported")
	}
	state, err := decodeProjectState(content)
	if err != nil {
		return projectState{}, false, nil, err
	}
	if err := validateProjectState(state); err != nil {
		return projectState{}, false, nil, err
	}
	return state, true, info, nil
}

func populateInitializeData(data *initializeData, state projectState, created bool) {
	revision := int64(state.Revision)
	data.Initialized = true
	data.Created = created
	data.ProjectID = state.ProjectID
	data.SchemaVersion = state.SchemaVersion
	data.Revision = &revision
	data.Mode = state.Mode
	data.Engine = &state.Engine
	data.UpdatedAt = state.UpdatedAt
}

func confirmExistingStateDurability(stateRoot *os.Root, expected os.FileInfo) error {
	info, err := stateRoot.Lstat("project.json")
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || expected == nil || !os.SameFile(expected, info) {
		return errors.New("project state is not a regular file")
	}
	file, err := stateRoot.Open("project.json")
	if err != nil {
		return err
	}
	openedInfo, err := file.Stat()
	if err != nil || !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		_ = file.Close()
		return errors.New("project state changed while opening")
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return syncStateDirectory(stateRoot)
}

func initializeStateFailure(started time.Time, result contract.Result, data initializeData, code, message string, err error, retryable bool) contract.Result {
	return initializeStateFailureWithData(started, result, data, code, message, err, retryable)
}

func initializeStateFailureWithData(started time.Time, result contract.Result, data initializeData, code, message string, err error, retryable bool) contract.Result {
	failure := contract.Error{Code: code, Category: "state", Message: message, Retryable: retryable, Details: map[string]any{"reason": safeStateFailureReason(err)}}
	result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Codex Game Atelier project state was not initialized cleanly.", data, failure)
	return result
}

func safeStateFailureReason(err error) string {
	switch {
	case errors.Is(err, errStateLocked):
		return "state lock is held"
	case errors.Is(err, os.ErrPermission):
		return "permission denied"
	case errors.Is(err, os.ErrExist):
		return "target already exists"
	case errors.Is(err, os.ErrNotExist):
		return "required path is missing"
	default:
		return "filesystem safety check failed"
	}
}

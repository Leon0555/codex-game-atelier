package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

const maxProjectStateBytes int64 = 1024 * 1024

type projectStateEngine struct {
	Kind             string `json:"kind"`
	RequestedVersion string `json:"requested_version"`
	Language         string `json:"language"`
}

type schemaInteger int64

func (integer *schemaInteger) UnmarshalJSON(data []byte) error {
	number, ok := new(big.Rat).SetString(string(data))
	if !ok || !number.IsInt() {
		return errors.New("revision must be a finite mathematical integer")
	}
	numerator := number.Num()
	if !numerator.IsInt64() {
		return errors.New("revision is outside the supported integer range")
	}
	value := numerator.Int64()
	*integer = schemaInteger(value)
	return nil
}

type projectState struct {
	SchemaVersion        string             `json:"schema_version"`
	ProjectID            string             `json:"project_id"`
	Revision             schemaInteger      `json:"revision"`
	Mode                 string             `json:"mode"`
	Engine               projectStateEngine `json:"engine"`
	TaskRefs             []string           `json:"task_refs"`
	ActiveRunRefs        []string           `json:"active_run_refs"`
	LastCommandResultRef string             `json:"last_command_result_ref,omitempty"`
	UpdatedAt            string             `json:"updated_at"`
	lastResultPresent    bool               `json:"-"`
}

type statusData struct {
	Initialized           bool                `json:"initialized"`
	StatePath             string              `json:"state_path"`
	ProjectID             string              `json:"project_id,omitempty"`
	SchemaVersion         string              `json:"schema_version,omitempty"`
	ObservedSchemaVersion string              `json:"observed_schema_version,omitempty"`
	Revision              *int64              `json:"revision,omitempty"`
	Mode                  string              `json:"mode,omitempty"`
	Engine                *projectStateEngine `json:"engine,omitempty"`
	TaskRefCount          int                 `json:"task_ref_count"`
	ActiveRunRefCount     int                 `json:"active_run_ref_count"`
	LastCommandResultRef  string              `json:"last_command_result_ref,omitempty"`
	UpdatedAt             string              `json:"updated_at,omitempty"`
}

func runStatus(started time.Time, args []string) contract.Result {
	set := newFlagSet("status")
	project := set.String("project", ".", "project directory")
	if err := rejectDuplicateFlags(args); err != nil {
		return parseError(started, "status", err.Error(), map[string]any{})
	}
	if err := set.Parse(args); err != nil || set.NArg() != 0 || *project == "" {
		return parseError(started, "status", "status accepts --project only", map[string]any{})
	}
	result := contract.NewResult(started, contract.Command{Name: "status", Arguments: map[string]any{"project": *project}})
	data := statusData{StatePath: ".gameatelier/project.json"}
	root, err := canonicalProjectRoot(*project)
	if err != nil {
		failure := contract.Error{Code: "STATE_READ_FAILED", Category: "state", Message: "The requested project directory cannot be resolved.", Retryable: false}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Project state could not be read.", data, failure)
		return result
	}
	content, err := readStateFileSafely(root)
	if errors.Is(err, os.ErrNotExist) {
		failure := prerequisiteError("PROJECT_NOT_INITIALIZED", "The project has no .gameatelier/project.json state file.", "Run the future initialize command before using project state workflows.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Codex Game Atelier project state is not initialized.", data, failure)
		return result
	}
	if err != nil {
		failure := contract.Error{Code: "STATE_READ_FAILED", Category: "state", Message: "The project state file could not be read safely.", Retryable: false}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Project state could not be read.", data, failure)
		return result
	}

	schemaVersion, err := inspectSchemaVersion(content)
	if err != nil {
		failure := contract.Error{Code: "STATE_INVALID", Category: "state", Message: "The project state file is invalid.", Retryable: false, Details: map[string]any{"reason": boundedDiagnostic(err.Error())}}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Project state is invalid.", data, failure)
		return result
	}
	data.ObservedSchemaVersion = schemaVersion
	if schemaVersion != contract.SchemaVersion {
		failure := contract.Error{Code: "STATE_SCHEMA_UNSUPPORTED", Category: "state", Message: "The project state schema version is unsupported.", Retryable: false, Details: map[string]any{"schema_version": boundedDiagnostic(schemaVersion)}}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Project state schema is unsupported.", data, failure)
		return result
	}
	state, err := decodeProjectState(content)
	if err != nil {
		failure := contract.Error{Code: "STATE_INVALID", Category: "state", Message: "The project state file is invalid.", Retryable: false, Details: map[string]any{"reason": boundedDiagnostic(err.Error())}}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Project state is invalid.", data, failure)
		return result
	}
	if err := validateProjectState(state); err != nil {
		failure := contract.Error{Code: "STATE_INVALID", Category: "state", Message: "The project state file violates the v1 contract.", Retryable: false, Details: map[string]any{"reason": boundedDiagnostic(err.Error())}}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitState, "Project state is invalid.", data, failure)
		return result
	}

	data.Initialized = true
	data.ProjectID = state.ProjectID
	data.SchemaVersion = state.SchemaVersion
	data.ObservedSchemaVersion = ""
	revision := int64(state.Revision)
	data.Revision = &revision
	data.Mode = state.Mode
	data.Engine = &state.Engine
	data.TaskRefCount = len(state.TaskRefs)
	data.ActiveRunRefCount = len(state.ActiveRunRefs)
	data.LastCommandResultRef = state.LastCommandResultRef
	data.UpdatedAt = state.UpdatedAt
	result.Finish(started, time.Now().UTC(), "PASS", contract.ExitOK, "Codex Game Atelier project state was read without modification.", data)
	return result
}

func readStateFileSafely(root string) ([]byte, error) {
	file, err := openStateFile(root)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("project state must be a regular file")
	}
	content, err := io.ReadAll(io.LimitReader(file, maxProjectStateBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > maxProjectStateBytes {
		return nil, errors.New("project state exceeds 1 MiB")
	}
	return content, nil
}

func decodeProjectState(content []byte) (projectState, error) {
	if !utf8.Valid(content) {
		return projectState{}, errors.New("project state is not valid UTF-8")
	}
	duplicateDecoder := json.NewDecoder(bytes.NewReader(content))
	duplicateDecoder.UseNumber()
	if err := rejectDuplicateObjectKeys(duplicateDecoder); err != nil {
		return projectState{}, err
	}
	if token, err := duplicateDecoder.Token(); err != io.EOF || token != nil {
		if err == nil {
			return projectState{}, errors.New("multiple JSON values are not allowed")
		}
		return projectState{}, err
	}
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(content, &rawFields); err != nil {
		return projectState{}, err
	}
	for _, required := range []string{"schema_version", "project_id", "revision", "mode", "engine", "task_refs", "active_run_refs", "updated_at"} {
		value, exists := rawFields[required]
		if !exists || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return projectState{}, fmt.Errorf("required field %q is missing or null", required)
		}
	}
	for key, value := range rawFields {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return projectState{}, fmt.Errorf("field %q may not be null", key)
		}
		if _, allowed := allowedProjectStateFields[key]; !allowed {
			return projectState{}, fmt.Errorf("unknown field %q", key)
		}
	}
	var rawEngine map[string]json.RawMessage
	if err := json.Unmarshal(rawFields["engine"], &rawEngine); err != nil {
		return projectState{}, fmt.Errorf("engine must be an object: %w", err)
	}
	for _, required := range []string{"kind", "requested_version", "language"} {
		value, exists := rawEngine[required]
		if !exists || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return projectState{}, fmt.Errorf("required engine field %q is missing or null", required)
		}
	}
	for key := range rawEngine {
		if _, allowed := allowedProjectStateEngineFields[key]; !allowed {
			return projectState{}, fmt.Errorf("unknown engine field %q", key)
		}
	}

	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var state projectState
	if err := decoder.Decode(&state); err != nil {
		return projectState{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return projectState{}, errors.New("multiple JSON values are not allowed")
	}
	_, state.lastResultPresent = rawFields["last_command_result_ref"]
	return state, nil
}

func inspectSchemaVersion(content []byte) (string, error) {
	if !utf8.Valid(content) {
		return "", errors.New("project state is not valid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	if err := rejectDuplicateObjectKeys(decoder); err != nil {
		return "", err
	}
	if token, err := decoder.Token(); err != io.EOF || token != nil {
		if err == nil {
			return "", errors.New("multiple JSON values are not allowed")
		}
		return "", err
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(content, &envelope); err != nil {
		return "", err
	}
	rawVersion, exists := envelope["schema_version"]
	if !exists || bytes.Equal(bytes.TrimSpace(rawVersion), []byte("null")) {
		return "", errors.New("required field \"schema_version\" is missing or null")
	}
	var version string
	if err := json.Unmarshal(rawVersion, &version); err != nil || version == "" {
		return "", errors.New("schema_version must be a non-empty string")
	}
	if utf8.RuneCountInString(version) > 128 {
		return "", errors.New("schema_version exceeds 128 characters")
	}
	return version, nil
}

var allowedProjectStateFields = map[string]struct{}{
	"schema_version": {}, "project_id": {}, "revision": {}, "mode": {}, "engine": {},
	"task_refs": {}, "active_run_refs": {}, "last_command_result_ref": {}, "updated_at": {},
}

var allowedProjectStateEngineFields = map[string]struct{}{
	"kind": {}, "requested_version": {}, "language": {},
}

func rejectDuplicateObjectKeys(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("object key is not a string")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate object key %q", key)
			}
			seen[key] = struct{}{}
			if err := rejectDuplicateObjectKeys(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case '[':
		for decoder.More() {
			if err := rejectDuplicateObjectKeys(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	default:
		return errors.New("unexpected JSON delimiter")
	}
}

func validateProjectState(state projectState) error {
	if !stateIdentifier.MatchString(state.ProjectID) || state.Revision < 0 || state.Engine.RequestedVersion == "" || utf8.RuneCountInString(state.Engine.RequestedVersion) > 128 || state.UpdatedAt == "" || state.TaskRefs == nil || state.ActiveRunRefs == nil {
		return errors.New("required state fields are missing")
	}
	if state.Mode != "manual" && state.Mode != "standard" && state.Mode != "strict" {
		return errors.New("mode is invalid")
	}
	if state.Engine.Kind != "godot" || state.Engine.Language != "gdscript" {
		return errors.New("engine must be Godot with GDScript")
	}
	if !canonicalTimestamp.MatchString(state.UpdatedAt) {
		return errors.New("updated_at is invalid")
	}
	if _, err := time.Parse(time.RFC3339Nano, state.UpdatedAt); err != nil {
		return errors.New("updated_at is invalid")
	}
	if err := validateUniqueStateReferences("task_refs", state.TaskRefs); err != nil {
		return err
	}
	if err := validateUniqueStateReferences("active_run_refs", state.ActiveRunRefs); err != nil {
		return err
	}
	if state.lastResultPresent && !safeStateReference(state.LastCommandResultRef) {
		return errors.New("last_command_result_ref is unsafe")
	}
	return nil
}

var stateIdentifier = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
var stateReferenceCharacters = regexp.MustCompile(`^[a-z0-9._/-]+$`)
var canonicalTimestamp = regexp.MustCompile(`^[0-9]{4}-(?:0[1-9]|1[0-2])-(?:0[1-9]|[12][0-9]|3[01])T(?:[01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9](?:\.[0-9]+)?(?:Z|[+-](?:[01][0-9]|2[0-3]):[0-5][0-9])$`)

func validateUniqueStateReferences(field string, references []string) error {
	seen := make(map[string]struct{}, len(references))
	for _, reference := range references {
		if !safeStateReference(reference) {
			return fmt.Errorf("unsafe state reference %q", reference)
		}
		if _, exists := seen[reference]; exists {
			return fmt.Errorf("%s contains duplicate reference %q", field, reference)
		}
		seen[reference] = struct{}{}
	}
	return nil
}

func safeStateReference(reference string) bool {
	if reference == "" || utf8.RuneCountInString(reference) > 4096 || !stateReferenceCharacters.MatchString(reference) {
		return false
	}
	if strings.HasPrefix(reference, "/") || !strings.HasPrefix(reference, ".gameatelier/") {
		return false
	}
	for _, part := range strings.Split(reference, "/") {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ".") {
			return false
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" ||
			len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
			return false
		}
	}
	return true
}

func boundedDiagnostic(message string) string {
	message = strings.ToValidUTF8(strings.TrimSpace(message), "")
	runes := []rune(message)
	if len(runes) > 512 {
		return string(runes[:512])
	}
	return message
}

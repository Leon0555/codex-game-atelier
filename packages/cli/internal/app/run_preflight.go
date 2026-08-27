package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

const maxRunPayloadCount = 16
const maxRunPayloadTotalBytes int64 = 16 * 1024 * 1024
const maxPersistedJSONNodes = 4096
const maxPersistedJSONDepth = 8
const maxPersistedStringBytes = 64 * 1024

var allowedPersistedCommands = map[string]struct{}{
	"validate": {}, "test": {}, "build": {}, "export": {}, "release check": {},
}

var allowedErrorCategories = map[string]struct{}{
	"usage": {}, "validation": {}, "policy": {}, "prerequisite": {}, "engine": {},
	"timeout": {}, "cancelled": {}, "state": {}, "internal": {},
}

var errorCodePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{2,63}$`)
var windowsAbsolutePath = regexp.MustCompile(`^[A-Za-z]:[\\/]`)
var embeddedWindowsAbsolutePath = regexp.MustCompile(`(?i)(?:^|[^A-Za-z])[A-Za-z]:[\\/]`)
var jsonNumberPattern = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?$`)
var generatedProjectIDPattern = regexp.MustCompile(`^project-[a-f0-9]{32}$`)
var producerVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[a-z0-9.-]+)?$`)

type persistedJSONBudget struct {
	nodes       int
	stringBytes int
}

func preflightRunIntent(state projectState, result contract.Result, producerVersion string) error {
	if err := validateProjectState(state); err != nil {
		return errors.New("project state is invalid for run persistence")
	}
	if state.SchemaVersion != contract.SchemaVersion || !runIDPattern.MatchString(result.RunID) || result.SchemaVersion != contract.SchemaVersion {
		return errors.New("run identity or schema version is invalid")
	}
	if !generatedProjectIDPattern.MatchString(state.ProjectID) || state.Engine.Kind != "godot" || state.Engine.RequestedVersion != "4.7.2-stable" || state.Engine.Language != "gdscript" {
		return errors.New("project state is outside the first persisted-run support contract")
	}
	if _, ok := allowedPersistedCommands[result.Command.Name]; !ok {
		return errors.New("command is not eligible for persisted evidence")
	}
	if result.Command.Arguments == nil {
		return errors.New("persisted command arguments are outside bounds")
	}
	switch result.Command.Name {
	case "validate":
		if err := validatePersistedValidateArguments(result.Command.Arguments); err != nil {
			return err
		}
	case "test":
		if err := validatePersistedTestArguments(result.Command.Arguments); err != nil {
			return err
		}
	case "export", "build":
		if err := validatePersistedExportArguments(result.Command.Arguments); err != nil {
			return err
		}
	default:
		return errors.New("persisted command arguments are outside bounds")
	}
	if err := validatePersistedJSON(result.Command.Arguments); err != nil {
		return fmt.Errorf("persisted command arguments are unsafe: %w", err)
	}
	if _, err := parsePersistedTimestamp(result.StartedAt); err != nil {
		return err
	}
	if len(producerVersion) == 0 || len(producerVersion) > 128 || !producerVersionPattern.MatchString(producerVersion) {
		return errors.New("producer version is invalid")
	}
	return nil
}

func preflightRunFinish(transaction *runTransaction, result contract.Result, payloads []runPayload) error {
	if transaction == nil || !producerVersionPattern.MatchString(transaction.producer) || result.RunID != transaction.runID || result.StartedAt != transaction.startedAt || !reflect.DeepEqual(result.Command, transaction.command) {
		return errors.New("run result does not match its immutable intent")
	}
	if err := preflightRunIntent(transaction.state, result, transaction.producer); err != nil {
		return err
	}
	if !validCommandOutcome(result.Outcome) || !validResultExitInvariant(result) || result.DurationMS < 0 || len(result.Summary) == 0 || len(result.Summary) > 2048 || !safePersistedString(result.Summary) {
		return errors.New("command result envelope is invalid")
	}
	started, err := parsePersistedTimestamp(result.StartedAt)
	if err != nil {
		return err
	}
	finished, err := parsePersistedTimestamp(result.FinishedAt)
	if err != nil || finished.Before(started) || result.DurationMS != finished.Sub(started).Milliseconds() {
		return errors.New("command result timing is invalid")
	}
	if result.Evidence == nil || result.Errors == nil || len(result.Evidence) != 0 || len(result.Errors) > 64 {
		return errors.New("result evidence must be empty before commit and errors must be bounded")
	}
	for _, failure := range result.Errors {
		if !errorCodePattern.MatchString(failure.Code) {
			return errors.New("structured error code is invalid")
		}
		if _, ok := allowedErrorCategories[failure.Category]; !ok || len(failure.Message) == 0 || len(failure.Message) > 2048 || !safePersistedString(failure.Message) || len(failure.Remediation) > 4096 || !safePersistedString(failure.Remediation) {
			return errors.New("structured error contains unsafe or invalid text")
		}
		if failure.Details != nil {
			if err := validatePersistedJSON(failure.Details); err != nil {
				return errors.New("structured error details are unsafe")
			}
		}
	}
	if result.Command.Name == "test" {
		return preflightTestRunFinish(result, payloads)
	}
	if result.Command.Name == "export" || result.Command.Name == "build" {
		return preflightExportRunFinish(result, payloads)
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data == nil {
		return errors.New("persisted command data must be a bounded object")
	}
	if err := validatePersistedJSON(data); err != nil {
		return errors.New("persisted command data is unsafe")
	}
	scope, checkCount, err := validateResultData(data)
	if err != nil {
		return err
	}
	if scope != validateCommandScope(result.Command) {
		return errors.New("validate result scope conflicts with its immutable command")
	}
	if len(payloads) != 1 {
		return errors.New("run payload count is outside bounds")
	}
	var total int64
	for _, payload := range payloads {
		if payload.Kind != "validation-report" || !evidenceKindPattern.MatchString(payload.Kind) || !validAssessmentOutcome(payload.Outcome) {
			return errors.New("evidence kind or outcome is invalid")
		}
		if payload.MediaType != "application/json" || len(payload.MediaType) > 128 || len(payload.Metadata) != 0 || !utf8.Valid(payload.Content) || int64(len(payload.Content)) > maxRunPayloadBytes {
			return errors.New("run payload violates the bounded evidence contract")
		}
		if payload.Outcome != result.Outcome {
			return errors.New("evidence outcome conflicts with the command result")
		}
		total += int64(len(payload.Content))
		if total > maxRunPayloadTotalBytes {
			return errors.New("run payload total exceeds its bound")
		}
		duplicateDecoder := json.NewDecoder(bytes.NewReader(payload.Content))
		duplicateDecoder.UseNumber()
		if err := rejectDuplicateObjectKeysWithin(duplicateDecoder, maxPersistedJSONDepth, maxPersistedJSONNodes); err != nil {
			return errors.New("evidence payload has duplicate keys or invalid JSON")
		}
		if token, err := duplicateDecoder.Token(); err != io.EOF || token != nil {
			return errors.New("evidence payload contains multiple JSON values")
		}
		decoder := json.NewDecoder(bytes.NewReader(payload.Content))
		decoder.UseNumber()
		var value any
		if err := decoder.Decode(&value); err != nil {
			return errors.New("evidence payload is not valid JSON")
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			return errors.New("evidence payload contains multiple JSON values")
		}
		if _, ok := value.(map[string]any); !ok {
			return errors.New("evidence JSON payload must be an object")
		}
		if err := validatePersistedJSON(value); err != nil {
			return errors.New("evidence payload contains unsafe data")
		}
		reportCheckCount, err := validateValidationReportPayload(payload.Content, scope, result.Outcome)
		if err != nil {
			return err
		}
		if reportCheckCount != checkCount {
			return errors.New("result and validation report check counts differ")
		}
	}
	return nil
}

type baselineValidationReport struct {
	SchemaVersion string                    `json:"schema_version"`
	Scope         string                    `json:"scope"`
	Outcome       string                    `json:"outcome"`
	Checks        []baselineValidationCheck `json:"checks"`
}

type baselineValidationCheck struct {
	ID      string `json:"id"`
	Outcome string `json:"outcome"`
	Summary string `json:"summary"`
}

func validateValidationReportPayload(content []byte, resultScope, resultOutcome string) (int, error) {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var report baselineValidationReport
	if err := decoder.Decode(&report); err != nil {
		return 0, errors.New("validation report payload violates its strict shape")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return 0, errors.New("validation report payload has trailing data")
	}
	if report.SchemaVersion != contract.SchemaVersion || report.Scope != resultScope || report.Outcome != resultOutcome || len(report.Checks) == 0 || len(report.Checks) > 64 {
		return 0, errors.New("validation report payload violates its command scope")
	}
	seen := make(map[string]struct{}, len(report.Checks))
	matchingOutcome := false
	for _, check := range report.Checks {
		if !evidenceKindPattern.MatchString(check.ID) || !validAssessmentOutcome(check.Outcome) || check.Outcome == "NOT_RUN" || len(check.Summary) == 0 || len(check.Summary) > 512 || !safePersistedString(check.Summary) {
			return 0, errors.New("validation report check is invalid")
		}
		if _, exists := seen[check.ID]; exists {
			return 0, errors.New("validation report check IDs must be unique")
		}
		seen[check.ID] = struct{}{}
		if check.Outcome == resultOutcome {
			matchingOutcome = true
		}
		if resultOutcome == "PASS" && check.Outcome != "PASS" {
			return 0, errors.New("passing validation report contains a non-passing check")
		}
	}
	if !matchingOutcome {
		return 0, errors.New("validation report does not contain its aggregate outcome")
	}
	return len(report.Checks), nil
}

func validatePersistedJSON(value any) error {
	budget := &persistedJSONBudget{}
	return walkPersistedJSON(value, 0, "", budget)
}

func walkPersistedJSON(value any, depth int, key string, budget *persistedJSONBudget) error {
	budget.nodes++
	if budget.nodes > maxPersistedJSONNodes || depth > maxPersistedJSONDepth {
		return errors.New("JSON structure exceeds its node or depth bound")
	}
	if sensitivePersistedKey(key) {
		return errors.New("sensitive field names are not persistable")
	}
	switch typed := value.(type) {
	case nil, bool:
		return nil
	case json.Number:
		if !jsonNumberPattern.MatchString(string(typed)) {
			return errors.New("JSON number is invalid")
		}
		if _, err := strconv.ParseFloat(string(typed), 64); err != nil {
			return errors.New("JSON number is outside the supported range")
		}
		return nil
	case string:
		budget.stringBytes += len(typed)
		if budget.stringBytes > maxPersistedStringBytes || !safePersistedString(typed) {
			return errors.New("string content is unsafe or exceeds its bound")
		}
		return nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return nil
	case float32:
		if math.IsInf(float64(typed), 0) || math.IsNaN(float64(typed)) {
			return errors.New("non-finite number is not persistable")
		}
		return nil
	case float64:
		if math.IsInf(typed, 0) || math.IsNaN(typed) {
			return errors.New("non-finite number is not persistable")
		}
		return nil
	case []any:
		for _, item := range typed {
			if err := walkPersistedJSON(item, depth+1, "", budget); err != nil {
				return err
			}
		}
		return nil
	case []string:
		for _, item := range typed {
			if err := walkPersistedJSON(item, depth+1, "", budget); err != nil {
				return err
			}
		}
		return nil
	case map[string]any:
		if len(typed) > 256 {
			return errors.New("JSON object has too many fields")
		}
		for field, item := range typed {
			if len(field) == 0 || len(field) > 128 || !utf8.ValidString(field) || !safePersistedString(field) {
				return errors.New("JSON field name is invalid")
			}
			budget.stringBytes += len(field)
			if budget.stringBytes > maxPersistedStringBytes {
				return errors.New("JSON string content exceeds its bound")
			}
			if err := walkPersistedJSON(item, depth+1, field, budget); err != nil {
				return err
			}
		}
		return nil
	default:
		return errors.New("value is not in the persistable JSON subset")
	}
}

func sensitivePersistedKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", "_"), " ", "_"))
	for _, marker := range []string{"token", "secret", "password", "credential", "authorization", "api_key", "apikey", "private_key"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func safePersistedString(value string) bool {
	if !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 {
		return false
	}
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "~/") || strings.HasPrefix(trimmed, `\\`) || windowsAbsolutePath.MatchString(trimmed) {
		return false
	}
	lower := strings.ToLower(trimmed)
	for _, pathMarker := range []string{"/users/", "/home/", "/volumes/", `\\users\\`, `\\home\\`} {
		if strings.Contains(lower, pathMarker) {
			return false
		}
	}
	if embeddedWindowsAbsolutePath.MatchString(trimmed) {
		return false
	}
	for _, marker := range []string{"bearer ", "ghp_", "github_pat_", "authorization:", "token=", "password=", "api_key=", "apikey="} {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	return true
}

func validateResultData(data map[string]any) (string, int, error) {
	if len(data) != 2 {
		return "", 0, errors.New("validate result data violates its bounded contract")
	}
	scope, ok := data["scope"].(string)
	if !ok || scope != "baseline" && scope != "headless" {
		return "", 0, errors.New("validate result scope is invalid")
	}
	countValue, ok := persistedInteger(data["check_count"])
	if !ok || countValue < 1 || countValue > 64 {
		return "", 0, errors.New("validate result check count is invalid")
	}
	return scope, int(countValue), nil
}

func validatePersistedValidateArguments(arguments map[string]any) error {
	project, exists := arguments["project"]
	normalized, ok := project.(string)
	if !exists || !ok || normalized != "." {
		return errors.New("persisted project argument must be normalized to dot")
	}
	if len(arguments) == 1 {
		return nil
	}
	if len(arguments) != 5 || arguments["headless"] != true {
		return errors.New("persisted validate arguments violate the headless contract")
	}
	timeout, ok := persistedInteger(arguments["timeout_ms"])
	if !ok || timeout < 1 || timeout > maxTimeoutMS {
		return errors.New("persisted headless timeout is invalid")
	}
	policy, ok := arguments["engine_user_data"].(string)
	if !ok || policy != "not-authorized" && policy != "standard-os-location" {
		return errors.New("persisted engine user-data policy is invalid")
	}
	source, ok := arguments["godot_source"].(string)
	if !ok || source != "explicit" && source != "discovery" {
		return errors.New("persisted Godot source is invalid")
	}
	return nil
}

func persistedInteger(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case json.Number:
		parsed, err := strconv.ParseInt(string(typed), 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func parsePersistedTimestamp(value string) (time.Time, error) {
	if !canonicalTimestamp.MatchString(value) {
		return time.Time{}, errors.New("timestamp is not canonical RFC 3339")
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, errors.New("timestamp is invalid")
	}
	return parsed, nil
}

func validCommandOutcome(outcome string) bool {
	return outcome == "PASS" || outcome == "FAIL" || outcome == "BLOCKED" || outcome == "SKIPPED"
}

func validResultExitInvariant(result contract.Result) bool {
	switch result.Outcome {
	case "PASS", "SKIPPED":
		return result.ExitCode == contract.ExitOK && len(result.Errors) == 0
	case "BLOCKED":
		return result.ExitCode == contract.ExitPrerequisite && len(result.Errors) > 0
	case "FAIL":
		return (result.ExitCode == contract.ExitUsage || result.ExitCode == contract.ExitValidation || result.ExitCode == contract.ExitEngine || result.ExitCode == contract.ExitInterrupted || result.ExitCode == contract.ExitState || result.ExitCode == contract.ExitInternal) && len(result.Errors) > 0
	default:
		return false
	}
}

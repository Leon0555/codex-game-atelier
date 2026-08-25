package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

const maxRunScanEntries = 512
const maxRunScanFiles = maxRunScanEntries * 4
const maxRunScanTotalBytes int64 = 256 * 1024 * 1024
const runScanReadChunk = 64 * 1024

var errRunScanBudgetExceeded = errors.New("run scan work budget exceeded")

type runScanBudget struct {
	maximumBytes int64
	maximumFiles int
	readBytes    int64
	readFiles    int
}

func newRunScanBudget(maximumBytes int64, maximumFiles int) *runScanBudget {
	return &runScanBudget{maximumBytes: maximumBytes, maximumFiles: maximumFiles}
}

func (budget *runScanBudget) beginFile(byteSize int64) error {
	if budget == nil || byteSize < 0 || budget.maximumBytes < 0 || budget.maximumFiles < 1 || budget.readFiles >= budget.maximumFiles || budget.readBytes > budget.maximumBytes || byteSize > budget.maximumBytes-budget.readBytes {
		return errRunScanBudgetExceeded
	}
	budget.readFiles++
	return nil
}

func (budget *runScanBudget) consumeBytes(count int) error {
	if budget == nil || count < 0 || budget.readBytes > budget.maximumBytes || int64(count) > budget.maximumBytes-budget.readBytes {
		return errRunScanBudgetExceeded
	}
	budget.readBytes += int64(count)
	return nil
}

type runScanResult struct {
	Counts     cleanRunCounts
	Candidates []cleanRunEntry
	Protected  []cleanRunEntry
}

type verifiedRun struct {
	Result      contract.Result
	PayloadKind string
	Payload     []byte
	Record      persistedEvidenceRecord
}

func scanRuns(ctx context.Context, stateRoot *os.Root, state projectState) (runScanResult, error) {
	return scanRunsWithBudget(ctx, stateRoot, state, newRunScanBudget(maxRunScanTotalBytes, maxRunScanFiles))
}

func scanRunsWithBudget(ctx context.Context, stateRoot *os.Root, state projectState, budget *runScanBudget) (runScanResult, error) {
	result := runScanResult{Candidates: []cleanRunEntry{}, Protected: []cleanRunEntry{}}
	if err := checkRunScanContext(ctx); err != nil {
		return runScanResult{}, err
	}
	runsRoot, exists, err := openExistingVerifiedDirectory(stateRoot, "runs")
	if err != nil || !exists {
		return result, err
	}
	defer runsRoot.Close()

	directory, err := runsRoot.Open(".")
	if err != nil {
		return runScanResult{}, err
	}
	entries, err := directory.ReadDir(maxRunScanEntries + 1)
	closeErr := directory.Close()
	if err != nil && !errors.Is(err, io.EOF) {
		return runScanResult{}, err
	}
	if closeErr != nil {
		return runScanResult{}, closeErr
	}
	if len(entries) > maxRunScanEntries {
		return runScanResult{}, errors.New("run directory count exceeds scan bound")
	}
	if err := checkRunScanContext(ctx); err != nil {
		return runScanResult{}, err
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].Name() < entries[right].Name() })

	for _, entry := range entries {
		if err := checkRunScanContext(ctx); err != nil {
			return runScanResult{}, err
		}
		name := entry.Name()
		if !runIDPattern.MatchString(name) {
			return runScanResult{}, errors.New("run store contains an invalid directory name")
		}
		runRoot, exists, err := openExistingVerifiedDirectory(runsRoot, name)
		if err != nil || !exists {
			return runScanResult{}, errors.New("run store contains an unsafe directory entry")
		}
		stateName, reason, classifyErr := classifyRun(ctx, budget, runRoot, state, name, nil)
		closeErr := runRoot.Close()
		if classifyErr != nil {
			return runScanResult{}, classifyErr
		}
		if closeErr != nil {
			return runScanResult{}, closeErr
		}
		item := cleanRunEntry{
			RunID:  name,
			State:  stateName,
			Path:   ".gameatelier/runs/" + name,
			Reason: reason,
		}
		switch stateName {
		case "committed":
			result.Counts.Committed++
		case "incomplete":
			result.Counts.Incomplete++
			result.Candidates = append(result.Candidates, item)
		case "orphan":
			result.Counts.Orphan++
			result.Candidates = append(result.Candidates, item)
		default:
			result.Counts.Corrupt++
			result.Protected = append(result.Protected, item)
		}
	}
	return result, nil
}

func classifyRun(ctx context.Context, budget *runScanBudget, runRoot *os.Root, state projectState, runID string, verified *verifiedRun) (string, string, error) {
	intentBytes, intentExists, err := readOptionalRunFile(ctx, budget, runRoot, "intent.json", maxRunIntentBytes)
	if err != nil {
		if isRunScanStop(err) {
			return "", "", err
		}
		return "corrupt", "INTENT_UNSAFE", nil
	}
	resultBytes, resultExists, err := readOptionalRunFile(ctx, budget, runRoot, "result.json", maxRunPayloadBytes)
	if err != nil {
		if isRunScanStop(err) {
			return "", "", err
		}
		return "corrupt", "RESULT_UNSAFE", nil
	}
	if !intentExists && !resultExists {
		return "orphan", "INTENT_AND_RESULT_MISSING", nil
	}
	if !intentExists {
		return "corrupt", "INTENT_MISSING", nil
	}

	var intent runIntentRecord
	if version, versionErr := inspectSchemaVersion(intentBytes); versionErr == nil && version != contract.SchemaVersion {
		return "corrupt", "SCHEMA_UNSUPPORTED", nil
	}
	if err := decodeStrictRunJSON(intentBytes, &intent); err != nil || validateScannedIntent(intent, state, runID) != nil {
		return "corrupt", "INTENT_INVALID", nil
	}
	if !resultExists {
		return "incomplete", "RESULT_MISSING", nil
	}
	if err := checkRunScanContext(ctx); err != nil {
		return "", "", err
	}

	var commandResult contract.Result
	if version, versionErr := inspectSchemaVersion(resultBytes); versionErr == nil && version != contract.SchemaVersion {
		return "corrupt", "SCHEMA_UNSUPPORTED", nil
	}
	if err := decodeStrictRunJSON(resultBytes, &commandResult); err != nil {
		return "corrupt", "RESULT_INVALID", nil
	}
	canonicalResult, err := marshalRunJSON(commandResult)
	if err != nil || !bytes.Equal(resultBytes, canonicalResult) {
		return "corrupt", "RESULT_NON_CANONICAL", nil
	}
	if err := validateIntentResultBinding(intent, commandResult, state, runID); err != nil {
		return "corrupt", "RESULT_INTENT_MISMATCH", nil
	}
	if len(commandResult.Evidence) != 1 {
		return "corrupt", "EVIDENCE_CLOSURE_INVALID", nil
	}

	payloadKind, ok := expectedRunPayloadKind(commandResult.Command.Name)
	if !ok {
		return "corrupt", "PAYLOAD_CLOSURE_INVALID", nil
	}
	evidenceName := "0001-" + payloadKind + ".json"
	payloadRoot, exists, err := openExistingVerifiedDirectory(runRoot, "payloads")
	if err != nil || !exists {
		return "corrupt", "PAYLOAD_CLOSURE_INVALID", nil
	}
	defer payloadRoot.Close()
	evidenceRoot, exists, err := openExistingVerifiedDirectory(runRoot, "evidence")
	if err != nil || !exists {
		return "corrupt", "EVIDENCE_CLOSURE_INVALID", nil
	}
	defer evidenceRoot.Close()

	payloadBytes, err := readScanRegularFile(ctx, budget, payloadRoot, evidenceName, maxRunPayloadBytes)
	if err != nil {
		if isRunScanStop(err) {
			return "", "", err
		}
		return "corrupt", "PAYLOAD_CLOSURE_INVALID", nil
	}
	recordBytes, err := readScanRegularFile(ctx, budget, evidenceRoot, evidenceName, maxRunPayloadBytes)
	if err != nil {
		if isRunScanStop(err) {
			return "", "", err
		}
		return "corrupt", "EVIDENCE_CLOSURE_INVALID", nil
	}
	var record persistedEvidenceRecord
	if version, versionErr := inspectSchemaVersion(recordBytes); versionErr == nil && version != contract.SchemaVersion {
		return "corrupt", "SCHEMA_UNSUPPORTED", nil
	}
	if err := decodeStrictRunJSON(recordBytes, &record); err != nil {
		return "corrupt", "EVIDENCE_RECORD_INVALID", nil
	}

	preflightResult := commandResult
	preflightResult.Evidence = []contract.EvidenceRef{}
	transaction := &runTransaction{
		state:     scannedIntentState(state, intent),
		runID:     runID,
		command:   intent.Command,
		startedAt: intent.StartedAt,
		producer:  intent.Producer.Version,
	}
	payload := runPayload{
		Kind:      record.Kind,
		Outcome:   record.Outcome,
		MediaType: record.MediaType,
		Content:   payloadBytes,
		Metadata:  record.Metadata,
	}
	if version, versionErr := inspectSchemaVersion(payloadBytes); versionErr == nil && version != contract.SchemaVersion {
		return "corrupt", "SCHEMA_UNSUPPORTED", nil
	}
	if err := preflightRunFinish(transaction, preflightResult, []runPayload{payload}); err != nil {
		return "corrupt", "RESULT_PREFLIGHT_FAILED", nil
	}
	if err := verifyScannedRunClosure(payloadBytes, recordBytes, commandResult, record, intent.Producer.Version); err != nil {
		return "corrupt", "EVIDENCE_CLOSURE_INVALID", nil
	}
	if err := checkRunScanContext(ctx); err != nil {
		return "", "", err
	}
	if verified != nil {
		verified.Result = commandResult
		verified.PayloadKind = payloadKind
		verified.Payload = payloadBytes
		verified.Record = record
	}
	return "committed", "RESULT_CLOSURE_VERIFIED", nil
}

func expectedRunPayloadKind(commandName string) (string, bool) {
	switch commandName {
	case "validate":
		return "validation-report", true
	case "test":
		return "test-report", true
	default:
		return "", false
	}
}

func validateScannedIntent(intent runIntentRecord, state projectState, runID string) error {
	base := ".gameatelier/runs/" + runID
	if intent.SchemaVersion != contract.SchemaVersion || intent.RunID != runID || intent.ProjectID != state.ProjectID || intent.ProjectRevision < 0 || intent.ProjectRevision > int64(state.Revision) {
		return errors.New("intent project snapshot mismatch")
	}
	if intent.Producer.Component != "gameatelier-cli" || !producerVersionPattern.MatchString(intent.Producer.Version) {
		return errors.New("intent producer is invalid")
	}
	if intent.ExpectedResultRef != base+"/result.json" || !equalStringSlices(intent.DeclaredWrites, []string{base}) || !equalStringSlices(intent.DeclaredExternal, declaredExternalWrites(intent.Command)) {
		return errors.New("intent write declaration is invalid")
	}
	probe := contract.Result{
		SchemaVersion: contract.SchemaVersion,
		RunID:         runID,
		Command:       intent.Command,
		StartedAt:     intent.StartedAt,
	}
	snapshot := scannedIntentState(state, intent)
	return preflightRunIntent(snapshot, probe, intent.Producer.Version)
}

func scannedIntentState(current projectState, intent runIntentRecord) projectState {
	snapshot := current
	snapshot.Revision = schemaInteger(intent.ProjectRevision)
	snapshot.Mode = intent.PolicyMode
	snapshot.Engine = intent.Engine
	return snapshot
}

func equalStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func validateIntentResultBinding(intent runIntentRecord, result contract.Result, state projectState, runID string) error {
	if result.SchemaVersion != contract.SchemaVersion || result.RunID != runID || result.StartedAt != intent.StartedAt || !reflect.DeepEqual(result.Command, intent.Command) {
		return errors.New("result does not match intent")
	}
	return validateScannedIntent(intent, state, runID)
}

func readOptionalRunFile(ctx context.Context, budget *runScanBudget, root *os.Root, name string, maximum int64) ([]byte, bool, error) {
	if err := checkRunScanContext(ctx); err != nil {
		return nil, false, err
	}
	_, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	content, err := readScanRegularFile(ctx, budget, root, name, maximum)
	if err != nil {
		return nil, true, err
	}
	return content, true, nil
}

func readScanRegularFile(ctx context.Context, budget *runScanBudget, root *os.Root, name string, maximum int64) ([]byte, error) {
	if err := checkRunScanContext(ctx); err != nil {
		return nil, err
	}
	info, err := root.Lstat(name)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, errors.New("run closure member is not a regular file")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil || !os.SameFile(info, openedInfo) {
		return nil, errors.New("run closure member changed while opening")
	}
	fileSize := openedInfo.Size()
	if fileSize < 0 || fileSize > maximum {
		return nil, errors.New("run closure member exceeds its bound")
	}
	if err := budget.beginFile(fileSize); err != nil {
		return nil, err
	}

	var content bytes.Buffer
	chunk := make([]byte, runScanReadChunk)
	for int64(content.Len()) < fileSize {
		if err := checkRunScanContext(ctx); err != nil {
			return nil, err
		}
		remaining := fileSize - int64(content.Len())
		readBuffer := chunk
		if remaining < int64(len(readBuffer)) {
			readBuffer = readBuffer[:int(remaining)]
		}
		count, readErr := file.Read(readBuffer)
		if count > 0 {
			if err := budget.consumeBytes(count); err != nil {
				return nil, err
			}
			if _, err := content.Write(chunk[:count]); err != nil {
				return nil, err
			}
		}
		if readErr != nil && !(errors.Is(readErr, io.EOF) && int64(content.Len()) == fileSize) {
			return nil, errors.New("run closure member changed while reading")
		}
		if count == 0 {
			return nil, errors.New("run closure member made no read progress")
		}
	}
	finalInfo, err := file.Stat()
	if err != nil || !os.SameFile(openedInfo, finalInfo) || finalInfo.Size() != fileSize {
		return nil, errors.New("run closure member changed while reading")
	}
	if err := checkRunScanContext(ctx); err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

func verifyScannedRunClosure(payloadBytes, recordBytes []byte, result contract.Result, record persistedEvidenceRecord, producerVersion string) error {
	if len(result.Evidence) != 1 {
		return errors.New("result evidence closure length mismatch")
	}
	name := "0001-" + record.Kind + ".json"
	sum := sha256.Sum256(payloadBytes)
	expectedRecordPath := ".gameatelier/runs/" + result.RunID + "/evidence/" + name
	expectedPayloadPath := ".gameatelier/runs/" + result.RunID + "/payloads/" + name
	if record.SchemaVersion != contract.SchemaVersion || record.RunID != result.RunID || record.Path != expectedPayloadPath || record.Producer.Component != "gameatelier-cli" || record.Producer.Version != producerVersion || record.CreatedAt != result.FinishedAt || result.Evidence[0].ID != record.ID || result.Evidence[0].Path != expectedRecordPath || record.SHA256 != hex.EncodeToString(sum[:]) || record.ByteSize != int64(len(payloadBytes)) {
		return errors.New("evidence payload closure mismatch")
	}
	expectedRecordBytes, err := marshalRunJSON(record)
	if err != nil || !bytes.Equal(recordBytes, expectedRecordBytes) {
		return errors.New("persisted evidence record mismatch")
	}
	return nil
}

func checkRunScanContext(ctx context.Context) error {
	if ctx == nil {
		return context.Canceled
	}
	return ctx.Err()
}

func isRunScanStop(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, errRunScanBudgetExceeded)
}

func openExistingVerifiedDirectory(parent *os.Root, name string) (*os.Root, bool, error) {
	if parent == nil || name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\\:`) {
		return nil, false, errors.New("unsafe state directory name")
	}
	info, err := parent.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, true, errors.New("state directory is not a real directory")
	}
	child, err := parent.OpenRoot(name)
	if err != nil {
		return nil, true, err
	}
	openedInfo, err := child.Stat(".")
	if err != nil || !openedInfo.IsDir() || !os.SameFile(info, openedInfo) {
		child.Close()
		return nil, true, errors.New("state directory changed while opening")
	}
	return child, true, nil
}

func decodeStrictRunJSON(content []byte, target any) error {
	if len(content) == 0 || !utf8.Valid(content) {
		return errors.New("run JSON is empty or not UTF-8")
	}
	duplicateDecoder := json.NewDecoder(bytes.NewReader(content))
	duplicateDecoder.UseNumber()
	if err := rejectDuplicateObjectKeysWithin(duplicateDecoder, maxPersistedJSONDepth, maxPersistedJSONNodes); err != nil {
		return err
	}
	if token, err := duplicateDecoder.Token(); err != io.EOF || token != nil {
		if err == nil {
			return errors.New("run JSON contains multiple values")
		}
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("run JSON contains trailing data")
	}
	return nil
}

package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

const maxRunPayloadBytes int64 = 4 * 1024 * 1024
const maxRunIntentBytes int64 = 256 * 1024

var runIDPattern = regexp.MustCompile(`^atelier-[0-9]{8}t[0-9]{6}\.[0-9]{9}z-[a-f0-9]{12}$`)
var evidenceKindPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

type runPayload struct {
	Kind      string
	Outcome   string
	MediaType string
	Content   []byte
	Metadata  map[string]any
}

type runIntentRecord struct {
	SchemaVersion     string             `json:"schema_version"`
	RunID             string             `json:"run_id"`
	ProjectID         string             `json:"project_id"`
	ProjectRevision   int64              `json:"project_revision"`
	PolicyMode        string             `json:"policy_mode"`
	Engine            projectStateEngine `json:"engine"`
	Command           contract.Command   `json:"command"`
	StartedAt         string             `json:"started_at"`
	Producer          evidenceProducer   `json:"producer"`
	ExpectedResultRef string             `json:"expected_result_ref"`
	DeclaredWrites    []string           `json:"declared_write_paths"`
	DeclaredExternal  []string           `json:"declared_external_writes"`
}

type evidenceProducer struct {
	Component string `json:"component"`
	Version   string `json:"version"`
}

type persistedEvidenceRecord struct {
	SchemaVersion string           `json:"schema_version"`
	ID            string           `json:"id"`
	RunID         string           `json:"run_id"`
	Kind          string           `json:"kind"`
	Outcome       string           `json:"outcome"`
	CreatedAt     string           `json:"created_at"`
	Producer      evidenceProducer `json:"producer"`
	Path          string           `json:"path"`
	MediaType     string           `json:"media_type"`
	SHA256        string           `json:"sha256"`
	ByteSize      int64            `json:"byte_size"`
	Metadata      map[string]any   `json:"metadata,omitempty"`
}

type runCommit struct {
	ResultBytes []byte
	Committed   bool
	Err         error
}

type runTransaction struct {
	state          projectState
	runID          string
	command        contract.Command
	startedAt      string
	producer       string
	identity       runPersistenceIdentity
	identityReader func(*os.Root) (runPersistenceIdentity, bool)
	runBase        string
	runRoot        *os.Root
	fault          runFault
	finished       bool
}

type runFault func(stage string) error

type runPersistenceIdentity struct {
	fsid [2]int64
}

type runBeginPhase uint8

const (
	runBeginBeforeRoot runBeginPhase = iota
	runBeginOrphan
	runBeginIncomplete
)

type runBeginError struct {
	phase runBeginPhase
	err   error
}

func (failure *runBeginError) Error() string { return failure.err.Error() }
func (failure *runBeginError) Unwrap() error { return failure.err }

func failRunBegin(phase runBeginPhase, err error) (*runTransaction, error) {
	return nil, &runBeginError{phase: phase, err: err}
}

func beginRun(stateRoot *os.Root, state projectState, result contract.Result, fault runFault) (*runTransaction, error) {
	return beginRunWithIdentity(stateRoot, state, result, fault, readRunPersistenceIdentity)
}

func beginRunWithIdentity(stateRoot *os.Root, state projectState, result contract.Result, fault runFault, identityReader func(*os.Root) (runPersistenceIdentity, bool)) (*runTransaction, error) {
	if stateRoot == nil || identityReader == nil {
		return failRunBegin(runBeginBeforeRoot, errors.New("run persistence is not enabled on this host"))
	}
	stateIdentity, ready := identityReader(stateRoot)
	if !ready {
		return failRunBegin(runBeginBeforeRoot, errors.New("run persistence is not enabled on this host"))
	}
	loadedState, exists, _, err := loadExistingState(stateRoot)
	if err != nil || !exists || !reflect.DeepEqual(loadedState, state) {
		return failRunBegin(runBeginBeforeRoot, errors.New("run state snapshot does not match its pinned state root"))
	}
	if err := preflightRunIntent(state, result, Version); err != nil {
		return failRunBegin(runBeginBeforeRoot, err)
	}
	if err := triggerRunFault(fault, "before-run-directory"); err != nil {
		return failRunBegin(runBeginBeforeRoot, err)
	}
	runsRoot, err := openOrCreateVerifiedDirectory(stateRoot, "runs", false)
	if err != nil {
		return failRunBegin(runBeginBeforeRoot, err)
	}
	defer runsRoot.Close()
	runsIdentity, ready := identityReader(runsRoot)
	if !ready || runsIdentity != stateIdentity {
		return failRunBegin(runBeginBeforeRoot, errors.New("runs directory is outside the verified persistence filesystem"))
	}
	runRoot, err := openOrCreateVerifiedDirectory(runsRoot, result.RunID, true)
	if err != nil {
		phase := runBeginBeforeRoot
		var directoryFailure *verifiedDirectoryError
		if errors.As(err, &directoryFailure) && directoryFailure.created {
			phase = runBeginOrphan
		}
		return failRunBegin(phase, err)
	}
	runIdentity, ready := identityReader(runRoot)
	if !ready || runIdentity != stateIdentity {
		runRoot.Close()
		return failRunBegin(runBeginOrphan, errors.New("run directory is outside the verified persistence filesystem"))
	}

	runBase := ".gameatelier/runs/" + result.RunID
	frozenArguments := make(map[string]any, len(result.Command.Arguments))
	for key, value := range result.Command.Arguments {
		frozenArguments[key] = value
	}
	frozenCommand := contract.Command{Name: result.Command.Name, Arguments: frozenArguments}
	intent := runIntentRecord{
		SchemaVersion:     contract.SchemaVersion,
		RunID:             result.RunID,
		ProjectID:         state.ProjectID,
		ProjectRevision:   int64(state.Revision),
		PolicyMode:        state.Mode,
		Engine:            state.Engine,
		Command:           frozenCommand,
		StartedAt:         result.StartedAt,
		Producer:          evidenceProducer{Component: "gameatelier-cli", Version: Version},
		ExpectedResultRef: runBase + "/result.json",
		DeclaredWrites:    []string{runBase},
		DeclaredExternal:  declaredExternalWrites(frozenCommand),
	}
	intentBytes, err := marshalRunJSON(intent)
	if err != nil || int64(len(intentBytes)) > maxRunIntentBytes {
		runRoot.Close()
		return failRunBegin(runBeginOrphan, errors.New("run intent could not be encoded within bounds"))
	}
	if err := triggerRunFault(fault, "before-intent"); err != nil {
		runRoot.Close()
		return failRunBegin(runBeginOrphan, err)
	}
	if published := publishRunFile(runRoot, ".intent.json.tmp", "intent.json", intentBytes, "intent", fault); published.err != nil {
		runRoot.Close()
		return failRunBegin(runBeginOrphan, published.err)
	} else if published.durabilityErr != nil {
		runRoot.Close()
		return failRunBegin(runBeginIncomplete, published.durabilityErr)
	}
	if err := triggerRunFault(fault, "after-intent"); err != nil {
		runRoot.Close()
		return failRunBegin(runBeginIncomplete, err)
	}
	return &runTransaction{
		state:          state,
		runID:          result.RunID,
		command:        frozenCommand,
		startedAt:      result.StartedAt,
		producer:       Version,
		identity:       stateIdentity,
		identityReader: identityReader,
		runBase:        runBase,
		runRoot:        runRoot,
		fault:          fault,
	}, nil
}

func declaredExternalWrites(command contract.Command) []string {
	if (command.Name == "validate" || command.Name == "test") && command.Arguments["engine_user_data"] == "standard-os-location" {
		return []string{"godot:user-data:standard-os-location"}
	}
	return []string{}
}

func (transaction *runTransaction) finish(result contract.Result, payloads []runPayload) runCommit {
	if transaction == nil || transaction.runRoot == nil || transaction.finished {
		return runCommit{Err: errors.New("run transaction is not open")}
	}
	transaction.finished = true
	if err := preflightRunFinish(transaction, result, payloads); err != nil {
		return runCommit{Err: err}
	}
	runRoot := transaction.runRoot
	fault := transaction.fault
	runBase := transaction.runBase
	refs := make([]contract.EvidenceRef, len(payloads))
	for index, payload := range payloads {
		sequence := fmt.Sprintf("%04d", index+1)
		name := sequence + "-" + payload.Kind + ".json"
		refs[index] = contract.EvidenceRef{
			ID:   result.RunID + "-e" + sequence,
			Path: runBase + "/evidence/" + name,
		}
	}
	result.Evidence = refs
	resultBytes, err := marshalRunJSON(result)
	if err != nil || int64(len(resultBytes)) > maxRunPayloadBytes {
		return runCommit{Err: errors.New("run result could not be encoded within bounds")}
	}
	payloadRoot, err := openOrCreateVerifiedDirectory(runRoot, "payloads", true)
	if err != nil {
		return runCommit{Err: err}
	}
	defer payloadRoot.Close()
	if identity, ready := transaction.identityReader(payloadRoot); !ready || identity != transaction.identity {
		return runCommit{Err: errors.New("payload directory is outside the verified persistence filesystem")}
	}
	evidenceRoot, err := openOrCreateVerifiedDirectory(runRoot, "evidence", true)
	if err != nil {
		return runCommit{Err: err}
	}
	defer evidenceRoot.Close()
	if identity, ready := transaction.identityReader(evidenceRoot); !ready || identity != transaction.identity {
		return runCommit{Err: errors.New("evidence directory is outside the verified persistence filesystem")}
	}

	records := make([]persistedEvidenceRecord, 0, len(payloads))
	for index, payload := range payloads {
		sequence := fmt.Sprintf("%04d", index+1)
		payloadName := sequence + "-" + payload.Kind + ".json"
		recordName := sequence + "-" + payload.Kind + ".json"
		if err := triggerRunFault(fault, "before-payload-"+sequence); err != nil {
			return runCommit{Err: err}
		}
		published := publishRunFile(payloadRoot, "."+payloadName+".tmp", payloadName, payload.Content, "payload-"+sequence, fault)
		if published.err != nil || published.durabilityErr != nil {
			return runCommit{Err: firstRunStoreError(published)}
		}
		payloadSum := sha256.Sum256(payload.Content)
		record := persistedEvidenceRecord{
			SchemaVersion: contract.SchemaVersion,
			ID:            result.RunID + "-e" + sequence,
			RunID:         result.RunID,
			Kind:          payload.Kind,
			Outcome:       payload.Outcome,
			CreatedAt:     result.FinishedAt,
			Producer:      evidenceProducer{Component: "gameatelier-cli", Version: transaction.producer},
			Path:          runBase + "/payloads/" + payloadName,
			MediaType:     payload.MediaType,
			SHA256:        hex.EncodeToString(payloadSum[:]),
			ByteSize:      int64(len(payload.Content)),
			Metadata:      payload.Metadata,
		}
		recordBytes, err := marshalRunJSON(record)
		if err != nil || int64(len(recordBytes)) > maxRunPayloadBytes {
			return runCommit{Err: errors.New("evidence record could not be encoded within bounds")}
		}
		if err := triggerRunFault(fault, "before-evidence-"+sequence); err != nil {
			return runCommit{Err: err}
		}
		published = publishRunFile(evidenceRoot, "."+recordName+".tmp", recordName, recordBytes, "evidence-"+sequence, fault)
		if published.err != nil || published.durabilityErr != nil {
			return runCommit{Err: firstRunStoreError(published)}
		}
		records = append(records, record)
	}

	if err := verifyRunClosure(payloadRoot, evidenceRoot, result, records, transaction.producer); err != nil {
		return runCommit{Err: err}
	}
	if err := triggerRunFault(fault, "before-result"); err != nil {
		return runCommit{Err: err}
	}
	published := publishRunFile(runRoot, ".result.json.tmp", "result.json", resultBytes, "result", fault)
	if published.err != nil {
		return runCommit{Err: published.err}
	}
	if published.durabilityErr != nil {
		return runCommit{ResultBytes: resultBytes, Committed: true, Err: published.durabilityErr}
	}
	if err := triggerRunFault(fault, "after-result"); err != nil {
		return runCommit{ResultBytes: resultBytes, Committed: true, Err: err}
	}
	return runCommit{ResultBytes: resultBytes, Committed: true}
}

func (transaction *runTransaction) close() error {
	if transaction == nil || transaction.runRoot == nil {
		return nil
	}
	err := transaction.runRoot.Close()
	transaction.runRoot = nil
	return err
}

func openOrCreateVerifiedDirectory(parent *os.Root, name string, exclusive bool) (*os.Root, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\\:`) {
		return nil, errors.New("unsafe state directory name")
	}
	err := parent.Mkdir(name, 0o700)
	created := err == nil
	if exclusive && errors.Is(err, os.ErrExist) {
		return nil, os.ErrExist
	}
	if err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	info, err := parent.Lstat(name)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, &verifiedDirectoryError{created: created, err: errors.New("state directory is not a real directory")}
	}
	child, err := parent.OpenRoot(name)
	if err != nil {
		return nil, &verifiedDirectoryError{created: created, err: err}
	}
	openedInfo, err := child.Stat(".")
	if err != nil || !openedInfo.IsDir() || !os.SameFile(info, openedInfo) {
		child.Close()
		return nil, &verifiedDirectoryError{created: created, err: errors.New("state directory changed while opening")}
	}
	if err := syncStateDirectory(parent); err != nil {
		child.Close()
		return nil, &verifiedDirectoryError{created: created, err: err}
	}
	return child, nil
}

type verifiedDirectoryError struct {
	created bool
	err     error
}

func (failure *verifiedDirectoryError) Error() string { return failure.err.Error() }
func (failure *verifiedDirectoryError) Unwrap() error { return failure.err }

func verifyRunClosure(payloadRoot, evidenceRoot *os.Root, result contract.Result, records []persistedEvidenceRecord, producerVersion string) error {
	if len(result.Evidence) != len(records) {
		return errors.New("result evidence closure length mismatch")
	}
	for index, record := range records {
		sequence := fmt.Sprintf("%04d", index+1)
		name := sequence + "-" + record.Kind + ".json"
		payload, err := readBoundedRegularFile(payloadRoot, name, maxRunPayloadBytes)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(payload)
		expectedRecordPath := ".gameatelier/runs/" + result.RunID + "/evidence/" + name
		expectedPayloadPath := ".gameatelier/runs/" + result.RunID + "/payloads/" + name
		if record.SchemaVersion != contract.SchemaVersion || record.RunID != result.RunID || record.Path != expectedPayloadPath || record.Producer.Component != "gameatelier-cli" || record.Producer.Version != producerVersion || record.CreatedAt != result.FinishedAt || result.Evidence[index].ID != record.ID || result.Evidence[index].Path != expectedRecordPath || record.SHA256 != hex.EncodeToString(sum[:]) || record.ByteSize != int64(len(payload)) {
			return errors.New("evidence payload closure mismatch")
		}
		recordBytes, err := readBoundedRegularFile(evidenceRoot, name, maxRunPayloadBytes)
		if err != nil {
			return err
		}
		expectedRecordBytes, err := marshalRunJSON(record)
		if err != nil || !bytes.Equal(recordBytes, expectedRecordBytes) {
			return errors.New("persisted evidence record mismatch")
		}
	}
	return nil
}

func readBoundedRegularFile(root *os.Root, name string, maximum int64) ([]byte, error) {
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
	content, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil || int64(len(content)) > maximum {
		return nil, errors.New("run closure member exceeds its bound")
	}
	return content, nil
}

func marshalRunJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func validAssessmentOutcome(outcome string) bool {
	return outcome == "PASS" || outcome == "FAIL" || outcome == "BLOCKED" || outcome == "SKIPPED" || outcome == "NOT_RUN"
}

func triggerRunFault(fault runFault, stage string) error {
	if fault == nil {
		return nil
	}
	return fault(stage)
}

func publishRunFile(root *os.Root, tempName, finalName string, content []byte, prefix string, fault runFault) statePublishResult {
	return publishImmutableFileWithFault(root, tempName, finalName, content, func(stage string) error {
		return triggerRunFault(fault, prefix+":"+stage)
	})
}

func firstRunStoreError(result statePublishResult) error {
	if result.err != nil {
		return result.err
	}
	return result.durabilityErr
}

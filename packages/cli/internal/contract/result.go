package contract

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

const SchemaVersion = "1.0.0"

const (
	ExitOK           = 0
	ExitUsage        = 2
	ExitValidation   = 3
	ExitPrerequisite = 4
	ExitEngine       = 5
	ExitInterrupted  = 6
	ExitState        = 7
	ExitInternal     = 8
)

type Command struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type Error struct {
	Code        string         `json:"code"`
	Category    string         `json:"category"`
	Message     string         `json:"message"`
	Retryable   bool           `json:"retryable"`
	Remediation string         `json:"remediation,omitempty"`
	Details     map[string]any `json:"details,omitempty"`
}

type EvidenceRef struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

type Result struct {
	SchemaVersion string        `json:"schema_version"`
	RunID         string        `json:"run_id"`
	Command       Command       `json:"command"`
	Outcome       string        `json:"outcome"`
	StartedAt     string        `json:"started_at"`
	FinishedAt    string        `json:"finished_at"`
	DurationMS    int64         `json:"duration_ms"`
	ExitCode      int           `json:"exit_code"`
	Summary       string        `json:"summary"`
	Errors        []Error       `json:"errors"`
	Evidence      []EvidenceRef `json:"evidence"`
	Data          any           `json:"data"`
}

func NewResult(started time.Time, command Command) Result {
	return Result{
		SchemaVersion: SchemaVersion,
		RunID:         newRunID(started),
		Command:       command,
		StartedAt:     started.UTC().Format(time.RFC3339Nano),
		Errors:        []Error{},
		Evidence:      []EvidenceRef{},
	}
}

func (r *Result) Finish(started, finished time.Time, outcome string, exitCode int, summary string, data any, failures ...Error) {
	r.Outcome = outcome
	r.FinishedAt = finished.UTC().Format(time.RFC3339Nano)
	r.DurationMS = finished.Sub(started).Milliseconds()
	if r.DurationMS < 0 {
		r.DurationMS = 0
	}
	r.ExitCode = exitCode
	r.Summary = summary
	r.Data = data
	r.Errors = append(r.Errors, failures...)
}

func newRunID(started time.Time) string {
	random := make([]byte, 6)
	if _, err := rand.Read(random); err == nil {
		return fmt.Sprintf("atelier-%s-%s", started.UTC().Format("20060102t150405.000000000z"), hex.EncodeToString(random))
	}
	return fmt.Sprintf("atelier-%s", started.UTC().Format("20060102t150405.000000000z"))
}

package contract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

func DecodeTaskEvent(raw []byte) (TaskEvent, error) {
	var event TaskEvent
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&event); err != nil {
		return TaskEvent{}, err
	}
	if err := ensureEOF(decoder); err != nil {
		return TaskEvent{}, err
	}
	if err := event.Validate(); err != nil {
		return TaskEvent{}, err
	}
	return event, nil
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("multiple JSON values are not allowed")
	}
	return err
}

func (e TaskEvent) Validate() error {
	if e.SchemaVersion != EventSchemaVersion {
		return fmt.Errorf("unsupported schema_version %q", e.SchemaVersion)
	}
	if !validID(e.TaskID) || !validID(e.EventID) {
		return errors.New("task_id and event_id must be non-empty bounded identifiers")
	}
	if !validEventType(e.Type) {
		return fmt.Errorf("unsupported event type %q", e.Type)
	}
	if e.Timestamp.IsZero() || e.Timestamp.Location() != nil && e.Timestamp.UTC() != e.Timestamp {
		return errors.New("timestamp must be a non-zero UTC value")
	}
	if len(e.WorkingDir) > 4096 || len(e.CommandFingerprint) > 256 || len(e.ReceiptID) > 256 {
		return errors.New("event field exceeds size limit")
	}
	if e.CommandFingerprint != "" && !strings.HasPrefix(e.CommandFingerprint, "sha256:") {
		return errors.New("command_fingerprint must use sha256")
	}
	if len(e.DeclaredOutputs) > 128 {
		return errors.New("declared_outputs exceeds size limit")
	}
	for _, output := range e.DeclaredOutputs {
		if output == "" || len(output) > 4096 {
			return errors.New("declared output is invalid")
		}
	}
	return nil
}

func (c Candidate) Validate() error {
	if !validID(c.ID) {
		return errors.New("candidate id is invalid")
	}
	if !validCandidateKind(c.Kind) || !validEvidenceLevel(c.EvidenceLevel) || !validCurrentStatus(c.CurrentStatus) {
		return errors.New("candidate enum is invalid")
	}
	if c.Recommendation != "" && c.Recommendation != "review" {
		return errors.New("recommendation must be empty or review")
	}
	if c.SizeBytes < 0 {
		return errors.New("size_bytes cannot be negative")
	}
	return nil
}

func (r Report) Validate() error {
	if r.SchemaVersion != ReportSchemaVersion || !validID(r.ReportID) || !validID(r.TaskID) {
		return errors.New("report identity or schema is invalid")
	}
	if !validReportStatus(r.Status) || r.CreatedAt.IsZero() {
		return errors.New("report status or timestamp is invalid")
	}
	for i, candidate := range r.Candidates {
		if err := candidate.Validate(); err != nil {
			return fmt.Errorf("candidate %d: %w", i, err)
		}
	}
	return nil
}

func validID(value string) bool { return value != "" && len(value) <= 256 }

func validEventType(value EventType) bool {
	switch value {
	case EventCommandStarted, EventCommandCompleted, EventProcessStarted, EventProcessExited,
		EventArtifactDeclared, EventTestPhaseStarted, EventTestPhaseCompleted, EventCleanupAttempted:
		return true
	default:
		return false
	}
}

func validCandidateKind(value CandidateKind) bool {
	switch value {
	case CandidateFile, CandidateDirectory, CandidateProcess, CandidatePort:
		return true
	default:
		return false
	}
}

func validEvidenceLevel(value EvidenceLevel) bool {
	switch value {
	case EvidenceBaselineObserved, EvidenceEventBound, EvidenceReceiptBound, EvidenceInferred, EvidenceUnattributed:
		return true
	default:
		return false
	}
}

func validCurrentStatus(value CurrentStatus) bool {
	switch value {
	case StatusPresent, StatusActiveReference, StatusNoLongerPresent, StatusChangedSinceReport, StatusUnknown:
		return true
	default:
		return false
	}
}

func validReportStatus(value ReportStatus) bool {
	switch value {
	case ReportNoCandidates, ReportReviewRequired, ReportPartialEvidence, ReportInterruptedTask, ReportObservationFailed:
		return true
	default:
		return false
	}
}

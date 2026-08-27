package contract

import (
	"strings"
	"testing"
	"time"
)

func TestDecodeTaskEventRejectsRawCommand(t *testing.T) {
	raw := []byte(`{"schema_version":"agent-task-event/1.0","task_id":"task-1","event_id":"event-1","type":"command_started","timestamp":"2026-08-27T00:00:00Z","command":"printenv"}`)
	if _, err := DecodeTaskEvent(raw); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown command field error, got %v", err)
	}
}

func TestDecodeTaskEventRejectsUnknownField(t *testing.T) {
	raw := []byte(`{"schema_version":"agent-task-event/1.0","task_id":"task-1","event_id":"event-1","type":"artifact_declared","timestamp":"2026-08-27T00:00:00Z","mystery":true}`)
	if _, err := DecodeTaskEvent(raw); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestTaskEventValidationAcceptsSafeProjection(t *testing.T) {
	event := TaskEvent{
		SchemaVersion: EventSchemaVersion,
		TaskID:        "task-1",
		EventID:       "event-1",
		Type:          EventArtifactDeclared,
		Timestamp:     time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC),
		WorkingDir:    "/tmp/task-root",
		DeclaredOutputs: []string{
			"/tmp/task-root/output",
		},
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("safe event rejected: %v", err)
	}
}

func TestCandidateEvidenceAndCurrentStateAreIndependent(t *testing.T) {
	candidate := Candidate{
		ID:            "candidate-1",
		Kind:          CandidateDirectory,
		EvidenceLevel: EvidenceBaselineObserved,
		CurrentStatus: StatusNoLongerPresent,
	}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("valid candidate rejected: %v", err)
	}
}

func TestReportRejectsSafeDeleteRecommendation(t *testing.T) {
	report := Report{
		SchemaVersion: ReportSchemaVersion,
		ReportID:      "report-1",
		TaskID:        "task-1",
		Status:        ReportReviewRequired,
		CreatedAt:     time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC),
		Candidates: []Candidate{{
			ID:             "candidate-1",
			Kind:           CandidateFile,
			EvidenceLevel:  EvidenceInferred,
			CurrentStatus:  StatusPresent,
			Recommendation: "safe_to_delete",
		}},
	}
	if err := report.Validate(); err == nil || !strings.Contains(err.Error(), "recommendation") {
		t.Fatalf("expected recommendation rejection, got %v", err)
	}
}

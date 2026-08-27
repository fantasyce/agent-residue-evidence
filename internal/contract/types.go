package contract

import "time"

const (
	EventSchemaVersion  = "agent-task-event/1.0"
	ReportSchemaVersion = "agent-residue-report/1.0"
	LifecycleVersion    = "agent-residue-lifecycle/1.0"
)

type TaskState string

const (
	TaskActive      TaskState = "ACTIVE"
	TaskCompleted   TaskState = "COMPLETED"
	TaskInterrupted TaskState = "INTERRUPTED"
	TaskExpired     TaskState = "EXPIRED"
)

type EventType string

const (
	EventCommandStarted     EventType = "command_started"
	EventCommandCompleted   EventType = "command_completed"
	EventProcessStarted     EventType = "process_started"
	EventProcessExited      EventType = "process_exited"
	EventArtifactDeclared   EventType = "artifact_declared"
	EventTestPhaseStarted   EventType = "test_phase_started"
	EventTestPhaseCompleted EventType = "test_phase_completed"
	EventCleanupAttempted   EventType = "cleanup_attempted"
)

type TaskScope struct {
	TaskID    string   `json:"task_id"`
	Workspace string   `json:"workspace"`
	TempRoots []string `json:"temp_roots,omitempty"`
}

type ProcessIdentity struct {
	PID       int       `json:"pid"`
	CreatedAt time.Time `json:"created_at"`
}

type TaskEvent struct {
	SchemaVersion      string           `json:"schema_version"`
	TaskID             string           `json:"task_id"`
	EventID            string           `json:"event_id"`
	Type               EventType        `json:"type"`
	Timestamp          time.Time        `json:"timestamp"`
	WorkingDir         string           `json:"working_directory,omitempty"`
	CommandFingerprint string           `json:"command_fingerprint,omitempty"`
	ExitCode           *int             `json:"exit_code,omitempty"`
	Process            *ProcessIdentity `json:"process,omitempty"`
	DeclaredOutputs    []string         `json:"declared_outputs,omitempty"`
	ReceiptID          string           `json:"receipt_id,omitempty"`
}

type CandidateKind string

const (
	CandidateFile      CandidateKind = "file"
	CandidateDirectory CandidateKind = "directory"
	CandidateProcess   CandidateKind = "process"
	CandidatePort      CandidateKind = "port"
)

type EvidenceLevel string

const (
	EvidenceBaselineObserved EvidenceLevel = "BASELINE_OBSERVED"
	EvidenceEventBound       EvidenceLevel = "EVENT_BOUND"
	EvidenceReceiptBound     EvidenceLevel = "RECEIPT_BOUND"
	EvidenceInferred         EvidenceLevel = "INFERRED"
	EvidenceUnattributed     EvidenceLevel = "UNATTRIBUTED"
)

type CurrentStatus string

const (
	StatusPresent            CurrentStatus = "PRESENT"
	StatusActiveReference    CurrentStatus = "ACTIVE_REFERENCE"
	StatusNoLongerPresent    CurrentStatus = "NO_LONGER_PRESENT"
	StatusChangedSinceReport CurrentStatus = "CHANGED_SINCE_REPORT"
	StatusUnknown            CurrentStatus = "UNKNOWN"
)

type Candidate struct {
	ID             string        `json:"id"`
	Kind           CandidateKind `json:"kind"`
	Path           string        `json:"path,omitempty"`
	ObjectIdentity string        `json:"object_identity,omitempty"`
	SizeBytes      int64         `json:"size_bytes,omitempty"`
	EvidenceLevel  EvidenceLevel `json:"evidence_level"`
	CurrentStatus  CurrentStatus `json:"current_status"`
	Reason         string        `json:"reason,omitempty"`
	Recommendation string        `json:"recommendation,omitempty"`
	Limitations    []string      `json:"limitations,omitempty"`
	Conflicts      []string      `json:"conflicts,omitempty"`
}

type ReportStatus string

const (
	ReportNoCandidates      ReportStatus = "NO_CANDIDATES_OBSERVED"
	ReportReviewRequired    ReportStatus = "REVIEW_REQUIRED"
	ReportPartialEvidence   ReportStatus = "PARTIAL_EVIDENCE"
	ReportInterruptedTask   ReportStatus = "INTERRUPTED_TASK"
	ReportObservationFailed ReportStatus = "OBSERVATION_FAILED"
)

type Report struct {
	SchemaVersion string       `json:"schema_version"`
	ReportID      string       `json:"report_id"`
	TaskID        string       `json:"task_id"`
	Status        ReportStatus `json:"status"`
	CreatedAt     time.Time    `json:"created_at"`
	Candidates    []Candidate  `json:"candidates"`
	Limitations   []string     `json:"limitations,omitempty"`
}

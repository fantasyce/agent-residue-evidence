package store

import (
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/contract"
	"github.com/fantasyce/agent-residue-evidence/internal/event"
	"github.com/fantasyce/agent-residue-evidence/internal/fsobserve"
	"github.com/fantasyce/agent-residue-evidence/internal/pathalias"
	processobserve "github.com/fantasyce/agent-residue-evidence/internal/process"
)

type TaskRecord struct {
	TaskID          string                   `json:"task_id"`
	State           contract.TaskState       `json:"state"`
	CreatedAt       time.Time                `json:"created_at"`
	HeartbeatAt     time.Time                `json:"heartbeat_at"`
	Baseline        fsobserve.Baseline       `json:"baseline"`
	ProcessBaseline processobserve.Baseline  `json:"process_baseline"`
	Events          []event.Summary          `json:"events"`
	Aliases         pathalias.Table          `json:"aliases"`
	ObservationMode contract.ObservationMode `json:"observation_mode"`
	RecoveryProfile contract.RecoveryProfile `json:"recovery_profile"`
	ExecutorGrants  map[string]ExecutorGrant `json:"executor_grants,omitempty"`
}

type ExecutorGrant struct {
	AppendKey    [32]byte  `json:"append_key"`
	ExpiresAt    time.Time `json:"expires_at"`
	AllowedTypes []string  `json:"allowed_types,omitempty"`
	AllowedRoots []string  `json:"allowed_roots,omitempty"`
}

type ReportRecord struct {
	Report         contract.Report                 `json:"report"`
	CompletedAt    time.Time                       `json:"completed_at"`
	Retained       bool                            `json:"retained"`
	Digest         string                          `json:"digest"`
	OriginalDigest string                          `json:"original_digest,omitempty"`
	ExactTargets   map[string]string               `json:"exact_targets,omitempty"`
	Aliases        pathalias.Table                 `json:"aliases"`
	Revisions      []contract.VerificationRevision `json:"revisions,omitempty"`
}

type OwnedEvidence struct {
	ExactTargets map[string]string
	Aliases      pathalias.Table
}

type OwnedTaskMetadata struct {
	Aliases         pathalias.Table
	ObservationMode contract.ObservationMode
	RecoveryProfile contract.RecoveryProfile
}

type RetrospectiveGrant struct {
	Scope     contract.TaskScope `json:"scope"`
	StartedAt time.Time          `json:"started_at"`
	EndedAt   time.Time          `json:"ended_at"`
	CreatedAt time.Time          `json:"created_at"`
	ExpiresAt time.Time          `json:"expires_at"`
}

type config struct {
	capacity     int64
	retention    time.Duration
	interruption time.Duration
	clock        func() time.Time
}

type Option func(*config)

func WithCapacity(bytes int64) Option {
	return func(config *config) { config.capacity = bytes }
}

func WithRetention(duration time.Duration) Option {
	return func(config *config) { config.retention = duration }
}

func WithInterruption(duration time.Duration) Option {
	return func(config *config) { config.interruption = duration }
}

func WithClock(clock func() time.Time) Option {
	return func(config *config) { config.clock = clock }
}

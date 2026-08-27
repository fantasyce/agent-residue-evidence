package store

import (
	"context"
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/contract"
	"github.com/fantasyce/agent-residue-evidence/internal/event"
	"github.com/fantasyce/agent-residue-evidence/internal/fsobserve"
	processobserve "github.com/fantasyce/agent-residue-evidence/internal/process"
)

type TaskRecord struct {
	TaskID          string                  `json:"task_id"`
	State           contract.TaskState      `json:"state"`
	CreatedAt       time.Time               `json:"created_at"`
	HeartbeatAt     time.Time               `json:"heartbeat_at"`
	Baseline        fsobserve.Baseline      `json:"baseline"`
	ProcessBaseline processobserve.Baseline `json:"process_baseline"`
	Events          []event.Summary         `json:"events"`
}

type ReportRecord struct {
	Report      contract.Report `json:"report"`
	CompletedAt time.Time       `json:"completed_at"`
	Retained    bool            `json:"retained"`
	Digest      string          `json:"digest"`
}

type Finalizer func(context.Context, TaskRecord) (contract.Report, error)

type config struct {
	capacity     int64
	retention    time.Duration
	interruption time.Duration
	clock        func() time.Time
	finalizer    Finalizer
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

func WithFinalizer(finalizer Finalizer) Option {
	return func(config *config) { config.finalizer = finalizer }
}

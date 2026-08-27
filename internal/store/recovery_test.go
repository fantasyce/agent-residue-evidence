package store

import (
	"context"
	"testing"
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/contract"
)

func TestSweepInterruptsAtTwentyFourHourBoundary(t *testing.T) {
	start := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	now := start.Add(24 * time.Hour)
	finalized := 0
	store := newTestStore(t,
		WithClock(func() time.Time { return start }),
		WithFinalizer(func(_ context.Context, task TaskRecord) (contract.Report, error) {
			finalized++
			return contract.Report{SchemaVersion: contract.ReportSchemaVersion, ReportID: "interrupted-" + task.TaskID, TaskID: task.TaskID, Status: contract.ReportInterruptedTask, CreatedAt: now, Candidates: []contract.Candidate{}}, nil
		}),
	)
	if err := store.CreateTask(context.Background(), "task-stale", testBaseline("task-stale", start)); err != nil {
		t.Fatal(err)
	}
	if err := store.Sweep(context.Background(), now.Add(-time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	if finalized != 0 {
		t.Fatal("task interrupted before boundary")
	}
	if err := store.Sweep(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if finalized != 1 {
		t.Fatalf("finalizer calls=%d", finalized)
	}
	record, err := store.GetReport("interrupted-task-stale")
	if err != nil || record.Report.Status != contract.ReportInterruptedTask {
		t.Fatalf("report=%#v err=%v", record, err)
	}
}

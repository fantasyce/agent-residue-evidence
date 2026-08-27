package store

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/contract"
	"github.com/fantasyce/agent-residue-evidence/internal/event"
	"github.com/fantasyce/agent-residue-evidence/internal/fsobserve"
)

func newTestStore(t *testing.T, options ...Option) *Store {
	t.Helper()
	store, err := Open(t.TempDir(), options...)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func testBaseline(taskID string, at time.Time) fsobserve.Baseline {
	return fsobserve.Baseline{CapturedAt: at, Entries: map[string]fsobserve.Entry{}}
}

func testReport(taskID, reportID string, at time.Time) contract.Report {
	return contract.Report{SchemaVersion: contract.ReportSchemaVersion, ReportID: reportID, TaskID: taskID, Status: contract.ReportNoCandidates, CreatedAt: at, Candidates: []contract.Candidate{}}
}

func TestCreateAppendRestartAndComplete(t *testing.T) {
	home := t.TempDir()
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	store, err := Open(home, WithClock(func() time.Time { return now }))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateTask(context.Background(), "task-1", testBaseline("task-1", now)); err != nil {
		t.Fatal(err)
	}
	assertPrivatePath(t, store.taskPath("task-1"), false)
	if err := store.AppendEvents(context.Background(), "task-1", nil); err != nil {
		t.Fatal(err)
	}
	restarted, err := Open(home, WithClock(func() time.Time { return now }))
	if err != nil {
		t.Fatal(err)
	}
	task, err := restarted.LoadTask("task-1")
	if err != nil || task.HeartbeatAt != now || task.State != contract.TaskActive {
		t.Fatalf("task=%#v err=%v", task, err)
	}
	report := testReport("task-1", "report-1", now)
	if err := restarted.CompleteTask(context.Background(), "task-1", report); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.LoadTask("task-1"); !os.IsNotExist(err) {
		t.Fatalf("completed task retained: %v", err)
	}
	if got, err := restarted.GetReport("report-1"); err != nil || got.Report.ReportID != "report-1" {
		t.Fatalf("report=%#v err=%v", got, err)
	}
	if err := restarted.VerifyReport("report-1"); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentEventAppendLosesNoEvents(t *testing.T) {
	now := time.Now().UTC()
	store := newTestStore(t, WithClock(func() time.Time { return now }))
	if err := store.CreateTask(context.Background(), "task-concurrent", testBaseline("task-concurrent", now)); err != nil {
		t.Fatal(err)
	}
	const count = 20
	var wait sync.WaitGroup
	for index := 0; index < count; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			err := store.AppendEvents(context.Background(), "task-concurrent", []event.Summary{{EventID: string(rune('a' + index)), Timestamp: now}})
			if err != nil {
				t.Errorf("append: %v", err)
			}
		}(index)
	}
	wait.Wait()
	task, err := store.LoadTask("task-concurrent")
	if err != nil || len(task.Events) != count {
		t.Fatalf("events=%d err=%v", len(task.Events), err)
	}
}

func TestStagingResidueAndCorruptReportAreNotTrusted(t *testing.T) {
	store := newTestStore(t)
	if err := os.WriteFile(filepath.Join(store.reportsDir, ".crash.tmp"), []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.reportsDir, "report-corrupt.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetReport("report-corrupt"); err == nil {
		t.Fatal("corrupt report trusted")
	}
}

func TestReportOperationsRejectPathsAndForgetExactReport(t *testing.T) {
	now := time.Now().UTC()
	store := newTestStore(t)
	if err := store.saveReport(ReportRecord{Report: testReport("task-1", "report-1", now), CompletedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.RetainReport("../report-1"); err == nil {
		t.Fatal("path accepted as report id")
	}
	if err := store.ForgetReport("report-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetReport("report-1"); !os.IsNotExist(err) {
		t.Fatalf("report not forgotten: %v", err)
	}
}

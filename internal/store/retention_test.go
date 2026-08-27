package store

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestSweepNeverEvictsActiveBaselineAndExpiresOldReport(t *testing.T) {
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	store := newTestStore(t, WithCapacity(100), WithClock(func() time.Time { return now }))
	if err := store.CreateTask(context.Background(), "task-active", testBaseline("task-active", now)); err != nil {
		t.Fatal(err)
	}
	old := testReport("task-old", "report-old", now.Add(-8*24*time.Hour))
	old.Limitations = []string{string(make([]byte, 200))}
	if err := store.saveReport(ReportRecord{Report: old, CompletedAt: old.CreatedAt}); err != nil {
		t.Fatal(err)
	}
	if err := store.Sweep(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadTask("task-active"); err != nil {
		t.Fatalf("active baseline lost: %v", err)
	}
	if _, err := store.GetReport("report-old"); !os.IsNotExist(err) {
		t.Fatalf("old report retained: %v", err)
	}
}

func TestCapacityEvictsOldestUnretainedAndPreservesRetained(t *testing.T) {
	now := time.Now().UTC()
	store := newTestStore(t, WithCapacity(900), WithRetention(30*24*time.Hour))
	for index, id := range []string{"old", "middle", "new"} {
		report := testReport("task-"+id, "report-"+id, now.Add(time.Duration(index-3)*time.Hour))
		report.Limitations = []string{string(make([]byte, 350))}
		if err := store.saveReport(ReportRecord{Report: report, CompletedAt: report.CreatedAt, Retained: id == "old"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Sweep(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetReport("report-old"); err != nil {
		t.Fatalf("retained report evicted: %v", err)
	}
	if _, err := store.GetReport("report-middle"); !os.IsNotExist(err) {
		t.Fatalf("oldest unretained report survived: %v", err)
	}
}

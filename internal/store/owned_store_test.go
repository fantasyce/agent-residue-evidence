package store

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/capability"
	"github.com/fantasyce/agent-residue-evidence/internal/contract"
	"github.com/fantasyce/agent-residue-evidence/internal/event"
	"github.com/fantasyce/agent-residue-evidence/internal/fsobserve"
	"github.com/fantasyce/agent-residue-evidence/internal/scope"
)

func TestOwnedTaskStateIsOpaqueAndOwnerIsRequired(t *testing.T) {
	home := t.TempDir()
	state, err := Open(home)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := capability.NewOwner()
	if err != nil {
		t.Fatal(err)
	}
	baseline := fsobserve.Baseline{
		CapturedAt: time.Now().UTC(),
		Scope: scope.Validated{TaskID: "goalboard-private-task", Roots: []scope.Root{{
			Path: "/Users/private/GoalBoard", Identity: "root-identity",
		}}},
		Entries: map[string]fsobserve.Entry{
			"private": {Path: "/Users/private/GoalBoard/build.log", Kind: "file"},
		},
	}
	if err := state.CreateOwnedTask(context.Background(), owner.String(), "goalboard-private-task", baseline); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(home, "tasks"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries=%v err=%v", entries, err)
	}
	if filepath.Ext(entries[0].Name()) != ".are" || bytes.Contains([]byte(entries[0].Name()), []byte("goalboard")) {
		t.Fatalf("non-opaque task filename: %s", entries[0].Name())
	}
	raw, err := os.ReadFile(filepath.Join(home, "tasks", entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range [][]byte{[]byte("goalboard-private-task"), []byte("GoalBoard"), []byte("/Users/private")} {
		if bytes.Contains(raw, forbidden) {
			t.Fatalf("state leaked %q: %s", forbidden, raw)
		}
	}
	loaded, err := state.LoadOwnedTask(owner.String())
	if err != nil || loaded.TaskID != "goalboard-private-task" {
		t.Fatalf("loaded=%#v err=%v", loaded, err)
	}
	other, _ := capability.NewOwner()
	if _, err := state.LoadOwnedTask(other.String()); err == nil || err.Error() != capability.ErrAccessDenied.Error() {
		t.Fatalf("cross-task access err=%v", err)
	}
}

func TestOwnedExecutorIsAppendOnlyAndRevokedOnCompletion(t *testing.T) {
	state := newTestStore(t)
	owner, err := capability.NewOwner()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := state.CreateOwnedTask(context.Background(), owner.String(), "task-owner", testBaseline("task-owner", now)); err != nil {
		t.Fatal(err)
	}
	executor, err := capability.NewExecutor(owner, now.Add(time.Hour), []string{"artifact_declared"}, []string{"workspace"})
	if err != nil {
		t.Fatal(err)
	}
	if err := state.AppendOwnedEvents(context.Background(), executor.String(), []event.Summary{{EventID: "event-1", Type: contract.EventArtifactDeclared, Timestamp: now}}); err != nil {
		t.Fatal(err)
	}
	if _, err := state.LoadOwnedTask(executor.String()); err == nil {
		t.Fatal("executor read task")
	}
	report := testReport("task-owner", "report-owner", now)
	if err := state.CompleteOwnedTask(context.Background(), owner.String(), report); err != nil {
		t.Fatal(err)
	}
	if err := state.AppendOwnedEvents(context.Background(), executor.String(), nil); err == nil {
		t.Fatal("executor remained active after completion")
	}
	got, err := state.GetOwnedReport(owner.String())
	if err != nil || got.Report.ReportID != "report-owner" {
		t.Fatalf("got=%#v err=%v", got, err)
	}
	if _, err := state.GetOwnedReport(executor.String()); err == nil {
		t.Fatal("executor read report")
	}
}

func TestOwnedRetentionRequiresOwnerAndExpiresWithoutDecryption(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	state := newTestStore(t, WithClock(func() time.Time { return now }), WithRetention(time.Hour), WithInterruption(30*time.Minute))
	owner, _ := capability.NewOwner()
	if err := state.CreateOwnedTask(context.Background(), owner.String(), "private-task", testBaseline("private-task", now)); err != nil {
		t.Fatal(err)
	}
	if err := state.Sweep(context.Background(), now.Add(31*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := state.LoadOwnedTask(owner.String()); err == nil {
		t.Fatal("expired active task remained readable")
	}

	owner, _ = capability.NewOwner()
	if err := state.CreateOwnedTask(context.Background(), owner.String(), "report-task", testBaseline("report-task", now)); err != nil {
		t.Fatal(err)
	}
	if err := state.CompleteOwnedTask(context.Background(), owner.String(), testReport("report-task", "report-private", now)); err != nil {
		t.Fatal(err)
	}
	other, _ := capability.NewOwner()
	if err := state.RetainOwnedReport(other.String()); err == nil || err.Error() != capability.ErrAccessDenied.Error() {
		t.Fatalf("wrong owner retain err=%v", err)
	}
	if err := state.RetainOwnedReport(owner.String()); err != nil {
		t.Fatal(err)
	}
	if err := state.Sweep(context.Background(), now.Add(61*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := state.GetOwnedReport(owner.String()); err != nil {
		t.Fatalf("retained report expired early: %v", err)
	}
	if err := state.ForgetOwnedReport(other.String()); err == nil || err.Error() != capability.ErrAccessDenied.Error() {
		t.Fatalf("wrong owner forget err=%v", err)
	}
	if err := state.ForgetOwnedReport(owner.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := state.GetOwnedReport(owner.String()); err == nil {
		t.Fatal("forgotten report remained readable")
	}
}

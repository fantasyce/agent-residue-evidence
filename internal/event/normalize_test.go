package event

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/contract"
	"github.com/fantasyce/agent-residue-evidence/internal/scope"
)

func eventScope(t *testing.T) scope.Validated {
	t.Helper()
	root := t.TempDir()
	validated, err := scope.NewGuard().Validate(contract.TaskScope{TaskID: "task-events", Workspace: root})
	if err != nil {
		t.Fatal(err)
	}
	return validated
}

func validEvent(kind contract.EventType, at time.Time) contract.TaskEvent {
	return contract.TaskEvent{
		SchemaVersion: contract.EventSchemaVersion,
		TaskID:        "task-events",
		EventID:       "event-" + string(kind),
		Type:          kind,
		Timestamp:     at,
	}
}

func TestNormalizeAcceptsAllEventTypesAndSortsByTime(t *testing.T) {
	validated := eventScope(t)
	types := []contract.EventType{
		contract.EventCommandStarted,
		contract.EventCommandCompleted,
		contract.EventProcessStarted,
		contract.EventProcessExited,
		contract.EventArtifactDeclared,
		contract.EventTestPhaseStarted,
		contract.EventTestPhaseCompleted,
		contract.EventCleanupAttempted,
	}
	base := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	batch := make([]contract.TaskEvent, 0, len(types))
	for index, kind := range types {
		e := validEvent(kind, base.Add(time.Duration(len(types)-index)*time.Second))
		e.EventID = fmt.Sprintf("event-%d", index)
		e.WorkingDir = validated.Roots[0].Path
		batch = append(batch, e)
	}
	got, err := Normalize(batch, validated)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(types) {
		t.Fatalf("summaries=%d", len(got))
	}
	for index := 1; index < len(got); index++ {
		if got[index].Timestamp.Before(got[index-1].Timestamp) {
			t.Fatalf("events not sorted: %#v", got)
		}
	}
}

func TestNormalizeEmptyBatchIsHeartbeat(t *testing.T) {
	got, err := Normalize(nil, eventScope(t))
	if err != nil || len(got) != 0 {
		t.Fatalf("got=%#v err=%v", got, err)
	}
}

func TestNormalizeRejectsDuplicateWrongTaskOutsidePathsAndOversizedBatch(t *testing.T) {
	validated := eventScope(t)
	now := time.Now().UTC()
	duplicate := validEvent(contract.EventCommandStarted, now)
	if _, err := Normalize([]contract.TaskEvent{duplicate, duplicate}, validated); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate error, got %v", err)
	}
	wrongTask := validEvent(contract.EventCommandStarted, now)
	wrongTask.TaskID = "other-task"
	if _, err := Normalize([]contract.TaskEvent{wrongTask}, validated); err == nil || !strings.Contains(err.Error(), "task_id") {
		t.Fatalf("expected task error, got %v", err)
	}
	outside := validEvent(contract.EventArtifactDeclared, now)
	outside.DeclaredOutputs = []string{filepath.Join(filepath.Dir(validated.Roots[0].Path), "outside")}
	if _, err := Normalize([]contract.TaskEvent{outside}, validated); err == nil || !strings.Contains(err.Error(), "outside task scope") {
		t.Fatalf("expected scope error, got %v", err)
	}
	batch := make([]contract.TaskEvent, MaxBatchEvents+1)
	for index := range batch {
		batch[index] = validEvent(contract.EventCommandStarted, now.Add(time.Duration(index)*time.Nanosecond))
		batch[index].EventID = fmt.Sprintf("event-%d", index)
	}
	if _, err := Normalize(batch, validated); err == nil || !strings.Contains(err.Error(), "batch") {
		t.Fatalf("expected batch error, got %v", err)
	}
}

func TestDecodeRejectsRawCommandAndEnvironmentFields(t *testing.T) {
	for _, prohibited := range []string{"command", "environment"} {
		payload := map[string]any{
			"schema_version": contract.EventSchemaVersion,
			"task_id":        "task-events",
			"event_id":       "event-1",
			"type":           contract.EventCommandStarted,
			"timestamp":      time.Now().UTC(),
			prohibited:       map[string]string{"SECRET": "must-not-enter"},
		}
		if prohibited == "command" {
			payload[prohibited] = "rm something"
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := contract.DecodeTaskEvent(raw); err == nil || !strings.Contains(err.Error(), "unknown field") {
			t.Fatalf("%s accepted: %v", prohibited, err)
		}
	}
}

func TestNormalizePreservesOnlySafeSummaryFields(t *testing.T) {
	validated := eventScope(t)
	output := filepath.Join(validated.Roots[0].Path, "result.json")
	if err := os.WriteFile(output, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	exitCode := 0
	e := validEvent(contract.EventCommandCompleted, time.Now().UTC())
	e.WorkingDir = validated.Roots[0].Path
	e.CommandFingerprint = "sha256:" + strings.Repeat("a", 64)
	e.ExitCode = &exitCode
	e.Process = &contract.ProcessIdentity{PID: 42, CreatedAt: time.Now().UTC().Add(-time.Second)}
	e.DeclaredOutputs = []string{output}
	e.ReceiptID = "receipt-1"
	got, err := Normalize([]contract.TaskEvent{e}, validated)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].CommandFingerprint != e.CommandFingerprint || got[0].ReceiptID != e.ReceiptID || got[0].Process == nil {
		t.Fatalf("unsafe or incomplete summary: %#v", got)
	}
}

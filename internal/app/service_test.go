package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/contract"
	"github.com/fantasyce/agent-residue-evidence/internal/store"
)

func testService(t *testing.T) *Service {
	t.Helper()
	state, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return New(state)
}

func TestBeginEndAndVerifyFilesystemResidue(t *testing.T) {
	service := testService(t)
	root := t.TempDir()
	ctx := context.Background()
	begin, err := service.Begin(ctx, contract.TaskScope{TaskID: "task-service", Workspace: root})
	if err != nil || begin.ObservationID == "" {
		t.Fatalf("begin=%#v err=%v", begin, err)
	}
	artifact := filepath.Join(root, "test-output")
	if err := os.Mkdir(artifact, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifact, "result.log"), []byte("result"), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := service.End(ctx, "task-service")
	if err != nil {
		t.Fatal(err)
	}
	wantStatus := contract.ReportReviewRequired
	if len(report.Limitations) > 0 {
		wantStatus = contract.ReportPartialEvidence
	}
	if report.Status != wantStatus || len(report.Candidates) < 2 {
		t.Fatalf("report=%#v", report)
	}
	if err := os.RemoveAll(artifact); err != nil {
		t.Fatal(err)
	}
	verified, err := service.Verify(ctx, report.ReportID)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range verified.Candidates {
		if candidate.Kind == contract.CandidateFile || candidate.Kind == contract.CandidateDirectory {
			if candidate.CurrentStatus != contract.StatusNoLongerPresent {
				t.Fatalf("candidate not updated: %#v", candidate)
			}
		}
	}
}

func TestNoEventBaselineAndBroadScopeRejection(t *testing.T) {
	service := testService(t)
	root := t.TempDir()
	if _, err := service.Begin(context.Background(), contract.TaskScope{TaskID: "task-empty", Workspace: root}); err != nil {
		t.Fatal(err)
	}
	report, err := service.End(context.Background(), "task-empty")
	if err != nil || report.Status != contract.ReportNoCandidates {
		t.Fatalf("report=%#v err=%v", report, err)
	}
	if _, err := service.Begin(context.Background(), contract.TaskScope{TaskID: "task-broad", Workspace: string(filepath.Separator)}); err == nil {
		t.Fatal("broad scope accepted")
	}
}

func TestInspectCompletedIsPartialAndNeverBaselineObserved(t *testing.T) {
	service := testService(t)
	root := t.TempDir()
	artifact := filepath.Join(root, "historical.log")
	if err := os.WriteFile(artifact, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	report, err := service.InspectCompleted(context.Background(), InspectCompletedInput{
		Scope:     contract.TaskScope{TaskID: "task-historical", Workspace: root},
		StartedAt: now.Add(-time.Hour), EndedAt: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != contract.ReportPartialEvidence || len(report.Candidates) == 0 {
		t.Fatalf("report=%#v", report)
	}
	for _, candidate := range report.Candidates {
		if candidate.EvidenceLevel == contract.EvidenceBaselineObserved {
			t.Fatal("retrospective evidence upgraded to baseline")
		}
	}
}

func TestEndRecordsObservationFailure(t *testing.T) {
	service := testService(t)
	root := t.TempDir()
	if _, err := service.Begin(context.Background(), contract.TaskScope{TaskID: "task-failure", Workspace: root}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(root); err != nil {
		t.Fatal(err)
	}
	report, err := service.End(context.Background(), "task-failure")
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != contract.ReportObservationFailed || len(report.Limitations) == 0 {
		t.Fatalf("report=%#v", report)
	}
}

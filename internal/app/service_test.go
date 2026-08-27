package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
	if err != nil || begin.ObservationID == "" || begin.OwnerHandle == "" {
		t.Fatalf("begin=%#v err=%v", begin, err)
	}
	artifact := filepath.Join(root, "test-output")
	if err := os.Mkdir(artifact, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifact, "result.log"), []byte("result"), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := service.End(ctx, begin.OwnerHandle)
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
	page, err := service.GetCandidatePage(begin.OwnerHandle, 0, "", 20)
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].DescendantCount < 1 {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	for _, candidate := range report.Candidates {
		if candidate.Path != "" && !strings.HasPrefix(candidate.Path, "workspace://") {
			t.Fatalf("absolute path exposed: %#v", candidate)
		}
	}
	if err := os.RemoveAll(artifact); err != nil {
		t.Fatal(err)
	}
	verified, err := service.Verify(ctx, begin.OwnerHandle)
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
	history, err := service.GetHistory(begin.OwnerHandle)
	if err != nil || len(history.Revisions) != 1 {
		t.Fatalf("history=%#v err=%v", history, err)
	}
	for _, candidate := range history.Report.Candidates {
		if (candidate.Kind == contract.CandidateFile || candidate.Kind == contract.CandidateDirectory) && candidate.CurrentStatus != contract.StatusPresent {
			t.Fatalf("original report mutated: %#v", candidate)
		}
	}
}

func TestNoEventBaselineAndBroadScopeRejection(t *testing.T) {
	service := testService(t)
	root := t.TempDir()
	begin, err := service.Begin(context.Background(), contract.TaskScope{TaskID: "task-empty", Workspace: root})
	if err != nil {
		t.Fatal(err)
	}
	report, err := service.End(context.Background(), begin.OwnerHandle)
	if err != nil || report.Status != contract.ReportNoCandidates {
		t.Fatalf("report=%#v err=%v", report, err)
	}
	if _, err := service.Begin(context.Background(), contract.TaskScope{TaskID: "task-broad", Workspace: string(filepath.Separator)}); err == nil {
		t.Fatal("broad scope accepted")
	}
}

func TestTaskAndReportIDsNeverAuthorizeCrossTaskAccess(t *testing.T) {
	service := testService(t)
	first, err := service.Begin(context.Background(), contract.TaskScope{TaskID: "shared-display-id", Workspace: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Begin(context.Background(), contract.TaskScope{TaskID: "other-task", Workspace: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.End(context.Background(), "shared-display-id"); err == nil || err.Error() != "access denied" {
		t.Fatalf("display id authorized end: %v", err)
	}
	if _, err := service.End(context.Background(), second.OwnerHandle); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetHistory(first.ObservationID); err == nil || err.Error() != "access denied" {
		t.Fatalf("observation id authorized read: %v", err)
	}
	if _, err := service.GetHistory(second.OwnerHandle); err != nil {
		t.Fatal(err)
	}
}

func TestOwnerResolvesOnlyExistingCandidateAndExecutorIsAppendOnly(t *testing.T) {
	service := testService(t)
	root := t.TempDir()
	begin, err := service.Begin(context.Background(), contract.TaskScope{TaskID: "delegated-task", Workspace: root})
	if err != nil {
		t.Fatal(err)
	}
	executor, err := service.DelegateExecutor(begin.OwnerHandle, time.Now().UTC().Add(time.Hour), []contract.EventType{contract.EventArtifactDeclared}, []string{"workspace"})
	if err != nil {
		t.Fatal(err)
	}
	event := contract.TaskEvent{SchemaVersion: contract.EventSchemaVersion, TaskID: "delegated-task", EventID: "event-1", Type: contract.EventArtifactDeclared, Timestamp: time.Now().UTC(), DeclaredOutputs: []string{"workspace://artifact.log"}}
	if err := service.AppendEvents(context.Background(), executor, []contract.TaskEvent{event}); err != nil {
		t.Fatal(err)
	}
	event.EventID = "event-2"
	event.Type = contract.EventCleanupAttempted
	if err := service.AppendEvents(context.Background(), executor, []contract.TaskEvent{event}); err == nil {
		t.Fatal("executor exceeded allowed event type")
	}
	if _, err := service.End(context.Background(), executor); err == nil {
		t.Fatal("executor ended observation")
	}
	artifact := filepath.Join(root, "artifact.log")
	if err := os.WriteFile(artifact, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := service.End(context.Background(), begin.OwnerHandle)
	if err != nil {
		t.Fatal(err)
	}
	var candidateID string
	for _, candidate := range report.Candidates {
		if candidate.Kind == contract.CandidateFile {
			candidateID = candidate.ID
			break
		}
	}
	resolved, err := service.ResolveCandidate(begin.OwnerHandle, candidateID)
	if err != nil || resolved != artifact {
		t.Fatalf("resolved=%q err=%v", resolved, err)
	}
	if _, err := service.ResolveCandidate(begin.OwnerHandle, "not-a-candidate"); err == nil {
		t.Fatal("unknown candidate resolved")
	}
	other, _ := service.Begin(context.Background(), contract.TaskScope{TaskID: "other", Workspace: t.TempDir()})
	if _, err := service.ResolveCandidate(other.OwnerHandle, candidateID); err == nil {
		t.Fatal("other owner resolved candidate")
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
	_, err := service.InspectCompleted(context.Background(), InspectCompletedInput{
		Scope:     contract.TaskScope{TaskID: "task-historical", Workspace: root},
		StartedAt: now.Add(-time.Hour), EndedAt: now.Add(time.Hour),
	})
	if err == nil || err.Error() != "access denied" {
		t.Fatalf("ungranted retrospective inspection err=%v", err)
	}
}

func TestRetrospectiveInspectionRequiresSingleUseExactScopeGrant(t *testing.T) {
	service := testService(t)
	root := t.TempDir()
	artifact := filepath.Join(root, "historical-granted.log")
	if err := os.WriteFile(artifact, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	grant, err := service.GrantRetrospective(InspectCompletedInput{
		Scope:     contract.TaskScope{TaskID: "retrospective-task", Workspace: root, ObservationMode: contract.ObservationRetrospective},
		StartedAt: now.Add(-time.Hour), EndedAt: now.Add(time.Hour),
	})
	if err != nil || grant.GrantHandle == "" {
		t.Fatalf("grant=%#v err=%v", grant, err)
	}
	report, err := service.InspectCompletedAuthorized(context.Background(), grant.GrantHandle, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != contract.ReportPartialEvidence || report.ObservationMode != contract.ObservationRetrospective {
		t.Fatalf("report=%#v", report)
	}
	for _, candidate := range report.Candidates {
		if candidate.EvidenceLevel == contract.EvidenceBaselineObserved || (candidate.Path != "" && !strings.HasPrefix(candidate.Path, "workspace://")) {
			t.Fatalf("retrospective candidate=%#v", candidate)
		}
	}
	if _, err := service.InspectCompletedAuthorized(context.Background(), grant.GrantHandle, nil); err == nil {
		t.Fatal("single-use grant reused")
	}
	if _, err := service.InspectCompletedAuthorized(context.Background(), "retrospective-task", nil); err == nil || err.Error() != "access denied" {
		t.Fatalf("task id authorized retrospective inspection: %v", err)
	}
}

func TestEndRecordsObservationFailure(t *testing.T) {
	service := testService(t)
	root := t.TempDir()
	begin, err := service.Begin(context.Background(), contract.TaskScope{TaskID: "task-failure", Workspace: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(root); err != nil {
		t.Fatal(err)
	}
	report, err := service.End(context.Background(), begin.OwnerHandle)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != contract.ReportObservationFailed || len(report.Limitations) == 0 {
		t.Fatalf("report=%#v", report)
	}
}

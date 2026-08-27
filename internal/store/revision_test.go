package store

import (
	"context"
	"testing"
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/capability"
	"github.com/fantasyce/agent-residue-evidence/internal/contract"
)

func TestVerificationRevisionAppendsWithoutChangingOriginal(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	state := newTestStore(t, WithClock(func() time.Time { return now }))
	owner, _ := capability.NewOwner()
	if err := state.CreateOwnedTask(context.Background(), owner.String(), "task-revision", testBaseline("task-revision", now)); err != nil {
		t.Fatal(err)
	}
	report := testReport("task-revision", "report-revision", now)
	report.Candidates = []contract.Candidate{{
		ID: "candidate-1", Kind: contract.CandidateFile, Path: "workspace://result.log",
		EvidenceLevel: contract.EvidenceBaselineObserved, CurrentStatus: contract.StatusPresent,
	}}
	if err := state.CompleteOwnedTask(context.Background(), owner.String(), report, OwnedEvidence{ExactTargets: map[string]string{"candidate-1": "/private/project/result.log"}}); err != nil {
		t.Fatal(err)
	}
	originalDigest, err := reportDigest(report)
	if err != nil {
		t.Fatal(err)
	}
	revision := contract.VerificationRevision{
		Revision: 1, CreatedAt: now.Add(time.Minute), PreviousDigest: originalDigest, Digest: "sha256:revision-1",
		Candidates: []contract.Candidate{{
			ID: "candidate-1", Kind: contract.CandidateFile, Path: "workspace://result.log",
			EvidenceLevel: contract.EvidenceBaselineObserved, CurrentStatus: contract.StatusNoLongerPresent,
		}},
	}
	if err := state.AppendOwnedRevision(owner.String(), revision); err != nil {
		t.Fatal(err)
	}
	got, err := state.GetOwnedReport(owner.String())
	if err != nil {
		t.Fatal(err)
	}
	if got.Report.Candidates[0].CurrentStatus != contract.StatusPresent {
		t.Fatalf("original mutated: %#v", got.Report.Candidates[0])
	}
	if len(got.Revisions) != 1 || got.Revisions[0].Candidates[0].CurrentStatus != contract.StatusNoLongerPresent {
		t.Fatalf("revisions=%#v", got.Revisions)
	}
	if got.ExactTargets["candidate-1"] != "/private/project/result.log" {
		t.Fatalf("targets=%#v", got.ExactTargets)
	}
	other, _ := capability.NewOwner()
	if err := state.AppendOwnedRevision(other.String(), revision); err == nil {
		t.Fatal("wrong owner appended revision")
	}
}

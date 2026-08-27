package correlate

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/contract"
	"github.com/fantasyce/agent-residue-evidence/internal/event"
	"github.com/fantasyce/agent-residue-evidence/internal/fsobserve"
)

func fileCandidate(path string, evidence contract.EvidenceLevel, status contract.CurrentStatus) contract.Candidate {
	return contract.Candidate{
		ID: "candidate-" + filepath.Base(path), Kind: contract.CandidateFile, Path: path,
		EvidenceLevel: evidence, CurrentStatus: status, Recommendation: "review",
	}
}

func TestCurrentPresenceWinsOverCleanupEvent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "output")
	got, err := BuildReport(Input{
		TaskID: "task-correlation",
		Now:    time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC),
		Diff: fsobserve.Diff{Candidates: []contract.Candidate{
			fileCandidate(path, contract.EvidenceBaselineObserved, contract.StatusPresent),
		}},
		Events: []event.Summary{{EventID: "cleanup-1", Type: contract.EventCleanupAttempted, DeclaredOutputs: []string{path}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Candidates[0].CurrentStatus != contract.StatusPresent {
		t.Fatalf("got %s", got.Candidates[0].CurrentStatus)
	}
	if len(got.Candidates[0].Conflicts) == 0 {
		t.Fatal("missing event conflict")
	}
}

func TestEvidenceOrderingAndDeterministicCandidateOrder(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a")
	b := filepath.Join(root, "b")
	c := filepath.Join(root, "c")
	got, err := BuildReport(Input{
		TaskID: "task-correlation",
		Now:    time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC),
		Diff: fsobserve.Diff{Candidates: []contract.Candidate{
			fileCandidate(c, contract.EvidenceUnattributed, contract.StatusPresent),
			fileCandidate(a, contract.EvidenceBaselineObserved, contract.StatusPresent),
			fileCandidate(b, contract.EvidenceInferred, contract.StatusPresent),
		}},
		Events: []event.Summary{
			{EventID: "artifact-1", Type: contract.EventArtifactDeclared, DeclaredOutputs: []string{a, b}, ReceiptID: "receipt-1"},
			{EventID: "artifact-2", Type: contract.EventArtifactDeclared, DeclaredOutputs: []string{c}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Candidates[0].Path != a || got.Candidates[1].Path != b || got.Candidates[2].Path != c {
		t.Fatalf("not deterministic: %#v", got.Candidates)
	}
	levels := []contract.EvidenceLevel{got.Candidates[0].EvidenceLevel, got.Candidates[1].EvidenceLevel, got.Candidates[2].EvidenceLevel}
	want := []contract.EvidenceLevel{contract.EvidenceBaselineObserved, contract.EvidenceReceiptBound, contract.EvidenceEventBound}
	for index := range want {
		if levels[index] != want[index] {
			t.Fatalf("candidate %d level=%s want=%s", index, levels[index], want[index])
		}
	}
	if len(got.Candidates[1].EventIDs) != 1 || len(got.Candidates[1].ReceiptIDs) != 1 {
		t.Fatalf("missing event provenance: %#v", got.Candidates[1])
	}
	if got.Status != contract.ReportReviewRequired || got.ReportID == "" {
		t.Fatalf("unexpected report: %#v", got)
	}
}

func TestNoCandidatesAndPartialEvidenceStatuses(t *testing.T) {
	now := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
	empty, err := BuildReport(Input{TaskID: "task-empty", Now: now})
	if err != nil || empty.Status != contract.ReportNoCandidates {
		t.Fatalf("empty=%#v err=%v", empty, err)
	}
	partial, err := BuildReport(Input{TaskID: "task-partial", Now: now, Diff: fsobserve.Diff{Limitations: []string{"permission denied"}}})
	if err != nil || partial.Status != contract.ReportPartialEvidence {
		t.Fatalf("partial=%#v err=%v", partial, err)
	}
}

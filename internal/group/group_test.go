package group

import (
	"testing"

	"github.com/fantasyce/agent-residue-evidence/internal/contract"
)

func TestDirectoryCoversRedundantDescendants(t *testing.T) {
	candidates := []contract.Candidate{
		{ID: "dir", Kind: contract.CandidateDirectory, Path: "workspace://node_modules", SizeBytes: 10, EvidenceLevel: contract.EvidenceBaselineObserved, CurrentStatus: contract.StatusPresent},
		{ID: "a", Kind: contract.CandidateFile, Path: "workspace://node_modules/a.js", SizeBytes: 20, EvidenceLevel: contract.EvidenceBaselineObserved, CurrentStatus: contract.StatusPresent},
		{ID: "b", Kind: contract.CandidateFile, Path: "workspace://node_modules/lib/b.js", SizeBytes: 30, EvidenceLevel: contract.EvidenceBaselineObserved, CurrentStatus: contract.StatusPresent},
	}
	groups := Candidates(candidates)
	if len(groups) != 1 {
		t.Fatalf("groups=%#v", groups)
	}
	if groups[0].ID != "dir" || groups[0].DescendantCount != 2 || groups[0].AggregateSizeBytes != 60 {
		t.Fatalf("group=%#v", groups[0])
	}
	if len(groups[0].SamplePaths) != 2 {
		t.Fatalf("samples=%v", groups[0].SamplePaths)
	}
}

func TestConflictingChildRemainsIndependent(t *testing.T) {
	candidates := []contract.Candidate{
		{ID: "dir", Kind: contract.CandidateDirectory, Path: "workspace://out", EvidenceLevel: contract.EvidenceBaselineObserved, CurrentStatus: contract.StatusPresent},
		{ID: "active", Kind: contract.CandidateFile, Path: "workspace://out/server.sock", EvidenceLevel: contract.EvidenceEventBound, CurrentStatus: contract.StatusActiveReference},
	}
	groups := Candidates(candidates)
	if len(groups) != 2 {
		t.Fatalf("groups=%#v", groups)
	}
}

func TestPageIsDeterministicAndNeverSilentlyTruncates(t *testing.T) {
	items := []contract.CandidateGroup{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	first, err := Page(items, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 2 || first.Total != 3 || first.NextCursor == "" {
		t.Fatalf("first=%#v", first)
	}
	second, err := Page(items, first.NextCursor, 2)
	if err != nil || len(second.Items) != 1 || second.Items[0].ID != "c" || second.NextCursor != "" {
		t.Fatalf("second=%#v err=%v", second, err)
	}
}

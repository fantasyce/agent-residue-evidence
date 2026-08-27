package fsobserve

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"

	"github.com/fantasyce/agent-residue-evidence/internal/contract"
)

func (o *Observer) Compare(ctx context.Context, baseline Baseline) (Diff, error) {
	current, rootIDs, err := o.snapshot(ctx, baseline.Scope)
	if err != nil {
		return Diff{}, err
	}
	if len(rootIDs) != len(baseline.RootIDs) {
		return Diff{}, fmt.Errorf("root identity changed: root count differs")
	}
	for i := range rootIDs {
		if rootIDs[i] != baseline.RootIDs[i] {
			return Diff{}, fmt.Errorf("root identity changed: root %d", i)
		}
	}

	diff := Diff{Candidates: []contract.Candidate{}}
	for key, entry := range current {
		before, existed := baseline.Entries[key]
		if !existed {
			diff.Candidates = append(diff.Candidates, candidateFor(entry, contract.StatusPresent, "created after task observation began"))
			continue
		}
		if entry.Identity != before.Identity || entry.Size != before.Size || !entry.ModTime.Equal(before.ModTime) || entry.Mode != before.Mode {
			diff.Candidates = append(diff.Candidates, candidateFor(entry, contract.StatusChangedSinceReport, "changed after task observation began"))
		}
	}
	for key, entry := range baseline.Entries {
		if _, exists := current[key]; !exists {
			diff.Removed = append(diff.Removed, entry.Relative)
		}
	}
	sort.Slice(diff.Candidates, func(i, j int) bool {
		return diff.Candidates[i].Path < diff.Candidates[j].Path
	})
	sort.Strings(diff.Removed)
	return diff, nil
}

func candidateFor(entry Entry, status contract.CurrentStatus, reason string) contract.Candidate {
	kind := contract.CandidateFile
	if entry.Kind == "directory" {
		kind = contract.CandidateDirectory
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s", kind, entry.Identity, entry.Path)))
	return contract.Candidate{
		ID:             fmt.Sprintf("fs-%x", digest[:12]),
		Kind:           kind,
		Path:           entry.Path,
		ObjectIdentity: entry.Identity,
		SizeBytes:      entry.Size,
		EvidenceLevel:  contract.EvidenceBaselineObserved,
		CurrentStatus:  status,
		Reason:         reason,
		Recommendation: "review",
	}
}

package correlate

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/contract"
	"github.com/fantasyce/agent-residue-evidence/internal/event"
	"github.com/fantasyce/agent-residue-evidence/internal/fsobserve"
)

type Input struct {
	TaskID string
	Now    time.Time
	Diff   fsobserve.Diff
	Events []event.Summary
}

func BuildReport(input Input) (contract.Report, error) {
	if input.TaskID == "" {
		return contract.Report{}, errors.New("task_id is required")
	}
	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	} else {
		input.Now = input.Now.UTC()
	}
	candidates := append([]contract.Candidate(nil), input.Diff.Candidates...)
	for index := range candidates {
		candidate := &candidates[index]
		for _, summary := range input.Events {
			if !eventMatchesCandidate(summary, *candidate) {
				continue
			}
			bound := contract.EvidenceEventBound
			if summary.ReceiptID != "" {
				bound = contract.EvidenceReceiptBound
			}
			candidate.EvidenceLevel = strongerEvidence(candidate.EvidenceLevel, bound)
			if summary.EventID != "" {
				candidate.EventIDs = appendUnique(candidate.EventIDs, summary.EventID)
			}
			if summary.ReceiptID != "" {
				candidate.ReceiptIDs = appendUnique(candidate.ReceiptIDs, summary.ReceiptID)
			}
			if summary.Type == contract.EventCleanupAttempted && candidate.CurrentStatus != contract.StatusNoLongerPresent {
				candidate.Conflicts = appendUnique(candidate.Conflicts, "cleanup was attempted but the object is still present or its current state is uncertain")
			}
		}
		candidate.Recommendation = "review"
		sort.Strings(candidate.EventIDs)
		sort.Strings(candidate.ReceiptIDs)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Kind == candidates[j].Kind {
			return candidates[i].ID < candidates[j].ID
		}
		return candidates[i].Kind < candidates[j].Kind
	})
	status := contract.ReportNoCandidates
	if len(candidates) > 0 {
		status = contract.ReportReviewRequired
	}
	if len(input.Diff.Limitations) > 0 {
		status = contract.ReportPartialEvidence
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s", input.TaskID, input.Now.Format(time.RFC3339Nano))))
	report := contract.Report{
		SchemaVersion: contract.ReportSchemaVersion,
		ReportID:      fmt.Sprintf("report-%x", digest[:12]),
		TaskID:        input.TaskID,
		Status:        status,
		CreatedAt:     input.Now,
		Candidates:    candidates,
		Limitations:   append([]string(nil), input.Diff.Limitations...),
	}
	if err := report.Validate(); err != nil {
		return contract.Report{}, err
	}
	return report, nil
}

func eventMatchesCandidate(summary event.Summary, candidate contract.Candidate) bool {
	if candidate.Path == "" {
		return false
	}
	for _, output := range summary.DeclaredOutputs {
		if filepath.Clean(output) == filepath.Clean(candidate.Path) {
			return true
		}
	}
	return false
}

func strongerEvidence(current, candidate contract.EvidenceLevel) contract.EvidenceLevel {
	rank := map[contract.EvidenceLevel]int{
		contract.EvidenceUnattributed:     0,
		contract.EvidenceInferred:         1,
		contract.EvidenceEventBound:       2,
		contract.EvidenceReceiptBound:     3,
		contract.EvidenceBaselineObserved: 4,
	}
	if rank[candidate] > rank[current] {
		return candidate
	}
	return current
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

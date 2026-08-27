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
	processobserve "github.com/fantasyce/agent-residue-evidence/internal/process"
)

type Input struct {
	TaskID             string
	Now                time.Time
	Diff               fsobserve.Diff
	Events             []event.Summary
	Processes          []processobserve.Evidence
	ProcessLimitations []processobserve.Limitation
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
	for _, processEvidence := range input.Processes {
		candidates = append(candidates, processCandidates(processEvidence)...)
	}
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
		if candidateKindRank(candidates[i].Kind) == candidateKindRank(candidates[j].Kind) {
			return candidates[i].ID < candidates[j].ID
		}
		return candidateKindRank(candidates[i].Kind) < candidateKindRank(candidates[j].Kind)
	})
	limitations := append([]string(nil), input.Diff.Limitations...)
	for _, limitation := range input.ProcessLimitations {
		limitations = append(limitations, fmt.Sprintf("%s: %s", limitation.Operation, limitation.Detail))
	}
	status := contract.ReportNoCandidates
	if len(candidates) > 0 {
		status = contract.ReportReviewRequired
	}
	if len(limitations) > 0 {
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
		Limitations:   limitations,
	}
	if err := report.Validate(); err != nil {
		return contract.Report{}, err
	}
	return report, nil
}

func processCandidates(evidence processobserve.Evidence) []contract.Candidate {
	identity := contract.ProcessIdentity{PID: evidence.Identity.PID, CreatedAt: evidence.Identity.CreatedAt.UTC()}
	level := processEvidenceLevel(evidence.Reason)
	identityText := fmt.Sprintf("%d:%s", identity.PID, identity.CreatedAt.Format(time.RFC3339Nano))
	processDigest := sha256.Sum256([]byte("process\x00" + identityText))
	result := []contract.Candidate{{
		ID: fmt.Sprintf("process-%x", processDigest[:12]), Kind: contract.CandidateProcess,
		ObjectIdentity: identityText, Process: &identity, ParentPID: evidence.ParentPID,
		EvidenceLevel: level, CurrentStatus: contract.StatusActiveReference,
		Reason: string(evidence.Reason), Recommendation: "review",
	}}
	for _, port := range evidence.Ports {
		portIdentity := contract.PortIdentity{Protocol: port.Protocol, Address: port.Address, Number: port.Number}
		portDigest := sha256.Sum256([]byte(fmt.Sprintf("port\x00%s\x00%s\x00%s\x00%d", identityText, port.Protocol, port.Address, port.Number)))
		processCopy := identity
		result = append(result, contract.Candidate{
			ID: fmt.Sprintf("port-%x", portDigest[:12]), Kind: contract.CandidatePort,
			ObjectIdentity: fmt.Sprintf("%s:%s:%d", identityText, port.Address, port.Number),
			Process:        &processCopy, Port: &portIdentity,
			EvidenceLevel: level, CurrentStatus: contract.StatusActiveReference,
			Reason: "listening port owned by attributed process", Recommendation: "review",
		})
	}
	return result
}

func processEvidenceLevel(reason processobserve.Attribution) contract.EvidenceLevel {
	switch reason {
	case processobserve.AttributionReceipt:
		return contract.EvidenceReceiptBound
	case processobserve.AttributionEvent:
		return contract.EvidenceEventBound
	default:
		return contract.EvidenceInferred
	}
}

func candidateKindRank(kind contract.CandidateKind) int {
	switch kind {
	case contract.CandidateFile:
		return 0
	case contract.CandidateDirectory:
		return 1
	case contract.CandidateProcess:
		return 2
	case contract.CandidatePort:
		return 3
	default:
		return 4
	}
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

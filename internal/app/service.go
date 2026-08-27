package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/contract"
	"github.com/fantasyce/agent-residue-evidence/internal/correlate"
	"github.com/fantasyce/agent-residue-evidence/internal/event"
	"github.com/fantasyce/agent-residue-evidence/internal/fsobserve"
	processobserve "github.com/fantasyce/agent-residue-evidence/internal/process"
	"github.com/fantasyce/agent-residue-evidence/internal/scope"
	"github.com/fantasyce/agent-residue-evidence/internal/store"
)

type Service struct {
	store *store.Store
	now   func() time.Time
}

type BeginResult struct {
	ObservationID string    `json:"observation_id"`
	TaskID        string    `json:"task_id"`
	StartedAt     time.Time `json:"started_at"`
}

type InspectCompletedInput struct {
	Scope     contract.TaskScope   `json:"scope"`
	StartedAt time.Time            `json:"started_at"`
	EndedAt   time.Time            `json:"ended_at"`
	Events    []contract.TaskEvent `json:"events,omitempty"`
}

func New(state *store.Store) *Service {
	return &Service{store: state, now: time.Now}
}

func (s *Service) Begin(ctx context.Context, taskScope contract.TaskScope) (BeginResult, error) {
	validated, err := scope.NewGuard().Validate(taskScope)
	if err != nil {
		return BeginResult{}, err
	}
	filesystem := fsobserve.NewObserver(fsobserve.Limits{})
	baseline, err := filesystem.Capture(ctx, validated)
	if err != nil {
		return BeginResult{}, err
	}
	processes := processobserve.NewNativeObserver(rootPaths(validated))
	processBaseline, err := processes.Baseline(ctx)
	if err != nil {
		return BeginResult{}, err
	}
	if err := s.store.CreateTask(ctx, taskScope.TaskID, baseline, processBaseline); err != nil {
		return BeginResult{}, err
	}
	return BeginResult{ObservationID: "observation-" + taskScope.TaskID, TaskID: taskScope.TaskID, StartedAt: baseline.CapturedAt}, nil
}

func (s *Service) AppendEvents(ctx context.Context, taskID string, events []contract.TaskEvent) error {
	task, err := s.store.LoadTask(taskID)
	if err != nil {
		return err
	}
	summaries, err := event.Normalize(events, task.Baseline.Scope)
	if err != nil {
		return err
	}
	return s.store.AppendEvents(ctx, taskID, summaries)
}

func (s *Service) End(ctx context.Context, taskID string) (contract.Report, error) {
	task, err := s.store.LoadTask(taskID)
	if err != nil {
		return contract.Report{}, err
	}
	now := s.now().UTC()
	diff, err := fsobserve.NewObserver(fsobserve.Limits{}).Compare(ctx, task.Baseline)
	if err != nil {
		report := failureReport(taskID, now, err)
		if saveErr := s.store.CompleteTask(ctx, taskID, report); saveErr != nil {
			return contract.Report{}, errors.Join(err, saveErr)
		}
		return report, nil
	}
	hints := processHints(task.Events)
	hints.CandidatePaths = candidatePaths(diff.Candidates)
	processObserver := processobserve.NewNativeObserver(rootPaths(task.Baseline.Scope))
	processEvidence, processLimitations := processObserver.Resolve(ctx, hints)
	processEvidence = excludeBaselineOnlyProcesses(processEvidence, task.ProcessBaseline, hints)
	report, err := correlate.BuildReport(correlate.Input{
		TaskID: taskID, Now: now, Diff: diff, Events: task.Events,
		Processes: processEvidence, ProcessLimitations: processLimitations,
	})
	if err != nil {
		return contract.Report{}, err
	}
	if err := s.store.CompleteTask(ctx, taskID, report); err != nil {
		return contract.Report{}, err
	}
	return report, nil
}

func (s *Service) InspectCompleted(ctx context.Context, input InspectCompletedInput) (contract.Report, error) {
	if input.StartedAt.IsZero() || input.EndedAt.IsZero() || input.EndedAt.Before(input.StartedAt) {
		return contract.Report{}, errors.New("a valid retrospective time window is required")
	}
	validated, err := scope.NewGuard().Validate(input.Scope)
	if err != nil {
		return contract.Report{}, err
	}
	summaries, err := event.Normalize(input.Events, validated)
	if err != nil {
		return contract.Report{}, err
	}
	baseline, err := fsobserve.NewObserver(fsobserve.Limits{}).Capture(ctx, validated)
	if err != nil {
		return contract.Report{}, err
	}
	diff := fsobserve.Diff{Candidates: retrospectiveCandidates(baseline, input.StartedAt.UTC(), input.EndedAt.UTC()), Limitations: []string{"no prospective baseline was available; retrospective attribution is limited to the declared roots and time window"}}
	hints := processHints(summaries)
	hints.CandidatePaths = candidatePaths(diff.Candidates)
	processEvidence, processLimitations := processobserve.NewNativeObserver(rootPaths(validated)).Resolve(ctx, hints)
	report, err := correlate.BuildReport(correlate.Input{
		TaskID: input.Scope.TaskID, Now: s.now().UTC(), Diff: diff, Events: summaries,
		Processes: processEvidence, ProcessLimitations: processLimitations,
	})
	if err != nil {
		return contract.Report{}, err
	}
	for index := range report.Candidates {
		if report.Candidates[index].EvidenceLevel == contract.EvidenceBaselineObserved {
			report.Candidates[index].EvidenceLevel = contract.EvidenceUnattributed
		}
	}
	if err := s.store.PutReport(report); err != nil {
		return contract.Report{}, err
	}
	return report, nil
}

func (s *Service) GetReport(reportID string) (contract.Report, error) {
	record, err := s.store.GetReport(reportID)
	return record.Report, err
}

func (s *Service) Verify(ctx context.Context, reportID string) (contract.Report, error) {
	record, err := s.store.GetReport(reportID)
	if err != nil {
		return contract.Report{}, err
	}
	report := record.Report
	identities := uniqueProcessIdentities(report.Candidates)
	verifiedProcesses, processLimitations := processobserve.NewNativeObserver(nil).Verify(ctx, identities)
	for _, limitation := range processLimitations {
		report.Limitations = append(report.Limitations, fmt.Sprintf("verify %s: %s", limitation.Operation, limitation.Detail))
	}
	for index := range report.Candidates {
		candidate := &report.Candidates[index]
		switch candidate.Kind {
		case contract.CandidateFile, contract.CandidateDirectory:
			status, err := fsobserve.VerifyCandidate(*candidate)
			if err != nil {
				candidate.CurrentStatus = contract.StatusUnknown
				candidate.Limitations = append(candidate.Limitations, err.Error())
			} else {
				candidate.CurrentStatus = status
			}
		case contract.CandidateProcess:
			if candidate.Process == nil {
				candidate.CurrentStatus = contract.StatusUnknown
			} else if _, exists := verifiedProcesses[candidate.Process.PID]; exists {
				candidate.CurrentStatus = contract.StatusActiveReference
			} else {
				candidate.CurrentStatus = contract.StatusNoLongerPresent
			}
		case contract.CandidatePort:
			candidate.CurrentStatus = verifyPort(*candidate, verifiedProcesses)
		}
	}
	verifiedAt := s.now().UTC()
	report.VerifiedAt = &verifiedAt
	if len(report.Limitations) > 0 {
		report.Status = contract.ReportPartialEvidence
	}
	if err := s.store.UpdateReport(reportID, report); err != nil {
		return contract.Report{}, err
	}
	return report, nil
}

func failureReport(taskID string, now time.Time, observationErr error) contract.Report {
	digest := sha256.Sum256([]byte(taskID + "\x00" + now.Format(time.RFC3339Nano) + "\x00failure"))
	return contract.Report{
		SchemaVersion: contract.ReportSchemaVersion, ReportID: fmt.Sprintf("report-%x", digest[:12]),
		TaskID: taskID, Status: contract.ReportObservationFailed, CreatedAt: now,
		Candidates: []contract.Candidate{}, Limitations: []string{observationErr.Error()},
	}
}

func rootPaths(validated scope.Validated) []string {
	paths := make([]string, 0, len(validated.Roots))
	for _, root := range validated.Roots {
		paths = append(paths, root.Path)
	}
	return paths
}

func processHints(events []event.Summary) processobserve.Hints {
	var hints processobserve.Hints
	for _, summary := range events {
		if summary.Process == nil {
			continue
		}
		identity := processobserve.Identity{PID: summary.Process.PID, CreatedAt: summary.Process.CreatedAt}
		if summary.ReceiptID != "" {
			hints.ReceiptProcesses = append(hints.ReceiptProcesses, identity)
		} else {
			hints.EventProcesses = append(hints.EventProcesses, identity)
		}
	}
	return hints
}

func candidatePaths(candidates []contract.Candidate) []string {
	paths := []string{}
	for _, candidate := range candidates {
		if candidate.Path != "" {
			paths = append(paths, candidate.Path)
		}
	}
	return paths
}

func excludeBaselineOnlyProcesses(evidence []processobserve.Evidence, baseline processobserve.Baseline, hints processobserve.Hints) []processobserve.Evidence {
	baselineIdentities := map[int]processobserve.Identity{}
	for _, metadata := range baseline.Processes {
		baselineIdentities[metadata.Identity.PID] = metadata.Identity
	}
	explicit := append(append([]processobserve.Identity(nil), hints.EventProcesses...), hints.ReceiptProcesses...)
	result := make([]processobserve.Evidence, 0, len(evidence))
	for _, item := range evidence {
		keep := false
		for _, identity := range explicit {
			if identity.PID == item.Identity.PID && identity.CreatedAt.Equal(item.Identity.CreatedAt) {
				keep = true
				break
			}
		}
		if before, existed := baselineIdentities[item.Identity.PID]; !existed || !before.CreatedAt.Equal(item.Identity.CreatedAt) {
			keep = true
		}
		if keep {
			result = append(result, item)
		}
	}
	return result
}

func retrospectiveCandidates(baseline fsobserve.Baseline, startedAt, endedAt time.Time) []contract.Candidate {
	candidates := []contract.Candidate{}
	for _, entry := range baseline.Entries {
		if entry.ModTime.Before(startedAt) || entry.ModTime.After(endedAt) {
			continue
		}
		kind := contract.CandidateFile
		if entry.Kind == "directory" {
			kind = contract.CandidateDirectory
		}
		digest := sha256.Sum256([]byte(fmt.Sprintf("retrospective\x00%s\x00%s", entry.Identity, entry.Path)))
		candidates = append(candidates, contract.Candidate{
			ID: fmt.Sprintf("retro-%x", digest[:12]), Kind: kind, Path: entry.Path,
			ObjectIdentity: entry.Identity, SizeBytes: entry.Size,
			EvidenceLevel: contract.EvidenceUnattributed, CurrentStatus: contract.StatusPresent,
			Reason: "object modification time falls within the declared task window", Recommendation: "review",
		})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ID < candidates[j].ID })
	return candidates
}

func uniqueProcessIdentities(candidates []contract.Candidate) []processobserve.Identity {
	seen := map[string]struct{}{}
	result := []processobserve.Identity{}
	for _, candidate := range candidates {
		if candidate.Process == nil {
			continue
		}
		key := fmt.Sprintf("%d:%s", candidate.Process.PID, candidate.Process.CreatedAt.Format(time.RFC3339Nano))
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, processobserve.Identity{PID: candidate.Process.PID, CreatedAt: candidate.Process.CreatedAt})
	}
	return result
}

func verifyPort(candidate contract.Candidate, processes map[int]processobserve.Evidence) contract.CurrentStatus {
	if candidate.Process == nil || candidate.Port == nil {
		return contract.StatusUnknown
	}
	process, exists := processes[candidate.Process.PID]
	if !exists {
		return contract.StatusNoLongerPresent
	}
	for _, port := range process.Ports {
		if port.Protocol == candidate.Port.Protocol && port.Address == candidate.Port.Address && port.Number == candidate.Port.Number {
			return contract.StatusActiveReference
		}
	}
	return contract.StatusNoLongerPresent
}

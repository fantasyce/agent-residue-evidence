package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/capability"
	"github.com/fantasyce/agent-residue-evidence/internal/contract"
	"github.com/fantasyce/agent-residue-evidence/internal/correlate"
	"github.com/fantasyce/agent-residue-evidence/internal/event"
	"github.com/fantasyce/agent-residue-evidence/internal/fsobserve"
	"github.com/fantasyce/agent-residue-evidence/internal/group"
	"github.com/fantasyce/agent-residue-evidence/internal/pathalias"
	processobserve "github.com/fantasyce/agent-residue-evidence/internal/process"
	"github.com/fantasyce/agent-residue-evidence/internal/scope"
	"github.com/fantasyce/agent-residue-evidence/internal/store"
)

type Service struct {
	store *store.Store
	now   func() time.Time
}

type BeginResult struct {
	ObservationID   string                   `json:"observation_id"`
	TaskID          string                   `json:"task_id"`
	StartedAt       time.Time                `json:"started_at"`
	OwnerHandle     string                   `json:"owner_handle,omitempty"`
	RecoveryProfile contract.RecoveryProfile `json:"recovery_profile"`
}

type ReportHistory struct {
	Report    contract.Report                 `json:"report"`
	Revisions []contract.VerificationRevision `json:"revisions"`
}

type ReportSummary struct {
	SchemaVersion   string                    `json:"schema_version"`
	ReportID        string                    `json:"report_id"`
	TaskID          string                    `json:"task_id"`
	ObservationMode contract.ObservationMode  `json:"observation_mode"`
	Status          contract.ReportStatus     `json:"status"`
	CreatedAt       time.Time                 `json:"created_at"`
	Revision        int                       `json:"revision"`
	CandidateTotal  int                       `json:"candidate_total"`
	Candidates      []contract.CandidateGroup `json:"candidates"`
	NextCursor      string                    `json:"next_cursor,omitempty"`
	Limitations     []string                  `json:"limitations,omitempty"`
}

type InspectCompletedInput struct {
	Scope     contract.TaskScope   `json:"scope"`
	StartedAt time.Time            `json:"started_at"`
	EndedAt   time.Time            `json:"ended_at"`
	Events    []contract.TaskEvent `json:"events,omitempty"`
}

type RetrospectiveGrantResult struct {
	GrantHandle string    `json:"grant_handle"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func New(state *store.Store) *Service {
	return &Service{store: state, now: time.Now}
}

func (s *Service) Begin(ctx context.Context, taskScope contract.TaskScope) (BeginResult, error) {
	if err := taskScope.ValidateMetadata(); err != nil {
		return BeginResult{}, err
	}
	if taskScope.ObservationMode == "" {
		taskScope.ObservationMode = contract.ObservationGuided
	}
	if taskScope.RecoveryProfile == "" {
		taskScope.RecoveryProfile = contract.RecoveryRecoverable
	}
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
	aliases, err := pathalias.New(validated)
	if err != nil {
		return BeginResult{}, err
	}
	owner, err := capability.NewOwner()
	if err != nil {
		return BeginResult{}, err
	}
	if err := s.store.CreateOwnedTaskWithMetadata(ctx, owner.String(), taskScope.TaskID, baseline, processBaseline, store.OwnedTaskMetadata{Aliases: aliases, ObservationMode: taskScope.ObservationMode, RecoveryProfile: taskScope.RecoveryProfile}); err != nil {
		return BeginResult{}, err
	}
	return BeginResult{ObservationID: "observation-" + owner.OpaqueID, TaskID: taskScope.TaskID, StartedAt: baseline.CapturedAt, OwnerHandle: owner.String(), RecoveryProfile: taskScope.RecoveryProfile}, nil
}

func (s *Service) AppendEvents(ctx context.Context, handle string, events []contract.TaskEvent) error {
	if executor, err := capability.ParseExecutor(handle, s.now().UTC()); err == nil {
		summaries, err := normalizeExecutorEvents(events, executor.AllowedRoots)
		if err != nil {
			return err
		}
		return s.store.AppendOwnedExecutorEvents(ctx, handle, summaries)
	}
	task, err := s.loadTaskForAppend(handle)
	if err != nil {
		return err
	}
	summaries, err := event.Normalize(events, task.Baseline.Scope)
	if err != nil {
		return err
	}
	if executor, parseErr := capability.ParseExecutor(handle, s.now().UTC()); parseErr == nil && !executorRootsAllowed(executor.AllowedRoots, summaries, task.Aliases) {
		return capability.ErrAccessDenied
	}
	return s.store.AppendOwnedEvents(ctx, handle, summaries)
}

func (s *Service) End(ctx context.Context, ownerHandle string) (contract.Report, error) {
	task, err := s.store.LoadOwnedTaskForEnd(ownerHandle)
	if err != nil {
		return contract.Report{}, err
	}
	task.Events, err = expandAliasedEvents(task.Events, task.Aliases)
	if err != nil {
		return contract.Report{}, err
	}
	now := s.now().UTC()
	diff, err := fsobserve.NewObserver(fsobserve.Limits{}).Compare(ctx, task.Baseline)
	if err != nil {
		report := failureReport(task.TaskID, now, err)
		report.ObservationMode = task.ObservationMode
		if saveErr := s.store.CompleteOwnedTask(ctx, ownerHandle, report); saveErr != nil {
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
		TaskID: task.TaskID, Now: now, Diff: diff, Events: task.Events,
		Processes: processEvidence, ProcessLimitations: processLimitations,
	})
	if err != nil {
		return contract.Report{}, err
	}
	report.ObservationMode = task.ObservationMode
	exactTargets, err := projectReportPaths(&report, task.Aliases)
	if err != nil {
		return contract.Report{}, err
	}
	if err := s.store.CompleteOwnedTask(ctx, ownerHandle, report, store.OwnedEvidence{ExactTargets: exactTargets, Aliases: task.Aliases}); err != nil {
		return contract.Report{}, err
	}
	return report, nil
}

func (s *Service) InspectCompleted(ctx context.Context, input InspectCompletedInput) (contract.Report, error) {
	return contract.Report{}, capability.ErrAccessDenied
}

func (s *Service) GrantRetrospective(input InspectCompletedInput) (RetrospectiveGrantResult, error) {
	if input.StartedAt.IsZero() || input.EndedAt.IsZero() || input.EndedAt.Before(input.StartedAt) {
		return RetrospectiveGrantResult{}, errors.New("a valid retrospective time window is required")
	}
	input.Scope.ObservationMode = contract.ObservationRetrospective
	input.Scope.RecoveryProfile = contract.RecoveryRecoverable
	validated, err := scope.NewGuard().Validate(input.Scope)
	if err != nil {
		return RetrospectiveGrantResult{}, err
	}
	if _, err := event.Normalize(input.Events, validated); err != nil {
		return RetrospectiveGrantResult{}, err
	}
	owner, err := capability.NewOwner()
	if err != nil {
		return RetrospectiveGrantResult{}, err
	}
	now := s.now().UTC()
	expires := now.Add(10 * time.Minute)
	grant := store.RetrospectiveGrant{Scope: input.Scope, StartedAt: input.StartedAt.UTC(), EndedAt: input.EndedAt.UTC(), CreatedAt: now, ExpiresAt: expires}
	if err := s.store.CreateRetrospectiveGrant(owner.String(), grant); err != nil {
		return RetrospectiveGrantResult{}, err
	}
	return RetrospectiveGrantResult{GrantHandle: owner.String(), ExpiresAt: expires}, nil
}

func (s *Service) InspectCompletedAuthorized(ctx context.Context, grantHandle string, events []contract.TaskEvent) (contract.Report, error) {
	grant, err := s.store.LoadRetrospectiveGrant(grantHandle)
	if err != nil {
		return contract.Report{}, err
	}
	validated, err := scope.NewGuard().Validate(grant.Scope)
	if err != nil {
		return contract.Report{}, err
	}
	summaries, err := event.Normalize(events, validated)
	if err != nil {
		return contract.Report{}, err
	}
	baseline, err := fsobserve.NewObserver(fsobserve.Limits{}).Capture(ctx, validated)
	if err != nil {
		return contract.Report{}, err
	}
	diff := fsobserve.Diff{Candidates: retrospectiveCandidates(baseline, grant.StartedAt.UTC(), grant.EndedAt.UTC()), Limitations: []string{"no prospective baseline was available; retrospective attribution is limited to the explicitly granted roots and time window"}}
	hints := processHints(summaries)
	hints.CandidatePaths = candidatePaths(diff.Candidates)
	processEvidence, processLimitations := processobserve.NewNativeObserver(rootPaths(validated)).Resolve(ctx, hints)
	report, err := correlate.BuildReport(correlate.Input{
		TaskID: grant.Scope.TaskID, Now: s.now().UTC(), Diff: diff, Events: summaries,
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
	report.ObservationMode = contract.ObservationRetrospective
	aliases, err := pathalias.New(validated)
	if err != nil {
		return contract.Report{}, err
	}
	exactTargets, err := projectReportPaths(&report, aliases)
	if err != nil {
		return contract.Report{}, err
	}
	if err := s.store.CompleteRetrospectiveGrant(grantHandle, report, store.OwnedEvidence{ExactTargets: exactTargets, Aliases: aliases}); err != nil {
		return contract.Report{}, err
	}
	return report, nil
}

func (s *Service) GetReport(ownerHandle string) (contract.Report, error) {
	record, err := s.store.GetOwnedReport(ownerHandle)
	return record.Report, err
}

func (s *Service) GetHistory(ownerHandle string) (ReportHistory, error) {
	record, err := s.store.GetOwnedReport(ownerHandle)
	if err != nil {
		return ReportHistory{}, err
	}
	return ReportHistory{Report: record.Report, Revisions: append([]contract.VerificationRevision(nil), record.Revisions...)}, nil
}

func (s *Service) GetCandidatePage(ownerHandle string, revision int, cursor string, limit int) (contract.CandidatePage, error) {
	record, err := s.store.GetOwnedReport(ownerHandle)
	if err != nil {
		return contract.CandidatePage{}, err
	}
	candidates := record.Report.Candidates
	if revision > 0 {
		if revision > len(record.Revisions) {
			return contract.CandidatePage{}, capability.ErrAccessDenied
		}
		candidates = record.Revisions[revision-1].Candidates
	} else if revision < 0 {
		return contract.CandidatePage{}, capability.ErrAccessDenied
	}
	return group.Page(group.Candidates(candidates), cursor, limit)
}

func (s *Service) Summarize(ownerHandle string, revision int, cursor string, limit int) (ReportSummary, error) {
	record, err := s.store.GetOwnedReport(ownerHandle)
	if err != nil {
		return ReportSummary{}, err
	}
	page, err := s.GetCandidatePage(ownerHandle, revision, cursor, limit)
	if err != nil {
		return ReportSummary{}, err
	}
	summary := ReportSummary{
		SchemaVersion: record.Report.SchemaVersion, ReportID: record.Report.ReportID, TaskID: record.Report.TaskID,
		ObservationMode: record.Report.ObservationMode, Status: record.Report.Status, CreatedAt: record.Report.CreatedAt,
		Revision: revision, CandidateTotal: page.Total, Candidates: page.Items, NextCursor: page.NextCursor,
		Limitations: append([]string(nil), record.Report.Limitations...),
	}
	if revision > 0 {
		selected := record.Revisions[revision-1]
		summary.CreatedAt = selected.CreatedAt
		summary.Limitations = append([]string(nil), selected.Limitations...)
	}
	return summary, nil
}

func (s *Service) DelegateExecutor(ownerHandle string, expiresAt time.Time, allowedTypes []contract.EventType, allowedRoots []string) (string, error) {
	if _, err := capability.ParseOwner(ownerHandle); err != nil {
		return "", capability.ErrAccessDenied
	}
	types := make([]string, 0, len(allowedTypes))
	for _, eventType := range allowedTypes {
		types = append(types, string(eventType))
	}
	return s.store.DelegateOwnedExecutor(ownerHandle, expiresAt, types, allowedRoots)
}

func (s *Service) ResolveCandidate(ownerHandle, candidateID string) (string, error) {
	record, err := s.store.GetOwnedReport(ownerHandle)
	if err != nil {
		return "", err
	}
	exact, exists := record.ExactTargets[candidateID]
	if !exists {
		return "", capability.ErrAccessDenied
	}
	for _, candidate := range record.Report.Candidates {
		if candidate.ID != candidateID || candidate.Path == "" {
			continue
		}
		resolved, err := record.Aliases.Resolve(candidate.Path)
		if err != nil || resolved != exact {
			return "", capability.ErrAccessDenied
		}
		return exact, nil
	}
	return "", capability.ErrAccessDenied
}

func (s *Service) Verify(ctx context.Context, ownerHandle string) (contract.VerificationRevision, error) {
	record, err := s.store.GetOwnedReport(ownerHandle)
	if err != nil {
		return contract.VerificationRevision{}, err
	}
	candidates := append([]contract.Candidate(nil), record.Report.Candidates...)
	if len(record.Revisions) > 0 {
		candidates = append([]contract.Candidate(nil), record.Revisions[len(record.Revisions)-1].Candidates...)
	}
	identities := uniqueProcessIdentities(candidates)
	verifiedProcesses, processLimitations := processobserve.NewNativeObserver(nil).Verify(ctx, identities)
	limitations := []string{}
	for _, limitation := range processLimitations {
		limitations = append(limitations, fmt.Sprintf("verify %s: %s", limitation.Operation, limitation.Detail))
	}
	for index := range candidates {
		candidate := &candidates[index]
		switch candidate.Kind {
		case contract.CandidateFile, contract.CandidateDirectory:
			exact, exists := record.ExactTargets[candidate.ID]
			if !exists {
				candidate.CurrentStatus = contract.StatusUnknown
				candidate.Limitations = append(candidate.Limitations, "exact candidate target is unavailable")
				continue
			}
			probe := *candidate
			probe.Path = exact
			status, err := fsobserve.VerifyCandidate(probe)
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
	previous := record.OriginalDigest
	if len(record.Revisions) > 0 {
		previous = record.Revisions[len(record.Revisions)-1].Digest
	}
	revision := contract.VerificationRevision{Revision: len(record.Revisions) + 1, CreatedAt: verifiedAt, PreviousDigest: previous, Candidates: candidates, Limitations: limitations}
	revision.Digest, err = verificationDigest(revision)
	if err != nil {
		return contract.VerificationRevision{}, err
	}
	if err := s.store.AppendOwnedRevision(ownerHandle, revision); err != nil {
		return contract.VerificationRevision{}, err
	}
	return revision, nil
}

func (s *Service) loadTaskForAppend(handle string) (store.TaskRecord, error) {
	return s.store.LoadOwnedTaskForAppend(handle)
}

func projectReportPaths(report *contract.Report, aliases pathalias.Table) (map[string]string, error) {
	targets := map[string]string{}
	for index := range report.Candidates {
		candidate := &report.Candidates[index]
		if candidate.Path == "" {
			continue
		}
		exact := candidate.Path
		alias, err := aliases.Project(exact)
		if err != nil {
			return nil, err
		}
		targets[candidate.ID] = exact
		candidate.Path = alias
	}
	return targets, nil
}

func verificationDigest(revision contract.VerificationRevision) (string, error) {
	copy := revision
	copy.Digest = ""
	raw, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func executorRootsAllowed(allowed []string, summaries []event.Summary, aliases pathalias.Table) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, summary := range summaries {
		paths := append([]string(nil), summary.DeclaredOutputs...)
		if summary.WorkingDir != "" {
			paths = append(paths, summary.WorkingDir)
		}
		for _, exact := range paths {
			alias, err := aliases.Project(exact)
			if err != nil || !allowedAlias(allowed, alias) {
				return false
			}
		}
	}
	return true
}

func allowedAlias(allowed []string, alias string) bool {
	for _, root := range allowed {
		prefix := root
		if root == "workspace" {
			prefix = "workspace://"
		}
		if alias == prefix || strings.HasPrefix(alias, strings.TrimSuffix(prefix, "/")+"/") {
			return true
		}
	}
	return false
}

func normalizeExecutorEvents(events []contract.TaskEvent, allowedRoots []string) ([]event.Summary, error) {
	if len(events) > event.MaxBatchEvents {
		return nil, errors.New("event batch exceeds size limit")
	}
	summaries := make([]event.Summary, 0, len(events))
	seen := map[string]struct{}{}
	for _, item := range events {
		if err := item.Validate(); err != nil {
			return nil, err
		}
		if _, exists := seen[item.EventID]; exists {
			return nil, errors.New("duplicate event_id")
		}
		seen[item.EventID] = struct{}{}
		paths := append([]string(nil), item.DeclaredOutputs...)
		if item.WorkingDir != "" {
			paths = append(paths, item.WorkingDir)
		}
		for _, alias := range paths {
			if !safeAlias(alias) || !allowedAlias(allowedRoots, alias) {
				return nil, capability.ErrAccessDenied
			}
		}
		summary := event.Summary{EventID: item.EventID, Type: item.Type, Timestamp: item.Timestamp.UTC(), WorkingDir: item.WorkingDir, CommandFingerprint: item.CommandFingerprint, DeclaredOutputs: append([]string(nil), item.DeclaredOutputs...), ReceiptID: item.ReceiptID}
		if item.ExitCode != nil {
			value := *item.ExitCode
			summary.ExitCode = &value
		}
		if item.Process != nil {
			value := *item.Process
			summary.Process = &value
		}
		summaries = append(summaries, summary)
	}
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Timestamp.Equal(summaries[j].Timestamp) {
			return summaries[i].EventID < summaries[j].EventID
		}
		return summaries[i].Timestamp.Before(summaries[j].Timestamp)
	})
	return summaries, nil
}

func safeAlias(value string) bool {
	if value == "" || !strings.Contains(value, "://") {
		return false
	}
	for _, part := range strings.Split(strings.ReplaceAll(value, "\\", "/"), "/") {
		if part == ".." {
			return false
		}
	}
	return true
}

func expandAliasedEvents(summaries []event.Summary, aliases pathalias.Table) ([]event.Summary, error) {
	result := append([]event.Summary(nil), summaries...)
	for index := range result {
		if strings.Contains(result[index].WorkingDir, "://") {
			exact, err := aliases.Resolve(result[index].WorkingDir)
			if err != nil {
				return nil, err
			}
			result[index].WorkingDir = exact
		}
		for outputIndex, output := range result[index].DeclaredOutputs {
			if !strings.Contains(output, "://") {
				continue
			}
			exact, err := aliases.Resolve(output)
			if err != nil {
				return nil, err
			}
			result[index].DeclaredOutputs[outputIndex] = exact
		}
	}
	return result, nil
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

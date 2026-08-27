package store

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/capability"
	"github.com/fantasyce/agent-residue-evidence/internal/contract"
	"github.com/fantasyce/agent-residue-evidence/internal/event"
	"github.com/fantasyce/agent-residue-evidence/internal/fsobserve"
	"github.com/fantasyce/agent-residue-evidence/internal/pathalias"
	processobserve "github.com/fantasyce/agent-residue-evidence/internal/process"
)

func (s *Store) CreateOwnedTask(_ context.Context, ownerHandle, taskID string, baseline fsobserve.Baseline, processBaselines ...processobserve.Baseline) error {
	var processBaseline processobserve.Baseline
	if len(processBaselines) > 0 {
		processBaseline = processBaselines[0]
	}
	aliases, _ := pathalias.New(baseline.Scope)
	return s.CreateOwnedTaskWithMetadata(context.Background(), ownerHandle, taskID, baseline, processBaseline, OwnedTaskMetadata{Aliases: aliases, ObservationMode: contract.ObservationGuided, RecoveryProfile: contract.RecoveryRecoverable})
}

func (s *Store) CreateOwnedTaskWithMetadata(_ context.Context, ownerHandle, taskID string, baseline fsobserve.Baseline, processBaseline processobserve.Baseline, metadata OwnedTaskMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner, err := capability.ParseOwner(ownerHandle)
	if err != nil {
		return capability.ErrAccessDenied
	}
	path := s.ownedTaskPath(owner.OpaqueID)
	if _, err := os.Stat(path); err == nil {
		return os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	now := s.config.clock().UTC()
	record := TaskRecord{TaskID: taskID, State: contract.TaskActive, CreatedAt: now, HeartbeatAt: now, Baseline: baseline, ProcessBaseline: processBaseline, Events: []event.Summary{}, Aliases: metadata.Aliases, ObservationMode: metadata.ObservationMode, RecoveryProfile: metadata.RecoveryProfile, ExecutorGrants: map[string]ExecutorGrant{}}
	return s.writeOwned(path, "task", owner, 0, now, now.Add(s.config.interruption), record)
}

func (s *Store) LoadOwnedTask(ownerHandle string) (TaskRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner, err := capability.ParseOwner(ownerHandle)
	if err != nil {
		return TaskRecord{}, capability.ErrAccessDenied
	}
	return s.loadOwnedTaskUnlocked(owner)
}

func (s *Store) LoadOwnedTaskForAppend(handle string) (TaskRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner, err := capability.ParseOwner(handle)
	if err != nil {
		return TaskRecord{}, capability.ErrAccessDenied
	}
	return s.loadOwnedTaskUnlocked(owner)
}

func (s *Store) LoadOwnedTaskForEnd(ownerHandle string) (TaskRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner, err := capability.ParseOwner(ownerHandle)
	if err != nil {
		return TaskRecord{}, capability.ErrAccessDenied
	}
	record, err := s.loadOwnedTaskUnlocked(owner)
	if err != nil {
		return TaskRecord{}, err
	}
	return s.mergeExecutorEvents(owner, record)
}

func (s *Store) loadOwnedTaskUnlocked(owner capability.Owner) (TaskRecord, error) {
	envelope, err := readEncryptedEnvelope(s.ownedTaskPath(owner.OpaqueID))
	if err != nil || envelope.RecordKind != "task" || envelope.OpaqueID != owner.OpaqueID || !s.config.clock().UTC().Before(envelope.ExpiresAt) {
		return TaskRecord{}, capability.ErrAccessDenied
	}
	if envelope.PublicKey != ownerPublicKey(owner) {
		return TaskRecord{}, capability.ErrAccessDenied
	}
	var record TaskRecord
	if err := openRecord(envelope, owner.RecordKey[:], &record); err != nil || record.State != contract.TaskActive {
		return TaskRecord{}, capability.ErrAccessDenied
	}
	return record, nil
}

func (s *Store) AppendOwnedEvents(_ context.Context, handle string, summaries []event.Summary) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.config.clock().UTC()
	owner, err := capability.ParseOwner(handle)
	if err != nil {
		return capability.ErrAccessDenied
	}
	record, err := s.loadOwnedTaskUnlocked(owner)
	if err != nil {
		return capability.ErrAccessDenied
	}
	record.Events = append(record.Events, summaries...)
	record.HeartbeatAt = now
	return s.writeOwned(s.ownedTaskPath(owner.OpaqueID), "task", owner, 0, record.CreatedAt, now.Add(s.config.interruption), record)
}

func (s *Store) DelegateOwnedExecutor(ownerHandle string, expiresAt time.Time, allowedTypes, allowedRoots []string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner, err := capability.ParseOwner(ownerHandle)
	if err != nil {
		return "", capability.ErrAccessDenied
	}
	record, err := s.loadOwnedTaskUnlocked(owner)
	if err != nil {
		return "", capability.ErrAccessDenied
	}
	executor, err := capability.NewExecutor(owner, expiresAt, allowedTypes, allowedRoots)
	if err != nil {
		return "", err
	}
	if record.ExecutorGrants == nil {
		record.ExecutorGrants = map[string]ExecutorGrant{}
	}
	record.ExecutorGrants[executor.GrantID] = ExecutorGrant{AppendKey: executor.AppendKey, ExpiresAt: time.Unix(executor.ExpiresUnix, 0).UTC(), AllowedTypes: append([]string(nil), executor.AllowedTypes...), AllowedRoots: append([]string(nil), executor.AllowedRoots...)}
	now := s.config.clock().UTC()
	if err := s.writeOwned(s.ownedTaskPath(owner.OpaqueID), "task", owner, 0, record.CreatedAt, now.Add(s.config.interruption), record); err != nil {
		return "", err
	}
	return executor.String(), nil
}

type executorBatch struct {
	GrantID string          `json:"grant_id"`
	Events  []event.Summary `json:"events"`
}

func (s *Store) AppendOwnedExecutorEvents(_ context.Context, executorHandle string, summaries []event.Summary) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.config.clock().UTC()
	executor, err := capability.ParseExecutor(executorHandle, now)
	if err != nil || !executorAllows(executor.AllowedTypes, summaries) {
		return capability.ErrAccessDenied
	}
	taskEnvelope, err := readEncryptedEnvelope(s.ownedTaskPath(executor.OpaqueID))
	if err != nil || taskEnvelope.RecordKind != "task" || !now.Before(taskEnvelope.ExpiresAt) {
		return capability.ErrAccessDenied
	}
	public, err := base64.RawURLEncoding.DecodeString(taskEnvelope.PublicKey)
	if err != nil || len(public) != ed25519.PublicKeySize || executor.Verify(ed25519.PublicKey(public)) != nil {
		return capability.ErrAccessDenied
	}
	directory := s.ownedEventDir(executor.OpaqueID)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	if err := protectPrivatePath(directory, true); err != nil {
		return err
	}
	random := make([]byte, 18)
	if _, err := rand.Read(random); err != nil {
		return err
	}
	name := executor.GrantID + "." + base64.RawURLEncoding.EncodeToString(random) + ".are"
	expires := time.Unix(executor.ExpiresUnix, 0).UTC()
	if taskEnvelope.ExpiresAt.Before(expires) {
		expires = taskEnvelope.ExpiresAt
	}
	envelope, err := sealRecord("executor-events", executor.OpaqueID+"."+executor.GrantID, 0, now, expires, taskEnvelope.PublicKey, false, executor.AppendKey[:], executorBatch{GrantID: executor.GrantID, Events: summaries})
	if err != nil {
		return err
	}
	return atomicWriteJSON(filepath.Join(directory, name), envelope)
}

func (s *Store) CompleteOwnedTask(_ context.Context, ownerHandle string, report contract.Report, evidence ...OwnedEvidence) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner, err := capability.ParseOwner(ownerHandle)
	if err != nil {
		return capability.ErrAccessDenied
	}
	record, err := s.loadOwnedTaskUnlocked(owner)
	if err == nil {
		record, err = s.mergeExecutorEvents(owner, record)
	}
	if err != nil || report.TaskID != record.TaskID || report.Validate() != nil {
		return capability.ErrAccessDenied
	}
	now := s.config.clock().UTC()
	digest, err := reportDigest(report)
	if err != nil {
		return err
	}
	reportRecord := ReportRecord{Report: report, CompletedAt: now, OriginalDigest: digest, Revisions: []contract.VerificationRevision{}}
	if len(evidence) > 0 {
		reportRecord.ExactTargets = cloneTargets(evidence[0].ExactTargets)
		reportRecord.Aliases = evidence[0].Aliases
	}
	if err := s.writeOwned(s.ownedReportPath(owner.OpaqueID), "report", owner, 0, now, now.Add(s.config.retention), reportRecord); err != nil {
		return err
	}
	if err := os.Remove(s.ownedTaskPath(owner.OpaqueID)); err != nil {
		return err
	}
	if err := removeDirectoryFiles(s.ownedEventDir(owner.OpaqueID)); err != nil {
		return err
	}
	return syncDirectory(s.tasksDir)
}

func (s *Store) AppendOwnedRevision(ownerHandle string, revision contract.VerificationRevision) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner, err := capability.ParseOwner(ownerHandle)
	if err != nil {
		return capability.ErrAccessDenied
	}
	path := s.ownedReportPath(owner.OpaqueID)
	envelope, err := readEncryptedEnvelope(path)
	if err != nil || envelope.RecordKind != "report" || envelope.PublicKey != ownerPublicKey(owner) {
		return capability.ErrAccessDenied
	}
	var record ReportRecord
	if err := openRecord(envelope, owner.RecordKey[:], &record); err != nil {
		return capability.ErrAccessDenied
	}
	wantRevision := len(record.Revisions) + 1
	previous := record.OriginalDigest
	if len(record.Revisions) > 0 {
		previous = record.Revisions[len(record.Revisions)-1].Digest
	}
	if revision.Revision != wantRevision || revision.CreatedAt.IsZero() || revision.PreviousDigest != previous || revision.Digest == "" {
		return errors.New("verification revision does not extend report chain")
	}
	record.Revisions = append(record.Revisions, revision)
	return s.writeOwnedProtected(path, "report", owner, envelope.Revision+1, envelope.CreatedAt, envelope.ExpiresAt, envelope.Protected, record)
}

func (s *Store) GetOwnedReport(ownerHandle string) (ReportRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner, err := capability.ParseOwner(ownerHandle)
	if err != nil {
		return ReportRecord{}, capability.ErrAccessDenied
	}
	envelope, err := readEncryptedEnvelope(s.ownedReportPath(owner.OpaqueID))
	if err != nil || envelope.RecordKind != "report" || envelope.OpaqueID != owner.OpaqueID || !s.config.clock().UTC().Before(envelope.ExpiresAt) || envelope.PublicKey != ownerPublicKey(owner) {
		return ReportRecord{}, capability.ErrAccessDenied
	}
	var record ReportRecord
	if err := openRecord(envelope, owner.RecordKey[:], &record); err != nil {
		return ReportRecord{}, capability.ErrAccessDenied
	}
	return record, nil
}

func (s *Store) RetainOwnedReport(ownerHandle string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner, err := capability.ParseOwner(ownerHandle)
	if err != nil {
		return capability.ErrAccessDenied
	}
	path := s.ownedReportPath(owner.OpaqueID)
	envelope, err := readEncryptedEnvelope(path)
	if err != nil || envelope.RecordKind != "report" || envelope.PublicKey != ownerPublicKey(owner) {
		return capability.ErrAccessDenied
	}
	var record ReportRecord
	if err := openRecord(envelope, owner.RecordKey[:], &record); err != nil {
		return capability.ErrAccessDenied
	}
	record.Retained = true
	return s.writeOwnedProtected(path, "report", owner, envelope.Revision, envelope.CreatedAt, envelope.ExpiresAt.Add(s.config.retention), true, record)
}

func (s *Store) ForgetOwnedReport(ownerHandle string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner, err := capability.ParseOwner(ownerHandle)
	if err != nil {
		return capability.ErrAccessDenied
	}
	path := s.ownedReportPath(owner.OpaqueID)
	envelope, err := readEncryptedEnvelope(path)
	if err != nil || envelope.RecordKind != "report" || envelope.PublicKey != ownerPublicKey(owner) {
		return capability.ErrAccessDenied
	}
	var record ReportRecord
	if err := openRecord(envelope, owner.RecordKey[:], &record); err != nil {
		return capability.ErrAccessDenied
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	return syncDirectory(s.reportsDir)
}

func (s *Store) CreateRetrospectiveGrant(ownerHandle string, grant RetrospectiveGrant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner, err := capability.ParseOwner(ownerHandle)
	if err != nil || grant.CreatedAt.IsZero() || grant.ExpiresAt.IsZero() || !grant.ExpiresAt.After(grant.CreatedAt) {
		return capability.ErrAccessDenied
	}
	path := s.ownedGrantPath(owner.OpaqueID)
	if _, err := os.Stat(path); err == nil {
		return os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return s.writeOwned(path, "retrospective-grant", owner, 0, grant.CreatedAt, grant.ExpiresAt, grant)
}

func (s *Store) LoadRetrospectiveGrant(ownerHandle string) (RetrospectiveGrant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner, err := capability.ParseOwner(ownerHandle)
	if err != nil {
		return RetrospectiveGrant{}, capability.ErrAccessDenied
	}
	envelope, err := readEncryptedEnvelope(s.ownedGrantPath(owner.OpaqueID))
	if err != nil || envelope.RecordKind != "retrospective-grant" || envelope.PublicKey != ownerPublicKey(owner) || !s.config.clock().UTC().Before(envelope.ExpiresAt) {
		return RetrospectiveGrant{}, capability.ErrAccessDenied
	}
	var grant RetrospectiveGrant
	if err := openRecord(envelope, owner.RecordKey[:], &grant); err != nil {
		return RetrospectiveGrant{}, capability.ErrAccessDenied
	}
	return grant, nil
}

func (s *Store) CompleteRetrospectiveGrant(ownerHandle string, report contract.Report, evidence OwnedEvidence) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner, err := capability.ParseOwner(ownerHandle)
	if err != nil {
		return capability.ErrAccessDenied
	}
	grantEnvelope, err := readEncryptedEnvelope(s.ownedGrantPath(owner.OpaqueID))
	if err != nil || grantEnvelope.RecordKind != "retrospective-grant" || grantEnvelope.PublicKey != ownerPublicKey(owner) || !s.config.clock().UTC().Before(grantEnvelope.ExpiresAt) {
		return capability.ErrAccessDenied
	}
	var grant RetrospectiveGrant
	if err := openRecord(grantEnvelope, owner.RecordKey[:], &grant); err != nil || report.TaskID != grant.Scope.TaskID || report.Validate() != nil {
		return capability.ErrAccessDenied
	}
	now := s.config.clock().UTC()
	digest, err := reportDigest(report)
	if err != nil {
		return err
	}
	record := ReportRecord{Report: report, CompletedAt: now, OriginalDigest: digest, ExactTargets: cloneTargets(evidence.ExactTargets), Aliases: evidence.Aliases, Revisions: []contract.VerificationRevision{}}
	if err := s.writeOwned(s.ownedReportPath(owner.OpaqueID), "report", owner, 0, now, now.Add(s.config.retention), record); err != nil {
		return err
	}
	if err := os.Remove(s.ownedGrantPath(owner.OpaqueID)); err != nil {
		return err
	}
	return syncDirectory(s.grantsDir)
}

func (s *Store) writeOwned(path, kind string, owner capability.Owner, revision int, createdAt, expiresAt time.Time, value any) error {
	return s.writeOwnedProtected(path, kind, owner, revision, createdAt, expiresAt, false, value)
}

func (s *Store) writeOwnedProtected(path, kind string, owner capability.Owner, revision int, createdAt, expiresAt time.Time, protected bool, value any) error {
	publicKey := ownerPublicKey(owner)
	envelope, err := sealRecord(kind, owner.OpaqueID, revision, createdAt, expiresAt, publicKey, protected, owner.RecordKey[:], value)
	if err != nil {
		return err
	}
	return atomicWriteJSON(path, envelope)
}

func readEncryptedEnvelope(path string) (encryptedEnvelope, error) {
	var envelope encryptedEnvelope
	if err := readJSON(path, &envelope); err != nil {
		return encryptedEnvelope{}, err
	}
	return envelope, nil
}

func ownerPublicKey(owner capability.Owner) string {
	private := ed25519.NewKeyFromSeed(owner.SigningSeed[:])
	return base64.RawURLEncoding.EncodeToString(private.Public().(ed25519.PublicKey))
}

func executorAllows(allowed []string, summaries []event.Summary) bool {
	if len(allowed) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(allowed))
	for _, value := range allowed {
		set[value] = struct{}{}
	}
	for _, summary := range summaries {
		if _, ok := set[string(summary.Type)]; !ok {
			return false
		}
	}
	return true
}

func (s *Store) mergeExecutorEvents(owner capability.Owner, record TaskRecord) (TaskRecord, error) {
	directory := s.ownedEventDir(owner.OpaqueID)
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return record, nil
	}
	if err != nil {
		return TaskRecord{}, err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".are" {
			continue
		}
		var grant ExecutorGrant
		var grantID string
		for candidate, value := range record.ExecutorGrants {
			if len(entry.Name()) > len(candidate) && entry.Name()[:len(candidate)+1] == candidate+"." {
				grantID, grant = candidate, value
				break
			}
		}
		if grantID == "" {
			return TaskRecord{}, capability.ErrAccessDenied
		}
		envelope, err := readEncryptedEnvelope(filepath.Join(directory, entry.Name()))
		if err != nil || envelope.RecordKind != "executor-events" || envelope.OpaqueID != owner.OpaqueID+"."+grantID || envelope.PublicKey != ownerPublicKey(owner) {
			return TaskRecord{}, capability.ErrAccessDenied
		}
		var batch executorBatch
		if err := openRecord(envelope, grant.AppendKey[:], &batch); err != nil || batch.GrantID != grantID {
			return TaskRecord{}, capability.ErrAccessDenied
		}
		record.Events = append(record.Events, batch.Events...)
	}
	return record, nil
}

func cloneTargets(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func (s *Store) ownedTaskPath(opaqueID string) string {
	return filepath.Join(s.tasksDir, opaqueID+".are")
}

func (s *Store) ownedReportPath(opaqueID string) string {
	return filepath.Join(s.reportsDir, opaqueID+".are")
}

func (s *Store) ownedGrantPath(opaqueID string) string {
	return filepath.Join(s.grantsDir, opaqueID+".are")
}

func (s *Store) ownedEventDir(opaqueID string) string {
	return filepath.Join(s.eventsDir, opaqueID)
}

func removeDirectoryFiles(directory string) error {
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return errors.New("unexpected nested executor event directory")
		}
		if err := os.Remove(filepath.Join(directory, entry.Name())); err != nil {
			return err
		}
	}
	return os.Remove(directory)
}

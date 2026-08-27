package store

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/capability"
	"github.com/fantasyce/agent-residue-evidence/internal/contract"
	"github.com/fantasyce/agent-residue-evidence/internal/event"
	"github.com/fantasyce/agent-residue-evidence/internal/fsobserve"
	processobserve "github.com/fantasyce/agent-residue-evidence/internal/process"
)

func (s *Store) CreateOwnedTask(_ context.Context, ownerHandle, taskID string, baseline fsobserve.Baseline, processBaselines ...processobserve.Baseline) error {
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
	var processBaseline processobserve.Baseline
	if len(processBaselines) > 0 {
		processBaseline = processBaselines[0]
	}
	record := TaskRecord{TaskID: taskID, State: contract.TaskActive, CreatedAt: now, HeartbeatAt: now, Baseline: baseline, ProcessBaseline: processBaseline, Events: []event.Summary{}}
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
	if owner, err := capability.ParseOwner(handle); err == nil {
		record, err := s.loadOwnedTaskUnlocked(owner)
		if err != nil {
			return capability.ErrAccessDenied
		}
		record.Events = append(record.Events, summaries...)
		record.HeartbeatAt = now
		return s.writeOwned(s.ownedTaskPath(owner.OpaqueID), "task", owner, 0, record.CreatedAt, now.Add(s.config.interruption), record)
	}
	executor, err := capability.ParseExecutor(handle, now)
	if err != nil {
		return capability.ErrAccessDenied
	}
	envelope, err := readEncryptedEnvelope(s.ownedTaskPath(executor.OpaqueID))
	if err != nil || envelope.RecordKind != "task" || !now.Before(envelope.ExpiresAt) {
		return capability.ErrAccessDenied
	}
	public, err := base64.RawURLEncoding.DecodeString(envelope.PublicKey)
	if err != nil || len(public) != ed25519.PublicKeySize || executor.Verify(ed25519.PublicKey(public)) != nil {
		return capability.ErrAccessDenied
	}
	if !executorAllows(executor.AllowedTypes, summaries) {
		return capability.ErrAccessDenied
	}
	var record TaskRecord
	if err := openRecord(envelope, executor.RecordKey[:], &record); err != nil || record.State != contract.TaskActive {
		return capability.ErrAccessDenied
	}
	record.Events = append(record.Events, summaries...)
	record.HeartbeatAt = now
	return s.writeExecutor(s.ownedTaskPath(executor.OpaqueID), envelope, executor.RecordKey[:], record, now.Add(s.config.interruption))
}

func (s *Store) CompleteOwnedTask(_ context.Context, ownerHandle string, report contract.Report) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	owner, err := capability.ParseOwner(ownerHandle)
	if err != nil {
		return capability.ErrAccessDenied
	}
	record, err := s.loadOwnedTaskUnlocked(owner)
	if err != nil || report.TaskID != record.TaskID || report.Validate() != nil {
		return capability.ErrAccessDenied
	}
	now := s.config.clock().UTC()
	reportRecord := ReportRecord{Report: report, CompletedAt: now}
	if err := s.writeOwned(s.ownedReportPath(owner.OpaqueID), "report", owner, 0, now, now.Add(s.config.retention), reportRecord); err != nil {
		return err
	}
	if err := os.Remove(s.ownedTaskPath(owner.OpaqueID)); err != nil {
		return err
	}
	return syncDirectory(s.tasksDir)
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
	return s.writeOwned(path, "report", owner, envelope.Revision, envelope.CreatedAt, envelope.ExpiresAt.Add(s.config.retention), record)
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

func (s *Store) writeOwned(path, kind string, owner capability.Owner, revision int, createdAt, expiresAt time.Time, value any) error {
	publicKey := ownerPublicKey(owner)
	envelope, err := sealRecord(kind, owner.OpaqueID, revision, createdAt, expiresAt, publicKey, owner.RecordKey[:], value)
	if err != nil {
		return err
	}
	return atomicWriteJSON(path, envelope)
}

func (s *Store) writeExecutor(path string, prior encryptedEnvelope, key []byte, value any, expiresAt time.Time) error {
	envelope, err := sealRecord(prior.RecordKind, prior.OpaqueID, prior.Revision, prior.CreatedAt, expiresAt, prior.PublicKey, key, value)
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

func (s *Store) ownedTaskPath(opaqueID string) string {
	return filepath.Join(s.tasksDir, opaqueID+".are")
}

func (s *Store) ownedReportPath(opaqueID string) string {
	return filepath.Join(s.reportsDir, opaqueID+".are")
}

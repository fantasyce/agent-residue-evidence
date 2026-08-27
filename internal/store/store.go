package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/contract"
	"github.com/fantasyce/agent-residue-evidence/internal/event"
	"github.com/fantasyce/agent-residue-evidence/internal/fsobserve"
	processobserve "github.com/fantasyce/agent-residue-evidence/internal/process"
)

var storeIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,255}$`)

type Store struct {
	home       string
	tasksDir   string
	reportsDir string
	config     config
	mu         sync.Mutex
}

func Open(home string, options ...Option) (*Store, error) {
	if home == "" {
		return nil, errors.New("store home is required")
	}
	abs, err := filepath.Abs(home)
	if err != nil {
		return nil, err
	}
	configuration := config{
		capacity: 100 * 1024 * 1024, retention: 7 * 24 * time.Hour,
		interruption: 24 * time.Hour, clock: time.Now,
	}
	for _, option := range options {
		option(&configuration)
	}
	if configuration.capacity <= 0 || configuration.retention <= 0 || configuration.interruption <= 0 || configuration.clock == nil {
		return nil, errors.New("store configuration is invalid")
	}
	store := &Store{home: abs, tasksDir: filepath.Join(abs, "tasks"), reportsDir: filepath.Join(abs, "reports"), config: configuration}
	for _, directory := range []string{store.home, store.tasksDir, store.reportsDir} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return nil, err
		}
		if err := protectPrivatePath(directory, true); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func (s *Store) CreateTask(_ context.Context, taskID string, baseline fsobserve.Baseline, processBaselines ...processobserve.Baseline) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateStoreID(taskID); err != nil {
		return err
	}
	path := s.taskPath(taskID)
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
	return atomicWriteJSON(path, record)
}

func (s *Store) AppendEvents(_ context.Context, taskID string, summaries []event.Summary) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.loadTaskUnlocked(taskID)
	if err != nil {
		return err
	}
	if record.State != contract.TaskActive {
		return errors.New("task is not active")
	}
	record.Events = append(record.Events, summaries...)
	record.HeartbeatAt = s.config.clock().UTC()
	return atomicWriteJSON(s.taskPath(taskID), record)
}

func (s *Store) CompleteTask(_ context.Context, taskID string, report contract.Report) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.loadTaskUnlocked(taskID)
	if err != nil {
		return err
	}
	if report.TaskID != taskID {
		return errors.New("report task_id does not match task")
	}
	if err := report.Validate(); err != nil {
		return err
	}
	if err := s.saveReportUnlocked(ReportRecord{Report: report, CompletedAt: s.config.clock().UTC()}); err != nil {
		return err
	}
	if err := os.Remove(s.taskPath(record.TaskID)); err != nil {
		return err
	}
	return syncDirectory(s.tasksDir)
}

func (s *Store) LoadTask(taskID string) (TaskRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadTaskUnlocked(taskID)
}

func (s *Store) loadTaskUnlocked(taskID string) (TaskRecord, error) {
	if err := validateStoreID(taskID); err != nil {
		return TaskRecord{}, err
	}
	var record TaskRecord
	if err := readJSON(s.taskPath(taskID), &record); err != nil {
		return TaskRecord{}, err
	}
	if record.TaskID != taskID || record.State != contract.TaskActive {
		return TaskRecord{}, errors.New("stored task identity or state is invalid")
	}
	return record, nil
}

func (s *Store) GetReport(reportID string) (ReportRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getReportUnlocked(reportID)
}

func (s *Store) getReportUnlocked(reportID string) (ReportRecord, error) {
	if err := validateStoreID(reportID); err != nil {
		return ReportRecord{}, err
	}
	var record ReportRecord
	if err := readJSON(s.reportPath(reportID), &record); err != nil {
		return ReportRecord{}, err
	}
	if record.Report.ReportID != reportID {
		return ReportRecord{}, errors.New("stored report identity is invalid")
	}
	if err := verifyRecord(record); err != nil {
		return ReportRecord{}, err
	}
	return record, nil
}

func (s *Store) VerifyReport(reportID string) error {
	_, err := s.GetReport(reportID)
	return err
}

func (s *Store) RetainReport(reportID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.getReportUnlocked(reportID)
	if err != nil {
		return err
	}
	record.Retained = true
	return s.saveReportUnlocked(record)
}

func (s *Store) ForgetReport(reportID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateStoreID(reportID); err != nil {
		return err
	}
	if err := os.Remove(s.reportPath(reportID)); err != nil {
		return err
	}
	return syncDirectory(s.reportsDir)
}

func (s *Store) saveReport(record ReportRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveReportUnlocked(record)
}

func (s *Store) PutReport(report contract.Report) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveReportUnlocked(ReportRecord{Report: report, CompletedAt: s.config.clock().UTC()})
}

func (s *Store) UpdateReport(reportID string, report contract.Report) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.getReportUnlocked(reportID)
	if err != nil {
		return err
	}
	if report.ReportID != reportID {
		return errors.New("updated report identity does not match")
	}
	record.Report = report
	return s.saveReportUnlocked(record)
}

func (s *Store) saveReportUnlocked(record ReportRecord) error {
	if err := validateStoreID(record.Report.ReportID); err != nil {
		return err
	}
	if err := record.Report.Validate(); err != nil {
		return err
	}
	if record.CompletedAt.IsZero() {
		record.CompletedAt = s.config.clock().UTC()
	}
	digest, err := reportDigest(record.Report)
	if err != nil {
		return err
	}
	record.Digest = digest
	return atomicWriteJSON(s.reportPath(record.Report.ReportID), record)
}

func verifyRecord(record ReportRecord) error {
	if err := record.Report.Validate(); err != nil {
		return err
	}
	digest, err := reportDigest(record.Report)
	if err != nil {
		return err
	}
	if record.Digest == "" || record.Digest != digest {
		return errors.New("report integrity check failed")
	}
	return nil
}

func reportDigest(report contract.Report) (string, error) {
	encoded, err := json.Marshal(report)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func validateStoreID(value string) error {
	if !storeIDPattern.MatchString(value) {
		return errors.New("invalid store identifier")
	}
	return nil
}

func (s *Store) taskPath(taskID string) string { return filepath.Join(s.tasksDir, taskID+".json") }
func (s *Store) reportPath(reportID string) string {
	return filepath.Join(s.reportsDir, reportID+".json")
}

func readJSON(path string, target any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 128*1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("stored JSON has trailing data")
	}
	return nil
}

func atomicWriteJSON(path string, value any) (returnErr error) {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".write-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		if returnErr != nil {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := protectPrivatePath(temporaryPath, false); err != nil {
		_ = temporary.Close()
		return err
	}
	encoder := json.NewEncoder(temporary)
	if err := encoder.Encode(value); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := replaceFile(temporaryPath, path); err != nil {
		return err
	}
	if err := syncDirectory(directory); err != nil {
		return fmt.Errorf("sync parent directory: %w", err)
	}
	return nil
}

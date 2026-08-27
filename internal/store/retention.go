package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/contract"
)

type reportFile struct {
	path   string
	size   int64
	record ReportRecord
}

func (s *Store) Sweep(ctx context.Context, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now = now.UTC()
	if err := s.interruptStaleTasks(ctx, now); err != nil {
		return err
	}
	reports, err := s.reportFiles()
	if err != nil {
		return err
	}
	kept := make([]reportFile, 0, len(reports))
	var total int64
	for _, report := range reports {
		if !report.record.Retained && now.Sub(report.record.CompletedAt) >= s.config.retention {
			if err := os.Remove(report.path); err != nil {
				return err
			}
			continue
		}
		kept = append(kept, report)
		total += report.size
	}
	if total > s.config.capacity {
		sort.Slice(kept, func(i, j int) bool { return kept[i].record.CompletedAt.Before(kept[j].record.CompletedAt) })
		for _, report := range kept {
			if total <= s.config.capacity {
				break
			}
			if report.record.Retained {
				continue
			}
			if err := os.Remove(report.path); err != nil {
				return err
			}
			total -= report.size
		}
	}
	return syncDirectory(s.reportsDir)
}

func (s *Store) interruptStaleTasks(ctx context.Context, now time.Time) error {
	entries, err := os.ReadDir(s.tasksDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		taskID := strings.TrimSuffix(entry.Name(), ".json")
		record, err := s.loadTaskUnlocked(taskID)
		if err != nil {
			return err
		}
		if now.Sub(record.HeartbeatAt) < s.config.interruption {
			continue
		}
		finalizer := s.config.finalizer
		if finalizer == nil {
			finalizer = func(_ context.Context, task TaskRecord) (contract.Report, error) {
				return contract.Report{
					SchemaVersion: contract.ReportSchemaVersion, ReportID: "interrupted-" + task.TaskID,
					TaskID: task.TaskID, Status: contract.ReportInterruptedTask, CreatedAt: now,
					Candidates: []contract.Candidate{}, Limitations: []string{"task heartbeat expired before a normal end observation"},
				}, nil
			}
		}
		report, err := finalizer(ctx, record)
		if err != nil {
			return err
		}
		if report.Status != contract.ReportInterruptedTask || report.TaskID != taskID {
			return errors.New("interruption finalizer returned an invalid report")
		}
		if err := s.saveReportUnlocked(ReportRecord{Report: report, CompletedAt: now}); err != nil {
			return err
		}
		if err := os.Remove(s.taskPath(taskID)); err != nil {
			return err
		}
	}
	return syncDirectory(s.tasksDir)
}

func (s *Store) reportFiles() ([]reportFile, error) {
	entries, err := os.ReadDir(s.reportsDir)
	if err != nil {
		return nil, err
	}
	reports := []reportFile{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(s.reportsDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		var record ReportRecord
		if err := readJSON(path, &record); err != nil {
			return nil, err
		}
		if err := verifyRecord(record); err != nil {
			return nil, err
		}
		reports = append(reports, reportFile{path: path, size: info.Size(), record: record})
	}
	return reports, nil
}

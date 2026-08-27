package store

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (s *Store) Sweep(_ context.Context, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now = now.UTC()
	return s.sweepOwned(now)
}

func (s *Store) sweepOwned(now time.Time) error {
	for _, directory := range []string{s.tasksDir, s.reportsDir, s.grantsDir} {
		entries, err := os.ReadDir(directory)
		if err != nil {
			return err
		}
		type ownedFile struct {
			path      string
			size      int64
			createdAt time.Time
			protected bool
		}
		kept := []ownedFile{}
		var total int64
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".are") {
				continue
			}
			path := filepath.Join(directory, entry.Name())
			envelope, err := readEncryptedEnvelope(path)
			if err != nil {
				return err
			}
			if !now.Before(envelope.ExpiresAt) {
				if err := os.Remove(path); err != nil {
					return err
				}
				if directory == s.tasksDir {
					if err := removeDirectoryFiles(s.ownedEventDir(envelope.OpaqueID)); err != nil {
						return err
					}
				}
				continue
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			kept = append(kept, ownedFile{path: path, size: info.Size(), createdAt: envelope.CreatedAt, protected: envelope.Protected})
			total += info.Size()
		}
		if directory == s.reportsDir && total > s.config.capacity {
			sort.Slice(kept, func(i, j int) bool { return kept[i].createdAt.Before(kept[j].createdAt) })
			for _, record := range kept {
				if total <= s.config.capacity {
					break
				}
				if record.protected {
					continue
				}
				if err := os.Remove(record.path); err != nil {
					return err
				}
				total -= record.size
			}
		}
		if err := syncDirectory(directory); err != nil {
			return err
		}
	}
	return syncDirectory(s.eventsDir)
}

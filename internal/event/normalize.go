package event

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/contract"
	"github.com/fantasyce/agent-residue-evidence/internal/scope"
)

const MaxBatchEvents = 1000

type Summary struct {
	EventID            string
	Type               contract.EventType
	Timestamp          time.Time
	WorkingDir         string
	CommandFingerprint string
	ExitCode           *int
	Process            *contract.ProcessIdentity
	DeclaredOutputs    []string
	ReceiptID          string
}

func Normalize(batch []contract.TaskEvent, validated scope.Validated) ([]Summary, error) {
	if len(batch) > MaxBatchEvents {
		return nil, fmt.Errorf("event batch exceeds %d entries", MaxBatchEvents)
	}
	if len(batch) == 0 {
		return []Summary{}, nil
	}
	seen := make(map[string]struct{}, len(batch))
	summaries := make([]Summary, 0, len(batch))
	for index := range batch {
		e := batch[index]
		if err := e.Validate(); err != nil {
			return nil, fmt.Errorf("event %d: %w", index, err)
		}
		if e.TaskID != validated.TaskID {
			return nil, fmt.Errorf("event %d task_id does not match observed task", index)
		}
		if _, duplicate := seen[e.EventID]; duplicate {
			return nil, fmt.Errorf("duplicate event_id %q", e.EventID)
		}
		seen[e.EventID] = struct{}{}

		workingDir := ""
		if e.WorkingDir != "" {
			var err error
			workingDir, err = normalizeScopedPath(e.WorkingDir, "", validated)
			if err != nil {
				return nil, fmt.Errorf("event %d working_directory: %w", index, err)
			}
		}
		base := workingDir
		if base == "" {
			base = validated.Roots[0].Path
		}
		outputs := make([]string, 0, len(e.DeclaredOutputs))
		for _, output := range e.DeclaredOutputs {
			normalized, err := normalizeScopedPath(output, base, validated)
			if err != nil {
				return nil, fmt.Errorf("event %d declared_outputs: %w", index, err)
			}
			outputs = append(outputs, normalized)
		}
		var process *contract.ProcessIdentity
		if e.Process != nil {
			copied := *e.Process
			process = &copied
		}
		var exitCode *int
		if e.ExitCode != nil {
			copied := *e.ExitCode
			exitCode = &copied
		}
		summaries = append(summaries, Summary{
			EventID:            e.EventID,
			Type:               e.Type,
			Timestamp:          e.Timestamp.UTC(),
			WorkingDir:         workingDir,
			CommandFingerprint: e.CommandFingerprint,
			ExitCode:           exitCode,
			Process:            process,
			DeclaredOutputs:    outputs,
			ReceiptID:          e.ReceiptID,
		})
	}
	sort.SliceStable(summaries, func(i, j int) bool {
		if summaries[i].Timestamp.Equal(summaries[j].Timestamp) {
			return summaries[i].EventID < summaries[j].EventID
		}
		return summaries[i].Timestamp.Before(summaries[j].Timestamp)
	})
	return summaries, nil
}

func normalizeScopedPath(raw, base string, validated scope.Validated) (string, error) {
	if raw == "" || strings.IndexByte(raw, 0) >= 0 {
		return "", errors.New("path is invalid")
	}
	path := raw
	if !filepath.IsAbs(path) {
		if base == "" {
			return "", errors.New("relative path has no working directory")
		}
		path = filepath.Join(base, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	for _, root := range validated.Roots {
		if pathWithin(root.Path, abs) {
			resolvedRoot, err := filepath.EvalSymlinks(root.Path)
			if err != nil {
				return "", err
			}
			resolved, err := resolveExistingPrefix(abs)
			if err != nil {
				return "", err
			}
			if !pathWithin(resolvedRoot, resolved) {
				return "", errors.New("path resolves outside task scope")
			}
			return abs, nil
		}
	}
	return "", errors.New("path is outside task scope")
}

func pathWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func resolveExistingPrefix(path string) (string, error) {
	probe := path
	for {
		_, err := os.Lstat(probe)
		if err == nil {
			resolved, err := filepath.EvalSymlinks(probe)
			if err != nil {
				return "", err
			}
			remainder, err := filepath.Rel(probe, path)
			if err != nil || remainder == "." {
				return resolved, err
			}
			return filepath.Join(resolved, remainder), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return path, nil
		}
		probe = parent
	}
}

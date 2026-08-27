package fsobserve

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/scope"
)

type Observer struct {
	limits Limits
	now    func() time.Time
}

func NewObserver(limits Limits) *Observer {
	if limits.MaxEntries <= 0 {
		limits.MaxEntries = 100_000
	}
	if limits.MaxBytes <= 0 {
		limits.MaxBytes = 1 << 40
	}
	if limits.MaxDuration <= 0 {
		limits.MaxDuration = 30 * time.Second
	}
	return &Observer{limits: limits, now: time.Now}
}

func (o *Observer) Capture(ctx context.Context, validated scope.Validated) (Baseline, error) {
	entries, rootIDs, err := o.snapshot(ctx, validated)
	if err != nil {
		return Baseline{}, err
	}
	return Baseline{
		Scope:      validated,
		CapturedAt: o.now().UTC(),
		Entries:    entries,
		RootIDs:    rootIDs,
	}, nil
}

func (o *Observer) snapshot(ctx context.Context, validated scope.Validated) (map[string]Entry, []string, error) {
	ctx, cancel := context.WithTimeout(ctx, o.limits.MaxDuration)
	defer cancel()

	entries := make(map[string]Entry)
	rootIDs := make([]string, 0, len(validated.Roots))
	var totalBytes int64
	for rootIndex, root := range validated.Roots {
		rootInfo, err := os.Lstat(root.Path)
		if err != nil {
			return nil, nil, fmt.Errorf("stat root %q: %w", root.Path, err)
		}
		rootID, err := objectIdentity(root.Path, rootInfo)
		if err != nil {
			return nil, nil, fmt.Errorf("identify root %q: %w", root.Path, err)
		}
		rootIDs = append(rootIDs, rootID)

		err = filepath.WalkDir(root.Path, func(path string, dirEntry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if path == root.Path {
				return nil
			}
			if len(entries) >= o.limits.MaxEntries {
				return errors.New("entry limit exceeded")
			}

			info, err := os.Lstat(path)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(root.Path, path)
			if err != nil {
				return err
			}
			identity, err := objectIdentity(path, info)
			if err != nil {
				return err
			}
			totalBytes += info.Size()
			if totalBytes > o.limits.MaxBytes {
				return errors.New("byte limit exceeded")
			}
			kind := "file"
			if info.IsDir() {
				kind = "directory"
			} else if info.Mode()&os.ModeSymlink != 0 {
				kind = "symlink"
			}
			key := entryKey(rootIndex, relative)
			entries[key] = Entry{
				RootIndex: rootIndex,
				Relative:  relative,
				Path:      path,
				Kind:      kind,
				Identity:  identity,
				Size:      info.Size(),
				ModTime:   info.ModTime().UTC(),
				Mode:      uint32(info.Mode()),
				Symlink:   info.Mode()&os.ModeSymlink != 0,
			}
			return nil
		})
		if err != nil {
			return nil, nil, fmt.Errorf("observe root %q: %w", root.Path, err)
		}
	}
	return entries, rootIDs, nil
}

func entryKey(rootIndex int, relative string) string {
	return fmt.Sprintf("%d:%s", rootIndex, filepath.Clean(relative))
}

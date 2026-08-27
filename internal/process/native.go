package process

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	psutilprocess "github.com/shirou/gopsutil/v4/process"
)

type nativeAdapter struct{}

func (nativeAdapter) Snapshot(ctx context.Context) ([]Metadata, error) {
	all, err := psutilprocess.ProcessesWithContext(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Metadata, 0, len(all))
	for _, candidate := range all {
		owned, err := processOwnedByCurrentUser(ctx, candidate)
		if err != nil || !owned {
			continue
		}
		createdMillis, err := candidate.CreateTimeWithContext(ctx)
		if err != nil || createdMillis <= 0 {
			continue
		}
		parent, err := candidate.PpidWithContext(ctx)
		if err != nil {
			continue
		}
		workingDir, _ := candidate.CwdWithContext(ctx)
		result = append(result, Metadata{
			Identity:  Identity{PID: int(candidate.Pid), CreatedAt: time.UnixMilli(createdMillis).UTC()},
			ParentPID: int(parent), WorkingDir: workingDir,
		})
	}
	return result, nil
}

func (nativeAdapter) ListeningPorts(ctx context.Context, identity Identity) ([]Port, error) {
	return nativeListeningPorts(ctx, identity)
}

func (nativeAdapter) HoldsAnyPath(ctx context.Context, identity Identity, paths []string) (bool, error) {
	if len(paths) == 0 {
		return false, nil
	}
	process, err := psutilprocess.NewProcessWithContext(ctx, int32(identity.PID))
	if err != nil {
		return false, err
	}
	createdMillis, err := process.CreateTimeWithContext(ctx)
	if err != nil || !time.UnixMilli(createdMillis).UTC().Equal(identity.CreatedAt) {
		return false, errors.New("process identity changed")
	}
	return nativeHoldsAnyPath(ctx, process, cleanPathSet(paths))
}

func cleanPathSet(paths []string) map[string]struct{} {
	result := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		result[filepath.Clean(path)] = struct{}{}
	}
	return result
}

func processStillMatches(ctx context.Context, identity Identity) error {
	process, err := psutilprocess.NewProcessWithContext(ctx, int32(identity.PID))
	if err != nil {
		return err
	}
	createdMillis, err := process.CreateTimeWithContext(ctx)
	if err != nil {
		return err
	}
	if !time.UnixMilli(createdMillis).UTC().Equal(identity.CreatedAt) {
		return errors.New("process identity changed")
	}
	return nil
}

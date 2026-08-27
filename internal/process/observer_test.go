package process

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeNative struct {
	processes []Metadata
	ports     map[int][]Port
	holds     map[int]bool
	err       error
}

func (f fakeNative) Snapshot(context.Context) ([]Metadata, error) {
	return append([]Metadata(nil), f.processes...), f.err
}

func (f fakeNative) ListeningPorts(_ context.Context, identity Identity) ([]Port, error) {
	return append([]Port(nil), f.ports[identity.PID]...), nil
}

func (f fakeNative) HoldsAnyPath(_ context.Context, identity Identity, _ []string) (bool, error) {
	return f.holds[identity.PID], nil
}

func TestAttributionUsesStableIdentityAndExcludesUnrelatedProcess(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	native := fakeNative{
		processes: []Metadata{
			{Identity: Identity{PID: 10, CreatedAt: now}, ParentPID: 1, WorkingDir: root},
			{Identity: Identity{PID: 11, CreatedAt: now.Add(time.Second)}, ParentPID: 10, WorkingDir: filepath.Dir(root)},
			{Identity: Identity{PID: 12, CreatedAt: now}, ParentPID: 1, WorkingDir: filepath.Dir(root)},
		},
		ports: map[int][]Port{10: {{Protocol: "tcp", Address: "127.0.0.1", Number: 43210}}},
	}
	observer := NewObserver([]string{root}, native)
	got, limitations := observer.Resolve(context.Background(), Hints{
		EventProcesses: []Identity{{PID: 10, CreatedAt: now.Add(-time.Second)}},
	})
	if len(limitations) != 0 {
		t.Fatalf("limitations=%#v", limitations)
	}
	if len(got) != 2 || got[0].Identity.PID != 10 || got[1].Identity.PID != 11 {
		t.Fatalf("evidence=%#v", got)
	}
	if got[0].Reason != AttributionWorkingDirectory || len(got[0].Ports) != 1 {
		t.Fatalf("unexpected root process: %#v", got[0])
	}
	if got[1].Reason != AttributionDescendant {
		t.Fatalf("unexpected child: %#v", got[1])
	}
	for _, evidence := range got {
		if evidence.Identity.PID == 12 {
			t.Fatal("unrelated process included")
		}
	}
}

func TestReceiptIdentityAndCandidatePathAttribution(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC()
	native := fakeNative{
		processes: []Metadata{
			{Identity: Identity{PID: 20, CreatedAt: now}, ParentPID: 1},
			{Identity: Identity{PID: 21, CreatedAt: now}, ParentPID: 1},
		},
		holds: map[int]bool{21: true},
	}
	got, limitations := NewObserver([]string{root}, native).Resolve(context.Background(), Hints{
		ReceiptProcesses: []Identity{{PID: 20, CreatedAt: now}},
		CandidatePaths:   []string{filepath.Join(root, "artifact")},
	})
	if len(limitations) != 0 || len(got) != 2 {
		t.Fatalf("got=%#v limitations=%#v", got, limitations)
	}
	if got[0].Reason != AttributionReceipt || got[1].Reason != AttributionOpenPath {
		t.Fatalf("wrong reasons: %#v", got)
	}
}

func TestExitedProcessAndPermissionFailureBecomeLimitations(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC()
	observer := NewObserver([]string{root}, fakeNative{err: errors.New("permission denied")})
	got, limitations := observer.Resolve(context.Background(), Hints{EventProcesses: []Identity{{PID: 99, CreatedAt: now}}})
	if len(got) != 0 || len(limitations) != 1 || !strings.Contains(limitations[0].Detail, "permission denied") {
		t.Fatalf("got=%#v limitations=%#v", got, limitations)
	}
}

func TestBaselineContainsIdentityWithoutCommandLines(t *testing.T) {
	now := time.Now().UTC()
	observer := NewObserver([]string{t.TempDir()}, fakeNative{processes: []Metadata{{Identity: Identity{PID: 30, CreatedAt: now}, ParentPID: 1}}})
	baseline, err := observer.Baseline(context.Background())
	if err != nil || len(baseline.Processes) != 1 {
		t.Fatalf("baseline=%#v err=%v", baseline, err)
	}
	if strings.Contains(baseline.String(), "command") || strings.Contains(baseline.String(), "environment") {
		t.Fatalf("unsafe metadata in baseline: %s", baseline.String())
	}
}

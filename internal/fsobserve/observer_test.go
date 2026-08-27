package fsobserve

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/contract"
	"github.com/fantasyce/agent-residue-evidence/internal/scope"
)

func validatedScope(t *testing.T, root string) scope.Validated {
	t.Helper()
	got, err := scope.NewGuard().Validate(contract.TaskScope{TaskID: "task-fs", Workspace: root})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestCompareReportsNewObjectsWithoutReadingContents(t *testing.T) {
	root := t.TempDir()
	observer := NewObserver(Limits{MaxEntries: 1000, MaxDuration: time.Second})
	baseline, err := observer.Capture(context.Background(), validatedScope(t, root))
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "test-output")
	if err := os.Mkdir(output, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "secret.txt"), []byte("never-copy-me"), 0o600); err != nil {
		t.Fatal(err)
	}
	diff, err := observer.Compare(context.Background(), baseline)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Candidates) != 2 {
		t.Fatalf("candidates=%d %#v", len(diff.Candidates), diff.Candidates)
	}
	if strings.Contains(diff.String(), "never-copy-me") {
		t.Fatal("file content leaked into diff")
	}
	for _, candidate := range diff.Candidates {
		if candidate.EvidenceLevel != contract.EvidenceBaselineObserved || candidate.Recommendation != "review" {
			t.Fatalf("unexpected candidate: %#v", candidate)
		}
	}
}

func TestCompareDoesNotFollowSymlinkOutsideRoot(t *testing.T) {
	root := t.TempDir()
	external := t.TempDir()
	link := filepath.Join(root, "external-link")
	if err := os.Symlink(external, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	observer := NewObserver(Limits{MaxEntries: 1000, MaxDuration: time.Second})
	baseline, err := observer.Capture(context.Background(), validatedScope(t, root))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(external, "outside.txt"), []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	diff, err := observer.Compare(context.Background(), baseline)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Candidates) != 0 {
		t.Fatalf("outside change leaked into diff: %#v", diff.Candidates)
	}
}

func TestCaptureEnforcesEntryLimit(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a", "b"} {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	observer := NewObserver(Limits{MaxEntries: 1, MaxDuration: time.Second})
	_, err := observer.Capture(context.Background(), validatedScope(t, root))
	if err == nil || !strings.Contains(err.Error(), "entry limit") {
		t.Fatalf("expected entry limit error, got %v", err)
	}
}

func TestCompareDetectsRootIdentityReplacement(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	observer := NewObserver(Limits{MaxEntries: 100, MaxDuration: time.Second})
	baseline, err := observer.Capture(context.Background(), validatedScope(t, root))
	if err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(base, "old")
	if err := os.Rename(root, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err = observer.Compare(context.Background(), baseline)
	if err == nil || !strings.Contains(err.Error(), "root identity changed") {
		t.Fatalf("expected root identity error, got %v", err)
	}
}

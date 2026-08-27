package fsobserve

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fantasyce/agent-residue-evidence/internal/contract"
)

func TestCompareMarksChangedAndRemovedBaselineEntries(t *testing.T) {
	root := t.TempDir()
	changed := filepath.Join(root, "changed.log")
	removed := filepath.Join(root, "removed.log")
	if err := os.WriteFile(changed, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(removed, []byte("remove"), 0o600); err != nil {
		t.Fatal(err)
	}
	observer := NewObserver(Limits{MaxEntries: 100, MaxDuration: time.Second})
	baseline, err := observer.Capture(context.Background(), validatedScope(t, root))
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(changed, []byte("two-two"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(removed); err != nil {
		t.Fatal(err)
	}
	diff, err := observer.Compare(context.Background(), baseline)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Candidates) != 1 || diff.Candidates[0].CurrentStatus != contract.StatusChangedSinceReport {
		t.Fatalf("unexpected candidates: %#v", diff.Candidates)
	}
	if len(diff.Removed) != 1 || diff.Removed[0] != "removed.log" {
		t.Fatalf("unexpected removed: %#v", diff.Removed)
	}
}

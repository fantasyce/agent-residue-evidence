package scope

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/fantasyce/agent-residue-evidence/internal/contract"
)

func TestGuardRejectsHomeDirectory(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewGuard().Validate(contract.TaskScope{TaskID: "task-1", Workspace: home})
	if !errors.Is(err, ErrScopeTooBroad) {
		t.Fatalf("expected ErrScopeTooBroad, got %v", err)
	}
}

func TestGuardRejectsFilesystemRoot(t *testing.T) {
	root := string(filepath.Separator)
	if runtime.GOOS == "windows" {
		root = filepath.VolumeName(os.TempDir()) + string(filepath.Separator)
	}
	_, err := NewGuard().Validate(contract.TaskScope{TaskID: "task-1", Workspace: root})
	if !errors.Is(err, ErrScopeTooBroad) {
		t.Fatalf("expected ErrScopeTooBroad, got %v", err)
	}
}

func TestGuardAcceptsWorkspaceAndTaskTempRoot(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "repo")
	tempRoot := filepath.Join(base, "task-temp")
	for _, path := range []string{workspace, tempRoot} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	validated, err := NewGuard().Validate(contract.TaskScope{
		TaskID:    "task-1",
		Workspace: workspace,
		TempRoots: []string{tempRoot},
	})
	if err != nil {
		t.Fatalf("valid scope rejected: %v", err)
	}
	if len(validated.Roots) != 2 || validated.TaskID != "task-1" {
		t.Fatalf("unexpected validated scope: %#v", validated)
	}
}

func TestGuardRejectsSymlinkRoot(t *testing.T) {
	base := t.TempDir()
	realRoot := filepath.Join(base, "real")
	if err := os.Mkdir(realRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	linkRoot := filepath.Join(base, "link")
	if err := os.Symlink(realRoot, linkRoot); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	_, err := NewGuard().Validate(contract.TaskScope{TaskID: "task-1", Workspace: linkRoot})
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("expected ErrUnsafePath, got %v", err)
	}
}

func TestGuardRejectsNestedDuplicateRoots(t *testing.T) {
	workspace := t.TempDir()
	nested := filepath.Join(workspace, "tmp")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := NewGuard().Validate(contract.TaskScope{
		TaskID:    "task-1",
		Workspace: workspace,
		TempRoots: []string{nested},
	})
	if !errors.Is(err, ErrOverlappingRoots) {
		t.Fatalf("expected ErrOverlappingRoots, got %v", err)
	}
}

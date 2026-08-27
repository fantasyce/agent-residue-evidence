package pathalias

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fantasyce/agent-residue-evidence/internal/scope"
)

func TestProjectAndResolveUseBoundAliases(t *testing.T) {
	workspace := t.TempDir()
	tempRoot := t.TempDir()
	table, err := New(scope.Validated{TaskID: "display-only", Roots: []scope.Root{
		{Path: workspace, Identity: rootIdentity(t, workspace)},
		{Path: tempRoot, Identity: rootIdentity(t, tempRoot)},
	}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(workspace, "build", "result.log")
	alias, err := table.Project(path)
	if err != nil {
		t.Fatal(err)
	}
	if alias != "workspace://build/result.log" {
		t.Fatalf("alias=%q", alias)
	}
	resolved, err := table.Resolve(alias)
	if err != nil || resolved != path {
		t.Fatalf("resolved=%q err=%v", resolved, err)
	}
	tempAlias, err := table.Project(filepath.Join(tempRoot, "pytest", "cache"))
	if err != nil || tempAlias != "temp://0/pytest/cache" {
		t.Fatalf("alias=%q err=%v", tempAlias, err)
	}
}

func TestResolveRejectsTraversalAndRootReplacement(t *testing.T) {
	workspace := t.TempDir()
	table, err := New(scope.Validated{Roots: []scope.Root{{Path: workspace, Identity: rootIdentity(t, workspace)}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := table.Resolve("workspace://../secret"); err == nil {
		t.Fatal("alias traversal accepted")
	}
	moved := workspace + "-moved"
	if err := os.Rename(workspace, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := table.Resolve("workspace://result.log"); err == nil {
		t.Fatal("replacement root accepted")
	}
}

func rootIdentity(t *testing.T, path string) string {
	t.Helper()
	identity, err := stableIdentity(path)
	if err != nil {
		t.Fatal(err)
	}
	return identity
}

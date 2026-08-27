package versioninfo

import "testing"

func TestStringIncludesVersionAndCommit(t *testing.T) {
	oldVersion, oldCommit := Version, Commit
	Version, Commit = "1.2.3", "abc123"
	t.Cleanup(func() { Version, Commit = oldVersion, oldCommit })
	if got, want := String(), "agent-residue-evidence 1.2.3 (abc123)"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

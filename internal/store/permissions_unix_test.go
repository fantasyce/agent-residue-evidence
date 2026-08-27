//go:build !windows

package store

import (
	"os"
	"testing"
)

func assertPrivatePath(t *testing.T, path string, directory bool) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	want := os.FileMode(0o600)
	if directory {
		want = 0o700
	}
	if info.Mode().Perm() != want {
		t.Fatalf("path mode=%v want=%v", info.Mode().Perm(), want)
	}
}

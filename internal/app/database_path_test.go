package app

import (
	"path/filepath"
	"testing"
)

func TestHomeDatabasePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	path, err := homeDBPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".sshm", DbDefaultName)
	if path != want {
		t.Fatalf("homeDBPath() = %q, want %q", path, want)
	}
}

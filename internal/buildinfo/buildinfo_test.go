package buildinfo

import "testing"

func TestInfoUsesDefaults(t *testing.T) {
	originalVersion := Version
	Version = ""
	t.Cleanup(func() {
		Version = originalVersion
	})

	info := Info()
	if info.Version != "dev" || info.Author != "nullecho" {
		t.Fatalf("Info() = %#v, want default metadata", info)
	}
}

func TestInfoTrimsVersion(t *testing.T) {
	originalVersion := Version
	Version = "  v0.1.1  "
	t.Cleanup(func() {
		Version = originalVersion
	})

	info := Info()
	if info.Version != "v0.1.1" {
		t.Fatalf("Version = %q, want trimmed version", info.Version)
	}
}

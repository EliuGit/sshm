package app

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDatabasePathRejectsMissingEnvironmentPath(t *testing.T) {
	t.Setenv(DbPathEnvVarKey, filepath.Join(t.TempDir(), "missing"))
	_, err := dbPath()
	if err == nil || !strings.Contains(err.Error(), DbPathEnvVarKey) {
		t.Fatalf("dbPath() error = %v", err)
	}
}

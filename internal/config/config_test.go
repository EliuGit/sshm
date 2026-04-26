package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCreatesDefaultConfigAndResolvesPaths(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", baseDir)
	t.Setenv("APPDATA", baseDir)

	runtimeConfig, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if runtimeConfig.Language != "en" {
		t.Fatalf("Language = %q, want %q", runtimeConfig.Language, "en")
	}
	if got, want := runtimeConfig.DatabasePath, filepath.Join(baseDir, "sshm", "data", "sshm.db"); got != want {
		t.Fatalf("DatabasePath = %q, want %q", got, want)
	}
	if _, err := os.Stat(runtimeConfig.ConfigPath); err != nil {
		t.Fatalf("config file stat error = %v", err)
	}
}

func TestLoadParsesCustomConfig(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", baseDir)
	t.Setenv("APPDATA", baseDir)

	configDir := filepath.Join(baseDir, "sshm")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := "[app]\nlanguage = \"zh-CN\"\n\n[storage]\ndatabase_path = \"custom/data.db\"\n\n[ssh]\ndefault_private_key_path = \"~/.ssh/id_ed25519\"\n"
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	runtimeConfig, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if runtimeConfig.Language != "zh-CN" {
		t.Fatalf("Language = %q, want %q", runtimeConfig.Language, "zh-CN")
	}
	if got, want := runtimeConfig.DatabasePath, filepath.Join(configDir, "custom", "data.db"); got != want {
		t.Fatalf("DatabasePath = %q, want %q", got, want)
	}
	if got, want := runtimeConfig.DefaultPrivateKeyPath, "~/.ssh/id_ed25519"; got != want {
		t.Fatalf("DefaultPrivateKeyPath = %q, want %q", got, want)
	}
}

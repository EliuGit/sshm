package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	appDirName            = "sshm"
	configFileName        = "config.toml"
	defaultLanguage       = "en"
	defaultTheme          = "dark"
	defaultDatabasePath   = "data/sshm.db"
	defaultPrivateKeyPath = "~/.ssh/id_rsa"
)

type FileConfig struct {
	App     AppConfig
	Storage StorageConfig
	SSH     SSHConfig
}

type AppConfig struct {
	Language string
	Theme    string
}

type StorageConfig struct {
	DatabasePath string
}

type SSHConfig struct {
	DefaultPrivateKeyPath string
}

type RuntimeConfig struct {
	ConfigDir             string
	ConfigPath            string
	Language              string
	Theme                 string
	DatabasePath          string
	KeyPath               string
	KnownHostsPath        string
	DefaultPrivateKeyPath string
}

func Load() (RuntimeConfig, error) {
	configDir, err := defaultConfigDir()
	if err != nil {
		return RuntimeConfig{}, err
	}
	configPath := filepath.Join(configDir, configFileName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return RuntimeConfig{}, err
	}
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := writeDefaultConfig(configPath); err != nil {
			return RuntimeConfig{}, err
		}
	} else if err != nil {
		return RuntimeConfig{}, err
	}

	fileConfig, err := readConfig(configPath)
	if err != nil {
		return RuntimeConfig{}, err
	}
	applyDefaults(&fileConfig)
	if err := validate(fileConfig); err != nil {
		return RuntimeConfig{}, err
	}

	return RuntimeConfig{
		ConfigDir:             configDir,
		ConfigPath:            configPath,
		Language:              fileConfig.App.Language,
		Theme:                 fileConfig.App.Theme,
		DatabasePath:          resolveConfigPath(configDir, fileConfig.Storage.DatabasePath),
		KeyPath:               filepath.Join(configDir, "app.key"),
		KnownHostsPath:        filepath.Join(configDir, "known_hosts"),
		DefaultPrivateKeyPath: fileConfig.SSH.DefaultPrivateKeyPath,
	}, nil
}

func EnsurePaths(runtime RuntimeConfig) error {
	dirs := []string{
		runtime.ConfigDir,
		filepath.Dir(runtime.DatabasePath),
		filepath.Dir(runtime.KeyPath),
		filepath.Dir(runtime.KnownHostsPath),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	return nil
}

func defaultConfigDir() (string, error) {
	// 优先尊重显式环境变量，方便 CI、测试和便携部署稳定覆盖默认配置目录。
	if baseDir := strings.TrimSpace(os.Getenv("APPDATA")); baseDir != "" {
		return filepath.Join(baseDir, appDirName), nil
	}
	if baseDir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); baseDir != "" {
		return filepath.Join(baseDir, appDirName), nil
	}
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baseDir, appDirName), nil
}

func writeDefaultConfig(path string) error {
	config := defaultFileConfig()
	content := strings.Join([]string{
		"[app]",
		"language = " + strconv.Quote(config.App.Language),
		"theme = " + strconv.Quote(config.App.Theme),
		"",
		"[storage]",
		"database_path = " + strconv.Quote(config.Storage.DatabasePath),
		"",
		"[ssh]",
		"default_private_key_path = " + strconv.Quote(config.SSH.DefaultPrivateKeyPath),
		"",
	}, "\n")
	return os.WriteFile(path, []byte(content), 0600)
}

func readConfig(path string) (FileConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return FileConfig{}, err
	}
	defer file.Close()

	config := defaultFileConfig()
	section := ""
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := stripComment(strings.TrimSpace(scanner.Text()))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return FileConfig{}, fmt.Errorf("invalid config line %d", lineNo)
		}
		parsedValue, err := parseStringValue(strings.TrimSpace(value))
		if err != nil {
			return FileConfig{}, fmt.Errorf("invalid config line %d: %w", lineNo, err)
		}
		switch section + "." + strings.TrimSpace(key) {
		case "app.language":
			config.App.Language = parsedValue
		case "app.theme":
			config.App.Theme = parsedValue
		case "storage.database_path":
			config.Storage.DatabasePath = parsedValue
		case "ssh.default_private_key_path":
			config.SSH.DefaultPrivateKeyPath = parsedValue
		}
	}
	if err := scanner.Err(); err != nil {
		return FileConfig{}, err
	}
	return config, nil
}

func defaultFileConfig() FileConfig {
	return FileConfig{
		App:     AppConfig{Language: defaultLanguage, Theme: defaultTheme},
		Storage: StorageConfig{DatabasePath: defaultDatabasePath},
		SSH:     SSHConfig{DefaultPrivateKeyPath: defaultPrivateKeyPath},
	}
}

func applyDefaults(config *FileConfig) {
	if strings.TrimSpace(config.App.Language) == "" {
		config.App.Language = defaultLanguage
	}
	config.App.Theme = normalizeThemeName(config.App.Theme)
	if config.App.Theme == "" {
		config.App.Theme = defaultTheme
	}
	if strings.TrimSpace(config.Storage.DatabasePath) == "" {
		config.Storage.DatabasePath = defaultDatabasePath
	}
	if strings.TrimSpace(config.SSH.DefaultPrivateKeyPath) == "" {
		config.SSH.DefaultPrivateKeyPath = defaultPrivateKeyPath
	}
}

func validate(config FileConfig) error {
	switch config.App.Language {
	case "en", "zh-CN":
	default:
		return fmt.Errorf("unsupported language %q", config.App.Language)
	}
	if strings.TrimSpace(config.Storage.DatabasePath) == "" {
		return fmt.Errorf("database path is required")
	}
	return nil
}

func resolveConfigPath(configDir string, value string) string {
	value = strings.TrimSpace(value)
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Clean(filepath.Join(configDir, value))
}

func stripComment(line string) string {
	inString := false
	escaped := false
	for index, char := range line {
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' && inString {
			escaped = true
			continue
		}
		if char == '"' {
			inString = !inString
			continue
		}
		if char == '#' && !inString {
			return strings.TrimSpace(line[:index])
		}
	}
	return line
}

func normalizeThemeName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func parseStringValue(value string) (string, error) {
	if strings.HasPrefix(value, "\"") {
		return strconv.Unquote(value)
	}
	return strings.TrimSpace(value), nil
}

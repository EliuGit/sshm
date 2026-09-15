package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"sshm/internal/i18n"
)

const (
	// DbPathEnvVarKey 是应用数据库路径环境变量。
	DbPathEnvVarKey = "SSHM_DB_PATH"
	// DbDefaultName 是应用数据库默认名称。
	DbDefaultName = "sshm.db"
)

// dbPath 返回本次运行使用的数据库路径。
// 显式配置的路径不可用时返回错误；未配置时查找已有数据库，否则返回用户配置目录中的默认路径。
func dbPath() (string, error) {
	if raw := os.Getenv(DbPathEnvVarKey); raw != "" {
		info, err := os.Stat(raw)
		if err != nil {
			return "", fmt.Errorf("%s: %w", i18n.T("SSHM_DB_PATH is unavailable"), err)
		}
		if info.Mode().IsRegular() {
			return raw, nil
		}
		if info.IsDir() {
			return filepath.Join(raw, DbDefaultName), nil
		}
		return "", errors.New(i18n.T("SSHM_DB_PATH is not a file or directory: %s", raw))
	}

	if path := findDB(); path != "" {
		return path, nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "sshm", DbDefaultName), nil
}

// findDB 按可执行文件目录、用户配置目录的顺序查找已有数据库。
func findDB() string {
	if executablePath, err := os.Executable(); err == nil {
		if path := existingFile(filepath.Join(filepath.Dir(executablePath), DbDefaultName)); path != "" {
			return path
		}
	}
	if configDir, err := os.UserConfigDir(); err == nil {
		return existingFile(filepath.Join(configDir, "sshm", DbDefaultName))
	}
	return ""
}

func existingFile(path string) string {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return path
}

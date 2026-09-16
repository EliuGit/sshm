package app

import (
	"os"
	"path/filepath"
)

// DbDefaultName 是应用数据库默认名称。
const DbDefaultName = "sshm.db"

// dbPath 返回本次运行使用的数据库路径。
// 按可执行文件目录、用户家目录查找，均不存在时使用用户家目录中的默认路径。
func dbPath() (string, error) {
	if path := findDB(); path != "" {
		return path, nil
	}
	return homeDBPath()
}

// findDB 按可执行文件目录、用户家目录的顺序查找已有数据库。
func findDB() string {
	if executablePath, err := os.Executable(); err == nil {
		if path := existingFile(filepath.Join(filepath.Dir(executablePath), DbDefaultName)); path != "" {
			return path
		}
	}
	if path, err := homeDBPath(); err == nil {
		return existingFile(path)
	}
	return ""
}

func homeDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".sshm", DbDefaultName), nil
}

func existingFile(path string) string {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return path
}

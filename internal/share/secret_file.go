package share

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// WriteSecretFile 将敏感数据通过同目录临时文件写入，并以 0600 权限替换目标文件。
// 调用方负责创建目标目录，且不得并发写入同一路径。
func WriteSecretFile(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".secret-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err = tmp.Chmod(0600); err == nil {
		_, err = tmp.Write(data)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(tmpPath, path); err == nil {
		return nil
	}
	if runtime.GOOS != "windows" {
		return err
	}

	// Windows 无法直接覆盖已有文件，先保留旧文件以便替换失败时恢复。
	backupPath := tmpPath + ".bak"
	if backupErr := os.Rename(path, backupPath); backupErr != nil {
		return errors.Join(err, fmt.Errorf("备份原文件: %w", backupErr))
	}
	defer os.Remove(backupPath)
	if replaceErr := os.Rename(tmpPath, path); replaceErr != nil {
		if restoreErr := os.Rename(backupPath, path); restoreErr != nil {
			return errors.Join(replaceErr, fmt.Errorf("恢复原文件: %w", restoreErr))
		}
		return replaceErr
	}
	return nil
}

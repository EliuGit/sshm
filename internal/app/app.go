package app

import (
	"errors"
	"fmt"

	"sshm/internal/repository"
	"sshm/internal/tui"
)

// Run 完成应用初始化并启动终端界面。
func Run() error {
	path, err := dbPath()
	if err != nil {
		return fmt.Errorf("初始化应用失败: %w", err)
	}
	status, err := repository.Inspect(path)
	if err != nil {
		return fmt.Errorf("初始化应用失败: %w", err)
	}
	if err := tui.RunApplication(path, status); err != nil {
		if errors.Is(err, tui.ErrInitializationCanceled) {
			return nil
		}
		return fmt.Errorf("启动应用失败: %w", err)
	}
	return nil
}

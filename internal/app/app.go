package app

import (
	"errors"
	"fmt"
	"os"

	"sshm/internal/i18n"
	"sshm/internal/repository"
	"sshm/internal/tui"
)

const (
	// 用户记住密码的环境变量
	UserSavePassEnvVarKey = "SSHM_SAVE_PASS"
)

// Run 完成应用初始化并启动终端界面。
func Run() error {
	i18n.Init()
	path, err := dbPath()
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("Failed to initialize SSHM"), err)
	}
	status, err := repository.Inspect(path)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("Failed to initialize SSHM"), err)
	}
	if err := tui.RunApplication(path, status, os.Getenv(UserSavePassEnvVarKey)); err != nil {
		if errors.Is(err, tui.ErrInitializationCanceled) {
			return nil
		}
		return fmt.Errorf("%s: %w", i18n.T("Failed to start SSHM"), err)
	}
	return nil
}

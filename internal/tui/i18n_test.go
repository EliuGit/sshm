package tui

import (
	"os"
	"testing"
)

// TestMain 固定现有界面快照使用中文，避免测试结果受运行机器语言影响。
func TestMain(m *testing.M) {
	_ = os.Setenv("SSHM_LANG", "zh")
	os.Exit(m.Run())
}

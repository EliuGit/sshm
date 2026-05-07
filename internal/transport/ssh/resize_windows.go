//go:build windows

package ssh

import (
	"sync"
	"time"

	"github.com/charmbracelet/x/term"
)

const windowsResizePollInterval = 200 * time.Millisecond

func watchWindowChanges(fd uintptr, onResize func(cols, rows int) error) func() {
	// 设计说明：
	// 1. Windows 控制台没有 Unix 风格的 SIGWINCH，无法像类 Unix 系统一样被动接收窗口变化信号。
	// 2. SSH PTY 的初始尺寸已在 RequestPty 阶段发送，这里只负责在本地 terminal 宽高变化后增量同步到远端 shell。
	// 3. 通过低频轮询 stdout 对应的终端尺寸，并仅在宽高真实变化时调用 WindowChange，避免无效网络写入。
	// 4. stop 回调负责关闭后台 goroutine，防止交互 shell 退出后继续占用资源。
	cols, rows, err := term.GetSize(fd)
	last, _ := detectTerminalSizeChange(terminalSize{}, cols, rows, err)

	done := make(chan struct{})
	ticker := time.NewTicker(windowsResizePollInterval)
	var stopOnce sync.Once

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				cols, rows, err := term.GetSize(fd)
				current, changed := detectTerminalSizeChange(last, cols, rows, err)
				if !changed {
					continue
				}
				last = current
				_ = onResize(current.cols, current.rows)
			case <-done:
				return
			}
		}
	}()

	return func() {
		stopOnce.Do(func() {
			close(done)
		})
	}
}

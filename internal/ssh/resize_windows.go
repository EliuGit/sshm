//go:build windows

package ssh

import (
	"sync"
	"time"

	"github.com/charmbracelet/x/term"
)

const resizePollInterval = 200 * time.Millisecond

// watchWindowChanges 在 Windows 上轮询窗口尺寸，并仅把真实变化同步到远端。
func watchWindowChanges(fd uintptr, onResize func(cols, rows int) error) func() {
	cols, rows, err := term.GetSize(fd)
	last, _ := terminalChanged(terminalSize{}, cols, rows, err)
	done := make(chan struct{})
	ticker := time.NewTicker(resizePollInterval)
	var stopOnce sync.Once
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				cols, rows, err := term.GetSize(fd)
				current, changed := terminalChanged(last, cols, rows, err)
				if changed {
					last = current
					_ = onResize(current.cols, current.rows)
				}
			case <-done:
				return
			}
		}
	}()
	return func() { stopOnce.Do(func() { close(done) }) }
}

//go:build !windows

package ssh

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/charmbracelet/x/term"
)

// watchWindowChanges 监听 SIGWINCH，并把本地窗口尺寸同步到远端。
func watchWindowChanges(fd uintptr, onResize func(cols, rows int) error) func() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGWINCH)
	done := make(chan struct{})
	var stopOnce sync.Once
	go func() {
		for {
			select {
			case <-signals:
				cols, rows, err := term.GetSize(fd)
				if err == nil {
					_ = onResize(cols, rows)
				}
			case <-done:
				return
			}
		}
	}()
	return func() {
		stopOnce.Do(func() {
			signal.Stop(signals)
			close(done)
		})
	}
}

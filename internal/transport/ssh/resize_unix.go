//go:build !windows

package ssh

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/x/term"
)

func watchWindowChanges(fd uintptr, onResize func(cols, rows int) error) func() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGWINCH)

	done := make(chan struct{})
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
		signal.Stop(signals)
		close(done)
	}
}

package ssh

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/muesli/cancelreader"
)

const inputStopTimeout = 1200 * time.Millisecond

// inputForwarder 管理本地 stdin 读取协程的完整生命周期。
// Shell 退出时必须先调用 Stop，之后 Bubble Tea 才能重新接管输入。
type inputForwarder struct {
	reader cancelreader.CancelReader
	writer io.WriteCloser
	done   chan error

	started   atomic.Bool
	startOnce sync.Once
	stopOnce  sync.Once
	stopErr   error
}

func newInputForwarder(input io.Reader, writer io.WriteCloser) (*inputForwarder, error) {
	if input == nil || writer == nil {
		return nil, errors.New("交互式输入或远端输入不可用")
	}
	reader, err := cancelreader.NewReader(input)
	if err != nil {
		return nil, fmt.Errorf("创建可取消的终端输入: %w", err)
	}
	return &inputForwarder{reader: reader, writer: writer, done: make(chan error, 1)}, nil
}

// Start 启动唯一的输入转发协程。
func (f *inputForwarder) Start() {
	f.startOnce.Do(func() {
		f.started.Store(true)
		go func() {
			_, copyErr := io.Copy(f.writer, f.reader)
			if errors.Is(copyErr, cancelreader.ErrCanceled) || errors.Is(copyErr, io.EOF) {
				copyErr = nil
			}
			closeErr := f.writer.Close()
			if copyErr == nil && !errors.Is(closeErr, io.EOF) {
				copyErr = closeErr
			}
			f.done <- copyErr
			close(f.done)
		}()
	})
}

// Stop 取消本地输入读取并等待转发协程退出，防止其与恢复后的 TUI 争抢按键。
func (f *inputForwarder) Stop() error {
	if f == nil {
		return nil
	}
	f.stopOnce.Do(func() {
		if !f.started.Load() {
			f.stopErr = errors.Join(f.writer.Close(), f.reader.Close())
			return
		}
		f.reader.Cancel()
		select {
		case err := <-f.done:
			f.stopErr = errors.Join(err, f.reader.Close())
		case <-time.After(inputStopTimeout):
			f.stopErr = errors.Join(errors.New("等待终端输入转发停止超时"), f.reader.Close())
		}
	})
	return f.stopErr
}

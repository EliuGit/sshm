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

const interactiveInputStopTimeout = 1200 * time.Millisecond

// interactiveInputForwarder 负责把本地交互输入转发到远端 shell 的 stdin pipe。
//
// 设计意图：
// 1. 避免直接把 os.Stdin 交给 x/crypto/ssh 的 Session.Stdin。
// 2. 主动掌控本地 stdin 读取 goroutine 的生命周期，确保 shell 退出时可以先停读，再恢复 TUI。
// 3. 将“输入转发”和“SSH session 生命周期”解耦，防止 x/crypto/ssh 内部残留读取吞掉返回列表后的首个按键。
//
// 数据流：
// local stdin -> cancelreader.CancelReader -> interactiveInputForwarder goroutine -> session.StdinPipe()
//
// 退出顺序：
// 1. shell wait 返回
// 2. Stop() 取消本地读取并等待 goroutine 退出
// 3. 远端 stdin pipe 关闭
// 4. 调用方再恢复终端 / Bubble Tea
//
// 注意事项：
// 1. 这里不做任何按键语义解析，只按字节原样转发，保持远端 shell 行为不变。
// 2. cancelreader 在不同平台的可取消能力由底层实现保证；Stop() 会等待 goroutine 退出，超时则返回错误，避免静默残留读取。
type interactiveInputForwarder struct {
	reader cancelreader.CancelReader
	writer io.WriteCloser

	done      chan error
	startOnce sync.Once
	stopOnce  sync.Once
	stopErr   error
	started   atomic.Bool
}

func newInteractiveInputForwarder(input io.Reader, writer io.WriteCloser) (*interactiveInputForwarder, error) {
	if input == nil {
		return nil, fmt.Errorf("interactive input reader is nil")
	}
	if writer == nil {
		return nil, fmt.Errorf("interactive input writer is nil")
	}

	reader, err := cancelreader.NewReader(input)
	if err != nil {
		return nil, fmt.Errorf("failed to create cancelable stdin reader: %w", err)
	}

	return &interactiveInputForwarder{
		reader: reader,
		writer: writer,
		done:   make(chan error, 1),
	}, nil
}

func (f *interactiveInputForwarder) Start() {
	f.startOnce.Do(func() {
		f.started.Store(true)
		go func() {
			defer close(f.done)
			_, copyErr := io.Copy(f.writer, f.reader)
			if errors.Is(copyErr, cancelreader.ErrCanceled) || errors.Is(copyErr, io.EOF) {
				copyErr = nil
			}

			closeErr := f.writer.Close()
			if copyErr == nil && closeErr != nil && !errors.Is(closeErr, io.EOF) {
				copyErr = closeErr
			}

			f.done <- copyErr
		}()
	})
}

func (f *interactiveInputForwarder) Stop() error {
	if f == nil {
		return nil
	}

	f.stopOnce.Do(func() {
		if !f.started.Load() {
			f.stopErr = errors.Join(f.writer.Close(), f.reader.Close())
			return
		}

		_ = f.reader.Cancel()
		select {
		case err := <-f.done:
			f.stopErr = errors.Join(err, f.reader.Close())
		case <-time.After(interactiveInputStopTimeout):
			closeErr := f.reader.Close()
			f.stopErr = errors.Join(fmt.Errorf("timed out waiting for interactive stdin forwarder to stop"), closeErr)
		}
	})

	return f.stopErr
}

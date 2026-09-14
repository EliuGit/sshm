package ssh

import (
	"io"
	"sync"
	"testing"

	"github.com/muesli/cancelreader"
)

// blockingCancelReader 模拟会一直等待终端输入、直到被主动取消的读取器。
type blockingCancelReader struct {
	done chan struct{}
	once sync.Once
}

func (r *blockingCancelReader) Read([]byte) (int, error) {
	<-r.done
	return 0, cancelreader.ErrCanceled
}

func (r *blockingCancelReader) Cancel() bool {
	r.once.Do(func() { close(r.done) })
	return true
}

func (r *blockingCancelReader) Close() error {
	r.once.Do(func() { close(r.done) })
	return nil
}

// closingWriter 记录远端 stdin 是否随转发协程一起关闭。
type closingWriter struct{ closed bool }

func (*closingWriter) Write(data []byte) (int, error) { return len(data), nil }
func (w *closingWriter) Close() error {
	w.closed = true
	return nil
}

func TestInteractiveInputForwarderStopsBeforeReturning(t *testing.T) {
	reader := &blockingCancelReader{done: make(chan struct{})}
	writer := &closingWriter{}
	forwarder := &inputForwarder{reader: reader, writer: writer, done: make(chan error, 1)}
	forwarder.Start()
	if err := forwarder.Stop(); err != nil {
		t.Fatal(err)
	}
	if !writer.closed {
		t.Fatal("输入转发停止后远端 stdin 仍未关闭")
	}
	if _, ok := <-forwarder.done; ok {
		t.Fatal("Stop 返回时输入转发协程仍未结束")
	}
}

func TestTerminalChanged(t *testing.T) {
	last := terminalSize{cols: 80, rows: 24}
	if got, changed := terminalChanged(last, 80, 24, nil); changed || got != last {
		t.Fatalf("相同尺寸被判为变化: %#v, %v", got, changed)
	}
	if got, changed := terminalChanged(last, 120, 30, nil); !changed || got != (terminalSize{cols: 120, rows: 30}) {
		t.Fatalf("新尺寸未被识别: %#v, %v", got, changed)
	}
	if got, changed := terminalChanged(last, 0, 0, io.EOF); changed || got != last {
		t.Fatalf("无效尺寸覆盖了旧值: %#v, %v", got, changed)
	}
}

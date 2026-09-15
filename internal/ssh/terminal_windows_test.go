//go:build windows

package ssh

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestUTF8ConsoleInputIsRestored(t *testing.T) {
	original, err := windows.GetConsoleCP()
	if err != nil {
		t.Skipf("当前进程没有 Windows 控制台: %v", err)
	}
	restore, err := useUTF8ConsoleInput()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)
	if current, err := windows.GetConsoleCP(); err != nil || current != utf8CodePage {
		t.Fatalf("控制台输入代码页未切换为 UTF-8: codePage=%d err=%v", current, err)
	}
	restore()
	if current, err := windows.GetConsoleCP(); err != nil || current != original {
		t.Fatalf("控制台输入代码页未恢复: codePage=%d want=%d err=%v", current, original, err)
	}
}

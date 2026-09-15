//go:build windows

package ssh

import (
	"github.com/charmbracelet/x/term"
	"golang.org/x/sys/windows"
)

const utf8CodePage = 65001

func resetInput(inputFd uintptr) error {
	return windows.FlushConsoleInputBuffer(windows.Handle(inputFd))
}

// prepareTerminal 保存终端状态，开启 raw、虚拟终端输出和延迟换行，并返回恢复函数。
func prepareTerminal(inputFd, outputFd uintptr) (func(), error) {
	restoreInputCodePage, err := useUTF8ConsoleInput()
	if err != nil {
		return nil, err
	}
	state, err := term.MakeRaw(inputFd)
	if err != nil {
		restoreInputCodePage()
		return nil, err
	}
	outputHandle := windows.Handle(outputFd)
	var outputMode uint32
	if err := windows.GetConsoleMode(outputHandle, &outputMode); err != nil {
		_ = term.Restore(inputFd, state)
		restoreInputCodePage()
		return nil, err
	}
	if err := windows.SetConsoleMode(outputHandle, outputMode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING|windows.DISABLE_NEWLINE_AUTO_RETURN); err != nil {
		_ = term.Restore(inputFd, state)
		restoreInputCodePage()
		return nil, err
	}
	return func() {
		_ = term.Restore(inputFd, state)
		restoreInputCodePage()
		_ = windows.SetConsoleMode(outputHandle, outputMode)
	}, nil
}

// useUTF8ConsoleInput 临时把 Windows 控制台输入切换为 UTF-8，并返回恢复函数。
// cancelreader 通过字节接口读取控制台，若保留本地代码页，中文会以非 UTF-8 字节发送到 Linux。
func useUTF8ConsoleInput() (func(), error) {
	codePage, err := windows.GetConsoleCP()
	if err != nil {
		return nil, err
	}
	if err := windows.SetConsoleCP(utf8CodePage); err != nil {
		return nil, err
	}
	return func() { _ = windows.SetConsoleCP(codePage) }, nil
}

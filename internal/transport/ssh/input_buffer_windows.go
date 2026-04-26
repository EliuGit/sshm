//go:build windows

package ssh

import "golang.org/x/sys/windows"

var (
	kernel32Input               = windows.NewLazySystemDLL("kernel32.dll")
	procFlushConsoleInputBuffer = kernel32Input.NewProc("FlushConsoleInputBuffer")
)

func resetInteractiveInput() error {
	inputHandle, err := windows.GetStdHandle(windows.STD_INPUT_HANDLE)
	if err != nil {
		return err
	}
	r1, _, callErr := procFlushConsoleInputBuffer.Call(uintptr(inputHandle))
	if r1 == 0 {
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return callErr
		}
		return windows.GetLastError()
	}
	return nil
}

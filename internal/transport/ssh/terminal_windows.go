//go:build windows

package ssh

import (
	"github.com/charmbracelet/x/term"
	"golang.org/x/sys/windows"
)

const enableVirtualTerminalProcessing = 0x0004

func prepareInteractiveTerminal(inputFd uintptr, outputFd uintptr) (func(), error) {
	state, err := term.MakeRaw(inputFd)
	if err != nil {
		return nil, err
	}

	outputHandle := windows.Handle(outputFd)
	var outputMode uint32
	outputModeChanged := false
	if err := windows.GetConsoleMode(outputHandle, &outputMode); err == nil {
		_ = windows.SetConsoleMode(outputHandle, outputMode|enableVirtualTerminalProcessing)
		outputModeChanged = true
	}

	return func() {
		_ = term.Restore(inputFd, state)
		if outputModeChanged {
			_ = windows.SetConsoleMode(outputHandle, outputMode)
		}
	}, nil
}

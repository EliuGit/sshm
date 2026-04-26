//go:build !windows

package ssh

import "github.com/charmbracelet/x/term"

func prepareInteractiveTerminal(inputFd uintptr, _ uintptr) (func(), error) {
	state, err := term.MakeRaw(inputFd)
	if err != nil {
		return nil, err
	}
	return func() {
		_ = term.Restore(inputFd, state)
	}, nil
}

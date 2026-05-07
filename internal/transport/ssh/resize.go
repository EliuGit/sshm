package ssh

type terminalSize struct {
	cols int
	rows int
}

func detectTerminalSizeChange(last terminalSize, cols int, rows int, err error) (terminalSize, bool) {
	if err != nil || cols <= 0 || rows <= 0 {
		return last, false
	}

	current := terminalSize{cols: cols, rows: rows}
	if current == last {
		return last, false
	}
	return current, true
}

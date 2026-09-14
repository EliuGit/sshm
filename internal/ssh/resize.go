package ssh

// terminalSize 保存最近一次已同步到远端的终端尺寸。
type terminalSize struct {
	cols int
	rows int
}

func terminalChanged(last terminalSize, cols, rows int, err error) (terminalSize, bool) {
	if err != nil || cols <= 0 || rows <= 0 {
		return last, false
	}
	current := terminalSize{cols: cols, rows: rows}
	return current, current != last
}

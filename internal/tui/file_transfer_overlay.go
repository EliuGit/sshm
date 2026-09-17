package tui

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// overlayKind 表示文件传输弹窗当前显示的内部覆盖层。
type overlayKind uint8

const (
	overlayNone overlayKind = iota
	overlayDelete
	overlayProgress
	overlayRename
	overlayCreate
	overlayHelp
)

// progressState 表示进度框当前所处的任务状态。
type progressState uint8

const (
	progressConnecting progressState = iota
	progressRunning
	progressCompleted
	progressCancelled
	progressFailed
)

// transferOverlay 保存删除、重命名或传输进度覆盖层的状态。
type transferOverlay struct {
	kind            overlayKind
	deletePaths     []string
	progress        progressState
	operation       string
	item            string
	itemIndex       int
	totalItems      int
	processedBytes  int64
	totalBytes      int64
	cancelRequested bool
	error           string
	renameOld       string
	renameInput     textinput.Model
}

func (m *transferModel) openDelete() {
	entries := m.operationEntries()
	if len(entries) == 0 {
		m.status = "没有可删除的项目"
		return
	}
	paths := make([]string, len(entries))
	for i, entry := range entries {
		if m.location == remoteSide {
			paths[i] = path.Join(m.remotePath, entry.name)
		} else {
			paths[i] = filepath.Join(m.localPath, entry.name)
		}
	}
	m.overlay = transferOverlay{kind: overlayDelete, deletePaths: paths}
}

func (m *transferModel) openRename() tea.Cmd {
	entry, ok := m.currentEntry()
	if !ok {
		m.status = "没有可重命名的项目"
		return nil
	}
	input := newTransferInput()
	input.SetValue(entry.name)
	input.CursorEnd()
	m.overlay = transferOverlay{kind: overlayRename, renameOld: entry.name, renameInput: input}
	return m.overlay.renameInput.Focus()
}

func (m *transferModel) openCreate() tea.Cmd {
	m.overlay = transferOverlay{kind: overlayCreate, renameInput: newTransferInput()}
	return m.overlay.renameInput.Focus()
}

// updateOverlay 保证内部覆盖层独占按键，避免底层路径、筛选和选择状态被修改。
func (m transferModel) handleOverlay(key tea.KeyPressMsg) (modalModel, tea.Cmd) {
	switch m.overlay.kind {
	case overlayDelete:
		switch key.String() {
		case "y":
			return m.startDelete()
		case "n", "esc":
			m.overlay = transferOverlay{}
		}
	case overlayProgress:
		if m.overlay.progress == progressConnecting || m.overlay.progress == progressRunning {
			if key.String() == "ctrl+c" && !m.overlay.cancelRequested {
				m.overlay.cancelRequested = true
				if m.overlay.progress == progressConnecting && m.cancelConnect != nil {
					m.cancelConnect()
				} else if m.task != nil {
					m.task.cancel()
				}
			}
			return m, nil
		}
		if key.String() == "enter" || key.String() == "esc" {
			m.overlay = transferOverlay{}
		}
	case overlayRename:
		switch key.String() {
		case "ctrl+c":
			m.overlay.renameInput.Reset()
			m.overlay.error = ""
		case "esc":
			m.overlay.renameInput.Blur()
			m.overlay = transferOverlay{}
		case "enter":
			if m.location == remoteSide {
				return m.startRemoteRename()
			}
			if err := renameLocal(m.localPath, m.overlay.renameOld, m.overlay.renameInput.Value()); err != nil {
				m.overlay.error = err.Error()
				return m, nil
			}
			oldName, newName := m.overlay.renameOld, m.overlay.renameInput.Value()
			if m.atClipboardSource() {
				for index := range m.clipboard.entries {
					if m.clipboard.entries[index].name == oldName {
						m.clipboard.entries[index].name = newName
					}
				}
			}
			m.overlay.renameInput.Blur()
			m.overlay = transferOverlay{}
			clear(m.selected)
			if err := m.refreshLocal(); err != nil {
				m.status = fmt.Sprintf("已将 %s 重命名为 %s，但刷新失败：%v", oldName, newName, err)
			} else {
				m.focusEntry(newName)
				m.status = fmt.Sprintf("已将 %s 重命名为 %s", oldName, newName)
			}
		default:
			var cmd tea.Cmd
			m.overlay.renameInput, cmd = m.overlay.renameInput.Update(key)
			m.overlay.error = ""
			return m, cmd
		}
	case overlayCreate:
		switch key.String() {
		case "ctrl+c":
			m.overlay.renameInput.Reset()
			m.overlay.error = ""
		case "esc":
			m.overlay.renameInput.Blur()
			m.overlay = transferOverlay{}
		case "enter":
			name := strings.TrimSpace(m.overlay.renameInput.Value())
			if err := validName(name); err != nil {
				m.overlay.error = err.Error()
				return m, nil
			}
			return m.startCreate(name)
		default:
			var cmd tea.Cmd
			m.overlay.renameInput, cmd = m.overlay.renameInput.Update(key)
			m.overlay.error = ""
			return m, cmd
		}
	case overlayHelp:
		if key.String() == "esc" || key.String() == "?" {
			m.overlay = transferOverlay{}
		}
	}
	return m, nil
}

// renderFileTransferOverlay 根据覆盖层类型渲染删除确认框或进度框。
func (m transferModel) renderOverlay() string {
	switch m.overlay.kind {
	case overlayDelete:
		body := strings.Join(append([]string{"确定要删除以下项目："}, m.overlay.deletePaths...), "\n")
		return formDialog("删除文件", body, "y 确认 | n/Esc 取消", confirmStyle, modalTitleStyle)
	case overlayProgress:
		return m.renderProgress()
	case overlayRename:
		input := m.overlay.renameInput
		input.SetWidth(modalContentWidth - 2)
		body := accentStyle.Render("> ") + input.View()
		if m.overlay.error != "" {
			body += "\n" + formErrorStyle.Render(m.overlay.error)
		}
		return formDialog("重命名", body, "Enter 确认 | Ctrl+C 清空 | Esc 取消", mutedStyle, modalTitleStyle)
	case overlayCreate:
		input := m.overlay.renameInput
		input.SetWidth(modalContentWidth - 2)
		body := accentStyle.Render("> ") + input.View()
		if m.overlay.error != "" {
			body += "\n" + formErrorStyle.Render(m.overlay.error)
		}
		return formDialog("新建文件夹", body, "Enter 确认 | Ctrl+C 清空 | Esc 取消", mutedStyle, modalTitleStyle)
	case overlayHelp:
		item := func(shortcut, description string) string {
			return plainStyle.Render(shortcut) + mutedStyle.Render(" "+description)
		}
		separator := mutedStyle.Render(" | ")
		body := strings.Join([]string{
			item("1/2", "切换本地/远程") + separator + item("↑/↓、j/k", "移动"),
			item("h/Backspace", "返回上级") + separator + item("l/Enter", "进入目录"),
			item("Space", "多选") + separator + item("Ctrl+A", "全选当前结果"),
			item("y/p", "复制/粘贴"),
			item("/", "筛选") + separator + item("Ctrl+G", "跳转"),
			item("n/s/t", "按名称/大小/时间排序"),
			item("d", "删除") + separator + item("r", "重命名"),
			item("a", "新建文件夹"),
			item("Ctrl+C", "清空/取消") + separator + item("q/Esc", "关闭/返回"),
		}, "\n")
		return formDialog("文件传输帮助", body, "?/Esc 关闭帮助", mutedStyle, modalTitleStyle)
	default:
		return ""
	}
}

// renderProgress 展示任务项目数、字节进度、结果或错误信息。
func (m transferModel) renderProgress() string {
	const progressWidth = 32

	title := m.overlay.operation + "中"
	totalItems := max(1, m.overlay.totalItems)
	itemIndex := max(1, m.overlay.itemIndex)
	percent := 0
	if m.overlay.totalBytes > 0 {
		percent = int(m.overlay.processedBytes * 100 / m.overlay.totalBytes)
	}
	percent = max(0, min(100, percent))
	filled := percent * progressWidth / 100
	bar := accentStyle.Render(strings.Repeat("█", filled)) +
		mutedStyle.Render(strings.Repeat("░", progressWidth-filled)) +
		plainStyle.Render(fmt.Sprintf(" %3d%%", percent))
	body := []string{
		plainStyle.Render(m.overlay.item),
		bar,
		mutedStyle.Render(fmt.Sprintf("%d / %d 项 · %s / %s", itemIndex, totalItems, formatSize(m.overlay.processedBytes), formatSize(m.overlay.totalBytes))),
	}
	footer := "Ctrl+C 取消"
	statusStyle := mutedStyle
	titleStyle := modalTitleStyle

	switch m.overlay.progress {
	case progressConnecting:
		title = "连接中"
		body = []string{plainStyle.Render(m.overlay.item), mutedStyle.Render("正在建立远程连接…")}
	case progressCompleted:
		title = "已完成"
		body = []string{
			plainStyle.Render(m.overlay.item),
			accentStyle.Render(strings.Repeat("█", progressWidth)) + plainStyle.Render(" 100%"),
			mutedStyle.Render(fmt.Sprintf("%d / %d 项 · %s / %s", totalItems, totalItems, formatSize(m.overlay.totalBytes), formatSize(m.overlay.totalBytes))),
		}
		footer = "Enter/Esc 关闭"
	case progressCancelled:
		title = "已取消"
		footer = "Enter/Esc 关闭"
	case progressFailed:
		title = "失败"
		body = []string{plainStyle.Render(m.overlay.item), formErrorStyle.Render(m.overlay.error)}
		footer = "Enter/Esc 关闭"
		statusStyle = formErrorStyle
		titleStyle = formErrorStyle
	}
	if m.overlay.cancelRequested && m.overlay.progress == progressRunning {
		footer = "正在取消…"
	}

	return formDialog(title, strings.Join(body, "\n"), footer, statusStyle, titleStyle)
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value, suffix := float64(size)/unit, "KB"
	if value >= unit {
		value, suffix = value/unit, "MB"
	}
	if value >= unit {
		value, suffix = value/unit, "GB"
	}
	return fmt.Sprintf("%.1f %s", value, suffix)
}

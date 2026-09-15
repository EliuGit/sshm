package tui

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"sshm/internal/i18n"
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
		m.status = i18n.T("No items to delete")
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
		m.status = i18n.T("No item to rename")
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
				m.status = i18n.T("Failed to refresh after renaming %s to %s: %v", oldName, newName, err)
			} else {
				m.focusEntry(newName)
				m.status = i18n.T("Renamed %s to %s", oldName, newName)
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
		body := strings.Join(append([]string{i18n.T("Confirm deletion of the following items:")}, m.overlay.deletePaths...), "\n")
		return formDialog(i18n.T("Delete files"), body, i18n.T("y Confirm | n/Esc Cancel"), confirmStyle, modalTitleStyle)
	case overlayProgress:
		return m.renderProgress()
	case overlayRename:
		input := m.overlay.renameInput
		input.SetWidth(modalContentWidth - 2)
		body := accentStyle.Render("> ") + input.View()
		if m.overlay.error != "" {
			body += "\n" + formErrorStyle.Render(m.overlay.error)
		}
		return formDialog(i18n.T("Rename"), body, i18n.T("Enter Confirm | Ctrl+C Clear | Esc Cancel"), mutedStyle, modalTitleStyle)
	case overlayCreate:
		input := m.overlay.renameInput
		input.SetWidth(modalContentWidth - 2)
		body := accentStyle.Render("> ") + input.View()
		if m.overlay.error != "" {
			body += "\n" + formErrorStyle.Render(m.overlay.error)
		}
		return formDialog(i18n.T("New folder"), body, i18n.T("Enter Confirm | Ctrl+C Clear | Esc Cancel"), mutedStyle, modalTitleStyle)
	case overlayHelp:
		item := func(shortcut, description string) string {
			return plainStyle.Render(shortcut) + mutedStyle.Render(" "+description)
		}
		separator := mutedStyle.Render(" | ")
		body := strings.Join([]string{
			item("1/2", i18n.T("Switch local/remote")) + separator + item(i18n.T("Up/Down, j/k"), i18n.T("Move")),
			item("h/Backspace", i18n.T("Go to parent")) + separator + item("l/Enter", i18n.T("Open directory")),
			item("Space", i18n.T("Select multiple")) + separator + item("y/p", i18n.T("Copy/Paste")),
			item("/", i18n.T("Filter")) + separator + item("Ctrl+G", i18n.T("Go to path")),
			item("n/s/t", i18n.T("Sort by name/size/time")),
			item("d", i18n.T("Delete")) + separator + item("r", i18n.T("Rename")),
			item("a", i18n.T("New folder")),
			item("Ctrl+C", i18n.T("Clear/Cancel")) + separator + item("q/Esc", i18n.T("Close/Back")),
		}, "\n")
		return formDialog(i18n.T("File transfer help"), body, i18n.T("?/Esc Close help"), mutedStyle, modalTitleStyle)
	default:
		return ""
	}
}

// renderProgress 展示任务项目数、字节进度、结果或错误信息。
func (m transferModel) renderProgress() string {
	const progressWidth = 32

	title := i18n.T("%s in progress", m.overlay.operation)
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
		mutedStyle.Render(i18n.T("%d / %d items · %s / %s", itemIndex, totalItems, formatSize(m.overlay.processedBytes), formatSize(m.overlay.totalBytes))),
	}
	footer := i18n.T("Ctrl+C Cancel")
	statusStyle := mutedStyle
	titleStyle := modalTitleStyle

	switch m.overlay.progress {
	case progressConnecting:
		title = i18n.T("Connecting")
		body = []string{plainStyle.Render(m.overlay.item), mutedStyle.Render(i18n.T("Connecting to the remote server..."))}
	case progressCompleted:
		title = i18n.T("Finished")
		body = []string{
			plainStyle.Render(m.overlay.item),
			accentStyle.Render(strings.Repeat("█", progressWidth)) + plainStyle.Render(" 100%"),
			mutedStyle.Render(i18n.T("%d / %d items · %s / %s", totalItems, totalItems, formatSize(m.overlay.totalBytes), formatSize(m.overlay.totalBytes))),
		}
		footer = i18n.T("Enter/Esc Close")
	case progressCancelled:
		title = i18n.T("Canceled")
		footer = i18n.T("Enter/Esc Close")
	case progressFailed:
		title = i18n.T("Failed")
		body = []string{plainStyle.Render(m.overlay.item), formErrorStyle.Render(m.overlay.error)}
		footer = i18n.T("Enter/Esc Close")
		statusStyle = formErrorStyle
		titleStyle = formErrorStyle
	}
	if m.overlay.cancelRequested && m.overlay.progress == progressRunning {
		footer = i18n.T("Canceling...")
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

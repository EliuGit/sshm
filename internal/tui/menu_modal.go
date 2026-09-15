package tui

import (
	"fmt"

	"sshm/internal/repository"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var menuItems = []string{"连接shell", "文件传输", "编辑连接", "删除连接"}

const menuWidth = 15

type menuModel struct {
	store      *repository.Store
	connection connectionRow
	selected   int
}

// shellPreparedMsg 将已解密的连接信息交给主模型启动交互式 Shell。
type shellPreparedMsg struct {
	connection repository.Connection
	credential []byte
	err        error
}

// shellFinishedMsg 表示交互式 Shell 已退出，TUI 已重新接管终端。
type shellFinishedMsg struct {
	connectionID int64
	err          error
}

func newMenu(store *repository.Store, connection connectionRow) menuModel {
	return menuModel{store: store, connection: connection}
}

// Update 处理连接操作菜单的导航、Shell、文件传输、编辑和删除入口。
func (m menuModel) Update(msg tea.Msg) (modalModel, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "esc", "q":
		return nil, nil
	case "up", "k":
		if m.selected > 0 {
			m.selected--
		}
	case "down", "j":
		if m.selected < len(menuItems)-1 {
			m.selected++
		}
	case "enter":
		switch m.selected {
		case 0:
			if m.store == nil {
				return newShellError(fmt.Errorf("数据库未连接")), nil
			}
			store, id := m.store, m.connection.id
			return m, func() tea.Msg {
				connection, credential, err := store.SSHConnection(id)
				return shellPreparedMsg{connection: connection, credential: credential, err: err}
			}
		case 1:
			transfer := newTransfer(m.connection, "", 0, 0)
			transfer.store = m.store
			return transfer, nil
		case 2:
			return newConnectionEditForm(m.store, m.connection), nil
		case 3:
			return newDeleteConfirm(m.store, m.connection), nil
		}
	}
	return m, nil
}

// shellErrorModel 显示连接或终端恢复失败信息。
type shellErrorModel struct{ err string }

func newShellError(err error) shellErrorModel { return shellErrorModel{err: err.Error()} }

// Update 处理 Shell 错误消息窗口的关闭。
func (m shellErrorModel) Update(msg tea.Msg) (modalModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "enter", "esc", "q":
			return nil, nil
		}
	}
	return m, nil
}

// View 显示 Shell 错误及关闭提示。
func (m shellErrorModel) View() string {
	return formDialog("连接 Shell 失败", m.err, "Enter/Esc 关闭", plainStyle, formErrorStyle)
}

// View 生成紧凑的圆角快捷菜单。
func (m menuModel) View() string {
	contentWidth := menuWidth - 2
	edgeStyle := lipgloss.NewStyle().Foreground(selectedColor).Background(backgroundColor)
	lines := make([]string, 0, len(menuItems))
	for i, item := range menuItems {
		line := fit("   "+item, contentWidth)
		if i == m.selected {
			line = edgeStyle.Render("") + selectedStyle.Width(contentWidth-2).Render("› "+item) + edgeStyle.Render("")
		}
		lines = append(lines, line)
	}
	return lipgloss.NewStyle().Width(menuWidth).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#4e9af1ff")).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// menuPosition 返回连接列表左边缘及当前可见选中行的下一行。
func menuPosition(height, selected int) (int, int) {
	visibleRows := max(1, height-10)
	return 4, 7 + min(max(0, selected), visibleRows-1)
}

type deleteConfirmModel struct {
	store      *repository.Store
	connection connectionRow
	err        string
}

func newDeleteConfirm(store *repository.Store, connection connectionRow) deleteConfirmModel {
	return deleteConfirmModel{store: store, connection: connection}
}

// Update 处理删除确认，并将数据库操作放入命令避免阻塞界面事件循环。
func (m deleteConfirmModel) Update(msg tea.Msg) (modalModel, tea.Cmd) {
	if result, ok := msg.(connectionDeleteFailedMsg); ok {
		m.err = result.err.Error()
		return m, nil
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "esc", "n":
		return nil, nil
	case "y":
		if m.store == nil {
			m.err = "数据库未连接"
			return m, nil
		}
		id := m.connection.id
		return m, func() tea.Msg {
			if err := m.store.DeleteConnection(id); err != nil {
				return connectionDeleteFailedMsg{err}
			}
			return connectionDeletedMsg{id: id}
		}
	}
	return m, nil
}

type connectionDeleteFailedMsg struct{ err error }

// View 显示待删除连接及确认快捷键。
func (m deleteConfirmModel) View() string {
	status := "y 确认 | n/Esc 取消"
	statusStyle := confirmStyle
	titleStyle := modalTitleStyle
	if m.err != "" {
		status = m.err
		statusStyle = formErrorStyle
		titleStyle = formErrorStyle
	}
	return formDialog("删除连接", fmt.Sprintf("确认删除连接“%s”？", m.connection.name), status, statusStyle, titleStyle)
}

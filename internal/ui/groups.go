package ui

import (
	"fmt"
	"sshm/internal/domain"
	"sshm/internal/i18n"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func newGroupPanelState(translator *i18n.Translator, theme Theme) groupPanelState {
	input := textinput.New()
	input.Placeholder = translator.T("group.name")
	input.Prompt = translator.T("group.input_prompt")
	input.PromptStyle = theme.Styles.SubtleText
	input.PlaceholderStyle = theme.Styles.MutedText
	input.Width = 28
	input.Blur()
	return groupPanelState{input: input}
}

func (m *Model) updateGroupPanel(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.hasActiveConfirm(confirmActionDeleteGroup) {
		next, cmd, handled := m.handleConfirmKey(keyMsg)
		if handled {
			return next, cmd
		}
	}

	if m.groups.inputMode != groupInputNone {
		switch keyMsg.String() {
		case "esc":
			m.groups.inputMode = groupInputNone
			m.groups.input.Blur()
			m.groups.input.SetValue("")
			m.groups.errorValue = ""
			return m, nil
		case "enter":
			name := strings.TrimSpace(m.groups.input.Value())
			if name == "" {
				m.groups.errorValue = m.translator.T("err.group_name_required")
				return m, nil
			}
			m.clearStaleErrorStatus()
			inputMode := m.groups.inputMode
			groupID := m.selectedGroupID()
			m.groups.inputMode = groupInputNone
			m.groups.input.Blur()
			m.groups.input.SetValue("")
			if inputMode == groupInputCreate {
				return m, m.createGroupCmd(name)
			}
			return m, m.renameGroupCmd(groupID, name)
		}
		var cmd tea.Cmd
		if m.groups.errorValue != "" {
			m.groups.errorValue = ""
		}
		m.groups.input, cmd = m.groups.input.Update(keyMsg)
		return m, cmd
	}

	switch keyMsg.String() {
	case "esc", "q":
		m.clearStaleErrorStatus()
		m.overlay = overlayNone
		return m, nil
	case "up", "k":
		if m.groups.selected > 0 {
			m.clearStaleErrorStatus()
			m.groups.errorValue = ""
			m.groups.selected--
		}
		return m, nil
	case "down", "j":
		if m.groups.selected < len(m.groups.items)-1 {
			m.clearStaleErrorStatus()
			m.groups.errorValue = ""
			m.groups.selected++
		}
		return m, nil
	case "a":
		m.clearStaleErrorStatus()
		m.groups.inputMode = groupInputCreate
		m.groups.input.Focus()
		m.groups.input.SetValue("")
		m.groups.errorValue = ""
		return m, nil
	case "r":
		item := m.selectedGroup()
		if item == nil || item.Ungrouped {
			m.groups.errorValue = m.translator.T("group.system_item_locked")
			return m, nil
		}
		m.clearStaleErrorStatus()
		m.groups.inputMode = groupInputRename
		m.groups.input.Focus()
		m.groups.input.SetValue(item.Name)
		m.groups.errorValue = ""
		return m, nil
	case "d":
		item := m.selectedGroup()
		if item == nil || item.Ungrouped {
			m.groups.errorValue = m.translator.T("group.system_item_locked")
			return m, nil
		}
		m.clearStaleErrorStatus()
		m.groups.errorValue = ""
		m.openDeleteGroupConfirm(*item)
		return m, nil
	case "enter":
		item := m.selectedGroup()
		if item == nil {
			return m, nil
		}
		m.clearStaleErrorStatus()
		m.groups.errorValue = ""
		if m.groups.mode == groupPanelMove {
			m.overlay = overlayNone
			var groupID *int64
			targetName := m.translator.T("group.ungrouped")
			if !item.Ungrouped {
				id := item.ID
				groupID = &id
				targetName = item.Name
			}
			return m, m.moveConnectionGroupCmd(m.groups.targetID, groupID, targetName)
		}
		m.overlay = overlayNone
		m.search = ""
		m.searchMode = false
		m.searchInput.Blur()
		m.searchInput.SetValue("")
		if item.Ungrouped {
			m.listScope = domain.ConnectionListScopeUngrouped
			m.listGroupID = 0
			m.listGroup = m.translator.T("group.ungrouped")
		} else {
			m.listScope = domain.ConnectionListScopeGroup
			m.listGroupID = item.ID
			m.listGroup = item.Name
		}
		m.selected = 0
		return m, m.loadConnectionsCmd()
	}
	return m, nil
}

func (m *Model) viewGroupPanel() string {
	styles := m.theme.Styles
	const tableWidth = 36
	title := m.translator.T("group.filter_title")
	desc := m.translator.T("group.filter_desc")
	if m.groups.mode == groupPanelMove {
		title = m.translator.T("group.move_title")
		desc = m.translator.T("group.move_desc")
	}
	lines := []string{styles.PageTitle.Render(title), styles.SubtleText.Render(desc), ""}
	if len(m.groups.items) == 0 {
		lines = append(lines, styles.SubtleText.Render(m.translator.T("group.empty")))
	} else {
		lines = append(lines,
			styles.SubtleText.Render(m.renderGroupTableHeader(tableWidth)),
			styles.MutedText.Render(strings.Repeat("─", tableWidth)),
		)
		for index, item := range m.groups.items {
			row := m.renderGroupRow(item, tableWidth)
			style := styles.Text
			if index == m.groups.selected {
				style = styles.Selection.Copy().Width(tableWidth)
			} else {
				style = styles.Text.Copy().Width(tableWidth)
			}
			lines = append(lines, style.Render(row))
		}
	}
	if m.groups.inputMode != groupInputNone {
		label := m.translator.T("group.create")
		if m.groups.inputMode == groupInputRename {
			label = m.translator.T("group.rename")
		}
		lines = append(lines, "", styles.FieldLabel.Render(label), m.groups.input.View())
	}
	if m.hasActiveConfirm(confirmActionDeleteGroup) {
		lines = append(lines, "",
			styles.PageTitle.Render(m.confirm.title),
			styles.SubtleText.Render(m.confirm.description),
			"",
			localizedShortcutHelpWidth(m.translator, m.theme, tableWidth,
				"enter/y", "group.shortcut_confirm",
				"esc/n", "group.shortcut_cancel",
				"q", "group.shortcut_close",
			),
		)
		return styles.Dialog.Width(44).Render(strings.Join(lines, "\n"))
	}
	if m.groups.errorValue != "" {
		lines = append(lines, "", styles.ErrorText.Render(m.groups.errorValue))
	}
	lines = append(lines, "", localizedShortcutHelpWidth(m.translator, m.theme, tableWidth,
		"enter", "group.shortcut_choose",
		"a", "group.shortcut_create",
		"r", "group.shortcut_rename",
		"d", "group.shortcut_delete",
		"esc/q", "group.shortcut_close",
	))
	return styles.Dialog.Width(44).Render(strings.Join(lines, "\n"))
}

func (m *Model) selectedGroupName() string {
	item := m.selectedGroup()
	if item == nil {
		return ""
	}
	if item.Ungrouped {
		return m.translator.T("group.ungrouped")
	}
	return item.Name
}

func (m *Model) renderGroupRow(item domain.ConnectionGroupListItem, width int) string {
	name := item.Name
	if item.Ungrouped {
		name = m.translator.T("group.ungrouped")
	}
	count := fmt.Sprintf("%d", item.ConnectionCount)
	return m.renderGroupTableRow(name, count, width)
}

func (m *Model) renderGroupTableHeader(width int) string {
	return m.renderGroupTableRow(
		m.translator.T("group.column_name"),
		m.translator.T("group.column_count"),
		width,
	)
}

func (m *Model) renderGroupTableRow(name string, count string, width int) string {
	countWidth := max(5, lipgloss.Width(m.translator.T("group.column_count")))
	nameWidth := max(8, width-countWidth-2)
	return padRight(truncate(name, nameWidth), nameWidth) + "  " + padLeft(truncate(count, countWidth), countWidth)
}

func padRight(value string, width int) string {
	gap := max(0, width-lipgloss.Width(value))
	return value + strings.Repeat(" ", gap)
}

func padLeft(value string, width int) string {
	gap := max(0, width-lipgloss.Width(value))
	return strings.Repeat(" ", gap) + value
}

func (m *Model) selectedGroup() *domain.ConnectionGroupListItem {
	if len(m.groups.items) == 0 || m.groups.selected < 0 || m.groups.selected >= len(m.groups.items) {
		return nil
	}
	return &m.groups.items[m.groups.selected]
}

func (m *Model) selectedGroupID() int64 {
	item := m.selectedGroup()
	if item == nil {
		return 0
	}
	return item.ID
}

func (m *Model) loadGroupsCmd() tea.Cmd {
	return func() tea.Msg {
		items, err := m.services.Groups.List()
		return groupsLoadedMsg{items: items, err: err}
	}
}

func (m *Model) createGroupCmd(name string) tea.Cmd {
	return func() tea.Msg {
		_, err := m.services.Groups.Create(name)
		if err != nil {
			return groupOpDoneMsg{err: err}
		}
		return groupOpDoneMsg{status: m.translator.T("status.group_created", name), success: true, reloadGroups: true}
	}
}

func (m *Model) renameGroupCmd(id int64, name string) tea.Cmd {
	return func() tea.Msg {
		err := m.services.Groups.Rename(id, name)
		if err != nil {
			return groupOpDoneMsg{err: err}
		}
		return groupOpDoneMsg{status: m.translator.T("status.group_renamed", name), success: true, reloadGroups: true, reloadConnections: true, groupID: id, groupName: name}
	}
}

func (m *Model) deleteGroupCmd(id int64, clearFilter bool) tea.Cmd {
	return func() tea.Msg {
		err := m.services.Groups.Delete(id)
		if err != nil {
			return groupOpDoneMsg{err: err}
		}
		return groupOpDoneMsg{status: m.translator.T("status.group_deleted"), success: true, reloadGroups: true, reloadConnections: true, groupID: id, clearGroupFilter: clearFilter}
	}
}

func (m *Model) moveConnectionGroupCmd(connectionID int64, groupID *int64, targetName string) tea.Cmd {
	return func() tea.Msg {
		err := m.services.Groups.MoveConnection(connectionID, groupID)
		if err != nil {
			return groupOpDoneMsg{err: err}
		}
		return groupOpDoneMsg{status: m.translator.T("status.connection_moved_group", targetName), success: true, reloadConnections: true}
	}
}

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
	if m.groups.confirming {
		switch keyMsg.String() {
		case "y", "enter":
			item := m.selectedGroup()
			if item == nil || item.Ungrouped {
				m.groups.confirming = false
				return m, nil
			}
			m.groups.confirming = false
			clearFilter := m.listScope == domain.ConnectionListScopeGroup && m.listGroupID == item.ID
			return m, m.deleteGroupCmd(item.ID, clearFilter)
		case "q":
			m.overlay = overlayNone
			m.groups.confirming = false
			return m, nil
		case "n":
			m.groups.confirming = false
			return m, nil
		}
		return m, nil
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
		m.groups.input, cmd = m.groups.input.Update(keyMsg)
		return m, cmd
	}

	switch keyMsg.String() {
	case "q":
		m.overlay = overlayNone
		return m, nil
	case "up", "k":
		if m.groups.selected > 0 {
			m.groups.selected--
		}
		return m, nil
	case "down", "j":
		if m.groups.selected < len(m.groups.items)-1 {
			m.groups.selected++
		}
		return m, nil
	case "a":
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
		m.groups.confirming = true
		return m, nil
	case "enter":
		item := m.selectedGroup()
		if item == nil {
			return m, nil
		}
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
		for index, item := range m.groups.items {
			row := m.renderGroupRow(item, 34)
			style := styles.Text
			if index == m.groups.selected {
				style = styles.Selection.Copy().Width(34)
			} else {
				style = styles.Text.Copy().Width(34)
			}
			lines = append(lines, style.Render(truncate(row, 34)))
		}
	}
	if m.groups.inputMode != groupInputNone {
		label := m.translator.T("group.create")
		if m.groups.inputMode == groupInputRename {
			label = m.translator.T("group.rename")
		}
		lines = append(lines, "", styles.FieldLabel.Render(label), m.groups.input.View())
	}
	if m.groups.confirming {
		lines = append(lines, "",
			styles.PageTitle.Render(m.translator.T("group.delete_title")),
			styles.SubtleText.Render(m.translator.T("group.delete_desc", m.selectedGroupName())),
			"",
			localizedShortcutHelpWidth(m.translator, m.theme, 34,
				"enter/y", "group.shortcut_confirm",
				"n", "group.shortcut_cancel",
				"q", "group.shortcut_close",
			),
		)
		return styles.Dialog.Width(42).Render(strings.Join(lines, "\n"))
	}
	if m.groups.errorValue != "" {
		lines = append(lines, "", styles.ErrorText.Render(m.groups.errorValue))
	}
	lines = append(lines, "", localizedShortcutHelpWidth(m.translator, m.theme, 34,
		"enter", "group.shortcut_choose",
		"a", "group.shortcut_create",
		"r", "group.shortcut_rename",
		"d", "group.shortcut_delete",
		"q", "group.shortcut_close",
	))
	return styles.Dialog.Width(42).Render(strings.Join(lines, "\n"))
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
	gap := max(1, width-lipgloss.Width(name)-lipgloss.Width(count))
	if gap == 1 && lipgloss.Width(name)+lipgloss.Width(count)+gap > width {
		name = truncate(name, max(1, width-lipgloss.Width(count)-gap))
	}
	return name + strings.Repeat(" ", gap) + count
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

package ui

import (
	"sshm/internal/domain"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) hasActiveConfirm(action confirmAction) bool {
	return m.confirm.action == action
}

func (m *Model) clearConfirm() {
	m.confirm = confirmState{}
}

func (m *Model) openDeleteConnectionConfirm(id int64) {
	m.confirm = confirmState{
		action:       confirmActionDeleteConnection,
		title:        m.translator.T("home.delete_title"),
		description:  m.translator.T("home.delete_desc"),
		connectionID: id,
	}
	m.overlay = overlayDelete
}

func (m *Model) openDeleteGroupConfirm(item domain.ConnectionGroupListItem) {
	m.confirm = confirmState{
		action:           confirmActionDeleteGroup,
		title:            m.translator.T("group.delete_title"),
		description:      m.translator.T("group.delete_desc", item.Name),
		groupID:          item.ID,
		clearGroupFilter: m.home.listScope == domain.ConnectionListScopeGroup && m.home.listGroupID == item.ID,
	}
}

func (m *Model) openBrowserOverwriteConfirm(sourcePath string, targetPath string, pending *browserPendingOperation) {
	m.confirm = confirmState{
		action:           confirmActionBrowserOverwrite,
		title:            m.translator.T("browser.overwrite"),
		sourcePath:       sourcePath,
		targetPath:       targetPath,
		choiceEnabled:    true,
		confirmSelection: false,
	}
	m.overlay = overlayBrowserConfirm
	m.browser.pending = pending
}

func (m *Model) openBrowserDeleteConfirm(entry domain.FileEntry, pending *browserPendingOperation) {
	m.confirm = confirmState{
		action:           confirmActionBrowserDelete,
		title:            m.translator.T("browser.delete_title"),
		description:      m.translator.T("browser.delete_desc", entry.Name),
		sourcePath:       entry.Path,
		choiceEnabled:    true,
		confirmSelection: false,
	}
	m.overlay = overlayBrowserConfirm
	m.browser.pending = pending
}

func (m *Model) handleConfirmKey(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.confirm.action {
	case confirmActionDeleteConnection:
		return m.handleDeleteConnectionConfirmKey(keyMsg)
	case confirmActionDeleteGroup:
		return m.handleDeleteGroupConfirmKey(keyMsg)
	case confirmActionBrowserOverwrite, confirmActionBrowserDelete:
		return m.handleBrowserPendingConfirmKey(keyMsg)
	default:
		return m, nil, false
	}
}

func (m *Model) handleDeleteConnectionConfirmKey(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch keyMsg.String() {
	case "y", "enter":
		id := m.confirm.connectionID
		m.clearConfirm()
		m.overlay = overlayNone
		return m, m.deleteConnectionCmd(id), true
	case "n", "esc", "q":
		m.clearConfirm()
		m.overlay = overlayNone
		m.setInfoStatus(m.translator.T("status.delete_cancelled"))
		return m, nil, true
	default:
		return m, nil, true
	}
}

func (m *Model) handleDeleteGroupConfirmKey(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch keyMsg.String() {
	case "y", "enter":
		groupID := m.confirm.groupID
		clearFilter := m.confirm.clearGroupFilter
		m.clearConfirm()
		return m, m.deleteGroupCmd(groupID, clearFilter), true
	case "n", "esc":
		m.clearConfirm()
		return m, nil, true
	case "q":
		m.clearConfirm()
		m.overlay = overlayNone
		return m, nil, true
	default:
		return m, nil, true
	}
}

func (m *Model) handleBrowserPendingConfirmKey(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch keyMsg.String() {
	case "tab", "left", "right", "h", "l":
		if !m.confirm.choiceEnabled {
			return m, nil, true
		}
		m.confirm.confirmSelection = !m.confirm.confirmSelection
		return m, nil, true
	case "y":
		next, cmd := m.runPendingBrowserOperation()
		return next, cmd, true
	case "enter":
		if m.confirm.choiceEnabled && !m.confirm.confirmSelection {
			next, cmd := m.cancelBrowserPendingConfirm()
			return next, cmd, true
		}
		next, cmd := m.runPendingBrowserOperation()
		return next, cmd, true
	case "n", "esc":
		next, cmd := m.cancelBrowserPendingConfirm()
		return next, cmd, true
	case "q":
		return m, nil, true
	default:
		return m, nil, true
	}
}

func (m *Model) cancelBrowserPendingConfirm() (tea.Model, tea.Cmd) {
	status := m.translator.T("status.cancelled")
	if m.confirm.action == confirmActionBrowserOverwrite {
		status = m.translator.T("status.transfer_cancelled")
	}
	if m.browser.pending != nil && m.browser.pending.cancel != "" {
		status = m.browser.pending.cancel
	}
	m.clearConfirm()
	m.overlay = overlayNone
	m.browser.pending = nil
	m.setInfoStatus(status)
	return m, nil
}

func (m *Model) runPendingBrowserOperation() (tea.Model, tea.Cmd) {
	pending := m.browser.pending
	m.clearConfirm()
	m.overlay = overlayNone
	m.browser.pending = nil
	if pending == nil {
		return m, nil
	}
	return m, m.browserWorkflow().runOperationCmd(pending)
}

func (m *Model) confirmSourceText(width int) string {
	return m.translator.T("browser.confirm_source", truncate(m.confirm.sourcePath, max(20, width)))
}

func (m *Model) confirmTargetText(width int) string {
	return truncate(m.translator.T("browser.confirm_target", m.confirm.targetPath), max(20, width))
}

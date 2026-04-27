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

func (m *Model) openBrowserOverwriteConfirm(sourcePath string, targetPath string, pending *browserTransfer) {
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

func (m *Model) handleConfirmKey(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.confirm.action {
	case confirmActionDeleteConnection:
		return m.handleDeleteConnectionConfirmKey(keyMsg)
	case confirmActionDeleteGroup:
		return m.handleDeleteGroupConfirmKey(keyMsg)
	case confirmActionBrowserOverwrite:
		return m.handleBrowserOverwriteConfirmKey(keyMsg)
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

func (m *Model) handleBrowserOverwriteConfirmKey(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch keyMsg.String() {
	case "tab", "left", "right", "h", "l":
		if !m.confirm.choiceEnabled {
			return m, nil, true
		}
		m.confirm.confirmSelection = !m.confirm.confirmSelection
		return m, nil, true
	case "y":
		next, cmd := m.runPendingBrowserTransfer()
		return next, cmd, true
	case "enter":
		if m.confirm.choiceEnabled && !m.confirm.confirmSelection {
			next, cmd := m.cancelBrowserOverwriteConfirm()
			return next, cmd, true
		}
		next, cmd := m.runPendingBrowserTransfer()
		return next, cmd, true
	case "n", "esc":
		next, cmd := m.cancelBrowserOverwriteConfirm()
		return next, cmd, true
	case "q":
		return m, nil, true
	default:
		return m, nil, true
	}
}

func (m *Model) cancelBrowserOverwriteConfirm() (tea.Model, tea.Cmd) {
	m.clearConfirm()
	m.overlay = overlayNone
	m.browser.pending = nil
	m.setInfoStatus(m.translator.T("status.transfer_cancelled"))
	return m, nil
}

func (m *Model) runPendingBrowserTransfer() (tea.Model, tea.Cmd) {
	pending := m.browser.pending
	m.clearConfirm()
	m.overlay = overlayNone
	m.browser.pending = nil
	if pending == nil {
		return m, nil
	}
	progressCh := make(chan transferProgressMsg, 16)
	return m, tea.Batch(
		func() tea.Msg {
			err := pending.run(func(progress domain.TransferProgress) {
				select {
				case progressCh <- transferProgressMsg{progress: progress, action: pending.action, source: progressCh}:
				default:
				}
			})
			close(progressCh)
			if err != nil {
				return opDoneMsg{err: err}
			}
			return opDoneMsg{
				status:        pending.success,
				success:       true,
				reloadBrowser: true,
				targetPanel:   pending.panel,
				selectName:    pending.selectBy,
			}
		},
		listenTransferProgress(progressCh),
	)
}

func (m *Model) confirmSourceText(width int) string {
	return m.translator.T("browser.confirm_source", truncate(m.confirm.sourcePath, max(20, width)))
}

func (m *Model) confirmTargetText(width int) string {
	return truncate(m.translator.T("browser.confirm_target", m.confirm.targetPath), max(20, width))
}

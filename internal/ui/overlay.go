package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) handleHomeOverlayKey(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	if m.overlay == overlayCommandPalette {
		next, cmd := m.updateCommandPalette(keyMsg)
		return next, cmd, true
	}
	if m.hasActiveConfirm(confirmActionDeleteConnection) {
		next, cmd, handled := m.handleConfirmKey(keyMsg)
		if handled {
			return next, cmd, true
		}
	}

	switch m.overlay {
	case overlayHelp:
		switch keyMsg.String() {
		case "?", "esc", "q", "enter":
			m.overlay = overlayNone
		}
		return m, nil, true
	case overlayGroup:
		next, cmd := m.updateGroupPanel(keyMsg)
		return next, cmd, true
	default:
		return m, nil, false
	}
}

func (m *Model) homeOverlayView(contentWidth int, contentHeight int) string {
	styles := m.theme.Styles
	switch m.overlay {
	case overlayDelete:
		return styles.Dialog.Width(44).Render(strings.Join([]string{
			styles.PageTitle.Render(m.confirm.title),
			"",
			styles.SubtleText.Render(m.confirm.description),
			"",
			styles.MutedText.Render(m.translator.T("home.delete_keys", styles.Keycap.Render("esc"), styles.Keycap.Render("enter"), styles.Keycap.Render("y"))),
		}, "\n"))
	case overlayHelp:
		return m.viewHomeHelp()
	case overlayGroup:
		return m.viewGroupPanel()
	case overlayCommandPalette:
		return m.viewCommandPalette(contentWidth)
	default:
		return ""
	}
}

package ui

import (
	"fmt"
	"path/filepath"
	"sshm/internal/domain"
	"sshm/internal/i18n"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) updateBrowser(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if m.overlay == overlayBrowserInput {
			switch keyMsg.String() {
			case "esc":
				m.clearStaleErrorStatus()
				m.overlay = overlayNone
				return m, nil
			case "enter":
				value := strings.TrimSpace(m.browser.input.Value())
				mode := m.browser.inputMode
				m.overlay = overlayNone
				if mode == browserInputFilter {
					if m.browser.activePanel == domain.LocalPanel {
						m.browser.localPanel.filter = value
						return m, m.loadLocalCmd(m.browser.localPanel.path, value)
					}
					m.browser.remotePanel.filter = value
					return m, m.loadRemoteCmd(m.browser.remotePanel.path, value)
				}
				if value == "" {
					return m, nil
				}
				if m.browser.activePanel == domain.LocalPanel {
					return m, m.loadLocalCmd(value, m.browser.localPanel.filter)
				}
				return m, m.loadRemoteCmd(value, m.browser.remotePanel.filter)
			default:
				var cmd tea.Cmd
				m.clearStaleErrorStatus()
				m.browser.input, cmd = m.browser.input.Update(msg)
				return m, cmd
			}
		}

		if m.hasActiveConfirm(confirmActionBrowserOverwrite) {
			next, cmd, handled := m.handleConfirmKey(keyMsg)
			if handled {
				return next, cmd
			}
		}

		switch keyMsg.String() {
		case "q":
			m.closeBrowserSession()
			m.page = pageHome
			m.overlay = overlayNone
			m.setInfoStatus(m.translator.T("status.returned_connections"))
			return m, tea.Batch(tea.ClearScreen, m.loadConnectionsCmd())
		case "esc":
			return m, m.clearActiveBrowserFilter()
		case "tab":
			m.clearStaleErrorStatus()
			if m.browser.activePanel == domain.LocalPanel {
				m.browser.activePanel = domain.RemotePanel
			} else {
				m.browser.activePanel = domain.LocalPanel
			}
		case "up", "k":
			panel := m.activeBrowserPanel()
			if panel.cursor > 0 {
				m.clearStaleErrorStatus()
				panel.cursor--
			}
		case "down", "j":
			panel := m.activeBrowserPanel()
			if panel.cursor < len(panel.rows())-1 {
				m.clearStaleErrorStatus()
				panel.cursor++
			}
		case "g":
			m.clearStaleErrorStatus()
			panel := m.activeBrowserPanel()
			panel.cursor = 0
		case "/":
			m.clearStaleErrorStatus()
			m.overlay = overlayBrowserInput
			m.browser.inputMode = browserInputFilter
			m.browser.input.SetValue(m.activeBrowserPanel().filter)
			m.browser.input.Focus()
			return m, nil
		case ":":
			m.clearStaleErrorStatus()
			m.overlay = overlayBrowserInput
			m.browser.inputMode = browserInputGoto
			m.browser.input.SetValue(m.activeBrowserPanel().path)
			m.browser.input.Focus()
			return m, nil
		case "r":
			m.clearStaleErrorStatus()
			return m, m.reloadBrowserCmd()
		case "enter", "right", "l":
			row, ok := m.activeBrowserPanel().selected()
			if !ok {
				return m, nil
			}
			if row.Name == ".." || row.IsDir {
				m.clearStaleErrorStatus()
				if m.browser.activePanel == domain.LocalPanel {
					return m, m.loadLocalCmd(row.Path, m.browser.localPanel.filter)
				}
				return m, m.loadRemoteCmd(row.Path, m.browser.remotePanel.filter)
			}
		case "backspace", "left", "h":
			panel := m.activeBrowserPanel()
			next := parentPath(panel.path, m.browser.activePanel == domain.RemotePanel)
			if panel.path == next {
				return m, nil
			}
			m.clearStaleErrorStatus()
			if m.browser.activePanel == domain.LocalPanel {
				return m, m.loadLocalCmd(next, panel.filter)
			}
			return m, m.loadRemoteCmd(next, panel.filter)
		case "ctrl+u":
			if m.browser.activePanel != domain.LocalPanel {
				m.setInfoStatus(m.translator.T("status.focus_local_upload"))
				return m, nil
			}
			row, ok := m.browser.localPanel.selected()
			if !ok || row.Name == ".." {
				return m, nil
			}
			m.clearStaleErrorStatus()
			targetPath := joinRemotePath(m.browser.remotePanel.path, row.Name)
			return m.prepareTransfer(m.translator.T("status.uploading"), row.Path, targetPath, domain.RemotePanel, func(progress func(domain.TransferProgress)) error {
				if m.browser.session == nil {
					return fmt.Errorf("browser session is not ready")
				}
				return m.browser.session.Upload(row.Path, m.browser.remotePanel.path, progress)
			}, m.translator.T("status.uploaded", row.Name), row.Name)
		case "ctrl+d":
			if m.browser.activePanel != domain.RemotePanel {
				m.setInfoStatus(m.translator.T("status.focus_remote_download"))
				return m, nil
			}
			row, ok := m.browser.remotePanel.selected()
			if !ok || row.Name == ".." {
				return m, nil
			}
			m.clearStaleErrorStatus()
			targetPath := filepath.Join(m.browser.localPanel.path, row.Name)
			return m.prepareTransfer(m.translator.T("status.downloading"), row.Path, targetPath, domain.LocalPanel, func(progress func(domain.TransferProgress)) error {
				if m.browser.session == nil {
					return fmt.Errorf("browser session is not ready")
				}
				return m.browser.session.Download(row.Path, m.browser.localPanel.path, progress)
			}, m.translator.T("status.downloaded", row.Name), row.Name)
		}
	}
	return m, nil
}

func (m *Model) prepareTransfer(action string, sourcePath string, targetPath string, panel domain.FilePanel, run func(func(domain.TransferProgress)) error, success string, selectName string) (tea.Model, tea.Cmd) {
	var (
		exists bool
		err    error
	)
	if panel == domain.RemotePanel {
		if m.browser.session == nil {
			err = fmt.Errorf("browser session is not ready")
		} else {
			exists, err = m.browser.session.PathExists(targetPath)
		}
	} else {
		exists, err = m.services.Files.ExistsLocal(targetPath)
	}
	if err != nil {
		if isBrowserSessionError(err) {
			return m.handleBrowserSessionFailure(err)
		}
		m.setErrorStatus(err)
		return m, nil
	}
	if exists {
		m.openBrowserOverwriteConfirm(sourcePath, targetPath, &browserTransfer{
			action:   action,
			source:   sourcePath,
			target:   targetPath,
			run:      run,
			success:  success,
			panel:    panel,
			selectBy: selectName,
		})
		return m, nil
	}
	progressCh := make(chan transferProgressMsg, 16)
	return m, tea.Batch(
		func() tea.Msg {
			err := run(func(progress domain.TransferProgress) {
				select {
				case progressCh <- transferProgressMsg{progress: progress, action: action, source: progressCh}:
				default:
				}
			})
			close(progressCh)
			if err != nil {
				return opDoneMsg{err: err}
			}
			return opDoneMsg{status: success, success: true, reloadBrowser: true, targetPanel: panel, selectName: selectName}
		},
		listenTransferProgress(progressCh),
	)
}

func (m *Model) viewBrowser() string {
	styles := m.theme.Styles
	viewportWidth := m.width
	if viewportWidth == 0 {
		viewportWidth = 120
	}
	viewportHeight := m.height
	if viewportHeight == 0 {
		viewportHeight = 34
	}
	contentWidth := max(40, viewportWidth-4)
	contentHeight := max(16, viewportHeight-2)
	footerHeight := 4
	headerHeight := 4
	panelWidth := max(18, (contentWidth-7)/2)
	panelHeight := max(8, contentHeight-headerHeight-footerHeight-4)
	leftPanel := m.renderBrowserPanel(m.browser.localPanel, panelWidth, panelHeight, m.browser.activePanel == domain.LocalPanel)
	rightPanel := m.renderBrowserPanel(m.browser.remotePanel, panelWidth, panelHeight, m.browser.activePanel == domain.RemotePanel)
	header := strings.Join([]string{
		styles.PageTitle.Render(m.translator.T("browser.title")),
		styles.SubtleText.Render(m.translator.T("browser.subtitle", m.browser.connection.Username, m.browser.connection.Host)),
	}, "\n")
	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "  ", rightPanel)
	content := strings.Join([]string{
		header,
		body,
		m.renderBrowserFooter(contentWidth, footerHeight),
	}, "\n\n")
	return styles.App.Render(content)
}

func (m *Model) renderBrowserPanel(panel filePanel, width int, height int, focused bool) string {
	styles := m.theme.Styles
	style := styles.Panel.Width(width).Height(height)
	if focused {
		style = styles.FocusedPanel.Width(width).Height(height)
	}
	rows := panel.rows()
	lines := []string{
		m.renderBrowserPanelHeader(panel, width, focused),
	}
	headerLines := len(lines)
	contentHeight := max(1, height-headerLines-2)
	start := 0
	if panel.cursor >= contentHeight {
		start = panel.cursor - contentHeight + 1
	}
	end := start + contentHeight
	if end > len(rows) {
		end = len(rows)
	}
	for index, row := range rows {
		if index < start || index >= end {
			continue
		}
		name := row.Name
		if row.IsDir {
			name = "📁 " + name + "/"
		} else {
			name = "📄 " + name
		}
		nameWidth := max(8, width-16)
		line := fmt.Sprintf("  %-*s %8s", nameWidth, truncate(name, nameWidth), humanSize(row.Size))
		if index == panel.cursor {
			rowStyle := styles.SelectionInactive
			if focused {
				rowStyle = styles.Selection
			}
			line = rowStyle.Width(max(12, width-4)).Render(fmt.Sprintf("› %-*s %8s", nameWidth, truncate(name, nameWidth), humanSize(row.Size)))
		}
		lines = append(lines, line)
	}
	if len(rows) == 0 {
		lines = append(lines, styles.SubtleText.Render(m.translator.T("browser.empty")))
	}
	if panel.loading {
		lines = append(lines, styles.SubtleText.Render(m.translator.T("browser.loading")))
	}
	for len(lines) < height-2 {
		lines = append(lines, "")
	}
	return style.Render(strings.Join(lines, "\n"))
}

func (m *Model) renderBrowserPanelHeader(panel filePanel, width int, focused bool) string {
	styles := m.theme.Styles
	panelTitleStyle := styles.PanelTitle
	if focused {
		panelTitleStyle = styles.PanelTitleFocused
	}

	title := panelTitleStyle.Render(panel.title)
	infoParts := make([]string, 0, 2)
	if panel.filter != "" {
		infoParts = append(infoParts, m.translator.T("browser.filter_label", panel.filter))
	}
	if strings.TrimSpace(panel.path) != "" {
		infoParts = append(infoParts, panel.path)
	}
	if len(infoParts) == 0 {
		return title
	}

	innerWidth := max(10, width-4)
	infoWidth := innerWidth - lipgloss.Width(panel.title) - 1
	if infoWidth <= 0 {
		return title
	}

	info := truncate(strings.Join(infoParts, " "), infoWidth)
	return lipgloss.JoinHorizontal(lipgloss.Left, title, styles.SubtleText.Render(" "+info))
}

func (m *Model) renderBrowserFooter(width int, height int) string {
	styles := m.theme.Styles
	var lines []string
	switch m.overlay {
	case overlayBrowserInput:
		title := m.translator.T("browser.path")
		if m.browser.inputMode == browserInputFilter {
			title = m.translator.T("browser.filter")
		}
		lines = []string{
			styles.FieldLabel.Render(title),
			m.browser.input.View(),
			localizedShortcutHelpWidth(m.translator, m.theme, max(24, width-8),
				"enter", "shortcut.confirm",
				"esc", "shortcut.cancel",
			),
		}
	case overlayBrowserConfirm:
		yesButton := "[ " + m.translator.T("browser.yes") + " ]"
		noButton := "[ " + m.translator.T("browser.no") + " ]"
		if m.confirm.confirmSelection {
			yesButton = styles.Selection.Render(yesButton)
		} else {
			noButton = styles.Selection.Render(noButton)
		}
		lines = []string{
			styles.FieldLabel.Render(m.confirm.title),
			styles.SubtleText.Render(m.confirmSourceText(width - 12)),
			m.confirmTargetText(width - 8),
			lipgloss.JoinHorizontal(lipgloss.Left, yesButton, "  ", noButton),
			localizedShortcutHelpWidth(m.translator, m.theme, max(24, width-8),
				"←/→", "shortcut.choose",
				"enter/y", "shortcut.confirm",
				"esc/n", "shortcut.cancel",
			),
		}
	default:
		active := m.activeBrowserPanel()
		lines = []string{
			m.renderStatus(),
			styles.SubtleText.Render(m.translator.T("browser.active_path", active.title, truncate(active.path, max(20, width-20)))),
			localizedShortcutHelpWidth(m.translator, m.theme, max(24, width-8),
				"j/k", "shortcut.move",
				"enter/l", "shortcut.open",
				"h/backspace", "shortcut.up",
				"tab", "shortcut.switch",
				"/", "shortcut.filter",
				":", "shortcut.goto",
				"c-u", "shortcut.upload",
				"c-d", "shortcut.download",
				"r", "shortcut.refresh",
				"g", "shortcut.top",
				"esc", "shortcut.clear_filter",
				"q", "shortcut.back",
			),
		}
	}
	for len(lines) < height-2 {
		lines = append(lines, "")
	}
	return styles.Panel.Width(width - 4).Height(height).Render(strings.Join(lines, "\n"))
}

func (m *Model) clearActiveBrowserFilter() tea.Cmd {
	panel := m.activeBrowserPanel()
	if strings.TrimSpace(panel.filter) == "" {
		return nil
	}

	panel.filter = ""
	if m.browser.activePanel == domain.LocalPanel {
		return m.loadLocalCmd(panel.path, "")
	}
	return m.loadRemoteCmd(panel.path, "")
}

func (m *Model) loadLocalCmd(pathValue string, filter string) tea.Cmd {
	m.browser.localPanel.loading = true
	m.setInfoStatus(m.translator.T("status.loading_browser"))
	return func() tea.Msg {
		items, currentPath, err := m.services.Files.ListLocal(pathValue, filter)
		return localLoadedMsg{items: items, path: currentPath, err: err}
	}
}

func (m *Model) loadRemoteCmd(pathValue string, filter string) tea.Cmd {
	m.browser.remotePanel.loading = true
	m.setInfoStatus(m.translator.T("status.loading_browser"))
	return func() tea.Msg {
		if m.browser.session == nil {
			return remoteLoadedMsg{err: fmt.Errorf("browser session is not ready")}
		}
		items, currentPath, err := m.browser.session.ListRemote(pathValue)
		if err == nil {
			items = filterRemoteEntries(items, filter)
		}
		return remoteLoadedMsg{items: items, path: currentPath, err: err}
	}
}

func (m *Model) reloadBrowserCmd() tea.Cmd {
	return tea.Batch(
		m.loadLocalCmd(m.browser.localPanel.path, m.browser.localPanel.filter),
		m.loadRemoteCmd(m.browser.remotePanel.path, m.browser.remotePanel.filter),
	)
}

func (m *Model) reloadBrowserSelectCmd(targetPanel domain.FilePanel, selectName string) tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			items, currentPath, err := m.services.Files.ListLocal(m.browser.localPanel.path, m.browser.localPanel.filter)
			msg := localLoadedMsg{items: items, path: currentPath, err: err}
			if targetPanel == domain.LocalPanel {
				msg.selectName = selectName
			}
			return msg
		},
		func() tea.Msg {
			if m.browser.session == nil {
				return remoteLoadedMsg{err: fmt.Errorf("browser session is not ready")}
			}
			items, currentPath, err := m.browser.session.ListRemote(m.browser.remotePanel.path)
			if err == nil {
				items = filterRemoteEntries(items, m.browser.remotePanel.filter)
			}
			msg := remoteLoadedMsg{items: items, path: currentPath, err: err}
			if targetPanel == domain.RemotePanel {
				msg.selectName = selectName
			}
			return msg
		},
	)
}

func (m *Model) activeBrowserPanel() *filePanel {
	if m.browser.activePanel == domain.LocalPanel {
		return &m.browser.localPanel
	}
	return &m.browser.remotePanel
}

func newBrowserState(translator *i18n.Translator, theme Theme) browserState {
	input := newInput(theme, "", 48)
	return browserState{
		localPanel: filePanel{
			panel: domain.LocalPanel,
			title: translator.T("browser.local"),
		},
		remotePanel: filePanel{
			panel: domain.RemotePanel,
			title: translator.T("browser.remote"),
		},
		activePanel: domain.LocalPanel,
		input:       input,
	}
}

func (m *Model) closeBrowserSession() {
	if m.browser.session == nil {
		return
	}
	_ = m.browser.session.Close()
	m.browser.session = nil
}

func filterRemoteEntries(items []domain.FileEntry, filter string) []domain.FileEntry {
	query := strings.ToLower(strings.TrimSpace(filter))
	if query == "" {
		return items
	}
	filtered := make([]domain.FileEntry, 0, len(items))
	for _, entry := range items {
		if strings.Contains(strings.ToLower(entry.Name), query) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func (m *Model) syncBrowserStatus() {
	if m.page != pageBrowser || m.browser.localPanel.loading || m.browser.remotePanel.loading {
		return
	}
	if m.err != nil {
		return
	}
	if m.status == m.translator.T("status.loading_browser") || strings.TrimSpace(m.status) == "" {
		m.setInfoStatus(m.translator.T("status.browser_ready"))
	}
}

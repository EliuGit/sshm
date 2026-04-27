package ui

import (
	"fmt"
	"sshm/internal/domain"
	"sshm/internal/i18n"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) updateBrowser(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if m.overlay == overlayCommandPalette {
			return m.updateCommandPalette(keyMsg)
		}
		if m.overlay == overlayBrowserInput {
			switch keyMsg.String() {
			case "esc":
				m.clearStaleErrorStatus()
				if m.browser.inputMode == browserInputFilter {
					panel := m.activeBrowserPanel()
					panel.filter = ""
					panel.cursor = clamp(panel.cursor, len(panel.rows()))
				}
				m.overlay = overlayNone
				return m, nil
			case "enter":
				value := strings.TrimSpace(m.browser.input.Value())
				mode := m.browser.inputMode
				m.overlay = overlayNone
				if mode == browserInputFilter {
					if m.browser.activePanel == domain.LocalPanel {
						m.browser.localPanel.filter = value
						m.browser.localPanel.cursor = clamp(m.browser.localPanel.cursor, len(m.browser.localPanel.rows()))
						return m, nil
					}
					m.browser.remotePanel.filter = value
					m.browser.remotePanel.cursor = clamp(m.browser.remotePanel.cursor, len(m.browser.remotePanel.rows()))
					return m, nil
				}
				if value == "" {
					return m, nil
				}
				return m, m.navigateBrowserPath(m.browser.activePanel, value)
			default:
				var cmd tea.Cmd
				m.clearStaleErrorStatus()
				before := m.browser.input.Value()
				m.browser.input, cmd = m.browser.input.Update(msg)
				after := strings.TrimSpace(m.browser.input.Value())
				if m.browser.inputMode == browserInputFilter && before != m.browser.input.Value() {
					if m.browser.activePanel == domain.LocalPanel {
						m.browser.localPanel.filter = after
						m.browser.localPanel.cursor = clamp(m.browser.localPanel.cursor, len(m.browser.localPanel.rows()))
						return m, cmd
					}
					m.browser.remotePanel.filter = after
					m.browser.remotePanel.cursor = clamp(m.browser.remotePanel.cursor, len(m.browser.remotePanel.rows()))
					return m, cmd
				}
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
		case "ctrl+p":
			return m.openCommandPalette()
		case "q":
			return m.leaveBrowser()
		case "esc":
			return m.clearBrowserFilter()
		case "tab":
			return m.switchBrowserPanel()
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
			return m.moveBrowserTop()
		case "/":
			return m.openBrowserFilterInput()
		case ":":
			return m.openBrowserGotoInput()
		case "r":
			return m.refreshBrowserPanels()
		case "enter", "right", "l":
			return m.openActiveBrowserSelection()
		case "backspace", "left", "h":
			return m.goParentBrowserDirectory()
		case "ctrl+u":
			return m.startBrowserUpload()
		case "ctrl+d":
			return m.startBrowserDownload()
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
	panelWidth := max(18, (contentWidth-2)/2)
	browserWidth := panelWidth*2 + 2
	header := m.viewBrowserHeader(browserWidth)
	status := m.renderStatusBar(browserWidth)
	footer := m.renderBrowserFooter(browserWidth)
	headerHeight := lipgloss.Height(header)
	statusHeight := lipgloss.Height(status)
	footerHeight := lipgloss.Height(footer)
	panelHeight := max(8, contentHeight-headerHeight-statusHeight-footerHeight)
	leftPanel := m.renderBrowserPanel(m.browser.localPanel, panelWidth, panelHeight, m.browser.activePanel == domain.LocalPanel)
	rightPanel := m.renderBrowserPanel(m.browser.remotePanel, panelWidth, panelHeight, m.browser.activePanel == domain.RemotePanel)
	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "  ", rightPanel)
	return m.renderShell(shellView{
		style:   styles.App,
		width:   contentWidth,
		height:  contentHeight,
		header:  header,
		body:    body,
		status:  status,
		footer:  footer,
		overlay: m.browserOverlayView(contentWidth, contentHeight),
	})
}

func (m *Model) renderBrowserPanel(panel filePanel, width int, height int, focused bool) string {
	styles := m.theme.Styles
	style := styles.Panel
	if focused {
		style = styles.FocusedPanel
	}
	innerWidth := max(12, width-style.GetHorizontalFrameSize())
	innerHeight := max(1, height-style.GetVerticalFrameSize())
	rows := panel.rows()
	header := m.renderBrowserPanelHeader(panel, innerWidth, focused)
	lines := []string{header}
	headerLines := strings.Count(header, "\n") + 1
	contentHeight := max(1, innerHeight-headerLines)
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
		sizeText := humanSize(row.Size)
		sizeWidth := 8
		prefix := " "
		if index == panel.cursor {
			rowStyle := styles.SelectionInactive
			if focused {
				rowStyle = styles.Selection
			}
			prefix = "›"
			nameWidth := max(6, innerWidth-lipgloss.Width(prefix)-2-sizeWidth)
			line := prefix +
				lipgloss.NewStyle().Width(nameWidth).Render(truncate(name, nameWidth)) +
				" " +
				lipgloss.NewStyle().Width(sizeWidth).Align(lipgloss.Right).Render(sizeText)
			lines = append(lines, renderSizedBlock(rowStyle, innerWidth, 0, line))
			continue
		}
		nameWidth := max(6, innerWidth-lipgloss.Width(prefix)-2-sizeWidth)
		line := prefix +
			lipgloss.NewStyle().Width(nameWidth).Render(truncate(name, nameWidth)) +
			" " +
			lipgloss.NewStyle().Width(sizeWidth).Align(lipgloss.Right).Render(sizeText)
		lines = append(lines, lipgloss.NewStyle().Width(innerWidth).Render(line))
	}
	if len(rows) == 0 {
		lines = append(lines, styles.SubtleText.Render(m.translator.T("browser.empty")))
	}
	if panel.loading {
		lines = append(lines, styles.SubtleText.Render(m.translator.T("browser.loading")))
	}
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}
	return renderSizedBlock(style, width, height, strings.Join(lines, "\n"))
}

func (m *Model) renderBrowserPanelHeader(panel filePanel, width int, focused bool) string {
	styles := m.theme.Styles
	panelTitleStyle := styles.BadgeMuted
	if focused {
		panelTitleStyle = styles.BadgeAccent
	}
	title := panelTitleStyle.Render(panel.title)
	meta := ""
	if panel.filter != "" {
		metaWidth := max(0, width-lipgloss.Width(title)-1)
		if metaWidth > 0 {
			meta = styles.SubtleText.Render(truncate(m.translator.T("browser.filter_label", panel.filter), metaWidth))
		}
	}
	firstLine := title
	if meta != "" {
		firstLine = lipgloss.JoinHorizontal(lipgloss.Left, title, " ", meta)
	}
	return strings.Join([]string{
		lipgloss.NewStyle().Width(width).Render(firstLine),
		"",
	}, "\n")
}

func (m *Model) renderBrowserFooter(width int) string {
	styles := m.theme.Styles
	innerWidth := max(12, width-styles.Panel.GetHorizontalFrameSize())
	switch m.overlay {
	case overlayBrowserInput:
		title := m.translator.T("browser.path")
		if m.browser.inputMode == browserInputFilter {
			title = m.translator.T("browser.filter")
		}
		return renderSizedBlock(styles.Panel, width, 0, strings.Join([]string{
			styles.FieldLabel.Render(title),
			m.browser.input.View(),
			localizedShortcutHelpWidth(m.translator, m.theme, max(24, innerWidth),
				"enter", "shortcut.confirm",
				"esc", "shortcut.cancel",
			),
		}, "\n"))
	default:
		return renderSizedBlock(styles.Panel, width, 0,
			localizedShortcutHelpWidth(m.translator, m.theme, max(24, innerWidth),
				"enter/l", "shortcut.open",
				"tab", "shortcut.switch",
				"c-u", "shortcut.upload",
				"c-d", "shortcut.download",
				"c-p", "shortcut.palette",
				"q", "shortcut.back",
			),
		)
	}
}

func (m *Model) viewBrowserHeader(width int) string {
	styles := m.theme.Styles
	connection := styles.PageTitle.Render(m.translator.T("browser.title"))
	target := styles.SubtleText.Render(m.translator.T("browser.subtitle", m.browser.connection.Username, m.browser.connection.Host))
	active := styles.BadgeAccent.Render(m.activeBrowserPanel().title)
	paths := lipgloss.JoinHorizontal(
		lipgloss.Center,
		styles.BadgeMuted.Render(m.translator.T("browser.local")),
		" ",
		styles.SubtleText.Render(truncate(m.browser.localPanel.path, max(10, width/3))),
		"   ",
		styles.BadgeMuted.Render(m.translator.T("browser.remote")),
		" ",
		styles.SubtleText.Render(truncate(m.browser.remotePanel.path, max(10, width/3))),
	)
	return renderSizedBlock(styles.Banner, max(24, width), 0, strings.Join([]string{
		lipgloss.JoinHorizontal(lipgloss.Center, connection, "   ", target, "   ", active),
		paths,
	}, "\n"))
}

func (m *Model) browserOverlayView(contentWidth int, _ int) string {
	switch m.overlay {
	case overlayBrowserConfirm:
		return m.viewBrowserOverwriteConfirm(contentWidth)
	case overlayCommandPalette:
		return m.viewCommandPalette(contentWidth)
	default:
		return ""
	}
}

func (m *Model) viewBrowserOverwriteConfirm(contentWidth int) string {
	styles := m.theme.Styles
	dialogWidth := min(72, max(46, contentWidth-6))
	innerWidth := max(20, dialogWidth-styles.Dialog.GetHorizontalFrameSize())
	yesButton := "[ " + m.translator.T("browser.yes") + " ]"
	noButton := "[ " + m.translator.T("browser.no") + " ]"
	if m.confirm.confirmSelection {
		yesButton = styles.Selection.Render(yesButton)
	} else {
		noButton = styles.Selection.Render(noButton)
	}
	lines := []string{
		styles.PageTitle.Render(m.confirm.title),
		"",
		styles.SubtleText.Render(truncate(m.confirmSourceText(innerWidth), innerWidth)),
		styles.SubtleText.Render(truncate(m.confirmTargetText(innerWidth), innerWidth)),
		"",
		lipgloss.JoinHorizontal(lipgloss.Left, yesButton, "  ", noButton),
		"",
		localizedShortcutHelpWidth(m.translator, m.theme, innerWidth,
			"←/→", "shortcut.choose",
			"enter/y", "shortcut.confirm",
			"esc/n", "shortcut.cancel",
		),
	}
	return styles.Dialog.Width(dialogWidth).Render(strings.Join(lines, "\n"))
}

func (m *Model) clearActiveBrowserFilter() tea.Cmd {
	panel := m.activeBrowserPanel()
	if strings.TrimSpace(panel.filter) == "" {
		return nil
	}

	panel.filter = ""
	panel.cursor = clamp(panel.cursor, len(panel.rows()))
	return nil
}

func (m *Model) loadLocalCmd(pathValue string, filter string) tea.Cmd {
	m.browser.localPanel.loading = true
	m.browser.localPanel.request++
	request := m.browser.localPanel.request
	m.setInfoStatus(m.translator.T("status.loading_browser"))
	return func() tea.Msg {
		items, currentPath, err := m.services.Files.ListLocal(pathValue, "")
		return localLoadedMsg{items: items, path: currentPath, request: request, err: err}
	}
}

func (m *Model) loadRemoteCmd(pathValue string, filter string) tea.Cmd {
	m.browser.remotePanel.loading = true
	m.browser.remotePanel.request++
	request := m.browser.remotePanel.request
	m.setInfoStatus(m.translator.T("status.loading_browser"))
	return func() tea.Msg {
		if m.browser.session == nil {
			return remoteLoadedMsg{request: request, err: fmt.Errorf("browser session is not ready")}
		}
		items, currentPath, err := m.browser.session.ListRemote(pathValue)
		return remoteLoadedMsg{items: items, path: currentPath, request: request, err: err}
	}
}

func (m *Model) reloadBrowserCmd() tea.Cmd {
	return tea.Batch(
		m.loadLocalCmd(m.browser.localPanel.path, m.browser.localPanel.filter),
		m.loadRemoteCmd(m.browser.remotePanel.path, m.browser.remotePanel.filter),
	)
}

func (m *Model) reloadBrowserSelectCmd(targetPanel domain.FilePanel, selectName string) tea.Cmd {
	m.browser.localPanel.loading = true
	m.browser.localPanel.request++
	localRequest := m.browser.localPanel.request
	m.browser.remotePanel.loading = true
	m.browser.remotePanel.request++
	remoteRequest := m.browser.remotePanel.request
	return tea.Batch(
		func() tea.Msg {
			items, currentPath, err := m.services.Files.ListLocal(m.browser.localPanel.path, m.browser.localPanel.filter)
			msg := localLoadedMsg{items: items, path: currentPath, request: localRequest, err: err}
			if targetPanel == domain.LocalPanel {
				msg.selectName = selectName
			}
			return msg
		},
		func() tea.Msg {
			if m.browser.session == nil {
				return remoteLoadedMsg{request: remoteRequest, err: fmt.Errorf("browser session is not ready")}
			}
			items, currentPath, err := m.browser.session.ListRemote(m.browser.remotePanel.path)
			msg := remoteLoadedMsg{items: items, path: currentPath, request: remoteRequest, err: err}
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

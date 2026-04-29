package ui

import (
	"sshm/internal/domain"
	"sshm/internal/i18n"
	"sshm/internal/themes"
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
				m.browser.input.Blur()
				m.browser.inputItem = domain.FileEntry{}
				return m, nil
			case "enter":
				value := strings.TrimSpace(m.browser.input.Value())
				mode := m.browser.inputMode
				m.overlay = overlayNone
				m.browser.input.Blur()
				defer func() {
					m.browser.inputItem = domain.FileEntry{}
				}()
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
				switch mode {
				case browserInputGoto:
					if value == "" {
						return m, nil
					}
					return m, m.navigateBrowserPath(m.browser.activePanel, value)
				case browserInputMkdir:
					return m.submitBrowserMkdir(value)
				case browserInputRename:
					return m.submitBrowserRename(value)
				default:
					return m, nil
				}
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

		if m.hasActiveConfirm(confirmActionBrowserOverwrite) || m.hasActiveConfirm(confirmActionBrowserDelete) {
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
		case "ctrl+n":
			return m.openBrowserMkdirInput()
		case "ctrl+r":
			return m.openBrowserRenameInput()
		case "ctrl+d":
			return m.confirmBrowserDelete()
		case "r":
			return m.refreshBrowserPanels()
		case "enter", "right", "l":
			return m.openActiveBrowserSelection()
		case "backspace", "left", "h":
			return m.goParentBrowserDirectory()
		case "ctrl+u":
			return m.startBrowserUpload()
		case "ctrl+s":
			return m.startBrowserDownload()
		}
	}
	return m, nil
}

func (m *Model) viewBrowser() string {
	styles := m.styles
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
	styles := m.styles
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
	rowAreaHeight := contentHeight
	if panel.loading && rowAreaHeight > 0 {
		rowAreaHeight--
	}
	start := 0
	if rowAreaHeight > 0 && panel.cursor >= rowAreaHeight {
		start = panel.cursor - rowAreaHeight + 1
	}
	end := start + rowAreaHeight
	if end > len(rows) {
		end = len(rows)
	}
	for index, row := range rows {
		if rowAreaHeight == 0 {
			break
		}
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
		for len(lines) < headerLines+rowAreaHeight {
			lines = append(lines, "")
		}
		lines = append(lines, lipgloss.NewStyle().Width(innerWidth).Align(lipgloss.Right).Render(styles.WarningText.Render(m.translator.T("browser.loading"))))
	}
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}
	return renderSizedBlock(style, width, height, strings.Join(lines, "\n"))
}

func (m *Model) renderBrowserPanelHeader(panel filePanel, width int, focused bool) string {
	styles := m.styles
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
	secondLine := ""
	if selected, ok := panel.selected(); ok && selected.Name != ".." {
		secondLine = m.styles.SubtleText.Render(truncate(renderBrowserEntryMeta(selected), width))
	}
	return strings.Join([]string{
		lipgloss.NewStyle().Width(width).Render(firstLine),
		secondLine,
	}, "\n")
}

func (m *Model) renderBrowserFooter(width int) string {
	styles := m.styles
	innerWidth := max(12, width-styles.Panel.GetHorizontalFrameSize())
	switch m.overlay {
	case overlayBrowserInput:
		title := m.browserInputTitle()
		return renderSizedBlock(styles.Panel, width, 0, strings.Join([]string{
			styles.FieldLabel.Render(title),
			m.browser.input.View(),
			localizedShortcutHelpWidth(m.translator, m.styles, max(24, innerWidth),
				"enter", "shortcut.confirm",
				"esc", "shortcut.cancel",
			),
		}, "\n"))
	default:
		return renderSizedBlock(styles.Panel, width, 0,
			localizedShortcutHelpWidth(m.translator, m.styles, max(24, innerWidth),
				"enter/l", "shortcut.open",
				"tab", "shortcut.switch",
				"c-n", "shortcut.new_dir",
				"c-r", "shortcut.rename",
				"c-d", "shortcut.delete",
				"c-u", "shortcut.upload",
				"c-s", "shortcut.download",
				"c-p", "shortcut.palette",
				"q", "shortcut.back",
			),
		)
	}
}

func (m *Model) viewBrowserHeader(width int) string {
	styles := m.styles
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
		return m.viewBrowserConfirm(contentWidth)
	case overlayCommandPalette:
		return m.viewCommandPalette(contentWidth)
	default:
		return ""
	}
}

func (m *Model) viewBrowserConfirm(contentWidth int) string {
	styles := m.styles
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
	}
	if strings.TrimSpace(m.confirm.description) != "" {
		lines = append(lines, styles.SubtleText.Render(truncate(m.confirm.description, innerWidth)), "")
	}
	if strings.TrimSpace(m.confirm.sourcePath) != "" {
		lines = append(lines, styles.SubtleText.Render(truncate(m.confirmSourceText(innerWidth), innerWidth)))
	}
	if strings.TrimSpace(m.confirm.targetPath) != "" {
		lines = append(lines, styles.SubtleText.Render(truncate(m.confirmTargetText(innerWidth), innerWidth)))
	}
	lines = append(lines,
		"",
		lipgloss.JoinHorizontal(lipgloss.Left, yesButton, "  ", noButton),
		"",
		localizedShortcutHelpWidth(m.translator, m.styles, innerWidth,
			"←/→", "shortcut.choose",
			"enter/y", "shortcut.confirm",
			"esc/n", "shortcut.cancel",
		),
	)
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

func (m *Model) activeBrowserPanel() *filePanel {
	if m.browser.activePanel == domain.LocalPanel {
		return &m.browser.localPanel
	}
	return &m.browser.remotePanel
}

func newBrowserState(translator *i18n.Translator, styles themes.Styles) browserState {
	input := newInput(styles, "", 48)
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

func (m *Model) browserInputTitle() string {
	switch m.browser.inputMode {
	case browserInputFilter:
		return m.translator.T("browser.filter")
	case browserInputMkdir:
		return m.translator.T("browser.mkdir")
	case browserInputRename:
		return m.translator.T("browser.rename")
	default:
		return m.translator.T("browser.path")
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

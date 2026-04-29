package ui

import (
	"fmt"
	"sshm/internal/buildinfo"
	"sshm/internal/domain"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) updateHome(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if next, cmd, handled := m.handleHomeOverlayKey(keyMsg); handled {
			return next, cmd
		}
		if m.home.connecting {
			switch keyMsg.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			}
			return m, nil
		}

		if m.home.searchMode {
			switch keyMsg.String() {
			case "esc":
				m.home.searchMode = false
				m.home.searchInput.Blur()
				m.setInfoStatus(m.translator.T("status.search_ready"))
				return m, nil
			case "enter":
				m.home.searchMode = false
				m.home.searchInput.Blur()
				if strings.TrimSpace(m.home.search) == "" {
					m.setInfoStatus(m.translator.T("status.search_cleared"))
				} else {
					m.setInfoStatus(m.translator.T("status.filtered_connections", len(m.home.connections)))
				}
				return m, nil
			}

			var cmd tea.Cmd
			before := m.home.searchInput.Value()
			m.home.searchInput, cmd = m.home.searchInput.Update(keyMsg)
			after := strings.TrimSpace(m.home.searchInput.Value())
			if before != after {
				m.home.search = after
				m.home.selected = 0
				return m, tea.Batch(cmd, m.loadConnectionsCmd())
			}
			return m, cmd
		}

		switch keyMsg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case ":", "ctrl+p":
			return m.openCommandPalette()
		case "?":
			m.clearStaleErrorStatus()
			m.overlay = overlayHelp
			return m, nil
		case "/":
			return m.openHomeSearch()
		case "esc":
			return m.clearHomeFilters()
		case "up", "k":
			if m.home.selected > 0 {
				m.clearStaleErrorStatus()
				m.home.selected--
			}
			return m, nil
		case "down", "j":
			if m.home.selected < len(m.home.connections)-1 {
				m.clearStaleErrorStatus()
				m.home.selected++
			}
			return m, nil
		case "ctrl+n":
			return m.openHomeCreateForm()
		case "ctrl+e":
			return m.openCurrentConnectionEdit()
		case "ctrl+d":
			return m.confirmCurrentConnectionDelete()
		case "g":
			return m.openHomeGroupFilter()
		case "ctrl+g":
			return m.openHomeMoveGroup()
		case "i":
			return m.openImportPage()
		case "enter":
			return m.openCurrentConnectionShell()
		case "ctrl+o":
			return m.openCurrentConnectionBrowser()
		}
	}
	return m, nil
}

func (m *Model) viewHome() string {
	width := m.width
	if width == 0 {
		width = 110
	}
	height := m.height
	if height == 0 {
		height = 34
	}
	contentWidth := max(36, width-4)
	contentHeight := max(16, height-2)
	mainWidth, listWidth, detailWidth := m.homePanelWidths(contentWidth)
	header := m.viewHomeHeader()
	search := m.viewHomeSearch(mainWidth)
	status := m.renderStatus()
	footer := m.viewHomeFooter(mainWidth)
	bodyHeight := max(8, contentHeight-lipgloss.Height(header)-lipgloss.Height(search)-lipgloss.Height(status)-lipgloss.Height(footer))
	body := strings.Join([]string{
		search,
		m.viewHomeBody(listWidth, detailWidth, bodyHeight),
	}, "\n")
	return m.renderShell(shellView{
		style:   m.styles.App,
		width:   contentWidth,
		height:  contentHeight,
		header:  header,
		body:    body,
		status:  status,
		footer:  footer,
		overlay: m.homeOverlayView(contentWidth, contentHeight),
	})
}

func (m *Model) viewHomeBody(listWidth int, detailWidth int, height int) string {
	styles := m.styles
	leftInnerWidth := max(12, listWidth-styles.FocusedPanel.GetHorizontalFrameSize())
	leftInnerHeight := max(1, height-styles.FocusedPanel.GetVerticalFrameSize())
	rightInnerWidth := max(12, detailWidth-styles.Panel.GetHorizontalFrameSize())
	rightInnerHeight := max(1, height-styles.Panel.GetVerticalFrameSize())

	left := renderSizedBlock(styles.FocusedPanel, listWidth, height, m.viewConnectionList(leftInnerWidth, leftInnerHeight))
	right := renderSizedBlock(styles.Panel, detailWidth, height, m.viewConnectionDetail(rightInnerWidth, rightInnerHeight))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
}

func (m *Model) viewConnectionList(width int, height int) string {
	styles := m.styles
	title := m.viewConnectionListTitle()
	if len(m.home.connections) == 0 {
		empty := []string{
			title,
			"",
			styles.SubtleText.Render(m.translator.T("home.empty")),
			"",
			m.translator.T("home.empty_action", styles.Keycap.Render("c-n")),
		}
		if strings.TrimSpace(m.home.search) != "" {
			empty = []string{
				title,
				"",
				styles.SubtleText.Render(m.translator.T("status.no_matches")),
				"",
				m.translator.T("home.no_match_action", styles.Keycap.Render("esc"), styles.Keycap.Render("/")),
			}
		}
		return strings.Join(empty, "\n")
	}
	contentWidth := max(24, width)
	lines := []string{title, ""}
	rowHeight := 2
	visible := max(1, (height-2)/rowHeight)
	start := 0
	if m.home.selected >= visible {
		start = m.home.selected - visible + 1
	}
	if start+visible > len(m.home.connections) {
		start = max(0, len(m.home.connections)-visible)
	}
	end := min(len(m.home.connections), start+visible)

	for index := start; index < end; index++ {
		lines = append(lines, m.renderConnectionRow(m.home.connections[index], index == m.home.selected, contentWidth))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) viewHomeHeader() string {
	styles := m.styles
	info := buildinfo.Info()
	width := m.width
	if width == 0 {
		width = 110
	}
	contentWidth := max(36, width-4)
	mainWidth, _, _ := m.homePanelWidths(contentWidth)
	title := lipgloss.JoinHorizontal(
		lipgloss.Center,
		styles.Badge.Render("SSH"),
		" ",
		styles.PageTitle.Render(m.translator.T("home.title")),
	)
	meta := lipgloss.JoinHorizontal(
		lipgloss.Center,
		styles.BadgeMuted.Render(info.Version),
		" ",
		styles.SubtleText.Render("by "+info.Author),
	)
	content := lipgloss.JoinHorizontal(
		lipgloss.Left,
		title,
		"   ",
		meta,
		"   ",
		m.homeScopeBadge(),
		" ",
		m.homeCountBadge(),
	)
	return renderSizedBlock(styles.Banner, max(24, mainWidth), 0, content)
}

func (m *Model) viewHomeSearch(width int) string {
	styles := m.styles
	style := styles.SearchBox
	if m.home.searchMode {
		style = styles.SearchBoxFocused
	}
	boxInnerWidth := max(12, width-style.GetHorizontalFrameSize())
	m.home.searchInput.PromptStyle = styles.SubtleText
	m.home.searchInput.PlaceholderStyle = styles.SubtleText
	m.home.searchInput.TextStyle = m.homeSearchValueStyle()
	m.home.searchInput.Width = boxInnerWidth
	return renderSizedBlock(style, max(28, width), 0, m.home.searchInput.View())
}

func (m *Model) homeSearchValueStyle() lipgloss.Style {
	if !m.home.searchMode && strings.TrimSpace(m.home.searchInput.Value()) != "" {
		return m.styles.SearchValueBlurred
	}
	return lipgloss.NewStyle()
}

func (m *Model) viewHomeHelp() string {
	styles := m.styles
	lines := []string{
		styles.PageTitle.Render(m.translator.T("home.help_title")),
		"",
	}
	for _, shortcut := range homeShortcuts() {
		lines = append(lines, m.actionRow(shortcut.key, shortcut.helpKey))
	}
	lines = append(lines, "", styles.HelpText.Render(m.translator.T("home.help_close")))
	return styles.Dialog.Width(40).Render(strings.Join(lines, "\n"))
}

func (m *Model) viewHomeFooter(width int) string {
	styles := m.styles
	items := make([]string, 0, len(homeFooterShortcuts())*2)
	for _, shortcut := range homeFooterShortcuts() {
		items = append(items, shortcut.key, shortcut.footerKey)
	}
	innerWidth := max(24, width-styles.Panel.GetHorizontalFrameSize())
	content := localizedShortcutHelpWidth(m.translator, m.styles, innerWidth, items...)
	return renderSizedBlock(styles.Panel, max(26, width), 0, content)
}

type homeShortcut struct {
	key       string
	helpKey   string
	footerKey string
}

func homeShortcuts() []homeShortcut {
	return []homeShortcut{
		{":/c-p", "home.help_palette", "home.footer_palette"},
		{"j/k", "home.help_move", "home.footer_move"},
		{"enter", "home.help_open_shell", "home.footer_shell"},
		{"c-o", "home.help_open_files", "home.footer_files"},
		{"c-n", "home.help_create", "home.footer_add"},
		{"c-e", "home.help_edit", "home.footer_edit"},
		{"c-d", "home.help_delete", "home.footer_delete"},
		{"g", "home.help_groups", "home.footer_groups"},
		{"c-g", "home.help_move_group", "home.footer_move_group"},
		{"i", "home.help_import", "home.footer_import"},
		{"/", "home.help_start_search", "home.footer_search"},
		{"esc", "home.help_search_clear", "home.footer_clear"},
		{"q/c-c", "home.help_quit", "home.footer_quit"},
	}
}

func homeFooterShortcuts() []homeShortcut {
	return []homeShortcut{
		{"enter", "home.help_open_shell", "home.footer_shell"},
		{"c-o", "home.help_open_files", "home.footer_files"},
		{"c-p", "home.help_palette", "home.footer_palette"},
		{"/", "home.help_start_search", "home.footer_search"},
		{"?", "home.help_help", "home.footer_help"},
		{"q", "home.help_quit", "home.footer_quit"},
	}
}

func (m *Model) renderStatus() string {
	return m.renderStatusBar(m.statusBarWidth())
}

func (m *Model) renderStatusBar(width int) string {
	bar, content := m.renderStatusLine(max(1, width))
	return renderSizedBlock(bar, width, 0, content)
}

func (m *Model) renderStatusLine(width int) (lipgloss.Style, string) {
	styles := m.styles
	label := styles.BadgeMuted.Render("INFO")
	bar := styles.StatusBar
	textWidth := max(12, width-bar.GetHorizontalFrameSize()-lipgloss.Width(label)-1)
	content := lipgloss.JoinHorizontal(lipgloss.Left, label, " ", styles.SubtleText.Render(truncate(m.status, textWidth)))
	if m.err != nil {
		label = styles.Badge.Render("ERR")
		bar = styles.StatusBarError
		textWidth = max(12, width-bar.GetHorizontalFrameSize()-lipgloss.Width(label)-1)
		content = lipgloss.JoinHorizontal(lipgloss.Left, label, " ", styles.ErrorText.Render(truncate(m.status, textWidth)))
	}
	if m.err == nil && m.statusSuccess {
		label = styles.BadgeAccent.Render("DONE")
		bar = styles.StatusBarSuccess
		textWidth = max(12, width-bar.GetHorizontalFrameSize()-lipgloss.Width(label)-1)
		content = lipgloss.JoinHorizontal(lipgloss.Left, label, " ", styles.SuccessText.Render(truncate(m.status, textWidth)))
	}
	return bar, content
}

func (m *Model) currentConnection() *Connection {
	if len(m.home.connections) == 0 || m.home.selected < 0 || m.home.selected >= len(m.home.connections) {
		return nil
	}
	return &m.home.connections[m.home.selected]
}

func (m *Model) loadConnectionsCmd() tea.Cmd {
	query := strings.TrimSpace(m.home.search)
	return func() tea.Msg {
		items, err := m.services.Connections.ListWithOptions(domain.ConnectionListOptions{
			Query:   query,
			Scope:   m.home.listScope,
			GroupID: m.home.listGroupID,
		})
		return connectionsLoadedMsg{items: items, err: err}
	}
}

func (m *Model) deleteConnectionCmd(id int64) tea.Cmd {
	return func() tea.Msg {
		err := m.services.Connections.Delete(id)
		if err != nil {
			return opDoneMsg{err: err}
		}
		return opDoneMsg{status: m.translator.T("status.connection_deleted"), success: true, reloadConnections: true}
	}
}

func (m *Model) startHomeProbe(conn Connection, action homeProbeAction) (tea.Model, tea.Cmd) {
	m.home.connecting = true
	m.setInfoStatus(m.homeProbePendingStatus(action, conn.Name))
	return m, m.probeConnectionCmd(conn, action)
}

func (m *Model) homeProbePendingStatus(action homeProbeAction, connectionName string) string {
	switch action {
	case homeProbeBrowser:
		return m.translator.T("status.connecting_browser", connectionName)
	default:
		return m.translator.T("status.connecting_shell", connectionName)
	}
}

func (m *Model) probeConnectionCmd(conn Connection, action homeProbeAction) tea.Cmd {
	return func() tea.Msg {
		session, err := m.services.Sessions.OpenSession(conn.ID)
		return homeProbeDoneMsg{
			action:         action,
			connectionName: conn.Name,
			connection:     conn,
			session:        session,
			err:            err,
		}
	}
}

func (m *Model) actionRow(key string, labelKey string) string {
	styles := m.styles
	return styles.Keycap.Render(key) + " " + styles.SubtleText.Render(m.translator.T(labelKey))
}

func (m *Model) renderConnectionRow(conn Connection, selected bool, width int) string {
	styles := m.styles
	rowStyle := styles.Text.Copy().Padding(0, 1)
	nameStyle := styles.ListItemTitle
	if selected {
		rowStyle = styles.Selection.Copy().Padding(0, 1)
		nameStyle = m.styles.SelectionTitle
	}
	nameLine := nameStyle.Render(truncate(conn.Name, max(8, width-2)))
	metaLine := styles.ListItemMeta.Render(truncate(m.connectionListMeta(conn), max(8, width-2)))
	content := strings.Join([]string{nameLine, metaLine}, "\n")
	if selected {
		return renderSizedBlock(rowStyle, width, 0, content)
	}
	return renderSizedBlock(rowStyle, width, 0, content)
}

func (m *Model) connectionListMeta(conn Connection) string {
	return fmt.Sprintf("%s@%s", conn.Username, conn.Host)
}

func (m *Model) viewConnectionListTitle() string {
	styles := m.styles
	title := styles.SectionTitle.Render(m.translator.T("home.connections"))
	if label := m.currentScopeLabel(); label != "" {
		return lipgloss.JoinHorizontal(lipgloss.Left, title, " ", styles.BadgeMuted.Render("("+label+")"))
	}
	return title
}

func (m *Model) connectionListScopeStyle() lipgloss.Style {
	if m.home.listScope == domain.ConnectionListScopeAll {
		return m.styles.SectionTitle
	}
	return m.styles.GroupScope
}

func (m *Model) currentScopeLabel() string {
	switch m.home.listScope {
	case domain.ConnectionListScopeUngrouped:
		return m.translator.T("group.ungrouped")
	case domain.ConnectionListScopeGroup:
		return m.home.listGroup
	default:
		return m.translator.T("group.all")
	}
}

func (m *Model) viewConnectionDetail(width int, height int) string {
	styles := m.styles
	title := styles.SectionTitle.Render(m.translator.T("home.details"))
	conn := m.currentConnection()
	if conn == nil {
		return strings.Join([]string{
			title,
			"",
			styles.SubtleText.Render(m.translator.T("home.empty")),
			styles.MutedText.Render(m.translator.T("home.empty_action", styles.Keycap.Render("c-n"))),
		}, "\n")
	}
	hostInfo := fmt.Sprintf("%s@%s:%d", conn.Username, conn.Host, conn.Port)
	lastUsed := m.translator.T("home.never")
	if conn.LastUsedAt != nil {
		lastUsed = m.relativeTime(*conn.LastUsedAt)
	}
	description := defaultString(conn.Description, "—")
	lines := []string{
		title,
		"",
		styles.PageTitle.Width(width).Render(wrapText(conn.Name, width)),
		styles.SubtleText.Width(width).Render(truncate(hostInfo, width)),
		"",
	}
	lines = append(lines,
		m.detailLine("home.detail_auth", m.authTypeLabel(conn.AuthType), width),
		m.detailLine("home.detail_group", defaultString(m.connectionGroupLabel(*conn), m.translator.T("group.ungrouped")), width),
		m.detailLine("form.host", conn.Host, width),
		m.detailLine("form.port", fmt.Sprintf("%d", conn.Port), width),
		m.detailLine("form.username", conn.Username, width),
		m.detailLine("home.table_address", hostInfo, width),
		m.detailLine("home.table_last_used", lastUsed, width),
	)
	if conn.AuthType == domain.AuthTypePrivateKey {
		lines = append(lines, m.detailLine("form.key_path", defaultString(conn.PrivateKeyPath, "—"), width))
	}
	lines = append(lines,
		"",
		styles.SectionTitle.Render(m.translator.T("home.table_description")),
		styles.MutedText.Width(width).Render(wrapText(description, width)),
	)
	return styles.Text.Copy().Width(width).Height(max(height, 1)).Align(lipgloss.Left, lipgloss.Top).Render(strings.Join(lines, "\n"))
}

func (m *Model) connectionGroupLabel(conn Connection) string {
	if conn.GroupID == nil {
		return m.translator.T("group.ungrouped")
	}
	return conn.GroupName
}

func (m *Model) homePanelWidths(contentWidth int) (int, int, int) {
	mainWidth := contentWidth
	listWidth := max(26, (mainWidth-2)*42/100)
	detailWidth := max(28, mainWidth-listWidth-2)
	return mainWidth, listWidth, detailWidth
}

func (m *Model) homeScopeBadge() string {
	return m.styles.BadgeMuted.Render(m.currentScopeLabel())
}

func (m *Model) homeCountBadge() string {
	return m.styles.BadgeAccent.Render(fmt.Sprintf("%d", len(m.home.connections)))
}

func (m *Model) statusBarWidth() int {
	width := m.width
	if width == 0 {
		width = 110
	}
	contentWidth := max(36, width-4)
	if m.page == pageHome {
		mainWidth, _, _ := m.homePanelWidths(contentWidth)
		return mainWidth
	}
	return contentWidth
}

func (m *Model) detailLine(labelKey string, value string, width int) string {
	styles := m.styles
	labelWidth := min(10, max(6, width/3))
	valueWidth := max(8, width-labelWidth-2)
	label := styles.SubtleText.Width(labelWidth).Render(m.translator.T(labelKey))
	renderedValue := styles.Text.Copy().Width(valueWidth).MaxWidth(valueWidth).Render(wrapText(defaultString(value, "—"), valueWidth))
	return lipgloss.JoinHorizontal(lipgloss.Top, label, "  ", renderedValue)
}

func (m *Model) authTypeLabel(authType domain.AuthType) string {
	if authType == domain.AuthTypePrivateKey {
		return m.translator.T("home.auth_private_key")
	}
	return m.translator.T("home.auth_password")
}

func wrapText(value string, width int) string {
	value = strings.TrimSpace(value)
	if width <= 0 || lipgloss.Width(value) <= width {
		return value
	}
	words := strings.Fields(value)
	if len(words) == 0 {
		return wrapRunes(value, width)
	}
	lines := []string{}
	current := ""
	for _, word := range words {
		if lipgloss.Width(word) > width {
			if current != "" {
				lines = append(lines, current)
				current = ""
			}
			lines = append(lines, strings.Split(wrapRunes(word, width), "\n")...)
			continue
		}
		if current == "" {
			current = word
			continue
		}
		if lipgloss.Width(current+" "+word) <= width {
			current += " " + word
			continue
		}
		lines = append(lines, current)
		current = word
	}
	if current != "" {
		lines = append(lines, current)
	}
	return strings.Join(lines, "\n")
}

func wrapRunes(value string, width int) string {
	lines := []string{}
	current := ""
	for _, item := range value {
		next := current + string(item)
		if current != "" && lipgloss.Width(next) > width {
			lines = append(lines, current)
			current = string(item)
			continue
		}
		current = next
	}
	if current != "" {
		lines = append(lines, current)
	}
	return strings.Join(lines, "\n")
}

func (m *Model) relativeTime(value time.Time) string {
	now := time.Now()
	if value.After(now) {
		value = now
	}
	diff := now.Sub(value)
	switch {
	case diff < time.Minute:
		return m.translator.T("home.just_now")
	case diff < time.Hour:
		return m.translator.T("home.minutes_ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return m.translator.T("home.hours_ago", int(diff.Hours()))
	case diff < 7*24*time.Hour:
		return m.translator.T("home.days_ago", int(diff.Hours()/24))
	default:
		return value.Local().Format("01-02")
	}
}

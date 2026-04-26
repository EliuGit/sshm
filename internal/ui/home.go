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
		if m.overlay == overlayDelete {
			switch keyMsg.String() {
			case "y", "enter":
				id := m.deleteTarget
				m.overlay = overlayNone
				m.deleteTarget = 0
				return m, m.deleteConnectionCmd(id)
			case "n", "esc", "q":
				m.overlay = overlayNone
				m.deleteTarget = 0
				m.setInfoStatus(m.translator.T("status.delete_cancelled"))
				return m, nil
			}
			return m, nil
		}
		if m.overlay == overlayHelp {
			switch keyMsg.String() {
			case "?", "esc", "q", "enter":
				m.overlay = overlayNone
			}
			return m, nil
		}
		if m.overlay == overlayGroup {
			return m.updateGroupPanel(keyMsg)
		}

		if m.searchMode {
			switch keyMsg.String() {
			case "esc":
				m.searchMode = false
				m.searchInput.Blur()
				m.setInfoStatus(m.translator.T("status.search_ready"))
				return m, nil
			case "enter":
				m.searchMode = false
				m.searchInput.Blur()
				if strings.TrimSpace(m.search) == "" {
					m.setInfoStatus(m.translator.T("status.search_cleared"))
				} else {
					m.setInfoStatus(m.translator.T("status.filtered_connections", len(m.connections)))
				}
				return m, nil
			}

			var cmd tea.Cmd
			before := m.searchInput.Value()
			m.searchInput, cmd = m.searchInput.Update(keyMsg)
			after := strings.TrimSpace(m.searchInput.Value())
			if before != after {
				m.search = after
				m.selected = 0
				return m, tea.Batch(cmd, m.loadConnectionsCmd())
			}
			return m, cmd
		}

		switch keyMsg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "?":
			m.overlay = overlayHelp
			return m, nil
		case "/":
			m.searchMode = true
			m.searchInput.Focus()
			m.setInfoStatus(m.translator.T("status.type_to_filter"))
			return m, nil
		case "esc":
			if strings.TrimSpace(m.search) != "" {
				m.search = ""
				m.searchInput.SetValue("")
				m.selected = 0
				m.setInfoStatus(m.translator.T("status.search_cleared"))
				return m, m.loadConnectionsCmd()
			}
			if m.listScope != domain.ConnectionListScopeAll {
				m.listScope = domain.ConnectionListScopeAll
				m.listGroupID = 0
				m.listGroup = ""
				m.selected = 0
				m.setInfoStatus(m.translator.T("status.group_filter_cleared"))
				return m, m.loadConnectionsCmd()
			}
			return m, nil
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
			return m, nil
		case "down", "j":
			if m.selected < len(m.connections)-1 {
				m.selected++
			}
			return m, nil
		case "ctrl+n":
			m.page = pageForm
			m.form = newFormState(nil, m.translator, m.defaultPrivateKeyPath, m.theme)
			return m, tea.ClearScreen
		case "ctrl+e":
			if conn := m.currentConnection(); conn != nil {
				m.page = pageForm
				m.form = newFormState(conn, m.translator, m.defaultPrivateKeyPath, m.theme)
				return m, tea.ClearScreen
			}
			return m, nil
		case "ctrl+d":
			if conn := m.currentConnection(); conn != nil {
				m.overlay = overlayDelete
				m.deleteTarget = conn.ID
			}
			return m, nil
		case "g":
			m.groups = newGroupPanelState(m.translator, m.theme)
			m.groups.mode = groupPanelFilter
			m.overlay = overlayGroup
			return m, m.loadGroupsCmd()
		case "ctrl+g":
			if conn := m.currentConnection(); conn != nil {
				m.groups = newGroupPanelState(m.translator, m.theme)
				m.groups.mode = groupPanelMove
				m.groups.targetID = conn.ID
				m.overlay = overlayGroup
				return m, m.loadGroupsCmd()
			}
			return m, nil
		case "i":
			m.page = pageImport
			m.imports = newImportState(m.translator, m.theme)
			return m, tea.ClearScreen
		case "enter":
			if conn := m.currentConnection(); conn != nil {
				return m, func() tea.Msg {
					return shellReadyMsg{connectionID: conn.ID}
				}
			}
			return m, nil
		case "ctrl+o":
			if conn := m.currentConnection(); conn != nil {
				m.page = pageBrowser
				m.browser = newBrowserState(m.translator, m.theme)
				m.browser.connectionID = conn.ID
				m.browser.connection = *conn
				m.browser.localPanel.path = m.startupDir
				m.browser.remotePanel.path = "."
				m.browser.activePanel = domain.LocalPanel
				m.browser.localPanel.loading = true
				m.browser.remotePanel.loading = true
				return m, tea.Batch(
					tea.ClearScreen,
					m.loadLocalCmd(m.browser.localPanel.path, m.browser.localPanel.filter),
					m.loadRemoteCmd(conn.ID, m.browser.remotePanel.path, m.browser.remotePanel.filter),
				)
			}
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) viewHome() string {
	styles := m.theme.Styles
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
	bodyHeight := max(8, (contentHeight-7)*3/4)
	body := m.viewHomeBody(listWidth, detailWidth, bodyHeight)

	header := m.viewHomeHeader()
	search := m.viewHomeSearch(mainWidth)
	footer := m.viewHomeFooter(contentWidth)
	chunks := []string{
		header,
		search,
		body,
		footer,
	}
	view := strings.Join([]string{
		chunks[0],
		"",
		chunks[1],
		chunks[2],
		"",
		chunks[3],
	}, "\n")
	if m.overlay == overlayDelete {
		dialog := styles.Dialog.Width(44).Render(strings.Join([]string{
			styles.PageTitle.Render(m.translator.T("home.delete_title")),
			"",
			styles.SubtleText.Render(m.translator.T("home.delete_desc")),
			"",
			styles.MutedText.Render(m.translator.T("home.delete_keys", styles.Keycap.Render("esc"), styles.Keycap.Render("enter"), styles.Keycap.Render("y"))),
		}, "\n"))
		view = overlayCenter(view, dialog, contentWidth, contentHeight)
	}
	if m.overlay == overlayHelp {
		view = overlayCenter(view, m.viewHomeHelp(), contentWidth, contentHeight)
	}
	if m.overlay == overlayGroup {
		view = overlayCenter(view, m.viewGroupPanel(), contentWidth, contentHeight)
	}
	return view
}

func (m *Model) viewHomeBody(listWidth int, detailWidth int, height int) string {
	styles := m.theme.Styles
	left := styles.FocusedPanel.Width(listWidth).Height(height).Render(m.viewConnectionList(listWidth-2, height-2))
	right := styles.Panel.Width(detailWidth).Height(height).Render(m.viewConnectionDetail(detailWidth-2, height-2))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
}

func (m *Model) viewConnectionList(width int, height int) string {
	styles := m.theme.Styles
	title := m.viewConnectionListTitle()
	if len(m.connections) == 0 {
		empty := []string{
			title,
			"",
			styles.SubtleText.Render(m.translator.T("home.empty")),
			"",
			m.translator.T("home.empty_action", styles.Keycap.Render("c-n")),
		}
		if strings.TrimSpace(m.search) != "" {
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
	if m.selected >= visible {
		start = m.selected - visible + 1
	}
	if start+visible > len(m.connections) {
		start = max(0, len(m.connections)-visible)
	}
	end := min(len(m.connections), start+visible)

	for index := start; index < end; index++ {
		lines = append(lines, m.renderConnectionRow(m.connections[index], index == m.selected, contentWidth))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) viewHomeHeader() string {
	styles := m.theme.Styles
	info := buildinfo.Info()
	title := styles.PageTitle.Render(m.translator.T("home.title"))
	meta := styles.SubtleText.Render(fmt.Sprintf("  %s  by %s", info.Version, info.Author))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, meta)
}

func (m *Model) viewHomeSearch(width int) string {
	styles := m.theme.Styles
	style := styles.SearchBox
	if m.searchMode {
		style = styles.SearchBoxFocused
	}
	m.searchInput.PromptStyle = styles.SubtleText
	m.searchInput.PlaceholderStyle = styles.SubtleText
	m.searchInput.TextStyle = m.homeSearchValueStyle()
	m.searchInput.Width = max(12, width-6)
	return style.Width(max(28, width+2)).Render(m.searchInput.View())
}

func (m *Model) homeSearchValueStyle() lipgloss.Style {
	if !m.searchMode && strings.TrimSpace(m.searchInput.Value()) != "" {
		return m.theme.Styles.SearchValueBlurred
	}
	return lipgloss.NewStyle()
}

func (m *Model) viewHomeHelp() string {
	styles := m.theme.Styles
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
	items := make([]string, 0, len(homeShortcuts())*2)
	for _, shortcut := range homeShortcuts() {
		items = append(items, shortcut.key, shortcut.footerKey)
	}
	return localizedShortcutHelpWidth(m.translator, m.theme, max(24, width-2), items...)
}

type homeShortcut struct {
	key       string
	helpKey   string
	footerKey string
}

func homeShortcuts() []homeShortcut {
	return []homeShortcut{
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

func (m *Model) renderStatus() string {
	styles := m.theme.Styles
	if m.err != nil {
		return styles.ErrorText.Render(m.status)
	}
	if m.statusSuccess {
		return styles.SuccessText.Render(m.status)
	}
	return styles.SubtleText.Render(m.status)
}

func (m *Model) currentConnection() *Connection {
	if len(m.connections) == 0 || m.selected < 0 || m.selected >= len(m.connections) {
		return nil
	}
	return &m.connections[m.selected]
}

func (m *Model) loadConnectionsCmd() tea.Cmd {
	query := strings.TrimSpace(m.search)
	return func() tea.Msg {
		items, err := m.services.Connections.ListWithOptions(domain.ConnectionListOptions{
			Query:   query,
			Scope:   m.listScope,
			GroupID: m.listGroupID,
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

func (m *Model) actionRow(key string, labelKey string) string {
	styles := m.theme.Styles
	return styles.Keycap.Render(key) + " " + styles.SubtleText.Render(m.translator.T(labelKey))
}

func (m *Model) renderConnectionRow(conn Connection, selected bool, width int) string {
	styles := m.theme.Styles
	nameLine := styles.ListItemTitle.Render(truncate(conn.Name, width))
	metaLine := styles.ListItemMeta.Render(truncate(m.connectionListMeta(conn), width))
	if selected {
		selectedText := styles.Selection.Copy().Width(width)
		return selectedText.Padding(0, 1).Render(strings.Join([]string{
			truncate(conn.Name, width),
			truncate(m.connectionListMeta(conn), width),
		}, "\n"))
	}
	return styles.Text.Copy().Width(width).Padding(0, 1).Render(strings.Join([]string{nameLine, metaLine}, "\n"))
}

func (m *Model) connectionListMeta(conn Connection) string {
	return fmt.Sprintf("%s@%s", conn.Username, conn.Host)
}

func (m *Model) viewConnectionListTitle() string {
	styles := m.theme.Styles
	title := styles.SectionTitle.Render(m.translator.T("home.connections"))
	if label := m.currentScopeLabel(); label != "" {
		return lipgloss.JoinHorizontal(lipgloss.Left, title, " ", m.connectionListScopeStyle().Render("("+label+")"))
	}
	return title
}

func (m *Model) connectionListScopeStyle() lipgloss.Style {
	if m.listScope == domain.ConnectionListScopeAll {
		return m.theme.Styles.SectionTitle
	}
	return m.theme.Styles.GroupScope
}

func (m *Model) currentScopeLabel() string {
	switch m.listScope {
	case domain.ConnectionListScopeUngrouped:
		return m.translator.T("group.ungrouped")
	case domain.ConnectionListScopeGroup:
		return m.listGroup
	default:
		return m.translator.T("group.all")
	}
}

func (m *Model) viewConnectionDetail(width int, height int) string {
	styles := m.theme.Styles
	title := styles.SectionTitle.Render(m.translator.T("home.details"))
	conn := m.currentConnection()
	if conn == nil {
		return strings.Join([]string{title, "", styles.SubtleText.Render(m.translator.T("home.empty"))}, "\n")
	}
	hostInfo := fmt.Sprintf("%s@%s:%d", conn.Username, conn.Host, conn.Port)
	lastUsed := m.translator.T("home.never")
	if conn.LastUsedAt != nil {
		lastUsed = m.relativeTime(*conn.LastUsedAt)
	}
	description := defaultString(conn.Description, "—")
	lines := []string{title, "",
		styles.PageTitle.Width(width).Render(wrapText(conn.Name, width)),
		"",
	}
	lines = append(lines,
		m.detailLine("home.detail_group", defaultString(m.connectionGroupLabel(*conn), "—"), width),
		m.detailLine("form.host", conn.Host, width),
		m.detailLine("form.port", fmt.Sprintf("%d", conn.Port), width),
		m.detailLine("form.username", conn.Username, width),
		m.detailLine("home.table_address", hostInfo, width),
		m.detailLine("home.detail_auth", m.authTypeLabel(conn.AuthType), width),
	)
	if conn.AuthType == domain.AuthTypePrivateKey {
		lines = append(lines, m.detailLine("form.key_path", defaultString(conn.PrivateKeyPath, "—"), width))
	}
	lines = append(lines,
		m.detailLine("home.table_last_used", lastUsed, width),
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
	listWidth := max(24, contentWidth*4/10)
	detailWidth := max(24, contentWidth*4/10)
	mainWidth := min(contentWidth, listWidth+2+detailWidth)
	return mainWidth, listWidth, detailWidth
}

func (m *Model) detailLine(labelKey string, value string, width int) string {
	styles := m.theme.Styles
	labelWidth := min(12, max(8, width/3))
	valueWidth := max(8, width-labelWidth-2)
	label := styles.SubtleText.Width(labelWidth).Render(m.translator.T(labelKey))
	renderedValue := styles.Text.Copy().Width(valueWidth).Render(wrapText(defaultString(value, "—"), valueWidth))
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

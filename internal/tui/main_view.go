package tui

import (
	"strings"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	windowTitle     = "SSHM v1.0"
	minWidth        = 80
	minHeight       = 20
	nameColumnWidth = 24
	userColumnWidth = 16
)

var (
	backgroundColor = lipgloss.Color("#282C34")
	textColor       = lipgloss.Color("#D8DEE9")
	accentStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#00b3e4")).Bold(true)
	mutedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B6F78"))
	borderStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6068"))
	labelStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#babbbf"))
	selectedStyle   = lipgloss.NewStyle().Background(lipgloss.Color("#007ea1")).Foreground(lipgloss.Color("#FFFFFF"))
	plainStyle      = lipgloss.NewStyle().Foreground(textColor)
)

func newSearchInput() textinput.Model {
	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = "搜索名称、主机、用户、备注..."
	input.CharLimit = 200
	styles := input.Styles()
	styles.Focused.Text = plainStyle
	styles.Focused.Placeholder = mutedStyle
	styles.Blurred.Text = plainStyle
	styles.Blurred.Placeholder = mutedStyle
	styles.Cursor.Color = lipgloss.Color("#7F6DF2")
	input.SetStyles(styles)
	return input
}

// View 根据当前终端尺寸生成完整画面，并启用备用屏幕模式。
func (m model) View() tea.View {
	view := tea.NewView(m.render())
	view.AltScreen = true
	view.WindowTitle = windowTitle
	view.BackgroundColor = backgroundColor
	view.ForegroundColor = textColor
	return view
}

func (m model) render() string {
	if m.width < minWidth || m.height < minHeight {
		return "终端窗口过小，请至少调整为 80×20"
	}
	input := m.searchInput
	if !m.searchReady {
		input = newSearchInput()
	}
	base := renderPanelWithConnections(m.width, m.height, m.selected, m.searchFocused, input, m.visibleConnections(), m.sortField, m.sortAsc)
	if m.modal != nil {
		if _, ok := m.modal.(menuModel); ok {
			x, y := menuPosition(m.height, m.selected)
			return placeModalAt(base, m.modal.View(), m.width, m.height, x, y)
		}
		return placeModal(base, m.modal.View(), m.width, m.height)
	}
	return base
}

// renderPanel 按终端单元格宽度绘制主界面，避免中文和宽字符造成分栏错位。
func renderPanel(width, height int) string {
	return renderPanelWithInput(width, height, 0, false, newSearchInput())
}

func renderPanelWithInput(width, height, selected int, searchFocused bool, searchInput textinput.Model) string {
	return renderPanelWithConnections(width, height, selected, searchFocused, searchInput, nil, 0, true)
}

// renderPanelWithConnections 绘制连接列表及详情，并使用已过滤、排序后的数据保持两侧选中项一致。
func renderPanelWithConnections(width, height, selected int, searchFocused bool, searchInput textinput.Model, connections []connectionRow, sortField byte, sortAsc bool) string {
	innerWidth, innerHeight := width-2, height-2
	rows := make([]string, innerHeight)
	for i := range rows {
		rows[i] = strings.Repeat(" ", innerWidth)
	}

	contentWidth := innerWidth - 6
	put := func(row int, value string) {
		rows[row] = "   " + fit(value, contentWidth) + "   "
	}

	searchMarker := mutedStyle.Render(">")
	if searchFocused {
		searchMarker = accentStyle.Render(">")
	}
	searchInput.SetWidth(contentWidth - 2)
	put(1, searchMarker+" "+searchInput.View())

	mainStart := 3
	footerLine := innerHeight - 2
	leftWidth := contentWidth * 2 / 3
	rightWidth := contentWidth - leftWidth - 3
	mainRow := func(left, right string) string {
		return fit(left, leftWidth) + " " + borderStyle.Render("│") + " " + fit(right, rightWidth)
	}

	leftRows := make([]string, innerHeight)
	rightRows := make([]string, innerHeight)
	visibleRows := max(1, footerLine-mainStart-3)
	tableConnections, tableSelected := connections, selected
	if selected >= visibleRows {
		start := selected - visibleRows + 1
		tableConnections, tableSelected = connections[start:], visibleRows-1
	}
	connectionTable := newConnectionTable(leftWidth, visibleRows+1, tableSelected, !searchFocused, tableConnections, sortField, sortAsc)
	tableLines := strings.Split(connectionTable.View(), "\n")
	leftRows[mainStart] = tableLines[0]
	rightRows[mainStart] = labelStyle.Render("Details")
	lineStyle := borderStyle
	if !searchFocused {
		lineStyle = accentStyle
	}
	leftRows[mainStart+1] = lineStyle.Render(strings.Repeat("─", leftWidth))
	rightRows[mainStart+1] = borderStyle.Render(strings.Repeat("─", rightWidth))
	for i, line := range tableLines[1:] {
		if mainStart+2+i >= footerLine-1 {
			break
		}
		leftRows[mainStart+2+i] = line
	}

	if len(connections) > 0 {
		if selected < 0 || selected >= len(connections) {
			selected = 0
		}
		connection := connections[selected]
		detailRow := mainStart + 3
		detailRow = setDetail(rightRows, detailRow, rightWidth, "名称：", connection.name)
		detailRow = setDetail(rightRows, detailRow, rightWidth, "地址：", connection.username+"@"+connection.host+":"+connection.port)
		detailRow = setDetail(rightRows, detailRow, rightWidth, "认证：", connection.auth)
		detailRow = setDetail(rightRows, detailRow, rightWidth, "凭据：", connection.credName)
		detailRow = setDetail(rightRows, detailRow, rightWidth, "上次使用：", connection.lastUsed)
		setDetail(rightRows, detailRow, rightWidth, "备注：", connection.remark)
	}
	for row := mainStart; row < footerLine-1; row++ {
		put(row, mainRow(leftRows[row], rightRows[row]))
	}

	put(footerLine, borderStyle.Render(strings.Repeat("─", contentWidth)))
	put(footerLine+1, mutedStyle.Render("Enter/l: 菜单 | a: 新增 | p: 凭据管理 | n/h/u: 排序 | ↑/k/↓/j: 滚动 | q: 退出 "))

	result := make([]string, 0, height)
	title := accentStyle.Render(" " + windowTitle + " ")
	result = append(result, borderStyle.Render("╭──┤")+title+borderStyle.Render("├"+strings.Repeat("─", innerWidth-ansi.StringWidth(title)-4)+"╮"))
	for _, row := range rows {
		result = append(result, borderStyle.Render("│")+row+borderStyle.Render("│"))
	}
	result = append(result, borderStyle.Render("╰"+strings.Repeat("─", innerWidth)+"╯"))
	return strings.Join(result, "\n")
}

func newConnectionTable(width, height, selected int, focused bool, connections []connectionRow, sortField byte, sortAsc bool) table.Model {
	rows := make([]table.Row, 0, len(connections))
	for _, connection := range connections {
		rows = append(rows, table.Row{fitEllipsis(" "+connection.name, nameColumnWidth), connection.host + ":" + connection.port, connection.username})
	}
	header := plainStyle
	if focused {
		header = accentStyle
	}
	styles := table.DefaultStyles()
	styles.Header = header
	// 单元格不单独输出颜色，避免重置选中行的整行背景色。
	styles.Cell = lipgloss.NewStyle()
	styles.Selected = lipgloss.NewStyle()
	if focused {
		styles.Selected = selectedStyle.Width(width).MaxWidth(width).Inline(true)
	}
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: sortTitle(" Name", 'n', sortField, sortAsc), Width: nameColumnWidth},
			{Title: sortTitle("Host:Port", 'h', sortField, sortAsc), Width: width - nameColumnWidth - userColumnWidth},
			{Title: sortTitle("User", 'u', sortField, sortAsc), Width: userColumnWidth},
		}),
		table.WithRows(rows),
		table.WithWidth(width),
		table.WithHeight(max(1, height)),
		table.WithFocused(focused),
		table.WithStyles(styles),
	)
	t.SetCursor(selected)
	return t
}

func sortTitle(title string, field, active byte, asc bool) string {
	if field != active {
		return title
	}
	if asc {
		return title + " ↑"
	}
	return title + " ↓"
}

func setDetail(rows []string, start, width int, label, value string) int {
	if start < 0 || start >= len(rows) {
		return len(rows)
	}
	labelWidth := ansi.StringWidth(label)
	parts := wrapText(value, max(1, width-labelWidth))
	rows[start] = labelStyle.Render(label) + parts[0]
	indent := strings.Repeat(" ", labelWidth)
	for i, part := range parts[1:] {
		if start+i+1 >= len(rows) {
			return len(rows)
		}
		rows[start+i+1] = indent + part
	}
	return start + len(parts) + 1
}

func wrapText(value string, width int) []string {
	if width <= 0 {
		return []string{value}
	}
	var lines []string
	var line []rune
	lineWidth := 0
	for _, r := range value {
		runeWidth := ansi.StringWidth(string(r))
		if len(line) > 0 && lineWidth+runeWidth > width {
			lines = append(lines, string(line))
			line, lineWidth = nil, 0
		}
		line = append(line, r)
		lineWidth += runeWidth
	}
	if len(line) > 0 || len(lines) == 0 {
		lines = append(lines, string(line))
	}
	return lines
}

func fit(value string, width int) string {
	if width <= 0 {
		return ""
	}
	value = ansi.Truncate(value, width, "")
	return value + strings.Repeat(" ", width-ansi.StringWidth(value))
}

func fitEllipsis(value string, width int) string {
	if ansi.StringWidth(value) <= width {
		return fit(value, width)
	}
	return ansi.Truncate(value, width, "...")
}

package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	localTagStyle      = lipgloss.NewStyle().Background(lipgloss.Color("#1473E6")).Foreground(lipgloss.Color("#FFFFFF")).Padding(0, 1)
	remoteTagStyle     = lipgloss.NewStyle().Background(lipgloss.Color("#7F5AF0")).Foreground(lipgloss.Color("#FFFFFF")).Padding(0, 1)
	folderStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#E5C07B"))
	fileStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#61AFEF"))
	selectedMarkStyle  = lipgloss.NewStyle().Background(lipgloss.Color("#007ea1")).Foreground(lipgloss.Color("#00b3e4"))
	copiedMarkStyle    = lipgloss.NewStyle().Background(lipgloss.Color("#2E7D32")).Foreground(lipgloss.Color("#00b3e4"))
	transferFrameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#4e9af1ff"))
)

var transferBorder = lipgloss.RoundedBorder()

// View 绘制中等尺寸的文件传输弹窗及其内部覆盖层。
func (m transferModel) View() string {
	width, height := transferSize(m.width, m.height)
	// 终端过小时无法容纳完整布局，返回等尺寸空白画布，避免负长度切片。
	if width < 10 || height < 8 {
		lines := make([]string, height)
		for i := range lines {
			lines[i] = strings.Repeat(" ", width)
		}
		return strings.Join(lines, "\n")
	}
	contentWidth := width - 2
	bodyHeight := height - 6
	leftWidth := contentWidth * 2 / 3
	rightWidth := contentWidth - leftWidth - 1

	lines := make([]string, 0, height)
	lines = append(lines, transferFrame(transferBorder.TopLeft, transferBorder.Top, transferBorder.TopRight, width))
	lines = append(lines, transferRow(m.renderHeader(contentWidth), contentWidth))
	lines = append(lines, m.renderSortSeparator(contentWidth, leftWidth))

	left := m.renderList(leftWidth, bodyHeight)
	right := m.renderDetails(rightWidth, bodyHeight)
	for row := range bodyHeight {
		content := left[row] + borderStyle.Render(transferBorder.Left) + right[row]
		lines = append(lines, transferRow(content, contentWidth))
	}

	lines = append(lines, transferSeparator(width))
	footer := mutedStyle.Render("1/2: 切换 | y/p: 复制/粘贴 | a: 新建 | q/Esc: 关闭 | ?: 帮助")
	lines = append(lines, transferRow(fit(" "+footer, contentWidth), contentWidth))
	lines = append(lines, transferFrame(transferBorder.BottomLeft, transferBorder.Bottom, transferBorder.BottomRight, width))
	view := strings.Join(lines, "\n")
	if overlay := m.renderOverlay(); overlay != "" {
		return placeModal(view, overlay, width, height)
	}
	return view
}

func transferSize(terminalWidth, terminalHeight int) (int, int) {
	width := max(68, min(96, terminalWidth*4/5))
	height := max(16, min(26, terminalHeight*3/4))
	return min(width, max(1, terminalWidth-2)), min(height, max(1, terminalHeight-2))
}

func (m transferModel) renderHeader(width int) string {
	label, labelStyle := "本地", localTagStyle
	if m.location == remoteSide {
		label, labelStyle = "远程", remoteTagStyle
	}
	tag := labelStyle.Render(label)
	remaining := max(1, width-lipgloss.Width(tag)-2)
	address := m.currentPath()
	marker := ""
	if m.focus == addressFocus {
		input := m.address
		input.SetWidth(max(1, remaining-2))
		address = input.View()
		marker = accentStyle.Render("> ")
	}
	return fit(" "+tag+" "+marker+address, width)
}

func (m transferModel) renderList(width, height int) []string {
	rows := make([]string, height)
	for i := range rows {
		rows[i] = strings.Repeat(" ", width)
	}
	search := m.search
	search.SetWidth(max(1, width-7))
	marker := "  "
	if m.focus == searchFocus {
		marker = accentStyle.Render("> ")
	}
	rows[0] = fit(marker+accentStyle.Render("🔍")+" "+search.View(), width)

	entries := m.visibleEntries()
	capacity := max(0, height-1)
	start := 0
	if capacity > 0 && m.cursor >= capacity {
		start = m.cursor - capacity + 1
	}
	for row := 0; row < capacity && start+row < len(entries); row++ {
		index, entry := start+row, entries[start+row]
		focused := index == m.cursor && m.focus == listFocus
		cursor := "  "
		if focused {
			cursor = accentStyle.Render(">") + " "
		}
		_, selected := m.selected[entry.name]
		clipboardEntry := m.isClipboardEntry(entry.name)
		if selected || clipboardEntry {
			markerStyle := selectedMarkStyle
			if clipboardEntry {
				markerStyle = copiedMarkStyle
			}
			marker := " "
			if focused {
				marker = ">"
			}
			cursor = markerStyle.Render(marker) + " "
		}
		iconText, iconStyle := "📄", fileStyle
		if entry.dir {
			iconText, iconStyle = "📁", folderStyle
		}
		icon := iconStyle.Render(iconText)
		name := entry.name
		if focused {
			name = accentStyle.Render(name)
		}
		rows[row+1] = fit(cursor+icon+" "+name, width)
	}
	return rows
}

func (m transferModel) renderDetails(width, height int) []string {
	rows := make([]string, height)
	values := []string{
		"",
		centeredSection("剪贴板", width),
	}
	if len(m.clipboard.entries) == 0 {
		values = append(values, " 空")
	} else {
		available := max(0, height-len(values)-3)
		for _, entry := range m.clipboard.entries[:min(len(m.clipboard.entries), available)] {
			values = append(values, " "+entry.name)
		}
	}
	for len(values) < height-3 {
		values = append(values, "")
	}
	if len(values) <= height-3 {
		values = append(values, "", transferSection("状态", width), " "+m.status)
	}
	for i := range rows {
		if i < len(values) {
			rows[i] = fit(values[i], width)
		} else {
			rows[i] = strings.Repeat(" ", width)
		}
	}
	return rows
}

func (m transferModel) sortLabel() string {
	name := map[byte]string{'n': "文件名", 's': "大小", 't': "修改时间"}[m.sortField]
	arrow := "↑"
	if !m.sortAsc {
		arrow = "↓"
	}
	return name + " " + arrow
}

func transferSection(label string, width int) string {
	lineWidth := max(0, width-lipgloss.Width(label)-4)
	return " " + mutedStyle.Render(label) + " " + borderStyle.Render(strings.Repeat(transferBorder.Top, lineWidth)) + "  "
}

func centeredSection(label string, width int) string {
	lineWidth := max(0, width-lipgloss.Width(label)-2)
	leftWidth := lineWidth / 2
	rightWidth := lineWidth - leftWidth
	return borderStyle.Render(strings.Repeat(transferBorder.Top, leftWidth)) + " " + mutedStyle.Render(label) + " " + borderStyle.Render(strings.Repeat(transferBorder.Top, rightWidth))
}

func (m transferModel) renderSortSeparator(contentWidth, leftWidth int) string {
	box := "┤ " + plainStyle.Render(m.sortLabel()) + " ├"
	boxWidth := lipgloss.Width(box)
	// 排序框相对中间竖线左移六个单元，避免挤压分隔线连接处。
	end := min(max(0, leftWidth-6), contentWidth)
	start := max(0, end-boxWidth)
	line := strings.Repeat(transferBorder.Top, start) + box + strings.Repeat(transferBorder.Top, contentWidth-end)
	return transferFrameStyle.Render(transferBorder.MiddleLeft) + fit(line, contentWidth) + transferFrameStyle.Render(transferBorder.MiddleRight)
}

func transferRow(content string, width int) string {
	return transferFrameStyle.Render(transferBorder.Left) + fit(content, width) + transferFrameStyle.Render(transferBorder.Right)
}

func transferFrame(left, middle, right string, width int) string {
	return transferFrameStyle.Render(left + strings.Repeat(middle, max(0, width-2)) + right)
}

func transferSeparator(width int) string {
	return transferFrameStyle.Render(transferBorder.Left) + " " +
		borderStyle.Render(strings.Repeat(transferBorder.Top, max(0, width-4))) +
		" " + transferFrameStyle.Render(transferBorder.Right)
}

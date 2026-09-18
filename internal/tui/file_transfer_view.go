package tui

import (
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var (
	localTagStyle      = lipgloss.NewStyle().Background(localTagColor).Foreground(selectedTextColor).Padding(0, 1)
	remoteTagStyle     = lipgloss.NewStyle().Background(remoteTagColor).Foreground(selectedTextColor).Padding(0, 1)
	folderStyle        = lipgloss.NewStyle().Foreground(folderColor)
	fileStyle          = lipgloss.NewStyle().Foreground(fileColor)
	transferFrameStyle = lipgloss.NewStyle().Foreground(modalBorderColor)
)

var transferBorder = lipgloss.RoundedBorder()

// 使用 Nerd Font 单格图标，避免 Emoji 在 tmux 与本地终端中的宽度差异造成边框偏移。
const (
	transferSearchIcon = "\ue68f"
	transferFolderIcon = "\uf07b"
	transferFileIcon   = "\uf15b"
)

type transferIconSpec struct {
	glyph string
	color string
}

// 常见扩展名沿用  Nerd Font 图标和配色，未匹配时回退到通用文件图标。
var transferExtensionIcons = map[string]transferIconSpec{
	"go": {"\ue627", transferIconGoColor}, "js": {"\ue781", transferIconScriptColor}, "mjs": {"\ue781", transferIconScriptColor},
	"ts": {"\U000f06e6", transferIconTypeScriptColor}, "py": {"\ue606", transferIconPythonColor}, "rs": {"\ue7a8", transferIconScriptColor},
	"java": {"\ue738", transferIconJavaColor}, "c": {"\ue649", transferIconCColor}, "cpp": {"\ue646", transferIconCColor},
	"h": {"\uf0fd", transferIconPythonColor}, "html": {"\uf13b", transferIconJavaColor}, "css": {"\uf13c", transferIconCSSColor},
	"json": {"\ue60b", transferIconJSONColor}, "yaml": {"\ue601", transferIconScriptColor}, "yml": {"\ue601", transferIconScriptColor},
	"toml": {"\U000f016a", transferIconScriptColor}, "md": {"\uf48a", transferIconTextColor}, "txt": {"\uf15c", transferIconTextColor},
	"pdf": {"\uf1c1", transferIconPDFColor}, "env": {"\uf462", transferIconEnvColor}, "sql": {"\uf1c0", transferIconSQLColor},
	"log": {"\uf18d", transferIconTextColor}, "sh": {"\uf489", transferIconShellColor}, "bash": {"\uf489", transferIconShellColor},
	"zsh": {"\uf489", transferIconShellColor}, "png": {"\uf1c5", transferIconImageArchiveColor}, "jpg": {"\uf1c5", transferIconImageArchiveColor},
	"jpeg": {"\uf1c5", transferIconImageArchiveColor}, "gif": {"\uf1c5", transferIconImageArchiveColor}, "svg": {"\uf1c5", transferIconImageArchiveColor},
	"webp": {"\uf1c5", transferIconImageArchiveColor}, "mp3": {"\uf001", transferIconAudioColor}, "wav": {"\uf001", transferIconAudioColor},
	"flac": {"\uf001", transferIconAudioColor}, "mp4": {"\uf03d", transferIconVideoColor}, "mkv": {"\uf03d", transferIconVideoColor},
	"avi": {"\uf03d", transferIconVideoColor}, "zip": {"\uf410", transferIconImageArchiveColor}, "tar": {"\uf410", transferIconImageArchiveColor},
	"gz": {"\uf410", transferIconImageArchiveColor}, "bz2": {"\uf410", transferIconImageArchiveColor}, "7z": {"\uf410", transferIconImageArchiveColor},
	"rar": {"\uf410", transferIconImageArchiveColor},
}

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
	footer := mutedStyle.Render("1/2 切换 | y/p 复制/粘贴 | a 新建 | q 关闭 | ? 帮助")
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
	label, labelStyle, tagColor := "本地", localTagStyle, localTagColor
	if m.location == remoteSide {
		label, labelStyle, tagColor = "远程", remoteTagStyle, remoteTagColor
	}
	edgeStyle := lipgloss.NewStyle().Foreground(tagColor)
	tag := edgeStyle.Render("") + labelStyle.Render(label) + edgeStyle.Render("")
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
	rows[0] = fit(marker+accentStyle.Render(transferSearchIcon)+" "+search.View(), width)

	entries := m.visibleEntries()
	selectionStyle := lipgloss.NewStyle().Background(multiSelectedColor)
	selectionEdgeStyle := lipgloss.NewStyle().Foreground(multiSelectedColor)
	capacity := max(0, height-1)
	start := 0
	if capacity > 0 && m.cursor >= capacity {
		start = m.cursor - capacity + 1
	}
	for row := 0; row < capacity && start+row < len(entries); row++ {
		index, entry := start+row, entries[start+row]
		focused := index == m.cursor && m.focus == listFocus
		cursor := " "
		if focused {
			cursor = accentStyle.Render(">")
		}
		_, selected := m.selected[entry.name]
		iconGlyph, iconStyle := transferEntryIcon(entry)
		icon := iconStyle.Render(iconGlyph)
		name := entry.name
		if focused {
			name = accentStyle.Render(name)
		}
		line := fit(" "+cursor+" "+icon+" "+name, width)
		if selected {
			nameWidth := max(0, width-6)
			name = ansi.Truncate(entry.name, nameWidth, "")
			cursorStyle, nameStyle := selectionStyle, selectionStyle
			if focused {
				cursorStyle = accentStyle.Background(multiSelectedColor)
				nameStyle = accentStyle.Background(multiSelectedColor)
			}
			padding := strings.Repeat(" ", max(0, nameWidth-ansi.StringWidth(name)))
			line = selectionEdgeStyle.Render("") + cursorStyle.Render(ansi.Strip(cursor)) + selectionStyle.Render(" ") +
				iconStyle.Background(multiSelectedColor).Render(iconGlyph) + selectionStyle.Render(" ") +
				nameStyle.Render(name) + selectionStyle.Render(padding) + selectionEdgeStyle.Render("")
		}
		rows[row+1] = line
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
			values = append(values, " "+renderTransferEntryIcon(entry)+" "+entry.name)
		}
	}
	for len(values) < height-3 {
		values = append(values, "")
	}
	if len(values) <= height-3 {
		values = append(values, "", centeredSection("状态", width), " "+m.status)
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

func renderTransferEntryIcon(entry transferEntry) string {
	glyph, style := transferEntryIcon(entry)
	return style.Render(glyph)
}

func transferEntryIcon(entry transferEntry) (string, lipgloss.Style) {
	if entry.dir {
		return transferFolderIcon, folderStyle
	}
	ext := strings.TrimPrefix(filepath.Ext(strings.ToLower(entry.name)), ".")
	if icon, ok := transferExtensionIcons[ext]; ok {
		return icon.glyph, lipgloss.NewStyle().Foreground(lipgloss.Color(icon.color))
	}
	return transferFileIcon, fileStyle
}

func (m transferModel) sortLabel() string {
	name := map[byte]string{'n': "文件名", 's': "大小", 't': "修改时间"}[m.sortField]
	arrow := "↑"
	if !m.sortAsc {
		arrow = "↓"
	}
	return name + " " + arrow
}

func centeredSection(label string, width int) string {
	lineWidth := max(0, width-lipgloss.Width(label)-4)
	leftWidth := lineWidth / 2
	rightWidth := lineWidth - leftWidth
	return " " + borderStyle.Render(strings.Repeat(transferBorder.Top, leftWidth)) + " " + mutedStyle.Render(label) + " " + borderStyle.Render(strings.Repeat(transferBorder.Top, rightWidth)) + " "
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

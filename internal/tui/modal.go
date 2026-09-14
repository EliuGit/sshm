package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const modalContentWidth = 50

var modalStyle = lipgloss.NewStyle().
	Width(54).
	Padding(0, 1).
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#4e9af1ff")).
	Foreground(textColor)

var confirmStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E5C07B"))
var modalTitleStyle = accentStyle.Bold(false)

// modalModel 定义可覆盖在主界面上的单层模态窗口。
type modalModel interface {
	Update(tea.Msg) (modalModel, tea.Cmd)
	View() string
}

// closeModalMsg 请求主界面关闭当前模态窗口。
type closeModalMsg struct{}

// isClearInputKey 判断是否为清空当前输入控件的快捷键。
func isClearInputKey(msg tea.Msg) bool {
	key, ok := msg.(tea.KeyPressMsg)
	return ok && key.String() == "ctrl+c"
}

// formDialog 使用连接/凭据表单相同的标题、分隔线和底部提示布局。
// statusStyle 和 titleStyle 由调用方决定，普通快捷提示使用正文色而不是灰色。
func formDialog(title, body, status string, statusStyle, titleStyle lipgloss.Style) string {
	header := titleStyle.Render(title)
	lines := []string{
		header,
		borderStyle.Render(strings.Repeat("─", modalContentWidth)),
	}
	lines = append(lines, strings.Split(body, "\n")...)
	lines = append(lines, borderStyle.Render(strings.Repeat("─", modalContentWidth)), statusStyle.Render(status))
	return modalStyle.Render(strings.Join(lines, "\n"))
}

// placeModal 将弹窗居中覆盖到背景上，并按终端单元格宽度处理 ANSI 和宽字符。
func placeModal(base, modal string, width, height int) string {
	modalWidth, modalHeight := lipgloss.Width(modal), lipgloss.Height(modal)
	return placeModalAt(base, modal, width, height, (width-modalWidth)/2, (height-modalHeight)/2)
}

// placeModalAt 将弹窗覆盖到指定坐标，并保证弹窗不会超出终端边界。
func placeModalAt(base, modal string, width, height, x, y int) string {
	if width <= 0 || height <= 0 || modal == "" {
		return base
	}
	baseLines := strings.Split(base, "\n")
	for len(baseLines) < height {
		baseLines = append(baseLines, strings.Repeat(" ", width))
	}
	modalLines := strings.Split(modal, "\n")
	modalWidth := 0
	for _, line := range modalLines {
		modalWidth = max(modalWidth, ansi.StringWidth(line))
	}
	modalWidth = min(modalWidth, width)
	x = max(0, min(x, width-modalWidth))
	y = max(0, min(y, height-len(modalLines)))
	for i, line := range modalLines {
		row := y + i
		if row >= height {
			break
		}
		baseLine := fit(baseLines[row], width)
		left := fit(ansi.Cut(baseLine, 0, x), x)
		rightStart := x + modalWidth
		rightCut := rightStart
		// 边界落在宽字符中间时跳过整个字符，避免右侧背景被挤后一格。
		for rightCut < width && ansi.StringWidth(ansi.Cut(baseLine, 0, rightCut)) < rightCut {
			rightCut++
		}
		right := fit(strings.Repeat(" ", rightCut-rightStart)+ansi.Cut(baseLine, rightCut, width), width-rightStart)
		baseLines[row] = left + fit(line, modalWidth) + right
	}
	return strings.Join(baseLines[:height], "\n")
}

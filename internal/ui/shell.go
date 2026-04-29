package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type shellView struct {
	style   lipgloss.Style
	width   int
	height  int
	header  string
	body    string
	status  string
	footer  string
	overlay string
}

// renderShell 统一拼装页面壳层，明确 header/body/status/footer/overlay 边界。
// 这里不要再尝试补“全局底色/整屏背景”：
// 1. 当前渲染模型是各组件先 Render 成 ANSI 字符串再拼接，不是分层画布。
// 2. 父级背景无法稳定托住子级后续 Render 内容，整屏铺底会在很多位置被 reset 回终端默认背景。
// 3. 当前项目结论是维持现有背景使用方式，只保留局部高亮块背景。
func (m *Model) renderShell(view shellView) string {
	sections := make([]string, 0, 4)
	for _, section := range []string{view.header, view.body, view.status, view.footer} {
		if strings.TrimSpace(section) == "" {
			continue
		}
		sections = append(sections, section)
	}

	base := strings.Join(sections, "\n")
	base = view.style.Render(base)
	if strings.TrimSpace(view.overlay) == "" {
		return base
	}
	return overlayCenter(base, view.overlay, view.width, view.height)
}

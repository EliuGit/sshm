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

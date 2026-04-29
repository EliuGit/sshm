package themes

import "github.com/charmbracelet/lipgloss"

func init() {
	// 参考 Nord 官方调色板，并沿用其“低饱和、长时间阅读友好”的思路：
	// - 边框与非激活状态保持蓝灰层次
	// - 聚焦与标题使用冰蓝/极光色，但控制饱和度不过激
	// - 选中底使用偏亮冷蓝，保证在深色终端里有明确焦点
	Register(NordName, Palette{
		PanelBorder:                 lipgloss.Color("#4C566A"),
		PanelBorderFocused:          lipgloss.Color("#88C0D0"),
		DialogBorder:                lipgloss.Color("#B48EAD"),
		Title:                       lipgloss.Color("#88C0D0"),
		SectionTitle:                lipgloss.Color("#8FBCBB"),
		Text:                        lipgloss.Color("#ECEFF4"),
		SubtleText:                  lipgloss.Color("#E5E9F0"),
		MutedText:                   lipgloss.Color("#81A1C1"),
		HelpText:                    lipgloss.Color("#D8DEE9"),
		ShortcutLabel:               lipgloss.Color("#E5E9F0"),
		Error:                       lipgloss.Color("#BF616A"),
		Success:                     lipgloss.Color("#A3BE8C"),
		Warning:                     lipgloss.Color("#EBCB8B"),
		SelectionText:               lipgloss.Color("#2E3440"),
		SelectionBackground:         lipgloss.Color("#81A1C1"),
		SelectionInactiveText:       lipgloss.Color("#ECEFF4"),
		SelectionInactiveBackground: lipgloss.Color("#434C5E"),
		FieldLabel:                  lipgloss.Color("#D08770"),
		FieldLabelFocused:           lipgloss.Color("#88C0D0"),
		GroupScope:                  lipgloss.Color("#A3BE8C"),
		SearchHighlight:             lipgloss.Color("#EBCB8B"),
		KeyText:                     lipgloss.Color("#2E3440"),
		KeyBackground:               lipgloss.Color("#8FBCBB"),
		StatusText:                  lipgloss.Color("#ECEFF4"),
		BadgeText:                   lipgloss.Color("#2E3440"),
		BadgeBackground:             lipgloss.Color("#88C0D0"),
		BadgeMutedBackground:        lipgloss.Color("#434C5E"),
	})
}

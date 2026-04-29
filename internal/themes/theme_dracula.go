package themes

import "github.com/charmbracelet/lipgloss"

func init() {
	// 参考 Dracula 官方调色板，并按 TUI 语义重新分配：
	// - 用紫色做主选中高亮，确保列表焦点足够醒目
	// - 用蓝灰做非激活选中底，避免整屏跳色
	// - 用粉/青/橙分别承担标题、聚焦、字段提示，提升信息分层
	Register(DraculaName, Palette{
		PanelBorder:                 lipgloss.Color("#6272A4"),
		PanelBorderFocused:          lipgloss.Color("#8BE9FD"),
		DialogBorder:                lipgloss.Color("#FFB86C"),
		Title:                       lipgloss.Color("#FF79C6"),
		SectionTitle:                lipgloss.Color("#8BE9FD"),
		Text:                        lipgloss.Color("#F8F8F2"),
		SubtleText:                  lipgloss.Color("#D7DAE5"),
		MutedText:                   lipgloss.Color("#6272A4"),
		HelpText:                    lipgloss.Color("#C7C9D3"),
		ShortcutLabel:               lipgloss.Color("#F8F8F2"),
		Error:                       lipgloss.Color("#FF5555"),
		Success:                     lipgloss.Color("#50FA7B"),
		Warning:                     lipgloss.Color("#F1FA8C"),
		SelectionText:               lipgloss.Color("#282A36"),
		SelectionBackground:         lipgloss.Color("#BD93F9"),
		SelectionInactiveText:       lipgloss.Color("#F8F8F2"),
		SelectionInactiveBackground: lipgloss.Color("#44475A"),
		FieldLabel:                  lipgloss.Color("#FFB86C"),
		FieldLabelFocused:           lipgloss.Color("#8BE9FD"),
		GroupScope:                  lipgloss.Color("#50FA7B"),
		SearchHighlight:             lipgloss.Color("#F1FA8C"),
		KeyText:                     lipgloss.Color("#282A36"),
		KeyBackground:               lipgloss.Color("#FF79C6"),
		StatusText:                  lipgloss.Color("#F8F8F2"),
		BadgeText:                   lipgloss.Color("#282A36"),
		BadgeBackground:             lipgloss.Color("#8BE9FD"),
		BadgeMutedBackground:        lipgloss.Color("#44475A"),
	})
}

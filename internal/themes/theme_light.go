package themes

import "github.com/charmbracelet/lipgloss"

func init() {
	// 亮色主题延续主流 TUI 的做法：深色正文 + 冷色聚焦 + 浅色选中底。
	// 这样在浅色终端里既能保住层次，也不会因为高饱和大面积铺色造成刺眼。
	Register(LightName, Palette{
		PanelBorder:                 lipgloss.Color("246"),
		PanelBorderFocused:          lipgloss.Color("32"),
		DialogBorder:                lipgloss.Color("68"),
		Title:                       lipgloss.Color("24"),
		SectionTitle:                lipgloss.Color("31"),
		Text:                        lipgloss.Color("236"),
		SubtleText:                  lipgloss.Color("239"),
		MutedText:                   lipgloss.Color("244"),
		HelpText:                    lipgloss.Color("240"),
		ShortcutLabel:               lipgloss.Color("238"),
		Error:                       lipgloss.Color("160"),
		Success:                     lipgloss.Color("28"),
		Warning:                     lipgloss.Color("130"),
		SelectionText:               lipgloss.Color("17"),
		SelectionBackground:         lipgloss.Color("153"),
		SelectionInactiveText:       lipgloss.Color("238"),
		SelectionInactiveBackground: lipgloss.Color("252"),
		FieldLabel:                  lipgloss.Color("60"),
		FieldLabelFocused:           lipgloss.Color("25"),
		GroupScope:                  lipgloss.Color("67"),
		SearchHighlight:             lipgloss.Color("130"),
		KeyText:                     lipgloss.Color("17"),
		KeyBackground:               lipgloss.Color("189"),
		StatusText:                  lipgloss.Color("236"),
		BadgeText:                   lipgloss.Color("255"),
		BadgeBackground:             lipgloss.Color("31"),
		BadgeMutedBackground:        lipgloss.Color("252"),
	})
}

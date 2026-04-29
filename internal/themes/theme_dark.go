package themes

import "github.com/charmbracelet/lipgloss"

func init() {
	Register(DefaultName, Palette{
		PanelBorder:                 lipgloss.Color("239"),
		PanelBorderFocused:          lipgloss.Color("44"),
		DialogBorder:                lipgloss.Color("180"),
		Title:                       lipgloss.Color("229"),
		SectionTitle:                lipgloss.Color("151"),
		Text:                        lipgloss.Color("254"),
		SubtleText:                  lipgloss.Color("252"),
		MutedText:                   lipgloss.Color("244"),
		HelpText:                    lipgloss.Color("246"),
		ShortcutLabel:               lipgloss.Color("251"),
		Error:                       lipgloss.Color("210"),
		Success:                     lipgloss.Color("157"),
		Warning:                     lipgloss.Color("222"),
		SelectionText:               lipgloss.Color("232"),
		SelectionBackground:         lipgloss.Color("115"),
		SelectionInactiveText:       lipgloss.Color("254"),
		SelectionInactiveBackground: lipgloss.Color("238"),
		FieldLabel:                  lipgloss.Color("223"),
		FieldLabelFocused:           lipgloss.Color("195"),
		GroupScope:                  lipgloss.Color("186"),
		SearchHighlight:             lipgloss.Color("222"),
		KeyText:                     lipgloss.Color("230"),
		KeyBackground:               lipgloss.Color("59"),
		StatusText:                  lipgloss.Color("255"),
		BadgeText:                   lipgloss.Color("230"),
		BadgeBackground:             lipgloss.Color("66"),
		BadgeMutedBackground:        lipgloss.Color("239"),
	})
}

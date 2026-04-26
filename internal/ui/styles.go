package ui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Palette ThemePalette
	Styles  ThemeStyles
}

type ThemePalette struct {
	Background                  lipgloss.Color
	PanelBorder                 lipgloss.Color
	PanelBorderFocused          lipgloss.Color
	DialogBorder                lipgloss.Color
	Title                       lipgloss.Color
	SectionTitle                lipgloss.Color
	Text                        lipgloss.Color
	SubtleText                  lipgloss.Color
	MutedText                   lipgloss.Color
	HelpText                    lipgloss.Color
	ShortcutLabel               lipgloss.Color
	Error                       lipgloss.Color
	Success                     lipgloss.Color
	Warning                     lipgloss.Color
	SelectionText               lipgloss.Color
	SelectionBackground         lipgloss.Color
	SelectionInactiveText       lipgloss.Color
	SelectionInactiveBackground lipgloss.Color
	FieldLabel                  lipgloss.Color
	FieldLabelFocused           lipgloss.Color
	GroupScope                  lipgloss.Color
	SearchHighlight             lipgloss.Color
	KeyText                     lipgloss.Color
	KeyBackground               lipgloss.Color
}

type ThemeStyles struct {
	App                lipgloss.Style
	Panel              lipgloss.Style
	FocusedPanel       lipgloss.Style
	Dialog             lipgloss.Style
	ActionBar          lipgloss.Style
	PageTitle          lipgloss.Style
	SectionTitle       lipgloss.Style
	Text               lipgloss.Style
	SubtleText         lipgloss.Style
	MutedText          lipgloss.Style
	HelpText           lipgloss.Style
	ShortcutLabel      lipgloss.Style
	ErrorText          lipgloss.Style
	SuccessText        lipgloss.Style
	WarningText        lipgloss.Style
	Selection          lipgloss.Style
	SelectionInactive  lipgloss.Style
	SelectionDetail    lipgloss.Style
	ListItemTitle      lipgloss.Style
	ListItemMeta       lipgloss.Style
	FieldLabel         lipgloss.Style
	FieldLabelFocused  lipgloss.Style
	GroupScope         lipgloss.Style
	SearchValueBlurred lipgloss.Style
	Input              lipgloss.Style
	InputFocused       lipgloss.Style
	Keycap             lipgloss.Style
	SearchBox          lipgloss.Style
	SearchBoxFocused   lipgloss.Style
	PanelTitle         lipgloss.Style
	PanelTitleFocused  lipgloss.Style
}

func newDefaultTheme() Theme {
	palette := ThemePalette{
		Background:                  lipgloss.Color("233"),
		PanelBorder:                 lipgloss.Color("240"),
		PanelBorderFocused:          lipgloss.Color("81"),
		DialogBorder:                lipgloss.Color("213"),
		Title:                       lipgloss.Color("220"),
		SectionTitle:                lipgloss.Color("81"),
		Text:                        lipgloss.Color("252"),
		SubtleText:                  lipgloss.Color("250"),
		MutedText:                   lipgloss.Color("244"),
		HelpText:                    lipgloss.Color("246"),
		ShortcutLabel:               lipgloss.Color("250"),
		Error:                       lipgloss.Color("203"),
		Success:                     lipgloss.Color("42"),
		Warning:                     lipgloss.Color("221"),
		SelectionText:               lipgloss.Color("232"),
		SelectionBackground:         lipgloss.Color("117"),
		SelectionInactiveText:       lipgloss.Color("252"),
		SelectionInactiveBackground: lipgloss.Color("239"),
		FieldLabel:                  lipgloss.Color("223"),
		FieldLabelFocused:           lipgloss.Color("229"),
		GroupScope:                  lipgloss.Color("213"),
		SearchHighlight:             lipgloss.Color("220"),
		KeyText:                     lipgloss.Color("81"),
		KeyBackground:               lipgloss.Color("237"),
	}
	return newTheme(palette)
}

func newTheme(palette ThemePalette) Theme {
	text := lipgloss.NewStyle().
		Foreground(palette.Text)

	subtle := lipgloss.NewStyle().
		Foreground(palette.SubtleText)

	muted := lipgloss.NewStyle().
		Foreground(palette.MutedText)

	selection := lipgloss.NewStyle().
		Foreground(palette.SelectionText).
		Background(palette.SelectionBackground)

	styles := ThemeStyles{
		App: lipgloss.NewStyle().
			Padding(1, 2).
			Foreground(palette.Text),

		Panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(palette.PanelBorder).
			Padding(0, 1),

		FocusedPanel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(palette.PanelBorderFocused).
			Padding(0, 1),

		Dialog: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(palette.DialogBorder).
			Padding(1, 2),

		ActionBar: lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(palette.PanelBorder).
			PaddingTop(1),

		PageTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(palette.Title),

		SectionTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(palette.SectionTitle),

		Text:               text,
		SubtleText:         subtle,
		MutedText:          muted,
		HelpText:           lipgloss.NewStyle().Foreground(palette.HelpText),
		ShortcutLabel:      lipgloss.NewStyle().Foreground(palette.ShortcutLabel),
		ErrorText:          lipgloss.NewStyle().Foreground(palette.Error),
		SuccessText:        lipgloss.NewStyle().Foreground(palette.Success),
		WarningText:        lipgloss.NewStyle().Foreground(palette.Warning),
		Selection:          selection,
		SelectionInactive:  lipgloss.NewStyle().Foreground(palette.SelectionInactiveText).Background(palette.SelectionInactiveBackground),
		SelectionDetail:    selection.Copy().Foreground(palette.Text),
		ListItemTitle:      text.Copy().Bold(true),
		ListItemMeta:       muted,
		FieldLabel:         lipgloss.NewStyle().Foreground(palette.FieldLabel),
		FieldLabelFocused:  lipgloss.NewStyle().Foreground(palette.FieldLabelFocused),
		GroupScope:         lipgloss.NewStyle().Bold(true).Foreground(palette.GroupScope),
		SearchValueBlurred: lipgloss.NewStyle().Bold(true).Foreground(palette.SearchHighlight),

		Input: lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(palette.Text),

		InputFocused: lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(palette.FieldLabelFocused),

		Keycap: lipgloss.NewStyle().
			Bold(true).
			Foreground(palette.KeyText).
			Background(palette.KeyBackground).
			Padding(0, 1),

		SearchBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(palette.PanelBorder).
			Padding(0, 1),

		SearchBoxFocused: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(palette.PanelBorderFocused).
			Padding(0, 1),

		PanelTitle:        subtle,
		PanelTitleFocused: lipgloss.NewStyle().Bold(true).Foreground(palette.Title),
	}

	return Theme{
		Palette: palette,
		Styles:  styles,
	}
}

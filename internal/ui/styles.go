package ui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Palette ThemePalette
	Styles  ThemeStyles
}

type ThemePalette struct {
	Background                  lipgloss.Color
	Surface                     lipgloss.Color
	SurfaceSoft                 lipgloss.Color
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
	StatusInfoBackground        lipgloss.Color
	StatusSuccessBackground     lipgloss.Color
	StatusErrorBackground       lipgloss.Color
	StatusText                  lipgloss.Color
	BadgeText                   lipgloss.Color
	BadgeBackground             lipgloss.Color
	BadgeMutedBackground        lipgloss.Color
}

type ThemeStyles struct {
	App                lipgloss.Style
	Banner             lipgloss.Style
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
	StatusBar          lipgloss.Style
	StatusBarSuccess   lipgloss.Style
	StatusBarError     lipgloss.Style
	Badge              lipgloss.Style
	BadgeMuted         lipgloss.Style
	BadgeAccent        lipgloss.Style
}

func newDefaultTheme() Theme {
	palette := ThemePalette{
		Background:                  lipgloss.Color("233"),
		Surface:                     lipgloss.Color("235"),
		SurfaceSoft:                 lipgloss.Color("237"),
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
		StatusInfoBackground:        lipgloss.Color("238"),
		StatusSuccessBackground:     lipgloss.Color("29"),
		StatusErrorBackground:       lipgloss.Color("88"),
		StatusText:                  lipgloss.Color("255"),
		BadgeText:                   lipgloss.Color("230"),
		BadgeBackground:             lipgloss.Color("66"),
		BadgeMutedBackground:        lipgloss.Color("239"),
	}
	return newTheme(palette)
}

func newTheme(palette ThemePalette) Theme {
	// 基于 lipgloss README 的 Inheritance 说明，后续如果要重新引入多层背景，需要记住：
	// 1. Inherit 只会继承接收方“尚未设置”的规则，不会覆盖已设置的 foreground/background。
	// 2. 父容器先 Render 出来的背景，不会自动“流入”子组件后续单独 Render 的内容。
	// 3. 如果未来要做真正稳定的多层背景，应先定义共享背景的 base style，再从 base 派生标题、正文、
	//    弱文本、输入框等样式；不要假设在页面上临时补一个 Background 就能让所有嵌套区域自然一致。
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
			Padding(0, 1).
			Foreground(palette.Text),

		Banner: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(palette.PanelBorderFocused).
			Padding(0, 1),

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
			Padding(0, 1),

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
		StatusBar: lipgloss.NewStyle().
			Foreground(palette.StatusText).
			Padding(0, 1),
		StatusBarSuccess: lipgloss.NewStyle().
			Foreground(palette.StatusText).
			Padding(0, 1),
		StatusBarError: lipgloss.NewStyle().
			Foreground(palette.StatusText).
			Padding(0, 1),
		Badge: lipgloss.NewStyle().
			Bold(true).
			Foreground(palette.BadgeText).
			Background(palette.BadgeBackground).
			Padding(0, 1),
		BadgeMuted: lipgloss.NewStyle().
			Bold(true).
			Foreground(palette.Text).
			Background(palette.BadgeMutedBackground).
			Padding(0, 1),
		BadgeAccent: lipgloss.NewStyle().
			Bold(true).
			Foreground(palette.SelectionText).
			Background(palette.SelectionBackground).
			Padding(0, 1),
	}

	return Theme{
		Palette: palette,
		Styles:  styles,
	}
}

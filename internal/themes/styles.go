package themes

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	DefaultName = "dark"
	LightName   = "light"
	DraculaName = "dracula"
	NordName    = "nord"
)

// Palette 只保留当前样式体系真实消费的颜色。
// 不要为了“以后可能会用”预留大块背景/面板底色字段，否则后续排查时容易误判为还有全局铺底方案可走。
type Palette struct {
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
	StatusText                  lipgloss.Color
	BadgeText                   lipgloss.Color
	BadgeBackground             lipgloss.Color
	BadgeMutedBackground        lipgloss.Color
}

// Styles 是 UI 唯一需要消费的最终样式集合。
// 主题名与原始调色板只在本包内部用于构建，不再泄漏到 ui 包。
type Styles struct {
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
	SelectionTitle     lipgloss.Style
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

var registeredPalettes = map[string]Palette{}

// Register 在包初始化阶段注册主题调色板。
// 后续新增主题时只需新增一个 theme_xxx.go 文件并在 init 中调用这里。
func Register(name string, palette Palette) {
	normalized := normalizeName(name)
	if normalized == "" {
		panic("themes: 主题名不能为空")
	}
	if _, exists := registeredPalettes[normalized]; exists {
		panic(fmt.Sprintf("themes: 重复注册主题 %q", normalized))
	}
	registeredPalettes[normalized] = palette
}

func SupportedNames() []string {
	names := make([]string, 0, len(registeredPalettes))
	for name := range registeredPalettes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func ResolveStyles(name string) (Styles, error) {
	normalized := normalizeName(name)
	if normalized == "" {
		normalized = DefaultName
	}

	palette, ok := registeredPalettes[normalized]
	if !ok {
		return Styles{}, fmt.Errorf("unsupported theme %q, supported themes: %s", name, strings.Join(SupportedNames(), ", "))
	}
	return buildStyles(palette), nil
}

func MustStyles(name string) Styles {
	styles, err := ResolveStyles(name)
	if err != nil {
		panic(err)
	}
	return styles
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func buildStyles(palette Palette) Styles {
	// 当前结论：
	// 1. 这个 UI 仍以“局部高亮背景 + 大部分容器透明”为准，不再尝试页面根层/全局 surface 铺底。
	// 2. lipgloss 在这里是 ANSI 字符串拼装，不是可层叠画布；父级背景不会稳定托住子组件后续单独 Render 的内容。
	// 3. 如果未来确实要引入实体背景，必须按容器级方案整体设计并逐层落地，不要恢复“先预留背景字段再局部试验”的状态。
	//
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

	return Styles{
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
		SelectionTitle:     lipgloss.NewStyle().Bold(true).Foreground(palette.SelectionText),
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
		// 状态栏当前只使用文字和已有 Badge/Keycap 背景，不单独引入整条背景，保持与现有页面背景方案一致。
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
}

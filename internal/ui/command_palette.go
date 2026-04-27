package ui

import (
	"sshm/internal/i18n"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func newCommandPaletteState(translator *i18n.Translator, theme Theme) commandPaletteState {
	input := textinput.New()
	input.Placeholder = translator.T("palette.placeholder")
	input.Prompt = translator.T("palette.prompt")
	input.PromptStyle = theme.Styles.SubtleText
	input.PlaceholderStyle = theme.Styles.MutedText
	input.Width = 36
	input.Blur()
	return commandPaletteState{input: input}
}

func (m *Model) openCommandPalette() (tea.Model, tea.Cmd) {
	if !m.commandPaletteSupported() {
		return m, nil
	}
	m.clearStaleErrorStatus()
	m.palette = newCommandPaletteState(m.translator, m.theme)
	m.palette.input.Focus()
	m.overlay = overlayCommandPalette
	return m, nil
}

func (m *Model) closeCommandPalette() {
	m.palette.input.Blur()
	m.overlay = overlayNone
}

func (m *Model) updateCommandPalette(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closeCommandPalette()
		return m, nil
	case "up", "k":
		if m.palette.selected > 0 {
			m.palette.selected--
		}
		return m, nil
	case "down", "j":
		actions := m.filteredCommandActions()
		if m.palette.selected < len(actions)-1 {
			m.palette.selected++
		}
		return m, nil
	case "enter":
		action, ok := m.selectedCommandAction()
		m.closeCommandPalette()
		if !ok {
			return m, nil
		}
		return m.executeCommandAction(action.id)
	}

	var cmd tea.Cmd
	before := m.palette.input.Value()
	m.palette.input, cmd = m.palette.input.Update(msg)
	if before != m.palette.input.Value() {
		m.palette.selected = 0
	}
	return m, cmd
}

func (m *Model) filteredCommandActions() []commandAction {
	actions := m.currentCommandActions()
	query := strings.ToLower(strings.TrimSpace(m.palette.input.Value()))
	if query == "" {
		return actions
	}
	filtered := make([]commandAction, 0, len(actions))
	for _, action := range actions {
		if strings.Contains(action.searchableText(), query) {
			filtered = append(filtered, action)
		}
	}
	return filtered
}

func (m *Model) selectedCommandAction() (commandAction, bool) {
	actions := m.filteredCommandActions()
	if len(actions) == 0 {
		return commandAction{}, false
	}
	index := clamp(m.palette.selected, len(actions))
	m.palette.selected = index
	return actions[index], true
}

func (m *Model) commandPaletteTitle() string {
	switch m.page {
	case pageBrowser:
		return m.translator.T("palette.browser_title")
	default:
		return m.translator.T("palette.home_title")
	}
}

func (m *Model) viewCommandPalette(width int) string {
	styles := m.theme.Styles
	actions := m.filteredCommandActions()
	dialogWidth := min(64, max(42, width-8))
	m.palette.input.Width = max(18, dialogWidth-8)

	lines := []string{
		styles.PageTitle.Render(m.translator.T("palette.title")),
		styles.SubtleText.Render(m.commandPaletteTitle()),
		"",
		m.palette.input.View(),
		"",
	}

	if len(actions) == 0 {
		lines = append(lines, styles.SubtleText.Render(m.translator.T("palette.empty")))
	} else {
		visible := min(8, len(actions))
		start := 0
		if m.palette.selected >= visible {
			start = m.palette.selected - visible + 1
		}
		end := min(len(actions), start+visible)
		for index := start; index < end; index++ {
			lines = append(lines, m.renderCommandActionRow(actions[index], index == m.palette.selected, dialogWidth-6))
		}
	}

	lines = append(lines, "",
		localizedShortcutHelpWidth(m.translator, m.theme, max(24, dialogWidth-6),
			"j/k", "shortcut.move",
			"enter", "shortcut.confirm",
			"esc", "shortcut.cancel",
		),
	)
	return styles.Dialog.Width(dialogWidth).Render(strings.Join(lines, "\n"))
}

func (m *Model) renderCommandActionRow(action commandAction, selected bool, width int) string {
	styles := m.theme.Styles
	titleWidth := max(10, width-10)
	content := lipgloss.JoinHorizontal(
		lipgloss.Center,
		truncate(action.title, titleWidth),
		" ",
		styles.Keycap.Render(action.shortcut),
	)
	if selected {
		return styles.Selection.Copy().Width(width).Render(content)
	}
	return styles.Text.Copy().Width(width).Render(content)
}

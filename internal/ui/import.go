package ui

import (
	"fmt"
	"sshm/internal/domain"
	"sshm/internal/i18n"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func newImportState(translator *i18n.Translator, theme Theme) importState {
	input := textinput.New()
	input.Placeholder = "~/.ssh/config"
	input.Prompt = translator.T("import.path_prompt")
	input.PromptStyle = theme.Styles.SubtleText
	input.PlaceholderStyle = theme.Styles.MutedText
	input.Width = 56
	input.SetValue("~/.ssh/config")
	input.Focus()
	return importState{step: importStepPath, path: input}
}

func (m *Model) updateImport(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch m.imports.step {
		case importStepPath:
			switch keyMsg.String() {
			case "esc":
				m.page = pageHome
				m.setInfoStatus(m.translator.T("status.cancelled"))
				return m, tea.ClearScreen
			case "enter", "ctrl+s":
				m.imports.loading = true
				m.imports.errorText = ""
				return m, m.previewImportCmd(m.imports.path.Value())
			}
			var cmd tea.Cmd
			if m.imports.errorText != "" {
				m.imports.errorText = ""
			}
			m.imports.path, cmd = m.imports.path.Update(msg)
			return m, cmd
		case importStepPreview:
			switch keyMsg.String() {
			case "esc":
				m.page = pageHome
				m.setInfoStatus(m.translator.T("status.cancelled"))
				return m, tea.ClearScreen
			case "up", "k":
				if m.imports.selected > 0 {
					m.imports.errorText = ""
					m.imports.selected--
				}
				return m, nil
			case "down", "j":
				if m.imports.selected < len(m.imports.items)-1 {
					m.imports.errorText = ""
					m.imports.selected++
				}
				return m, nil
			case " ":
				m.imports.errorText = ""
				m.cycleImportAction()
				return m, nil
			case "ctrl+s", "enter":
				m.imports.errorText = ""
				return m, m.applyImportCmd()
			}
		}
	}
	return m, nil
}

func (m *Model) viewImport() string {
	styles := m.theme.Styles
	title := styles.PageTitle.Render(m.translator.T("import.title"))
	if m.imports.step == importStepPath {
		lines := []string{
			title,
			styles.SubtleText.Render(m.translator.T("import.subtitle")),
			"",
			m.imports.path.View(),
			"",
			localizedShortcutHelpWidth(m.translator, m.theme, 72,
				"enter/c-s", "import.shortcut_preview",
				"esc", "import.shortcut_back",
			),
		}
		if m.imports.loading {
			lines = append(lines, "", styles.SubtleText.Render(m.translator.T("import.loading")))
		}
		if m.imports.errorText != "" {
			lines = append(lines, "", styles.ErrorText.Render(m.imports.errorText))
		}
		return styles.Panel.Width(76).Render(strings.Join(lines, "\n"))
	}

	lines := []string{
		title,
		styles.SubtleText.Render(m.translator.T("import.preview_subtitle")),
		"",
	}
	if len(m.imports.items) == 0 {
		lines = append(lines, styles.SubtleText.Render(m.translator.T("import.empty")))
	} else {
		visible := 12
		start := 0
		if m.imports.selected >= visible {
			start = m.imports.selected - visible + 1
		}
		end := min(len(m.imports.items), start+visible)
		for index := start; index < end; index++ {
			lines = append(lines, m.renderImportRow(m.imports.items[index], index == m.imports.selected, 72))
		}
	}
	if len(m.imports.warnings) > 0 {
		lines = append(lines, "", styles.WarningText.Render(truncate(strings.Join(m.imports.warnings, "；"), 72)))
	}
	if m.imports.errorText != "" {
		lines = append(lines, "", styles.ErrorText.Render(m.imports.errorText))
	}
	lines = append(lines, "", localizedShortcutHelpWidth(m.translator, m.theme, 72,
		"j/k", "import.shortcut_move",
		"space", "import.shortcut_action",
		"enter/c-s", "import.shortcut_import",
		"esc", "import.shortcut_back",
	))
	return styles.Panel.Width(80).Render(strings.Join(lines, "\n"))
}

func (m *Model) renderImportRow(item domain.ImportCandidate, selected bool, width int) string {
	styles := m.theme.Styles
	action := m.importActionLabel(item.Action)
	group := item.GroupName
	if strings.TrimSpace(group) == "" {
		group = m.translator.T("group.ungrouped")
	}
	status := action
	if item.ExistingID != 0 {
		status = status + " / " + m.translator.T("import.conflict")
	}
	if item.Skipped {
		status = m.translator.T("import.skipped")
	}
	line := fmt.Sprintf("%-12s %-12s %s@%s:%d", status, group, item.Connection.Username, item.Connection.Host, item.Connection.Port)
	name := item.Connection.Name
	if len(item.Warnings) > 0 {
		name += "  " + strings.Join(item.Warnings, "；")
	}
	style := styles.Text.Copy().Width(width)
	metaStyle := styles.SubtleText.Copy().Width(width)
	if selected {
		style = styles.Selection.Copy().Width(width)
		metaStyle = styles.SelectionDetail.Copy().Width(width)
	}
	return strings.Join([]string{
		style.Render(truncate(name, width)),
		metaStyle.Render(truncate(line, width)),
	}, "\n")
}

func (m *Model) importActionLabel(action domain.ImportAction) string {
	switch action {
	case domain.ImportActionCreate:
		return m.translator.T("import.action_create")
	case domain.ImportActionUpdate:
		return m.translator.T("import.action_update")
	case domain.ImportActionCopy:
		return m.translator.T("import.action_copy")
	default:
		return m.translator.T("import.action_skip")
	}
}

func (m *Model) cycleImportAction() {
	if len(m.imports.items) == 0 || m.imports.selected < 0 || m.imports.selected >= len(m.imports.items) {
		return
	}
	item := &m.imports.items[m.imports.selected]
	if item.Skipped {
		return
	}
	if item.ExistingID != 0 {
		switch item.Action {
		case domain.ImportActionSkip:
			item.Action = domain.ImportActionUpdate
		case domain.ImportActionUpdate:
			item.Action = domain.ImportActionCopy
		default:
			item.Action = domain.ImportActionSkip
		}
		return
	}
	if item.Action == domain.ImportActionCreate {
		item.Action = domain.ImportActionSkip
	} else {
		item.Action = domain.ImportActionCreate
	}
}

func (m *Model) previewImportCmd(path string) tea.Cmd {
	return func() tea.Msg {
		preview, err := m.services.Imports.PreviewSSHConfig(path)
		return importPreviewMsg{preview: preview, err: err}
	}
}

func (m *Model) applyImportCmd() tea.Cmd {
	preview := domain.ImportPreview{Candidates: append([]domain.ImportCandidate{}, m.imports.items...), Warnings: append([]string{}, m.imports.warnings...)}
	return func() tea.Msg {
		summary, err := m.services.Imports.Apply(preview)
		msg := importDoneMsg{summary: summary, err: err, reloadConnections: true}
		if err != nil {
			return msg
		}
		msg.setScope = true
		msg.scope = domain.ConnectionListScopeAll
		groupName, ok := singleImportedGroup(preview.Candidates)
		if ok {
			if groupName == "" {
				msg.scope = domain.ConnectionListScopeUngrouped
				msg.groupName = m.translator.T("group.ungrouped")
			} else {
				groups, err := m.services.Groups.List()
				if err == nil {
					for _, group := range groups {
						if !group.Ungrouped && group.Name == groupName {
							msg.scope = domain.ConnectionListScopeGroup
							msg.groupID = group.ID
							msg.groupName = group.Name
							break
						}
					}
				}
			}
		}
		return msg
	}
}

func singleImportedGroup(items []domain.ImportCandidate) (string, bool) {
	groupName := ""
	seen := false
	for _, item := range items {
		if item.Skipped || item.Action == domain.ImportActionSkip {
			continue
		}
		name := strings.TrimSpace(item.GroupName)
		if !seen {
			groupName = name
			seen = true
			continue
		}
		if groupName != name {
			return "", false
		}
	}
	return groupName, seen
}

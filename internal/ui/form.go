package ui

import (
	"sshm/internal/domain"
	"sshm/internal/i18n"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.page = pageHome
			m.form = newFormState(nil, m.translator, m.defaultPrivateKeyPath, m.theme)
			m.setInfoStatus(m.translator.T("status.cancelled"))
			return m, tea.ClearScreen
		case "tab", "down":
			m.form.focusIndex = m.form.nextFocus()
			m.form.applyFocus()
			return m, nil
		case "shift+tab", "up":
			m.form.focusIndex = m.form.prevFocus()
			m.form.applyFocus()
			return m, nil
		case "left", "right", " ":
			if m.form.focusIndex == 4 {
				if m.form.authType == domain.AuthTypePassword {
					m.form.authType = domain.AuthTypePrivateKey
					m.form.keepPassword = false
				} else {
					m.form.authType = domain.AuthTypePassword
					m.form.keepPassword = m.form.canKeepPassword()
				}
			}
			return m, nil
		case "ctrl+s":
			return m.submitForm()
		case "enter":
			if m.form.focusIndex == 7 {
				return m.submitForm()
			}
			if m.form.focusIndex == 8 {
				m.page = pageHome
				return m, tea.ClearScreen
			}
			m.form.focusIndex = m.form.nextFocus()
			m.form.applyFocus()
			return m, nil
		}
	}

	var cmd tea.Cmd
	switch m.form.focusIndex {
	case 0:
		m.form.name, cmd = m.form.name.Update(msg)
	case 1:
		m.form.host, cmd = m.form.host.Update(msg)
	case 2:
		m.form.port, cmd = m.form.port.Update(msg)
	case 3:
		m.form.username, cmd = m.form.username.Update(msg)
	case 5:
		m.form.description, cmd = m.form.description.Update(msg)
	case 6:
		if m.form.authType == domain.AuthTypePassword {
			m.form.password, cmd = m.form.password.Update(msg)
		} else {
			m.form.privateKeyPath, cmd = m.form.privateKeyPath.Update(msg)
		}
	}
	return m, cmd
}

func (m *Model) submitForm() (tea.Model, tea.Cmd) {
	port, _ := strconv.Atoi(strings.TrimSpace(m.form.port.Value()))
	if m.form.editingID == 0 {
		input := domain.ConnectionInput{
			GroupID:        m.form.groupID,
			Name:           m.form.name.Value(),
			Host:           m.form.host.Value(),
			Port:           port,
			Username:       m.form.username.Value(),
			AuthType:       m.form.authType,
			PrivateKeyPath: m.form.privateKeyPath.Value(),
			Description:    m.form.description.Value(),
			Password:       m.form.password.Value(),
		}
		conn, err := m.services.Connections.Create(input)
		if err != nil {
			m.form.errorMessage = m.translator.Error(err)
			return m, nil
		}
		m.page = pageHome
		m.setSuccessStatus(m.translator.T("status.created_connection", conn.Name))
		return m, m.loadConnectionsCmd()
	}

	input := domain.ConnectionUpdateInput{
		GroupID:        m.form.groupID,
		Name:           m.form.name.Value(),
		Host:           m.form.host.Value(),
		Port:           port,
		Username:       m.form.username.Value(),
		AuthType:       m.form.authType,
		PrivateKeyPath: m.form.privateKeyPath.Value(),
		Description:    m.form.description.Value(),
		Password:       m.form.password.Value(),
		KeepPassword:   m.form.shouldKeepPassword(),
	}
	conn, err := m.services.Connections.Update(m.form.editingID, input)
	if err != nil {
		m.form.errorMessage = m.translator.Error(err)
		return m, nil
	}
	m.page = pageHome
	m.setSuccessStatus(m.translator.T("status.updated_connection", conn.Name))
	return m, m.loadConnectionsCmd()
}

func (m *Model) viewForm() string {
	styles := m.theme.Styles
	contentWidth := 76
	if m.width > 0 {
		contentWidth = min(84, max(64, m.width-10))
	}

	title := m.translator.T("form.add_title")
	subtitle := m.translator.T("form.add_subtitle")
	if m.form.editingID != 0 {
		title = m.translator.T("form.edit_title")
		subtitle = m.translator.T("form.edit_subtitle")
	}

	lines := []string{
		styles.PageTitle.Render(title),
		styles.SubtleText.Render(subtitle),
		"",
		m.renderInlineField(0, m.translator.T("form.name")),
		m.renderInlineField(1, m.translator.T("form.host")),
		m.renderInlineField(2, m.translator.T("form.port")),
		m.renderInlineField(3, m.translator.T("form.username")),
		m.renderAuthField(),
	}
	if m.form.authType == domain.AuthTypePassword {
		lines = append(lines, m.renderInlineField(6, m.translator.T("form.password")))
		if m.form.canKeepPassword() {
			lines = append(lines, styles.SubtleText.Render(m.translator.T("form.password_keep_hint")))
		}
	} else {
		lines = append(lines, m.renderInlineField(6, m.translator.T("form.key_path")))
	}

	lines = append(lines,
		"",
		m.renderInlineField(5, m.translator.T("form.description")),
		"",
		styles.ActionBar.Render(m.renderButtons()),
	)
	if m.form.errorMessage != "" {
		lines = append(lines, "", styles.ErrorText.Render(m.form.errorMessage))
	}
	lines = append(lines,
		"",
		localizedShortcutHelpWidth(m.translator, m.theme, max(24, contentWidth-4),
			"tab/s-tab", "form.shortcut_move",
			"enter", "form.shortcut_next_save",
			"c-s", "form.shortcut_save",
			"esc", "form.shortcut_cancel",
		),
	)

	return styles.Panel.Width(contentWidth).Render(strings.Join(lines, "\n"))
}

func (m *Model) renderInlineField(index int, label string) string {
	styles := m.theme.Styles
	labelStyle := styles.FieldLabel
	contentStyle := styles.Input
	promptStyle := styles.SubtleText
	if m.form.focusIndex == index {
		labelStyle = styles.FieldLabelFocused
		contentStyle = styles.InputFocused
		promptStyle = styles.PageTitle
	}
	labelWidth := m.formLabelWidth()
	var content string
	if index == 0 {
		m.form.name.PromptStyle = promptStyle
		content = m.form.name.View()
	} else if index == 1 {
		m.form.host.PromptStyle = promptStyle
		content = m.form.host.View()
	} else if index == 2 {
		m.form.port.PromptStyle = promptStyle
		content = m.form.port.View()
	} else if index == 3 {
		m.form.username.PromptStyle = promptStyle
		content = m.form.username.View()
	} else if index == 5 {
		m.form.description.PromptStyle = promptStyle
		content = m.form.description.View()
	} else if index == 6 {
		if m.form.authType == domain.AuthTypePassword {
			m.form.password.PromptStyle = promptStyle
			content = m.form.password.View()
		} else {
			m.form.privateKeyPath.PromptStyle = promptStyle
			content = m.form.privateKeyPath.View()
		}
	}
	row := lipgloss.JoinHorizontal(
		lipgloss.Center,
		labelStyle.Width(labelWidth).Align(lipgloss.Right).Render(label+":"),
		contentStyle.Render(content),
	)
	return lipgloss.NewStyle().MarginBottom(1).Render(row)
}

func (m *Model) renderAuthField() string {
	styles := m.theme.Styles
	password := "( ) " + m.translator.T("form.password")
	key := "( ) " + m.translator.T("form.private_key")
	if m.form.authType == domain.AuthTypePassword {
		password = "(●) " + m.translator.T("form.password")
	} else {
		key = "(●) " + m.translator.T("form.private_key")
	}

	choiceStyle := styles.SubtleText
	labelStyle := styles.FieldLabel
	contentStyle := styles.Input
	if m.form.focusIndex == 4 {
		choiceStyle = styles.PageTitle
		labelStyle = styles.FieldLabelFocused
		contentStyle = styles.InputFocused
	}

	choices := lipgloss.JoinHorizontal(lipgloss.Left, choiceStyle.Render(password), "   ", choiceStyle.Render(key))
	labelWidth := m.formLabelWidth()
	return lipgloss.NewStyle().MarginBottom(1).Render(lipgloss.JoinHorizontal(
		lipgloss.Center,
		labelStyle.Width(labelWidth).Align(lipgloss.Right).Render(m.translator.T("form.auth_type")+":"),
		contentStyle.Render(choices),
	))
}

func (m *Model) renderButtons() string {
	styles := m.theme.Styles
	save := "[ " + m.translator.T("form.save") + " ]"
	cancel := "[ " + m.translator.T("form.cancel") + " ]"
	if m.form.focusIndex == 7 {
		save = styles.Selection.Render(save)
	}
	if m.form.focusIndex == 8 {
		cancel = styles.Selection.Render(cancel)
	}
	return save + "  " + cancel
}

func (m *Model) formLabelWidth() int {
	return 12
}

func newFormState(conn *domain.Connection, translator *i18n.Translator, defaultPrivateKeyPath string, theme Theme) formState {
	state := formState{
		authType:         domain.AuthTypePassword,
		originalAuthType: domain.AuthTypePassword,
	}
	state.name = newInput(theme, translator.T("form.name"), 40)
	state.host = newInput(theme, translator.T("form.host"), 40)
	state.port = newInput(theme, translator.T("form.port"), 8)
	state.username = newInput(theme, translator.T("form.username"), 24)
	state.description = newInput(theme, translator.T("form.description"), 48)
	state.password = newInput(theme, translator.T("form.password"), 32)
	state.password.EchoMode = textinput.EchoPassword
	state.password.EchoCharacter = '•'
	state.privateKeyPath = newInput(theme, translator.T("form.key_path"), 48)
	state.port.SetValue("22")
	state.privateKeyPath.SetValue(defaultPrivateKeyPath)

	if conn != nil {
		state.editingID = conn.ID
		state.groupID = conn.GroupID
		state.name.SetValue(conn.Name)
		state.host.SetValue(conn.Host)
		state.port.SetValue(strconv.Itoa(conn.Port))
		state.username.SetValue(conn.Username)
		state.description.SetValue(conn.Description)
		state.privateKeyPath.SetValue(conn.PrivateKeyPath)
		state.authType = conn.AuthType
		state.originalAuthType = conn.AuthType
		if conn.AuthType == domain.AuthTypePassword {
			state.keepPassword = true
		}
	}
	state.applyFocus()
	return state
}

func (f *formState) canKeepPassword() bool {
	return f.editingID != 0 && f.originalAuthType == domain.AuthTypePassword && f.authType == domain.AuthTypePassword
}

func (f *formState) shouldKeepPassword() bool {
	return f.canKeepPassword() && (f.keepPassword || strings.TrimSpace(f.password.Value()) == "")
}

func (f *formState) fields() []int {
	return []int{0, 1, 2, 3, 4, 6, 5, 7, 8}
}

func (f *formState) nextFocus() int {
	fields := f.fields()
	for index, field := range fields {
		if field == f.focusIndex {
			return fields[(index+1)%len(fields)]
		}
	}
	return fields[0]
}

func (f *formState) prevFocus() int {
	fields := f.fields()
	for index, field := range fields {
		if field == f.focusIndex {
			return fields[(index-1+len(fields))%len(fields)]
		}
	}
	return fields[0]
}

func (f *formState) applyFocus() {
	inputs := []*textinput.Model{&f.name, &f.host, &f.port, &f.username, &f.description, &f.password, &f.privateKeyPath}
	for _, input := range inputs {
		input.Blur()
	}
	switch f.focusIndex {
	case 0:
		f.name.Focus()
	case 1:
		f.host.Focus()
	case 2:
		f.port.Focus()
	case 3:
		f.username.Focus()
	case 5:
		f.description.Focus()
	case 6:
		if f.authType == domain.AuthTypePassword {
			f.password.Focus()
		} else {
			f.privateKeyPath.Focus()
		}
	}
}

package tui

import (
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"sshm/internal/i18n"
	"sshm/internal/repository"
)

const (
	connectionNameField = iota
	hostField
	portField
	usernameField
	remarkField
	connectionCredentialField
	connectionFieldCount
)

var (
	formLabelStyle     = mutedStyle.Width(13)
	formActiveLabel    = accentStyle.Width(13).Bold(false)
	formInputStyle     = lipgloss.NewStyle().Width(35).Padding(0, 1)
	formInputFocused   = formInputStyle.Foreground(textColor)
	formSectionStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA1AE"))
	formChoiceStyle    = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("#9AA1AE"))
	formChoiceSelected = formChoiceStyle.Foreground(lipgloss.Color("#B5A7FF")).Bold(true).Underline(true)
	formErrorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B"))
)

// connectionCreatedMsg 通知主界面新增连接已经持久化。
type connectionCreatedMsg struct {
	connection repository.Connection
}

type connectionUpdatedMsg struct{ connection repository.Connection }
type connectionDeletedMsg struct{ id int64 }

// connectionCreateFailedMsg 将异步保存错误交回连接表单显示。
type connectionCreateFailedMsg struct {
	err error
}

// connectionFormModel 只管理连接字段，凭据通过独立弹窗选择或创建。
type connectionFormModel struct {
	store          *repository.Store
	connectionName textinput.Model
	host           textinput.Model
	port           textinput.Model
	username       textinput.Model
	remark         textinput.Model
	credential     *repository.Credential
	focus          int
	err            string
	saving         bool
	connectionID   int64
	editing        bool
}

func newConnectionForm(store *repository.Store) connectionFormModel {
	m := connectionFormModel{
		store:          store,
		connectionName: newFormInput(i18n.T("e.g. production server"), 24),
		host:           newFormInput(i18n.T("host name or 192.168.1.10"), 125),
		port:           newFormInput("22", 5),
		username:       newFormInput(i18n.T("e.g. root"), 24),
		remark:         newFormInput(i18n.T("optional"), 256),
	}
	m.port.SetValue("22")
	m.connectionName.Focus()
	return m
}

func newConnectionEditForm(store *repository.Store, connection connectionRow) connectionFormModel {
	m := newConnectionForm(store)
	m.connectionID, m.editing = connection.id, true
	m.connectionName.SetValue(connection.name)
	m.host.SetValue(connection.host)
	m.port.SetValue(connection.port)
	m.username.SetValue(connection.username)
	m.remark.SetValue(connection.remark)
	m.credential = &repository.Credential{ID: connection.credID, Name: connection.credName, Type: authCode(connection.auth)}
	return m
}

func newFormInput(placeholder string, limit int) textinput.Model {
	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = placeholder
	input.CharLimit = limit
	input.SetWidth(33)
	styles := input.Styles()
	styles.Focused.Text = plainStyle
	styles.Focused.Placeholder = mutedStyle
	styles.Blurred.Text = plainStyle
	styles.Blurred.Placeholder = mutedStyle
	styles.Cursor.Color = lipgloss.Color("#e7e7e7")
	input.SetStyles(styles)
	return input
}

// Update 处理连接字段导航，并在凭据字段打开独立的凭据选择弹窗。
func (m connectionFormModel) Update(msg tea.Msg) (modalModel, tea.Cmd) {
	if result, ok := msg.(connectionCreateFailedMsg); ok {
		m.saving = false
		m.err = result.err.Error()
		return m, nil
	}
	if m.saving {
		return m, nil
	}
	if isClearInputKey(msg) {
		switch m.focus {
		case connectionNameField:
			m.connectionName.Reset()
		case hostField:
			m.host.Reset()
		case portField:
			m.port.Reset()
		case usernameField:
			m.username.Reset()
		case remarkField:
			m.remark.Reset()
		}
		m.err = ""
		return m, nil
	}
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "esc":
			return m, func() tea.Msg { return closeModalMsg{} }
		case "tab", "down":
			return m.moveFocus(1)
		case "shift+tab", "up":
			return m.moveFocus(-1)
		case "ctrl+s":
			return m.save()
		case "enter", "space", " ":
			if m.focus == connectionCredentialField {
				return newCredentialPicker(m.store, m), nil
			}
			if key.String() == "enter" {
				return m.moveFocus(1)
			}
		}
	}
	m.err = ""
	return m.updateInput(msg)
}

func (m connectionFormModel) moveFocus(step int) (modalModel, tea.Cmd) {
	m.blurInputs()
	m.focus = (m.focus + step + connectionFieldCount) % connectionFieldCount
	return m, m.focusInput()
}

func (m *connectionFormModel) blurInputs() {
	m.connectionName.Blur()
	m.host.Blur()
	m.port.Blur()
	m.username.Blur()
	m.remark.Blur()
}

func (m *connectionFormModel) focusInput() tea.Cmd {
	switch m.focus {
	case connectionNameField:
		return m.connectionName.Focus()
	case hostField:
		return m.host.Focus()
	case portField:
		return m.port.Focus()
	case usernameField:
		return m.username.Focus()
	case remarkField:
		return m.remark.Focus()
	}
	return nil
}

func (m connectionFormModel) updateInput(msg tea.Msg) (modalModel, tea.Cmd) {
	var cmd tea.Cmd
	switch m.focus {
	case connectionNameField:
		m.connectionName, cmd = m.connectionName.Update(msg)
	case hostField:
		m.host, cmd = m.host.Update(msg)
	case portField:
		m.port, cmd = m.port.Update(msg)
	case usernameField:
		m.username, cmd = m.username.Update(msg)
	case remarkField:
		m.remark, cmd = m.remark.Update(msg)
	}
	return m, cmd
}

// save 校验连接字段，并仅使用已选凭据 ID 创建连接。
func (m connectionFormModel) save() (modalModel, tea.Cmd) {
	port, err := strconv.Atoi(strings.TrimSpace(m.port.Value()))
	if err != nil || port < 1 || port > 65535 {
		m.err = i18n.T("Port must be between 1 and 65535")
		return m, nil
	}
	if strings.TrimSpace(m.connectionName.Value()) == "" || strings.TrimSpace(m.host.Value()) == "" || strings.TrimSpace(m.username.Value()) == "" {
		m.err = i18n.T("Name, host, and user are required")
		return m, nil
	}
	if m.credential == nil {
		m.err = i18n.T("Select a login credential")
		return m, nil
	}
	if m.store == nil {
		m.err = i18n.T("Database is not connected")
		return m, nil
	}
	input := repository.NewConnection{
		Name: m.connectionName.Value(), Host: m.host.Value(), Port: port,
		Username: m.username.Value(), CredentialID: m.credential.ID, Remark: m.remark.Value(),
	}
	m.saving = true
	m.err = ""
	return m, func() tea.Msg {
		if m.editing {
			connection, err := m.store.UpdateConnection(m.connectionID, input)
			if err != nil {
				return connectionCreateFailedMsg{err: err}
			}
			return connectionUpdatedMsg{connection: connection}
		}
		connection, err := m.store.CreateConnection(input)
		if err != nil {
			return connectionCreateFailedMsg{err: err}
		}
		return connectionCreatedMsg{connection: connection}
	}
}

func (m connectionFormModel) View() string {
	lines := []string{
		modalTitleStyle.Render(map[bool]string{true: i18n.T("Edit connection"), false: i18n.T("New connection")}[m.editing]),
		borderStyle.Render(strings.Repeat("─", modalContentWidth)),
		m.inputRow(i18n.T("Name"), m.connectionName.View(), connectionNameField),
		"",
		m.inputRow(i18n.T("Host"), m.host.View(), hostField),
		"",
		m.inputRow(i18n.T("Port"), m.port.View(), portField),
		"",
		m.inputRow(i18n.T("User"), m.username.View(), usernameField),
		"",
		m.inputRow(i18n.T("Remark"), m.remark.View(), remarkField),
		"",
		m.credentialRow(),
		borderStyle.Render(strings.Repeat("─", modalContentWidth)),
	}
	status := i18n.T("Ctrl+S: Save | Esc: Cancel")
	if m.err != "" {
		status = formErrorStyle.Render(m.err)
	} else if m.saving {
		status = accentStyle.Render(i18n.T("Saving..."))
	} else {
		status = mutedStyle.Render(status)
	}
	return modalStyle.Render(strings.Join(append(lines, status), "\n"))
}

func (m connectionFormModel) inputRow(label, value string, field int) string {
	labelStyle, inputStyle := formLabelStyle, formInputStyle
	if m.focus == field {
		labelStyle, inputStyle = formActiveLabel, formInputFocused
		label = label + " >"
	}
	return "  " + labelStyle.Render(label) + inputStyle.Render(value)
}

func (m connectionFormModel) credentialRow() string {
	value := i18n.T("None") + mutedStyle.Render(i18n.T(" (Enter to choose)"))
	if m.credential != nil {
		kind := authLabel(m.credential.Type)
		value = "[" + kind + "]" + m.credential.Name
	}
	return m.inputRow(i18n.T("Credential"), value, connectionCredentialField)
}

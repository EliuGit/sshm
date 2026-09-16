package tui

import (
	"errors"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"sshm/internal/repository"
)

const (
	oldPasswordField = iota
	newPasswordField
	repeatPasswordField
	passwordFieldCount
)

type passwordChangedMsg struct{}
type passwordChangeFailedMsg struct{ err error }

func passwordErrorMessage(err error) string {
	if errors.Is(err, repository.ErrInvalidPassword) {
		return "密码不正确"
	}
	return err.Error()
}

// passwordFormModel 管理应用主密码修改表单。
type passwordFormModel struct {
	changePassword func([]byte, []byte) error
	inputs         [passwordFieldCount]textinput.Model
	focus          int
	err            string
	saving         bool
}

func newPasswordForm(changePassword func([]byte, []byte) error) passwordFormModel {
	m := passwordFormModel{changePassword: changePassword}
	placeholders := [...]string{"当前应用密码", "新的应用密码", "再次输入新密码"}
	for i := range m.inputs {
		m.inputs[i] = newFormInput(placeholders[i], 0)
		m.inputs[i].EchoMode = textinput.EchoPassword
		m.inputs[i].EchoCharacter = '•'
	}
	m.inputs[oldPasswordField].Focus()
	return m
}

// Update 处理密码字段导航、校验和修改操作。
func (m passwordFormModel) Update(msg tea.Msg) (modalModel, tea.Cmd) {
	if result, ok := msg.(passwordChangeFailedMsg); ok {
		m.saving = false
		m.err = passwordErrorMessage(result.err)
		return m, nil
	}
	if m.saving {
		return m, nil
	}
	if isClearInputKey(msg) {
		m.inputs[m.focus].Reset()
		m.err = ""
		return m, nil
	}
	if key, ok := msg.(tea.KeyPressMsg); ok {
		m.err = ""
		switch key.String() {
		case "esc":
			return m, func() tea.Msg { return closeModalMsg{} }
		case "tab", "down":
			return m.moveFocus(1)
		case "shift+tab", "up":
			return m.moveFocus(-1)
		case "ctrl+s":
			return m.save()
		case "enter":
			return m.moveFocus(1)
		}
	}
	var cmd tea.Cmd
	m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
	return m, cmd
}

func (m passwordFormModel) moveFocus(step int) (modalModel, tea.Cmd) {
	for i := range m.inputs {
		m.inputs[i].Blur()
	}
	m.focus = (m.focus + step + passwordFieldCount) % passwordFieldCount
	return m, m.inputs[m.focus].Focus()
}

func (m passwordFormModel) save() (modalModel, tea.Cmd) {
	oldPassword := m.inputs[oldPasswordField].Value()
	newPassword := m.inputs[newPasswordField].Value()
	if oldPassword == "" || newPassword == "" || m.inputs[repeatPasswordField].Value() == "" {
		m.err = "原密、新密和重复不能为空"
		return m, nil
	}
	if newPassword != m.inputs[repeatPasswordField].Value() {
		m.err = "新密与重复不一致"
		return m, nil
	}
	oldBytes, newBytes := []byte(oldPassword), []byte(newPassword)
	m.saving = true
	m.err = ""
	return m, func() tea.Msg {
		defer clear(oldBytes)
		defer clear(newBytes)
		if err := m.changePassword(oldBytes, newBytes); err != nil {
			return passwordChangeFailedMsg{err: err}
		}
		return passwordChangedMsg{}
	}
}

func (m passwordFormModel) View() string {
	labels := [...]string{"原密", "新密", "重复"}
	lines := []string{
		modalTitleStyle.Render("修改应用密码"),
		borderStyle.Render(strings.Repeat("─", modalContentWidth)),
	}
	for i, label := range labels {
		lines = append(lines, m.inputRow(label, m.inputs[i].View(), i), "")
	}
	lines = append(lines[:len(lines)-1], borderStyle.Render(strings.Repeat("─", modalContentWidth)))
	status := "Ctrl+S 保存 | Esc 取消"
	style := mutedStyle
	if m.err != "" {
		status, style = m.err, formErrorStyle
	} else if m.saving {
		status, style = "正在修改...", accentStyle
	}
	return modalStyle.Render(strings.Join(append(lines, style.Render(status)), "\n"))
}

func (m passwordFormModel) inputRow(label, value string, field int) string {
	labelStyle, inputStyle := formLabelStyle, formInputStyle
	if m.focus == field {
		labelStyle, inputStyle = formActiveLabel, formInputFocused
		label += " >"
	}
	return "  " + labelStyle.Render(label) + inputStyle.Render(value)
}

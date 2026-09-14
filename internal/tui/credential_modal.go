package tui

import (
	"bytes"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"sshm/internal/repository"
)

const pickerRows = 7

const (
	credentialNameField = iota
	credentialTypeField
	credentialContentField
	credentialFieldCount
)

// credentialPickerModel 管理登录凭据，从连接表单进入时还可选择凭据。
type credentialPickerModel struct {
	store         *repository.Store
	parent        connectionFormModel
	selectable    bool
	credentials   []repository.Credential
	searchInput   textinput.Model
	selected      int
	searchFocused bool
	confirmDelete bool
	err           string
}

func newCredentialPicker(store *repository.Store, parent connectionFormModel) credentialPickerModel {
	searchInput := newSearchInput()
	searchInput.Placeholder = "按 / 搜索凭据..."
	searchInput.SetWidth(24)
	m := credentialPickerModel{store: store, parent: parent, selectable: true, searchInput: searchInput}
	return m.reload()
}

func newCredentialManager(store *repository.Store) credentialPickerModel {
	m := newCredentialPicker(store, connectionFormModel{})
	m.selectable = false
	return m
}

func (m credentialPickerModel) reload() credentialPickerModel {
	if m.store == nil {
		m.err = "数据库未连接"
		return m
	}
	credentials, err := m.store.ListCredentials()
	if err != nil {
		m.err = "读取凭据列表失败：" + err.Error()
		return m
	}
	m.credentials = credentials
	if m.parent.credential != nil {
		selectedID := m.parent.credential.ID
		for i := range credentials {
			if credentials[i].ID == selectedID {
				m.selected = i
				break
			}
		}
	}
	visible := m.visibleCredentials()
	if m.selected >= len(visible) {
		m.selected = max(0, len(visible)-1)
	}
	m.err = ""
	return m
}

func (m credentialPickerModel) visibleCredentials() []repository.Credential {
	query := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))
	if query == "" {
		return m.credentials
	}
	filtered := make([]repository.Credential, 0, len(m.credentials))
	for _, credential := range m.credentials {
		if strings.Contains(strings.ToLower(credential.Name), query) {
			filtered = append(filtered, credential)
		}
	}
	return filtered
}

func (m credentialPickerModel) currentCredential() (repository.Credential, bool) {
	credentials := m.visibleCredentials()
	if m.selected < 0 || m.selected >= len(credentials) {
		return repository.Credential{}, false
	}
	return credentials[m.selected], true
}

// Update 处理凭据列表的筛选、增删改和选择。
func (m credentialPickerModel) Update(msg tea.Msg) (modalModel, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if m.searchFocused {
		if isClearInputKey(msg) {
			m.searchInput.Reset()
			m.selected = 0
			return m, nil
		}
		switch key.String() {
		case "esc":
			m.searchInput.SetValue("")
			m.selected = 0
			m.searchFocused = false
			m.searchInput.Blur()
			return m, nil
		case "enter":
			m.searchFocused = false
			m.searchInput.Blur()
			return m, nil
		}
		before := m.searchInput.Value()
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		if m.searchInput.Value() != before {
			m.selected = 0
		}
		return m, cmd
	}
	if m.confirmDelete {
		switch key.String() {
		case "y":
			credential, ok := m.currentCredential()
			m.confirmDelete = false
			if !ok {
				return m, nil
			}
			if err := m.store.DeleteCredential(credential.ID); err != nil {
				m.err = err.Error()
				return m, nil
			}
			return m.reload(), nil
		case "n", "esc":
			m.confirmDelete = false
		}
		return m, nil
	}
	m.err = ""
	switch key.String() {
	case "esc", "q":
		if !m.selectable {
			return m, func() tea.Msg { return closeModalMsg{} }
		}
		return m.parent, m.parent.focusInput()
	case "/":
		m.searchFocused = true
		return m, m.searchInput.Focus()
	case "up", "k":
		if m.selected > 0 {
			m.selected--
		}
	case "down", "j":
		if m.selected < len(m.visibleCredentials())-1 {
			m.selected++
		}
	case "a":
		return newCredentialForm(m, nil), nil
	case "e":
		if credential, ok := m.currentCredential(); ok {
			return newCredentialForm(m, &credential), nil
		}
	case "d":
		if credential, ok := m.currentCredential(); ok {
			if credential.ConnectionCount > 0 {
				m.err = fmt.Sprintf("该凭据仍关联 %d 个连接，不能删除", credential.ConnectionCount)
			} else {
				m.confirmDelete = true
			}
		}
	case "enter":
		if m.selectable {
			credential, ok := m.currentCredential()
			if !ok {
				break
			}
			m.parent.credential = &credential
			m.parent.err = ""
			return m.parent, m.parent.focusInput()
		}
	}
	return m, nil
}

func (m credentialPickerModel) View() string {
	if m.confirmDelete {
		credential, _ := m.currentCredential()
		return formDialog("删除凭据", fmt.Sprintf("确认删除凭据“%s”？", credential.Name), "y 确认 | n/Esc 取消", confirmStyle, modalTitleStyle)
	}
	titleStyle := modalTitleStyle
	if m.err != "" {
		titleStyle = formErrorStyle
	}
	searchMarker := mutedStyle.Render(">")
	if m.searchFocused {
		searchMarker = accentStyle.Render(">")
	}
	lines := []string{
		titleStyle.Render("凭据管理"),
		borderStyle.Render(strings.Repeat("─", modalContentWidth)),
		"  " + searchMarker + " " + m.searchInput.View(),
		"",
	}
	credentials := m.visibleCredentials()
	start := max(0, m.selected-pickerRows+1)
	end := min(len(credentials), start+pickerRows)
	for i := start; i < end; i++ {
		kind := map[string]string{"passwd": "密码", "key": "私钥"}[credentials[i].Type]
		item := fmt.Sprintf("[%s] %s | %d", kind, credentials[i].Name, credentials[i].ConnectionCount)
		prefix, style := "  ", plainStyle
		if i == m.selected {
			prefix, style = "› ", accentStyle
		}
		lines = append(lines, fit(style.Render(prefix+item), modalContentWidth))
	}
	if len(credentials) == 0 {
		lines = append(lines, mutedStyle.Render("  无匹配凭据"))
	}
	for len(lines) < pickerRows+4 {
		lines = append(lines, "")
	}
	status := "a 新增 | e 编辑 | d 删除 | Esc 返回"
	if m.selectable {
		status = "Enter 选择 | " + status
	}
	statusStyle := mutedStyle
	if m.searchFocused {
		status = "Enter 完成 | Esc 清除"
	}
	if m.err != "" {
		status = m.err
		statusStyle = formErrorStyle
	}
	lines = append(lines, borderStyle.Render(strings.Repeat("─", modalContentWidth)), statusStyle.Render(status))
	return modalStyle.Render(strings.Join(lines, "\n"))
}

// credentialSavedMsg 通知凭据表单保存成功。
type credentialSavedMsg struct{ credential repository.Credential }

// credentialSaveFailedMsg 将异步保存错误交回凭据表单显示。
type credentialSaveFailedMsg struct{ err error }

// credentialFormModel 新增或编辑密码、私钥凭据，编辑时不读取原密文。
type credentialFormModel struct {
	manager        credentialPickerModel
	credential     *repository.Credential
	credentialType string
	name           textinput.Model
	password       textinput.Model
	privateKey     textarea.Model
	focus          int
	err            string
	saving         bool
}

func newCredentialForm(manager credentialPickerModel, credential *repository.Credential) credentialFormModel {
	credentialType := "passwd"
	name := newFormInput("用于识别和复用", 24)
	password := newFormInput("SSH 登录密码", 20)
	privateKey := newKeyInput()
	if credential != nil {
		credentialType = credential.Type
		name.SetValue(credential.Name)
		password.Placeholder = "留空则保留原密码"
		privateKey.Placeholder = "留空则保留原私钥"
	}
	m := credentialFormModel{
		manager: manager, credential: credential, credentialType: credentialType,
		name: name, password: password, privateKey: privateKey,
	}
	// m.password.EchoMode = textinput.EchoPassword
	// m.password.EchoCharacter = '•'
	m.name.Focus()
	return m
}

func newKeyInput() textarea.Model {
	input := textarea.New()
	input.Prompt = ""
	input.Placeholder = "粘贴 OpenSSH / PEM 私钥内容"
	input.ShowLineNumbers = false
	input.EndOfBufferCharacter = ' '
	input.CharLimit = 64 * 1024
	input.SetWidth(40)
	input.SetHeight(5)
	styles := input.Styles()
	styles.Focused.Text = plainStyle
	styles.Focused.Placeholder = mutedStyle
	styles.Focused.CursorLine = lipgloss.NewStyle()
	styles.Blurred.Text = plainStyle
	styles.Blurred.Placeholder = mutedStyle
	styles.Blurred.CursorLine = lipgloss.NewStyle()
	styles.Cursor.Color = lipgloss.Color("#e7e7e7")
	input.SetStyles(styles)
	return input
}

// Update 处理凭据表单的字段联动、导航和保存。
func (m credentialFormModel) Update(msg tea.Msg) (modalModel, tea.Cmd) {
	if result, ok := msg.(credentialSaveFailedMsg); ok {
		m.saving = false
		m.err = result.err.Error()
		return m, nil
	}
	if _, ok := msg.(credentialSavedMsg); ok {
		return m.manager.reload(), nil
	}
	if m.saving {
		return m, nil
	}
	if isClearInputKey(msg) {
		switch m.focus {
		case credentialNameField:
			m.name.Reset()
		case credentialContentField:
			if m.credentialType == "key" {
				m.privateKey.Reset()
			} else {
				m.password.Reset()
			}
		}
		m.err = ""
		return m, nil
	}
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "esc":
			return m.manager, nil
		case "tab":
			return m.moveFocus(1)
		case "shift+tab":
			return m.moveFocus(-1)
		case "ctrl+s":
			return m.save()
		case "left", "right", "h", "l", "space", " ":
			if m.focus == credentialTypeField {
				m.toggleType()
				return m, nil
			}
		case "enter":
			if m.focus != credentialContentField || m.credentialType == "passwd" {
				return m.moveFocus(1)
			}
		}
	}
	return m.updateInput(msg)
}

func (m *credentialFormModel) toggleType() {
	m.password.Blur()
	m.privateKey.Blur()
	if m.credentialType == "passwd" {
		m.credentialType = "key"
	} else {
		m.credentialType = "passwd"
	}
	if m.focus == credentialContentField {
		m.focusInput()
	}
}

func (m credentialFormModel) moveFocus(step int) (modalModel, tea.Cmd) {
	m.name.Blur()
	m.password.Blur()
	m.privateKey.Blur()
	m.focus = (m.focus + step + credentialFieldCount) % credentialFieldCount
	return m, m.focusInput()
}

func (m *credentialFormModel) focusInput() tea.Cmd {
	switch m.focus {
	case credentialNameField:
		return m.name.Focus()
	case credentialContentField:
		if m.credentialType == "key" {
			return m.privateKey.Focus()
		}
		return m.password.Focus()
	}
	return nil
}

func (m credentialFormModel) updateInput(msg tea.Msg) (modalModel, tea.Cmd) {
	m.err = ""
	var cmd tea.Cmd
	switch m.focus {
	case credentialNameField:
		m.name, cmd = m.name.Update(msg)
	case credentialContentField:
		if m.credentialType == "key" {
			m.privateKey, cmd = m.privateKey.Update(msg)
		} else {
			m.password, cmd = m.password.Update(msg)
		}
	}
	return m, cmd
}

func (m credentialFormModel) save() (modalModel, tea.Cmd) {
	content := []byte(m.password.Value())
	if m.credentialType == "key" {
		content = []byte(m.privateKey.Value())
	}
	if strings.TrimSpace(m.name.Value()) == "" {
		clear(content)
		m.err = "凭据名称不能为空"
		return m, nil
	}
	if m.credential == nil && len(bytes.TrimSpace(content)) == 0 {
		clear(content)
		m.err = "新增凭据的密码或私钥不能为空"
		return m, nil
	}
	if m.manager.store == nil {
		clear(content)
		m.err = "数据库未连接"
		return m, nil
	}
	name, credentialType := m.name.Value(), m.credentialType
	m.saving = true
	m.err = ""
	return m, func() tea.Msg {
		defer clear(content)
		var err error
		var saved repository.Credential
		if m.credential == nil {
			saved, err = m.manager.store.CreateCredential(repository.NewCredential{Name: name, Type: credentialType, Content: content})
		} else {
			err = m.manager.store.UpdateCredential(m.credential.ID, name, credentialType, content)
			saved = repository.Credential{ID: m.credential.ID, Name: strings.TrimSpace(name), Type: credentialType}
		}
		if err != nil {
			return credentialSaveFailedMsg{err: err}
		}
		return credentialSavedMsg{credential: saved}
	}
}

func (m credentialFormModel) View() string {
	title := "新增凭据"
	if m.credential != nil {
		title = "编辑凭据"
	}
	lines := []string{
		modalTitleStyle.Render(title),
		borderStyle.Render(strings.Repeat("─", modalContentWidth)),
		m.inputRow("名称", m.name.View(), credentialNameField),
		"",
		m.typeRow(),
		"",
	}
	if m.credentialType == "key" {
		lines = append(lines, m.privateKeyRow())
	} else {
		lines = append(lines, m.inputRow("密码", m.password.View(), credentialContentField))
	}
	status := "←/→ 选择类型 | Ctrl+S 保存 | Esc 返回"
	statusStyle := mutedStyle
	if m.err != "" {
		status = m.err
		statusStyle = formErrorStyle
	} else if m.saving {
		status = "正在加密并保存..."
		statusStyle = accentStyle
	}
	lines = append(lines, borderStyle.Render(strings.Repeat("─", modalContentWidth)), statusStyle.Render(status))
	return modalStyle.Render(strings.Join(lines, "\n"))
}

func (m credentialFormModel) inputRow(label, value string, field int) string {
	labelStyle, inputStyle := formLabelStyle, formInputStyle
	if m.focus == field {
		labelStyle, inputStyle = formActiveLabel, formInputFocused
		label += " >"
	}
	return "  " + labelStyle.Render(label) + inputStyle.Render(value)
}

func (m credentialFormModel) typeRow() string {
	labelStyle := formLabelStyle
	if m.focus == credentialTypeField {
		labelStyle = formActiveLabel
	}
	label := "类型"
	if m.focus == credentialTypeField {
		label += " >"
	}
	passwordStyle, keyStyle := formChoiceStyle, formChoiceStyle
	if m.credentialType == "passwd" {
		passwordStyle = formChoiceSelected
	} else {
		keyStyle = formChoiceSelected
	}
	return "  " + labelStyle.Render(label) + passwordStyle.Render("密码") + keyStyle.Render("私钥")
}

func (m credentialFormModel) privateKeyRow() string {
	labelStyle, inputStyle := formLabelStyle, formInputStyle
	label := "私钥"
	if m.focus == credentialContentField {
		labelStyle, inputStyle = formActiveLabel, formInputFocused
		label += " >"
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, "  "+labelStyle.Render(label), inputStyle.Render(m.privateKey.View()))
}

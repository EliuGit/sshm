package tui

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"sshm/internal/repository"
)

// ErrInitializationCanceled 表示用户在初始化或解锁界面主动退出。
var ErrInitializationCanceled = errors.New("初始化已取消")

// initializeMode 区分首次设置密码和已有数据库解锁。
type initializeMode uint8

const (
	initializeNew initializeMode = iota
	initializeUnlock
)

// initializeModel 管理主界面启动前的数据库密码交互。
type initializeModel struct {
	path   string
	mode   initializeMode
	width  int
	height int

	password textinput.Model
	repeat   textinput.Model
	err      string
	saving   bool
	store    *repository.Store
	result   error
}

// initializeResultMsg 携带初始化或解锁的异步执行结果。
type initializeResultMsg struct {
	store *repository.Store
	err   error
}

// applicationModel 在同一个 Bubble Tea 程序内衔接初始化界面和主界面，避免切换终端时闪屏。
type applicationModel struct {
	initializing initializeModel
	main         *model
	result       error
}

// RunApplication 在一次 TUI 会话中完成记住密码解锁、初始化和主界面运行。
func RunApplication(path string, status repository.Status) error {
	var saved []byte
	var loadErr error
	if status == repository.Ready {
		saved, loadErr = loadPassword()
	}
	app := newApplicationModel(path, status, saved, loadErr)
	clear(saved)
	final, err := tea.NewProgram(app, tea.WithEnvironment(bubbleTeaEnvironment(os.Environ()))).Run()
	if err != nil {
		if app.main != nil && app.main.store != nil {
			_ = app.main.store.Close()
		}
		return err
	}
	result := final.(applicationModel)
	if result.main != nil && result.main.store != nil {
		_ = result.main.store.Close()
	}
	return result.result
}

// bubbleTeaEnvironment 让 SSH 会话也探测同步输出能力，实际启用仍以终端对 DEC 2026 模式的响应为准。
// Bubble Tea 2.0.9 默认因 SSH_TTY 跳过探测；WT_SESSION 仅传给 Bubble Tea，不修改进程环境。
func bubbleTeaEnvironment(env []string) []string {
	var sshSession, windowsTerminal bool
	for _, variable := range env {
		name, _, _ := strings.Cut(variable, "=")
		sshSession = sshSession || name == "SSH_TTY"
		windowsTerminal = windowsTerminal || name == "WT_SESSION"
	}
	if sshSession && !windowsTerminal {
		return append(env, "WT_SESSION=sshm")
	}
	return env
}

func newApplicationModel(path string, status repository.Status, saved []byte, loadErr error) applicationModel {
	rememberedInvalid := status == repository.Ready && loadErr != nil && !errors.Is(loadErr, os.ErrNotExist)
	if status == repository.Ready && loadErr == nil {
		store, err := repository.Unlock(path, saved)
		if err == nil {
			main, modelErr := newModel(store, path)
			if modelErr != nil {
				_ = store.Close()
				return applicationModel{result: fmt.Errorf("读取连接列表: %w", modelErr)}
			}
			return applicationModel{main: &main}
		}
		if !errors.Is(err, repository.ErrInvalidPassword) {
			return applicationModel{initializing: initializeModel{path: path, mode: initializeUnlock, err: err.Error()}, result: err}
		}
		rememberedInvalid = true
	}
	if rememberedInvalid {
		_ = removePassword()
		m := newInitializeModel(path, initializeUnlock)
		m.err = "记住的密码已失效，请重新输入"
		return applicationModel{initializing: m}
	}
	mode := initializeUnlock
	if status == repository.NotFound || status == repository.Uninitialized {
		mode = initializeNew
	}
	return applicationModel{initializing: newInitializeModel(path, mode)}
}

func (m applicationModel) Init() tea.Cmd {
	if m.result != nil {
		return tea.Quit
	}
	if m.main != nil {
		return m.main.Init()
	}
	return m.initializing.Init()
}

func (m applicationModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.main != nil {
		updated, cmd := m.main.Update(msg)
		main := updated.(model)
		m.main = &main
		return m, cmd
	}
	updated, cmd := m.initializing.Update(msg)
	m.initializing = updated.(initializeModel)
	if m.initializing.result != nil {
		m.result = m.initializing.result
		return m, cmd
	}
	if m.initializing.store != nil {
		main, err := newModel(m.initializing.store, m.initializing.path)
		if err != nil {
			_ = m.initializing.store.Close()
			m.result = fmt.Errorf("读取连接列表: %w", err)
			return m, tea.Quit
		}
		main.width, main.height = m.initializing.width, m.initializing.height
		m.main = &main
		return m, nil
	}
	return m, cmd
}

func (m applicationModel) View() tea.View {
	if m.main != nil {
		return m.main.View()
	}
	return m.initializing.View()
}

func newInitializeModel(path string, mode initializeMode) initializeModel {
	m := initializeModel{path: path, mode: mode}
	m.password = newFormInput("应用密码", passwordCharLimit)
	if mode == initializeUnlock {
		m.password.Placeholder = ""
	}
	m.password.EchoMode = textinput.EchoPassword
	m.password.EchoCharacter = '•'
	m.password.Focus()
	if mode == initializeNew {
		m.repeat = newFormInput("再次输入密码", passwordCharLimit)
		m.repeat.EchoMode = textinput.EchoPassword
		m.repeat.EchoCharacter = '•'
	}
	return m
}

func (m initializeModel) Init() tea.Cmd { return nil }

// Update 处理初始化、解锁的输入和异步结果。
func (m initializeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = min(size.Width, maxViewWidth), size.Height
	}
	if result, ok := msg.(initializeResultMsg); ok {
		m.saving = false
		if result.err != nil {
			m.password.Reset()
			m.repeat.Reset()
			m.err = passwordErrorMessage(result.err)
			return m, nil
		}
		m.store = result.store
		return m.finish(nil)
	}
	if m.saving {
		return m, nil
	}
	if isClearInputKey(msg) {
		m.focusedInput().Reset()
		m.err = ""
		return m, nil
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	m.err = ""
	switch key.String() {
	case "esc", "ctrl+c":
		return m.finish(ErrInitializationCanceled)
	case "tab", "down":
		if m.mode == initializeNew {
			m.password.Blur()
			if m.repeat.Focused() {
				m.repeat.Blur()
				m.password.Focus()
			} else {
				m.repeat.Focus()
			}
			return m, nil
		}
	case "shift+tab", "up":
		if m.mode == initializeNew {
			m.repeat.Blur()
			if m.password.Focused() {
				m.password.Blur()
				m.repeat.Focus()
			} else {
				m.password.Focus()
			}
			return m, nil
		}
	case "ctrl+s":
		if m.mode == initializeUnlock {
			return m.submit(true)
		}
		return m.submit(false)
	case "enter":
		return m.submit(false)
	}
	var cmd tea.Cmd
	input := m.focusedInput()
	*input, cmd = input.Update(msg)
	return m, cmd
}

func (m *initializeModel) focusedInput() *textinput.Model {
	if m.mode == initializeNew && m.repeat.Focused() {
		return &m.repeat
	}
	return &m.password
}

func (m initializeModel) submit(remember bool) (tea.Model, tea.Cmd) {
	password := m.password.Value()
	if password == "" {
		m.err = "密码不能为空"
		return m, nil
	}
	if m.mode == initializeNew {
		if m.repeat.Value() == "" {
			m.err = "重复密码不能为空"
			return m, nil
		}
		if password != m.repeat.Value() {
			m.err = "两次输入的密码不一致"
			return m, nil
		}
	}
	bytes := []byte(password)
	m.saving, m.err = true, ""
	return m, func() tea.Msg {
		defer clear(bytes)
		var store *repository.Store
		var err error
		if m.mode == initializeNew {
			store, err = repository.Initialize(m.path, bytes)
		} else {
			store, err = repository.Unlock(m.path, bytes)
			if err == nil && remember {
				if err = savePassword(bytes); err != nil {
					_ = store.Close()
					store = nil
					err = fmt.Errorf("记住密码失败: %w", err)
				}
			}
		}
		return initializeResultMsg{store: store, err: err}
	}
}

func (m initializeModel) finish(err error) (tea.Model, tea.Cmd) {
	m.result = err
	return m, tea.Quit
}

func (m initializeModel) View() tea.View {
	title := "初始化应用"
	if m.mode == initializeUnlock {
		title = "解锁"
	}
	body := m.viewBody(title)
	if m.width > 0 && m.height > 0 {
		base := strings.Repeat(" ", m.width)
		base = strings.Repeat(base+"\n", m.height-1) + base
		body = placeModal(base, body, m.width, m.height)
	}
	view := tea.NewView(body)
	view.AltScreen = true
	view.WindowTitle = windowTitle
	view.BackgroundColor = backgroundColor
	view.ForegroundColor = textColor
	return view
}

func (m initializeModel) viewBody(title string) string {
	status := "Enter/Ctrl+S 确定 | Esc 退出"
	if m.mode == initializeUnlock {
		status = "Enter 解锁 | Ctrl+S 解锁并记住密码 | Esc 退出"
	}
	style := mutedStyle
	if m.err != "" {
		status, style = m.err, formErrorStyle
	}
	passwordStyle := formInputStyle
	if m.password.Focused() {
		passwordStyle = formInputFocused
	}
	repeatStyle := formInputStyle
	if m.repeat.Focused() {
		repeatStyle = formInputFocused
	}
	passwordLabelStyle, passwordLabel := formLabelStyle, "密码"
	if m.password.Focused() {
		passwordLabelStyle, passwordLabel = formActiveLabel, "密码 >"
	}
	repeatLabelStyle, repeatLabel := formLabelStyle, "重复"
	if m.repeat.Focused() {
		repeatLabelStyle, repeatLabel = formActiveLabel, "重复 >"
	}
	passwordRow := ">" + passwordStyle.Render(m.password.View())
	if m.mode == initializeNew {
		passwordRow = "  " + passwordLabelStyle.Render(passwordLabel) + passwordStyle.Render(m.password.View())
	}
	lines := []string{modalTitleStyle.Render(title), borderStyle.Render(strings.Repeat("─", modalContentWidth)), passwordRow}
	if m.mode == initializeNew {
		lines = append(lines, "", "  "+repeatLabelStyle.Render(repeatLabel)+repeatStyle.Render(m.repeat.View()))
	}
	lines = append(lines, borderStyle.Render(strings.Repeat("─", modalContentWidth)), style.Render(status))
	return modalStyle.Render(strings.Join(lines, "\n"))
}

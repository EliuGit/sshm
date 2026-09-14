// Package tui 提供 SSHM 的终端用户界面。
package tui

import (
	"fmt"
	"time"

	"sshm/internal/repository"
	ssh "sshm/internal/ssh"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// model 保存界面状态。终端尺寸由 Bubble Tea 通过 WindowSizeMsg 提供。
type model struct {
	store         *repository.Store
	width         int
	height        int
	selected      int
	searchFocused bool
	searchInput   textinput.Model
	searchReady   bool
	connections   []connectionRow
	sortField     byte
	sortAsc       bool
	modal         modalModel
}

func newModel(store *repository.Store) (model, error) {
	m := model{store: store, searchInput: newSearchInput(), searchReady: true}
	connections, err := store.ListConnections()
	if err != nil {
		return m, err
	}
	m.connections = make([]connectionRow, 0, len(connections))
	for _, connection := range connections {
		m.connections = append(m.connections, rowFromConnection(connection))
	}
	return m, nil
}

// Init 返回程序启动时需要执行的命令；连接数据已在创建模型时读取。
func (model) Init() tea.Cmd { return nil }

// Update 消费终端事件，并返回更新后的状态和后续命令。
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = size.Width, size.Height
	}
	if connected, ok := msg.(sftpReadyMsg); ok {
		if _, active := m.modal.(transferModel); !active {
			if connected.client != nil {
				_ = connected.client.Close()
			}
			return m, nil
		}
	}
	if _, ok := msg.(closeModalMsg); ok {
		if transfer, active := m.modal.(transferModel); active {
			transfer.closeSFTP()
		}
		m.modal = nil
		return m, nil
	}
	if saved, ok := msg.(credentialSavedMsg); ok {
		for i := range m.connections {
			if m.connections[i].credID == saved.credential.ID {
				m.connections[i].credName = saved.credential.Name
				m.connections[i].auth = map[string]string{"passwd": "密码", "key": "私钥"}[saved.credential.Type]
			}
		}
	}
	if prepared, ok := msg.(shellPreparedMsg); ok {
		if prepared.err != nil {
			clear(prepared.credential)
			m.modal = newShellError(prepared.err)
			return m, nil
		}
		shell := ssh.NewShell(prepared.connection, prepared.credential)
		m.modal = nil
		store, id := m.store, prepared.connection.ID
		return m, tea.Exec(shell, func(err error) tea.Msg {
			if err == nil {
				if markErr := store.MarkUsed(id); markErr != nil {
					err = fmt.Errorf("记录连接使用状态: %w", markErr)
				}
			}
			return shellFinishedMsg{connectionID: id, err: err}
		})
	}
	if finished, ok := msg.(shellFinishedMsg); ok {
		m.modal = nil
		if finished.err != nil {
			m.modal = newShellError(finished.err)
			return m, nil
		}
		for i := range m.connections {
			if m.connections[i].id == finished.connectionID {
				m.connections[i].useCount++
				m.connections[i].lastUsed = time.Now().Format("2006-01-02 15:04")
				break
			}
		}
		return m, nil
	}
	if created, ok := msg.(connectionCreatedMsg); ok {
		m.connections = append(m.connections, rowFromConnection(created.connection))
		m.modal = nil
		return m, nil
	}
	if updated, ok := msg.(connectionUpdatedMsg); ok {
		for i := range m.connections {
			if m.connections[i].id == updated.connection.ID {
				m.connections[i] = rowFromConnection(updated.connection)
				break
			}
		}
		m.modal = nil
		return m, nil
	}
	if deleted, ok := msg.(connectionDeletedMsg); ok {
		for i := range m.connections {
			if m.connections[i].id == deleted.id {
				m.connections = append(m.connections[:i], m.connections[i+1:]...)
				if m.selected >= len(m.visibleConnections()) {
					m.selected = max(0, len(m.visibleConnections())-1)
				}
				break
			}
		}
		m.modal = nil
		return m, nil
	}
	if m.modal != nil {
		var cmd tea.Cmd
		m.modal, cmd = m.modal.Update(msg)
		if transfer, ok := m.modal.(transferModel); ok {
			transfer.width, transfer.height = m.width, m.height
			m.modal = transfer
		}
		return m, cmd
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
	case tea.KeyPressMsg:
		if !m.searchReady {
			m.searchInput = newSearchInput()
			m.searchReady = true
		}
		if m.searchFocused {
			if isClearInputKey(msg) {
				m.searchInput.Reset()
				m.selected = 0
				return m, nil
			}
			switch msg.String() {
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
			var cmd tea.Cmd
			before := m.searchInput.Value()
			m.searchInput, cmd = m.searchInput.Update(msg)
			if m.searchInput.Value() != before {
				m.selected = 0
			}
			return m, cmd
		}
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "a":
			form := newConnectionForm(m.store)
			m.modal = form
			return m, nil
		case "p":
			m.modal = newCredentialManager(m.store)
			return m, nil
		case "enter", "l":
			connections := m.visibleConnections()
			if m.selected >= 0 && m.selected < len(connections) {
				m.modal = newMenu(m.store, connections[m.selected])
			}
			return m, nil
		case "/":
			m.searchFocused = true
			return m, m.searchInput.Focus()
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected < len(m.visibleConnections())-1 {
				m.selected++
			}
		case "n", "h", "u":
			if m.sortField == msg.String()[0] {
				m.sortAsc = !m.sortAsc
			} else {
				m.sortField, m.sortAsc = msg.String()[0], true
			}
			if m.selected >= len(m.visibleConnections()) {
				m.selected = max(0, len(m.visibleConnections())-1)
			}
		}
	}
	return m, nil
}

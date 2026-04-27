package ui

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sshm/internal/app"
	"sshm/internal/domain"
	"sshm/internal/i18n"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	services              *app.Services
	translator            *i18n.Translator
	theme                 Theme
	startupDir            string
	defaultPrivateKeyPath string
	result                AppResult

	shellState
	screenState
	overlayState
}

func NewModel(services *app.Services, translator *i18n.Translator, startupDir string, defaultPrivateKeyPath string) *Model {
	if translator == nil {
		translator, _ = i18n.New("en")
	}
	theme := newDefaultTheme()
	searchInput := textinput.New()
	searchInput.Placeholder = translator.T("home.search_placeholder")
	searchInput.Prompt = translator.T("home.search_prompt")
	searchInput.Width = 40
	searchInput.PromptStyle = theme.Styles.SubtleText
	searchInput.PlaceholderStyle = theme.Styles.SubtleText
	searchInput.Blur()

	return &Model{
		services:              services,
		translator:            translator,
		theme:                 theme,
		startupDir:            startupDir,
		defaultPrivateKeyPath: defaultPrivateKeyPath,
		shellState: shellState{
			page:   pageHome,
			status: translator.T("status.ready"),
		},
		screenState: screenState{
			home: homeScreenState{
				searchInput: searchInput,
			},
			form:    newFormState(nil, translator, defaultPrivateKeyPath, theme),
			browser: newBrowserState(translator, theme),
			imports: newImportState(translator, theme),
		},
		overlayState: overlayState{
			groups: newGroupPanelState(translator, theme),
		},
	}
}

func (m *Model) Result() AppResult {
	return m.result
}

func (m *Model) setInfoStatus(status string) {
	m.err = nil
	m.status = status
	m.statusSuccess = false
}

func (m *Model) setSuccessStatus(status string) {
	m.err = nil
	m.status = status
	m.statusSuccess = true
}

func (m *Model) setErrorStatus(err error) {
	m.err = err
	m.status = m.translator.Error(err)
	m.statusSuccess = false
}

func (m *Model) clearStaleErrorStatus() {
	if m.err == nil {
		return
	}
	m.err = nil
	m.statusSuccess = false
	switch m.page {
	case pageBrowser:
		if m.browser.localPanel.loading || m.browser.remotePanel.loading {
			m.status = m.translator.T("status.loading_browser")
			return
		}
		m.status = m.translator.T("status.browser_ready")
	case pageHome:
		m.status = m.homeReadyStatus()
	default:
		m.status = m.translator.T("status.ready")
	}
}

func (m *Model) homeReadyStatus() string {
	if len(m.home.connections) == 0 {
		if strings.TrimSpace(m.home.search) != "" {
			return m.translator.T("status.no_matches")
		}
		return m.translator.T("status.no_connections")
	}
	if strings.TrimSpace(m.home.search) != "" {
		return m.translator.T("status.found_connections", len(m.home.connections))
	}
	return m.translator.T("status.connections_ready", len(m.home.connections))
}

func (m *Model) applyConnectionsLoaded(items []Connection) {
	m.home.connections = items
	m.home.selected = clamp(m.home.selected, len(m.home.connections))
	if len(m.home.connections) == 0 {
		m.home.selected = 0
		m.setInfoStatus(m.homeReadyStatus())
		return
	}
	m.setInfoStatus(m.homeReadyStatus())
}

func (m *Model) applyLoadedBrowserPanel(panel *filePanel, items []domain.FileEntry, path string, selectName string) {
	panel.items = items
	panel.path = path
	panel.loading = false
	if selectName != "" {
		panel.selectByName(selectName)
	} else {
		panel.cursor = clamp(panel.cursor, len(panel.rows()))
	}
	m.err = nil
	m.syncBrowserStatus()
}

func (m *Model) Init() tea.Cmd {
	return m.loadConnectionsCmd()
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case connectionsLoadedMsg:
		if msg.err != nil {
			m.setErrorStatus(msg.err)
			return m, nil
		}
		m.applyConnectionsLoaded(msg.items)
	case groupsLoadedMsg:
		if msg.err != nil {
			m.setErrorStatus(msg.err)
			return m, nil
		}
		m.groups.items = msg.items
		m.groups.selected = clamp(m.groups.selected, len(m.groups.items))
	case groupOpDoneMsg:
		if msg.err != nil {
			m.setErrorStatus(msg.err)
			return m, nil
		}
		m.err = nil
		if msg.groupName != "" && m.home.listScope == domain.ConnectionListScopeGroup && m.home.listGroupID == msg.groupID {
			m.home.listGroup = msg.groupName
		}
		if msg.clearGroupFilter {
			m.home.listScope = domain.ConnectionListScopeAll
			m.home.listGroupID = 0
			m.home.listGroup = ""
		}
		if msg.success {
			m.setSuccessStatus(msg.status)
		} else {
			m.setInfoStatus(msg.status)
		}
		var cmds []tea.Cmd
		if msg.reloadGroups {
			cmds = append(cmds, m.loadGroupsCmd())
		}
		if msg.reloadConnections {
			cmds = append(cmds, m.loadConnectionsCmd())
		}
		return m, tea.Batch(cmds...)
	case importPreviewMsg:
		if msg.err != nil {
			m.imports.errorText = m.translator.Error(msg.err)
			m.imports.loading = false
			return m, nil
		}
		m.imports.items = msg.preview.Candidates
		m.imports.warnings = msg.preview.Warnings
		m.imports.selected = 0
		m.imports.step = importStepPreview
		m.imports.loading = false
		m.imports.errorText = ""
	case importDoneMsg:
		if msg.err != nil {
			m.imports.errorText = m.translator.Error(msg.err)
			return m, nil
		}
		m.page = pageHome
		if msg.setScope {
			m.home.listScope = msg.scope
			m.home.listGroupID = msg.groupID
			m.home.listGroup = msg.groupName
			m.home.selected = 0
		}
		m.setSuccessStatus(m.translator.T("status.import_done", msg.summary.Created, msg.summary.Updated, msg.summary.Skipped))
		if msg.reloadConnections {
			return m, m.loadConnectionsCmd()
		}
		return m, nil
	case localLoadedMsg:
		if msg.request != 0 && msg.request != m.browser.localPanel.request {
			return m, nil
		}
		if msg.err != nil {
			m.setErrorStatus(msg.err)
			m.browser.localPanel.loading = false
			return m, nil
		}
		m.applyLoadedBrowserPanel(&m.browser.localPanel, msg.items, msg.path, msg.selectName)
	case remoteLoadedMsg:
		if msg.request != 0 && msg.request != m.browser.remotePanel.request {
			return m, nil
		}
		if msg.err != nil {
			if m.page == pageBrowser && isBrowserSessionError(msg.err) {
				return m.handleBrowserSessionFailure(msg.err)
			}
			m.setErrorStatus(msg.err)
			m.browser.remotePanel.loading = false
			return m, nil
		}
		m.applyLoadedBrowserPanel(&m.browser.remotePanel, msg.items, msg.path, msg.selectName)
	case homeProbeDoneMsg:
		m.home.connecting = false
		if msg.err != nil {
			m.setErrorStatus(fmt.Errorf(m.homeProbeFailedStatus(msg.action, msg.connectionName, msg.err)))
			return m, nil
		}
		return m.completeHomeProbe(msg)
	case shellReadyMsg:
		m.result.ShellSession = msg.session
		return m, tea.Quit
	case opDoneMsg:
		if msg.err != nil {
			if m.page == pageBrowser && isBrowserSessionError(msg.err) {
				return m.handleBrowserSessionFailure(msg.err)
			}
			m.setErrorStatus(msg.err)
			return m, nil
		}
		if msg.success {
			m.setSuccessStatus(msg.status)
		} else {
			m.setInfoStatus(msg.status)
		}
		if msg.reloadBrowser {
			return m, m.reloadBrowserSelectCmd(msg.targetPanel, msg.selectName)
		}
		if msg.reloadConnections {
			return m, m.loadConnectionsCmd()
		}
		return m, nil
	case transferProgressMsg:
		m.err = nil
		m.status = renderTransferProgress(msg.action, msg.progress)
		m.statusSuccess = false
		return m, listenTransferProgress(msg.source)
	}

	return m.currentScreen().update(m, msg)
}

func (m *Model) homeProbeFailedStatus(action homeProbeAction, connectionName string, err error) string {
	switch action {
	case homeProbeBrowser:
		return m.translator.T("status.browser_connect_failed", connectionName, m.translator.Error(err))
	default:
		return m.translator.T("status.shell_connect_failed", connectionName, m.translator.Error(err))
	}
}

func (m *Model) completeHomeProbe(msg homeProbeDoneMsg) (tea.Model, tea.Cmd) {
	switch msg.action {
	case homeProbeBrowser:
		return m.openBrowserForConnection(msg.connection, msg.session)
	default:
		return m, func() tea.Msg {
			return shellReadyMsg{session: msg.session}
		}
	}
}

func (m *Model) openBrowserForConnection(conn Connection, session app.RemoteSession) (tea.Model, tea.Cmd) {
	m.page = pageBrowser
	m.browser = newBrowserState(m.translator, m.theme)
	m.browser.connectionID = conn.ID
	m.browser.connection = conn
	m.browser.session = session
	m.browser.localPanel.path = m.startupDir
	m.browser.remotePanel.path = "."
	m.browser.activePanel = domain.LocalPanel
	m.browser.localPanel.loading = true
	m.browser.remotePanel.loading = true
	return m, tea.Batch(
		tea.ClearScreen,
		m.loadLocalCmd(m.browser.localPanel.path, m.browser.localPanel.filter),
		m.loadRemoteCmd(m.browser.remotePanel.path, m.browser.remotePanel.filter),
	)
}

func (m *Model) handleBrowserSessionFailure(err error) (tea.Model, tea.Cmd) {
	m.closeBrowserSession()
	m.page = pageHome
	m.overlay = overlayNone
	m.browser.localPanel.loading = false
	m.browser.remotePanel.loading = false
	m.setErrorStatus(err)
	return m, tea.Batch(tea.ClearScreen, m.loadConnectionsCmd())
}

func isBrowserSessionError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || errors.Is(err, app.ErrRemoteSessionClosed) {
		return true
	}
	message := strings.ToLower(err.Error())
	for _, fragment := range []string{
		"broken pipe",
		"connection reset by peer",
		"use of closed network connection",
		"connection closed",
		"failed to create session",
		"remote session is closed",
	} {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}

func (m *Model) View() string {
	return m.currentScreen().view(m)
}

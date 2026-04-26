package ui

import (
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

	width  int
	height int

	page    page
	overlay overlayKind

	searchInput textinput.Model
	search      string
	searchMode  bool
	listScope   domain.ConnectionListScope
	listGroupID int64
	listGroup   string

	connections   []Connection
	selected      int
	status        string
	statusSuccess bool
	err           error

	form formState

	browser browserState
	groups  groupPanelState
	imports importState

	deleteTarget int64
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
		page:                  pageHome,
		searchInput:           searchInput,
		status:                translator.T("status.ready"),
		form:                  newFormState(nil, translator, defaultPrivateKeyPath, theme),
		browser:               newBrowserState(translator, theme),
		groups:                newGroupPanelState(translator, theme),
		imports:               newImportState(translator, theme),
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

func (m *Model) applyConnectionsLoaded(items []Connection) {
	m.connections = items
	m.selected = clamp(m.selected, len(m.connections))
	if len(m.connections) == 0 {
		m.selected = 0
		if strings.TrimSpace(m.search) != "" {
			m.setInfoStatus(m.translator.T("status.no_matches"))
			return
		}
		m.setInfoStatus(m.translator.T("status.no_connections"))
		return
	}
	if strings.TrimSpace(m.search) != "" {
		m.setInfoStatus(m.translator.T("status.found_connections", len(m.connections)))
		return
	}
	m.setInfoStatus(m.translator.T("status.connections_ready", len(m.connections)))
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
		m.err = nil
	case groupOpDoneMsg:
		if msg.err != nil {
			m.setErrorStatus(msg.err)
			return m, nil
		}
		m.err = nil
		if msg.groupName != "" && m.listScope == domain.ConnectionListScopeGroup && m.listGroupID == msg.groupID {
			m.listGroup = msg.groupName
		}
		if msg.clearGroupFilter {
			m.listScope = domain.ConnectionListScopeAll
			m.listGroupID = 0
			m.listGroup = ""
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
			m.listScope = msg.scope
			m.listGroupID = msg.groupID
			m.listGroup = msg.groupName
			m.selected = 0
		}
		m.setSuccessStatus(m.translator.T("status.import_done", msg.summary.Created, msg.summary.Updated, msg.summary.Skipped))
		if msg.reloadConnections {
			return m, m.loadConnectionsCmd()
		}
		return m, nil
	case localLoadedMsg:
		if msg.err != nil {
			m.setErrorStatus(msg.err)
			m.browser.localPanel.loading = false
			return m, nil
		}
		m.applyLoadedBrowserPanel(&m.browser.localPanel, msg.items, msg.path, msg.selectName)
	case remoteLoadedMsg:
		if msg.err != nil {
			m.setErrorStatus(msg.err)
			m.browser.remotePanel.loading = false
			return m, nil
		}
		m.applyLoadedBrowserPanel(&m.browser.remotePanel, msg.items, msg.path, msg.selectName)
	case shellReadyMsg:
		m.result.ShellConnectionID = msg.connectionID
		return m, tea.Quit
	case opDoneMsg:
		if msg.err != nil {
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

	switch m.page {
	case pageHome:
		return m.updateHome(msg)
	case pageForm:
		return m.updateForm(msg)
	case pageBrowser:
		return m.updateBrowser(msg)
	case pageImport:
		return m.updateImport(msg)
	default:
		return m, nil
	}
}

func (m *Model) View() string {
	styles := m.theme.Styles
	switch m.page {
	case pageForm:
		return styles.App.Render(m.viewForm())
	case pageBrowser:
		return m.viewBrowser()
	case pageImport:
		return styles.App.Render(m.viewImport())
	default:
		return styles.App.Render(m.viewHome())
	}
}

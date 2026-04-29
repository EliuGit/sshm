package ui

import (
	"path/filepath"
	"sshm/internal/domain"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	commandActionHomeShell       = "home.shell"
	commandActionHomeBrowser     = "home.browser"
	commandActionHomeCreate      = "home.create"
	commandActionHomeEdit        = "home.edit"
	commandActionHomeDelete      = "home.delete"
	commandActionHomeGroups      = "home.groups"
	commandActionHomeMoveGroup   = "home.move_group"
	commandActionHomeImport      = "home.import"
	commandActionHomeSearch      = "home.search"
	commandActionHomeClear       = "home.clear"
	commandActionBrowserOpen     = "browser.open"
	commandActionBrowserUp       = "browser.up"
	commandActionBrowserSwitch   = "browser.switch"
	commandActionBrowserFilter   = "browser.filter"
	commandActionBrowserGoto     = "browser.goto"
	commandActionBrowserMkdir    = "browser.mkdir"
	commandActionBrowserRename   = "browser.rename"
	commandActionBrowserDelete   = "browser.delete"
	commandActionBrowserUpload   = "browser.upload"
	commandActionBrowserDownload = "browser.download"
	commandActionBrowserRefresh  = "browser.refresh"
	commandActionBrowserTop      = "browser.top"
	commandActionBrowserClear    = "browser.clear"
	commandActionBrowserBack     = "browser.back"
)

type commandAction struct {
	id       string
	title    string
	shortcut string
	aliases  []string
}

func (a commandAction) searchableText() string {
	parts := []string{a.title, a.shortcut}
	parts = append(parts, a.aliases...)
	return strings.ToLower(strings.Join(parts, " "))
}

func (m *Model) commandPaletteSupported() bool {
	return m.page == pageHome || m.page == pageBrowser
}

func (m *Model) currentCommandActions() []commandAction {
	switch m.page {
	case pageBrowser:
		return m.browserCommandActions()
	default:
		return m.homeCommandActions()
	}
}

func (m *Model) homeCommandActions() []commandAction {
	actions := []commandAction{
		{id: commandActionHomeSearch, title: m.translator.T("palette.home.search"), shortcut: "/"},
		{id: commandActionHomeCreate, title: m.translator.T("palette.home.create"), shortcut: "c-n"},
		{id: commandActionHomeGroups, title: m.translator.T("palette.home.groups"), shortcut: "g"},
		{id: commandActionHomeImport, title: m.translator.T("palette.home.import"), shortcut: "i"},
	}
	if strings.TrimSpace(m.home.search) != "" || m.home.listScope != domain.ConnectionListScopeAll {
		actions = append(actions, commandAction{id: commandActionHomeClear, title: m.translator.T("palette.home.clear"), shortcut: "esc"})
	}
	if conn := m.currentConnection(); conn != nil {
		actions = append(actions,
			commandAction{id: commandActionHomeShell, title: m.translator.T("palette.home.shell"), shortcut: "enter", aliases: []string{conn.Name}},
			commandAction{id: commandActionHomeBrowser, title: m.translator.T("palette.home.browser"), shortcut: "c-o", aliases: []string{conn.Name}},
			commandAction{id: commandActionHomeEdit, title: m.translator.T("palette.home.edit"), shortcut: "c-e", aliases: []string{conn.Name}},
			commandAction{id: commandActionHomeDelete, title: m.translator.T("palette.home.delete"), shortcut: "c-d", aliases: []string{conn.Name}},
			commandAction{id: commandActionHomeMoveGroup, title: m.translator.T("palette.home.move_group"), shortcut: "c-g", aliases: []string{conn.Name}},
		)
	}
	return actions
}

func (m *Model) browserCommandActions() []commandAction {
	actions := []commandAction{
		{id: commandActionBrowserOpen, title: m.translator.T("palette.browser.open"), shortcut: "enter/l"},
		{id: commandActionBrowserUp, title: m.translator.T("palette.browser.up"), shortcut: "h/backspace"},
		{id: commandActionBrowserSwitch, title: m.translator.T("palette.browser.switch"), shortcut: "tab"},
		{id: commandActionBrowserFilter, title: m.translator.T("palette.browser.filter"), shortcut: "/"},
		{id: commandActionBrowserGoto, title: m.translator.T("palette.browser.goto"), shortcut: ":"},
		{id: commandActionBrowserMkdir, title: m.translator.T("palette.browser.mkdir"), shortcut: "c-n"},
		{id: commandActionBrowserRefresh, title: m.translator.T("palette.browser.refresh"), shortcut: "r"},
		{id: commandActionBrowserTop, title: m.translator.T("palette.browser.top"), shortcut: "g"},
		{id: commandActionBrowserBack, title: m.translator.T("palette.browser.back"), shortcut: "q"},
	}
	if strings.TrimSpace(m.activeBrowserPanel().filter) != "" {
		actions = append(actions, commandAction{id: commandActionBrowserClear, title: m.translator.T("palette.browser.clear"), shortcut: "esc"})
	}
	if _, ok := m.activeBrowserEditableEntry(); ok {
		actions = append(actions,
			commandAction{id: commandActionBrowserRename, title: m.translator.T("palette.browser.rename"), shortcut: "c-r"},
			commandAction{id: commandActionBrowserDelete, title: m.translator.T("palette.browser.delete"), shortcut: "c-d"},
		)
	}
	if m.browser.activePanel == domain.LocalPanel {
		actions = append(actions, commandAction{id: commandActionBrowserUpload, title: m.translator.T("palette.browser.upload"), shortcut: "c-u"})
	}
	if m.browser.activePanel == domain.RemotePanel {
		actions = append(actions, commandAction{id: commandActionBrowserDownload, title: m.translator.T("palette.browser.download"), shortcut: "c-s"})
	}
	return actions
}

func (m *Model) executeCommandAction(actionID string) (tea.Model, tea.Cmd) {
	switch actionID {
	case commandActionHomeShell:
		return m.openCurrentConnectionShell()
	case commandActionHomeBrowser:
		return m.openCurrentConnectionBrowser()
	case commandActionHomeCreate:
		return m.openHomeCreateForm()
	case commandActionHomeEdit:
		return m.openCurrentConnectionEdit()
	case commandActionHomeDelete:
		return m.confirmCurrentConnectionDelete()
	case commandActionHomeGroups:
		return m.openHomeGroupFilter()
	case commandActionHomeMoveGroup:
		return m.openHomeMoveGroup()
	case commandActionHomeImport:
		return m.openImportPage()
	case commandActionHomeSearch:
		return m.openHomeSearch()
	case commandActionHomeClear:
		return m.clearHomeFilters()
	case commandActionBrowserOpen:
		return m.openActiveBrowserSelection()
	case commandActionBrowserUp:
		return m.goParentBrowserDirectory()
	case commandActionBrowserSwitch:
		return m.switchBrowserPanel()
	case commandActionBrowserFilter:
		return m.openBrowserFilterInput()
	case commandActionBrowserGoto:
		return m.openBrowserGotoInput()
	case commandActionBrowserMkdir:
		return m.openBrowserMkdirInput()
	case commandActionBrowserRename:
		return m.openBrowserRenameInput()
	case commandActionBrowserDelete:
		return m.confirmBrowserDelete()
	case commandActionBrowserUpload:
		return m.startBrowserUpload()
	case commandActionBrowserDownload:
		return m.startBrowserDownload()
	case commandActionBrowserRefresh:
		return m.refreshBrowserPanels()
	case commandActionBrowserTop:
		return m.moveBrowserTop()
	case commandActionBrowserClear:
		return m.clearBrowserFilter()
	case commandActionBrowserBack:
		return m.leaveBrowser()
	default:
		return m, nil
	}
}

func (m *Model) openCurrentConnectionShell() (tea.Model, tea.Cmd) {
	conn := m.currentConnection()
	if conn == nil {
		return m, nil
	}
	return m.startHomeProbe(*conn, homeProbeShell)
}

func (m *Model) openCurrentConnectionBrowser() (tea.Model, tea.Cmd) {
	conn := m.currentConnection()
	if conn == nil {
		return m, nil
	}
	return m.startHomeProbe(*conn, homeProbeBrowser)
}

func (m *Model) openHomeCreateForm() (tea.Model, tea.Cmd) {
	m.clearStaleErrorStatus()
	m.page = pageForm
	m.form = newFormState(nil, m.translator, m.defaultPrivateKeyPath, m.styles)
	return m, tea.ClearScreen
}

func (m *Model) openCurrentConnectionEdit() (tea.Model, tea.Cmd) {
	conn := m.currentConnection()
	if conn == nil {
		return m, nil
	}
	m.clearStaleErrorStatus()
	m.page = pageForm
	m.form = newFormState(conn, m.translator, m.defaultPrivateKeyPath, m.styles)
	return m, tea.ClearScreen
}

func (m *Model) confirmCurrentConnectionDelete() (tea.Model, tea.Cmd) {
	conn := m.currentConnection()
	if conn == nil {
		return m, nil
	}
	m.clearStaleErrorStatus()
	m.openDeleteConnectionConfirm(conn.ID)
	return m, nil
}

func (m *Model) openHomeGroupFilter() (tea.Model, tea.Cmd) {
	m.clearStaleErrorStatus()
	m.groups = newGroupPanelState(m.translator, m.styles)
	m.groups.mode = groupPanelFilter
	m.overlay = overlayGroup
	return m, m.loadGroupsCmd()
}

func (m *Model) openHomeMoveGroup() (tea.Model, tea.Cmd) {
	conn := m.currentConnection()
	if conn == nil {
		return m, nil
	}
	m.clearStaleErrorStatus()
	m.groups = newGroupPanelState(m.translator, m.styles)
	m.groups.mode = groupPanelMove
	m.groups.targetID = conn.ID
	m.overlay = overlayGroup
	return m, m.loadGroupsCmd()
}

func (m *Model) openImportPage() (tea.Model, tea.Cmd) {
	m.clearStaleErrorStatus()
	m.page = pageImport
	m.imports = newImportState(m.translator, m.styles)
	return m, tea.ClearScreen
}

func (m *Model) openHomeSearch() (tea.Model, tea.Cmd) {
	m.home.searchMode = true
	m.home.searchInput.Focus()
	m.setInfoStatus(m.translator.T("status.type_to_filter"))
	return m, nil
}

func (m *Model) clearHomeFilters() (tea.Model, tea.Cmd) {
	if strings.TrimSpace(m.home.search) != "" {
		m.home.search = ""
		m.home.searchInput.SetValue("")
		m.home.selected = 0
		m.setInfoStatus(m.translator.T("status.search_cleared"))
		return m, m.loadConnectionsCmd()
	}
	if m.home.listScope != domain.ConnectionListScopeAll {
		m.home.listScope = domain.ConnectionListScopeAll
		m.home.listGroupID = 0
		m.home.listGroup = ""
		m.home.selected = 0
		m.setInfoStatus(m.translator.T("status.group_filter_cleared"))
		return m, m.loadConnectionsCmd()
	}
	return m, nil
}

func (m *Model) leaveBrowser() (tea.Model, tea.Cmd) {
	m.closeBrowserSession()
	m.page = pageHome
	m.overlay = overlayNone
	m.setInfoStatus(m.translator.T("status.returned_connections"))
	return m, tea.Batch(tea.ClearScreen, m.loadConnectionsCmd())
}

func (m *Model) switchBrowserPanel() (tea.Model, tea.Cmd) {
	m.clearStaleErrorStatus()
	if m.browser.activePanel == domain.LocalPanel {
		m.browser.activePanel = domain.RemotePanel
	} else {
		m.browser.activePanel = domain.LocalPanel
	}
	return m, nil
}

func (m *Model) moveBrowserTop() (tea.Model, tea.Cmd) {
	m.clearStaleErrorStatus()
	m.activeBrowserPanel().cursor = 0
	return m, nil
}

func (m *Model) openBrowserFilterInput() (tea.Model, tea.Cmd) {
	m.clearStaleErrorStatus()
	m.overlay = overlayBrowserInput
	m.browser.inputMode = browserInputFilter
	m.browser.inputItem = domain.FileEntry{}
	m.browser.input.SetValue(m.activeBrowserPanel().filter)
	m.browser.input.Focus()
	return m, nil
}

func (m *Model) openBrowserGotoInput() (tea.Model, tea.Cmd) {
	m.clearStaleErrorStatus()
	m.overlay = overlayBrowserInput
	m.browser.inputMode = browserInputGoto
	m.browser.inputItem = domain.FileEntry{}
	m.browser.input.SetValue(m.activeBrowserPanel().path)
	m.browser.input.Focus()
	return m, nil
}

func (m *Model) refreshBrowserPanels() (tea.Model, tea.Cmd) {
	m.clearStaleErrorStatus()
	return m, m.browserWorkflow().reloadCmd()
}

func (m *Model) openActiveBrowserSelection() (tea.Model, tea.Cmd) {
	row, ok := m.activeBrowserPanel().selected()
	if !ok {
		return m, nil
	}
	if row.Name != ".." && !row.IsDir {
		return m, nil
	}
	m.clearStaleErrorStatus()
	return m, m.navigateBrowserPath(m.browser.activePanel, row.Path)
}

func (m *Model) goParentBrowserDirectory() (tea.Model, tea.Cmd) {
	panel := m.activeBrowserPanel()
	next := parentPath(panel.path, m.browser.activePanel == domain.RemotePanel)
	if panel.path == next {
		return m, nil
	}
	m.clearStaleErrorStatus()
	return m, m.navigateBrowserPath(m.browser.activePanel, next)
}

func (m *Model) startBrowserUpload() (tea.Model, tea.Cmd) {
	if m.browser.activePanel != domain.LocalPanel {
		m.setInfoStatus(m.translator.T("status.focus_local_upload"))
		return m, nil
	}
	row, ok := m.browser.localPanel.selected()
	if !ok || row.Name == ".." {
		return m, nil
	}
	m.clearStaleErrorStatus()
	targetPath := joinRemotePath(m.browser.remotePanel.path, row.Name)
	return m.browserWorkflow().prepareOperation(m.translator.T("status.uploading"), row.Path, targetPath, domain.RemotePanel, func(progress func(domain.TransferProgress)) error {
		if m.browser.session == nil {
			return errBrowserSessionNotReady()
		}
		return m.browser.session.Upload(row.Path, m.browser.remotePanel.path, progress)
	}, func() error {
		if m.browser.session == nil {
			return errBrowserSessionNotReady()
		}
		return m.browser.session.Remove(targetPath)
	}, m.translator.T("status.uploaded", row.Name), row.Name, m.translator.T("status.transfer_cancelled"))
}

func (m *Model) startBrowserDownload() (tea.Model, tea.Cmd) {
	if m.browser.activePanel != domain.RemotePanel {
		m.setInfoStatus(m.translator.T("status.focus_remote_download"))
		return m, nil
	}
	row, ok := m.browser.remotePanel.selected()
	if !ok || row.Name == ".." {
		return m, nil
	}
	m.clearStaleErrorStatus()
	targetPath := filepath.Join(m.browser.localPanel.path, row.Name)
	return m.browserWorkflow().prepareOperation(m.translator.T("status.downloading"), row.Path, targetPath, domain.LocalPanel, func(progress func(domain.TransferProgress)) error {
		if m.browser.session == nil {
			return errBrowserSessionNotReady()
		}
		return m.browser.session.Download(row.Path, m.browser.localPanel.path, progress)
	}, func() error {
		return m.services.Files.RemoveLocal(targetPath)
	}, m.translator.T("status.downloaded", row.Name), row.Name, m.translator.T("status.transfer_cancelled"))
}

func (m *Model) clearBrowserFilter() (tea.Model, tea.Cmd) {
	return m, m.clearActiveBrowserFilter()
}

func (m *Model) navigateBrowserPath(targetPanel domain.FilePanel, path string) tea.Cmd {
	return m.browserWorkflow().navigatePathCmd(targetPanel, path)
}

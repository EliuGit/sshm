package ui

import (
	"sshm/internal/app"
	"sshm/internal/domain"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// browserWorkflow 负责文件工作区的业务编排。
//
// 这里集中放置“用户意图”背后的复杂流程：
// - 异步命令构造
// - 传输前存在性检查
// - 传输完成后的刷新策略
// - 会话失效后的统一回收
//
// 这样做的目的不是再套一层框架，而是明确边界：
// 页面 update 负责分发输入，具体业务流统一收口到这里，
// 避免后续继续把浏览器逻辑堆回 Model.Update。
type browserWorkflow struct {
	model *Model
}

func (m *Model) browserWorkflow() browserWorkflow {
	return browserWorkflow{model: m}
}

func (w browserWorkflow) openConnection(conn Connection, session app.FileSession) (tea.Model, tea.Cmd) {
	m := w.model
	m.page = pageBrowser
	m.browser = newBrowserState(m.translator, m.styles)
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
		w.loadLocalCmd(m.browser.localPanel.path),
		w.loadRemoteCmd(m.browser.remotePanel.path),
	)
}

func (w browserWorkflow) handleSessionFailure(err error) (tea.Model, tea.Cmd) {
	m := w.model
	m.closeBrowserSession()
	m.page = pageHome
	m.overlay = overlayNone
	m.browser.localPanel.loading = false
	m.browser.remotePanel.loading = false
	m.setErrorStatus(err)
	return m, tea.Batch(tea.ClearScreen, m.loadConnectionsCmd())
}

func (w browserWorkflow) prepareOperation(action string, sourcePath string, targetPath string, panel domain.FilePanel, run func(func(domain.TransferProgress)) error, replaceTarget func() error, success string, selectName string, cancelStatus string) (tea.Model, tea.Cmd) {
	m := w.model
	pending := &browserPendingOperation{
		action:   action,
		run:      run,
		success:  success,
		cancel:   cancelStatus,
		panel:    panel,
		selectBy: selectName,
	}
	if strings.TrimSpace(targetPath) == "" {
		return w.runOperation(pending)
	}
	exists, err := w.pathExists(panel, targetPath)
	if err != nil {
		if isBrowserSessionError(err) {
			return w.handleSessionFailure(err)
		}
		m.setErrorStatus(err)
		return m, nil
	}
	if exists {
		if replaceTarget != nil {
			pending.run = func(progress func(domain.TransferProgress)) error {
				if err := replaceTarget(); err != nil {
					return err
				}
				return run(progress)
			}
		}
		m.openBrowserOverwriteConfirm(sourcePath, targetPath, pending)
		return m, nil
	}
	return w.runOperation(pending)
}

func (w browserWorkflow) runOperation(pending *browserPendingOperation) (tea.Model, tea.Cmd) {
	return w.model, w.runOperationCmd(pending)
}

func (w browserWorkflow) runOperationCmd(pending *browserPendingOperation) tea.Cmd {
	progressCh := make(chan transferProgressMsg, 16)
	return tea.Batch(
		func() tea.Msg {
			err := pending.run(func(progress domain.TransferProgress) {
				select {
				case progressCh <- transferProgressMsg{progress: progress, action: pending.action, source: progressCh}:
				default:
				}
			})
			close(progressCh)
			if err != nil {
				return opDoneMsg{err: err}
			}
			return opDoneMsg{status: pending.success, success: true, reloadBrowser: true, targetPanel: pending.panel, selectName: pending.selectBy}
		},
		listenTransferProgress(progressCh),
	)
}

func (w browserWorkflow) pathExists(panel domain.FilePanel, targetPath string) (bool, error) {
	m := w.model
	if panel == domain.RemotePanel {
		if m.browser.session == nil {
			return false, errBrowserSessionNotReady()
		}
		return m.browser.session.PathExists(targetPath)
	}
	return m.services.Files.ExistsLocal(targetPath)
}

func (w browserWorkflow) loadLocalCmd(pathValue string) tea.Cmd {
	return w.loadLocalCmdWithStatus(pathValue, true)
}

func (w browserWorkflow) loadLocalCmdWithStatus(pathValue string, updateStatus bool) tea.Cmd {
	m := w.model
	m.browser.localPanel.loading = true
	m.browser.localPanel.request++
	request := m.browser.localPanel.request
	if updateStatus {
		m.setInfoStatus(m.translator.T("status.loading_browser"))
	}
	return func() tea.Msg {
		items, currentPath, err := m.services.Files.ListLocal(pathValue)
		return browserLoadedMsg{panel: domain.LocalPanel, items: items, path: currentPath, request: request, err: err}
	}
}

func (w browserWorkflow) loadRemoteCmd(pathValue string) tea.Cmd {
	return w.loadRemoteCmdWithStatus(pathValue, true)
}

func (w browserWorkflow) loadRemoteCmdWithStatus(pathValue string, updateStatus bool) tea.Cmd {
	m := w.model
	m.browser.remotePanel.loading = true
	m.browser.remotePanel.request++
	request := m.browser.remotePanel.request
	if updateStatus {
		m.setInfoStatus(m.translator.T("status.loading_browser"))
	}
	return func() tea.Msg {
		if m.browser.session == nil {
			return browserLoadedMsg{panel: domain.RemotePanel, request: request, err: errBrowserSessionNotReady()}
		}
		items, currentPath, err := m.browser.session.ListRemote(pathValue)
		return browserLoadedMsg{panel: domain.RemotePanel, items: items, path: currentPath, request: request, err: err}
	}
}

func (w browserWorkflow) reloadCmd() tea.Cmd {
	m := w.model
	return tea.Batch(
		w.loadLocalCmd(m.browser.localPanel.path),
		w.loadRemoteCmd(m.browser.remotePanel.path),
	)
}

func (w browserWorkflow) reloadSelectCmd(targetPanel domain.FilePanel, selectName string) tea.Cmd {
	m := w.model
	m.browser.localPanel.loading = true
	m.browser.localPanel.request++
	localRequest := m.browser.localPanel.request
	m.browser.remotePanel.loading = true
	m.browser.remotePanel.request++
	remoteRequest := m.browser.remotePanel.request
	return tea.Batch(
		func() tea.Msg {
			items, currentPath, err := m.services.Files.ListLocal(m.browser.localPanel.path)
			msg := browserLoadedMsg{panel: domain.LocalPanel, items: items, path: currentPath, request: localRequest, err: err}
			if targetPanel == domain.LocalPanel {
				msg.selectName = selectName
			}
			return msg
		},
		func() tea.Msg {
			if m.browser.session == nil {
				return browserLoadedMsg{panel: domain.RemotePanel, request: remoteRequest, err: errBrowserSessionNotReady()}
			}
			items, currentPath, err := m.browser.session.ListRemote(m.browser.remotePanel.path)
			msg := browserLoadedMsg{panel: domain.RemotePanel, items: items, path: currentPath, request: remoteRequest, err: err}
			if targetPanel == domain.RemotePanel {
				msg.selectName = selectName
			}
			return msg
		},
	)
}

func (w browserWorkflow) navigatePathCmd(targetPanel domain.FilePanel, path string) tea.Cmd {
	m := w.model
	if targetPanel == domain.LocalPanel {
		if path != m.browser.localPanel.path {
			m.browser.localPanel.filter = ""
		}
		return w.loadLocalCmd(path)
	}
	if path != m.browser.remotePanel.path {
		m.browser.remotePanel.filter = ""
	}
	return w.loadRemoteCmdWithStatus(path, false)
}

func errBrowserSessionNotReady() error {
	return domain.ErrBrowserSessionNotReady
}

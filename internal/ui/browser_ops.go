package ui

import (
	"path"
	"path/filepath"
	"sshm/internal/domain"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) openBrowserMkdirInput() (tea.Model, tea.Cmd) {
	m.clearStaleErrorStatus()
	m.overlay = overlayBrowserInput
	m.browser.inputMode = browserInputMkdir
	m.browser.inputItem = domain.FileEntry{}
	m.browser.input.SetValue("")
	m.browser.input.Focus()
	return m, nil
}

func (m *Model) openBrowserRenameInput() (tea.Model, tea.Cmd) {
	row, ok := m.activeBrowserEditableEntry()
	if !ok {
		return m, nil
	}
	m.clearStaleErrorStatus()
	m.overlay = overlayBrowserInput
	m.browser.inputMode = browserInputRename
	m.browser.inputItem = row
	m.browser.input.SetValue(row.Name)
	m.browser.input.Focus()
	return m, nil
}

func (m *Model) submitBrowserMkdir(value string) (tea.Model, tea.Cmd) {
	panel := m.browser.activePanel
	currentPath := m.activeBrowserPanel().path
	targetPath := resolveBrowserTargetPath(panel, currentPath, value)
	if strings.TrimSpace(targetPath) == "" {
		m.setErrorStatus(domain.ErrPathRequired)
		return m, nil
	}
	selectName := browserBaseName(panel, targetPath)
	return m.browserWorkflow().prepareOperation(
		"",
		currentPath,
		targetPath,
		panel,
		func(func(domain.TransferProgress)) error {
			return m.browserMkdir(panel, targetPath)
		},
		nil,
		m.translator.T("status.browser_dir_created", selectName),
		selectName,
		m.translator.T("status.cancelled"),
	)
}

func (m *Model) submitBrowserRename(value string) (tea.Model, tea.Cmd) {
	row, ok := m.activeBrowserInputEntry()
	if !ok {
		return m, nil
	}
	name := strings.TrimSpace(value)
	if name == "" {
		m.setErrorStatus(domain.ErrFileNameRequired)
		return m, nil
	}
	if !isValidBrowserFileName(row.Panel, name) {
		m.setErrorStatus(domain.ErrFileNamePathSeparator)
		return m, nil
	}
	if name == row.Name {
		return m, nil
	}
	targetPath := browserJoinPath(row.Panel, m.activeBrowserPanel().path, name)
	return m.browserWorkflow().prepareOperation(
		"",
		row.Path,
		targetPath,
		row.Panel,
		func(func(domain.TransferProgress)) error {
			return m.browserRename(row.Panel, row.Path, targetPath)
		},
		func() error {
			return m.browserRemove(row.Panel, targetPath)
		},
		m.translator.T("status.browser_renamed", row.Name, name),
		name,
		m.translator.T("status.cancelled"),
	)
}

func (m *Model) confirmBrowserDelete() (tea.Model, tea.Cmd) {
	row, ok := m.activeBrowserEditableEntry()
	if !ok {
		return m, nil
	}
	m.clearStaleErrorStatus()
	m.openBrowserDeleteConfirm(row, &browserPendingOperation{
		run: func(func(domain.TransferProgress)) error {
			return m.browserRemove(row.Panel, row.Path)
		},
		success: m.translator.T("status.browser_deleted", row.Name),
		cancel:  m.translator.T("status.cancelled"),
		panel:   row.Panel,
	})
	return m, nil
}

func (m *Model) activeBrowserEditableEntry() (domain.FileEntry, bool) {
	row, ok := m.activeBrowserPanel().selected()
	if !ok || row.Name == ".." {
		return domain.FileEntry{}, false
	}
	return row, true
}

func (m *Model) activeBrowserInputEntry() (domain.FileEntry, bool) {
	if m.browser.inputItem.Name == "" && strings.TrimSpace(m.browser.inputItem.Path) == "" {
		return domain.FileEntry{}, false
	}
	return m.browser.inputItem, true
}

func (m *Model) browserMkdir(panel domain.FilePanel, targetPath string) error {
	if panel == domain.RemotePanel {
		if m.browser.session == nil {
			return errBrowserSessionNotReady()
		}
		return m.browser.session.Mkdir(targetPath)
	}
	return m.services.Files.MkdirLocal(targetPath)
}

func (m *Model) browserRemove(panel domain.FilePanel, targetPath string) error {
	if panel == domain.RemotePanel {
		if m.browser.session == nil {
			return errBrowserSessionNotReady()
		}
		return m.browser.session.Remove(targetPath)
	}
	return m.services.Files.RemoveLocal(targetPath)
}

func (m *Model) browserRename(panel domain.FilePanel, sourcePath string, targetPath string) error {
	if panel == domain.RemotePanel {
		if m.browser.session == nil {
			return errBrowserSessionNotReady()
		}
		return m.browser.session.Rename(sourcePath, targetPath)
	}
	return m.services.Files.RenameLocal(sourcePath, targetPath)
}

func resolveBrowserTargetPath(panel domain.FilePanel, currentPath string, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if panel == domain.RemotePanel {
		if strings.HasPrefix(trimmed, "/") || trimmed == "~" || strings.HasPrefix(trimmed, "~/") {
			return path.Clean(trimmed)
		}
		return joinRemotePath(currentPath, trimmed)
	}
	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed)
	}
	return filepath.Clean(filepath.Join(currentPath, trimmed))
}

func browserJoinPath(panel domain.FilePanel, currentPath string, name string) string {
	if panel == domain.RemotePanel {
		return joinRemotePath(currentPath, name)
	}
	return filepath.Join(currentPath, name)
}

func browserBaseName(panel domain.FilePanel, targetPath string) string {
	if panel == domain.RemotePanel {
		return path.Base(targetPath)
	}
	return filepath.Base(targetPath)
}

func isValidBrowserFileName(panel domain.FilePanel, value string) bool {
	if panel == domain.RemotePanel {
		return !strings.Contains(value, "/")
	}
	return !strings.ContainsAny(value, `/\`)
}

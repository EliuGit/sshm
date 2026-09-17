package tui

import (
	"cmp"
	"context"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"sshm/internal/repository"
	ssh "sshm/internal/ssh"
)

// transferSide 表示文件浏览器当前显示本地还是远程位置。
type transferSide uint8

const (
	localSide transferSide = iota
	remoteSide
)

// transferFocus 表示文件传输弹窗当前接收键盘输入的区域。
type transferFocus uint8

const (
	listFocus transferFocus = iota
	searchFocus
	addressFocus
)

// transferEntry 保存文件列表展示和本地操作需要的目录项信息。
type transferEntry struct {
	name     string
	size     int64
	modified string
	dir      bool
	symlink  bool
}

// transferClipboard 保存内部复制剪贴板的来源目录和项目。
type transferClipboard struct {
	source  transferSide
	path    string
	entries []transferEntry
}

// transferModel 管理文件传输弹窗的导航、焦点、多选、排序、剪贴板和任务状态。
type transferModel struct {
	store         *repository.Store
	connection    connectionRow
	width         int
	height        int
	location      transferSide
	localPath     string
	remotePath    string
	remoteHome    string
	localEntries  []transferEntry
	remoteEntries []transferEntry
	cursor        int
	localCursor   int
	remoteCursor  int
	selected      map[string]struct{}
	sortField     byte
	sortAsc       bool
	focus         transferFocus
	search        textinput.Model
	address       textinput.Model
	clipboard     transferClipboard
	overlay       transferOverlay
	task          *transferTask
	sftp          *ssh.SFTP
	cancelConnect context.CancelFunc
	cancelRead    context.CancelFunc
	status        string
}

// newFileTransfer 创建文件传输弹窗并读取本地初始目录。
func newTransfer(connection connectionRow, workingDir string, width, height int) transferModel {
	search := newTransferInput()
	search.Placeholder = "筛选文件..."
	address := newTransferInput()
	address.CharLimit = 1000
	m := transferModel{
		connection: connection,
		width:      width,
		height:     height,
		localPath:  workingDir,
		remotePath: "/",
		selected:   make(map[string]struct{}),
		sortField:  'n',
		sortAsc:    true,
		search:     search,
		address:    address,
		status:     "正在读取本地目录",
	}
	if m.localPath == "" {
		var err error
		m.localPath, err = os.Getwd()
		if err != nil {
			m.status = "读取当前工作目录失败：" + err.Error()
			return m
		}
	}
	m.syncAddress()
	m.refreshLocal()
	return m
}

func newTransferInput() textinput.Model {
	input := textinput.New()
	input.Prompt = ""
	input.CharLimit = 200
	styles := input.Styles()
	styles.Focused.Text = accentStyle
	styles.Focused.Placeholder = mutedStyle
	styles.Blurred.Text = plainStyle
	styles.Blurred.Placeholder = mutedStyle
	styles.Cursor.Color = accentStyle.GetForeground()
	input.SetStyles(styles)
	return input
}

// Update 处理文件传输弹窗的键盘交互和后台文件消息。
func (m transferModel) Update(msg tea.Msg) (modalModel, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = size.Width, size.Height
		return m, nil
	}
	if updated, cmd, ok := m.handleTask(msg); ok {
		return updated, cmd
	}
	if updated, cmd, ok := m.handleRemote(msg); ok {
		return updated, cmd
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if m.overlay.kind != overlayNone {
		return m.handleOverlay(key)
	}
	if m.cancelRead != nil {
		if key.String() == "q" {
			m.cancelRemoteRead()
			m.closeSFTP()
			return nil, nil
		}
		m.status = "正在读取远程目录"
		return m, nil
	}
	if m.focus == addressFocus {
		return m.updateAddress(key)
	}
	if m.focus == searchFocus {
		return m.updateSearch(key)
	}
	if key.String() == "ctrl+c" || key.String() == "esc" {
		focused, hasFocused := m.currentEntry()
		m.search.Reset()
		clear(m.selected)
		if hasFocused {
			m.focusEntry(focused.name)
		}
		return m, nil
	}

	switch key.String() {
	case "q":
		m.closeSFTP()
		return nil, nil
	case "ctrl+g":
		m.focus = addressFocus
		m.address.SetValue(m.currentPath())
		m.address.CursorEnd()
		return m, m.address.Focus()
	case "/":
		m.focus = searchFocus
		return m, m.search.Focus()
	case "1":
		return m, m.switchLocation(localSide)
	case "2":
		return m, m.switchLocation(remoteSide)
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.visibleEntries())-1 {
			m.cursor++
		}
	case "h", "backspace":
		return m, m.goParent()
	case "l", "enter":
		return m, m.enterDirectory()
	case "space":
		m.toggleSelection()
	case "ctrl+a":
		for _, entry := range m.visibleEntries() {
			m.selected[entry.name] = struct{}{}
		}
	case "y":
		m.copySelection()
	case "p":
		m.pruneClipboard()
		if len(m.clipboard.entries) == 0 {
			m.status = "剪贴板为空"
		} else if (m.location == remoteSide || m.clipboard.source == remoteSide) && m.sftp == nil {
			m.status = "远程连接不可用"
		} else {
			return m.startPaste()
		}
	case "n", "s", "t":
		m.toggleSort(key.String()[0])
	case "d":
		m.openDelete()
	case "r":
		if m.location == remoteSide && m.sftp == nil {
			m.status = "远程连接不可用"
		} else {
			return m, m.openRename()
		}
	case "a":
		if m.location == remoteSide && m.sftp == nil {
			m.status = "远程连接不可用"
		} else {
			return m, m.openCreate()
		}
	case "?":
		m.overlay = transferOverlay{kind: overlayHelp}
	}
	return m, nil
}

func (m transferModel) updateAddress(key tea.KeyPressMsg) (modalModel, tea.Cmd) {
	switch key.String() {
	case "ctrl+c":
		m.address.Reset()
		return m, nil
	case "esc":
		m.address.SetValue(m.currentPath())
		m.address.Blur()
		m.focus = listFocus
		return m, nil
	case "enter":
		target := strings.TrimSpace(m.address.Value())
		if target == "" {
			m.status = "路径不能为空"
			return m, nil
		}
		if m.location == localSide {
			if err := m.changeLocalDir(target); err != nil {
				m.status = "跳转失败：" + err.Error()
				return m, nil
			}
		} else {
			return m, m.loadRemoteDir(target, "", true)
		}
		m.address.Blur()
		m.focus = listFocus
		m.syncAddress()
		return m, nil
	}
	var cmd tea.Cmd
	m.address, cmd = m.address.Update(key)
	return m, cmd
}

func (m transferModel) updateSearch(key tea.KeyPressMsg) (modalModel, tea.Cmd) {
	switch key.String() {
	case "ctrl+c", "esc":
		m.search.Reset()
		m.cursor = 0
		clear(m.selected)
		if key.String() == "esc" {
			m.search.Blur()
			m.focus = listFocus
		}
		return m, nil
	case "enter":
		m.search.Blur()
		m.focus = listFocus
		return m, nil
	}
	before := m.search.Value()
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(key)
	if m.search.Value() != before {
		m.cursor = 0
		clear(m.selected)
	}
	return m, cmd
}

func (m *transferModel) switchLocation(location transferSide) tea.Cmd {
	if m.location == location {
		if location == remoteSide && m.sftp == nil && m.cancelConnect == nil {
			return m.connectRemote()
		}
		return nil
	}
	if m.location == localSide {
		m.localCursor = m.cursor
	} else {
		m.remoteCursor = m.cursor
	}
	m.location = location
	clear(m.selected)
	m.syncAddress()
	cursor := m.remoteCursor
	var cmd tea.Cmd
	if location == localSide {
		m.refreshLocal()
		cursor = m.localCursor
	} else if m.sftp == nil {
		cmd = m.connectRemote()
	} else {
		m.status = "远程目录已就绪"
	}
	m.cursor = min(cursor, max(0, len(m.visibleEntries())-1))
	return cmd
}

func (m *transferModel) goParent() tea.Cmd {
	current := m.currentPath()
	child := filepath.Base(filepath.Clean(current))
	if m.location == remoteSide {
		child = path.Base(path.Clean(current))
		return m.loadRemoteDir(path.Dir(current), child, false)
	} else {
		if err := m.changeLocalDir(filepath.Dir(current)); err != nil {
			m.status = "返回上级失败：" + err.Error()
			return nil
		}
		m.focusEntry(child)
		return nil
	}
}

func (m *transferModel) enterDirectory() tea.Cmd {
	entry, ok := m.currentEntry()
	if !ok || !entry.dir {
		return nil
	}
	if entry.symlink {
		m.status = "符号链接仅展示，不能进入"
		return nil
	}
	if m.location == remoteSide {
		return m.loadRemoteDir(path.Join(m.remotePath, entry.name), "", false)
	} else {
		if err := m.changeLocalDir(filepath.Join(m.localPath, entry.name)); err != nil {
			m.status = "进入目录失败：" + err.Error()
		}
		return nil
	}
}

func (m *transferModel) toggleSelection() {
	entry, ok := m.currentEntry()
	if !ok {
		return
	}
	if _, exists := m.selected[entry.name]; exists {
		delete(m.selected, entry.name)
		return
	}
	m.selected[entry.name] = struct{}{}
}

func (m *transferModel) copySelection() {
	if m.location == remoteSide && m.sftp == nil {
		m.status = "远程连接不可用"
		return
	}
	hadSelection := len(m.selected) > 0
	chosen := m.operationEntries()
	if !hadSelection && len(chosen) > 0 {
		m.selected[chosen[0].name] = struct{}{}
	}
	for _, entry := range chosen {
		if entry.symlink {
			m.status = "符号链接不能复制"
			return
		}
	}
	if len(chosen) == 0 {
		m.status = "没有可复制的项目"
		return
	}
	m.clipboard = transferClipboard{source: m.location, path: m.currentPath(), entries: chosen}
	m.status = "复制：已加入剪贴板"
}

func (m *transferModel) toggleSort(field byte) {
	current, hasCurrent := m.currentEntry()
	if m.sortField == field {
		m.sortAsc = !m.sortAsc
	} else {
		m.sortField, m.sortAsc = field, true
	}
	m.cursor = 0
	if hasCurrent {
		m.focusEntry(current.name)
	}
}

// resetSelection 在路径切换后清空临时多选。
func (m *transferModel) resetSelection() {
	clear(m.selected)
	m.cursor = 0
}

func (m transferModel) atClipboardSource() bool {
	if len(m.clipboard.entries) == 0 || m.clipboard.source != m.location {
		return false
	}
	if m.location == remoteSide {
		return path.Clean(m.clipboard.path) == path.Clean(m.remotePath)
	}
	return filepath.Clean(m.clipboard.path) == filepath.Clean(m.localPath)
}

func (m *transferModel) syncAddress() {
	m.address.SetValue(m.currentPath())
}

func (m transferModel) currentPath() string {
	if m.location == remoteSide {
		return m.remotePath
	}
	return m.localPath
}

func (m transferModel) currentEntry() (transferEntry, bool) {
	entries := m.visibleEntries()
	if m.cursor < 0 || m.cursor >= len(entries) {
		return transferEntry{}, false
	}
	return entries[m.cursor], true
}

// focusEntry 按当前排序定位项目；筛选隐藏目标时清空筛选以保证目标可见。
func (m *transferModel) focusEntry(name string) {
	for index, entry := range m.visibleEntries() {
		if entry.name == name {
			m.cursor = index
			return
		}
	}
	if m.search.Value() == "" {
		return
	}
	m.search.Reset()
	for index, entry := range m.visibleEntries() {
		if entry.name == name {
			m.cursor = index
			return
		}
	}
}

// visibleEntries 返回经过筛选和排序的当前目录项，文件夹分组始终排在文件之前。
func (m transferModel) visibleEntries() []transferEntry {
	query := strings.ToLower(strings.TrimSpace(m.search.Value()))
	all := m.currentEntries()
	entries := make([]transferEntry, 0, len(all))
	for _, entry := range all {
		if query == "" || strings.Contains(strings.ToLower(entry.name), query) {
			entries = append(entries, entry)
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		left, right := entries[i], entries[j]
		if left.dir != right.dir {
			return left.dir
		}
		comparison := 0
		switch m.sortField {
		case 's':
			comparison = cmp.Compare(left.size, right.size)
		case 't':
			comparison = strings.Compare(left.modified, right.modified)
		default:
			comparison = strings.Compare(strings.ToLower(left.name), strings.ToLower(right.name))
		}
		if comparison == 0 {
			comparison = strings.Compare(strings.ToLower(left.name), strings.ToLower(right.name))
		}
		if m.sortAsc {
			return comparison < 0
		}
		return comparison > 0
	})
	return entries
}

func (m transferModel) currentEntries() []transferEntry {
	if m.location == remoteSide {
		return m.remoteEntries
	}
	return m.localEntries
}

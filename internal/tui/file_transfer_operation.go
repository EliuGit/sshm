package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	ssh "sshm/internal/ssh"
)

// sftpReadyMsg 携带异步建立的 SFTP 会话及远程初始目录。
type sftpReadyMsg struct {
	client  *ssh.SFTP
	home    string
	entries []transferEntry
	err     error
}

// remoteDirMsg 携带一次远程目录读取结果。
type remoteDirMsg struct {
	client  *ssh.SFTP
	path    string
	focus   string
	address bool
	entries []transferEntry
	err     error
}

// transferTask 保存单个后台文件任务的取消函数和进度消息通道。
type transferTask struct {
	cancel         context.CancelFunc
	updates        chan tea.Msg
	clearClipboard bool
	run            func(context.Context, *transferTask)
	remote         bool
	renameOld      string
	renameNew      string
	focus          string
}

// localOp 描述一批本地复制或删除任务。
type localOp struct {
	action    byte
	sources   []string
	targetDir string
}

// transferProgressMsg 将后台任务的当前项目和字节进度交回界面。
type transferProgressMsg struct {
	task           *transferTask
	item           string
	itemIndex      int
	totalItems     int
	processedBytes int64
	totalBytes     int64
}

// transferDoneMsg 表示后台任务已经完成、取消或失败。
type transferDoneMsg struct {
	task      *transferTask
	item      string
	cancelled bool
	err       error
}

// startRemoteConnection 解密当前连接凭据，并在后台完成 SSH 认证和远程主目录读取。
func (m *transferModel) connectRemote() tea.Cmd {
	if m.store == nil {
		m.status = "数据库未连接，无法建立远程连接"
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelConnect = cancel
	m.overlay = transferOverlay{
		kind:      overlayProgress,
		progress:  progressConnecting,
		operation: "连接",
		item:      m.connection.name,
	}
	m.status = "正在建立远程连接"
	store, connectionID := m.store, m.connection.id
	return func() tea.Msg {
		connection, credential, err := store.SSHConnection(connectionID)
		if err != nil {
			clear(credential)
			if ctx.Err() != nil {
				err = ctx.Err()
			}
			return sftpReadyMsg{err: err}
		}
		if err := ctx.Err(); err != nil {
			clear(credential)
			return sftpReadyMsg{err: err}
		}
		client, err := ssh.NewSFTP(ctx, connection, credential)
		if err != nil {
			return sftpReadyMsg{err: err}
		}
		stopCancel := context.AfterFunc(ctx, func() { _ = client.Close() })
		if err := ctx.Err(); err != nil {
			if stopCancel() {
				_ = client.Close()
			}
			return sftpReadyMsg{err: err}
		}
		home, err := client.Getwd()
		if err == nil {
			var infos []os.FileInfo
			infos, err = client.ReadDir(ctx, home)
			if err == nil {
				if !stopCancel() || ctx.Err() != nil {
					return sftpReadyMsg{err: ctx.Err()}
				}
				return sftpReadyMsg{client: client, home: path.Clean(home), entries: entriesFromRemote(infos)}
			}
			err = fmt.Errorf("读取远程主目录 %s: %w", home, err)
		} else {
			err = fmt.Errorf("读取远程主目录: %w", err)
		}
		if stopCancel() {
			_ = client.Close()
		}
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return sftpReadyMsg{err: err}
	}
}

// updateRemote 消费 SFTP 连接和目录读取结果，忽略已经关闭会话的过期消息。
func (m transferModel) handleRemote(msg tea.Msg) (modalModel, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case sftpReadyMsg:
		if m.cancelConnect == nil {
			if msg.client != nil {
				_ = msg.client.Close()
			}
			return m, nil, true
		}
		m.cancelConnect()
		m.cancelConnect = nil
		if m.overlay.cancelRequested {
			if msg.client != nil {
				_ = msg.client.Close()
			}
			m.overlay.progress = progressCancelled
			m.status = "远程连接已取消"
			return m, nil, true
		}
		if msg.err != nil {
			if errors.Is(msg.err, context.Canceled) {
				m.overlay.progress = progressCancelled
				m.status = "远程连接已取消"
			} else {
				m.overlay.progress = progressFailed
				m.overlay.error = msg.err.Error()
				m.status = "远程连接失败"
			}
			return m, nil, true
		}
		m.sftp = msg.client
		m.remoteHome = msg.home
		m.remotePath = msg.home
		m.remoteEntries = msg.entries
		m.overlay = transferOverlay{}
		m.syncAddress()
		m.status = fmt.Sprintf("远程连接成功，已读取 %d 项", len(msg.entries))
		return m, nil, true
	case remoteDirMsg:
		if msg.client != m.sftp {
			return m, nil, true
		}
		if m.cancelRead != nil {
			m.cancelRead()
			m.cancelRead = nil
		}
		if errors.Is(msg.err, context.Canceled) {
			return m, nil, true
		}
		if msg.err != nil {
			prefix := "读取远程目录失败："
			if msg.address {
				prefix = "跳转失败："
			}
			m.status = prefix + msg.err.Error()
			return m, nil, true
		}
		m.remotePath, m.remoteEntries = msg.path, msg.entries
		if m.location == remoteSide {
			m.resetSelection()
			m.syncAddress()
			if msg.focus != "" {
				m.focusEntry(msg.focus)
			}
			if msg.address {
				m.address.Blur()
				m.focus = listFocus
			}
			m.status = fmt.Sprintf("已读取 %d 项", len(msg.entries))
		}
		return m, nil, true
	default:
		return m, nil, false
	}
}

func (m *transferModel) loadRemoteDir(target, focus string, address bool) tea.Cmd {
	if m.sftp == nil {
		m.status = "远程连接不可用"
		return nil
	}
	if m.cancelRead != nil {
		m.status = "正在读取远程目录"
		return nil
	}
	resolved := resolveRemoteDir(m.remotePath, m.remoteHome, target)
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelRead = cancel
	m.status = "正在读取远程目录"
	client := m.sftp
	return func() tea.Msg {
		info, err := client.Lstat(resolved)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			err = errors.New("符号链接仅展示，不能进入")
		}
		if err == nil && !info.IsDir() {
			err = errors.New("目标不是目录")
		}
		var infos []os.FileInfo
		if err == nil {
			infos, err = client.ReadDir(ctx, resolved)
		}
		if err == nil {
			err = ctx.Err()
		}
		return remoteDirMsg{client: client, path: resolved, focus: focus, address: address, entries: entriesFromRemote(infos), err: err}
	}
}

func (m *transferModel) cancelRemoteRead() {
	if m.cancelRead != nil {
		m.cancelRead()
		m.cancelRead = nil
	}
}

func (m *transferModel) closeSFTP() {
	m.cancelRemoteRead()
	if m.cancelConnect != nil {
		m.cancelConnect()
		m.cancelConnect = nil
	}
	if m.sftp != nil {
		_ = m.sftp.Close()
		m.sftp = nil
	}
}

func resolveRemoteDir(current, home, target string) string {
	if target == "~" {
		target = home
	} else if strings.HasPrefix(target, "~/") {
		target = path.Join(home, strings.TrimPrefix(target, "~/"))
	} else if !path.IsAbs(target) {
		target = path.Join(current, target)
	}
	return path.Clean(target)
}

func entriesFromRemote(infos []os.FileInfo) []transferEntry {
	entries := make([]transferEntry, 0, len(infos))
	for _, info := range infos {
		entries = append(entries, transferEntry{
			name: info.Name(), size: info.Size(), modified: info.ModTime().Format("2006-01-02 15:04"),
			dir: info.IsDir(), symlink: info.Mode()&os.ModeSymlink != 0,
		})
	}
	return entries
}

// readLocalDirectory 读取单层目录项，不跟随符号链接。
func readLocalDir(directory string) ([]transferEntry, error) {
	items, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	entries := make([]transferEntry, 0, len(items))
	for _, item := range items {
		info, err := item.Info()
		if err != nil {
			return nil, fmt.Errorf("读取 %s: %w", item.Name(), err)
		}
		entries = append(entries, transferEntry{
			name:     item.Name(),
			size:     info.Size(),
			modified: info.ModTime().Format("2006-01-02 15:04"),
			dir:      item.IsDir(),
			symlink:  item.Type()&os.ModeSymlink != 0,
		})
	}
	return entries, nil
}

// resolveLocalDirectory 解析绝对路径、相对路径和本地用户主目录写法。
func resolveLocalDir(current, target string) (string, error) {
	if target == "~" || strings.HasPrefix(target, "~/") || strings.HasPrefix(target, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("读取用户主目录: %w", err)
		}
		target = filepath.Join(home, strings.TrimLeft(target[1:], `/\`))
	} else if !filepath.IsAbs(target) {
		target = filepath.Join(current, target)
	}
	absolute, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("符号链接仅展示，不能进入")
	}
	if !info.IsDir() {
		return "", errors.New("目标不是目录")
	}
	return filepath.Clean(absolute), nil
}

func (m *transferModel) refreshLocal() error {
	entries, err := readLocalDir(m.localPath)
	if err != nil {
		m.status = "读取目录失败：" + err.Error()
		return err
	}
	m.localEntries = entries
	m.pruneClipboard()
	m.resetSelection()
	m.status = fmt.Sprintf("已读取 %d 项", len(entries))
	return nil
}

func (m *transferModel) changeLocalDir(target string) error {
	resolved, err := resolveLocalDir(m.localPath, target)
	if err != nil {
		return err
	}
	entries, err := readLocalDir(resolved)
	if err != nil {
		return err
	}
	m.localPath = resolved
	m.localEntries = entries
	m.pruneClipboard()
	m.resetSelection()
	m.syncAddress()
	m.status = fmt.Sprintf("已读取 %d 项", len(entries))
	return nil
}

func (m transferModel) operationEntries() []transferEntry {
	if len(m.selected) == 0 {
		if entry, ok := m.currentEntry(); ok {
			return []transferEntry{entry}
		}
		return nil
	}
	entries := make([]transferEntry, 0, len(m.selected))
	for _, entry := range m.currentEntries() {
		if _, ok := m.selected[entry.name]; ok {
			entries = append(entries, entry)
		}
	}
	return entries
}

func (m transferModel) startPaste() (modalModel, tea.Cmd) {
	if m.location != localSide || m.clipboard.source != localSide {
		return m.startRemotePaste()
	}
	sources := make([]string, 0, len(m.clipboard.entries))
	for _, entry := range m.clipboard.entries {
		sources = append(sources, filepath.Join(m.clipboard.path, entry.name))
	}
	operation := localOp{action: 'c', sources: sources, targetDir: m.localPath}
	return m.startLocalTask(operation, "复制", true)
}

func (m transferModel) startDelete() (modalModel, tea.Cmd) {
	if m.location == remoteSide {
		return m.startRemoteDelete()
	}
	entries := m.operationEntries()
	if len(entries) == 0 {
		m.overlay = transferOverlay{}
		m.status = "没有可删除的项目"
		return m, nil
	}
	for _, entry := range entries {
		if entry.symlink {
			m.status = "符号链接不能删除"
			return m, nil
		}
	}
	sources := make([]string, 0, len(entries))
	for _, entry := range entries {
		sources = append(sources, filepath.Join(m.localPath, entry.name))
	}
	return m.startLocalTask(localOp{action: 'd', sources: sources}, "删除", false)
}

func (m transferModel) startLocalTask(operation localOp, name string, clearClipboard bool) (modalModel, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	task := &transferTask{cancel: cancel, updates: make(chan tea.Msg), clearClipboard: clearClipboard}
	task.run = func(ctx context.Context, task *transferTask) { runLocalTask(ctx, task, operation) }
	return m.startTask(task, name, operation.sources, ctx)
}

func (m transferModel) startTask(task *transferTask, name string, sources []string, ctx context.Context) (modalModel, tea.Cmd) {
	m.task = task
	item := ""
	if len(sources) > 0 {
		item = filepath.Base(sources[0])
	}
	m.overlay = transferOverlay{
		kind:       overlayProgress,
		progress:   progressRunning,
		operation:  name,
		item:       item,
		itemIndex:  1,
		totalItems: len(sources),
	}
	return m, func() tea.Msg {
		go task.run(ctx, task)
		return <-task.updates
	}
}

// startRemotePasteOperation 根据剪贴板来源和当前目标端选择上传、下载或远程复制。
func (m transferModel) startRemotePaste() (modalModel, tea.Cmd) {
	if m.sftp == nil {
		m.status = "远程连接不可用"
		return m, nil
	}
	entries := m.clipboard.entries
	if len(entries) == 0 {
		m.status = "剪贴板为空"
		return m, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	task := &transferTask{cancel: cancel, updates: make(chan tea.Msg), clearClipboard: true}
	sourceLocation, targetLocation := m.clipboard.source, m.location
	task.remote = sourceLocation == remoteSide || targetLocation == remoteSide
	sourceDir, targetDir := m.clipboard.path, m.currentPath()
	task.run = func(ctx context.Context, task *transferTask) {
		runRemotePaste(ctx, task, m.sftp, sourceLocation, targetLocation, sourceDir, targetDir, entries)
	}
	sources := make([]string, len(entries))
	for i := range entries {
		sources[i] = entries[i].name
	}
	name := "复制"
	if sourceLocation != targetLocation {
		if sourceLocation == localSide {
			name = "上传"
		} else {
			name = "下载"
		}
	}
	return m.startTask(task, name, sources, ctx)
}

// startRemoteDeleteOperation 在后台递归删除远程选择，并通过同一任务通道报告进度。
func (m transferModel) startRemoteDelete() (modalModel, tea.Cmd) {
	if m.sftp == nil {
		m.overlay = transferOverlay{}
		m.status = "远程连接不可用"
		return m, nil
	}
	entries := m.operationEntries()
	if len(entries) == 0 {
		m.overlay = transferOverlay{}
		m.status = "没有可删除的项目"
		return m, nil
	}
	for _, entry := range entries {
		if entry.symlink {
			m.overlay = transferOverlay{}
			m.status = "符号链接不能删除"
			return m, nil
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	task := &transferTask{cancel: cancel, updates: make(chan tea.Msg), remote: true}
	directory := m.remotePath
	task.run = func(ctx context.Context, task *transferTask) {
		var processed, total int64
		for _, entry := range entries {
			size, err := m.sftp.Size(ctx, path.Join(directory, entry.name))
			if err != nil {
				task.updates <- transferDoneMsg{task: task, item: entry.name, cancelled: errors.Is(err, context.Canceled), err: err}
				return
			}
			total += size
		}
		for index, entry := range entries {
			if err := ctx.Err(); err != nil {
				task.updates <- transferDoneMsg{task: task, item: entry.name, cancelled: true, err: err}
				return
			}
			task.updates <- transferProgressMsg{task: task, item: entry.name, itemIndex: index + 1, totalItems: len(entries), processedBytes: processed, totalBytes: total}
			err := m.sftp.RemoveAll(ctx, path.Join(directory, entry.name), func(delta int64) {
				processed += delta
				task.updates <- transferProgressMsg{task: task, item: entry.name, itemIndex: index + 1, totalItems: len(entries), processedBytes: processed, totalBytes: total}
			})
			if err != nil {
				task.updates <- transferDoneMsg{task: task, item: entry.name, cancelled: errors.Is(err, context.Canceled), err: err}
				return
			}
		}
		task.updates <- transferDoneMsg{task: task, item: entries[len(entries)-1].name}
	}
	sources := make([]string, len(entries))
	for i := range entries {
		sources[i] = entries[i].name
	}
	return m.startTask(task, "删除", sources, ctx)
}

// startRemoteRename 校验单个名称、拒绝覆盖，并在后台提交远程重命名。
func (m transferModel) startRemoteRename() (modalModel, tea.Cmd) {
	if m.sftp == nil {
		m.overlay.error = "远程连接不可用"
		return m, nil
	}
	oldName, newName := m.overlay.renameOld, strings.TrimSpace(m.overlay.renameInput.Value())
	if newName == "" || newName == "." || newName == ".." || path.Base(newName) != newName || strings.Contains(newName, "/") {
		m.overlay.error = "名称不能包含路径且不能为空"
		return m, nil
	}
	if newName == oldName {
		m.overlay = transferOverlay{}
		return m, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	task := &transferTask{cancel: cancel, updates: make(chan tea.Msg), remote: true, renameOld: oldName, renameNew: newName}
	oldPath, newPath := path.Join(m.remotePath, oldName), path.Join(m.remotePath, newName)
	task.run = func(ctx context.Context, task *transferTask) {
		task.updates <- transferProgressMsg{task: task, item: oldName, itemIndex: 1, totalItems: 1}
		if _, err := m.sftp.Lstat(newPath); err == nil {
			task.updates <- transferDoneMsg{task: task, item: oldName, err: errors.New("目标名称已存在")}
			return
		} else if !errors.Is(err, os.ErrNotExist) {
			task.updates <- transferDoneMsg{task: task, item: oldName, err: err}
			return
		}
		err := m.sftp.Rename(oldPath, newPath)
		task.updates <- transferDoneMsg{task: task, item: oldName, err: err}
	}
	return m.startTask(task, "重命名", []string{oldName}, ctx)
}

func validName(name string) error {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
		return errors.New("名称不能包含路径且不能为空")
	}
	return nil
}

func (m transferModel) startCreate(name string) (modalModel, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	task := &transferTask{cancel: cancel, updates: make(chan tea.Msg), remote: m.location == remoteSide, focus: name}
	directory := m.currentPath()
	task.run = func(ctx context.Context, task *transferTask) {
		task.updates <- transferProgressMsg{task: task, item: name, itemIndex: 1, totalItems: 1}
		var err error
		if task.remote {
			err = m.sftp.Mkdir(path.Join(directory, name))
		} else {
			err = os.Mkdir(filepath.Join(directory, name), 0o755)
		}
		if errors.Is(err, os.ErrExist) {
			err = errors.New("目标文件夹已存在")
		}
		task.updates <- transferDoneMsg{task: task, item: name, err: err}
	}
	return m.startTask(task, "新建文件夹", []string{name}, ctx)
}

// runRemotePasteOperation 顺序执行跨端复制任务。
func runRemotePaste(ctx context.Context, task *transferTask, client *ssh.SFTP, sourceLocation, targetLocation transferSide, sourceDir, targetDir string, entries []transferEntry) {
	item := ""
	var err error
	defer func() {
		task.updates <- transferDoneMsg{task: task, item: item, cancelled: errors.Is(err, context.Canceled), err: err}
	}()
	if len(entries) == 0 {
		err = errors.New("没有待处理项目")
		return
	}
	names, nameErr := planTransferNames(client, targetLocation, targetDir, entries)
	if nameErr != nil {
		err = nameErr
		return
	}
	var processed int64
	var total int64
	for _, entry := range entries {
		if sourceLocation == localSide {
			size, sizeErr := localSize(ctx, []string{filepath.Join(sourceDir, entry.name)}, true)
			if sizeErr != nil {
				err = sizeErr
				return
			}
			total += size
		} else {
			size, sizeErr := client.Size(ctx, path.Join(sourceDir, entry.name))
			if sizeErr != nil {
				err = sizeErr
				return
			}
			total += size
		}
	}
	for index, entry := range entries {
		item = entry.name
		task.updates <- transferProgressMsg{task: task, item: item, itemIndex: index + 1, totalItems: len(entries), processedBytes: processed, totalBytes: total}
		report := func(delta int64) {
			processed += delta
			task.updates <- transferProgressMsg{task: task, item: item, itemIndex: index + 1, totalItems: len(entries), processedBytes: processed, totalBytes: total}
		}
		source := path.Join(sourceDir, entry.name)
		target := path.Join(targetDir, names[index])
		if sourceLocation == remoteSide && targetLocation == remoteSide && entry.dir && remotePathContains(source, targetDir) {
			err = errors.New("不能复制目录到其自身或子目录")
			return
		}
		if sourceLocation == localSide && targetLocation == remoteSide {
			err = client.Upload(ctx, filepath.Join(sourceDir, entry.name), target, report)
		} else if sourceLocation == remoteSide && targetLocation == localSide {
			err = client.Download(ctx, source, filepath.Join(targetDir, names[index]), report)
		} else {
			err = client.Copy(ctx, source, target, report)
		}
		if err != nil {
			return
		}
	}
}

// planTransferTargets 扫描目标目录，为整批跨端任务分配互不冲突的顶层名称。
func planTransferNames(client *ssh.SFTP, targetLocation transferSide, targetDir string, entries []transferEntry) ([]string, error) {
	used := make(map[string]struct{}, len(entries))
	if targetLocation == remoteSide {
		infos, err := client.ReadDir(context.Background(), targetDir)
		if err != nil {
			return nil, err
		}
		for _, info := range infos {
			used[info.Name()] = struct{}{}
		}
	} else {
		infos, err := os.ReadDir(targetDir)
		if err != nil {
			return nil, err
		}
		for _, info := range infos {
			used[info.Name()] = struct{}{}
		}
	}
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = conflictName(entry.name, entry.dir, used)
		used[names[i]] = struct{}{}
	}
	return names, nil
}

func remotePathContains(source, target string) bool {
	source, target = path.Clean(source), path.Clean(target)
	return target == source || strings.HasPrefix(target, source+"/")
}

// updateOperation 消费后台任务进度，并在任务结束后刷新当前本地目录。
func (m transferModel) handleTask(msg tea.Msg) (modalModel, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case transferProgressMsg:
		if msg.task != m.task {
			return m, nil, true
		}
		m.overlay.item = msg.item
		m.overlay.itemIndex = msg.itemIndex
		m.overlay.totalItems = msg.totalItems
		m.overlay.processedBytes = msg.processedBytes
		m.overlay.totalBytes = msg.totalBytes
		return m, func() tea.Msg { return <-msg.task.updates }, true
	case transferDoneMsg:
		if msg.task != m.task {
			return m, nil, true
		}
		msg.task.cancel()
		m.task = nil
		m.overlay.item = msg.item
		resultStatus := "操作已完成"
		if msg.cancelled {
			m.overlay.progress = progressCancelled
			resultStatus = "操作已取消"
		} else if msg.err != nil {
			m.overlay.progress = progressFailed
			m.overlay.error = fmt.Sprintf("%s：%v", msg.item, msg.err)
			resultStatus = "操作失败"
		} else {
			m.overlay.progress = progressCompleted
			m.overlay.processedBytes = m.overlay.totalBytes
			if msg.task.clearClipboard {
				m.clipboard = transferClipboard{}
			}
		}
		clear(m.selected)
		if msg.task.renameOld != "" && msg.task.renameNew != "" && msg.err == nil && m.clipboard.source == m.location && m.atClipboardSource() {
			for index := range m.clipboard.entries {
				if m.clipboard.entries[index].name == msg.task.renameOld {
					m.clipboard.entries[index].name = msg.task.renameNew
				}
			}
		}
		var refreshErr error
		var refreshCmd tea.Cmd
		if msg.task.remote {
			if m.location == remoteSide {
				refreshCmd = m.loadRemoteDir(m.remotePath, msg.task.focus, false)
			} else {
				refreshErr = m.refreshLocal()
			}
		} else {
			refreshErr = m.refreshLocal()
		}
		if refreshErr != nil {
			m.status = resultStatus + "，但刷新失败：" + refreshErr.Error()
		} else {
			m.status = resultStatus
			if msg.task.focus != "" && !msg.cancelled && msg.err == nil && !msg.task.remote {
				m.focusEntry(msg.task.focus)
			}
		}
		if !msg.cancelled && msg.err == nil && msg.task.renameOld == "" {
			m.overlay = transferOverlay{}
		}
		return m, refreshCmd, true
	default:
		return m, nil, false
	}
}

// reconcileLocalClipboard 移除已经被删除的本地来源项。
func (m *transferModel) pruneClipboard() {
	if m.clipboard.source != localSide || len(m.clipboard.entries) == 0 {
		return
	}
	entries := m.clipboard.entries[:0]
	for _, entry := range m.clipboard.entries {
		_, err := os.Lstat(filepath.Join(m.clipboard.path, entry.name))
		if err == nil || !errors.Is(err, os.ErrNotExist) {
			entries = append(entries, entry)
		}
	}
	m.clipboard.entries = entries
	if len(entries) == 0 {
		m.clipboard = transferClipboard{}
	}
}

// runLocalFileOperation 顺序执行一批本地复制或删除任务。
func runLocalTask(ctx context.Context, task *transferTask, operation localOp) {
	item := ""
	err := func() error {
		if len(operation.sources) == 0 {
			return errors.New("没有待处理项目")
		}
		item = filepath.Base(operation.sources[0])
		totalBytes, err := localSize(ctx, operation.sources, operation.action != 'd')
		if err != nil {
			return err
		}
		planned := []string(nil)
		if operation.action != 'd' {
			planned, err = planLocalTargets(operation.targetDir, operation.sources)
			if err != nil {
				return err
			}
		}
		processed := int64(0)
		// ponytail: 规划后若目标被并发占用则由排他创建安全失败；确有并发写入时再增加重试。
		for index, source := range operation.sources {
			if err := ctx.Err(); err != nil {
				return err
			}
			item = filepath.Base(source)
			report := func(delta int64) {
				processed += delta
				task.updates <- transferProgressMsg{task, item, index + 1, len(operation.sources), processed, totalBytes}
			}
			task.updates <- transferProgressMsg{task, item, index + 1, len(operation.sources), processed, totalBytes}
			if operation.action == 'd' {
				err = removeLocal(ctx, source, report)
			} else {
				info, statErr := os.Lstat(source)
				if statErr != nil {
					return statErr
				}
				target := filepath.Join(operation.targetDir, planned[index])
				if info.IsDir() && localContains(source, operation.targetDir) {
					return errors.New("不能复制目录到其自身或子目录")
				}
				err = copyLocal(ctx, source, target, report)
			}
			if err != nil {
				return err
			}
		}
		return nil
	}()
	task.updates <- transferDoneMsg{task: task, item: item, cancelled: errors.Is(err, context.Canceled), err: err}
}

// operationSize 预扫描任务总字节数，并在复制前拒绝不支持的项目类型。
func localSize(ctx context.Context, sources []string, rejectUnsupported bool) (int64, error) {
	var total int64
	for _, source := range sources {
		err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if rejectUnsupported && entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("符号链接 %s 不支持文件操作", entry.Name())
			}
			if entry.IsDir() {
				return nil
			}
			info, err := entry.Info()
			if err == nil && rejectUnsupported && !info.Mode().IsRegular() {
				return fmt.Errorf("不支持的文件类型：%s", entry.Name())
			}
			if err == nil {
				total += info.Size()
			}
			return err
		})
		if err != nil {
			return 0, err
		}
	}
	return total, nil
}

// planLocalTargets 一次扫描目标目录，为整批任务分配互不冲突的顶层名称。
func planLocalTargets(targetDir string, sources []string) ([]string, error) {
	items, err := os.ReadDir(targetDir)
	if err != nil {
		return nil, err
	}
	used := make(map[string]struct{}, len(items)+len(sources))
	for _, item := range items {
		used[item.Name()] = struct{}{}
	}
	names := make([]string, 0, len(sources))
	for _, source := range sources {
		info, err := os.Lstat(source)
		if err != nil {
			return nil, err
		}
		name := conflictName(filepath.Base(source), info.IsDir(), used)
		used[name] = struct{}{}
		names = append(names, name)
	}
	return names, nil
}

// renameLocalEntry 只接受单个文件名，并拒绝覆盖目录内已有项目。
func renameLocal(directory, oldName, newName string) error {
	if strings.TrimSpace(newName) == "" {
		return errors.New("名称不能为空")
	}
	if newName == "." || newName == ".." || filepath.Base(newName) != newName || strings.ContainsAny(newName, `/\`) {
		return errors.New("名称不能包含路径")
	}
	if newName == oldName {
		return nil
	}
	target := filepath.Join(directory, newName)
	if _, err := os.Lstat(target); err == nil {
		return errors.New("目标名称已存在")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(filepath.Join(directory, oldName), target)
}

func conflictName(name string, directory bool, used map[string]struct{}) string {
	if _, exists := used[name]; !exists {
		return name
	}
	extension := ""
	base := name
	if !directory {
		extension = filepath.Ext(name)
		base = strings.TrimSuffix(name, extension)
		if base == "" {
			base, extension = name, ""
		}
	}
	for index := 1; ; index++ {
		candidate := fmt.Sprintf("%s_%d%s", base, index, extension)
		if _, exists := used[candidate]; !exists {
			return candidate
		}
	}
}

// localPathContains 在解析现有符号链接后判断目标是否为源目录本身或其子目录。
func localContains(source, target string) bool {
	source = evalPath(source)
	target = evalPath(target)
	relative, err := filepath.Rel(source, target)
	return err == nil && (relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func evalPath(value string) string {
	if evaluated, err := filepath.EvalSymlinks(value); err == nil {
		return evaluated
	}
	absolute, _ := filepath.Abs(value)
	return absolute
}

// copyLocalEntry 递归复制普通文件或目录；失败时清理本次创建的不完整目标。
func copyLocal(ctx context.Context, source, target string, report func(int64)) (err error) {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("符号链接不支持复制")
	}
	if !info.IsDir() {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("不支持的文件类型：%s", filepath.Base(source))
		}
		return copyFile(ctx, source, target, info.Mode(), report)
	}
	if err := os.Mkdir(target, info.Mode().Perm()); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(target)
		}
	}()
	err = filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("符号链接 %s 不支持复制", entry.Name())
		}
		if path == source {
			return nil
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.Mkdir(destination, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("不支持的文件类型：%s", entry.Name())
		}
		return copyFile(ctx, path, destination, info.Mode(), report)
	})
	return err
}

// copyLocalFile 使用排他创建防止覆盖，并在每个数据块之间检查取消信号。
func copyFile(ctx context.Context, source, target string, mode os.FileMode, report func(int64)) (err error) {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode.Perm())
	if err != nil {
		return err
	}
	defer func() {
		closeErr := output.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(target)
		}
	}()
	return ssh.CopyData(ctx, output, input, report)
}

// removeLocalEntry 按子项到父目录的顺序删除，且不会跟随符号链接。
func removeLocal(ctx context.Context, source string, report func(int64)) error {
	// removable 保存逆序删除所需的路径和可计入进度的文件大小。
	type removable struct {
		path string
		size int64
	}
	var entries []removable
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("符号链接 %s 不支持删除", entry.Name())
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		size := info.Size()
		if entry.IsDir() {
			size = 0
		}
		entries = append(entries, removable{path: path, size: size})
		return nil
	})
	if err != nil {
		return err
	}
	for index := len(entries) - 1; index >= 0; index-- {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := os.Remove(entries[index].path); err != nil {
			return err
		}
		if entries[index].size > 0 {
			report(entries[index].size)
		}
	}
	return nil
}

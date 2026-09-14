package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"sshm/internal/repository"
)

func newTestTransfer(t *testing.T, connection connectionRow) transferModel {
	t.Helper()
	directory := t.TempDir()
	for _, name := range []string{"downloads", "projects", "配置文件"} {
		if err := os.Mkdir(filepath.Join(directory, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"README.md", "server.log", "部署说明.txt"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return newTransfer(connection, directory, 100, 30)
}

func TestResolveRemoteDirectory(t *testing.T) {
	tests := map[string]string{
		"logs":               "/srv/app/logs",
		"../shared":          "/srv/shared",
		"/var/lib/app":       "/var/lib/app",
		"~":                  "/home/tester",
		"~/projects/service": "/home/tester/projects/service",
	}
	for target, want := range tests {
		if got := resolveRemoteDir("/srv/app", "/home/tester", target); got != want {
			t.Fatalf("远程路径 %q 解析为 %q，want %q", target, got, want)
		}
	}
}

func TestFileTransferAppliesRemoteDirectoryResult(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	m.location = remoteSide
	m.remotePath = "/home/tester"
	m.cancelRead = func() {}
	m.focus = addressFocus
	m.address.SetValue("projects")
	m.selected["old"] = struct{}{}

	updated, _ := m.Update(remoteDirMsg{
		path: "/home/tester/projects", address: true,
		entries: []transferEntry{{name: "src", dir: true}, {name: "README.md", size: 12}},
	})
	m = updated.(transferModel)
	if m.cancelRead != nil || m.remotePath != "/home/tester/projects" || m.address.Value() != m.remotePath {
		t.Fatalf("远程目录结果未应用: cancel=%v path=%q address=%q", m.cancelRead, m.remotePath, m.address.Value())
	}
	if m.focus != listFocus || len(m.selected) != 0 || len(m.remoteEntries) != 2 {
		t.Fatalf("远程跳转后状态错误: focus=%d selected=%v entries=%v", m.focus, m.selected, m.remoteEntries)
	}

	m.cancelRead = func() {}
	m.focus = addressFocus
	m.address.SetValue("missing")
	updated, _ = m.Update(remoteDirMsg{path: "/missing", address: true, err: errors.New("不存在")})
	m = updated.(transferModel)
	if m.cancelRead != nil || m.remotePath != "/home/tester/projects" || m.focus != addressFocus || m.address.Value() != "missing" || !strings.Contains(m.status, "跳转失败") {
		t.Fatalf("远程跳转失败未保留输入状态: cancel=%v path=%q focus=%d address=%q status=%q", m.cancelRead, m.remotePath, m.focus, m.address.Value(), m.status)
	}

	m.location = localSide
	m.selected["downloads"] = struct{}{}
	m.cancelRead = func() {}
	updated, _ = m.Update(remoteDirMsg{path: "/home/tester", entries: []transferEntry{{name: "remote"}}, focus: "projects"})
	m = updated.(transferModel)
	if _, ok := m.selected["downloads"]; !ok || m.remotePath != "/home/tester" || len(m.remoteEntries) != 1 {
		t.Fatalf("后台远程读取结果污染了本地选择或未保存远程结果: selected=%v path=%q entries=%v", m.selected, m.remotePath, m.remoteEntries)
	}
}

func TestFileTransferCancelsRemoteConnection(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelConnect = cancel
	m.overlay = transferOverlay{kind: overlayProgress, progress: progressConnecting}
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	m = updated.(transferModel)
	if !m.overlay.cancelRequested || ctx.Err() != context.Canceled {
		t.Fatalf("远程连接取消未生效: requested=%v err=%v", m.overlay.cancelRequested, ctx.Err())
	}
	updated, _ = m.Update(sftpReadyMsg{err: context.Canceled})
	m = updated.(transferModel)
	if m.overlay.progress != progressCancelled || m.cancelConnect != nil {
		t.Fatalf("远程连接取消结果错误: progress=%d cancel=%v", m.overlay.progress, m.cancelConnect)
	}
}

func finishTransferTask(t *testing.T, m transferModel, cmd tea.Cmd) transferModel {
	t.Helper()
	for cmd != nil {
		updated, next := m.Update(cmd())
		m = updated.(transferModel)
		cmd = next
	}
	return m
}

func TestFileTransferOpensFromMenu(t *testing.T) {
	m := model{
		width:       100,
		height:      30,
		connections: []connectionRow{{name: "测试服务器", host: "192.168.0.1"}},
		searchInput: newSearchInput(),
		searchReady: true,
	}
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = updated.(model)
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m = updated.(model)
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd != nil {
		t.Fatal("打开文件传输弹窗时不应立即连接远程")
	}
	transfer, ok := updated.(model).modal.(transferModel)
	if !ok {
		t.Fatalf("文件传输菜单未打开弹窗: %#v", updated.(model).modal)
	}
	if transfer.localPath == "" || transfer.width != 100 || transfer.height != 30 {
		t.Fatalf("文件传输初始化状态错误: path=%q size=%d×%d", transfer.localPath, transfer.width, transfer.height)
	}
}

func TestFileTransferConnectsOnFirstRemoteSwitch(t *testing.T) {
	m := newTestTransfer(t, connectionRow{id: 1, name: "测试服务器"})
	m.store = &repository.Store{}
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: "2"}))
	m = updated.(transferModel)
	defer m.closeSFTP()
	if cmd == nil || m.location != remoteSide || m.cancelConnect == nil || m.overlay.progress != progressConnecting {
		t.Fatalf("首次切换远程未启动连接: cmd=%v location=%d cancel=%v progress=%d", cmd, m.location, m.cancelConnect, m.overlay.progress)
	}
	if duplicate := m.switchLocation(remoteSide); duplicate != nil {
		t.Fatal("远程连接进行中重复启动了连接")
	}
}

func TestFileTransferSwitchRestoresEachCursor(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	m.remoteEntries = []transferEntry{{name: "alpha"}, {name: "beta"}, {name: "gamma"}}
	m.cursor = 4
	m.switchLocation(remoteSide)
	m.cursor = 2
	m.switchLocation(localSide)
	if m.cursor != 4 {
		t.Fatalf("返回本地后光标 = %d，want 4", m.cursor)
	}
	m.switchLocation(remoteSide)
	if m.cursor != 2 {
		t.Fatalf("返回远程后光标 = %d，want 2", m.cursor)
	}
}

func TestFileTransferViewLayout(t *testing.T) {
	m := newTestTransfer(t, connectionRow{name: "测试服务器", host: "192.168.0.1"})
	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 22 {
		t.Fatalf("弹窗高度 = %d, want 22", len(lines))
	}
	for i, line := range lines {
		if width := ansi.StringWidth(line); width != 80 {
			t.Fatalf("第 %d 行宽度 = %d, want 80", i+1, width)
		}
	}
	plain := ansi.Strip(view)
	for _, content := range []string{"本地", "🔍", "筛选文件...", "📁", "📄", "1/2: 切换", "y/p: 复制/粘贴", "q/Esc: 关闭", "?: 帮助", "已读取"} {
		if !strings.Contains(plain, content) {
			t.Fatalf("文件传输弹窗缺少 %q: %q", content, plain)
		}
	}
	if strings.Contains(plain, "远程") || strings.Contains(plain, "地址：") {
		t.Fatalf("顶部不应同时显示两个位置标签或地址标签: %q", plain)
	}
	if !strings.Contains(lines[3], "🔍") || !strings.Contains(lines[4], "📁") {
		t.Fatalf("筛选框和文件列表之间存在间隔: %q", lines[3:5])
	}
	if localTagStyle.GetBackground() == remoteTagStyle.GetBackground() {
		t.Fatal("本地和远程标签使用了相同背景色")
	}
	footer := ansi.Strip(lines[len(lines)-2])
	if strings.Contains(footer, "n/s/d") || strings.Contains(footer, "Ctrl+G") || strings.Contains(footer, "Ctrl+D") || strings.Contains(footer, "r: 重命名") {
		t.Fatalf("底部显示了应移入帮助弹窗的快捷键: %q", footer)
	}
	styles := m.search.Styles()
	if styles.Focused.Placeholder.GetForeground() != styles.Blurred.Placeholder.GetForeground() {
		t.Fatal("筛选占位文字在获得焦点后改变了颜色")
	}
}

func TestFileTransferViewFitsTinyTerminal(t *testing.T) {
	for _, size := range [][2]int{{1, 1}, {9, 7}, {20, 8}} {
		m := newTestTransfer(t, connectionRow{})
		m.width, m.height = size[0], size[1]
		wantWidth, wantHeight := transferSize(size[0], size[1])
		lines := strings.Split(m.View(), "\n")
		if len(lines) != wantHeight {
			t.Fatalf("尺寸 %dx%d 渲染行数 = %d，want %d", size[0], size[1], len(lines), wantHeight)
		}
		for _, line := range lines {
			if got := ansi.StringWidth(line); got != wantWidth {
				t.Fatalf("尺寸 %dx%d 行宽 = %d，want %d", size[0], size[1], got, wantWidth)
			}
		}
	}
}

func TestFileTransferSelectionAddressAndPasteDirection(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeySpace}))
	m = updated.(transferModel)
	if _, ok := m.selected["downloads"]; !ok {
		t.Fatal("Space 未选中当前项目")
	}
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "y"}))
	m = updated.(transferModel)
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "2"}))
	m = updated.(transferModel)
	if len(m.selected) != 0 {
		t.Fatal("切换位置后未清空当前路径的多选")
	}
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "p"}))
	m = updated.(transferModel)
	if m.status != "远程连接不可用" {
		t.Fatalf("跨端粘贴未提示连接不可用: %q", m.status)
	}

	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'g', Mod: tea.ModCtrl}))
	m = updated.(transferModel)
	if m.focus != addressFocus || m.address.Value() != m.remotePath {
		t.Fatalf("Ctrl+G 未聚焦当前地址: focus=%d value=%q", m.focus, m.address.Value())
	}
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	m = updated.(transferModel)
	if m.address.Value() != "" || m.focus != addressFocus {
		t.Fatalf("Ctrl+C 未清空地址或错误移除焦点: value=%q focus=%d", m.address.Value(), m.focus)
	}
}

func TestFileTransferSortKeepsDirectoriesFirst(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	for _, field := range []byte{'n', 's', 't'} {
		m.sortField = field
		for _, asc := range []bool{true, false} {
			m.sortAsc = asc
			entries := m.visibleEntries()
			seenFile := false
			for _, entry := range entries {
				if !entry.dir {
					seenFile = true
				} else if seenFile {
					t.Fatalf("排序 %c asc=%v 时文件夹出现在文件之后: %#v", field, asc, entries)
				}
			}
		}
	}
}

func TestFileTransferSortKeepsFocusedEntry(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	m.focusEntry("server.log")
	m.toggleSort('s')
	entry, ok := m.currentEntry()
	if !ok || entry.name != "server.log" {
		t.Fatalf("排序后焦点跳到其他项目: cursor=%d entry=%#v", m.cursor, entry)
	}
}

func TestFileTransferDetailsUseSectionSpacing(t *testing.T) {
	m := newTestTransfer(t, connectionRow{name: "测试服务器", host: "192.168.0.1"})
	rows := m.renderDetails(25, 16)
	for _, row := range []int{0, 4, 7} {
		if strings.TrimSpace(ansi.Strip(rows[row])) != "" {
			t.Fatalf("右侧第 %d 行不是分组间隔行: %q", row+1, rows[row])
		}
	}
	view := ansi.Strip(strings.Join(rows, "\n"))
	if strings.Contains(view, "主机") || strings.Contains(view, "排序") || !strings.Contains(view, "剪贴板") {
		t.Fatalf("右侧不应显示主机和排序区块: %q", view)
	}
	section := ansi.Strip(centeredSection("剪贴板", 25))
	if ansi.StringWidth(section) != 25 || !strings.Contains(section, "─ 剪贴板 ─") {
		t.Fatalf("剪贴板标题未使用左右横线: %q", section)
	}
	m.clipboard.entries = []transferEntry{{name: "server.log"}}
	view = ansi.Strip(strings.Join(m.renderDetails(25, 16), "\n"))
	if !strings.Contains(view, "server.log") || strings.Contains(view, "复制 ·") {
		t.Fatalf("剪贴板未直接显示文件名列表: %q", view)
	}
}

func TestFileTransferSeparatorsAndSelectionBackground(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	m.selected["downloads"] = struct{}{}
	lines := strings.Split(m.View(), "\n")
	if plain := ansi.Strip(lines[2]); !strings.Contains(plain, "┤ 文件名 ↑ ├") || !strings.HasPrefix(plain, "├") || !strings.HasSuffix(plain, "┤") {
		t.Fatalf("排序分隔线格式错误: %q", plain)
	}
	if !strings.Contains(lines[2], "38;2;216;222;233") {
		t.Fatalf("排序标题未使用正文色: %q", lines[2])
	}
	plain := ansi.Strip(lines[2])
	boxStart := strings.Index(plain, "┤ 文件名 ↑ ├")
	if boxStart < 0 || ansi.StringWidth(plain[:boxStart]) != 1+52-6-lipgloss.Width("┤ 文件名 ↑ ├") {
		t.Fatalf("排序标题未向左偏移六个单元: start=%d line=%q", ansi.StringWidth(plain[:max(0, boxStart)]), plain)
	}
	if plain := ansi.Strip(lines[len(lines)-3]); plain != "│ "+strings.Repeat("─", 76)+" │" {
		t.Fatalf("底部分隔线未保留两端缺口: %q", plain)
	}
	list := m.renderList(51, 10)
	if !strings.Contains(list[1], "48;2;") || strings.Contains(list[2], "48;2;") {
		t.Fatalf("多选背景未限制在单行: selected=%q next=%q", list[1], list[2])
	}
	iconAt := strings.Index(list[1], "📁")
	if iconAt < 0 || strings.LastIndex(list[1][:iconAt], "\x1b[m") < strings.LastIndex(list[1][:iconAt], "48;2;") {
		t.Fatalf("多选背景未在光标列结束: %q", list[1])
	}
	if !strings.Contains(list[1], "38;2;0;179;228;48;2;") || !strings.HasPrefix(ansi.Strip(list[1]), "> 📁") {
		t.Fatalf("光标经过多选行时改变了 > 的颜色: %q", list[1])
	}
}

func TestFileTransferClipboardColorDoesNotRestoreSelection(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	m.selected["downloads"] = struct{}{}
	defaultLine := m.renderList(51, 10)[1]
	m.copySelection()
	copyLine := m.renderList(51, 10)[1]
	if defaultLine == copyLine {
		t.Fatal("普通多选和复制未使用不同的图标背景色")
	}

	m.enterDirectory()
	if len(m.selected) != 0 {
		t.Fatal("离开剪贴板来源目录后仍保留多选")
	}
	m.goParent()
	if len(m.selected) != 0 || !m.isClipboardEntry("downloads") {
		t.Fatalf("返回剪贴板来源目录后剪贴板标记污染了多选: selected=%v", m.selected)
	}
	if line := m.renderList(51, 10)[1]; line != copyLine {
		t.Fatalf("剪贴板标记未恢复复制颜色: got=%q want=%q", line, copyLine)
	}
}

func TestFileTransferClipboardSelectsCurrentEntryByDefault(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	m.copySelection()
	if _, ok := m.selected["downloads"]; !ok || !m.isClipboardEntry("downloads") {
		t.Fatalf("无多选时未选中当前项: selected=%v", m.selected)
	}
	if !strings.Contains(m.renderList(51, 10)[1], "48;2;") {
		t.Fatal("无多选时未显示当前项背景标记")
	}
}

func TestFileTransferOperationsUseBatchOnlyWhenFocusIsSelected(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	m.selected["projects"] = struct{}{}
	m.copySelection()
	if len(m.clipboard.entries) != 1 || m.clipboard.entries[0].name != "downloads" || len(m.selected) != 0 {
		t.Fatalf("复制未回退到焦点项: clipboard=%v selected=%v", m.clipboard.entries, m.selected)
	}

	m = newTestTransfer(t, connectionRow{})
	m.selected["downloads"] = struct{}{}
	m.selected["projects"] = struct{}{}
	m.focusEntry("projects")
	m.copySelection()
	if len(m.clipboard.entries) != 2 || len(m.selected) != 2 {
		t.Fatalf("焦点位于选择项时未批量复制: clipboard=%v selected=%v", m.clipboard.entries, m.selected)
	}

	m = newTestTransfer(t, connectionRow{})
	m.selected["projects"] = struct{}{}
	m.openDelete()
	if len(m.overlay.deletePaths) != 1 || m.overlay.deletePaths[0] != filepath.Join(m.localPath, "downloads") || len(m.selected) != 0 {
		t.Fatalf("删除未回退到焦点项: overlay=%#v selected=%v", m.overlay, m.selected)
	}

	m = newTestTransfer(t, connectionRow{})
	m.selected["downloads"] = struct{}{}
	m.selected["projects"] = struct{}{}
	m.focusEntry("projects")
	m.openDelete()
	if len(m.overlay.deletePaths) != 2 || len(m.selected) != 2 {
		t.Fatalf("焦点位于选择项时未批量删除: overlay=%#v selected=%v", m.overlay, m.selected)
	}
}

func TestFileTransferCutShortcutIsUnavailable(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: "x"}))
	m = updated.(transferModel)
	if cmd != nil || len(m.clipboard.entries) != 0 || len(m.selected) != 0 {
		t.Fatalf("剪切快捷键仍可用: cmd=%v clipboard=%v selected=%v", cmd, m.clipboard.entries, m.selected)
	}
}

func TestFileTransferHelpListsAllShortcuts(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: "?"}))
	m = updated.(transferModel)
	if cmd != nil || m.overlay.kind != overlayHelp {
		t.Fatalf("? 未打开帮助弹窗: overlay=%d cmd=%v", m.overlay.kind, cmd)
	}
	view := ansi.Strip(m.View())
	for _, shortcut := range []string{"1/2", "↑/↓、j/k", "h/Backspace", "l/Enter", "Space", "y/p", "/ 筛选", "Ctrl+G", "n/s/t", "d 删除", "r 重命名", "a 新建文件夹", "Ctrl+C", "Esc"} {
		if !strings.Contains(view, shortcut) {
			t.Fatalf("帮助弹窗缺少 %q: %q", shortcut, view)
		}
	}
	styled := m.renderOverlay()
	if !strings.Contains(styled, plainStyle.Render("1/2")) || !strings.Contains(styled, mutedStyle.Render(" 切换本地/远程")) {
		t.Fatalf("帮助快捷键和说明未使用不同颜色: %q", styled)
	}
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if updated.(transferModel).overlay.kind != overlayNone {
		t.Fatal("Esc 未关闭帮助弹窗")
	}
}

func TestFileTransferShortcutMapping(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: "d"}))
	m = updated.(transferModel)
	if cmd != nil || m.overlay.kind != overlayDelete {
		t.Fatalf("d 未打开删除确认框: overlay=%d cmd=%v", m.overlay.kind, cmd)
	}

	m = newTestTransfer(t, connectionRow{})
	updated, cmd = m.Update(tea.KeyPressMsg(tea.Key{Text: "t"}))
	m = updated.(transferModel)
	if cmd != nil || m.sortField != 't' {
		t.Fatalf("t 未切换到时间排序: sortField=%q cmd=%v", m.sortField, cmd)
	}
}

func TestFileTransferDeleteOverlayIsolatesInputAndRestoresState(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	m.selected["downloads"] = struct{}{}
	m.selected["projects"] = struct{}{}

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "d"}))
	m = updated.(transferModel)
	if m.overlay.kind != overlayDelete || !strings.Contains(ansi.Strip(m.View()), "确定要删除以下项目：") || len(m.overlay.deletePaths) != 2 {
		t.Fatalf("d 未打开多选删除确认框: %#v", m.overlay)
	}

	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	m = updated.(transferModel)
	if m.cursor != 0 || m.overlay.kind != overlayDelete {
		t.Fatalf("确认框未隔离底层按键: cursor=%d overlay=%d", m.cursor, m.overlay.kind)
	}
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = updated.(transferModel)
	if m.overlay.kind != overlayDelete {
		t.Fatal("Enter 不应确认删除")
	}
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "n"}))
	m = updated.(transferModel)
	if m.overlay.kind != overlayNone || len(m.selected) != 2 {
		t.Fatalf("取消删除未恢复原状态: overlay=%d selected=%v", m.overlay.kind, m.selected)
	}
}

func TestFileTransferDeleteOverlayRunsLocalTask(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	deletedPath := filepath.Join(m.localPath, "downloads")
	m.clipboard = transferClipboard{source: localSide, path: m.localPath, entries: []transferEntry{{name: "downloads", dir: true}}}
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "d"}))
	m = updated.(transferModel)
	if !strings.Contains(ansi.Strip(m.View()), "downloads") {
		t.Fatal("单项删除确认框未显示当前名称")
	}
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: "y"}))
	m = updated.(transferModel)
	if cmd == nil || m.overlay.kind != overlayProgress {
		t.Fatal("确认删除未启动后台任务")
	}
	m = finishTransferTask(t, m, cmd)
	if _, err := os.Stat(deletedPath); !os.IsNotExist(err) || m.overlay.kind != overlayNone || len(m.clipboard.entries) != 0 {
		t.Fatalf("删除完成后状态错误: stat=%v overlay=%d clipboard=%v", err, m.overlay.kind, m.clipboard)
	}
}

func TestFileTransferDeleteKeepsClipboardFromOtherPath(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	m.clipboard = transferClipboard{source: localSide, path: filepath.Join(m.localPath, "other"), entries: []transferEntry{{name: "item"}}}
	updated, cmd := m.startDelete()
	if cmd == nil || updated.(transferModel).task.clearClipboard {
		t.Fatalf("删除其他路径时错误标记清空剪贴板: model=%#v", updated)
	}
}

func TestTransferProgressOverlayLifecycle(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	m.copySelection()
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: "p"}))
	m = updated.(transferModel)
	if cmd == nil || m.overlay.kind != overlayProgress || !strings.Contains(ansi.Strip(m.View()), "复制中") {
		t.Fatalf("粘贴未打开进度框: %#v", m.overlay)
	}

	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	m = updated.(transferModel)
	if m.overlay.progress != progressRunning {
		t.Fatal("运行中的进度框不应响应 Esc")
	}
	m = finishTransferTask(t, m, cmd)
	if m.overlay.kind != overlayNone {
		t.Fatal("复制完成后进度弹窗未自动关闭")
	}
}

func TestFileTransferRenameInput(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	m.selected["downloads"] = struct{}{}
	m.selected["projects"] = struct{}{}
	for index, entry := range m.visibleEntries() {
		if entry.name == "server.log" {
			m.cursor = index
		}
	}
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "r"}))
	m = updated.(transferModel)
	if m.overlay.kind != overlayRename || m.overlay.renameInput.Value() != "server.log" {
		t.Fatalf("重命名框未预填当前名称: %#v", m.overlay)
	}
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	m = updated.(transferModel)
	if m.overlay.renameInput.Value() != "" {
		t.Fatal("Ctrl+C 未清空重命名输入")
	}
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	m = updated.(transferModel)
	if m.overlay.kind != overlayNone {
		t.Fatal("Esc 未取消重命名")
	}
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "r"}))
	m = updated.(transferModel)
	m.overlay.renameInput.SetValue("application.log")
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = updated.(transferModel)
	if _, err := os.Stat(filepath.Join(m.localPath, "application.log")); err != nil || m.overlay.kind != overlayNone {
		t.Fatalf("重命名未完成: %v", err)
	}
	entry, ok := m.currentEntry()
	if !ok || entry.name != "application.log" {
		t.Fatalf("重命名后焦点未保持在原项目: cursor=%d entry=%#v", m.cursor, entry)
	}
	if len(m.selected) != 0 {
		t.Fatalf("重命名完成后未清空多选: %v", m.selected)
	}
}

func TestTransferCreateFolder(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: "a"}))
	m = updated.(transferModel)
	if cmd == nil || m.overlay.kind != overlayCreate {
		t.Fatalf("新建文件夹弹窗未打开: overlay=%d cmd=%v", m.overlay.kind, cmd)
	}
	m.overlay.renameInput.SetValue("../escape")
	updated, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = updated.(transferModel)
	if cmd != nil || m.overlay.kind != overlayCreate || m.overlay.error == "" {
		t.Fatalf("新建文件夹错误名称未拒绝: overlay=%d cmd=%v error=%q", m.overlay.kind, cmd, m.overlay.error)
	}
	m.overlay.renameInput.SetValue("archives")
	updated, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = updated.(transferModel)
	if cmd == nil || m.overlay.kind != overlayProgress {
		t.Fatalf("新建文件夹未启动任务: overlay=%d cmd=%v", m.overlay.kind, cmd)
	}
	m = finishTransferTask(t, m, cmd)
	if _, err := os.Stat(filepath.Join(m.localPath, "archives")); err != nil || m.overlay.kind != overlayNone {
		t.Fatalf("新建文件夹失败: err=%v overlay=%d", err, m.overlay.kind)
	}
	entry, ok := m.currentEntry()
	if !ok || entry.name != "archives" {
		t.Fatalf("新建后未定位到文件夹: cursor=%d entry=%#v", m.cursor, entry)
	}

	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "a"}))
	m = updated.(transferModel)
	m.overlay.renameInput.SetValue("archives")
	updated, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = updated.(transferModel)
	if cmd == nil {
		t.Fatal("重名创建文件夹未进入任务")
	}
	m = finishTransferTask(t, m, cmd)
	if m.overlay.progress != progressFailed || !strings.Contains(m.overlay.error, "已存在") {
		t.Fatalf("重名文件夹未报错: state=%d error=%q", m.overlay.progress, m.overlay.error)
	}
}

func TestFileTransferGoParentFocusesChildDirectory(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	parent := m.localPath
	if err := m.changeLocalDir(filepath.Join(parent, "projects")); err != nil {
		t.Fatal(err)
	}
	m.search.SetValue("不会匹配")
	m.goParent()
	entry, ok := m.currentEntry()
	if !ok || m.localPath != parent || entry.name != "projects" || m.search.Value() != "" {
		t.Fatalf("返回上级后未聚焦原目录: path=%q cursor=%d entry=%#v filter=%q", m.localPath, m.cursor, entry, m.search.Value())
	}
}

func TestTransferProgressRendersEveryPrototypeState(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	m.overlay = transferOverlay{kind: overlayProgress, operation: "复制", item: "server.log", error: "server.log：模拟错误"}
	tests := []struct {
		state progressState
		want  string
	}{
		{progressConnecting, "连接中"},
		{progressRunning, "复制中"},
		{progressCompleted, "已完成"},
		{progressCancelled, "已取消"},
		{progressFailed, "server.log：模拟错误"},
	}
	for _, test := range tests {
		m.overlay.progress = test.state
		if view := ansi.Strip(m.View()); !strings.Contains(view, test.want) {
			t.Fatalf("进度状态 %d 缺少 %q: %q", test.state, test.want, view)
		}
	}
}

func TestRemotePathContainsOnlyRejectsSelfOrDescendant(t *testing.T) {
	for _, test := range []struct {
		source, target string
		want           bool
	}{
		{"/home/data", "/home/data", true},
		{"/home/data", "/home/data/sub", true},
		{"/home/data", "/home/database", false},
		{"/home/data", "/home/other", false},
	} {
		if got := remotePathContains(test.source, test.target); got != test.want {
			t.Fatalf("remotePathContains(%q, %q) = %v, want %v", test.source, test.target, got, test.want)
		}
	}
}

func TestFileTransferReadsAndJumpsLocalDirectories(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	m.selected["downloads"] = struct{}{}
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "/"}))
	m = updated.(transferModel)
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "r"}))
	m = updated.(transferModel)
	if len(m.selected) != 0 {
		t.Fatalf("筛选变化后保留了可能不可见的多选: %v", m.selected)
	}
	m.focus = listFocus
	m.search.Reset()
	m.search.SetValue("readme")
	entries := m.visibleEntries()
	if len(entries) != 1 || entries[0].name != "README.md" {
		t.Fatalf("本地筛选结果错误: %#v", entries)
	}
	m.search.Reset()
	m.focus = addressFocus
	m.address.SetValue("downloads")
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = updated.(transferModel)
	if filepath.Base(m.localPath) != "downloads" || len(m.localEntries) != 0 || m.focus != listFocus {
		t.Fatalf("相对路径跳转失败: path=%q entries=%v focus=%d", m.localPath, m.localEntries, m.focus)
	}
	m.focus = addressFocus
	m.address.SetValue("missing")
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = updated.(transferModel)
	if m.focus != addressFocus || m.address.Value() != "missing" || !strings.Contains(m.status, "跳转失败") {
		t.Fatalf("无效路径未保留输入: focus=%d value=%q status=%q", m.focus, m.address.Value(), m.status)
	}
}

func TestFileTransferListFocusClearsFilter(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	m.search.SetValue("readme")
	m.selected["README.md"] = struct{}{}
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	m, ok := updated.(transferModel)
	if !ok || cmd != nil || m.search.Value() != "" || m.cursor != 0 || len(m.selected) != 0 {
		t.Fatalf("列表焦点下 Ctrl+C 未取消过滤: model=%#v cmd=%v", updated, cmd)
	}
}

func TestFileTransferSearchEscClearsAndBlurs(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	m.focus = searchFocus
	m.search.Focus()
	m.search.SetValue("readme")
	m.cursor = 1
	m.selected["README.md"] = struct{}{}

	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	m = updated.(transferModel)
	if cmd != nil || m.search.Value() != "" || m.search.Focused() || m.focus != listFocus || m.cursor != 0 || len(m.selected) != 0 {
		t.Fatalf("筛选焦点下 Esc 状态错误: focus=%d value=%q focused=%v cursor=%d selected=%v cmd=%v", m.focus, m.search.Value(), m.search.Focused(), m.cursor, m.selected, cmd)
	}
}

func TestFileTransferEscClosesEvenWithFilter(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	m.search.SetValue("readme")
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if updated != nil {
		t.Fatalf("存在过滤值时 Esc 未关闭弹窗: %#v", updated)
	}
}

func TestFileTransferQClosesFromList(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "q"}))
	if updated != nil {
		t.Fatalf("列表焦点下 q 未关闭弹窗: %#v", updated)
	}

	m = newTestTransfer(t, connectionRow{})
	m.focus = searchFocus
	m.search.Focus()
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "q"}))
	if updated == nil || updated.(transferModel).search.Value() != "q" {
		t.Fatalf("筛选焦点下 q 错误关闭弹窗: %#v", updated)
	}
}

func TestFileTransferPrunesMissingClipboardBeforePaste(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	m.copySelection()
	if err := os.RemoveAll(filepath.Join(m.localPath, "downloads")); err != nil {
		t.Fatal(err)
	}
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: "p"}))
	m = updated.(transferModel)
	if cmd != nil || len(m.clipboard.entries) != 0 || m.status != "剪贴板为空" {
		t.Fatalf("粘贴前未清理失效剪贴板: cmd=%v clipboard=%v status=%q", cmd, m.clipboard.entries, m.status)
	}
}

func TestLocalCopyUsesConflictNames(t *testing.T) {
	root := t.TempDir()
	sourceDir, targetDir := filepath.Join(root, "source"), filepath.Join(root, "target")
	for _, directory := range []string{sourceDir, targetDir, filepath.Join(sourceDir, "data"), filepath.Join(targetDir, "data")} {
		if err := os.Mkdir(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "report.txt"), []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "data", "nested.txt"), []byte("nested"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"report.txt", "report_1.txt"} {
		if err := os.WriteFile(filepath.Join(targetDir, name), []byte("existing"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	m := newTransfer(connectionRow{}, sourceDir, 100, 30)
	m.selected["report.txt"] = struct{}{}
	m.selected["data"] = struct{}{}
	m.copySelection()
	if err := m.changeLocalDir(targetDir); err != nil {
		t.Fatal(err)
	}
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: "p"}))
	m = finishTransferTask(t, updated.(transferModel), cmd)
	content, err := os.ReadFile(filepath.Join(targetDir, "report_2.txt"))
	if err != nil || string(content) != "source" {
		t.Fatalf("冲突文件复制失败: content=%q err=%v", content, err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "data_1", "nested.txt")); err != nil {
		t.Fatalf("冲突目录未整体改名复制: %v", err)
	}
	if len(m.selected) != 0 || len(m.clipboard.entries) != 0 {
		t.Fatalf("粘贴完成后选择或剪贴板状态错误: selected=%v clipboard=%v", m.selected, m.clipboard.entries)
	}
}

func TestRenameAfterPasteKeepsClipboardSeparateFromDeleteSelection(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "temp"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := newTransfer(connectionRow{}, root, 100, 30)
	m.copySelection()
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: "p"}))
	m = finishTransferTask(t, updated.(transferModel), cmd)
	if len(m.selected) != 0 || len(m.clipboard.entries) != 0 {
		t.Fatalf("粘贴后未清空剪贴板或污染多选: selected=%v clipboard=%v", m.selected, m.clipboard.entries)
	}
	m.focusEntry("temp_1")
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "r"}))
	m = updated.(transferModel)
	m.overlay.renameInput.SetValue("temp_2")
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = updated.(transferModel)
	entry, ok := m.currentEntry()
	if !ok || entry.name != "temp_2" || len(m.selected) != 0 || len(m.clipboard.entries) != 0 {
		t.Fatalf("重命名后状态冲突: entry=%#v selected=%v clipboard=%v", entry, m.selected, m.clipboard.entries)
	}
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "d"}))
	m = updated.(transferModel)
	if len(m.overlay.deletePaths) != 1 || m.overlay.deletePaths[0] != filepath.Join(m.localPath, "temp_2") {
		t.Fatalf("删除错误选择了剪贴板来源项: %#v", m.overlay)
	}
}

func TestRenamingClipboardSourceUpdatesClipboardReference(t *testing.T) {
	m := newTestTransfer(t, connectionRow{})
	m.copySelection()
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "r"}))
	m = updated.(transferModel)
	m.overlay.renameInput.SetValue("archives")
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = updated.(transferModel)
	if len(m.selected) != 0 || len(m.clipboard.entries) != 1 || m.clipboard.entries[0].name != "archives" || !m.isClipboardEntry("archives") {
		t.Fatalf("重命名来源项后剪贴板未同步: selected=%v clipboard=%v", m.selected, m.clipboard.entries)
	}
}

func TestLocalCopyCancellationPreservesSource(t *testing.T) {
	root := t.TempDir()
	sourceDir, targetDir := filepath.Join(root, "source"), filepath.Join(root, "target")
	if err := os.Mkdir(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(sourceDir, "large.bin")
	if err := os.WriteFile(source, make([]byte, 2*1024*1024), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newTransfer(connectionRow{}, sourceDir, 100, 30)
	m.copySelection()
	if err := m.changeLocalDir(targetDir); err != nil {
		t.Fatal(err)
	}
	updated, firstCmd := m.Update(tea.KeyPressMsg(tea.Key{Text: "p"}))
	m = updated.(transferModel)
	updated, waitCmd := m.Update(firstCmd())
	m = updated.(transferModel)
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	m = finishTransferTask(t, updated.(transferModel), waitCmd)
	if m.overlay.progress != progressCancelled {
		t.Fatalf("取消后状态错误: %d", m.overlay.progress)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("取消复制后源文件丢失: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "large.bin")); !os.IsNotExist(err) {
		t.Fatalf("取消复制后残留不完整目标: %v", err)
	}
}

func TestConflictNameAndRenameValidation(t *testing.T) {
	used := map[string]struct{}{"report.txt": {}, "report_1.txt": {}, "data": {}, ".env": {}}
	for _, test := range []struct {
		name string
		dir  bool
		want string
	}{{"report.txt", false, "report_2.txt"}, {"data", true, "data_1"}, {".env", false, ".env_1"}} {
		if got := conflictName(test.name, test.dir, used); got != test.want {
			t.Fatalf("冲突名称 %q = %q, want %q", test.name, got, test.want)
		}
	}
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "old"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "exists"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", "../escape", "exists"} {
		if err := renameLocal(directory, "old", name); err == nil {
			t.Fatalf("重命名错误地接受 %q", name)
		}
	}
}

func TestLocalDirectoryCannotCopyIntoItsChild(t *testing.T) {
	root := t.TempDir()
	tree, child := filepath.Join(root, "tree"), filepath.Join(root, "tree", "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, "source.txt"), []byte("safe"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newTransfer(connectionRow{}, root, 100, 30)
	m.selected["tree"] = struct{}{}
	m.copySelection()
	if err := m.changeLocalDir(child); err != nil {
		t.Fatal(err)
	}
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Text: "p"}))
	m = finishTransferTask(t, updated.(transferModel), cmd)
	if m.overlay.progress != progressFailed || !strings.Contains(m.overlay.error, "自身或子目录") {
		t.Fatalf("复制到自身子目录未被拒绝: state=%d error=%q", m.overlay.progress, m.overlay.error)
	}
	if content, err := os.ReadFile(filepath.Join(tree, "source.txt")); err != nil || string(content) != "safe" {
		t.Fatalf("拒绝复制后源文件受损: content=%q err=%v", content, err)
	}
	if _, err := os.Stat(filepath.Join(child, "tree")); !os.IsNotExist(err) {
		t.Fatalf("拒绝复制后创建了目标目录: %v", err)
	}
}

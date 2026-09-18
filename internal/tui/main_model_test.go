package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"sshm/internal/repository"
)

var testConnections = []connectionRow{
	{name: "生产服务器", host: "192.168.1.10", port: "22", username: "root", auth: "密码", lastUsed: "今天 09:32", remark: "主站", useCount: 3},
	{name: "开发环境", host: "dev.example.com", port: "22", username: "developer", auth: "密钥", lastUsed: "昨天 18:20", remark: "日常开发", useCount: 8},
	{name: "测试环境", host: "test.example.com", port: "2222", username: "tester", auth: "密钥", lastUsed: "3 天前", remark: "自动化测试", useCount: 1},
	{name: "备份服务器", host: "backup.example.com", port: "22", username: "backup", auth: "密码", lastUsed: "7 天前", remark: "定期备份", useCount: 5},
}

func TestRenderPanelSizeAndSections(t *testing.T) {
	const width, height = 100, 30
	panel := renderPanelWithConnections(width, height, 0, false, newSearchInput(), nil, 0, true)
	lines := strings.Split(panel, "\n")
	if len(lines) != height {
		t.Fatalf("行数 = %d, want %d", len(lines), height)
	}
	for i, line := range lines {
		if got := ansi.StringWidth(line); got != width {
			t.Fatalf("第 %d 行宽度 = %d, want %d", i+1, got, width)
		}
	}
	for _, text := range []string{"SSHM", "搜索名称、主机、用户、备注...", "名称", "主机:端口", "详情", "Enter/l 菜单", "a 新增", "p 凭据", "Ctrl+P 修改密码"} {
		if !strings.Contains(ansi.Strip(panel), text) {
			t.Fatalf("界面缺少 %q", text)
		}
	}
	if strings.Contains(ansi.Strip(panel), "滚动") {
		t.Fatal("底部仍显示已替换的滚动提示")
	}
	if !strings.Contains(lines[height-2], "a 新增") {
		t.Fatal("底部快捷键行后仍有空行")
	}
	compact := ansi.Strip(renderPanelWithConnections(minWidth, height, 0, false, newSearchInput(), nil, 0, true))
	if !strings.Contains(compact, "Ctrl+P 修改密码 | q 退出") {
		t.Fatalf("最小窗口宽度下快捷键提示被截断: %q", compact)
	}
}

func TestClosingCredentialManagerRefreshesConnectionCredentialName(t *testing.T) {
	m := model{connections: []connectionRow{{credID: 7, credName: "旧名称", auth: "密码"}}, modal: credentialFormModel{}}
	updated, _ := m.Update(credentialSavedMsg{credential: repository.Credential{ID: 7, Name: "新名称", Type: "key"}})
	rows := updated.(model).connections
	if len(rows) != 1 || rows[0].credName != "新名称" || rows[0].auth != "私钥" {
		t.Fatalf("凭据保存后连接列表未刷新: %#v", rows)
	}
}

func TestConnectionTableColumns(t *testing.T) {
	const width = 60
	table := newConnectionTable(width, 10, 0, true, testConnections, 0, true)
	columns := table.Columns()
	if len(columns) != 3 || columns[0].Width != nameColumnWidth || columns[2].Width != userColumnWidth {
		t.Fatalf("列表列定义 = %#v", columns)
	}
	if !strings.Contains(strings.Join(table.Rows()[0], " "), "192.168.1.10:22") {
		t.Fatal("列表行未合并 主机:端口")
	}
	if !strings.Contains(fitEllipsis(" 这是一个非常长的连接名称", nameColumnWidth), "...") {
		t.Fatal("长名称未追加省略号")
	}
	selected := strings.Split(table.View(), "\n")[1]
	if plain := ansi.Strip(selected); !strings.HasPrefix(plain, "") || !strings.HasSuffix(plain, "") {
		t.Fatalf("选中行未渲染为胶囊: %q", plain)
	}
	if got := ansi.StringWidth(selected); got != width {
		t.Fatalf("胶囊选中行宽度 = %d, want %d", got, width)
	}
	opaqueEdge := lipgloss.NewStyle().Foreground(selectedColor).Background(backgroundColor).Render("")
	if strings.Contains(selected, opaqueEdge) {
		t.Fatal("选中行圆角边缘不应设置不透明背景色")
	}
}

func TestConnectionDetailsIncludeCredentialName(t *testing.T) {
	panel := renderPanelWithConnections(100, 30, 0, false, newSearchInput(), []connectionRow{{name: "生产", host: "db", port: "22", username: "root", auth: "密码", credName: "生产凭据"}}, 0, true)
	view := ansi.Strip(panel)
	if !strings.Contains(view, "凭据：") || !strings.Contains(view, "生产凭据") {
		t.Fatalf("详情缺少凭据名称: %q", view)
	}
}

func TestSelectionMovesWithinConnections(t *testing.T) {
	m := model{connections: testConnections}
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "down"}))
	if updated.(model).selected != 1 {
		t.Fatalf("下移后 selected = %d", updated.(model).selected)
	}
	for range testConnections {
		updated, _ = updated.(model).Update(tea.KeyPressMsg(tea.Key{Text: "down"}))
	}
	if updated.(model).selected != len(testConnections)-1 {
		t.Fatalf("下移越界 selected = %d", updated.(model).selected)
	}
	updated, _ = updated.(model).Update(tea.KeyPressMsg(tea.Key{Text: "up"}))
	if updated.(model).selected != len(testConnections)-2 {
		t.Fatalf("上移后 selected = %d", updated.(model).selected)
	}
}

func TestSearchFocusIsMutuallyExclusiveWithListFocus(t *testing.T) {
	m := model{selected: 1, searchInput: newSearchInput(), searchReady: true}
	m.searchInput.SetValue("dev")
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "/"}))
	m = updated.(model)
	if !m.searchFocused {
		t.Fatal("搜索未获得焦点")
	}
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "down"}))
	if updated.(model).selected != 1 {
		t.Fatal("搜索焦点下列表仍响应下移")
	}
	updated, _ = updated.(model).Update(tea.KeyPressMsg(tea.Key{Text: "esc"}))
	m = updated.(model)
	if m.searchFocused || m.searchInput.Value() != "" || m.selected != 0 {
		t.Fatalf("Esc 后搜索状态错误: focused=%v value=%q selected=%d", m.searchFocused, m.searchInput.Value(), m.selected)
	}
}

func TestSearchTextInputAndBackspace(t *testing.T) {
	m := model{connections: testConnections, searchInput: newSearchInput(), searchReady: true}
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "/"}))
	m = updated.(model)
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "q"}))
	m = updated.(model)
	if m.searchInput.Value() != "q" || !m.searchFocused {
		t.Fatalf("搜索输入状态 = %q, focused=%v", m.searchInput.Value(), m.searchFocused)
	}
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}))
	if updated.(model).searchInput.Value() != "" {
		t.Fatalf("退格后搜索文本 = %q", updated.(model).searchInput.Value())
	}
	input := newSearchInput()
	input.SetValue("dev")
	if !strings.Contains(ansi.Strip(renderPanelWithConnections(100, 30, 0, true, input, nil, 0, true)), "dev") {
		t.Fatal("搜索文本未渲染")
	}
}

func TestCtrlCClearsMainSearch(t *testing.T) {
	m := model{searchInput: newSearchInput(), searchReady: true, searchFocused: true, selected: 2}
	m.searchInput.SetValue("dev")
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	m = updated.(model)
	if m.searchInput.Value() != "" || m.selected != 0 || !m.searchFocused {
		t.Fatalf("Ctrl+C 清空主搜索框失败: value=%q selected=%d focused=%v", m.searchInput.Value(), m.selected, m.searchFocused)
	}
}

func TestEscClearsMainSearchFromList(t *testing.T) {
	m := model{
		connections: []connectionRow{
			{id: 1, name: "生产", useCount: 2},
			{id: 2, name: "开发", useCount: 1},
			{id: 3, name: "备份", useCount: 3},
		},
		searchInput: newSearchInput(), searchReady: true,
	}
	m.searchInput.SetValue("开发")
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	m = updated.(model)
	connections := m.visibleConnections()
	if cmd != nil || m.searchInput.Value() != "" || m.searchFocused || connections[m.selected].id != 2 {
		t.Fatalf("列表焦点下 Esc 清空主搜索失败: value=%q selected=%d focused=%v cmd=%v", m.searchInput.Value(), m.selected, m.searchFocused, cmd)
	}
}

func TestSearchFiltersAndSortToggles(t *testing.T) {
	m := model{connections: testConnections, searchInput: newSearchInput(), searchReady: true}
	m.searchInput.SetValue("example.com")
	if got := len(m.visibleConnections()); got != 3 {
		t.Fatalf("过滤结果数 = %d, want 3", got)
	}
	m.sortField, m.sortAsc = 'h', true
	rows := m.visibleConnections()
	if rows[0].host > rows[1].host {
		t.Fatal("主机升序排序失败")
	}
	m.sortAsc = false
	rows = m.visibleConnections()
	if rows[0].host < rows[1].host {
		t.Fatal("主机降序排序失败")
	}
}

func TestDefaultSortUsesCountDescending(t *testing.T) {
	m := model{connections: testConnections, searchInput: newSearchInput(), searchReady: true}
	rows := m.visibleConnections()
	for i := 1; i < len(rows); i++ {
		if rows[i-1].useCount < rows[i].useCount {
			t.Fatalf("默认排序未按使用次数降序: %#v", rows)
		}
	}
}

func TestSortUsesCountAsTieBreaker(t *testing.T) {
	connections := []connectionRow{
		{name: "a", host: "same", username: "z", useCount: 1},
		{name: "b", host: "same", username: "a", useCount: 9},
	}
	m := model{connections: connections, searchInput: newSearchInput(), sortField: 'h', sortAsc: true}
	rows := m.visibleConnections()
	if rows[0].name != "b" {
		t.Fatalf("排序字段相同未按使用次数降序: %#v", rows)
	}
}

func TestSortHeaderIndicator(t *testing.T) {
	view := ansi.Strip(newConnectionTable(60, 10, 0, true, testConnections, 'n', true).View())
	if !strings.Contains(view, "名称 ↑") {
		t.Fatalf("升序表头缺少箭头: %q", view)
	}
	view = ansi.Strip(newConnectionTable(60, 10, 0, true, testConnections, 'n', false).View())
	if !strings.Contains(view, "名称 ↓") {
		t.Fatalf("降序表头缺少箭头: %q", view)
	}
}

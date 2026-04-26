package ui

import (
	"fmt"
	"sshm/internal/domain"
	"sshm/internal/i18n"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestNewModelUsesRuntimeDefaults(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, `C:\work\project`, "~/.ssh/id_ed25519")

	if model.startupDir != `C:\work\project` {
		t.Fatalf("startupDir = %q", model.startupDir)
	}
	if model.form.privateKeyPath.Value() != "~/.ssh/id_ed25519" {
		t.Fatalf("privateKeyPath = %q", model.form.privateKeyPath.Value())
	}
	if model.searchInput.Placeholder != "搜索名称 / 主机 / 用户 / 描述" {
		t.Fatalf("placeholder = %q", model.searchInput.Placeholder)
	}
}

func TestHomeHeaderShowsVersionAndAuthor(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, "", "~/.ssh/id_rsa")
	header := model.viewHomeHeader()
	if !strings.Contains(header, "TUI SSH 管理器") || !strings.Contains(header, "dev") || !strings.Contains(header, "nullecho") {
		t.Fatalf("header = %q, want title, version and author", header)
	}
}

func TestNewModelDefaultsVersionAndAuthor(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, "", "~/.ssh/id_rsa")
	header := model.viewHomeHeader()
	if !strings.Contains(header, "dev") || !strings.Contains(header, "nullecho") {
		t.Fatalf("header = %q, want default version and author", header)
	}
}

func TestConnectionListTitleDoesNotRepeatFilter(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}
	model := NewModel(nil, translator, "", "~/.ssh/id_rsa")
	model.listScope = domain.ConnectionListScopeGroup
	model.listGroup = "生产环境"
	model.search = "web"

	title := model.viewConnectionListTitle()
	if !strings.Contains(title, "(生产环境)") {
		t.Fatalf("title = %q", title)
	}
	if strings.Contains(title, "web") {
		t.Fatalf("title = %q, want no filter text", title)
	}
	if fmt.Sprint(model.connectionListScopeStyle().GetForeground()) != fmt.Sprint(model.theme.Styles.GroupScope.GetForeground()) {
		t.Fatalf("scope color = %v, want %v", model.connectionListScopeStyle().GetForeground(), model.theme.Styles.GroupScope.GetForeground())
	}
}

func TestConnectionListTitleShowsAllGroupScope(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}
	model := NewModel(nil, translator, "", "~/.ssh/id_rsa")

	title := model.viewConnectionListTitle()
	if !strings.Contains(title, "(全部)") {
		t.Fatalf("title = %q", title)
	}
	if fmt.Sprint(model.connectionListScopeStyle().GetForeground()) != fmt.Sprint(model.theme.Styles.SectionTitle.GetForeground()) {
		t.Fatalf("scope color = %v, want %v", model.connectionListScopeStyle().GetForeground(), model.theme.Styles.SectionTitle.GetForeground())
	}
}

func TestHomeSearchBlurredValueUsesHighlightColor(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}
	model := NewModel(nil, translator, "", "~/.ssh/id_rsa")
	model.searchInput.SetValue("prod")

	model.viewHomeSearch(32)
	if fmt.Sprint(model.searchInput.TextStyle.GetForeground()) != fmt.Sprint(model.theme.Styles.SearchValueBlurred.GetForeground()) {
		t.Fatalf("text color = %v, want %v", model.searchInput.TextStyle.GetForeground(), model.theme.Styles.SearchValueBlurred.GetForeground())
	}
}

func TestHomeSearchFocusedValueKeepsDefaultColor(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}
	model := NewModel(nil, translator, "", "~/.ssh/id_rsa")
	model.searchMode = true
	model.searchInput.SetValue("prod")

	model.viewHomeSearch(32)
	if fmt.Sprint(model.searchInput.TextStyle.GetForeground()) != fmt.Sprint(lipgloss.NewStyle().GetForeground()) {
		t.Fatalf("text color = %v, want default empty style", model.searchInput.TextStyle.GetForeground())
	}
}

func TestGroupDeleteRequiresConfirmation(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}
	model := NewModel(nil, translator, "", "~/.ssh/id_rsa")
	model.overlay = overlayGroup
	model.groups.items = []domain.ConnectionGroupListItem{
		{Name: "未分组", Ungrouped: true},
		{ID: 1, Name: "生产环境"},
	}
	model.groups.selected = 1

	updated, _ := model.updateGroupPanel(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	got := updated.(*Model)
	if !got.groups.confirming {
		t.Fatalf("confirming = false, want true")
	}

	updated, _ = got.updateGroupPanel(tea.KeyMsg{Type: tea.KeyEsc})
	got = updated.(*Model)
	if !got.groups.confirming || got.overlay != overlayGroup {
		t.Fatalf("esc should keep group confirmation open: confirming=%v overlay=%v", got.groups.confirming, got.overlay)
	}

	updated, _ = got.updateGroupPanel(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	got = updated.(*Model)
	if got.groups.confirming || got.overlay != overlayGroup {
		t.Fatalf("n should cancel confirmation only: confirming=%v overlay=%v", got.groups.confirming, got.overlay)
	}

	updated, _ = got.updateGroupPanel(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	got = updated.(*Model)
	updated, _ = got.updateGroupPanel(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	got = updated.(*Model)
	if got.overlay != overlayNone || got.groups.confirming {
		t.Fatalf("q should close group panel: confirming=%v overlay=%v", got.groups.confirming, got.overlay)
	}
}

func TestRenderConnectionRowUsesTwoLinesWithoutPort(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}
	model := NewModel(nil, translator, "", "~/.ssh/id_rsa")

	row := model.renderConnectionRow(Connection{
		Name:     "ctk8s",
		Username: "root",
		Host:     "113.249.104.116",
		Port:     22,
	}, false, 32)

	if !strings.Contains(row, "ctk8s") {
		t.Fatalf("row = %q, want name", row)
	}
	if !strings.Contains(row, "root@113.249.104.116") {
		t.Fatalf("row = %q, want second line meta", row)
	}
	if strings.Contains(row, ":22") {
		t.Fatalf("row = %q, want no port", row)
	}
}

func TestViewConnectionListUsesTwoLineViewport(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}
	model := NewModel(nil, translator, "", "~/.ssh/id_rsa")
	model.connections = []Connection{
		{Name: "one", Username: "root", Host: "10.0.0.1", Port: 22},
		{Name: "two", Username: "root", Host: "10.0.0.2", Port: 22},
		{Name: "three", Username: "root", Host: "10.0.0.3", Port: 22},
	}

	view := model.viewConnectionList(32, 6)
	if strings.Contains(view, "three") {
		t.Fatalf("view = %q, want only first two items in viewport", view)
	}
	if !strings.Contains(view, "one") || !strings.Contains(view, "two") {
		t.Fatalf("view = %q, want first two items", view)
	}
}

func TestHomeShortcutsUseLowercaseKeys(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}
	model := NewModel(nil, translator, "", "~/.ssh/id_rsa")

	help := model.viewHomeHelp()
	footer := model.viewHomeFooter(500)
	for _, shortcut := range homeShortcuts() {
		if strings.ToLower(shortcut.key) != shortcut.key {
			t.Fatalf("shortcut key = %q, want lowercase", shortcut.key)
		}
		if !strings.Contains(help, shortcut.key) {
			t.Fatalf("help = %q, want shortcut %q", help, shortcut.key)
		}
		if !strings.Contains(footer, shortcut.key) {
			t.Fatalf("footer = %q, want shortcut %q", footer, shortcut.key)
		}
	}
}

func TestFormKeepPasswordDependsOnOriginalAuthType(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, "", "~/.ssh/id_rsa")
	model.form = newFormState(&domain.Connection{
		ID:       1,
		Name:     "prod",
		Host:     "example.com",
		Port:     22,
		Username: "root",
		AuthType: domain.AuthTypePrivateKey,
	}, translator, "~/.ssh/id_rsa", model.theme)
	model.form.authType = domain.AuthTypePassword
	model.form.keepPassword = true
	if model.form.shouldKeepPassword() {
		t.Fatal("shouldKeepPassword() = true, want false for original private key auth")
	}

	model.form = newFormState(&domain.Connection{
		ID:       2,
		Name:     "prod2",
		Host:     "example.com",
		Port:     22,
		Username: "root",
		AuthType: domain.AuthTypePassword,
	}, translator, "~/.ssh/id_rsa", model.theme)
	if !model.form.shouldKeepPassword() {
		t.Fatal("shouldKeepPassword() = false, want true for existing password auth")
	}
}

func TestHomeQuitUsesQOrCtrlC(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}
	model := NewModel(nil, translator, "", "~/.ssh/id_rsa")

	for _, msg := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
		{Type: tea.KeyCtrlC},
	} {
		_, cmd := model.updateHome(msg)
		if cmd == nil {
			t.Fatalf("%q cmd = nil, want quit", msg.String())
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("%q cmd msg = %T, want tea.QuitMsg", msg.String(), cmd())
		}
	}
}

func TestHomeCtrlShortcutsNoLongerAcceptSingleKeys(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}
	model := NewModel(nil, translator, "", "~/.ssh/id_rsa")
	model.connections = []Connection{{ID: 7, Name: "prod", Username: "root", Host: "10.0.0.1", Port: 22}}

	for _, key := range []rune{'a', 'e', 'd', 'o'} {
		updated, cmd := model.updateHome(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
		got := updated.(*Model)
		if cmd != nil || got.page != pageHome || got.overlay != overlayNone {
			t.Fatalf("single key %q changed state: page=%v overlay=%v cmd nil=%v", string(key), got.page, got.overlay, cmd == nil)
		}
	}
}

func TestHomeCtrlShortcutsStillWork(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, "", "~/.ssh/id_rsa")
	updated, cmd := model.updateHome(tea.KeyMsg{Type: tea.KeyCtrlN})
	got := updated.(*Model)
	if got.page != pageForm || cmd == nil {
		t.Fatalf("ctrl+n page=%v cmd nil=%v, want form and clear screen cmd", got.page, cmd == nil)
	}

	model = NewModel(nil, translator, "", "~/.ssh/id_rsa")
	model.connections = []Connection{{ID: 7, Name: "prod", Username: "root", Host: "10.0.0.1", Port: 22}}
	updated, _ = model.updateHome(tea.KeyMsg{Type: tea.KeyCtrlD})
	got = updated.(*Model)
	if got.overlay != overlayDelete || got.deleteTarget != 7 {
		t.Fatalf("ctrl+d overlay=%v deleteTarget=%d, want delete overlay for 7", got.overlay, got.deleteTarget)
	}

	model = NewModel(nil, translator, "", "~/.ssh/id_rsa")
	model.connections = []Connection{{ID: 9, Name: "stage", Username: "root", Host: "10.0.0.2", Port: 22}}
	updated, cmd = model.updateHome(tea.KeyMsg{Type: tea.KeyCtrlG})
	got = updated.(*Model)
	if got.overlay != overlayGroup || got.groups.mode != groupPanelMove || got.groups.targetID != 9 || cmd == nil {
		t.Fatalf("ctrl+g overlay=%v mode=%v target=%d cmd nil=%v, want move group overlay", got.overlay, got.groups.mode, got.groups.targetID, cmd == nil)
	}
}

func TestGroupPanelTitleShowsCurrentMode(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}
	model := NewModel(nil, translator, "", "~/.ssh/id_rsa")
	model.groups.items = []domain.ConnectionGroupListItem{{Name: "未分组", Ungrouped: true}}

	model.groups.mode = groupPanelFilter
	filterView := model.viewGroupPanel()
	if !strings.Contains(filterView, "选择组") || !strings.Contains(filterView, "当前：选择分组以过滤主列表") {
		t.Fatalf("filter group panel = %q, want filter mode title and desc", filterView)
	}

	model.groups.mode = groupPanelMove
	moveView := model.viewGroupPanel()
	if !strings.Contains(moveView, "移动组") || !strings.Contains(moveView, "当前：将选中连接移动到目标分组") {
		t.Fatalf("move group panel = %q, want move mode title and desc", moveView)
	}
}

func TestGroupFilterSelectionClearsSearch(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}
	model := NewModel(nil, translator, "", "~/.ssh/id_rsa")
	model.overlay = overlayGroup
	model.groups.mode = groupPanelFilter
	model.groups.items = []domain.ConnectionGroupListItem{
		{Name: "未分组", Ungrouped: true},
		{ID: 1, Name: "生产环境"},
	}
	model.groups.selected = 1
	model.search = "prod"
	model.searchMode = true
	model.searchInput.SetValue("prod")

	updated, cmd := model.updateGroupPanel(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*Model)
	if got.overlay != overlayNone {
		t.Fatalf("overlay = %v, want %v", got.overlay, overlayNone)
	}
	if got.listScope != domain.ConnectionListScopeGroup || got.listGroupID != 1 || got.listGroup != "生产环境" {
		t.Fatalf("scope=%v groupID=%d group=%q", got.listScope, got.listGroupID, got.listGroup)
	}
	if got.search != "" {
		t.Fatalf("search = %q, want empty", got.search)
	}
	if got.searchInput.Value() != "" {
		t.Fatalf("searchInput = %q, want empty", got.searchInput.Value())
	}
	if got.searchMode {
		t.Fatalf("searchMode = true, want false")
	}
	if cmd == nil {
		t.Fatalf("cmd = nil, want reload connections command")
	}
}

func TestImportPanelUsesEscToQuitAndJKToMove(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}
	model := NewModel(nil, translator, "", "~/.ssh/id_rsa")
	model.page = pageImport
	model.imports = newImportState(translator, model.theme)

	pathView := model.viewImport()
	if !strings.Contains(pathView, "esc") || strings.Contains(pathView, "\nq ") {
		t.Fatalf("path import shortcuts = %q, want esc", pathView)
	}

	updated, cmd := model.updateImport(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(*Model)
	if got.page != pageHome || cmd == nil {
		t.Fatalf("esc on path page=%v cmd nil=%v, want home and clear screen", got.page, cmd == nil)
	}

	model.page = pageImport
	model.imports.step = importStepPreview
	previewView := model.viewImport()
	if !strings.Contains(previewView, "j/k") || !strings.Contains(previewView, "esc") || strings.Contains(previewView, "↑/↓") {
		t.Fatalf("preview import shortcuts = %q, want j/k and esc, no arrows", previewView)
	}

	updated, cmd = model.updateImport(tea.KeyMsg{Type: tea.KeyEsc})
	got = updated.(*Model)
	if got.page != pageHome || cmd == nil {
		t.Fatalf("esc on preview page=%v cmd nil=%v, want home and clear screen", got.page, cmd == nil)
	}
}

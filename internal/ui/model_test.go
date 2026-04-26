package ui

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sshm/internal/app"
	"sshm/internal/domain"
	"sshm/internal/i18n"
	"sshm/internal/security"
	"sshm/internal/store/sqlite"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type probeOnlyRemote struct {
	probeErr error
}

type stubRemoteSession struct{}

func (*stubRemoteSession) OpenShell() error { return nil }
func (*stubRemoteSession) ListRemote(targetPath string) ([]domain.FileEntry, string, error) {
	return nil, "", nil
}
func (*stubRemoteSession) PathExists(targetPath string) (bool, error) { return false, nil }
func (*stubRemoteSession) Upload(localPath string, remoteDir string, progress func(domain.TransferProgress)) error {
	return nil
}
func (*stubRemoteSession) Download(remotePath string, localDir string, progress func(domain.TransferProgress)) error {
	return nil
}
func (*stubRemoteSession) HomeDir() (string, error) { return "/", nil }
func (*stubRemoteSession) Close() error             { return nil }

func (r probeOnlyRemote) ProbeShell(conn domain.Connection, password string) error { return r.probeErr }
func (r probeOnlyRemote) OpenSession(conn domain.Connection, password string) (app.RemoteSession, error) {
	if r.probeErr != nil {
		return nil, r.probeErr
	}
	return &stubRemoteSession{}, nil
}
func (probeOnlyRemote) OpenShell(conn domain.Connection, password string) error { return nil }
func (probeOnlyRemote) RunCommand(conn domain.Connection, password string, command string, stdout io.Writer, stderr io.Writer) error {
	return nil
}
func (probeOnlyRemote) ListRemote(conn domain.Connection, password string, targetPath string) ([]domain.FileEntry, string, error) {
	return nil, "", nil
}
func (probeOnlyRemote) PathExists(conn domain.Connection, password string, targetPath string) (bool, error) {
	return false, nil
}
func (probeOnlyRemote) Upload(conn domain.Connection, password string, localPath string, remoteDir string, progress func(domain.TransferProgress)) error {
	return nil
}
func (probeOnlyRemote) Download(conn domain.Connection, password string, remotePath string, localDir string, progress func(domain.TransferProgress)) error {
	return nil
}
func (probeOnlyRemote) HomeDir(conn domain.Connection, password string) (string, error) {
	return "/", nil
}

func newModelWithProbeServices(t *testing.T, probeErr error) *Model {
	t.Helper()

	tempDir := t.TempDir()
	repo, err := sqlite.Open(filepath.Join(tempDir, "sshm.db"))
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	crypto, err := security.LoadOrCreateKey(filepath.Join(tempDir, "app.key"))
	if err != nil {
		t.Fatalf("LoadOrCreateKey() error = %v", err)
	}
	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	services := app.NewServices(repo, crypto, probeOnlyRemote{probeErr: probeErr}, "~/.ssh/id_rsa")
	model := NewModel(services, translator, "", "~/.ssh/id_rsa")
	created, err := services.Connections.Create(domain.ConnectionInput{
		Name:     "prod",
		Host:     "example.com",
		Port:     22,
		Username: "root",
		AuthType: domain.AuthTypePrivateKey,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	model.connections = []Connection{created}
	return model
}

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
	if !strings.Contains(header, "SSH 管理器") || !strings.Contains(header, "dev") || !strings.Contains(header, "nullecho") {
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

func TestHomeEnterStartsShellProbe(t *testing.T) {
	t.Parallel()

	model := newModelWithProbeServices(t, nil)

	updated, cmd := model.updateHome(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*Model)
	if !got.connecting {
		t.Fatal("connecting = false, want true")
	}
	if got.Result().ShellSession != nil {
		t.Fatalf("ShellSession = %#v, want nil before probe completes", got.Result().ShellSession)
	}
	if got.status != got.translator.T("status.connecting_shell", "prod") {
		t.Fatalf("status = %q, want connecting status", got.status)
	}
	if !strings.Contains(got.viewHome(), got.translator.T("status.connecting_shell", "prod")) {
		t.Fatalf("viewHome() = %q, want connecting status rendered", got.viewHome())
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want probe command")
	}

	msg := cmd()
	probeDone, ok := msg.(homeProbeDoneMsg)
	if !ok {
		t.Fatalf("cmd() msg = %T, want homeProbeDoneMsg", msg)
	}
	if probeDone.action != homeProbeShell || probeDone.connectionName != "prod" || probeDone.err != nil || probeDone.session == nil {
		t.Fatalf("probeDone = %#v", probeDone)
	}
}

func TestShellProbeFailureStaysOnHomeAndShowsError(t *testing.T) {
	t.Parallel()

	model := newModelWithProbeServices(t, errors.New("auth failed"))
	model.selected = 0

	updated, cmd := model.updateHome(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("cmd = nil, want probe command")
	}
	got := updated.(*Model)

	updated, nextCmd := got.Update(cmd())
	if nextCmd != nil {
		t.Fatalf("nextCmd = %v, want nil on probe failure", nextCmd)
	}
	got = updated.(*Model)
	if got.page != pageHome {
		t.Fatalf("page = %v, want %v", got.page, pageHome)
	}
	if got.connecting {
		t.Fatal("connecting = true, want false")
	}
	if got.selected != 0 {
		t.Fatalf("selected = %d, want 0", got.selected)
	}
	if got.err == nil {
		t.Fatal("err = nil, want error status")
	}
	want := got.translator.T("status.shell_connect_failed", "prod", "auth failed")
	if got.status != want {
		t.Fatalf("status = %q, want %q", got.status, want)
	}
}

func TestShellProbeSuccessTransitionsToShellReady(t *testing.T) {
	t.Parallel()

	model := newModelWithProbeServices(t, nil)

	updated, cmd := model.updateHome(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("cmd = nil, want probe command")
	}

	updated, nextCmd := updated.(*Model).Update(cmd())
	if nextCmd == nil {
		t.Fatal("nextCmd = nil, want shell ready command")
	}

	msg := nextCmd()
	shellReady, ok := msg.(shellReadyMsg)
	if !ok {
		t.Fatalf("nextCmd() msg = %T, want shellReadyMsg", msg)
	}
	if shellReady.session == nil {
		t.Fatal("session = nil, want non-nil")
	}

	finalModel, quitCmd := updated.(*Model).Update(msg)
	if quitCmd == nil {
		t.Fatal("quitCmd = nil, want tea.Quit")
	}
	if finalModel.(*Model).Result().ShellSession != shellReady.session {
		t.Fatalf("ShellSession = %#v, want %#v", finalModel.(*Model).Result().ShellSession, shellReady.session)
	}
}

func TestHomeCtrlOStartsBrowserProbe(t *testing.T) {
	t.Parallel()

	model := newModelWithProbeServices(t, nil)

	updated, cmd := model.updateHome(tea.KeyMsg{Type: tea.KeyCtrlO})
	got := updated.(*Model)
	if !got.connecting {
		t.Fatal("connecting = false, want true")
	}
	if got.page != pageHome {
		t.Fatalf("page = %v, want %v before probe completes", got.page, pageHome)
	}
	if got.status != got.translator.T("status.connecting_browser", "prod") {
		t.Fatalf("status = %q, want browser connecting status", got.status)
	}
	if !strings.Contains(got.viewHome(), got.translator.T("status.connecting_browser", "prod")) {
		t.Fatalf("viewHome() = %q, want browser connecting status rendered", got.viewHome())
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want browser probe command")
	}

	msg := cmd()
	probeDone, ok := msg.(homeProbeDoneMsg)
	if !ok {
		t.Fatalf("cmd() msg = %T, want homeProbeDoneMsg", msg)
	}
	if probeDone.action != homeProbeBrowser || probeDone.connectionName != "prod" || probeDone.err != nil || probeDone.session == nil {
		t.Fatalf("probeDone = %#v", probeDone)
	}
}

func TestBrowserProbeFailureStaysOnHomeAndShowsError(t *testing.T) {
	t.Parallel()

	model := newModelWithProbeServices(t, errors.New("host unreachable"))

	updated, cmd := model.updateHome(tea.KeyMsg{Type: tea.KeyCtrlO})
	if cmd == nil {
		t.Fatal("cmd = nil, want browser probe command")
	}

	updated, nextCmd := updated.(*Model).Update(cmd())
	if nextCmd != nil {
		t.Fatalf("nextCmd = %v, want nil on browser probe failure", nextCmd)
	}
	got := updated.(*Model)
	if got.page != pageHome {
		t.Fatalf("page = %v, want %v", got.page, pageHome)
	}
	if got.connecting {
		t.Fatal("connecting = true, want false")
	}
	want := got.translator.T("status.browser_connect_failed", "prod", "host unreachable")
	if got.status != want {
		t.Fatalf("status = %q, want %q", got.status, want)
	}
}

func TestBrowserProbeSuccessOpensBrowser(t *testing.T) {
	t.Parallel()

	model := newModelWithProbeServices(t, nil)

	updated, cmd := model.updateHome(tea.KeyMsg{Type: tea.KeyCtrlO})
	if cmd == nil {
		t.Fatal("cmd = nil, want browser probe command")
	}

	updated, nextCmd := updated.(*Model).Update(cmd())
	if nextCmd == nil {
		t.Fatal("nextCmd = nil, want browser load commands")
	}

	got := updated.(*Model)
	if got.page != pageBrowser {
		t.Fatalf("page = %v, want %v", got.page, pageBrowser)
	}
	if got.connecting {
		t.Fatal("connecting = true, want false")
	}
	if got.browser.connectionID == 0 || got.browser.connection.Name != "prod" {
		t.Fatalf("browser connection = %#v", got.browser.connection)
	}
	if got.browser.session == nil {
		t.Fatal("browser session = nil, want non-nil")
	}
	if !got.browser.localPanel.loading || !got.browser.remotePanel.loading {
		t.Fatalf("loading local=%v remote=%v, want both true", got.browser.localPanel.loading, got.browser.remotePanel.loading)
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
	if !strings.Contains(filterView, "分组") || !strings.Contains(filterView, "连接数") {
		t.Fatalf("filter group panel = %q, want table headers", filterView)
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

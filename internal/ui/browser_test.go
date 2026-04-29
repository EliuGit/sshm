package ui

import (
	"errors"
	"sshm/internal/app"
	"sshm/internal/domain"
	"sshm/internal/i18n"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type closeTrackingSession struct {
	closed bool
}

func (s *closeTrackingSession) OpenShell() error { return nil }
func (s *closeTrackingSession) ListRemote(targetPath string) ([]domain.FileEntry, string, error) {
	return nil, "", nil
}
func (s *closeTrackingSession) PathExists(targetPath string) (bool, error) { return false, nil }
func (s *closeTrackingSession) Upload(localPath string, remoteDir string, progress func(domain.TransferProgress)) error {
	return nil
}
func (s *closeTrackingSession) Download(remotePath string, localDir string, progress func(domain.TransferProgress)) error {
	return nil
}
func (s *closeTrackingSession) HomeDir() (string, error) { return "/", nil }
func (s *closeTrackingSession) Close() error {
	s.closed = true
	return nil
}

var _ app.RemoteSession = (*closeTrackingSession)(nil)

func TestBrowserEscClearsActiveFilter(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, `C:\work\project`, "~/.ssh/id_ed25519")
	model.page = pageBrowser
	model.browser.activePanel = domain.LocalPanel
	model.browser.localPanel.path = `C:\work\project`
	model.browser.localPanel.filter = "log"

	updated, cmd := model.updateBrowser(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(*Model)

	if got.page != pageBrowser {
		t.Fatalf("page = %v, want %v", got.page, pageBrowser)
	}
	if got.browser.localPanel.filter != "" {
		t.Fatalf("filter = %q, want empty", got.browser.localPanel.filter)
	}
	if cmd != nil {
		t.Fatalf("cmd = %v, want no reload command", cmd)
	}
}

func TestBrowserFilterInputEscClearsFilter(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, `C:\work\project`, "~/.ssh/id_ed25519")
	model.page = pageBrowser
	model.browser.activePanel = domain.LocalPanel
	model.browser.localPanel.path = `C:\work\project`
	model.browser.localPanel.filter = "log"
	model.browser.localPanel.items = []domain.FileEntry{
		{Name: "logs", Path: `C:\work\project\logs`, IsDir: true, Panel: domain.LocalPanel},
	}
	model.overlay = overlayBrowserInput
	model.browser.inputMode = browserInputFilter
	model.browser.input.SetValue("lo")

	updated, cmd := model.updateBrowser(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(*Model)

	if got.overlay != overlayNone {
		t.Fatalf("overlay = %v, want %v", got.overlay, overlayNone)
	}
	if got.browser.localPanel.filter != "" {
		t.Fatalf("filter = %q, want empty", got.browser.localPanel.filter)
	}
	if cmd != nil {
		t.Fatalf("cmd = %v, want nil", cmd)
	}
}

func TestBrowserFilterInputUpdatesLocalPanelInRealTime(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(&app.Services{Files: &app.FileService{}}, translator, t.TempDir(), "~/.ssh/id_ed25519")
	model.page = pageBrowser
	model.browser.activePanel = domain.LocalPanel
	model.browser.localPanel.path = t.TempDir()
	model.browser.localPanel.items = []domain.FileEntry{
		{Name: "logs", Path: `C:\work\project\logs`, IsDir: true, Panel: domain.LocalPanel},
		{Name: "app.txt", Path: `C:\work\project\app.txt`, Panel: domain.LocalPanel},
	}

	updated, _ := model.openBrowserFilterInput()
	got := updated.(*Model)
	updated, _ = got.updateBrowser(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	got = updated.(*Model)

	if got.overlay != overlayBrowserInput {
		t.Fatalf("overlay = %v, want %v", got.overlay, overlayBrowserInput)
	}
	if got.browser.localPanel.filter != "l" {
		t.Fatalf("filter = %q, want %q", got.browser.localPanel.filter, "l")
	}
	if got.browser.localPanel.loading {
		t.Fatal("localPanel.loading = true, want cached filter without reload")
	}
	if len(got.browser.localPanel.rows()) != 2 {
		t.Fatalf("rows = %d, want parent + matched item", len(got.browser.localPanel.rows()))
	}
}

func TestBrowserFilterInputUpdatesRemotePanelInRealTime(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, `C:\work\project`, "~/.ssh/id_ed25519")
	model.page = pageBrowser
	model.browser.activePanel = domain.RemotePanel
	model.browser.remotePanel.path = "/var/log"
	model.browser.session = &closeTrackingSession{}
	model.browser.remotePanel.items = []domain.FileEntry{
		{Name: "app.log", Path: "/var/log/app.log", Panel: domain.RemotePanel},
		{Name: "syslog", Path: "/var/log/syslog", Panel: domain.RemotePanel},
	}

	updated, _ := model.openBrowserFilterInput()
	got := updated.(*Model)
	updated, _ = got.updateBrowser(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got = updated.(*Model)

	if got.overlay != overlayBrowserInput {
		t.Fatalf("overlay = %v, want %v", got.overlay, overlayBrowserInput)
	}
	if got.browser.remotePanel.filter != "a" {
		t.Fatalf("filter = %q, want %q", got.browser.remotePanel.filter, "a")
	}
	if got.browser.remotePanel.loading {
		t.Fatal("remotePanel.loading = true, want cached filter without reload")
	}
	if len(got.browser.remotePanel.rows()) != 2 {
		t.Fatalf("rows = %d, want parent + matched item", len(got.browser.remotePanel.rows()))
	}
}

func TestBrowserPathChangeClearsFilter(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, `C:\work\project`, "~/.ssh/id_ed25519")
	model.page = pageBrowser
	model.browser.activePanel = domain.LocalPanel
	model.browser.localPanel.path = `C:\work\project`
	model.browser.localPanel.filter = "log"
	model.browser.localPanel.items = []domain.FileEntry{
		{Name: "logs", Path: `C:\work\project\logs`, IsDir: true, Panel: domain.LocalPanel},
	}

	updated, cmd := model.openActiveBrowserSelection()
	got := updated.(*Model)

	if got.browser.localPanel.filter != "" {
		t.Fatalf("filter = %q, want cleared on path change", got.browser.localPanel.filter)
	}
	if !got.browser.localPanel.loading {
		t.Fatal("localPanel.loading = false, want true")
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want navigation reload command")
	}
}

func TestBrowserRefreshSamePathKeepsFilter(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, `C:\work\project`, "~/.ssh/id_ed25519")
	model.page = pageBrowser
	model.browser.localPanel.path = `C:\work\project`
	model.browser.localPanel.filter = "log"

	cmd := model.navigateBrowserPath(domain.LocalPanel, `C:\work\project`)
	if model.browser.localPanel.filter != "log" {
		t.Fatalf("filter = %q, want preserved on same path", model.browser.localPanel.filter)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want reload command")
	}
}

func TestBrowserIgnoresStaleRemoteLoadResult(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, `C:\work\project`, "~/.ssh/id_ed25519")
	model.page = pageBrowser
	model.browser.remotePanel.request = 2
	model.browser.remotePanel.loading = true
	model.browser.remotePanel.filter = "new"

	updated, _ := model.Update(remoteLoadedMsg{
		request: 1,
		path:    "/var/log",
		items: []domain.FileEntry{
			{Name: "old.log", Path: "/var/log/old.log", Panel: domain.RemotePanel},
		},
	})
	got := updated.(*Model)

	if !got.browser.remotePanel.loading {
		t.Fatal("remotePanel.loading = false, want still loading for newer request")
	}
	if len(got.browser.remotePanel.items) != 0 {
		t.Fatalf("items = %#v, want stale result ignored", got.browser.remotePanel.items)
	}
	if got.browser.remotePanel.filter != "new" {
		t.Fatalf("filter = %q, want unchanged", got.browser.remotePanel.filter)
	}
}

func TestBrowserQReturnsHome(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, `C:\work\project`, "~/.ssh/id_ed25519")
	model.page = pageBrowser

	updated, _ := model.updateBrowser(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	got := updated.(*Model)

	if got.page != pageHome {
		t.Fatalf("page = %v, want %v", got.page, pageHome)
	}
	if got.status != translator.T("status.returned_connections") {
		t.Fatalf("status = %q", got.status)
	}
}

func TestBrowserQClosesRemoteSession(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, `C:\work\project`, "~/.ssh/id_ed25519")
	model.page = pageBrowser
	session := &closeTrackingSession{}
	model.browser.session = session

	updated, _ := model.updateBrowser(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	got := updated.(*Model)
	if !session.closed {
		t.Fatal("session.closed = false, want true")
	}
	if got.browser.session != nil {
		t.Fatalf("browser.session = %#v, want nil", got.browser.session)
	}
}

func TestRemoteLoadedConnectionErrorReturnsHomeAndClosesSession(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, `C:\work\project`, "~/.ssh/id_ed25519")
	model.page = pageBrowser
	session := &closeTrackingSession{}
	model.browser.session = session

	updated, cmd := model.Update(remoteLoadedMsg{err: errors.New("broken pipe")})
	got := updated.(*Model)
	if got.page != pageHome {
		t.Fatalf("page = %v, want %v", got.page, pageHome)
	}
	if !session.closed {
		t.Fatal("session.closed = false, want true")
	}
	if got.browser.session != nil {
		t.Fatalf("browser.session = %#v, want nil", got.browser.session)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want reload connections command")
	}
}

func TestBrowserOpConnectionErrorReturnsHomeAndClosesSession(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, `C:\work\project`, "~/.ssh/id_ed25519")
	model.page = pageBrowser
	session := &closeTrackingSession{}
	model.browser.session = session

	updated, cmd := model.Update(opDoneMsg{err: errors.New("remote session is closed")})
	got := updated.(*Model)
	if got.page != pageHome {
		t.Fatalf("page = %v, want %v", got.page, pageHome)
	}
	if !session.closed {
		t.Fatal("session.closed = false, want true")
	}
	if got.browser.session != nil {
		t.Fatalf("browser.session = %#v, want nil", got.browser.session)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want reload connections command")
	}
}

func TestRenderBrowserPanelShowsInlineFilterWithoutPath(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, `C:\work\project`, "~/.ssh/id_ed25519")
	panel := filePanel{
		panel:  domain.LocalPanel,
		title:  translator.T("browser.local"),
		path:   `C:\work\project`,
		filter: "log",
	}

	view := model.renderBrowserPanel(panel, 60, 8, true)
	if !strings.Contains(view, "（过滤：log）") {
		t.Fatalf("view missing filter hint: %q", view)
	}
	if strings.Contains(view, `C:\work\project`) {
		t.Fatalf("view = %q, want path removed from header", view)
	}
}

func TestRenderBrowserPanelUsesIcons(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, `C:\work\project`, "~/.ssh/id_ed25519")
	panel := filePanel{
		panel: domain.LocalPanel,
		title: translator.T("browser.local"),
		path:  `C:\work\project`,
		items: []domain.FileEntry{
			{Name: "logs", Path: `C:\work\project\logs`, IsDir: true, Panel: domain.LocalPanel},
			{Name: "app.log", Path: `C:\work\project\app.log`, Panel: domain.LocalPanel},
		},
	}

	view := model.renderBrowserPanel(panel, 60, 8, true)
	if !strings.Contains(view, "📁 logs/") {
		t.Fatalf("view missing dir icon: %q", view)
	}
	if !strings.Contains(view, "📄 app.log") {
		t.Fatalf("view missing file icon: %q", view)
	}
}

func TestBrowserFooterMatchesActiveShortcuts(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, `C:\work\project`, "~/.ssh/id_ed25519")
	model.page = pageBrowser
	model.browser = newBrowserState(translator, model.styles)
	model.browser.activePanel = domain.LocalPanel
	model.browser.localPanel.path = `C:\work\project`

	footer := model.renderBrowserFooter(220)
	for _, key := range []string{"enter/l", "tab", "c-u", "c-d", "c-p", "q"} {
		if !strings.Contains(footer, key) {
			t.Fatalf("footer = %q, want shortcut %q", footer, key)
		}
	}
	if strings.Contains(footer, `C:\work\project`) {
		t.Fatalf("footer = %q, want no path text", footer)
	}
}

func TestBrowserConfirmDialogShowsSourceAndTarget(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, `C:\work\project`, "~/.ssh/id_ed25519")
	model.page = pageBrowser
	model.overlay = overlayBrowserConfirm
	model.confirm = confirmState{
		action:           confirmActionBrowserOverwrite,
		title:            translator.T("browser.overwrite"),
		sourcePath:       `C:\work\project\archive.log`,
		targetPath:       `/tmp/archive.log`,
		confirmSelection: false,
		choiceEnabled:    true,
	}
	model.browser.pending = &browserTransfer{source: `C:\work\project\archive.log`}

	dialog := model.viewBrowserOverwriteConfirm(100)
	if !strings.Contains(dialog, "来源：") || !strings.Contains(dialog, `C:\work\project\archive.log`) {
		t.Fatalf("dialog = %q, want source path", dialog)
	}
	if !strings.Contains(dialog, "目标：") || !strings.Contains(dialog, `/tmp/archive.log`) {
		t.Fatalf("dialog = %q, want target path", dialog)
	}
}

func TestBrowserConfirmEscCancelsOnlyConfirmState(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, `C:\work\project`, "~/.ssh/id_ed25519")
	model.page = pageBrowser
	model.overlay = overlayBrowserConfirm
	model.browser.activePanel = domain.RemotePanel
	model.browser.localPanel.path = `C:\work\project`
	model.browser.localPanel.filter = "log"
	model.browser.remotePanel.path = "/var/log"
	model.browser.remotePanel.filter = "app"
	model.browser.pending = &browserTransfer{
		source:   `C:\work\project\archive.log`,
		target:   "/var/log/archive.log",
		selectBy: "archive.log",
		panel:    domain.RemotePanel,
	}
	model.confirm = confirmState{
		action:           confirmActionBrowserOverwrite,
		title:            translator.T("browser.overwrite"),
		sourcePath:       `C:\work\project\archive.log`,
		targetPath:       "/var/log/archive.log",
		choiceEnabled:    true,
		confirmSelection: false,
	}

	updated, _ := model.updateBrowser(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(*Model)
	if got.overlay != overlayNone {
		t.Fatalf("overlay = %v, want %v", got.overlay, overlayNone)
	}
	if got.confirm.action != confirmActionNone {
		t.Fatalf("confirm.action = %v, want none", got.confirm.action)
	}
	if got.browser.pending != nil {
		t.Fatalf("pending = %#v, want nil", got.browser.pending)
	}
	if got.page != pageBrowser {
		t.Fatalf("page = %v, want %v", got.page, pageBrowser)
	}
	if got.browser.activePanel != domain.RemotePanel || got.browser.localPanel.path != `C:\work\project` || got.browser.remotePanel.path != "/var/log" {
		t.Fatalf("browser state changed unexpectedly: active=%v local=%q remote=%q", got.browser.activePanel, got.browser.localPanel.path, got.browser.remotePanel.path)
	}
	if got.browser.localPanel.filter != "log" || got.browser.remotePanel.filter != "app" {
		t.Fatalf("filters changed unexpectedly: local=%q remote=%q", got.browser.localPanel.filter, got.browser.remotePanel.filter)
	}
	if got.status != translator.T("status.transfer_cancelled") {
		t.Fatalf("status = %q, want cancelled", got.status)
	}
}

func TestBrowserPaletteOpensAndCanSwitchPanel(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, `C:\work\project`, "~/.ssh/id_ed25519")
	model.page = pageBrowser
	model.browser = newBrowserState(translator, model.styles)
	model.browser.activePanel = domain.LocalPanel

	updated, _ := model.updateBrowser(tea.KeyMsg{Type: tea.KeyCtrlP})
	got := updated.(*Model)
	if got.overlay != overlayCommandPalette {
		t.Fatalf("overlay = %v, want %v", got.overlay, overlayCommandPalette)
	}

	updated, _ = got.updateBrowser(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'切'}})
	got = updated.(*Model)
	if got.palette.input.Value() != "切" {
		t.Fatalf("palette query = %q, want 切", got.palette.input.Value())
	}

	updated, cmd := got.updateBrowser(tea.KeyMsg{Type: tea.KeyEnter})
	got = updated.(*Model)
	if got.browser.activePanel != domain.RemotePanel {
		t.Fatalf("activePanel = %v, want %v", got.browser.activePanel, domain.RemotePanel)
	}
	if got.overlay != overlayNone {
		t.Fatalf("overlay = %v, want %v", got.overlay, overlayNone)
	}
	if cmd != nil {
		t.Fatalf("cmd = %v, want nil", cmd)
	}
}

func TestBrowserFurtherActionClearsStaleErrorStatus(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, `C:\work\project`, "~/.ssh/id_ed25519")
	model.page = pageBrowser
	model.browser = newBrowserState(translator, model.styles)
	model.browser.activePanel = domain.LocalPanel
	model.browser.localPanel.path = `C:\work\project`
	model.browser.localPanel.items = []domain.FileEntry{
		{Name: "a.log", Path: `C:\work\project\a.log`, Panel: domain.LocalPanel},
		{Name: "b.log", Path: `C:\work\project\b.log`, Panel: domain.LocalPanel},
	}
	model.setErrorStatus(errors.New("列表加载失败"))

	updated, _ := model.updateBrowser(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := updated.(*Model)

	if got.err != nil {
		t.Fatal("err != nil, want cleared after further action")
	}
	if got.status != translator.T("status.browser_ready") {
		t.Fatalf("status = %q, want browser ready", got.status)
	}
	if got.browser.localPanel.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", got.browser.localPanel.cursor)
	}
}

func TestBrowserPlainUploadDownloadKeysDoNotTriggerTransfer(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, `C:\work\project`, "~/.ssh/id_ed25519")
	model.page = pageBrowser
	model.browser = newBrowserState(translator, model.styles)
	model.browser.activePanel = domain.LocalPanel
	model.browser.localPanel.path = `C:\work\project`
	model.browser.remotePanel.path = "/tmp"
	model.browser.localPanel.items = []domain.FileEntry{
		{Name: "archive.log", Path: `C:\work\project\archive.log`, Panel: domain.LocalPanel},
	}

	updated, cmd := model.updateBrowser(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	got := updated.(*Model)
	if cmd != nil {
		t.Fatal("cmd != nil, want no transfer command for plain u")
	}
	if got.overlay != overlayNone || got.browser.pending != nil {
		t.Fatalf("overlay=%v pending=%#v, want no confirm or pending", got.overlay, got.browser.pending)
	}

	model.browser.activePanel = domain.RemotePanel
	model.browser.remotePanel.items = []domain.FileEntry{
		{Name: "archive.log", Path: "/tmp/archive.log", Panel: domain.RemotePanel},
	}
	updated, cmd = model.updateBrowser(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	got = updated.(*Model)
	if cmd != nil {
		t.Fatal("cmd != nil, want no transfer command for plain d")
	}
	if got.overlay != overlayNone || got.browser.pending != nil {
		t.Fatalf("overlay=%v pending=%#v, want no confirm or pending", got.overlay, got.browser.pending)
	}
}

func TestBrowserViewFitsViewportWidth(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, `D:\code\go\tssh`, "~/.ssh/id_ed25519")
	model.page = pageBrowser
	model.width = 80
	model.height = 24
	model.browser.connection.Username = "root"
	model.browser.connection.Host = "127.0.0.1"
	model.browser.localPanel.path = `D:\code\go\tssh`
	model.browser.remotePanel.path = "."

	view := model.viewBrowser()
	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > model.width {
			t.Fatalf("line width = %d, want <= %d: %q", lipgloss.Width(line), model.width, line)
		}
	}
}

func TestBrowserStatusAndFooterRenderSeparately(t *testing.T) {
	t.Parallel()

	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	model := NewModel(nil, translator, `D:\code\go\tssh`, "~/.ssh/id_ed25519")
	model.page = pageBrowser
	model.setInfoStatus(translator.T("status.browser_ready"))

	status := ansi.Strip(model.renderStatusBar(60))
	footer := ansi.Strip(model.renderBrowserFooter(60))
	if !strings.Contains(status, "INFO") {
		t.Fatalf("status = %q, want INFO label", status)
	}
	if !strings.Contains(footer, "enter/l") {
		t.Fatalf("footer = %q, want browser shortcuts", footer)
	}
	if strings.Contains(footer, "INFO") {
		t.Fatalf("footer = %q, want no status text in shortcut panel", footer)
	}
}

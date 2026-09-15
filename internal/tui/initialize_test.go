package tui

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"sshm/internal/repository"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestInitializeModelCreatesStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sshm.db")
	m := newInitializeModel(path, initializeNew)
	m.password.SetValue("secret")
	m.repeat.SetValue("secret")
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 's', Mod: tea.ModCtrl}))
	if cmd == nil {
		t.Fatal("初始化未开始保存")
	}
	result, ok := cmd().(initializeResultMsg)
	if !ok || result.err != nil {
		t.Fatalf("初始化结果 = %#v", result)
	}
	final, _ := updated.(initializeModel).Update(result)
	model := final.(initializeModel)
	if model.store == nil || model.result != nil {
		t.Fatalf("初始化状态错误: store=%v err=%v", model.store, model.result)
	}
	model.store.Close()
}

func TestApplicationModelSwitchesToMainWithoutQuitting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sshm.db")
	store, err := repository.Initialize(path, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	app := newApplicationModel(path, repository.Ready, "")
	app.initializing.width, app.initializing.height = 100, 30
	updated, cmd := app.Update(initializeResultMsg{store: store})
	if cmd != nil {
		t.Fatal("解锁成功后不应退出 Bubble Tea 程序")
	}
	main := updated.(applicationModel).main
	if main == nil {
		t.Fatal("解锁成功后未切换主界面")
	}
	if main.width != 100 || main.height != 30 {
		t.Fatalf("主界面未继承终端尺寸: %d×%d", main.width, main.height)
	}
}

func TestSavedPasswordFailureShowsEnvironmentMessage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sshm.db")
	store, err := repository.Initialize(path, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	store.Close()

	app := newApplicationModel(path, repository.Ready, "wrong")
	if app.initializing.err != "环境变量密码错误，请手动输入" {
		t.Fatalf("环境变量密码错误提示 = %q", app.initializing.err)
	}
}

func TestUnlockShowsSimplePasswordErrorWithoutLabel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sshm.db")
	store, err := repository.Initialize(path, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	store.Close()

	m := newInitializeModel(path, initializeUnlock)
	if view := ansi.Strip(m.View().Content); strings.Contains(view, "密码 >") || strings.Contains(view, "│   应用密码") || !m.password.Focused() {
		t.Fatalf("解锁界面不应显示标签且密码框应有焦点: %q", view)
	}
	m.password.SetValue("wrong")
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	result := cmd().(initializeResultMsg)
	updated, _ = updated.(initializeModel).Update(result)
	view := updated.(initializeModel)
	if view.err != "密码不正确" {
		t.Fatalf("错误提示 = %q", view.err)
	}
	updated, _ = view.Update(tea.KeyPressMsg(tea.Key{Text: "x"}))
	if updated.(initializeModel).err != "" {
		t.Fatalf("键盘输入后未清除错误: %q", updated.(initializeModel).err)
	}
}

func TestInitializeLayoutAndFocusCycle(t *testing.T) {
	m := newInitializeModel("", initializeNew)
	lines := strings.Split(ansi.Strip(m.viewBody("初始化应用")), "\n")
	if len(lines) < 6 || strings.Trim(lines[4], "│ ") != "" || !strings.Contains(lines[3], "应用密码") || !strings.Contains(lines[5], "重复") {
		t.Fatalf("密码与重复密码之间缺少空行: %q", lines)
	}

	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	m = updated.(initializeModel)
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	m = updated.(initializeModel)
	if !m.password.Focused() {
		t.Fatal("Tab 未从重复密码循环到密码")
	}
	updated, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}))
	m = updated.(initializeModel)
	if !m.repeat.Focused() {
		t.Fatal("Shift+Tab 未从密码循环到重复密码")
	}
}

func TestInitializeEscReturnsSilentCancel(t *testing.T) {
	updated, _ := newInitializeModel("", initializeNew).Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if !errors.Is(updated.(initializeModel).result, ErrInitializationCanceled) {
		t.Fatalf("Esc 返回错误 = %v", updated.(initializeModel).result)
	}
}

func TestUnlockShortcutChangesPasswordAndReturnsStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sshm.db")
	initial, err := repository.Initialize(path, []byte("old-password"))
	if err != nil {
		t.Fatal(err)
	}
	initial.Close()

	m := newInitializeModel(path, initializeUnlock)
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'p', Mod: tea.ModCtrl}))
	m = updated.(initializeModel)
	form := m.modal.(passwordFormModel)
	form.inputs[oldPasswordField].SetValue("old-password")
	form.inputs[newPasswordField].SetValue("new-password")
	form.inputs[repeatPasswordField].SetValue("new-password")
	updatedModal, cmd := form.Update(tea.KeyPressMsg(tea.Key{Code: 's', Mod: tea.ModCtrl}))
	if cmd == nil {
		t.Fatal("修改密码未开始保存")
	}
	m.modal = updatedModal
	msg, ok := cmd().(passwordChangedMsg)
	if !ok || msg.store == nil {
		t.Fatalf("修改密码结果错误: %#v", msg)
	}
	final, _ := m.Update(msg)
	m = final.(initializeModel)
	if m.store == nil || m.result != nil {
		t.Fatalf("修改密码后未完成解锁: store=%v err=%v", m.store, m.result)
	}
	m.store.Close()
	if store, err := repository.Unlock(path, []byte("new-password")); err != nil {
		t.Fatalf("新密码无法解锁: %v", err)
	} else {
		store.Close()
	}
}

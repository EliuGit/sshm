package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"sshm/internal/repository"
)

func TestPlaceModalPreservesBackground(t *testing.T) {
	base := strings.Join([]string{
		"背景第一行                              ",
		"背景第二行                              ",
		"背景第三行                              ",
		"背景第四行                              ",
	}, "\n")
	view := placeModal(base, "╭────╮\n│弹窗│\n╰────╯", 40, 4)
	if !strings.Contains(view, "背景第一行") || !strings.Contains(view, "弹窗") {
		t.Fatalf("覆盖结果未同时保留背景和弹窗: %q", view)
	}
	for i, line := range strings.Split(view, "\n") {
		if width := ansi.StringWidth(line); width != 40 {
			t.Fatalf("第 %d 行宽度 = %d, want 40", i+1, width)
		}
	}
}

func TestPlaceModalDoesNotShiftWideCharacterAtRightEdge(t *testing.T) {
	view := placeModal("abcdef中ghi", "XYZ", 11, 1)
	if view != "abcdXYZ ghi" {
		t.Fatalf("弹窗右边界切到宽字符后发生偏移: %q", view)
	}
}

func TestModalDoesNotOverrideMainBackground(t *testing.T) {
	styles := map[string]lipgloss.Style{
		"弹窗":   modalStyle,
		"普通输入": formInputStyle, "焦点输入": formInputFocused,
		"普通选项": formChoiceStyle, "选中选项": formChoiceSelected,
	}
	privateKeyStyles := newKeyInput().Styles()
	styles["私钥焦点"] = privateKeyStyles.Focused.Base
	styles["私钥失焦"] = privateKeyStyles.Blurred.Base
	for name, style := range styles {
		if _, ok := style.GetBackground().(lipgloss.NoColor); !ok {
			t.Fatalf("%s配置了独立背景色: %v", name, style.GetBackground())
		}
	}
	if _, ok := privateKeyStyles.Focused.CursorLine.GetBackground().(lipgloss.NoColor); !ok {
		t.Fatalf("私钥输入当前行配置了背景色: %v", privateKeyStyles.Focused.CursorLine.GetBackground())
	}
}

func TestCredentialFormLabelsMatchConnectionForm(t *testing.T) {
	form := newCredentialForm(credentialPickerModel{}, nil)
	if !strings.Contains(ansi.Strip(form.View()), "名称 >") {
		t.Fatal("凭据名称焦点缺少 > 标记")
	}
	form.focus = credentialTypeField
	if !strings.Contains(ansi.Strip(form.View()), "类型 >") {
		t.Fatal("凭据类型焦点缺少 > 标记")
	}
	form.credentialType = "key"
	form.focus = credentialContentField
	if !strings.Contains(ansi.Strip(form.View()), "私钥 >") {
		t.Fatal("私钥内容焦点缺少 > 标记")
	}
	form.privateKey.SetValue("第一行内容\n第二行内容")
	lines := strings.Split(ansi.Strip(form.privateKeyRow()), "\n")
	firstOffset := ansi.StringWidth(lines[0][:strings.Index(lines[0], "第一行内容")])
	secondOffset := ansi.StringWidth(lines[1][:strings.Index(lines[1], "第二行内容")])
	if firstOffset != secondOffset {
		t.Fatalf("私钥换行内容未对齐: %q", lines)
	}
	passwordLine := ansi.Strip(form.inputRow("密码", "密码内容", credentialContentField))
	passwordOffset := ansi.StringWidth(passwordLine[:strings.Index(passwordLine, "密码内容")])
	if firstOffset != passwordOffset {
		t.Fatalf("私钥组件未与普通输入对齐: 私钥=%d 密码=%d", firstOffset, passwordOffset)
	}
}

func TestAOpensAndEscClosesConnectionForm(t *testing.T) {
	m := model{width: 100, height: 30, connections: testConnections, searchInput: newSearchInput(), searchReady: true, selected: 2}
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "a"}))
	m = updated.(model)
	if m.modal == nil {
		t.Fatal("a 未打开新增连接弹窗")
	}
	view := ansi.Strip(m.render())
	if !strings.Contains(view, "SSHM") || !strings.Contains(view, "新增连接") {
		t.Fatalf("弹窗未覆盖在主界面上: %q", view)
	}
	lines := strings.Split(m.render(), "\n")
	if len(lines) != 30 {
		t.Fatalf("弹窗界面行数 = %d, want 30", len(lines))
	}
	for i, line := range lines {
		if width := ansi.StringWidth(line); width != 100 {
			t.Fatalf("弹窗界面第 %d 行宽度 = %d, want 100", i+1, width)
		}
	}
	updated, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	m = updated.(model)
	if cmd == nil {
		t.Fatal("Esc 未生成关闭弹窗消息")
	}
	updated, _ = m.Update(cmd())
	m = updated.(model)
	if m.modal != nil || m.selected != 2 {
		t.Fatalf("关闭弹窗后状态错误: modal=%v selected=%d", m.modal, m.selected)
	}
}

func TestPOpensStandaloneCredentialManager(t *testing.T) {
	store, err := repository.Initialize(filepath.Join(t.TempDir(), "sshm.db"), []byte("master-password"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	m := model{store: store, searchInput: newSearchInput(), searchReady: true}
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "p"}))
	m = updated.(model)
	picker, ok := m.modal.(credentialPickerModel)
	if !ok || picker.selectable {
		t.Fatalf("p 未打开独立凭据管理: %#v", m.modal)
	}
	if view := ansi.Strip(picker.View()); strings.Contains(view, "Enter 选择") || !strings.Contains(view, "a 新增") {
		t.Fatalf("独立凭据管理快捷提示错误: %q", view)
	}
	updatedModal, cmd := picker.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd != nil || updatedModal.(credentialPickerModel).selectable {
		t.Fatal("Enter 不应在独立凭据管理中选择凭据")
	}
	_, cmd = picker.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if cmd == nil {
		t.Fatal("Esc 未生成关闭凭据管理消息")
	}
}

func TestEnterOpensConnectionMenuAndEditForm(t *testing.T) {
	m := model{connections: []connectionRow{{id: 7, name: "开发机", host: "dev", port: "22", username: "root", auth: "密码", credID: 3, credName: "凭据"}}, searchInput: newSearchInput(), searchReady: true}
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = updated.(model)
	if _, ok := m.modal.(menuModel); !ok {
		t.Fatalf("Enter 未打开连接菜单: %#v", m.modal)
	}
	menu := m.modal.(menuModel)
	for range 2 {
		next, _ := menu.Update(tea.KeyPressMsg(tea.Key{Text: "down"}))
		menu = next.(menuModel)
	}
	next, _ := menu.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	form, ok := next.(connectionFormModel)
	if !ok || !form.editing || form.connectionID != 7 || form.connectionName.Value() != "开发机" {
		t.Fatalf("编辑表单未复用连接数据: %#v", next)
	}
}

func TestConnectionMenuOpensFileTransfer(t *testing.T) {
	menu := newMenu(nil, connectionRow{name: "开发机"})
	updated, cmd := menu.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if cmd != nil {
		t.Fatal("选择文件传输不应立即建立远程连接")
	}
	updated, cmd = updated.(menuModel).Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	transfer, ok := updated.(transferModel)
	if !ok || cmd != nil || transfer.connection.name != "开发机" {
		t.Fatalf("文件传输入口状态错误: value=%#v cmd=%v", updated, cmd)
	}
}

func TestConnectionMenuIsCompactAndPositionedBelowSelection(t *testing.T) {
	menu := newMenu(nil, connectionRow{name: "开发机"})
	view := ansi.Strip(menu.View())
	if strings.Contains(view, "开发机") || strings.Contains(view, "Enter") || lipgloss.Width(view) != menuWidth {
		t.Fatalf("连接菜单不够紧凑: %q", view)
	}
	if lines := strings.Split(view, "\n"); len(lines) != len(menuItems)+2 || !strings.Contains(lines[1], "› 连接shell") || !strings.Contains(lines[1], "") {
		t.Fatalf("连接菜单选中行发生错位: %q", view)
	}
	x, y := menuPosition(30, 2)
	if x != 4 || y != 9 {
		t.Fatalf("菜单位置 = (%d,%d), want (4,9)", x, y)
	}
}

func TestDeleteConnectionConfirmationRequiresY(t *testing.T) {
	confirm := newDeleteConfirm(nil, connectionRow{name: "开发机"})
	updated, cmd := confirm.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd != nil {
		t.Fatal("Enter 不应触发删除")
	}
	if _, ok := updated.(deleteConfirmModel); !ok {
		t.Fatalf("Enter 后确认框类型错误: %#v", updated)
	}
	view := ansi.Strip(confirm.View())
	if strings.Contains(view, "确认操作") || strings.Contains(view, "y/Enter") || !strings.Contains(view, "y 确认") {
		t.Fatalf("确认框快捷提示错误: %q", view)
	}
}

func TestFormDialogUsesFormLayout(t *testing.T) {
	view := ansi.Strip(formDialog("消息", "操作完成", "Enter/Esc 关闭", plainStyle, modalTitleStyle))
	lines := strings.Split(view, "\n")
	if !strings.Contains(view, "消息") || strings.Contains(view, "操作结果") || !strings.Contains(view, "操作完成") || !strings.Contains(view, "Enter/Esc 关闭") {
		t.Fatalf("消息框未使用表单头尾布局: %q", view)
	}
	separatorCount := 0
	for _, line := range lines {
		if strings.Contains(line, "操作完成") || strings.Contains(line, "Enter/Esc 关闭") {
			continue
		}
		if strings.Contains(line, "─") {
			separatorCount++
		}
	}
	if separatorCount < 4 {
		t.Fatalf("消息框缺少头尾分隔线: %q", view)
	}
	for _, line := range lines[1 : len(lines)-1] {
		if strings.Trim(line, "│ ") == "" {
			t.Fatalf("弹窗正文前后仍有空行: %q", view)
		}
	}
}

func TestConnectionMenuPreparesShellCredential(t *testing.T) {
	store, err := repository.Initialize(filepath.Join(t.TempDir(), "sshm.db"), []byte("master-password"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	credential, err := store.CreateCredential(repository.NewCredential{Name: "开发机密码", Type: "passwd", Content: []byte("secret")})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := store.CreateConnection(repository.NewConnection{Name: "开发机", Host: "dev.example.com", Port: 22, Username: "root", CredentialID: credential.ID})
	if err != nil {
		t.Fatal(err)
	}
	menu := newMenu(store, connectionRow{id: connection.ID})
	_, cmd := menu.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("连接 Shell 未读取连接凭据")
	}
	prepared, ok := cmd().(shellPreparedMsg)
	if !ok {
		t.Fatal("连接 Shell 未返回 shellPreparedMsg")
	}
	defer clear(prepared.credential)
	if prepared.err != nil || prepared.connection.ID != connection.ID || string(prepared.credential) != "secret" {
		t.Fatalf("Shell 准备结果 = %#v, %v", prepared.connection, prepared.err)
	}
}

func TestConnectionFormValidation(t *testing.T) {
	form := newConnectionForm(nil)
	updated, cmd := form.Update(tea.KeyPressMsg(tea.Key{Code: 's', Mod: tea.ModCtrl}))
	form = updated.(connectionFormModel)
	if cmd != nil || form.err == "" {
		t.Fatalf("空表单校验结果: cmd=%v err=%q", cmd, form.err)
	}
}

func TestCtrlCClearsFocusedFormInput(t *testing.T) {
	clearKey := tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl})
	form := newConnectionForm(nil)
	form.connectionName.SetValue("连接")
	form.focus = connectionNameField
	updated, _ := form.Update(clearKey)
	form = updated.(connectionFormModel)
	if form.connectionName.Value() != "" {
		t.Fatalf("Ctrl+C 未清空连接名称: %q", form.connectionName.Value())
	}

	manager := credentialPickerModel{}
	credentialForm := newCredentialForm(manager, nil)
	credentialForm.credentialType = "key"
	credentialForm.privateKey.SetValue("private-key")
	credentialForm.focus = credentialContentField
	updatedModal, _ := credentialForm.Update(clearKey)
	credentialForm = updatedModal.(credentialFormModel)
	if credentialForm.privateKey.Value() != "" {
		t.Fatalf("Ctrl+C 未清空私钥输入: %q", credentialForm.privateKey.Value())
	}
}

func TestConnectionFormSavesConnection(t *testing.T) {
	store, err := repository.Initialize(filepath.Join(t.TempDir(), "sshm.db"), []byte("master-password"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	credential, err := store.CreateCredential(repository.NewCredential{Name: "开发机密码", Type: "passwd", Content: []byte("secret")})
	if err != nil {
		t.Fatal(err)
	}
	form := newConnectionForm(store)
	form.connectionName.SetValue("开发机")
	form.host.SetValue("dev.example.com")
	form.username.SetValue("root")
	form.credential = &credential
	form.remark.SetValue("测试")
	updated, cmd := form.Update(tea.KeyPressMsg(tea.Key{Code: 's', Mod: tea.ModCtrl}))
	form = updated.(connectionFormModel)
	if cmd == nil || !form.saving {
		t.Fatal("有效表单未开始保存")
	}
	if _, ok := cmd().(connectionCreatedMsg); !ok {
		t.Fatal("保存未返回 connectionCreatedMsg")
	}
	connections, err := store.ListConnections()
	if err != nil || len(connections) != 1 || connections[0].Name != "开发机" {
		t.Fatalf("保存后的连接 = %#v, %v", connections, err)
	}
}

func TestConnectionFormAcceptsMultilinePrivateKey(t *testing.T) {
	store, err := repository.Initialize(filepath.Join(t.TempDir(), "sshm.db"), []byte("master-password"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	parent := newConnectionForm(store)
	manager := newCredentialPicker(store, parent)
	form := newCredentialForm(manager, nil)
	form.credentialType = "key"
	form.name.SetValue("部署私钥")
	form.privateKey.SetValue("-----BEGIN OPENSSH PRIVATE KEY-----\nline 2\n-----END OPENSSH PRIVATE KEY-----")
	if lines := strings.Count(form.View(), "\n") + 1; lines > minHeight {
		t.Fatalf("私钥表单高度 = %d, 超过最小终端高度 %d", lines, minHeight)
	}
	updated, cmd := form.Update(tea.KeyPressMsg(tea.Key{Code: 's', Mod: tea.ModCtrl}))
	form = updated.(credentialFormModel)
	if cmd == nil || !form.saving {
		t.Fatalf("私钥表单未开始保存: err=%q", form.err)
	}
	created, ok := cmd().(credentialSavedMsg)
	if !ok {
		t.Fatal("私钥表单保存失败")
	}
	updated, _ = form.Update(created)
	manager = updated.(credentialPickerModel)
	updated, _ = manager.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	connectionForm := updated.(connectionFormModel)
	if connectionForm.credential == nil || connectionForm.credential.Type != "key" {
		t.Fatalf("新凭据未返回连接表单: %#v", connectionForm.credential)
	}
}

func TestCredentialPickerSelectsExistingOrOpensCreateForm(t *testing.T) {
	store, err := repository.Initialize(filepath.Join(t.TempDir(), "sshm.db"), []byte("master-password"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	credential, err := store.CreateCredential(repository.NewCredential{Name: "共享密码", Type: "passwd", Content: []byte("secret")})
	if err != nil {
		t.Fatal(err)
	}
	parent := newConnectionForm(store)
	parent.connectionName.SetValue("保留的连接草稿")
	picker := newCredentialPicker(store, parent)
	parent.credential = &credential
	picker = newCredentialPicker(store, parent)
	selectedCredential, ok := picker.currentCredential()
	if !ok || selectedCredential.ID != credential.ID {
		t.Fatalf("当前凭据未定位到选中项: selected=%d credential=%#v", picker.selected, selectedCredential)
	}
	selected, _ := picker.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	form := selected.(connectionFormModel)
	if form.credential == nil || form.credential.ID != credential.ID || form.connectionName.Value() != "保留的连接草稿" {
		t.Fatalf("未选中已有凭据: %#v", form.credential)
	}
	if !strings.Contains(ansi.Strip(form.View()), "[密码]共享密码") {
		t.Fatal("连接表单未按类型和名称显示凭据")
	}
	if view := ansi.Strip(parent.View()); strings.Contains(view, "切换") || strings.Contains(view, "Enter 选择") || !strings.Contains(view, "Ctrl+S 保存") {
		t.Fatalf("连接表单快捷提示未更新: %q", view)
	}
	created, _ := picker.Update(tea.KeyPressMsg(tea.Key{Text: "a"}))
	if _, ok := created.(credentialFormModel); !ok {
		t.Fatalf("未打开新增凭据表单: %#v", created)
	}
}

func TestCredentialPickerFiltersAndRestrictsDeletion(t *testing.T) {
	store, err := repository.Initialize(filepath.Join(t.TempDir(), "sshm.db"), []byte("master-password"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	used, err := store.CreateCredential(repository.NewCredential{Name: "共享密码", Type: "passwd", Content: []byte("secret")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateConnection(repository.NewConnection{Name: "开发机", Host: "dev.example.com", Port: 22, Username: "root", CredentialID: used.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateCredential(repository.NewCredential{Name: "部署私钥", Type: "key", Content: []byte("key")}); err != nil {
		t.Fatal(err)
	}
	picker := newCredentialPicker(store, newConnectionForm(store))
	if view := ansi.Strip(picker.View()); !strings.Contains(view, "[密码] 共享密码 | 1") || !strings.Contains(view, "[私钥] 部署私钥 | 0") || !strings.Contains(view, "按 / 搜索凭据...") || !strings.Contains(view, "Enter 选择 | a 新增 | e 编辑 | d 删除 | Esc 返回") {
		t.Fatalf("凭据列表格式错误: %q", view)
	}
	picker.searchInput.SetValue("共享")
	if len(picker.visibleCredentials()) != 1 {
		t.Fatal("凭据名称筛选失败")
	}
	updated, _ := picker.Update(tea.KeyPressMsg(tea.Key{Text: "d"}))
	picker = updated.(credentialPickerModel)
	if picker.err == "" || picker.confirmDelete {
		t.Fatal("允许删除有关联的凭据")
	}
	picker.searchInput.SetValue("部署")
	updated, _ = picker.Update(tea.KeyPressMsg(tea.Key{Text: "d"}))
	picker = updated.(credentialPickerModel)
	if !picker.confirmDelete {
		t.Fatal("删除未进入确认状态")
	}
	if view := ansi.Strip(picker.View()); strings.Contains(view, "选择或管理登录凭据") || !strings.Contains(view, "删除凭据") || !strings.Contains(view, "y 确认") {
		t.Fatalf("凭据确认框样式或快捷提示错误: %q", view)
	}
	updated, _ = picker.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	picker = updated.(credentialPickerModel)
	if !picker.confirmDelete || len(picker.visibleCredentials()) != 1 {
		t.Fatal("Enter 不应确认删除凭据")
	}
	updated, _ = picker.Update(tea.KeyPressMsg(tea.Key{Text: "y"}))
	picker = updated.(credentialPickerModel)
	if len(picker.visibleCredentials()) != 0 {
		t.Fatal("确认后未删除无关联凭据")
	}
}

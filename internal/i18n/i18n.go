package i18n

import (
	"errors"
	"fmt"
	"sshm/internal/domain"
	"strings"
)

type Translator struct {
	language string
	messages map[string]string
}

func New(language string) (*Translator, error) {
	switch language {
	case "", "en":
		return &Translator{language: "en", messages: english}, nil
	case "zh-CN":
		return &Translator{language: "zh-CN", messages: chinese}, nil
	default:
		return nil, fmt.Errorf("unsupported language %q", language)
	}
}

func (t *Translator) Language() string {
	if t == nil {
		return "en"
	}
	return t.language
}

func (t *Translator) T(key string, args ...any) string {
	value := key
	if t != nil {
		if translated, ok := t.messages[key]; ok {
			value = translated
		} else if translated, ok := english[key]; ok {
			value = translated
		}
	} else if translated, ok := english[key]; ok {
		value = translated
	}
	if len(args) == 0 {
		return value
	}
	return fmt.Sprintf(value, args...)
}

func (t *Translator) Error(err error) string {
	if err == nil {
		return ""
	}
	var translatable domain.TranslatableError
	if errors.As(err, &translatable) {
		return t.T(translatable.TranslationKey(), translatable.TranslationArgs()...)
	}
	if errors.Is(err, domain.ErrConnectionNotFound) {
		return t.T("err.connection_not_found")
	}
	if errors.Is(err, domain.ErrConnectionSecretNotFound) {
		return t.T("err.connection_secret_not_found")
	}
	message := err.Error()
	if strings.HasPrefix(message, "connection ") && strings.HasSuffix(message, " not found") {
		return t.T("err.connection_name_not_found", strings.TrimSuffix(strings.TrimPrefix(message, "connection "), " not found"))
	}
	if strings.HasPrefix(message, "connection name ") && strings.HasSuffix(message, " is duplicated") {
		return t.T("err.connection_name_duplicated", strings.TrimSuffix(strings.TrimPrefix(message, "connection name "), " is duplicated"))
	}
	if key := errorKey(message); key != "" {
		return t.T(key)
	}
	return message
}

func errorKey(message string) string {
	switch {
	case message == "name is required":
		return "err.name_required"
	case message == "host is required":
		return "err.host_required"
	case message == "username is required":
		return "err.username_required"
	case message == "port must be between 1 and 65535":
		return "err.port_range"
	case message == "password is required":
		return "err.password_required"
	case message == "unsupported auth type":
		return "err.unsupported_auth_type"
	case message == "connection not found":
		return "err.connection_not_found"
	case message == "connection name is required":
		return "err.connection_name_required"
	case message == "interactive shell requires a terminal":
		return "err.interactive_terminal"
	case strings.HasPrefix(message, "unsupported language "):
		return "err.unsupported_language"
	case message == "database path is required":
		return "err.database_path_required"
	case message == "group name is required":
		return "err.group_name_required"
	case message == "group is required":
		return "err.group_required"
	default:
		return ""
	}
}

var english = map[string]string{
	"app.error_prefix":                   "Error: %v\n",
	"app.ssh_session_failed":             "SSH session failed: %v\n",
	"status.ready":                       "Ready",
	"status.no_matches":                  "No connections match the current filter",
	"status.no_connections":              "No connections yet. Press c-n to add one",
	"status.found_connections":           "Found %d connection(s)",
	"status.connections_ready":           "%d connection(s) ready",
	"status.delete_cancelled":            "Delete cancelled",
	"status.search_ready":                "Search ready",
	"status.search_cleared":              "Search cleared",
	"status.filtered_connections":        "Filtered to %d connection(s)",
	"status.type_to_filter":              "Type to filter connections",
	"status.connecting_shell":            "Connecting to %s...",
	"status.connecting_browser":          "Connecting to %s for file workspace...",
	"status.loading_browser":             "Loading file browser...",
	"status.browser_ready":               "File browser ready",
	"status.connection_deleted":          "Connection deleted",
	"status.cancelled":                   "Cancelled",
	"status.created_connection":          "Created connection %s",
	"status.updated_connection":          "Updated connection %s",
	"status.transfer_cancelled":          "Transfer cancelled",
	"status.returned_connections":        "Returned to connections",
	"status.focus_local_upload":          "Move focus to local panel to upload",
	"status.focus_remote_download":       "Move focus to remote panel to download",
	"status.uploading":                   "Uploading",
	"status.uploaded":                    "Uploaded %s",
	"status.downloading":                 "Downloading",
	"status.downloaded":                  "Downloaded %s",
	"status.group_filter_cleared":        "Returned to all connections",
	"status.group_created":               "Created group %s",
	"status.group_renamed":               "Renamed group to %s",
	"status.group_deleted":               "Group deleted",
	"status.connection_moved_group":      "Moved to %s",
	"status.import_done":                 "Import complete: %d created, %d updated, %d skipped",
	"status.shell_connect_failed":        "Connection %s failed: %s",
	"status.browser_connect_failed":      "File workspace for %s failed: %s",
	"home.search_placeholder":            "Search name / host / user / description",
	"home.search_prompt":                 "Search: ",
	"home.title":                         "SSH Manage",
	"home.connections":                   "Connections",
	"home.details":                       "Details",
	"home.empty":                         "No saved connections yet.",
	"home.empty_action":                  "Create your first profile with %s.",
	"home.no_match_action":               "Press %s to clear search or %s to refine it.",
	"home.delete_title":                  "Delete connection?",
	"home.delete_desc":                   "This removes the saved profile from the local workspace.",
	"home.delete_keys":                   "Press %s to cancel, %s or %s to confirm.",
	"home.help_title":                    "Home shortcuts",
	"home.help_move":                     "Move selection",
	"home.help_start_search":             "Start search",
	"home.help_search_clear":             "Exit search or clear filter",
	"home.help_open_shell":               "Open shell",
	"home.help_open_files":               "Open file workspace",
	"home.help_create":                   "Create connection",
	"home.help_edit":                     "Edit selected connection",
	"home.help_delete":                   "Delete selected connection",
	"home.help_groups":                   "Manage groups",
	"home.help_move_group":               "Move selected connection to group",
	"home.help_import":                   "Import ssh_config",
	"home.help_quit":                     "Quit application",
	"home.help_close":                    "Press ? or esc to close",
	"home.footer_move":                   "move",
	"home.footer_shell":                  "shell",
	"home.footer_files":                  "files",
	"home.footer_add":                    "add",
	"home.footer_edit":                   "edit",
	"home.footer_delete":                 "delete",
	"home.footer_groups":                 "groups",
	"home.footer_move_group":             "move group",
	"home.footer_import":                 "import",
	"home.footer_search":                 "search",
	"home.footer_clear":                  "clear",
	"home.footer_quit":                   "quit",
	"home.table_address":                 "Address",
	"home.table_last_used":               "Last Used",
	"home.table_description":             "Description",
	"home.detail_auth":                   "Auth",
	"home.detail_group":                  "Group",
	"home.auth_password":                 "Password",
	"home.auth_private_key":              "Private Key",
	"home.never":                         "never",
	"home.just_now":                      "just now",
	"home.minutes_ago":                   "%dm ago",
	"home.hours_ago":                     "%dh ago",
	"home.days_ago":                      "%dd ago",
	"form.add_title":                     "Add Connection",
	"form.add_subtitle":                  "Create a new SSH connection profile",
	"form.edit_title":                    "Edit Connection",
	"form.edit_subtitle":                 "Update connection details and authentication",
	"form.name":                          "Name",
	"form.host":                          "Host",
	"form.port":                          "Port",
	"form.username":                      "Username",
	"form.description":                   "Description",
	"form.password":                      "Password",
	"form.key_path":                      "Key Path",
	"form.auth_type":                     "Auth Type",
	"form.private_key":                   "Private Key",
	"form.password_keep_hint":            "Leave blank to keep the current password",
	"form.save":                          "Save",
	"form.cancel":                        "Cancel",
	"form.shortcut_move":                 "move",
	"form.shortcut_next_save":            "next/save",
	"form.shortcut_save":                 "save",
	"form.shortcut_cancel":               "cancel",
	"browser.local":                      "Local",
	"browser.remote":                     "Remote",
	"browser.title":                      "SSH/SCP file transfer",
	"browser.subtitle":                   "%s@%s",
	"browser.filter_label":               "(Filter: %s)",
	"browser.empty":                      "<empty>",
	"browser.loading":                    "loading...",
	"browser.path":                       "Path",
	"browser.filter":                     "Filter",
	"browser.overwrite":                  "Overwrite target?",
	"browser.yes":                        "Yes",
	"browser.no":                         "No",
	"browser.active_path":                "Active: %s    Path: %s",
	"group.filter_title":                 "Select Group",
	"group.filter_desc":                  "Mode: filter the home list by group",
	"group.move_title":                   "Move Group",
	"group.move_desc":                    "Mode: move the selected connection to a group",
	"group.delete_title":                 "Delete group?",
	"group.delete_desc":                  "Delete %s and move its connections to Ungrouped.",
	"group.all":                          "All",
	"group.ungrouped":                    "Ungrouped",
	"group.empty":                        "No groups yet",
	"group.name":                         "Group name",
	"group.column_name":                  "Group",
	"group.column_count":                 "Count",
	"group.input_prompt":                 "Name: ",
	"group.create":                       "Create group",
	"group.rename":                       "Rename group",
	"group.system_item_locked":           "System group cannot be changed",
	"group.shortcut_choose":              "choose",
	"group.shortcut_create":              "create",
	"group.shortcut_rename":              "rename",
	"group.shortcut_delete":              "delete",
	"group.shortcut_confirm":             "confirm",
	"group.shortcut_cancel":              "cancel",
	"group.shortcut_close":               "close",
	"import.title":                       "Import Connections",
	"import.subtitle":                    "Read Host entries from OpenSSH ssh_config.",
	"import.path_prompt":                 "Path: ",
	"import.loading":                     "Parsing...",
	"import.preview_subtitle":            "space cycles action. Conflicts default to skip.",
	"import.shortcut_preview":            "preview",
	"import.shortcut_back":               "quit",
	"import.shortcut_move":               "move",
	"import.shortcut_action":             "action",
	"import.shortcut_import":             "import",
	"import.empty":                       "No importable Host entries found",
	"import.conflict":                    "conflict",
	"import.skipped":                     "skipped",
	"import.action_create":               "create",
	"import.action_skip":                 "skip",
	"import.action_update":               "update",
	"import.action_copy":                 "copy",
	"shortcut.switch":                    "switch",
	"shortcut.move":                      "move",
	"shortcut.open":                      "open",
	"shortcut.up":                        "up",
	"shortcut.top":                       "top",
	"shortcut.filter":                    "filter",
	"shortcut.goto":                      "goto",
	"shortcut.upload":                    "upload",
	"shortcut.download":                  "download",
	"shortcut.refresh":                   "refresh",
	"shortcut.clear_filter":              "clear filter",
	"shortcut.back":                      "back",
	"shortcut.confirm":                   "confirm",
	"shortcut.cancel":                    "cancel",
	"shortcut.choose":                    "choose",
	"cli.error_prefix":                   "Error: %v\n",
	"cli.ls.header_name":                 "NAME",
	"cli.ls.header_host":                 "HOST",
	"cli.ls.header_description":          "DESCRIPTION",
	"cli.run.start":                      "[%s] Running command\n",
	"cli.run.failed":                     "[%s] Command failed: %v\n",
	"cli.run.success":                    "[%s] Command succeeded\n",
	"cli.upload.start":                   "[%s] Uploading %s -> %s\n",
	"cli.upload.check_failed":            "[%s] Failed to check remote target: %v\n",
	"cli.upload.exists":                  "[%s] Upload failed: remote target already exists; use -f/--force to overwrite\n",
	"cli.upload.failed":                  "[%s] Upload failed: %v\n",
	"cli.upload.success":                 "[%s] Upload succeeded\n",
	"cli.download.start":                 "[%s] Downloading %s -> %s\n",
	"cli.download.check_failed":          "[%s] Failed to check local target: %v\n",
	"cli.download.exists":                "[%s] Download failed: local target already exists; use -f/--force to overwrite\n",
	"cli.download.failed":                "[%s] Download failed: %v\n",
	"cli.download.success":               "[%s] Download succeeded\n",
	"cli.err.unknown_command":            "unknown command",
	"cli.err.read_script_failed":         "failed to read script file: %v",
	"cli.err.invalid_local_path":         "invalid local path",
	"cli.err.invalid_remote_path":        "invalid remote path",
	"cli.err.unknown_subcommand":         "unknown subcommand %q",
	"cli.err.list_unsupported_arg":       "ls does not support argument %q",
	"cli.err.run_unsupported_arg":        "run does not support argument %q; use -- before the remote command",
	"cli.err.group_not_found":            "group %q not found",
	"cli.err.name_required":              "-n/--name is required",
	"cli.err.command_file_conflict":      "remote command and --file cannot be used together",
	"cli.err.command_required":           "remote command or --file is required",
	"cli.err.transfer_unsupported_arg":   "%s does not support argument %q",
	"cli.err.download_batch_not_allowed": "download does not allow multiple names",
	"cli.err.local_required":             "--local/-l is required",
	"cli.err.remote_required":            "--remote/-r is required",
	"cli.err.option_value_required":      "%s requires a value",
	"cli.help": `sshm headless mode

Usage:
  sshm ls [--group <group>] [--filter <query>]
  sshm run -n <name[,name...]> -- <command>
  sshm run -n <name[,name...]> --file <script-file>
  sshm upload -n <name[,name...]> --local <local-file-or-dir> --remote <remote-dir> [-f]
  sshm download -n <name> --remote <remote-file-or-dir> --local <local-dir> [-f]
  sshm version

Options:
  -g, --group      Filter by exact group name; use "ungrouped" for ungrouped items
      --filter     Fuzzy match name / host / user / description / group
  -n, --name       Connection name; use commas for multiple names
      --file       Local script file used by run
  -l, --local      Local file or directory path
  -r, --remote     Remote file or directory path
  -f, --force      Overwrite existing upload/download targets
  -ff, --fail-fast Stop batch run/upload at the first failure

Notes:
  download does not allow multiple names.
  Starting without arguments enters the TUI.
  version prints the current build version.
`,
	"err.connection_not_found":        "connection not found",
	"err.connection_secret_not_found": "connection secret not found",
	"err.connection_name_not_found":   "connection %s not found",
	"err.connection_name_duplicated":  "connection name %s is duplicated",
	"err.connection_name_required":    "connection name is required",
	"err.name_required":               "name is required",
	"err.host_required":               "host is required",
	"err.username_required":           "username is required",
	"err.port_range":                  "port must be between 1 and 65535",
	"err.password_required":           "password is required",
	"err.unsupported_auth_type":       "unsupported auth type",
	"err.interactive_terminal":        "interactive shell requires a terminal",
	"err.unsupported_language":        "unsupported language",
	"err.database_path_required":      "database path is required",
	"err.group_name_required":         "group name is required",
	"err.group_required":              "group is required",
}

var chinese = map[string]string{
	"app.error_prefix":                   "错误：%v\n",
	"app.ssh_session_failed":             "SSH 会话失败：%v\n",
	"status.ready":                       "就绪",
	"status.no_matches":                  "没有连接匹配当前过滤条件",
	"status.no_connections":              "还没有连接。按 c-n 新建",
	"status.found_connections":           "找到 %d 个连接",
	"status.connections_ready":           "%d 个连接可用",
	"status.delete_cancelled":            "已取消删除",
	"status.search_ready":                "搜索已就绪",
	"status.search_cleared":              "搜索已清空",
	"status.filtered_connections":        "已过滤到 %d 个连接",
	"status.type_to_filter":              "输入内容以过滤连接",
	"status.connecting_shell":            "正在连接 %s...",
	"status.connecting_browser":          "正在为 %s 连接文件工作区...",
	"status.loading_browser":             "正在加载文件浏览器...",
	"status.browser_ready":               "文件浏览器已就绪",
	"status.connection_deleted":          "连接已删除",
	"status.cancelled":                   "已取消",
	"status.created_connection":          "已创建连接 %s",
	"status.updated_connection":          "已更新连接 %s",
	"status.transfer_cancelled":          "传输已取消",
	"status.returned_connections":        "已返回连接列表",
	"status.focus_local_upload":          "请先切换到本地面板再上传",
	"status.focus_remote_download":       "请先切换到远端面板再下载",
	"status.uploading":                   "正在上传",
	"status.uploaded":                    "已上传 %s",
	"status.downloading":                 "正在下载",
	"status.downloaded":                  "已下载 %s",
	"status.group_filter_cleared":        "已返回全部连接",
	"status.group_created":               "已创建分组 %s",
	"status.group_renamed":               "已重命名分组为 %s",
	"status.group_deleted":               "分组已删除",
	"status.connection_moved_group":      "已移动到 %s",
	"status.import_done":                 "导入完成：新增 %d，更新 %d，跳过 %d",
	"status.shell_connect_failed":        "连接 %s 失败：%s",
	"status.browser_connect_failed":      "连接 %s 的文件工作区失败：%s",
	"home.search_placeholder":            "搜索名称 / 主机 / 用户 / 描述",
	"home.search_prompt":                 "搜索：",
	"home.title":                         "SSH 管理器",
	"home.connections":                   "连接",
	"home.details":                       "详情",
	"home.empty":                         "还没有保存的连接。",
	"home.empty_action":                  "使用 %s 创建第一个连接。",
	"home.no_match_action":               "按 %s 清空搜索，或按 %s 重新过滤。",
	"home.delete_title":                  "删除连接？",
	"home.delete_desc":                   "这会从本地工作区移除已保存的连接配置。",
	"home.delete_keys":                   "按 %s 取消，按 %s 或 %s 确认。",
	"home.help_title":                    "主页快捷键",
	"home.help_move":                     "移动选择",
	"home.help_start_search":             "开始搜索",
	"home.help_search_clear":             "退出搜索或清空过滤",
	"home.help_open_shell":               "打开 Shell",
	"home.help_open_files":               "打开文件工作区",
	"home.help_create":                   "创建连接",
	"home.help_edit":                     "编辑选中连接",
	"home.help_delete":                   "删除选中连接",
	"home.help_groups":                   "管理分组",
	"home.help_move_group":               "移动选中连接到分组",
	"home.help_import":                   "导入 ssh_config",
	"home.help_quit":                     "退出应用",
	"home.help_close":                    "按 ? 或 esc 关闭",
	"home.footer_move":                   "移动",
	"home.footer_shell":                  "Shell",
	"home.footer_files":                  "文件",
	"home.footer_add":                    "新增",
	"home.footer_edit":                   "编辑",
	"home.footer_delete":                 "删除",
	"home.footer_groups":                 "分组",
	"home.footer_move_group":             "移动分组",
	"home.footer_import":                 "导入",
	"home.footer_search":                 "搜索",
	"home.footer_clear":                  "清空",
	"home.footer_quit":                   "退出",
	"home.table_address":                 "地址",
	"home.table_last_used":               "最近使用",
	"home.table_description":             "描述",
	"home.detail_auth":                   "认证",
	"home.detail_group":                  "分组",
	"home.auth_password":                 "密码",
	"home.auth_private_key":              "私钥",
	"home.never":                         "从未",
	"home.just_now":                      "刚刚",
	"home.minutes_ago":                   "%d 分钟前",
	"home.hours_ago":                     "%d 小时前",
	"home.days_ago":                      "%d 天前",
	"form.add_title":                     "新增连接",
	"form.add_subtitle":                  "创建新的 SSH 连接配置",
	"form.edit_title":                    "编辑连接",
	"form.edit_subtitle":                 "更新连接详情和认证方式",
	"form.name":                          "名称",
	"form.host":                          "主机",
	"form.port":                          "端口",
	"form.username":                      "用户名",
	"form.description":                   "描述",
	"form.password":                      "密码",
	"form.key_path":                      "密钥路径",
	"form.auth_type":                     "认证方式",
	"form.private_key":                   "私钥",
	"form.password_keep_hint":            "留空时保留当前密码",
	"form.save":                          "保存",
	"form.cancel":                        "取消",
	"form.shortcut_move":                 "移动",
	"form.shortcut_next_save":            "下一项/保存",
	"form.shortcut_save":                 "保存",
	"form.shortcut_cancel":               "取消",
	"browser.local":                      "本地",
	"browser.remote":                     "远端",
	"browser.title":                      "SSH/SCP 文件传输",
	"browser.subtitle":                   "%s@%s",
	"browser.filter_label":               "（过滤：%s）",
	"browser.empty":                      "<空>",
	"browser.loading":                    "加载中...",
	"browser.path":                       "路径",
	"browser.filter":                     "过滤",
	"browser.overwrite":                  "覆盖目标？",
	"browser.confirm_source":             "来源：%s",
	"browser.confirm_target":             "目标：%s",
	"browser.yes":                        "是",
	"browser.no":                         "否",
	"browser.active_path":                "当前：%s    路径：%s",
	"group.filter_title":                 "选择组",
	"group.filter_desc":                  "当前：选择分组以过滤主列表",
	"group.move_title":                   "移动组",
	"group.move_desc":                    "当前：将选中连接移动到目标分组",
	"group.delete_title":                 "删除分组？",
	"group.delete_desc":                  "删除 %s，并将其连接移回未分组。",
	"group.all":                          "全部",
	"group.ungrouped":                    "未分组",
	"group.empty":                        "暂无分组",
	"group.name":                         "分组名称",
	"group.column_name":                  "分组",
	"group.column_count":                 "连接数",
	"group.input_prompt":                 "名称：",
	"group.create":                       "新建分组",
	"group.rename":                       "重命名分组",
	"group.system_item_locked":           "系统分组不能修改",
	"group.shortcut_choose":              "选择",
	"group.shortcut_create":              "新建",
	"group.shortcut_rename":              "重命名",
	"group.shortcut_delete":              "删除",
	"group.shortcut_confirm":             "确认",
	"group.shortcut_cancel":              "取消",
	"group.shortcut_close":               "关闭",
	"import.title":                       "导入连接",
	"import.subtitle":                    "从 OpenSSH ssh_config 读取 Host 配置。",
	"import.path_prompt":                 "路径：",
	"import.loading":                     "解析中...",
	"import.preview_subtitle":            "space 切换动作。冲突项默认跳过。",
	"import.shortcut_preview":            "预览",
	"import.shortcut_back":               "退出",
	"import.shortcut_move":               "移动",
	"import.shortcut_action":             "动作",
	"import.shortcut_import":             "导入",
	"import.empty":                       "没有可导入的 Host 配置",
	"import.conflict":                    "冲突",
	"import.skipped":                     "已跳过",
	"import.action_create":               "新建",
	"import.action_skip":                 "跳过",
	"import.action_update":               "覆盖",
	"import.action_copy":                 "副本",
	"shortcut.switch":                    "切换",
	"shortcut.move":                      "移动",
	"shortcut.open":                      "打开",
	"shortcut.up":                        "上级",
	"shortcut.top":                       "顶部",
	"shortcut.filter":                    "过滤",
	"shortcut.goto":                      "跳转",
	"shortcut.upload":                    "上传",
	"shortcut.download":                  "下载",
	"shortcut.refresh":                   "刷新",
	"shortcut.clear_filter":              "取消过滤",
	"shortcut.back":                      "返回",
	"shortcut.confirm":                   "确认",
	"shortcut.cancel":                    "取消",
	"shortcut.choose":                    "选择",
	"cli.error_prefix":                   "错误：%v\n",
	"cli.ls.header_name":                 "名称",
	"cli.ls.header_host":                 "主机",
	"cli.ls.header_description":          "描述",
	"cli.run.start":                      "[%s] 开始执行命令\n",
	"cli.run.failed":                     "[%s] 执行失败：%v\n",
	"cli.run.success":                    "[%s] 执行成功\n",
	"cli.upload.start":                   "[%s] 开始上传 %s -> %s\n",
	"cli.upload.check_failed":            "[%s] 检查远端目标失败：%v\n",
	"cli.upload.exists":                  "[%s] 上传失败：远端目标已存在，使用 -f/--force 覆盖\n",
	"cli.upload.failed":                  "[%s] 上传失败：%v\n",
	"cli.upload.success":                 "[%s] 上传成功\n",
	"cli.download.start":                 "[%s] 开始下载 %s -> %s\n",
	"cli.download.check_failed":          "[%s] 检查本地目标失败：%v\n",
	"cli.download.exists":                "[%s] 下载失败：本地目标已存在，使用 -f/--force 覆盖\n",
	"cli.download.failed":                "[%s] 下载失败：%v\n",
	"cli.download.success":               "[%s] 下载成功\n",
	"cli.err.unknown_command":            "未知命令",
	"cli.err.read_script_failed":         "读取脚本文件失败：%v",
	"cli.err.invalid_local_path":         "本地路径无效",
	"cli.err.invalid_remote_path":        "远端路径无效",
	"cli.err.unknown_subcommand":         "未知子命令 %q",
	"cli.err.list_unsupported_arg":       "ls 不支持参数 %q",
	"cli.err.run_unsupported_arg":        "run 不支持参数 %q，请使用 -- 分隔远端命令",
	"cli.err.group_not_found":            "分组 %q 不存在",
	"cli.err.name_required":              "缺少 -n/--name",
	"cli.err.command_file_conflict":      "远端命令和 --file 不能同时使用",
	"cli.err.command_required":           "缺少远端命令或 --file",
	"cli.err.transfer_unsupported_arg":   "%s 不支持参数 %q",
	"cli.err.download_batch_not_allowed": "download 不允许批量名称",
	"cli.err.local_required":             "缺少 --local/-l",
	"cli.err.remote_required":            "缺少 --remote/-r",
	"cli.err.option_value_required":      "%s 缺少值",
	"cli.help": `sshm 无头模式

用法：
  sshm ls [--group <分组>] [--filter <关键字>]
  sshm run -n <名称[,名称...]> -- <命令>
  sshm run -n <名称[,名称...]> --file <脚本文件>
  sshm upload -n <名称[,名称...]> --local <本地文件或目录> --remote <远端目录> [-f]
  sshm download -n <名称> --remote <远端文件或目录> --local <本地目录> [-f]
  sshm version

选项：
  -g, --group      按精确分组名过滤，未分组可使用 "ungrouped" 或 "未分组"
      --filter     按名称 / 主机 / 用户 / 描述 / 分组进行模糊匹配
  -n, --name       连接名称，多个名称使用英文逗号分隔
      --file       run 使用的本地脚本文件
  -l, --local      本地文件或目录路径
  -r, --remote     远端文件或目录路径
  -f, --force      upload/download 目标存在时覆盖
  -ff, --fail-fast 批量 run/upload 遇到首个失败即停止

说明：
  download 不允许批量名称。
  无参数启动时进入 TUI。
  version 显示当前构建版本。
`,
	"err.connection_not_found":        "连接不存在",
	"err.connection_secret_not_found": "连接密码不存在",
	"err.connection_name_not_found":   "连接 %s 不存在",
	"err.connection_name_duplicated":  "连接名称 %s 重复",
	"err.connection_name_required":    "连接名称不能为空",
	"err.name_required":               "名称不能为空",
	"err.host_required":               "主机不能为空",
	"err.username_required":           "用户名不能为空",
	"err.port_range":                  "端口必须在 1 到 65535 之间",
	"err.password_required":           "密码不能为空",
	"err.unsupported_auth_type":       "不支持的认证方式",
	"err.interactive_terminal":        "交互式 Shell 需要终端环境",
	"err.unsupported_language":        "不支持的语言",
	"err.database_path_required":      "数据库路径不能为空",
	"err.group_name_required":         "分组名称不能为空",
	"err.group_required":              "分组不能为空",
}

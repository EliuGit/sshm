# sshm

`sshm` 是一个基于 Go 与 Bubble Tea 的终端 SSH 工作台，用于集中管理 SSH 连接、打开远端 Shell，并在本地与远端之间传输文件。

![sshm screenshot](image/main.png)
![sshm screenshot](image/scp.png)

## 特性

- 终端 TUI：连接列表、详情面板、分组管理、搜索过滤
- 连接管理：新增、编辑、删除、最近使用排序
- 导入能力：从 OpenSSH `ssh_config` 导入连接并支持分组注释
- 认证方式：密码认证、私钥认证
- 远端访问：从主界面直接进入交互式 SSH Shell
- 文件工作区：本地 / 远端双栏浏览、上传、下载、覆盖确认、进度显示
- 无头模式：支持 `ls`、`run`、`upload`、`download`，便于脚本或自动化调用
- 本地安全：SQLite 持久化、密码加密存储、`known_hosts` 指纹记录
- 国际化：支持 `en` 与 `zh-CN`


## 环境要求

- Go `1.23+`
- 可交互终端环境
- 可访问的 SSH 目标主机
- 支持 CGO 的本地 C 编译工具链（项目依赖 `github.com/mattn/go-sqlite3`）

## 快速开始

```bash
go build ./...
go run .
```

无参数启动进入 TUI。

## 无头模式

```bash
sshm ls
sshm ls --group 生产
sshm run -n prod -- "uname -a"
sshm run -n prod,web --file ./deploy.sh
sshm upload -n prod -l ./dist -r /tmp/
sshm download -n prod -r /var/log/app.log -l ./logs/
```

说明：

- `ls`：列出连接，可按分组或关键字过滤
- `run`：执行远端命令或脚本，支持批量目标
- `upload`：上传本地文件或目录到远端，支持批量目标
- `download`：下载远端文件或目录到本地，仅支持单目标
- 默认目标已存在即失败，添加 `-f` / `--force` 可覆盖
- 批量 `run` / `upload` 可用 `--fail-fast` 在首个失败时停止

## TUI 快捷键

### 主界面

| 按键 | 说明 |
| --- | --- |
| `j` / `k` | 移动选中项 |
| `/` | 搜索 |
| `enter` | 打开远端 Shell |
| `c-o` | 打开文件工作区 |
| `c-n` | 新建连接 |
| `c-e` | 编辑连接 |
| `c-d` | 删除连接 |
| `g` | 选择分组过滤 |
| `c-g` | 移动连接到分组 |
| `i` | 导入 `ssh_config` |
| `?` | 查看帮助 |
| `q` / `c-c` | 退出 |

### 文件工作区

| 按键 | 说明 |
| --- | --- |
| `tab` | 切换本地 / 远端面板 |
| `j` / `k` | 移动 |
| `enter` / `l` / `→` | 进入目录 |
| `h` / `backspace` / `←` | 返回上级 |
| `/` | 过滤当前面板 |
| `:` | 跳转路径 |
| `u` | 上传 |
| `d` | 下载 |
| `r` | 刷新 |
| `q` | 返回主界面 |

## 配置与数据目录

首次启动会自动创建配置目录：

```text
Linux:   ~/.config/sshm/ 或 $XDG_CONFIG_HOME/sshm/
Windows: %AppData%\sshm\
macOS:   ~/Library/Application Support/sshm/
```

默认生成：

- `config.toml`
- `data/sshm.db`
- `app.key`
- `known_hosts`

默认配置：

```toml
[app]
language = "en"

[storage]
database_path = "data/sshm.db"

[ssh]
default_private_key_path = "~/.ssh/id_rsa"
```

## 导入 `ssh_config`

支持读取 OpenSSH `Host` 配置，并支持通过注释为后续 Host 指定分组：

```sshconfig
# sshm:group=生产环境
Host prod-web
  HostName 10.0.0.1
  User deploy
  Port 2222
  IdentityFile ~/.ssh/id_ed25519
```

## 已知限制

- 暂不支持远端新建目录、删除、重命名等文件管理操作
- `ConnectionService` 已具备测试连接能力，但当前 TUI 还未暴露入口
- 文件传输更适合类 Unix 远端环境

## 安全说明

- 不要提交 `sshm.db`、日志、`app.key`、`known_hosts` 或本地配置
- SSH 凭据、主机名、加密后的密码数据都应视为敏感信息

## 开发

```bash
gofmt -w <files>
go test ./...
go build ./...
```

// Package ssh 提供交互式 SSH Shell 能力。
package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"sshm/internal/i18n"
	"sshm/internal/repository"

	"github.com/charmbracelet/x/term"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const dialTimeout = 10 * time.Second

// Shell 是可交给 Bubble Tea tea.Exec 执行的一次性交互式 SSH 命令。
// Run 返回后会清空内部凭据，不能重复执行同一个实例。
type Shell struct {
	connection repository.Connection
	credential []byte
	stdin      io.Reader
	stdout     io.Writer
	stderr     io.Writer
}

// NewShell 创建交互式 SSH 命令并接管 credential；调用方之后不得再使用该切片。
func NewShell(connection repository.Connection, credential []byte) *Shell {
	return &Shell{
		connection: connection,
		credential: credential,
		stdin:      os.Stdin,
		stdout:     os.Stdout,
		stderr:     os.Stderr,
	}
}

// Close 清空 Shell 持有的凭据；可重复调用。
func (s *Shell) Close() error {
	clear(s.credential)
	return nil
}

// SetStdin 设置交互式 Shell 的标准输入。
func (s *Shell) SetStdin(input io.Reader) { s.stdin = input }

// SetStdout 设置交互式 Shell 的标准输出。
func (s *Shell) SetStdout(output io.Writer) { s.stdout = output }

// SetStderr 设置交互式 Shell 的标准错误输出。
func (s *Shell) SetStderr(output io.Writer) { s.stderr = output }

// Run 连接远端、切换本地终端模式并阻塞到 Shell 退出。
func (s *Shell) Run() error {
	defer s.Close()
	input, inputOK := s.stdin.(interface{ Fd() uintptr })
	output, outputOK := s.stdout.(interface{ Fd() uintptr })
	if !inputOK || !outputOK || !term.IsTerminal(input.Fd()) || !term.IsTerminal(output.Fd()) {
		return errors.New(i18n.T("SSH shell requires an interactive terminal"))
	}

	client, err := dial(context.Background(), s.connection, s.credential)
	if err != nil {
		return err
	}
	defer client.Close()
	return s.open(client, input.Fd(), output.Fd())
}

// dial 建立 Shell 与 SFTP 共用的 SSH 连接，并在认证结束后立即清空明文凭据。
func dial(ctx context.Context, connection repository.Connection, credential []byte) (*gossh.Client, error) {
	defer clear(credential)
	ctx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()
	auth, err := authMethods(connection.Credential, credential)
	if err != nil {
		return nil, err
	}
	hostKeyCallback, err := knownHostsCallback()
	if err != nil {
		return nil, err
	}
	config := &gossh.ClientConfig{
		User:            connection.Username,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
		Timeout:         dialTimeout,
	}
	address := net.JoinHostPort(connection.Host, strconv.Itoa(connection.Port))
	network, err := (&net.Dialer{Timeout: dialTimeout}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.T("Connect to %s", address), err)
	}
	stopCancel := context.AfterFunc(ctx, func() { _ = network.Close() })
	sshConnection, channels, requests, err := gossh.NewClientConn(network, address, config)
	stopped := stopCancel()
	if err != nil {
		_ = network.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%s: %w", i18n.T("Connect to %s", address), err)
	}
	if !stopped || ctx.Err() != nil {
		_ = sshConnection.Close()
		return nil, ctx.Err()
	}
	return gossh.NewClient(sshConnection, channels, requests), nil
}

// open 严格按“停止输入转发、恢复终端、恢复 TUI”的顺序结束会话，
// 避免远端退出后遗留的 stdin 读取协程吞掉 TUI 的首个按键。
func (s *Shell) open(client *gossh.Client, inputFd, outputFd uintptr) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("Failed to create SSH session"), err)
	}
	defer session.Close()

	if err := resetInput(inputFd); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("Failed to clean up terminal input"), err)
	}
	restoreTerminal, err := prepareTerminal(inputFd, outputFd)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("Failed to switch terminal mode"), err)
	}
	defer restoreTerminal()

	width, height, err := term.GetSize(outputFd)
	if err != nil {
		width, height = 80, 24
	}
	modes := gossh.TerminalModes{
		gossh.ECHO:          1,
		gossh.TTY_OP_ISPEED: 14400,
		gossh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty(terminalName(), height, width, modes); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("Failed to request remote PTY"), err)
	}

	remoteInput, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("Failed to create remote standard input"), err)
	}
	forwarder, err := newInputForwarder(s.stdin, remoteInput)
	if err != nil {
		return err
	}
	session.Stdout = s.stdout
	session.Stderr = s.stderr

	stopResize := watchWindowChanges(outputFd, func(cols, rows int) error {
		return session.WindowChange(rows, cols)
	})
	defer stopResize()

	if err := session.Shell(); err != nil {
		_ = forwarder.Stop()
		return fmt.Errorf("%s: %w", i18n.T("Failed to start remote shell"), err)
	}
	forwarder.Start()
	waitErr := session.Wait()
	stopErr := forwarder.Stop()
	if exitErr, ok := waitErr.(*gossh.ExitError); ok && exitErr.ExitStatus() == 0 {
		waitErr = nil
	}
	return errors.Join(waitErr, stopErr)
}

// authMethods 根据数据库凭据类型生成密码或私钥认证方法。
func authMethods(credentialType string, credential []byte) ([]gossh.AuthMethod, error) {
	if len(credential) == 0 {
		return nil, errors.New(i18n.T("SSH credential is empty"))
	}
	switch credentialType {
	case "passwd":
		password := string(credential)
		return []gossh.AuthMethod{
			gossh.Password(password),
			gossh.KeyboardInteractive(func(_ string, _ string, questions []string, _ []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range answers {
					answers[i] = password
				}
				return answers, nil
			}),
		}, nil
	case "key":
		signer, err := gossh.ParsePrivateKey(credential)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", i18n.T("Failed to parse SSH private key"), err)
		}
		return []gossh.AuthMethod{gossh.PublicKeys(signer)}, nil
	default:
		return nil, errors.New(i18n.T("Unsupported SSH credential type: %s", credentialType))
	}
}

func terminalName() string {
	if name := strings.TrimSpace(os.Getenv("TERM")); name != "" {
		return name
	}
	return "xterm-256color"
}

// knownHostsCallback 首次连接时记录主机公钥，后续连接拒绝不匹配的公钥。
func knownHostsCallback() (gossh.HostKeyCallback, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.T("Failed to read user home directory"), err)
	}
	path := filepath.Join(homeDir, ".ssh", "known_hosts")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.T("Failed to create SSH config directory"), err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.T("Failed to open known_hosts"), err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.T("Failed to close known_hosts"), err)
	}
	callback, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.T("Failed to read known_hosts"), err)
	}
	return func(hostname string, remote net.Addr, key gossh.PublicKey) error {
		err := callback(hostname, remote, key)
		if err == nil {
			return nil
		}
		var keyErr *knownhosts.KeyError
		if !errors.As(err, &keyErr) || len(keyErr.Want) != 0 {
			return err
		}
		knownHost, openErr := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
		if openErr != nil {
			return openErr
		}
		_, writeErr := fmt.Fprintln(knownHost, knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key))
		return errors.Join(writeErr, knownHost.Close())
	}, nil
}

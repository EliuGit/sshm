package ssh

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"sort"
	"sshm/internal/app"
	"sshm/internal/domain"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/term"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const defaultDialTimeout = 10 * time.Second

type Client struct {
	knownHostsPath        string
	defaultPrivateKeyPath string
}

type reusableSession struct {
	owner    *Client
	conn     domain.Connection
	password string

	mu     sync.Mutex
	client *gossh.Client
	closed bool
}

func NewClient(knownHostsPath string, defaultPrivateKeyPath string) *Client {
	if strings.TrimSpace(defaultPrivateKeyPath) == "" {
		defaultPrivateKeyPath = "~/.ssh/id_rsa"
	}
	return &Client{knownHostsPath: knownHostsPath, defaultPrivateKeyPath: defaultPrivateKeyPath}
}

func (c *Client) OpenSession(conn domain.Connection, password string) (app.RemoteSession, error) {
	client, err := c.dial(conn, password)
	if err != nil {
		return nil, err
	}
	return &reusableSession{
		owner:    c,
		conn:     conn,
		password: password,
		client:   client,
	}, nil
}

func (c *Client) ProbeShell(conn domain.Connection, password string) error {
	session, err := c.OpenSession(conn, password)
	if err != nil {
		return err
	}
	return session.Close()
}

func (c *Client) OpenShell(conn domain.Connection, password string) error {
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		return fmt.Errorf("interactive shell requires a terminal")
	}

	client, err := c.dial(conn, password)
	if err != nil {
		return err
	}
	defer client.Close()

	_, err = openShellOnClient(client)
	return err
}

func (s *reusableSession) OpenShell() error {
	return s.withShellReconnect(func(client *gossh.Client) (bool, error) {
		return openShellOnClient(client)
	})
}

func (s *reusableSession) ListRemote(targetPath string) ([]domain.FileEntry, string, error) {
	result, err := withReconnectResult(s, func(client *gossh.Client) (listRemoteResult, error) {
		entries, currentPath, err := listRemoteWithClient(client, targetPath)
		return listRemoteResult{entries: entries, currentPath: currentPath}, err
	})
	return result.entries, result.currentPath, err
}

func (s *reusableSession) PathExists(targetPath string) (bool, error) {
	return withReconnectResult(s, func(client *gossh.Client) (bool, error) {
		return pathExistsWithClient(client, targetPath)
	})
}

func (s *reusableSession) Upload(localPath string, remoteDir string, progress func(domain.TransferProgress)) error {
	return s.withReconnect(func(client *gossh.Client) error {
		return uploadPath(client, localPath, remoteDir, progress)
	})
}

func (s *reusableSession) Download(remotePath string, localDir string, progress func(domain.TransferProgress)) error {
	return s.withReconnect(func(client *gossh.Client) error {
		return downloadPath(client, remotePath, localDir, progress)
	})
}

func (s *reusableSession) HomeDir() (string, error) {
	return withReconnectResult(s, func(client *gossh.Client) (string, error) {
		return homeDirWithClient(client)
	})
}

func (s *reusableSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	return s.closeClientLocked()
}

type listRemoteResult struct {
	entries     []domain.FileEntry
	currentPath string
}

func withReconnectResult[T any](s *reusableSession, op func(*gossh.Client) (T, error)) (T, error) {
	var zero T

	client, err := s.acquireClient()
	if err != nil {
		return zero, err
	}

	result, err := op(client)
	if !isConnectionError(err) {
		return result, err
	}

	client, err = s.reconnectOrReuse(client)
	if err != nil {
		return zero, err
	}
	result, err = op(client)
	if isConnectionError(err) {
		_ = s.invalidateClient(client)
	}
	return result, err
}

func (s *reusableSession) withReconnect(op func(*gossh.Client) error) error {
	_, err := withReconnectResult(s, func(client *gossh.Client) (struct{}, error) {
		return struct{}{}, op(client)
	})
	return err
}

func (s *reusableSession) withShellReconnect(op func(*gossh.Client) (bool, error)) error {
	client, err := s.acquireClient()
	if err != nil {
		return err
	}

	started, err := op(client)
	if err == nil {
		return nil
	}
	if started || !isConnectionError(err) {
		if isConnectionError(err) {
			_ = s.invalidateClient(client)
		}
		return err
	}

	client, err = s.reconnectOrReuse(client)
	if err != nil {
		return err
	}
	started, err = op(client)
	if isConnectionError(err) {
		_ = s.invalidateClient(client)
	}
	_ = started
	return err
}

func (s *reusableSession) acquireClient() (*gossh.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, app.ErrRemoteSessionClosed
	}
	if err := s.ensureClientLocked(); err != nil {
		return nil, err
	}
	return s.client, nil
}

func (s *reusableSession) reconnectOrReuse(current *gossh.Client) (*gossh.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, app.ErrRemoteSessionClosed
	}
	if s.client != nil && s.client != current {
		return s.client, nil
	}
	if err := s.redialLocked(); err != nil {
		return nil, err
	}
	return s.client, nil
}

func (s *reusableSession) invalidateClient(current *gossh.Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client != current {
		return nil
	}
	return s.closeClientLocked()
}

func (s *reusableSession) ensureClientLocked() error {
	if s.client != nil {
		return nil
	}
	client, err := s.owner.dial(s.conn, s.password)
	if err != nil {
		return err
	}
	s.client = client
	return nil
}

func (s *reusableSession) redialLocked() error {
	_ = s.closeClientLocked()
	return s.ensureClientLocked()
}

func (s *reusableSession) closeClientLocked() error {
	if s.client == nil {
		return nil
	}
	err := s.client.Close()
	s.client = nil
	if err != nil && !errors.Is(err, net.ErrClosed) {
		return err
	}
	return nil
}

func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || errors.Is(err, app.ErrRemoteSessionClosed) {
		return true
	}
	message := strings.ToLower(err.Error())
	for _, fragment := range []string{
		"broken pipe",
		"connection reset by peer",
		"use of closed network connection",
		"connection closed",
		"failed to create session",
	} {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}

func openShellOnClient(client *gossh.Client) (bool, error) {
	session, err := client.NewSession()
	if err != nil {
		return false, err
	}
	defer session.Close()

	if err := resetInteractiveInput(); err != nil {
		return false, fmt.Errorf("failed to reset terminal input: %w", err)
	}

	restoreTerminal, err := prepareInteractiveTerminal(os.Stdin.Fd(), os.Stdout.Fd())
	if err != nil {
		return false, fmt.Errorf("failed to switch terminal to raw mode: %w", err)
	}
	defer restoreTerminal()

	width, height, err := term.GetSize(os.Stdout.Fd())
	if err != nil {
		width = 80
		height = 24
	}

	modes := gossh.TerminalModes{
		gossh.ECHO:          1,
		gossh.TTY_OP_ISPEED: 14400,
		gossh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", height, width, modes); err != nil {
		return false, fmt.Errorf("failed to request PTY: %w", err)
	}

	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	stopResize := watchWindowChanges(os.Stdout.Fd(), func(cols, rows int) error {
		return session.WindowChange(rows, cols)
	})
	defer stopResize()

	if err := session.Shell(); err != nil {
		return false, fmt.Errorf("failed to start shell: %w", err)
	}

	if err := session.Wait(); err != nil {
		if exitErr, ok := err.(*gossh.ExitError); ok && exitErr.ExitStatus() == 0 {
			return true, nil
		}
		return true, err
	}
	return true, nil
}

func (c *Client) RunCommand(conn domain.Connection, password string, command string, stdout io.Writer, stderr io.Writer) error {
	client, err := c.dial(conn, password)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	session.Stdout = stdout
	session.Stderr = stderr
	return session.Run("sh -lc " + shellQuote(command))
}

func (c *Client) HomeDir(conn domain.Connection, password string) (string, error) {
	client, err := c.dial(conn, password)
	if err != nil {
		return "", err
	}
	defer client.Close()

	return homeDirWithClient(client)
}

func (c *Client) ListRemote(conn domain.Connection, password string, targetPath string) ([]domain.FileEntry, string, error) {
	client, err := c.dial(conn, password)
	if err != nil {
		return nil, "", err
	}
	defer client.Close()

	return listRemoteWithClient(client, targetPath)
}

func (c *Client) PathExists(conn domain.Connection, password string, targetPath string) (bool, error) {
	client, err := c.dial(conn, password)
	if err != nil {
		return false, err
	}
	defer client.Close()

	return pathExistsWithClient(client, targetPath)
}

func homeDirWithClient(client *gossh.Client) (string, error) {
	out, err := runRemoteCommand(client, "pwd")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func listRemoteWithClient(client *gossh.Client, targetPath string) ([]domain.FileEntry, string, error) {
	currentPath, err := resolveRemotePath(client, targetPath)
	if err != nil {
		return nil, "", err
	}

	script := fmt.Sprintf(
		"cd %s && find . -mindepth 1 -maxdepth 1 -printf '%%P\\t%%y\\t%%s\\t%%T@\\n'",
		shellQuote(currentPath),
	)
	out, err := runRemoteCommand(client, script)
	if err != nil {
		return nil, "", err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	entries := make([]domain.FileEntry, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 4 {
			continue
		}
		size, _ := strconv.ParseInt(parts[2], 10, 64)
		epoch, _ := strconv.ParseFloat(parts[3], 64)
		entries = append(entries, domain.FileEntry{
			Name:    parts[0],
			Path:    joinRemotePath(currentPath, parts[0]),
			Size:    size,
			ModTime: time.Unix(int64(epoch), 0),
			IsDir:   parts[1] == "d",
			Panel:   domain.RemotePanel,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, currentPath, nil
}

func pathExistsWithClient(client *gossh.Client, targetPath string) (bool, error) {
	out, err := runRemoteCommand(client, fmt.Sprintf("if [ -e %s ]; then printf yes; else printf no; fi", shellQuote(targetPath)))
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "yes", nil
}

func (c *Client) Upload(conn domain.Connection, password string, localPath string, remoteDir string, progress func(domain.TransferProgress)) error {
	client, err := c.dial(conn, password)
	if err != nil {
		return err
	}
	defer client.Close()
	return uploadPath(client, localPath, remoteDir, progress)
}

func (c *Client) Download(conn domain.Connection, password string, remotePath string, localDir string, progress func(domain.TransferProgress)) error {
	client, err := c.dial(conn, password)
	if err != nil {
		return err
	}
	defer client.Close()
	return downloadPath(client, remotePath, localDir, progress)
}

func (c *Client) dial(conn domain.Connection, password string) (*gossh.Client, error) {
	authMethods, err := c.buildAuthMethods(conn, password)
	if err != nil {
		return nil, err
	}
	hostKeyCallback, err := newHostKeyCallback(c.knownHostsPath)
	if err != nil {
		return nil, err
	}
	config := &gossh.ClientConfig{
		User:            conn.Username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         defaultDialTimeout,
	}
	address := fmt.Sprintf("%s:%d", conn.Host, conn.Port)
	return gossh.Dial("tcp", address, config)
}

func (c *Client) buildAuthMethods(conn domain.Connection, password string) ([]gossh.AuthMethod, error) {
	switch conn.AuthType {
	case domain.AuthTypePassword:
		pass := strings.TrimSpace(password)
		if pass == "" {
			return nil, fmt.Errorf("password is empty")
		}
		return []gossh.AuthMethod{
			gossh.Password(pass),
			gossh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for idx := range questions {
					answers[idx] = pass
				}
				return answers, nil
			}),
		}, nil
	case domain.AuthTypePrivateKey:
		keyPath := c.validPrivateKeyPath(conn.PrivateKeyPath)
		keyBytes, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key %q: %w", keyPath, err)
		}
		signer, err := gossh.ParsePrivateKey(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key %q: %w", keyPath, err)
		}
		return []gossh.AuthMethod{gossh.PublicKeys(signer)}, nil
	default:
		return nil, fmt.Errorf("unsupported auth type: %s", conn.AuthType)
	}
}

func (c *Client) validPrivateKeyPath(input string) string {
	keyPath := strings.TrimSpace(input)
	if keyPath == "" {
		keyPath = c.defaultPrivateKeyPath
	}
	keyPath = expandPath(keyPath)
	if info, err := os.Stat(keyPath); err == nil && !info.IsDir() {
		return keyPath
	}
	return expandPath(c.defaultPrivateKeyPath)
}

func expandPath(pathValue string) string {
	if pathValue == "~" || strings.HasPrefix(pathValue, "~/") || strings.HasPrefix(pathValue, "~\\") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return pathValue
		}
		if pathValue == "~" {
			return homeDir
		}
		return filepath.Join(homeDir, pathValue[2:])
	}
	return pathValue
}

func newHostKeyCallback(knownHostsPath string) (gossh.HostKeyCallback, error) {
	if err := os.MkdirAll(filepath.Dir(knownHostsPath), 0700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(knownHostsPath, os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	file.Close()

	baseCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, err
	}

	callback := func(hostname string, remote net.Addr, key gossh.PublicKey) error {
		err := baseCallback(hostname, remote, key)
		if err == nil {
			return nil
		}
		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) {
			if len(keyErr.Want) == 0 {
				line := knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key)
				appendErr := appendKnownHost(knownHostsPath, line)
				if appendErr != nil {
					return appendErr
				}
				return nil
			}
		}
		return err
	}
	return callback, nil
}

func appendKnownHost(path string, line string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(line + "\n")
	return err
}

func resolveRemotePath(client *gossh.Client, targetPath string) (string, error) {
	if strings.TrimSpace(targetPath) == "" {
		targetPath = "."
	}
	out, err := runRemoteCommand(client, fmt.Sprintf("cd %s && pwd -P", shellQuote(targetPath)))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func joinRemotePath(basePath string, name string) string {
	if basePath == "" {
		return path.Clean("/" + name)
	}
	return path.Clean(path.Join(basePath, name))
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func runRemoteCommand(client *gossh.Client, script string) ([]byte, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	out, err := session.CombinedOutput("sh -lc " + shellQuote(script))
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return nil, fmt.Errorf(msg)
		}
		return nil, err
	}
	return out, nil
}

func uploadPath(client *gossh.Client, localPath string, remoteDir string, progress func(domain.TransferProgress)) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	writer, err := session.StdinPipe()
	if err != nil {
		return err
	}
	reader, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	cmd := fmt.Sprintf("scp %s-t -- %s", recursiveFlag(info.IsDir()), shellQuote(remoteDir))
	if err := session.Start(cmd); err != nil {
		return err
	}
	ackReader := bufio.NewReader(reader)
	if err := readScpAck(ackReader); err != nil {
		_ = writer.Close()
		_ = session.Wait()
		return err
	}

	if info.IsDir() {
		err = sendDirectory(ackReader, writer, localPath, info, progress)
	} else {
		err = sendFile(ackReader, writer, localPath, info, progress)
	}
	if err == nil && info.IsDir() {
		_, err = fmt.Fprint(writer, "E\n")
		if err == nil {
			err = readScpAck(ackReader)
		}
	}
	_ = writer.Close()
	if waitErr := session.Wait(); err == nil {
		err = waitErr
	}
	return err
}

func downloadPath(client *gossh.Client, remotePath string, localDir string, progress func(domain.TransferProgress)) error {
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return err
	}
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		return err
	}
	reader := bufio.NewReader(stdout)
	cmd := fmt.Sprintf("scp -r -f -- %s", shellQuote(remotePath))
	if err := session.Start(cmd); err != nil {
		return err
	}
	if _, err := stdin.Write([]byte{0}); err != nil {
		return err
	}
	stack := []string{localDir}
	for {
		code, line, err := readScpLine(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			_ = stdin.Close()
			_ = session.Wait()
			return err
		}
		switch code {
		case 'T':
			if _, err := stdin.Write([]byte{0}); err != nil {
				return err
			}
		case 'D':
			mode, _, name, err := parseScpHeader(line)
			if err != nil {
				return err
			}
			targetDir := filepath.Join(stack[len(stack)-1], name)
			if err := os.MkdirAll(targetDir, mode); err != nil {
				return err
			}
			stack = append(stack, targetDir)
			if _, err := stdin.Write([]byte{0}); err != nil {
				return err
			}
		case 'C':
			mode, size, name, err := parseScpHeader(line)
			if err != nil {
				return err
			}
			targetPath := filepath.Join(stack[len(stack)-1], name)
			if _, err := stdin.Write([]byte{0}); err != nil {
				return err
			}
			if err := receiveScpFile(reader, targetPath, mode, size, progress); err != nil {
				return err
			}
			if _, err := stdin.Write([]byte{0}); err != nil {
				return err
			}
		case 'E':
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
			if _, err := stdin.Write([]byte{0}); err != nil {
				return err
			}
		case 0:
			continue
		case 1, 2:
			return fmt.Errorf(strings.TrimSpace(line))
		default:
			return fmt.Errorf("unexpected SCP response: %q %s", code, line)
		}
	}
	_ = stdin.Close()
	return session.Wait()
}

func recursiveFlag(isDir bool) string {
	if isDir {
		return "-r "
	}
	return ""
}

func readScpAck(reader *bufio.Reader) error {
	code, err := reader.ReadByte()
	if err != nil {
		return err
	}
	switch code {
	case 0:
		return nil
	case 1, 2:
		line, _ := reader.ReadString('\n')
		return fmt.Errorf(strings.TrimSpace(line))
	default:
		return fmt.Errorf("unexpected SCP ack: %d", code)
	}
}

func readScpLine(reader *bufio.Reader) (byte, string, error) {
	code, err := reader.ReadByte()
	if err != nil {
		return 0, "", err
	}
	if code == 0 {
		return 0, "", nil
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return 0, "", err
	}
	return code, strings.TrimSuffix(line, "\n"), nil
}

func parseScpHeader(line string) (os.FileMode, int64, string, error) {
	parts := strings.SplitN(line, " ", 3)
	if len(parts) != 3 {
		return 0, 0, "", fmt.Errorf("invalid SCP header: %q", line)
	}
	modeValue, err := strconv.ParseUint(parts[0], 8, 32)
	if err != nil {
		return 0, 0, "", err
	}
	size, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, "", err
	}
	return os.FileMode(modeValue), size, parts[2], nil
}

func receiveScpFile(reader *bufio.Reader, targetPath string, mode os.FileMode, size int64, progress func(domain.TransferProgress)) error {
	file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer file.Close()
	if progress != nil {
		progress(domain.TransferProgress{Path: filepath.Base(targetPath), Bytes: 0, Total: size})
	}
	if _, err := copyWithProgressN(file, reader, size, filepath.Base(targetPath), progress); err != nil {
		return err
	}
	end, err := reader.ReadByte()
	if err != nil {
		return err
	}
	if end != 0 {
		line, _ := reader.ReadString('\n')
		return fmt.Errorf("invalid SCP file trailer: %d %s", end, strings.TrimSpace(line))
	}
	return nil
}

func sendDirectory(reader *bufio.Reader, writer io.Writer, localPath string, info os.FileInfo, progress func(domain.TransferProgress)) error {
	if _, err := fmt.Fprintf(writer, "D%04o 0 %s\n", info.Mode().Perm(), info.Name()); err != nil {
		return err
	}
	if err := readScpAck(reader); err != nil {
		return err
	}
	items, err := os.ReadDir(localPath)
	if err != nil {
		return err
	}
	for _, item := range items {
		itemPath := filepath.Join(localPath, item.Name())
		itemInfo, err := item.Info()
		if err != nil {
			return err
		}
		if itemInfo.IsDir() {
			if err := sendDirectory(reader, writer, itemPath, itemInfo, progress); err != nil {
				return err
			}
			continue
		}
		if err := sendFile(reader, writer, itemPath, itemInfo, progress); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprint(writer, "E\n"); err != nil {
		return err
	}
	return readScpAck(reader)
}

func sendFile(reader *bufio.Reader, writer io.Writer, localPath string, info os.FileInfo, progress func(domain.TransferProgress)) error {
	if _, err := fmt.Fprintf(writer, "C%04o %d %s\n", info.Mode().Perm(), info.Size(), info.Name()); err != nil {
		return err
	}
	if err := readScpAck(reader); err != nil {
		return err
	}
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()
	if progress != nil {
		progress(domain.TransferProgress{Path: info.Name(), Bytes: 0, Total: info.Size()})
	}
	if _, err := copyWithProgress(writer, file, info.Size(), info.Name(), progress); err != nil {
		return err
	}
	if _, err := writer.Write([]byte{0}); err != nil {
		return err
	}
	return readScpAck(reader)
}

func copyWithProgress(dst io.Writer, src io.Reader, total int64, name string, progress func(domain.TransferProgress)) (int64, error) {
	buffer := make([]byte, 32*1024)
	var written int64
	for {
		nr, readErr := src.Read(buffer)
		if nr > 0 {
			nw, writeErr := dst.Write(buffer[:nr])
			if nw > 0 {
				written += int64(nw)
				if progress != nil {
					progress(domain.TransferProgress{Path: name, Bytes: written, Total: total})
				}
			}
			if writeErr != nil {
				return written, writeErr
			}
			if nw != nr {
				return written, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return written, nil
			}
			return written, readErr
		}
	}
}

func copyWithProgressN(dst io.Writer, src io.Reader, total int64, name string, progress func(domain.TransferProgress)) (int64, error) {
	if total < 0 {
		return 0, fmt.Errorf("invalid transfer size: %d", total)
	}
	bufferSize := 32 * 1024
	if total > 0 && total < int64(bufferSize) {
		bufferSize = int(total)
		if bufferSize == 0 {
			bufferSize = 1
		}
	}
	buffer := make([]byte, bufferSize)
	var written int64
	for written < total {
		remaining := total - written
		chunkSize := len(buffer)
		if remaining < int64(chunkSize) {
			chunkSize = int(remaining)
		}
		nr, readErr := src.Read(buffer[:chunkSize])
		if nr > 0 {
			nw, writeErr := dst.Write(buffer[:nr])
			if nw > 0 {
				written += int64(nw)
				if progress != nil {
					progress(domain.TransferProgress{Path: name, Bytes: written, Total: total})
				}
			}
			if writeErr != nil {
				return written, writeErr
			}
			if nw != nr {
				return written, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if readErr == io.EOF && written == total {
				break
			}
			return written, readErr
		}
		if nr == 0 {
			return written, io.ErrUnexpectedEOF
		}
	}
	return written, nil
}

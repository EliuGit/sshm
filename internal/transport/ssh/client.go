package ssh

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sshm/internal/domain"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const defaultDialTimeout = 10 * time.Second

// Client 统一承载 SSH 认证与拨号能力。
//
// shell 与文件能力会复用这里的拨号逻辑，但实现分别落在不同文件中，
// 避免文件工作区再次耦合回交互 shell 或 SCP 细节。
type Client struct {
	knownHostsPath        string
	defaultPrivateKeyPath string
}

func NewClient(knownHostsPath string, defaultPrivateKeyPath string) *Client {
	if strings.TrimSpace(defaultPrivateKeyPath) == "" {
		defaultPrivateKeyPath = "~/.ssh/id_rsa"
	}
	return &Client{knownHostsPath: knownHostsPath, defaultPrivateKeyPath: defaultPrivateKeyPath}
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
		if errors.As(err, &keyErr) && len(keyErr.Want) == 0 {
			line := knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key)
			if appendErr := appendKnownHost(knownHostsPath, line); appendErr != nil {
				return appendErr
			}
			return nil
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

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

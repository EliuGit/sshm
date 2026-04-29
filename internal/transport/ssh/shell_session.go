package ssh

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sshm/internal/app"
	"sshm/internal/domain"
	"strings"
	"sync"

	"github.com/charmbracelet/x/term"
	gossh "golang.org/x/crypto/ssh"
)

type shellSession struct {
	owner    *Client
	conn     domain.Connection
	password string

	mu     sync.Mutex
	client *gossh.Client
	closed bool
}

func (c *Client) ProbeShell(conn domain.Connection, password string) error {
	client, err := c.dial(conn, password)
	if err != nil {
		return err
	}
	return client.Close()
}

func (c *Client) OpenSession(conn domain.Connection, password string) (app.ShellSession, error) {
	client, err := c.dial(conn, password)
	if err != nil {
		return nil, err
	}
	return &shellSession{
		owner:    c,
		conn:     conn,
		password: password,
		client:   client,
	}, nil
}

func (c *Client) OpenShell(conn domain.Connection, password string) error {
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		return domain.ErrInteractiveTerminal
	}

	client, err := c.dial(conn, password)
	if err != nil {
		return err
	}
	defer client.Close()

	_, err = openShellOnClient(client)
	return err
}

func (s *shellSession) OpenShell() error {
	return s.withShellReconnect(func(client *gossh.Client) (bool, error) {
		return openShellOnClient(client)
	})
}

func (s *shellSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	return s.closeClientLocked()
}

func (s *shellSession) withShellReconnect(op func(*gossh.Client) (bool, error)) error {
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

func (s *shellSession) acquireClient() (*gossh.Client, error) {
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

func (s *shellSession) reconnectOrReuse(current *gossh.Client) (*gossh.Client, error) {
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

func (s *shellSession) invalidateClient(current *gossh.Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client != current {
		return nil
	}
	return s.closeClientLocked()
}

func (s *shellSession) ensureClientLocked() error {
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

func (s *shellSession) redialLocked() error {
	_ = s.closeClientLocked()
	return s.ensureClientLocked()
}

func (s *shellSession) closeClientLocked() error {
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
		"failure",
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

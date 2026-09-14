package ssh

import (
	"context"
	"errors"
	"fmt"
	"os"

	"sshm/internal/repository"

	"github.com/pkg/sftp"
	gossh "golang.org/x/crypto/ssh"
)

// SFTP 持有同一次远程文件会话的 SFTP 与底层 SSH 连接。
type SFTP struct {
	client *sftp.Client
	ssh    *gossh.Client
}

// NewSFTP 建立经过 known_hosts 校验的 SFTP 连接，并接管 credential。
// 返回前无论认证成功与否都会清空 credential，调用方不得再使用该切片。
func NewSFTP(ctx context.Context, connection repository.Connection, credential []byte) (*SFTP, error) {
	sshClient, err := dial(ctx, connection, credential)
	if err != nil {
		return nil, err
	}
	client, err := sftp.NewClient(sshClient)
	if err != nil {
		_ = sshClient.Close()
		return nil, fmt.Errorf("创建 SFTP 会话: %w", err)
	}
	return &SFTP{client: client, ssh: sshClient}, nil
}

// Getwd 返回建立 SFTP 会话时远程用户的工作目录。
func (c *SFTP) Getwd() (string, error) {
	return c.client.Getwd()
}

// Lstat 返回远程路径本身的信息，不跟随符号链接。
func (c *SFTP) Lstat(path string) (os.FileInfo, error) {
	return c.client.Lstat(path)
}

// ReadDir 读取远程单层目录，并支持协作式取消。
func (c *SFTP) ReadDir(ctx context.Context, path string) ([]os.FileInfo, error) {
	return c.client.ReadDirContext(ctx, path)
}

// Close 关闭 SFTP 会话及其底层 SSH 连接。
func (c *SFTP) Close() error {
	return errors.Join(c.client.Close(), c.ssh.Close())
}

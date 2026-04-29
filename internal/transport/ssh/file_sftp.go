package ssh

import (
	"errors"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"sort"
	"sshm/internal/app"
	"sshm/internal/domain"
	"strings"
	"sync"

	"github.com/pkg/sftp"
	gossh "golang.org/x/crypto/ssh"
)

// fileSession 是 transport 层对单个远端文件工作区会话的实现。
//
// 设计说明：
// 1. transport 只负责 SSH/SFTP 连接、重连与远端文件语义，不感知 UI 的过滤、选中和刷新状态。
// 2. 所有远端文件失败都会在这里统一包成 domain.FileError，向上游暴露稳定的错误模型。
// 3. 会话内部允许“连接错误后重拨重试一次”，但不会扩散到 app/ui 去重复维护这套补偿逻辑。
type fileSession struct {
	owner    *Client
	conn     domain.Connection
	password string

	mu         sync.Mutex
	sshClient  *gossh.Client
	sftpClient *sftp.Client
	closed     bool
}

type listRemoteResult struct {
	entries     []domain.FileEntry
	currentPath string
}

func (c *Client) OpenFileSession(conn domain.Connection, password string) (app.FileSession, error) {
	sshClient, sftpClient, err := c.dialSFTP(conn, password)
	if err != nil {
		return nil, err
	}
	return &fileSession{
		owner:      c,
		conn:       conn,
		password:   password,
		sshClient:  sshClient,
		sftpClient: sftpClient,
	}, nil
}

func (s *fileSession) ListRemote(targetPath string) ([]domain.FileEntry, string, error) {
	result, err := withFileClientResult(s, func(client *sftp.Client) (listRemoteResult, error) {
		return listRemoteWithSFTP(client, targetPath)
	})
	return result.entries, result.currentPath, domain.WrapFileError(domain.RemotePanel, domain.FileOpList, remotePathContext(targetPath, "."), "", err)
}

func (s *fileSession) PathExists(targetPath string) (bool, error) {
	exists, err := withFileClientResult(s, func(client *sftp.Client) (bool, error) {
		resolved, err := resolveRemotePath(client, targetPath)
		if err != nil {
			if isSFTPNotExist(err) {
				return false, nil
			}
			return false, err
		}
		_, err = client.Stat(resolved)
		if err == nil {
			return true, nil
		}
		if isSFTPNotExist(err) {
			return false, nil
		}
		return false, err
	})
	return exists, domain.WrapFileError(domain.RemotePanel, domain.FileOpStat, remotePathContext(targetPath, "."), "", err)
}

func (s *fileSession) Mkdir(targetPath string) error {
	return domain.WrapFileError(domain.RemotePanel, domain.FileOpMkdir, remotePathContext(targetPath, "."), "", s.withFileClient(func(client *sftp.Client) error {
		resolvedParent, baseName, err := resolveRemoteParent(client, targetPath)
		if err != nil {
			return err
		}
		return client.Mkdir(joinRemotePath(resolvedParent, baseName))
	}))
}

func (s *fileSession) Remove(targetPath string) error {
	return domain.WrapFileError(domain.RemotePanel, domain.FileOpRemove, remotePathContext(targetPath, "."), "", s.withFileClient(func(client *sftp.Client) error {
		resolvedPath, err := resolveRemotePath(client, targetPath)
		if err != nil {
			return err
		}
		if isRemoteDeleteProtectedPath(resolvedPath) {
			return domain.ErrRemoteDeleteProtected
		}
		return removeRemotePath(client, resolvedPath)
	}))
}

func (s *fileSession) Rename(sourcePath string, targetPath string) error {
	return domain.WrapFileError(domain.RemotePanel, domain.FileOpRename, remotePathContext(sourcePath, "."), remotePathContext(targetPath, "."), s.withFileClient(func(client *sftp.Client) error {
		resolvedSource, err := resolveRemotePath(client, sourcePath)
		if err != nil {
			return err
		}
		resolvedParent, baseName, err := resolveRemoteParent(client, targetPath)
		if err != nil {
			return err
		}
		resolvedTarget := joinRemotePath(resolvedParent, baseName)
		if err := client.PosixRename(resolvedSource, resolvedTarget); err == nil {
			return nil
		}
		return client.Rename(resolvedSource, resolvedTarget)
	}))
}

func (s *fileSession) Upload(localPath string, remoteDir string, progress func(domain.TransferProgress)) error {
	return domain.WrapFileError(domain.RemotePanel, domain.FileOpUpload, localPath, remotePathContext(remoteDir, "."), s.withFileClient(func(client *sftp.Client) error {
		resolvedDir, err := resolveRemotePath(client, remoteDir)
		if err != nil {
			return err
		}
		return uploadLocalPath(client, localPath, resolvedDir, progress)
	}))
}

func (s *fileSession) Download(remotePath string, localDir string, progress func(domain.TransferProgress)) error {
	return domain.WrapFileError(domain.RemotePanel, domain.FileOpDownload, remotePathContext(remotePath, "."), localDir, s.withFileClient(func(client *sftp.Client) error {
		resolvedPath, err := resolveRemotePath(client, remotePath)
		if err != nil {
			return err
		}
		return downloadRemotePath(client, resolvedPath, localDir, progress)
	}))
}

func (s *fileSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	return s.closeClientsLocked()
}

func (c *Client) dialSFTP(conn domain.Connection, password string) (*gossh.Client, *sftp.Client, error) {
	sshClient, err := c.dial(conn, password)
	if err != nil {
		return nil, nil, err
	}
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		_ = sshClient.Close()
		return nil, nil, err
	}
	return sshClient, sftpClient, nil
}

func withFileClientResult[T any](s *fileSession, op func(*sftp.Client) (T, error)) (T, error) {
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

func (s *fileSession) withFileClient(op func(*sftp.Client) error) error {
	_, err := withFileClientResult(s, func(client *sftp.Client) (struct{}, error) {
		return struct{}{}, op(client)
	})
	return err
}

func (s *fileSession) acquireClient() (*sftp.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, app.ErrRemoteSessionClosed
	}
	if err := s.ensureClientLocked(); err != nil {
		return nil, err
	}
	return s.sftpClient, nil
}

func (s *fileSession) reconnectOrReuse(current *sftp.Client) (*sftp.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, app.ErrRemoteSessionClosed
	}
	if s.sftpClient != nil && s.sftpClient != current {
		return s.sftpClient, nil
	}
	if err := s.redialLocked(); err != nil {
		return nil, err
	}
	return s.sftpClient, nil
}

func (s *fileSession) invalidateClient(current *sftp.Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sftpClient != current {
		return nil
	}
	return s.closeClientsLocked()
}

func (s *fileSession) ensureClientLocked() error {
	if s.sftpClient != nil && s.sshClient != nil {
		return nil
	}
	sshClient, sftpClient, err := s.owner.dialSFTP(s.conn, s.password)
	if err != nil {
		return err
	}
	s.sshClient = sshClient
	s.sftpClient = sftpClient
	return nil
}

func (s *fileSession) redialLocked() error {
	_ = s.closeClientsLocked()
	return s.ensureClientLocked()
}

func (s *fileSession) closeClientsLocked() error {
	var closeErr error
	if s.sftpClient != nil {
		if err := s.sftpClient.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			closeErr = err
		}
		s.sftpClient = nil
	}
	if s.sshClient != nil {
		if err := s.sshClient.Close(); err != nil && !errors.Is(err, net.ErrClosed) && closeErr == nil {
			closeErr = err
		}
		s.sshClient = nil
	}
	return closeErr
}

func listRemoteWithSFTP(client *sftp.Client, targetPath string) (listRemoteResult, error) {
	currentPath, err := resolveRemotePath(client, targetPath)
	if err != nil {
		return listRemoteResult{}, err
	}

	items, err := client.ReadDir(currentPath)
	if err != nil {
		return listRemoteResult{}, err
	}

	entries := make([]domain.FileEntry, 0, len(items))
	for _, item := range items {
		entries = append(entries, domain.FileEntry{
			Name:    item.Name(),
			Path:    joinRemotePath(currentPath, item.Name()),
			Size:    item.Size(),
			ModTime: item.ModTime(),
			Mode:    item.Mode(),
			IsDir:   item.IsDir(),
			Panel:   domain.RemotePanel,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return listRemoteResult{entries: entries, currentPath: currentPath}, nil
}

func resolveRemotePath(client *sftp.Client, targetPath string) (string, error) {
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" || targetPath == "." {
		return client.RealPath(".")
	}
	if targetPath == "~" {
		return client.RealPath(".")
	}
	if strings.HasPrefix(targetPath, "~/") {
		homeDir, err := client.RealPath(".")
		if err != nil {
			return "", err
		}
		return client.RealPath(path.Join(homeDir, strings.TrimPrefix(targetPath, "~/")))
	}
	return client.RealPath(targetPath)
}

func resolveRemoteParent(client *sftp.Client, targetPath string) (string, string, error) {
	trimmed := strings.TrimSpace(targetPath)
	if trimmed == "" {
		return "", "", os.ErrInvalid
	}
	baseName := path.Base(trimmed)
	if baseName == "." || baseName == "/" {
		return "", "", os.ErrInvalid
	}
	parent := path.Dir(trimmed)
	if parent == "." && !strings.HasPrefix(trimmed, "~/") && !strings.HasPrefix(trimmed, "/") && trimmed != "~" {
		parent = "."
	}
	resolvedParent, err := resolveRemotePath(client, parent)
	if err != nil {
		return "", "", err
	}
	return resolvedParent, baseName, nil
}

func uploadLocalPath(client *sftp.Client, localPath string, remoteDir string, progress func(domain.TransferProgress)) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		targetDir := joinRemotePath(remoteDir, filepath.Base(localPath))
		return uploadLocalDir(client, localPath, targetDir, info, progress)
	}
	return uploadLocalFile(client, localPath, joinRemotePath(remoteDir, info.Name()), info, progress)
}

func uploadLocalDir(client *sftp.Client, localPath string, remotePath string, info os.FileInfo, progress func(domain.TransferProgress)) error {
	if err := client.MkdirAll(remotePath); err != nil {
		return err
	}
	items, err := os.ReadDir(localPath)
	if err != nil {
		return err
	}
	for _, item := range items {
		childPath := filepath.Join(localPath, item.Name())
		childInfo, err := item.Info()
		if err != nil {
			return err
		}
		if childInfo.IsDir() {
			if err := uploadLocalDir(client, childPath, joinRemotePath(remotePath, item.Name()), childInfo, progress); err != nil {
				return err
			}
			continue
		}
		if err := uploadLocalFile(client, childPath, joinRemotePath(remotePath, item.Name()), childInfo, progress); err != nil {
			return err
		}
	}
	if err := client.Chmod(remotePath, info.Mode().Perm()); err != nil {
		return err
	}
	return client.Chtimes(remotePath, info.ModTime(), info.ModTime())
}

func uploadLocalFile(client *sftp.Client, localPath string, remotePath string, info os.FileInfo, progress func(domain.TransferProgress)) error {
	localFile, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer localFile.Close()

	if err := client.MkdirAll(path.Dir(remotePath)); err != nil {
		return err
	}
	remoteFile, err := client.OpenFile(remotePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY)
	if err != nil {
		return err
	}
	defer remoteFile.Close()

	if progress != nil {
		progress(domain.TransferProgress{Path: info.Name(), Bytes: 0, Total: info.Size()})
	}
	if _, err := copyWithProgress(remoteFile, localFile, info.Size(), info.Name(), progress); err != nil {
		return err
	}
	if err := client.Chmod(remotePath, info.Mode().Perm()); err != nil {
		return err
	}
	return client.Chtimes(remotePath, info.ModTime(), info.ModTime())
}

func downloadRemotePath(client *sftp.Client, remotePath string, localDir string, progress func(domain.TransferProgress)) error {
	info, err := client.Stat(remotePath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		targetDir := filepath.Join(localDir, path.Base(remotePath))
		return downloadRemoteDir(client, remotePath, targetDir, info, progress)
	}
	return downloadRemoteFile(client, remotePath, filepath.Join(localDir, path.Base(remotePath)), info, progress)
}

func downloadRemoteDir(client *sftp.Client, remotePath string, localPath string, info os.FileInfo, progress func(domain.TransferProgress)) error {
	if err := os.MkdirAll(localPath, info.Mode().Perm()); err != nil {
		return err
	}
	items, err := client.ReadDir(remotePath)
	if err != nil {
		return err
	}
	for _, item := range items {
		childRemotePath := joinRemotePath(remotePath, item.Name())
		if item.IsDir() {
			if err := downloadRemoteDir(client, childRemotePath, filepath.Join(localPath, item.Name()), item, progress); err != nil {
				return err
			}
			continue
		}
		if err := downloadRemoteFile(client, childRemotePath, filepath.Join(localPath, item.Name()), item, progress); err != nil {
			return err
		}
	}
	if err := os.Chmod(localPath, info.Mode().Perm()); err != nil {
		return err
	}
	return os.Chtimes(localPath, info.ModTime(), info.ModTime())
}

func downloadRemoteFile(client *sftp.Client, remotePath string, localPath string, info os.FileInfo, progress func(domain.TransferProgress)) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}
	remoteFile, err := client.Open(remotePath)
	if err != nil {
		return err
	}
	defer remoteFile.Close()

	localFile, err := os.OpenFile(localPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer localFile.Close()

	if progress != nil {
		progress(domain.TransferProgress{Path: info.Name(), Bytes: 0, Total: info.Size()})
	}
	if _, err := copyWithProgress(localFile, remoteFile, info.Size(), info.Name(), progress); err != nil {
		return err
	}
	if err := os.Chmod(localPath, info.Mode().Perm()); err != nil {
		return err
	}
	return os.Chtimes(localPath, info.ModTime(), info.ModTime())
}

func isSFTPNotExist(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, sftp.ErrSSHFxNoSuchFile)
}

func removeRemotePath(client *sftp.Client, targetPath string) error {
	info, err := client.Stat(targetPath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return client.Remove(targetPath)
	}
	items, err := client.ReadDir(targetPath)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := removeRemotePath(client, joinRemotePath(targetPath, item.Name())); err != nil {
			return err
		}
	}
	return client.RemoveDirectory(targetPath)
}

func isRemoteDeleteProtectedPath(targetPath string) bool {
	cleaned := path.Clean(strings.TrimSpace(targetPath))
	if cleaned == "/" {
		return true
	}
	return path.Dir(cleaned) == "/"
}

func joinRemotePath(basePath string, name string) string {
	if basePath == "" {
		return path.Clean("/" + name)
	}
	return path.Clean(path.Join(basePath, name))
}

func remotePathContext(targetPath string, fallback string) string {
	trimmed := strings.TrimSpace(targetPath)
	if trimmed == "" {
		return fallback
	}
	return trimmed
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

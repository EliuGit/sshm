package app

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sshm/internal/domain"
	"strings"
)

var ErrRemoteSessionClosed = errors.New("remote session is closed")

// ShellSession 表示用于进入交互式远端终端的会话。
type ShellSession interface {
	OpenShell() error
	Close() error
}

// FileSession 表示用于文件工作区的远端文件会话。
//
// 设计说明：
// 1. 这里只暴露文件工作区真正需要的能力，明确禁止回流到 shell / 命令执行链路。
// 2. 覆盖确认所需的存在性检查保留在会话层，避免 UI 为此重新拼装额外的远端请求入口。
// 3. 路径过滤、选中态、刷新策略都由 UI 持有；app 层只负责返回目录快照与执行文件操作。
type FileSession interface {
	ListRemote(targetPath string) ([]domain.FileEntry, string, error)
	PathExists(targetPath string) (bool, error)
	Mkdir(targetPath string) error
	Remove(targetPath string) error
	Rename(sourcePath string, targetPath string) error
	Upload(localPath string, remoteDir string, progress func(domain.TransferProgress)) error
	Download(remotePath string, localDir string, progress func(domain.TransferProgress)) error
	Close() error
}

type ShellClient interface {
	ProbeShell(conn domain.Connection, password string) error
	OpenSession(conn domain.Connection, password string) (ShellSession, error)
	OpenShell(conn domain.Connection, password string) error
	RunCommand(conn domain.Connection, password string, command string, stdout io.Writer, stderr io.Writer) error
}

type FileClient interface {
	OpenFileSession(conn domain.Connection, password string) (FileSession, error)
}

type RemoteClient interface {
	ShellClient
	FileClient
}

type Services struct {
	Connections *ConnectionService
	Groups      *GroupService
	Imports     *ImportService
	Sessions    *SessionService
	Files       *FileService
}

func NewServices(repo Repository, crypto CryptoProvider, remote RemoteClient, defaultPrivateKeyPath string) *Services {
	if strings.TrimSpace(defaultPrivateKeyPath) == "" {
		defaultPrivateKeyPath = "~/.ssh/id_rsa"
	}
	connSvc := &ConnectionService{repo: repo, crypto: crypto, defaultPrivateKeyPath: defaultPrivateKeyPath}
	return &Services{
		Connections: connSvc,
		Groups:      &GroupService{repo: repo},
		Imports:     &ImportService{repo: repo, connections: connSvc, defaultPrivateKeyPath: defaultPrivateKeyPath},
		Sessions:    &SessionService{repo: repo, crypto: crypto, remote: remote},
		Files:       &FileService{repo: repo, crypto: crypto, remote: remote},
	}
}

type ConnectionService struct {
	repo                  Repository
	crypto                CryptoProvider
	defaultPrivateKeyPath string
}

func (s *ConnectionService) List(query string) ([]domain.Connection, error) {
	return s.repo.ListConnections(domain.ConnectionListOptions{Query: query})
}

func (s *ConnectionService) ListWithOptions(opts domain.ConnectionListOptions) ([]domain.Connection, error) {
	return s.repo.ListConnections(opts)
}

func (s *ConnectionService) Get(id int64) (domain.Connection, error) {
	return s.repo.GetConnection(id)
}

func (s *ConnectionService) ResolveNames(names []string) ([]domain.Connection, error) {
	requested := make([]string, 0, len(names))
	seen := map[string]bool{}
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if seen[key] {
			continue
		}
		seen[key] = true
		requested = append(requested, trimmed)
	}
	if len(requested) == 0 {
		return nil, domain.ErrConnectionNameRequired
	}

	connections, err := s.repo.ListConnections(domain.ConnectionListOptions{})
	if err != nil {
		return nil, err
	}
	byName := map[string][]domain.Connection{}
	for _, conn := range connections {
		key := strings.ToLower(strings.TrimSpace(conn.Name))
		byName[key] = append(byName[key], conn)
	}

	resolved := make([]domain.Connection, 0, len(requested))
	for _, name := range requested {
		matches := byName[strings.ToLower(name)]
		switch len(matches) {
		case 0:
			return nil, domain.NewConnectionNameNotFoundError(name)
		case 1:
			resolved = append(resolved, matches[0])
		default:
			return nil, domain.NewConnectionNameDuplicatedError(name)
		}
	}
	return resolved, nil
}

func (s *ConnectionService) Create(input domain.ConnectionInput) (domain.Connection, error) {
	conn, secret, err := s.buildCreate(input)
	if err != nil {
		return domain.Connection{}, err
	}
	id, err := s.repo.CreateConnection(conn, secret)
	if err != nil {
		return domain.Connection{}, err
	}
	return s.repo.GetConnection(id)
}

func (s *ConnectionService) Update(id int64, input domain.ConnectionUpdateInput) (domain.Connection, error) {
	current, err := s.repo.GetConnection(id)
	if err != nil {
		return domain.Connection{}, err
	}
	conn, secret, deleteSecret, err := s.buildUpdate(current, input)
	if err != nil {
		return domain.Connection{}, err
	}
	if err := s.repo.UpdateConnection(id, conn, secret, deleteSecret); err != nil {
		return domain.Connection{}, err
	}
	return s.repo.GetConnection(id)
}

func (s *ConnectionService) Delete(id int64) error {
	return s.repo.DeleteConnection(id)
}

func (s *ConnectionService) buildCreate(input domain.ConnectionInput) (domain.Connection, *domain.ConnectionSecret, error) {
	conn, err := validateConnectionInput(input, s.defaultPrivateKeyPath, true)
	if err != nil {
		return domain.Connection{}, nil, err
	}
	if input.AuthType == domain.AuthTypePassword {
		ciphertext, err := s.crypto.Encrypt(input.Password)
		if err != nil {
			return domain.Connection{}, nil, err
		}
		return conn, &domain.ConnectionSecret{PasswordCiphertext: ciphertext}, nil
	}
	return conn, nil, nil
}

func (s *ConnectionService) buildUpdate(current domain.Connection, input domain.ConnectionUpdateInput) (domain.Connection, *domain.ConnectionSecret, bool, error) {
	requirePassword := input.AuthType == domain.AuthTypePassword && !input.KeepPassword
	conn, err := validateConnectionInput(domain.ConnectionInput{
		GroupID:        input.GroupID,
		Name:           input.Name,
		Host:           input.Host,
		Port:           input.Port,
		Username:       input.Username,
		AuthType:       input.AuthType,
		PrivateKeyPath: input.PrivateKeyPath,
		Description:    input.Description,
		Password:       input.Password,
	}, s.defaultPrivateKeyPath, requirePassword)
	if err != nil {
		return domain.Connection{}, nil, false, err
	}
	var secret *domain.ConnectionSecret
	deleteSecret := false
	if input.AuthType == domain.AuthTypePassword {
		if input.KeepPassword {
			if _, err := s.repo.GetSecret(current.ID); err != nil {
				if errors.Is(err, domain.ErrConnectionSecretNotFound) {
					return domain.Connection{}, nil, false, domain.ErrPasswordRequired
				}
				return domain.Connection{}, nil, false, err
			}
		} else {
			ciphertext, err := s.crypto.Encrypt(input.Password)
			if err != nil {
				return domain.Connection{}, nil, false, err
			}
			secret = &domain.ConnectionSecret{ConnectionID: current.ID, PasswordCiphertext: ciphertext}
		}
	} else {
		deleteSecret = true
	}
	conn.ID = current.ID
	conn.CreatedAt = current.CreatedAt
	conn.LastUsedAt = current.LastUsedAt
	return conn, secret, deleteSecret, nil
}

type SessionService struct {
	repo   Repository
	crypto CryptoProvider
	remote ShellClient
}

type managedShellSession struct {
	connectionID int64
	repo         Repository
	inner        ShellSession
}

func (s *managedShellSession) OpenShell() error {
	if err := s.inner.OpenShell(); err != nil {
		return err
	}
	return s.repo.MarkUsed(s.connectionID)
}

func (s *managedShellSession) Close() error {
	if s.inner == nil {
		return nil
	}
	return s.inner.Close()
}

func (s *SessionService) ProbeShell(connectionID int64) error {
	conn, password, err := loadConnectionWithPassword(s.repo, s.crypto, connectionID)
	if err != nil {
		return err
	}
	return s.remote.ProbeShell(conn, password)
}

func (s *SessionService) OpenSession(connectionID int64) (ShellSession, error) {
	conn, password, err := loadConnectionWithPassword(s.repo, s.crypto, connectionID)
	if err != nil {
		return nil, err
	}
	session, err := s.remote.OpenSession(conn, password)
	if err != nil {
		return nil, err
	}
	return &managedShellSession{
		connectionID: connectionID,
		repo:         s.repo,
		inner:        session,
	}, nil
}

func (s *SessionService) OpenShell(connectionID int64) error {
	conn, password, err := loadConnectionWithPassword(s.repo, s.crypto, connectionID)
	if err != nil {
		return err
	}
	if err := s.remote.OpenShell(conn, password); err != nil {
		return err
	}
	return s.repo.MarkUsed(connectionID)
}

func (s *SessionService) RunCommand(connectionID int64, command string, stdout io.Writer, stderr io.Writer) error {
	conn, password, err := loadConnectionWithPassword(s.repo, s.crypto, connectionID)
	if err != nil {
		return err
	}
	if err := s.remote.RunCommand(conn, password, command, stdout, stderr); err != nil {
		return err
	}
	return s.repo.MarkUsed(connectionID)
}

type FileService struct {
	repo   Repository
	crypto CryptoProvider
	remote FileClient
}

// managedFileSession 负责把“文件操作成功后更新 LastUsedAt”的横切逻辑收口在 app 层。
//
// 这样 transport 只关心 SSH/SFTP 语义，UI 也不用在每个文件动作完成后自行补记使用时间。
type managedFileSession struct {
	connectionID int64
	repo         Repository
	inner        FileSession
}

func (s *managedFileSession) ListRemote(targetPath string) ([]domain.FileEntry, string, error) {
	return s.inner.ListRemote(targetPath)
}

func (s *managedFileSession) PathExists(targetPath string) (bool, error) {
	return s.inner.PathExists(targetPath)
}

func (s *managedFileSession) Mkdir(targetPath string) error {
	if err := s.inner.Mkdir(targetPath); err != nil {
		return err
	}
	return s.repo.MarkUsed(s.connectionID)
}

func (s *managedFileSession) Remove(targetPath string) error {
	if err := s.inner.Remove(targetPath); err != nil {
		return err
	}
	return s.repo.MarkUsed(s.connectionID)
}

func (s *managedFileSession) Rename(sourcePath string, targetPath string) error {
	if err := s.inner.Rename(sourcePath, targetPath); err != nil {
		return err
	}
	return s.repo.MarkUsed(s.connectionID)
}

func (s *managedFileSession) Upload(localPath string, remoteDir string, progress func(domain.TransferProgress)) error {
	if err := s.inner.Upload(localPath, remoteDir, progress); err != nil {
		return err
	}
	return s.repo.MarkUsed(s.connectionID)
}

func (s *managedFileSession) Download(remotePath string, localDir string, progress func(domain.TransferProgress)) error {
	if err := s.inner.Download(remotePath, localDir, progress); err != nil {
		return err
	}
	return s.repo.MarkUsed(s.connectionID)
}

func (s *managedFileSession) Close() error {
	if s.inner == nil {
		return nil
	}
	return s.inner.Close()
}

func (s *FileService) OpenSession(connectionID int64) (FileSession, error) {
	conn, password, err := loadConnectionWithPassword(s.repo, s.crypto, connectionID)
	if err != nil {
		return nil, err
	}
	session, err := s.remote.OpenFileSession(conn, password)
	if err != nil {
		return nil, err
	}
	return &managedFileSession{
		connectionID: connectionID,
		repo:         s.repo,
		inner:        session,
	}, nil
}

func (s *FileService) LoadConnection(id int64) (domain.Connection, error) {
	return s.repo.GetConnection(id)
}

// ListLocal / ListRemote 只返回当前路径的完整快照。
//
// 过滤条件属于 UI 文件面板状态，必须留在 ui 层持有，否则 app 会再次耦合页面交互细节。
func (s *FileService) ListLocal(targetPath string) ([]domain.FileEntry, string, error) {
	currentPath := expandPath(targetPath)
	if strings.TrimSpace(currentPath) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, "", domain.WrapFileError(domain.LocalPanel, domain.FileOpList, currentPath, "", err)
		}
		currentPath = wd
	}
	currentPath = filepath.Clean(currentPath)
	items, err := os.ReadDir(currentPath)
	if err != nil {
		return nil, "", domain.WrapFileError(domain.LocalPanel, domain.FileOpList, currentPath, "", err)
	}
	entries := make([]domain.FileEntry, 0, len(items))
	for _, item := range items {
		info, err := item.Info()
		if err != nil {
			return nil, "", domain.WrapFileError(domain.LocalPanel, domain.FileOpList, filepath.Join(currentPath, item.Name()), "", err)
		}
		entries = append(entries, domain.FileEntry{
			Name:    item.Name(),
			Path:    filepath.Join(currentPath, item.Name()),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Mode:    info.Mode(),
			IsDir:   item.IsDir(),
			Panel:   domain.LocalPanel,
		})
	}
	sortEntries(entries)
	return entries, currentPath, nil
}

func (s *FileService) ListRemote(connectionID int64, targetPath string) ([]domain.FileEntry, string, error) {
	session, err := s.OpenSession(connectionID)
	if err != nil {
		return nil, "", err
	}
	defer session.Close()
	return session.ListRemote(targetPath)
}

func (s *FileService) ExistsLocal(targetPath string) (bool, error) {
	_, err := os.Stat(targetPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, domain.WrapFileError(domain.LocalPanel, domain.FileOpStat, targetPath, "", err)
}

func (s *FileService) MkdirLocal(targetPath string) error {
	return domain.WrapFileError(domain.LocalPanel, domain.FileOpMkdir, targetPath, "", os.Mkdir(targetPath, 0755))
}

func (s *FileService) RemoveLocal(targetPath string) error {
	return domain.WrapFileError(domain.LocalPanel, domain.FileOpRemove, targetPath, "", os.RemoveAll(targetPath))
}

func (s *FileService) RenameLocal(sourcePath string, targetPath string) error {
	return domain.WrapFileError(domain.LocalPanel, domain.FileOpRename, sourcePath, targetPath, os.Rename(sourcePath, targetPath))
}

func (s *FileService) ExistsRemote(connectionID int64, targetPath string) (bool, error) {
	session, err := s.OpenSession(connectionID)
	if err != nil {
		return false, err
	}
	defer session.Close()
	return session.PathExists(targetPath)
}

func (s *FileService) Upload(connectionID int64, localPath string, remoteDir string, progress func(domain.TransferProgress)) error {
	session, err := s.OpenSession(connectionID)
	if err != nil {
		return err
	}
	defer session.Close()
	return session.Upload(localPath, remoteDir, progress)
}

func (s *FileService) Download(connectionID int64, remotePath string, localDir string, progress func(domain.TransferProgress)) error {
	session, err := s.OpenSession(connectionID)
	if err != nil {
		return err
	}
	defer session.Close()
	return session.Download(remotePath, localDir, progress)
}

func loadConnectionWithPassword(repo Repository, crypto CryptoProvider, connectionID int64) (domain.Connection, string, error) {
	conn, err := repo.GetConnection(connectionID)
	if err != nil {
		return domain.Connection{}, "", err
	}
	if conn.AuthType != domain.AuthTypePassword {
		return conn, "", nil
	}
	secret, err := repo.GetSecret(connectionID)
	if err != nil {
		return domain.Connection{}, "", err
	}
	password, err := crypto.Decrypt(secret.PasswordCiphertext)
	if err != nil {
		return domain.Connection{}, "", err
	}
	return conn, password, nil
}

func validateConnectionInput(input domain.ConnectionInput, defaultPrivateKeyPath string, requirePassword bool) (domain.Connection, error) {
	conn := domain.Connection{
		GroupID:        input.GroupID,
		Name:           strings.TrimSpace(input.Name),
		Host:           strings.TrimSpace(input.Host),
		Port:           input.Port,
		Username:       strings.TrimSpace(input.Username),
		AuthType:       input.AuthType,
		PrivateKeyPath: strings.TrimSpace(input.PrivateKeyPath),
		Description:    strings.TrimSpace(input.Description),
	}
	if conn.Name == "" {
		return domain.Connection{}, domain.ErrNameRequired
	}
	if conn.Host == "" {
		return domain.Connection{}, domain.ErrHostRequired
	}
	if conn.Username == "" {
		return domain.Connection{}, domain.ErrUsernameRequired
	}
	if conn.Port <= 0 || conn.Port > 65535 {
		return domain.Connection{}, domain.ErrPortRange
	}
	switch conn.AuthType {
	case domain.AuthTypePassword:
		if requirePassword && strings.TrimSpace(input.Password) == "" {
			return domain.Connection{}, domain.ErrPasswordRequired
		}
	case domain.AuthTypePrivateKey:
		if conn.PrivateKeyPath == "" {
			conn.PrivateKeyPath = defaultPrivateKeyPath
		}
	default:
		return domain.Connection{}, domain.ErrUnsupportedAuthType
	}
	return conn, nil
}

type GroupService struct {
	repo Repository
}

func (s *GroupService) List() ([]domain.ConnectionGroupListItem, error) {
	return s.repo.ListGroups()
}

func (s *GroupService) Create(name string) (domain.ConnectionGroup, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.ConnectionGroup{}, domain.ErrGroupNameRequired
	}
	return s.repo.CreateGroup(name)
}

func (s *GroupService) Rename(id int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.ErrGroupNameRequired
	}
	return s.repo.RenameGroup(id, name)
}

func (s *GroupService) Delete(id int64) error {
	if id == 0 {
		return domain.ErrGroupRequired
	}
	return s.repo.DeleteGroup(id)
}

func (s *GroupService) MoveConnection(connectionID int64, groupID *int64) error {
	return s.repo.SetConnectionGroup(connectionID, groupID)
}

func expandPath(path string) string {
	if path == "" {
		return path
	}
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		if path == "~" {
			return homeDir
		}
		return filepath.Join(homeDir, path[2:])
	}
	return path
}

func sortEntries(entries []domain.FileEntry) {
	sort.Slice(entries, func(left, right int) bool {
		if entries[left].IsDir != entries[right].IsDir {
			return entries[left].IsDir
		}
		return strings.ToLower(entries[left].Name) < strings.ToLower(entries[right].Name)
	})
}

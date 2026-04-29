package app

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sshm/internal/domain"
	"sshm/internal/i18n"
	"sshm/internal/security"
	"sshm/internal/store/sqlite"
	"strings"
	"testing"
)

type noopRemote struct{}
type noopSession struct{}

func (noopRemote) ProbeShell(conn domain.Connection, password string) error { return nil }
func (noopRemote) OpenSession(conn domain.Connection, password string) (ShellSession, error) {
	return &noopSession{}, nil
}
func (noopRemote) OpenFileSession(conn domain.Connection, password string) (FileSession, error) {
	return &noopSession{}, nil
}
func (noopRemote) OpenShell(conn domain.Connection, password string) error { return nil }
func (noopRemote) RunCommand(conn domain.Connection, password string, command string, stdout io.Writer, stderr io.Writer) error {
	return nil
}

func TestGroupServiceMovesConnection(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	repo, err := sqlite.Open(filepath.Join(tempDir, "sshm.db"))
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	defer repo.Close()

	crypto, err := security.LoadOrCreateKey(filepath.Join(tempDir, "app.key"))
	if err != nil {
		t.Fatalf("LoadOrCreateKey() error = %v", err)
	}

	services := NewServices(repo, crypto, noopRemote{}, "~/.ssh/id_rsa")
	group, err := services.Groups.Create("生产环境")
	if err != nil {
		t.Fatalf("Groups.Create() error = %v", err)
	}
	created, err := services.Connections.Create(domain.ConnectionInput{
		Name:     "prod",
		Host:     "example.com",
		Port:     22,
		Username: "root",
		AuthType: domain.AuthTypePrivateKey,
	})
	if err != nil {
		t.Fatalf("Connections.Create() error = %v", err)
	}
	if err := services.Groups.MoveConnection(created.ID, &group.ID); err != nil {
		t.Fatalf("MoveConnection() error = %v", err)
	}

	grouped, err := services.Connections.ListWithOptions(domain.ConnectionListOptions{
		Scope:   domain.ConnectionListScopeGroup,
		GroupID: group.ID,
	})
	if err != nil {
		t.Fatalf("ListWithOptions(group) error = %v", err)
	}
	if len(grouped) != 1 || grouped[0].GroupName != "生产环境" {
		t.Fatalf("grouped = %#v", grouped)
	}

	if err := services.Groups.MoveConnection(created.ID, nil); err != nil {
		t.Fatalf("MoveConnection(nil) error = %v", err)
	}
	ungrouped, err := services.Connections.ListWithOptions(domain.ConnectionListOptions{Scope: domain.ConnectionListScopeUngrouped})
	if err != nil {
		t.Fatalf("ListWithOptions(ungrouped) error = %v", err)
	}
	if len(ungrouped) != 1 || ungrouped[0].GroupID != nil {
		t.Fatalf("ungrouped = %#v", ungrouped)
	}
}

func TestImportSSHConfigUsesCommentGroupsAndUngroupedDefault(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	repo, err := sqlite.Open(filepath.Join(tempDir, "sshm.db"))
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	defer repo.Close()

	crypto, err := security.LoadOrCreateKey(filepath.Join(tempDir, "app.key"))
	if err != nil {
		t.Fatalf("LoadOrCreateKey() error = %v", err)
	}
	configPath := filepath.Join(tempDir, "ssh_config")
	content := strings.Join([]string{
		"# sshm:group=生产环境",
		"Host prod-web",
		"  HostName 10.0.0.1",
		"  User deploy",
		"  Port 2222",
		"  IdentityFile ~/.ssh/prod",
		"",
		"# sshm:group=",
		"Host dev-web",
		"  HostName 10.0.0.2",
		"  User dev",
		"Host *",
		"  User ignored",
	}, "\n")
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	services := NewServices(repo, crypto, noopRemote{}, "~/.ssh/id_rsa")
	preview, err := services.Imports.PreviewSSHConfig(configPath)
	if err != nil {
		t.Fatalf("PreviewSSHConfig() error = %v", err)
	}
	if len(preview.Candidates) != 3 {
		t.Fatalf("Candidates len = %d, want 3", len(preview.Candidates))
	}
	if preview.Candidates[0].GroupName != "生产环境" {
		t.Fatalf("prod group = %q", preview.Candidates[0].GroupName)
	}
	if preview.Candidates[1].GroupName != "" {
		t.Fatalf("dev group = %q, want empty", preview.Candidates[1].GroupName)
	}
	if !preview.Candidates[2].Skipped {
		t.Fatalf("wildcard Host should be skipped")
	}
	summary, err := services.Imports.Apply(preview)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if summary.Created != 2 || summary.Skipped != 1 {
		t.Fatalf("summary = %#v", summary)
	}

	groups, err := services.Groups.List()
	if err != nil {
		t.Fatalf("Groups.List() error = %v", err)
	}
	if len(groups) != 2 || groups[0].ConnectionCount != 1 || groups[1].Name != "生产环境" || groups[1].ConnectionCount != 1 {
		t.Fatalf("groups = %#v", groups)
	}
}
func (*noopSession) OpenShell() error { return nil }
func (*noopSession) ListRemote(targetPath string) ([]domain.FileEntry, string, error) {
	return nil, "", nil
}
func (*noopSession) PathExists(targetPath string) (bool, error) { return false, nil }
func (*noopSession) Mkdir(targetPath string) error              { return nil }
func (*noopSession) Remove(targetPath string) error             { return nil }
func (*noopSession) Rename(sourcePath string, targetPath string) error {
	return nil
}
func (*noopSession) Upload(localPath string, remoteDir string, progress func(domain.TransferProgress)) error {
	return nil
}
func (*noopSession) Download(remotePath string, localDir string, progress func(domain.TransferProgress)) error {
	return nil
}
func (*noopSession) Close() error { return nil }

type probeRemote struct {
	probeErr error
	probed   bool
}

type trackingSession struct {
	openShellCalls int
}

func (r *probeRemote) ProbeShell(conn domain.Connection, password string) error {
	r.probed = true
	return r.probeErr
}

func (r *probeRemote) OpenSession(conn domain.Connection, password string) (ShellSession, error) {
	if r.probeErr != nil {
		return nil, r.probeErr
	}
	return &noopSession{}, nil
}

func (r *probeRemote) OpenFileSession(conn domain.Connection, password string) (FileSession, error) {
	if r.probeErr != nil {
		return nil, r.probeErr
	}
	return &noopSession{}, nil
}

func (*probeRemote) OpenShell(conn domain.Connection, password string) error { return nil }
func (*probeRemote) RunCommand(conn domain.Connection, password string, command string, stdout io.Writer, stderr io.Writer) error {
	return nil
}
func (s *trackingSession) OpenShell() error {
	s.openShellCalls++
	return nil
}
func (*trackingSession) ListRemote(targetPath string) ([]domain.FileEntry, string, error) {
	return nil, "", nil
}
func (*trackingSession) PathExists(targetPath string) (bool, error) { return false, nil }
func (*trackingSession) Mkdir(targetPath string) error              { return nil }
func (*trackingSession) Remove(targetPath string) error             { return nil }
func (*trackingSession) Rename(sourcePath string, targetPath string) error {
	return nil
}
func (*trackingSession) Upload(localPath string, remoteDir string, progress func(domain.TransferProgress)) error {
	return nil
}
func (*trackingSession) Download(remotePath string, localDir string, progress func(domain.TransferProgress)) error {
	return nil
}
func (*trackingSession) Close() error { return nil }

type sessionOpenRemote struct {
	session *trackingSession
}

func (r *sessionOpenRemote) ProbeShell(conn domain.Connection, password string) error { return nil }
func (r *sessionOpenRemote) OpenSession(conn domain.Connection, password string) (ShellSession, error) {
	if r.session == nil {
		r.session = &trackingSession{}
	}
	return r.session, nil
}
func (r *sessionOpenRemote) OpenFileSession(conn domain.Connection, password string) (FileSession, error) {
	return &noopSession{}, nil
}
func (*sessionOpenRemote) OpenShell(conn domain.Connection, password string) error { return nil }
func (*sessionOpenRemote) RunCommand(conn domain.Connection, password string, command string, stdout io.Writer, stderr io.Writer) error {
	return nil
}

func TestConnectionUpdateKeepsPassword(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	repo, err := sqlite.Open(filepath.Join(tempDir, "sshm.db"))
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	defer repo.Close()

	crypto, err := security.LoadOrCreateKey(filepath.Join(tempDir, "app.key"))
	if err != nil {
		t.Fatalf("LoadOrCreateKey() error = %v", err)
	}

	services := NewServices(repo, crypto, noopRemote{}, "~/.ssh/id_rsa")

	created, err := services.Connections.Create(domain.ConnectionInput{
		Name:        "prod",
		Host:        "example.com",
		Port:        22,
		Username:    "root",
		AuthType:    domain.AuthTypePassword,
		Description: "server",
		Password:    "topsecret",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	updated, err := services.Connections.Update(created.ID, domain.ConnectionUpdateInput{
		Name:         "prod-new",
		Host:         "example.com",
		Port:         22,
		Username:     "root",
		AuthType:     domain.AuthTypePassword,
		Description:  "server-2",
		KeepPassword: true,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != "prod-new" {
		t.Fatalf("updated.Name = %q, want %q", updated.Name, "prod-new")
	}

	secret, err := repo.GetSecret(created.ID)
	if err != nil {
		t.Fatalf("GetSecret() error = %v", err)
	}
	password, err := crypto.Decrypt(secret.PasswordCiphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if password != "topsecret" {
		t.Fatalf("password = %q, want %q", password, "topsecret")
	}
}

func TestConnectionCreateUsesConfiguredDefaultPrivateKey(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	repo, err := sqlite.Open(filepath.Join(tempDir, "sshm.db"))
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	defer repo.Close()

	crypto, err := security.LoadOrCreateKey(filepath.Join(tempDir, "app.key"))
	if err != nil {
		t.Fatalf("LoadOrCreateKey() error = %v", err)
	}

	services := NewServices(repo, crypto, noopRemote{}, "~/.ssh/id_ed25519")
	created, err := services.Connections.Create(domain.ConnectionInput{
		Name:     "prod",
		Host:     "example.com",
		Port:     22,
		Username: "root",
		AuthType: domain.AuthTypePrivateKey,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.PrivateKeyPath != "~/.ssh/id_ed25519" {
		t.Fatalf("PrivateKeyPath = %q, want %q", created.PrivateKeyPath, "~/.ssh/id_ed25519")
	}
}

func TestConnectionUpdatePasswordWithoutStoredSecretFailsClearly(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	repo, err := sqlite.Open(filepath.Join(tempDir, "sshm.db"))
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	defer repo.Close()

	crypto, err := security.LoadOrCreateKey(filepath.Join(tempDir, "app.key"))
	if err != nil {
		t.Fatalf("LoadOrCreateKey() error = %v", err)
	}
	translator, err := i18n.New("zh-CN")
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}

	services := NewServices(repo, crypto, noopRemote{}, "~/.ssh/id_rsa")
	created, err := services.Connections.Create(domain.ConnectionInput{
		Name:     "prod",
		Host:     "example.com",
		Port:     22,
		Username: "root",
		AuthType: domain.AuthTypePrivateKey,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, err = services.Connections.Update(created.ID, domain.ConnectionUpdateInput{
		Name:         "prod",
		Host:         "example.com",
		Port:         22,
		Username:     "root",
		AuthType:     domain.AuthTypePassword,
		KeepPassword: true,
	})
	if err == nil {
		t.Fatal("Update() error = nil, want password required")
	}
	if got := translator.Error(err); got != translator.T("err.password_required") {
		t.Fatalf("translator.Error() = %q, want %q", got, translator.T("err.password_required"))
	}
}

func TestConnectionResolveNamesUsesExactCaseInsensitiveMatch(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	repo, err := sqlite.Open(filepath.Join(tempDir, "sshm.db"))
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	defer repo.Close()

	crypto, err := security.LoadOrCreateKey(filepath.Join(tempDir, "app.key"))
	if err != nil {
		t.Fatalf("LoadOrCreateKey() error = %v", err)
	}

	services := NewServices(repo, crypto, noopRemote{}, "~/.ssh/id_rsa")
	for _, name := range []string{"prod", "web"} {
		if _, err := services.Connections.Create(domain.ConnectionInput{
			Name:     name,
			Host:     name + ".example.com",
			Port:     22,
			Username: "root",
			AuthType: domain.AuthTypePrivateKey,
		}); err != nil {
			t.Fatalf("Create(%q) error = %v", name, err)
		}
	}

	resolved, err := services.Connections.ResolveNames([]string{"WEB", "prod"})
	if err != nil {
		t.Fatalf("ResolveNames() error = %v", err)
	}
	if len(resolved) != 2 || resolved[0].Name != "web" || resolved[1].Name != "prod" {
		t.Fatalf("resolved = %#v", resolved)
	}
}

func TestConnectionResolveNamesRejectsDuplicateNames(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	repo, err := sqlite.Open(filepath.Join(tempDir, "sshm.db"))
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	defer repo.Close()

	crypto, err := security.LoadOrCreateKey(filepath.Join(tempDir, "app.key"))
	if err != nil {
		t.Fatalf("LoadOrCreateKey() error = %v", err)
	}

	services := NewServices(repo, crypto, noopRemote{}, "~/.ssh/id_rsa")
	for _, host := range []string{"one.example.com", "two.example.com"} {
		if _, err := services.Connections.Create(domain.ConnectionInput{
			Name:     "prod",
			Host:     host,
			Port:     22,
			Username: "root",
			AuthType: domain.AuthTypePrivateKey,
		}); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	if _, err := services.Connections.ResolveNames([]string{"prod"}); err == nil {
		t.Fatal("ResolveNames() error = nil, want duplicate error")
	}
}

func TestSessionProbeShellDoesNotMarkConnectionUsed(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	repo, err := sqlite.Open(filepath.Join(tempDir, "sshm.db"))
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	defer repo.Close()

	crypto, err := security.LoadOrCreateKey(filepath.Join(tempDir, "app.key"))
	if err != nil {
		t.Fatalf("LoadOrCreateKey() error = %v", err)
	}

	remote := &probeRemote{}
	services := NewServices(repo, crypto, remote, "~/.ssh/id_rsa")
	created, err := services.Connections.Create(domain.ConnectionInput{
		Name:     "prod",
		Host:     "example.com",
		Port:     22,
		Username: "root",
		AuthType: domain.AuthTypePrivateKey,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := services.Sessions.ProbeShell(created.ID); err != nil {
		t.Fatalf("ProbeShell() error = %v", err)
	}
	if !remote.probed {
		t.Fatal("ProbeShell() did not call remote probe")
	}

	loaded, err := services.Connections.Get(created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if loaded.LastUsedAt != nil {
		t.Fatalf("LastUsedAt = %v, want nil", loaded.LastUsedAt)
	}
}

func TestSessionProbeShellReturnsRemoteError(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	repo, err := sqlite.Open(filepath.Join(tempDir, "sshm.db"))
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	defer repo.Close()

	crypto, err := security.LoadOrCreateKey(filepath.Join(tempDir, "app.key"))
	if err != nil {
		t.Fatalf("LoadOrCreateKey() error = %v", err)
	}

	expected := errors.New("auth failed")
	remote := &probeRemote{probeErr: expected}
	services := NewServices(repo, crypto, remote, "~/.ssh/id_rsa")
	created, err := services.Connections.Create(domain.ConnectionInput{
		Name:     "prod",
		Host:     "example.com",
		Port:     22,
		Username: "root",
		AuthType: domain.AuthTypePrivateKey,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	err = services.Sessions.ProbeShell(created.ID)
	if !errors.Is(err, expected) {
		t.Fatalf("ProbeShell() error = %v, want %v", err, expected)
	}
}

func TestOpenSessionShellMarksConnectionUsed(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	repo, err := sqlite.Open(filepath.Join(tempDir, "sshm.db"))
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	defer repo.Close()

	crypto, err := security.LoadOrCreateKey(filepath.Join(tempDir, "app.key"))
	if err != nil {
		t.Fatalf("LoadOrCreateKey() error = %v", err)
	}

	remote := &sessionOpenRemote{}
	services := NewServices(repo, crypto, remote, "~/.ssh/id_rsa")
	created, err := services.Connections.Create(domain.ConnectionInput{
		Name:     "prod",
		Host:     "example.com",
		Port:     22,
		Username: "root",
		AuthType: domain.AuthTypePrivateKey,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	session, err := services.Sessions.OpenSession(created.ID)
	if err != nil {
		t.Fatalf("OpenSession() error = %v", err)
	}
	if err := session.OpenShell(); err != nil {
		t.Fatalf("session.OpenShell() error = %v", err)
	}
	if remote.session == nil || remote.session.openShellCalls != 1 {
		t.Fatalf("openShellCalls = %d, want 1", remote.session.openShellCalls)
	}

	loaded, err := services.Connections.Get(created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if loaded.LastUsedAt == nil {
		t.Fatal("LastUsedAt = nil, want non-nil")
	}
}

func TestFileServiceLocalOpsManageLifecycle(t *testing.T) {
	t.Parallel()

	service := &FileService{}
	baseDir := t.TempDir()
	sourceDir := filepath.Join(baseDir, "logs")
	renamedDir := filepath.Join(baseDir, "logs-archive")

	if err := service.MkdirLocal(sourceDir); err != nil {
		t.Fatalf("MkdirLocal() error = %v", err)
	}
	exists, err := service.ExistsLocal(sourceDir)
	if err != nil {
		t.Fatalf("ExistsLocal() error = %v", err)
	}
	if !exists {
		t.Fatal("ExistsLocal() = false, want true after MkdirLocal")
	}

	if err := service.RenameLocal(sourceDir, renamedDir); err != nil {
		t.Fatalf("RenameLocal() error = %v", err)
	}
	exists, err = service.ExistsLocal(renamedDir)
	if err != nil {
		t.Fatalf("ExistsLocal(renamed) error = %v", err)
	}
	if !exists {
		t.Fatal("ExistsLocal(renamed) = false, want true after RenameLocal")
	}

	if err := service.RemoveLocal(renamedDir); err != nil {
		t.Fatalf("RemoveLocal() error = %v", err)
	}
	exists, err = service.ExistsLocal(renamedDir)
	if err != nil {
		t.Fatalf("ExistsLocal(after remove) error = %v", err)
	}
	if exists {
		t.Fatal("ExistsLocal(after remove) = true, want false")
	}
}

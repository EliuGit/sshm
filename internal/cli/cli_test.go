package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"sshm/internal/app"
	"sshm/internal/buildinfo"
	"sshm/internal/domain"
	"sshm/internal/i18n"
	"sshm/internal/security"
	"sshm/internal/store/sqlite"
)

type fakeRemote struct {
	runFailures  map[string]error
	remoteExists map[string]bool
	runCalls     []string
	uploadCalls  []string
	downloadCall []string
}

func (f *fakeRemote) OpenShell(conn domain.Connection, password string) error {
	return nil
}

func (f *fakeRemote) RunCommand(conn domain.Connection, password string, command string, stdout io.Writer, stderr io.Writer) error {
	f.runCalls = append(f.runCalls, conn.Name+":"+command)
	if stdout != nil {
		fmt.Fprintf(stdout, "out-%s\n", conn.Name)
	}
	if err := f.runFailures[conn.Name]; err != nil {
		return err
	}
	return nil
}

func (f *fakeRemote) ListRemote(conn domain.Connection, password string, targetPath string) ([]domain.FileEntry, string, error) {
	return nil, "", nil
}

func (f *fakeRemote) PathExists(conn domain.Connection, password string, targetPath string) (bool, error) {
	return f.remoteExists[conn.Name+"\x00"+targetPath], nil
}

func (f *fakeRemote) Upload(conn domain.Connection, password string, localPath string, remoteDir string, progress func(domain.TransferProgress)) error {
	f.uploadCalls = append(f.uploadCalls, conn.Name+":"+localPath+"->"+remoteDir)
	return nil
}

func (f *fakeRemote) Download(conn domain.Connection, password string, remotePath string, localDir string, progress func(domain.TransferProgress)) error {
	f.downloadCall = append(f.downloadCall, conn.Name+":"+remotePath+"->"+localDir)
	return nil
}

func (f *fakeRemote) HomeDir(conn domain.Connection, password string) (string, error) {
	return "/", nil
}

func TestParseRejectsConnectionNameShortcut(t *testing.T) {
	t.Parallel()

	if _, err := parse([]string{"prod", "--", "uname -a"}); err == nil {
		t.Fatal("parse() error = nil, want shortcut rejection")
	}
}

func TestParseRunUsesFileFlagAndRejectsInlineConflict(t *testing.T) {
	t.Parallel()

	parsed, err := parse([]string{"run", "-n", "prod", "--file", "deploy.sh"})
	if err != nil {
		t.Fatalf("parse(--file) error = %v", err)
	}
	if parsed.kind != taskRun || parsed.file != "deploy.sh" {
		t.Fatalf("parsed = %#v", parsed)
	}

	if _, err := parse([]string{"run", "-n", "prod", "--file", "deploy.sh", "--", "uname -a"}); err == nil {
		t.Fatal("parse(conflict) error = nil, want conflict error")
	}
}

func TestParseTransferForceAndDownloadBatchRule(t *testing.T) {
	t.Parallel()

	parsed, err := parse([]string{"upload", "-n", "prod,web", "-l", "./app", "-r", "/tmp", "-f", "-ff"})
	if err != nil {
		t.Fatalf("parse(upload) error = %v", err)
	}
	if parsed.kind != taskUpload || !parsed.force || !parsed.failFast || parsed.local != "./app" || parsed.remote != "/tmp" || len(parsed.names) != 2 {
		t.Fatalf("parsed = %#v", parsed)
	}

	if _, err := parse([]string{"download", "-n", "prod,web", "--remote", "/tmp/app", "--local", "./out"}); err == nil {
		t.Fatal("parse(download batch) error = nil, want error")
	}
}

func TestParseListSupportsGroupAndFilter(t *testing.T) {
	t.Parallel()

	parsed, err := parse([]string{"ls", "-g", "prod", "--filter", "web"})
	if err != nil {
		t.Fatalf("parse(ls) error = %v", err)
	}
	if parsed.kind != taskList || parsed.group != "prod" || parsed.filter != "web" {
		t.Fatalf("parsed = %#v", parsed)
	}
}

func TestParseVersion(t *testing.T) {
	t.Parallel()

	parsed, err := parse([]string{"version"})
	if err != nil {
		t.Fatalf("parse(version) error = %v", err)
	}
	if parsed.kind != taskVersion {
		t.Fatalf("parsed.kind = %v, want %v", parsed.kind, taskVersion)
	}
}

func TestVersionCommandPrintsBuildVersion(t *testing.T) {
	originalVersion := buildinfo.Version
	buildinfo.Version = "v0.1.1"
	t.Cleanup(func() {
		buildinfo.Version = originalVersion
	})

	var stdout bytes.Buffer
	code := New(nil, testTranslator(t, "en"), &stdout, io.Discard).Run([]string{"version"})
	if code != exitSuccess {
		t.Fatalf("exit code = %d, want %d", code, exitSuccess)
	}
	if stdout.String() != "v0.1.1\n" {
		t.Fatalf("stdout = %q, want version", stdout.String())
	}
}

func TestVersionCommandDefaultsToDev(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	code := New(nil, testTranslator(t, "en"), &stdout, io.Discard).Run([]string{"version"})
	if code != exitSuccess {
		t.Fatalf("exit code = %d, want %d", code, exitSuccess)
	}
	if stdout.String() != "dev\n" {
		t.Fatalf("stdout = %q, want dev", stdout.String())
	}
}

func TestListConnectionsSupportsGroupAndFilter(t *testing.T) {
	t.Parallel()

	services, _ := newTestServicesWithConnections(t, []testConnectionSpec{
		{Name: "prod-api", Host: "10.0.0.10", Description: "生产接口", Group: "生产"},
		{Name: "prod-job", Host: "10.0.0.11", Description: "生产任务", Group: "生产"},
		{Name: "dev-web", Host: "10.0.1.20", Description: "开发前端", Group: "开发"},
		{Name: "jump", Host: "10.0.9.9", Description: "未分组堡垒机"},
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := New(services, testTranslator(t, "zh-CN"), &stdout, &stderr).Run([]string{"ls", "--group", "生产", "--filter", "api"})
	if code != exitSuccess {
		t.Fatalf("exit code = %d, want %d, stderr = %q", code, exitSuccess, stderr.String())
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("stdout lines = %#v, want 2 lines", lines)
	}
	if !strings.Contains(lines[0], "名称") || !strings.Contains(lines[0], "主机") || !strings.Contains(lines[0], "描述") {
		t.Fatalf("header = %q, want localized columns", lines[0])
	}
	if !strings.Contains(lines[1], "prod-api") || !strings.Contains(lines[1], "10.0.0.10") || !strings.Contains(lines[1], "生产接口") {
		t.Fatalf("row = %q, want filtered connection", lines[1])
	}
	if strings.Contains(stdout.String(), "prod-job") || strings.Contains(stdout.String(), "dev-web") || strings.Contains(stdout.String(), "jump") {
		t.Fatalf("stdout = %q, want only grouped+filtered row", stdout.String())
	}
}

func TestListConnectionsSupportsUngroupedAlias(t *testing.T) {
	t.Parallel()

	services, _ := newTestServicesWithConnections(t, []testConnectionSpec{
		{Name: "prod-api", Host: "10.0.0.10", Description: "生产接口", Group: "生产"},
		{Name: "jump", Host: "10.0.9.9", Description: "未分组堡垒机"},
	})
	var stdout bytes.Buffer

	code := New(services, testTranslator(t, "en"), &stdout, io.Discard).Run([]string{"ls", "--group", "ungrouped"})
	if code != exitSuccess {
		t.Fatalf("exit code = %d, want %d", code, exitSuccess)
	}
	if !strings.Contains(stdout.String(), "jump") {
		t.Fatalf("stdout = %q, want ungrouped connection", stdout.String())
	}
	if strings.Contains(stdout.String(), "prod-api") {
		t.Fatalf("stdout = %q, want only ungrouped connections", stdout.String())
	}
}

func TestListConnectionsFailsWhenGroupMissing(t *testing.T) {
	t.Parallel()

	services, _ := newTestServices(t, []string{"prod"})
	var stderr bytes.Buffer

	code := New(services, testTranslator(t, "zh-CN"), io.Discard, &stderr).Run([]string{"ls", "--group", "不存在"})
	if code != exitFailure {
		t.Fatalf("exit code = %d, want %d", code, exitFailure)
	}
	if !strings.Contains(stderr.String(), "分组 \"不存在\" 不存在") {
		t.Fatalf("stderr = %q, want missing group message", stderr.String())
	}
}

func TestRunBatchContinuesAndReturnsFailure(t *testing.T) {
	t.Parallel()

	services, remote := newTestServices(t, []string{"prod", "web"})
	remote.runFailures = map[string]error{"prod": fmt.Errorf("boom")}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := New(services, testTranslator(t, "en"), &stdout, &stderr).Run([]string{"run", "-n", "prod,web", "--", "uptime"})
	if code != exitFailure {
		t.Fatalf("exit code = %d, want %d", code, exitFailure)
	}
	if len(remote.runCalls) != 2 {
		t.Fatalf("runCalls = %#v, want 2 calls", remote.runCalls)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("[web] out-web")) {
		t.Fatalf("stdout = %q, want prefixed web output", stdout.String())
	}
}

func TestUploadRequiresForceWhenRemoteTargetExists(t *testing.T) {
	t.Parallel()

	services, remote := newTestServices(t, []string{"prod"})
	remote.remoteExists["prod\x00/tmp/app"] = true
	var stderr bytes.Buffer

	code := New(services, testTranslator(t, "en"), io.Discard, &stderr).Run([]string{"upload", "-n", "prod", "--local", "./app", "--remote", "/tmp"})
	if code != exitFailure {
		t.Fatalf("exit code = %d, want %d", code, exitFailure)
	}
	if len(remote.uploadCalls) != 0 {
		t.Fatalf("uploadCalls = %#v, want none", remote.uploadCalls)
	}

	code = New(services, testTranslator(t, "en"), io.Discard, &stderr).Run([]string{"upload", "-n", "prod", "--local", "./app", "--remote", "/tmp", "-f"})
	if code != exitSuccess {
		t.Fatalf("exit code = %d, want %d", code, exitSuccess)
	}
	if len(remote.uploadCalls) != 1 {
		t.Fatalf("uploadCalls = %#v, want 1 call", remote.uploadCalls)
	}
}

func TestDownloadRequiresForceWhenLocalTargetExists(t *testing.T) {
	t.Parallel()

	services, remote := newTestServices(t, []string{"prod"})
	localDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(localDir, "app.log"), []byte("old"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	var stderr bytes.Buffer

	code := New(services, testTranslator(t, "en"), io.Discard, &stderr).Run([]string{"download", "-n", "prod", "--remote", "/tmp/app.log", "--local", localDir})
	if code != exitFailure {
		t.Fatalf("exit code = %d, want %d", code, exitFailure)
	}
	if len(remote.downloadCall) != 0 {
		t.Fatalf("downloadCall = %#v, want none", remote.downloadCall)
	}

	code = New(services, testTranslator(t, "en"), io.Discard, &stderr).Run([]string{"download", "-n", "prod", "--remote", "/tmp/app.log", "--local", localDir, "-f"})
	if code != exitSuccess {
		t.Fatalf("exit code = %d, want %d", code, exitSuccess)
	}
	if len(remote.downloadCall) != 1 {
		t.Fatalf("downloadCall = %#v, want 1 call", remote.downloadCall)
	}
}

func TestCommandUsesTranslatorForHelpAndErrors(t *testing.T) {
	t.Parallel()

	services, _ := newTestServices(t, []string{"prod"})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := New(services, testTranslator(t, "zh-CN"), &stdout, &stderr).Run([]string{"help"})
	if code != exitSuccess {
		t.Fatalf("exit code = %d, want %d", code, exitSuccess)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("sshm 无头模式")) {
		t.Fatalf("stdout = %q, want Chinese help", stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("sshm version")) {
		t.Fatalf("stdout = %q, want version usage", stdout.String())
	}

	code = New(services, testTranslator(t, "en"), &stdout, &stderr).Run([]string{"run"})
	if code != exitFailure {
		t.Fatalf("exit code = %d, want %d", code, exitFailure)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("Error: -n/--name is required")) {
		t.Fatalf("stderr = %q, want English error", stderr.String())
	}
}

type testConnectionSpec struct {
	Name        string
	Host        string
	Description string
	Group       string
}

func newTestServices(t *testing.T, names []string) (*app.Services, *fakeRemote) {
	specs := make([]testConnectionSpec, 0, len(names))
	for _, name := range names {
		specs = append(specs, testConnectionSpec{
			Name: name,
			Host: name + ".example.com",
		})
	}
	return newTestServicesWithConnections(t, specs)
}

func newTestServicesWithConnections(t *testing.T, specs []testConnectionSpec) (*app.Services, *fakeRemote) {
	t.Helper()

	tempDir := t.TempDir()
	repo, err := sqlite.Open(filepath.Join(tempDir, "sshm.db"))
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = repo.Close()
	})

	crypto, err := security.LoadOrCreateKey(filepath.Join(tempDir, "app.key"))
	if err != nil {
		t.Fatalf("LoadOrCreateKey() error = %v", err)
	}
	remote := &fakeRemote{
		runFailures:  map[string]error{},
		remoteExists: map[string]bool{},
	}
	services := app.NewServices(repo, crypto, remote, "~/.ssh/id_rsa")
	groupIDs := map[string]*int64{}
	for _, spec := range specs {
		var groupID *int64
		if strings.TrimSpace(spec.Group) != "" {
			if cached, ok := groupIDs[spec.Group]; ok {
				groupID = cached
			} else {
				group, err := services.Groups.Create(spec.Group)
				if err != nil {
					t.Fatalf("CreateGroup(%q) error = %v", spec.Group, err)
				}
				groupID = &group.ID
				groupIDs[spec.Group] = groupID
			}
		}
		if _, err := services.Connections.Create(domain.ConnectionInput{
			GroupID:     groupID,
			Name:        spec.Name,
			Host:        defaultString(spec.Host, spec.Name+".example.com"),
			Port:        22,
			Username:    "root",
			AuthType:    domain.AuthTypePrivateKey,
			Description: spec.Description,
		}); err != nil {
			t.Fatalf("Create(%q) error = %v", spec.Name, err)
		}
	}
	return services, remote
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func testTranslator(t *testing.T, language string) *i18n.Translator {
	t.Helper()

	translator, err := i18n.New(language)
	if err != nil {
		t.Fatalf("i18n.New() error = %v", err)
	}
	return translator
}

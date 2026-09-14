package ssh

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/sftp"
)

func newMemorySFTP(t *testing.T) *SFTP {
	t.Helper()
	serverConn, clientConn := net.Pipe()
	server := sftp.NewRequestServer(serverConn, sftp.InMemHandler())
	go func() { _ = server.Serve() }()
	client, err := sftp.NewClientPipe(clientConn, clientConn)
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = server.Close()
		_ = client.Close()
	})
	return &SFTP{client: client}
}

func TestSFTPTransferRoundTripAndRemove(t *testing.T) {
	client := newMemorySFTP(t)
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "hello.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := client.client.Mkdir("/remote"); err != nil {
		t.Fatal(err)
	}
	if err := client.Mkdir("/remote/new-folder"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.client.Stat("/remote/new-folder"); err != nil {
		t.Fatalf("远程文件夹未创建: %v", err)
	}
	if err := client.Mkdir("/remote/new-folder"); err == nil {
		t.Fatal("重名文件夹未返回错误")
	}
	if err := client.Upload(context.Background(), source, "/remote/source", nil); err != nil {
		t.Fatal(err)
	}
	if err := client.Copy(context.Background(), "/remote/source", "/remote/copy", nil); err != nil {
		t.Fatal(err)
	}
	down := filepath.Join(root, "download")
	if err := client.Download(context.Background(), "/remote/copy", down, nil); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(down, "hello.txt"))
	if err != nil || string(content) != "hello" {
		t.Fatalf("下载内容错误: %q, %v", content, err)
	}
	if err := client.RemoveAll(context.Background(), "/remote/source", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := client.client.Stat("/remote/source"); !os.IsNotExist(err) {
		t.Fatalf("远程源目录未删除: %v", err)
	}
}

func TestSFTPTransferCancellationCleansTargets(t *testing.T) {
	client := newMemorySFTP(t)
	root := t.TempDir()
	source := filepath.Join(root, "large.bin")
	if err := os.WriteFile(source, make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := client.client.Mkdir("/remote"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := client.Upload(ctx, source, "/remote/large.bin", nil); err == nil {
		t.Fatal("取消上传未返回错误")
	}
	if _, err := client.client.Stat("/remote/large.bin"); !os.IsNotExist(err) {
		t.Fatalf("取消上传留下目标: %v", err)
	}
}

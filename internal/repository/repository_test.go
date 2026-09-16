package repository

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInitializeUnlockAndDatabaseEncryption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sshm.db")
	store, err := Initialize(path, []byte("correct horse"))
	if err != nil {
		t.Fatal(err)
	}
	if status, err := Inspect(path); err != nil || status != Ready {
		t.Fatalf("Inspect() = %v, %v", status, err)
	}
	if key, err := os.ReadFile(path + ".key"); err != nil || len(key) != sealedKeySize {
		t.Fatalf("密钥文件长度 = %d, %v", len(key), err)
	}
	if _, err = store.CreateCredential(NewCredential{Name: "加密检查", Type: "passwd", Content: []byte("database-encryption-marker")}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Unlock(path, []byte("wrong")); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("Unlock() error = %v, want ErrInvalidPassword", err)
	}
	store, err = Unlock(path, []byte("correct horse"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.HasPrefix(data, []byte("SQLite format 3")) {
		t.Fatal("数据库仍包含 SQLite 明文文件头")
	}
	if bytes.Contains(data, []byte("database-encryption-marker")) {
		t.Fatal("数据库仍包含凭据明文")
	}
}

func TestChangePasswordPreservesCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sshm.db")
	store, err := Initialize(path, []byte("old-password"))
	if err != nil {
		t.Fatal(err)
	}
	credential, err := store.CreateCredential(NewCredential{Name: "登录密码", Type: "passwd", Content: []byte("secret")})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := store.CreateConnection(NewConnection{Name: "开发机", Host: "dev.example.com", Port: 22, Username: "root", CredentialID: credential.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.ChangePassword([]byte("wrong-password"), []byte("new-password")); err != ErrInvalidPassword {
		t.Fatalf("错误原密码的结果 = %v, want ErrInvalidPassword", err)
	}
	if err = store.ChangePassword([]byte("old-password"), []byte("new-password")); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = Unlock(path, []byte("old-password")); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("旧密码解锁结果 = %v, want ErrInvalidPassword", err)
	}
	store, err = Unlock(path, []byte("new-password"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	_, secret, err := store.SSHConnection(connection.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(secret)
	if string(secret) != "secret" {
		t.Fatalf("修改密码后的凭据 = %q, want secret", secret)
	}
}

func TestInspectEmptyDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.db")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	status, err := Inspect(path)
	if err != nil || status != Uninitialized {
		t.Fatalf("Inspect() = %v, %v", status, err)
	}
}

func TestInspectMissingDatabaseIgnoresOrphanKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.db")
	if err := os.WriteFile(path+".key", []byte("orphan"), 0600); err != nil {
		t.Fatal(err)
	}
	status, err := Inspect(path)
	if err != nil || status != NotFound {
		t.Fatalf("Inspect() = %v, %v", status, err)
	}
}

func TestListConnections(t *testing.T) {
	store, err := Initialize(filepath.Join(t.TempDir(), "sshm.db"), []byte("password"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.db.Exec(`INSERT INTO credentials(name,type,content) VALUES('key-1','key',x'00')`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO connections(name,host,username,credential_id,use_count,remark) VALUES('开发机','dev.example.com','root',1,7,'测试')`); err != nil {
		t.Fatal(err)
	}
	connections, err := store.ListConnections()
	if err != nil || len(connections) != 1 {
		t.Fatalf("ListConnections() = %#v, %v", connections, err)
	}
	if connections[0].Name != "开发机" || connections[0].Credential != "key" || connections[0].CredentialName != "key-1" || connections[0].UseCount != 7 || connections[0].LastUsedAt.Valid {
		t.Fatalf("连接数据 = %#v", connections[0])
	}
}

func TestSSHConnectionAndMarkUsed(t *testing.T) {
	store, err := Initialize(filepath.Join(t.TempDir(), "sshm.db"), []byte("password"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	credential, err := store.CreateCredential(NewCredential{Name: "登录密码", Type: "passwd", Content: []byte("secret")})
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.CreateConnection(NewConnection{Name: "开发机", Host: "dev.example.com", Port: 2222, Username: "root", CredentialID: credential.ID})
	if err != nil {
		t.Fatal(err)
	}
	connection, secret, err := store.SSHConnection(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(secret)
	if connection.Host != "dev.example.com" || connection.Port != 2222 || connection.Credential != "passwd" || string(secret) != "secret" {
		t.Fatalf("SSH 连接数据 = %#v, secret=%q", connection, secret)
	}
	if err := store.MarkUsed(created.ID); err != nil {
		t.Fatal(err)
	}
	connections, err := store.ListConnections()
	if err != nil || connections[0].UseCount != 1 || !connections[0].LastUsedAt.Valid {
		t.Fatalf("连接使用状态 = %#v, %v", connections, err)
	}
}

func TestCreateCredentialAndReuseItForConnections(t *testing.T) {
	store, err := Initialize(filepath.Join(t.TempDir(), "sshm.db"), []byte("password"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	credential, err := store.CreateCredential(NewCredential{Name: "生产服务器密码", Type: "passwd", Content: []byte("secret")})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := store.CreateConnection(NewConnection{Name: "生产服务器", Host: "server.example.com", Port: 2222, Username: "root", CredentialID: credential.ID, Remark: "主站"})
	if err != nil {
		t.Fatal(err)
	}
	if connection.ID == 0 || connection.Credential != "passwd" || connection.CredentialID != credential.ID {
		t.Fatalf("创建结果 = %#v", connection)
	}
	var content []byte
	if err := store.db.QueryRow("SELECT content FROM credentials WHERE name=?", "生产服务器密码").Scan(&content); err != nil {
		t.Fatal(err)
	}
	if string(content) != "secret" {
		t.Fatalf("凭据内容 = %q", content)
	}
	credentials, err := store.ListCredentials()
	if err != nil || len(credentials) != 1 {
		t.Fatalf("凭据列表 = %#v, %v", credentials, err)
	}
	connection, err = store.CreateConnection(NewConnection{Name: "备用服务器", Host: "backup.example.com", Port: 22, Username: "root", CredentialID: credentials[0].ID})
	if err != nil || connection.Credential != "passwd" {
		t.Fatalf("复用凭据创建结果 = %#v, %v", connection, err)
	}
	connections, err := store.ListConnections()
	if err != nil || len(connections) != 2 {
		t.Fatalf("连接列表 = %#v, %v", connections, err)
	}
}

func TestCreateCredentialEncryptsPrivateKey(t *testing.T) {
	store, err := Initialize(filepath.Join(t.TempDir(), "sshm.db"), []byte("password"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	privateKey := "-----BEGIN OPENSSH PRIVATE KEY-----\nmultiline\n-----END OPENSSH PRIVATE KEY-----\n"
	credential, err := store.CreateCredential(NewCredential{Name: "部署私钥", Type: "key", Content: []byte(privateKey)})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := store.CreateConnection(NewConnection{Name: "私钥服务器", Host: "key.example.com", Port: 22, Username: "git", CredentialID: credential.ID})
	if err != nil || connection.Credential != "key" {
		t.Fatalf("私钥连接创建结果 = %#v, %v", connection, err)
	}
	var content []byte
	if err := store.db.QueryRow("SELECT content FROM credentials WHERE name=?", "部署私钥").Scan(&content); err != nil {
		t.Fatal(err)
	}
	if string(content) != privateKey {
		t.Fatalf("私钥内容 = %q", content)
	}
}

func TestUpdateAndDeleteConnection(t *testing.T) {
	store, err := Initialize(filepath.Join(t.TempDir(), "sshm.db"), []byte("password"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	credential, err := store.CreateCredential(NewCredential{Name: "凭据", Type: "passwd", Content: []byte("secret")})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := store.CreateConnection(NewConnection{Name: "旧名称", Host: "old", Port: 22, Username: "root", CredentialID: credential.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkUsed(connection.ID); err != nil {
		t.Fatal(err)
	}
	updated, err := store.UpdateConnection(connection.ID, NewConnection{Name: "新名称", Host: "new", Port: 2222, Username: "admin", CredentialID: credential.ID, Remark: "备注"})
	if err != nil || updated.Name != "新名称" || updated.Port != 2222 || updated.UseCount != 1 || !updated.LastUsedAt.Valid {
		t.Fatalf("更新连接 = %#v, %v", updated, err)
	}
	if err := store.DeleteConnection(connection.ID); err != nil {
		t.Fatal(err)
	}
	if rows, err := store.ListConnections(); err != nil || len(rows) != 0 {
		t.Fatalf("删除连接后列表 = %#v, %v", rows, err)
	}
}

func TestManageCredentials(t *testing.T) {
	store, err := Initialize(filepath.Join(t.TempDir(), "sshm.db"), []byte("password"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	credential, err := store.CreateCredential(NewCredential{Name: "共享密码", Type: "passwd", Content: []byte("old-secret")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateConnection(NewConnection{Name: "开发机", Host: "dev.example.com", Port: 22, Username: "root", CredentialID: credential.ID}); err != nil {
		t.Fatal(err)
	}
	credentials, err := store.ListCredentials()
	if err != nil || len(credentials) != 1 || credentials[0].ConnectionCount != 1 {
		t.Fatalf("凭据关联数量 = %#v, %v", credentials, err)
	}
	var oldContent []byte
	if err := store.db.QueryRow("SELECT content FROM credentials WHERE id=?", credential.ID).Scan(&oldContent); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateCredential(credential.ID, "共享私钥", "key", nil); err != nil {
		t.Fatal(err)
	}
	var credentialType string
	var content []byte
	if err := store.db.QueryRow("SELECT type,content FROM credentials WHERE id=?", credential.ID).Scan(&credentialType, &content); err != nil {
		t.Fatal(err)
	}
	if credentialType != "key" || !bytes.Equal(content, oldContent) {
		t.Fatal("空内容编辑未保留原凭据")
	}
	if err := store.UpdateCredential(credential.ID, "共享私钥", "key", []byte("new-secret")); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow("SELECT content FROM credentials WHERE id=?", credential.ID).Scan(&content); err != nil {
		t.Fatal(err)
	}
	if string(content) != "new-secret" {
		t.Fatalf("更新后的凭据内容 = %q", content)
	}
	if err := store.DeleteCredential(credential.ID); err == nil {
		t.Fatal("删除了仍有关联的凭据")
	}
	unused, err := store.CreateCredential(NewCredential{Name: "未使用", Type: "passwd", Content: []byte("secret")})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteCredential(unused.ID); err != nil {
		t.Fatal(err)
	}
}

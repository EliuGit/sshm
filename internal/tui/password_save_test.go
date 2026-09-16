package tui

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPasswordCipher(t *testing.T) {
	password := []byte("应用密码")
	data, err := encryptPassword(password, "machine-a")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, password) {
		t.Fatal("保存内容不应包含明文密码")
	}
	decrypted, err := decryptPassword(data, "machine-a")
	if err != nil || !bytes.Equal(decrypted, password) {
		t.Fatalf("解密结果 = %q, %v", decrypted, err)
	}
	if _, err = decryptPassword(data, "machine-b"); err == nil {
		t.Fatal("Machine ID 变化后不应解密成功")
	}
	data[0] ^= 0xff
	if _, err = decryptPassword(data, "machine-a"); err == nil {
		t.Fatal("损坏的文件不应解密成功")
	}
}

func TestSavedPassword(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sshm.db")
	for _, password := range []string{"first-password", "changed-password"} {
		if err := savePassword(dbPath, []byte(password)); err != nil {
			t.Fatal(err)
		}
		decrypted, err := loadPassword(dbPath)
		if err != nil || string(decrypted) != password {
			t.Fatalf("解密结果 = %q, %v", decrypted, err)
		}
		clear(decrypted)
	}
	if err := removePassword(dbPath); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPassword(dbPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("删除后的读取错误 = %v", err)
	}
}

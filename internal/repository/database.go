package repository

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/vfs/adiantum"

	"sshm/internal/share"
)

//go:embed schema.sql
var schemaFS embed.FS

func keyPath(path string) string { return path + ".key" }

// Inspect 只检查数据库和密钥文件是否齐全；数据库结构在输入密码后检查。
func Inspect(path string) (Status, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return NotFound, nil
	}
	if err != nil {
		return 0, err
	}
	if !info.Mode().IsRegular() {
		return 0, errors.New("数据库路径不是普通文件")
	}
	if info.Size() == 0 {
		if _, keyErr := os.Stat(keyPath(path)); errors.Is(keyErr, os.ErrNotExist) {
			return Uninitialized, nil
		}
	}
	if _, _, _, err = readSealedKey(keyPath(path)); errors.Is(err, os.ErrNotExist) {
		return 0, errors.New("数据库秘钥已损坏,请删除数据库文件和密钥文件后重新初始化")
	} else if err != nil {
		return 0, err
	}
	return Ready, nil
}

// Initialize 创建由随机数据密钥加密的新数据库，并使用应用密码封装数据密钥。
func Initialize(path string, password []byte) (*Store, error) {
	if len(password) == 0 {
		return nil, errors.New("主密码不能为空")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	dataKey := make([]byte, keySize)
	if _, err := rand.Read(dataKey); err != nil {
		return nil, err
	}
	fail := func(err error) (*Store, error) {
		clear(dataKey)
		_ = os.Remove(path)
		_ = os.Remove(keyPath(path))
		return nil, err
	}
	salt, nonce, ciphertext, err := sealDataKey(password, dataKey)
	if err != nil {
		return fail(err)
	}
	if err = writeSealedKey(keyPath(path), salt, nonce, ciphertext); err != nil {
		return fail(err)
	}
	db, err := openEncrypted(path, dataKey, "rwc")
	if err != nil {
		return fail(err)
	}
	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		_ = db.Close()
		return fail(err)
	}
	tx, err := db.Begin()
	if err != nil {
		_ = db.Close()
		return fail(err)
	}
	if _, err = tx.Exec(string(schema)); err != nil {
		_ = tx.Rollback()
		_ = db.Close()
		return fail(err)
	}
	if err = tx.Commit(); err != nil {
		_ = db.Close()
		return fail(err)
	}
	return &Store{db: db, dataKey: dataKey, keyPath: keyPath(path)}, nil
}

// Unlock 使用应用密码解封数据密钥并打开加密数据库。
func Unlock(path string, password []byte) (*Store, error) {
	salt, nonce, ciphertext, err := readSealedKey(keyPath(path))
	if err != nil {
		return nil, err
	}
	dataKey, err := unsealDataKey(password, salt, nonce, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPassword, err)
	}
	db, err := openEncrypted(path, dataKey, "rw")
	if err != nil {
		clear(dataKey)
		return nil, err
	}
	var version int
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		_ = db.Close()
		clear(dataKey)
		return nil, fmt.Errorf("打开加密数据库: %w", err)
	}
	if version != 1 {
		_ = db.Close()
		clear(dataKey)
		return nil, fmt.Errorf("不支持的数据库版本: %d", version)
	}
	if _, err = db.Exec("SELECT content FROM credentials LIMIT 0"); err != nil {
		_ = db.Close()
		clear(dataKey)
		return nil, errors.New("数据库不是 SSHM 加密格式")
	}
	return &Store{db: db, dataKey: dataKey, keyPath: keyPath(path)}, nil
}

func openEncrypted(path string, dataKey []byte, mode string) (*sql.DB, error) {
	dsn := "file:" + filepath.ToSlash(path) + "?mode=" + mode + "&vfs=adiantum"
	hexKey := hex.EncodeToString(dataKey)
	db, err := driver.Open(dsn, func(conn *sqlite3.Conn) error {
		if err := conn.Exec("PRAGMA hexkey='" + hexKey + "'"); err != nil {
			return err
		}
		if err := conn.Exec("PRAGMA foreign_keys=ON"); err != nil {
			return err
		}
		return conn.Exec("PRAGMA temp_store=memory")
	})
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// ChangePassword 验证原密码，并使用新密码重新封装数据库数据密钥。
func (s *Store) ChangePassword(oldPassword, newPassword []byte) error {
	if len(oldPassword) == 0 {
		return ErrInvalidPassword
	}
	if len(newPassword) == 0 {
		return errors.New("新密码不能为空")
	}
	salt, nonce, ciphertext, err := readSealedKey(s.keyPath)
	if err != nil {
		return err
	}
	dataKey, err := unsealDataKey(oldPassword, salt, nonce, ciphertext)
	if err != nil {
		return ErrInvalidPassword
	}
	defer clear(dataKey)
	if !bytes.Equal(dataKey, s.dataKey) {
		return errors.New("数据库密钥与当前会话不一致")
	}
	salt, nonce, ciphertext, err = sealDataKey(newPassword, s.dataKey)
	if err != nil {
		return err
	}
	return writeSealedKey(s.keyPath, salt, nonce, ciphertext)
}

func readSealedKey(path string) ([]byte, []byte, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(data) != sealedKeySize {
		return nil, nil, nil, errors.New("数据库密钥文件格式无效")
	}
	salt := data[:saltSize]
	nonce := data[saltSize : saltSize+24]
	ciphertext := data[saltSize+24:]
	return salt, nonce, ciphertext, nil
}

func writeSealedKey(path string, salt, nonce, ciphertext []byte) error {
	data := make([]byte, 0, sealedKeySize)
	data = append(data, salt...)
	data = append(data, nonce...)
	data = append(data, ciphertext...)
	return share.WriteSecretFile(path, data)
}

// Close 清理内存中的数据密钥并关闭数据库。
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	clear(s.dataKey)
	s.dataKey = nil
	return s.db.Close()
}

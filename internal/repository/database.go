package repository

import (
	"bytes"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaFS embed.FS

// Inspect 检查数据库文件和新 schema，不会创建文件。
func Inspect(path string) (Status, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return NotFound, nil
	} else if err != nil {
		return 0, err
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		return 0, err
	}
	defer db.Close()
	var version int
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return 0, err
	}
	var n int
	if err = db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='master_key'").Scan(&n); err != nil {
		return 0, err
	}
	if n == 0 {
		if version != 0 {
			return 0, fmt.Errorf("不支持的数据库版本: %d", version)
		}
		if err = db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&n); err != nil {
			return 0, err
		}
		if n == 0 {
			return Uninitialized, nil
		}
		return 0, errors.New("数据库不是 SSHM 新版格式")
	}
	if version != 1 {
		return 0, fmt.Errorf("不支持的数据库版本: %d", version)
	}
	if err = db.QueryRow("SELECT count(*) FROM master_key WHERE id=1").Scan(&n); err != nil {
		return 0, err
	}
	if n == 0 {
		return Uninitialized, nil
	}
	var salt, nonce, ciphertext []byte
	if err = db.QueryRow("SELECT kdf_salt, nonce, encrypted_data_key FROM master_key WHERE id=1").Scan(&salt, &nonce, &ciphertext); err != nil {
		return 0, err
	}
	if err = validateMasterKey(salt, nonce, ciphertext); err != nil {
		return 0, err
	}
	return Ready, nil
}

// Initialize 创建新数据库并用主密码封装随机数据密钥。
func Initialize(path string, password []byte) (*Store, error) {
	if len(password) == 0 {
		return nil, errors.New("主密码不能为空")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err = db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, err
	}
	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		db.Close()
		return nil, err
	}
	tx, err := db.Begin()
	if err != nil {
		db.Close()
		return nil, err
	}
	if _, err = tx.Exec(string(schema)); err != nil {
		tx.Rollback()
		db.Close()
		return nil, err
	}
	dataKey, err := sealStore(tx, password)
	if err != nil {
		tx.Rollback()
		db.Close()
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		clear(dataKey)
		db.Close()
		return nil, err
	}
	return &Store{db: db, dataKey: dataKey}, nil
}

// Unlock 使用主密码解密数据库中的数据密钥。
func Unlock(path string, password []byte) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=rw")
	if err != nil {
		return nil, err
	}
	if _, err = db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, err
	}
	var salt, nonce, ciphertext []byte
	if err = db.QueryRow("SELECT kdf_salt, nonce, encrypted_data_key FROM master_key WHERE id=1").Scan(&salt, &nonce, &ciphertext); err != nil {
		db.Close()
		return nil, err
	}
	if err = validateMasterKey(salt, nonce, ciphertext); err != nil {
		db.Close()
		return nil, err
	}
	dataKey, err := unsealDataKey(password, salt, nonce, ciphertext)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("%w: %v", ErrInvalidPassword, err)
	}
	return &Store{db: db, dataKey: dataKey}, nil
}

// ChangePassword 验证原密码，并使用非空的新密码重新封装当前数据密钥。
// 修改只更新主密钥记录，不会重新加密凭据；返回 nil 后旧密码立即失效。
func (s *Store) ChangePassword(oldPassword, newPassword []byte) error {
	if len(oldPassword) == 0 {
		return ErrInvalidPassword
	}
	if len(newPassword) == 0 {
		return errors.New("新密码不能为空")
	}
	var salt, nonce, ciphertext []byte
	if err := s.db.QueryRow("SELECT kdf_salt, nonce, encrypted_data_key FROM master_key WHERE id=1").Scan(&salt, &nonce, &ciphertext); err != nil {
		return err
	}
	if err := validateMasterKey(salt, nonce, ciphertext); err != nil {
		return err
	}
	dataKey, err := unsealDataKey(oldPassword, salt, nonce, ciphertext)
	if err != nil {
		return ErrInvalidPassword
	}
	defer clear(dataKey)
	if !bytes.Equal(dataKey, s.dataKey) {
		return errors.New("主密钥与当前会话不一致")
	}
	salt, nonce, ciphertext, err = sealDataKey(newPassword, s.dataKey)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("UPDATE master_key SET kdf_salt=?, nonce=?, encrypted_data_key=? WHERE id=1", salt, nonce, ciphertext)
	return err
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

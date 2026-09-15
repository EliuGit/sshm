package repository

import (
	"crypto/rand"
	"database/sql"
	"errors"

	"sshm/internal/i18n"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

const (
	argonMemory  = 64 * 1024
	argonTime    = 3
	argonThreads = 1
	keySize      = 32
	saltSize     = 16
)

// Encrypt 使用当前进程的数据密钥加密凭据明文，返回随机 nonce 和密文。
func (s *Store) Encrypt(plaintext []byte) ([]byte, []byte, error) {
	aead, err := chacha20poly1305.NewX(s.dataKey)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	if _, err = rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return nonce, aead.Seal(nil, nonce, plaintext, nil), nil
}

// Decrypt 使用当前进程的数据密钥解密凭据密文。
func (s *Store) Decrypt(nonce, ciphertext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(s.dataKey)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nonce, ciphertext, nil)
}

func validateMasterKey(salt, nonce, ciphertext []byte) error {
	if len(salt) != saltSize || len(nonce) != chacha20poly1305.NonceSizeX || len(ciphertext) != keySize+chacha20poly1305.Overhead {
		return errors.New(i18n.T("Invalid master key record"))
	}
	return nil
}

func unsealDataKey(password, salt, nonce, ciphertext []byte) ([]byte, error) {
	key := argon2.IDKey(password, salt, argonTime, argonMemory, argonThreads, keySize)
	defer clear(key)
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nonce, ciphertext, nil)
}

func sealDataKey(password, dataKey []byte) ([]byte, []byte, []byte, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, nil, nil, err
	}
	key := argon2.IDKey(password, salt, argonTime, argonMemory, argonThreads, keySize)
	defer clear(key)
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, nil, nil, err
	}
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	if _, err = rand.Read(nonce); err != nil {
		return nil, nil, nil, err
	}
	return salt, nonce, aead.Seal(nil, nonce, dataKey, nil), nil
}

func sealStore(db *sql.Tx, password []byte) ([]byte, error) {
	dataKey := make([]byte, keySize)
	if _, err := rand.Read(dataKey); err != nil {
		return nil, err
	}
	salt, nonce, ciphertext, err := sealDataKey(password, dataKey)
	if err != nil {
		clear(dataKey)
		return nil, err
	}
	if _, err = db.Exec("INSERT INTO master_key(id,kdf_salt,nonce,encrypted_data_key) VALUES(1,?,?,?)", salt, nonce, ciphertext); err != nil {
		clear(dataKey)
		return nil, err
	}
	return dataKey, nil
}

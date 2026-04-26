package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

type Crypto struct {
	key []byte
}

func LoadOrCreateKey(path string) (*Crypto, error) {
	key, err := os.ReadFile(path)
	if err == nil {
		if len(key) != 32 {
			return nil, fmt.Errorf("invalid key length: %d", len(key))
		}
		return &Crypto{key: key}, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}

	key = make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, key, 0600); err != nil {
		return nil, err
	}
	return &Crypto{key: key}, nil
}

func (c *Crypto) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (c *Crypto) Decrypt(ciphertext string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, payload := raw[:nonceSize], raw[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, payload, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

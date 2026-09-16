package repository

import (
	"crypto/rand"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

const (
	argonMemory   = 64 * 1024
	argonTime     = 3
	argonThreads  = 1
	keySize       = 32
	saltSize      = 16
	sealedKeySize = saltSize + chacha20poly1305.NonceSizeX + keySize + chacha20poly1305.Overhead
)

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

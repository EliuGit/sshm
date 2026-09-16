package tui

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/denisbrodbeck/machineid"
	"golang.org/x/crypto/chacha20poly1305"
)

const (
	saveFile    = ".sshm_save"
	saveMagic   = "SSRP"
	saveVersion = 1
	saveHeader  = len(saveMagic) + 1
)

func savePath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, saveFile), nil
}

func loadPassword() ([]byte, error) {
	path, err := savePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	id, err := machineID()
	if err != nil {
		return nil, err
	}
	return decryptPassword(data, id)
}

func savePassword(password []byte) error {
	if len(password) == 0 {
		return errors.New("记住的密码不能为空")
	}
	id, err := machineID()
	if err != nil {
		return err
	}
	data, err := encryptPassword(password, id)
	if err != nil {
		return err
	}
	path, err := savePath()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".save-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err = tmp.Chmod(0600); err == nil {
		_, err = tmp.Write(data)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(tmpPath, path); err == nil {
		return nil
	}
	if runtime.GOOS != "windows" {
		return err
	}
	// Windows 无法直接覆盖已有文件，删除后重命名作为兼容路径。
	if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return err
	}
	return os.Rename(tmpPath, path)
}

func encryptPassword(password []byte, id string) ([]byte, error) {
	key := passwordKey(id)
	defer clear(key)
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	data := make([]byte, saveHeader+len(nonce))
	copy(data, saveMagic)
	data[len(saveMagic)] = saveVersion
	copy(data[saveHeader:], nonce)
	return aead.Seal(data, nonce, password, nil), nil
}

func decryptPassword(data []byte, id string) ([]byte, error) {
	if len(data) < saveHeader+chacha20poly1305.NonceSizeX+chacha20poly1305.Overhead || string(data[:len(saveMagic)]) != saveMagic || data[len(saveMagic)] != saveVersion {
		return nil, errors.New("记住密码文件格式无效")
	}
	key := passwordKey(id)
	defer clear(key)
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	nonceEnd := saveHeader + chacha20poly1305.NonceSizeX
	return aead.Open(nil, data[saveHeader:nonceEnd], data[nonceEnd:], nil)
}

func removePassword() error {
	path, err := savePath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func passwordKey(id string) []byte {
	sum := sha256.Sum256([]byte("sshm/save-password/v1:" + id))
	return sum[:]
}

func machineID() (string, error) {
	id, err := machineid.ID()
	if err != nil {
		return "", fmt.Errorf("读取 Machine ID 失败: %w", err)
	}
	if strings.TrimSpace(id) == "" {
		return "", errors.New("Machine ID 为空")
	}
	return id, nil
}

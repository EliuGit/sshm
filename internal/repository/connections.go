package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"sshm/internal/i18n"
)

// ListConnections 返回数据库中的全部连接，按创建顺序读取；凭据密文不会被读取。
func (s *Store) ListConnections() ([]Connection, error) {
	rows, err := s.db.Query(`SELECT c.id, c.name, c.host, c.port, c.username, c.credential_id, cr.type, cr.name, c.use_count, c.last_used_at, c.remark
		FROM connections c JOIN credentials cr ON cr.id = c.credential_id ORDER BY c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Connection
	for rows.Next() {
		var c Connection
		if err := rows.Scan(&c.ID, &c.Name, &c.Host, &c.Port, &c.Username, &c.CredentialID, &c.Credential, &c.CredentialName, &c.UseCount, &c.LastUsedAt, &c.Remark); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

// SSHConnection 返回指定连接及解密后的凭据内容。
// 调用方使用完凭据后必须立即清空返回的字节切片，避免明文长期驻留内存。
func (s *Store) SSHConnection(id int64) (Connection, []byte, error) {
	if id <= 0 {
		return Connection{}, nil, errors.New(i18n.T("Connection does not exist"))
	}
	var c Connection
	var nonce, ciphertext []byte
	err := s.db.QueryRow(`SELECT c.id, c.name, c.host, c.port, c.username, c.credential_id, cr.type, cr.name, c.use_count, c.last_used_at, c.remark, cr.nonce, cr.ciphertext
		FROM connections c JOIN credentials cr ON cr.id = c.credential_id WHERE c.id=?`, id).
		Scan(&c.ID, &c.Name, &c.Host, &c.Port, &c.Username, &c.CredentialID, &c.Credential, &c.CredentialName, &c.UseCount, &c.LastUsedAt, &c.Remark, &nonce, &ciphertext)
	if err != nil {
		return Connection{}, nil, fmt.Errorf("%s: %w", i18n.T("Failed to read connection credential"), err)
	}
	credential, err := s.Decrypt(nonce, ciphertext)
	if err != nil {
		return Connection{}, nil, fmt.Errorf("%s: %w", i18n.T("Failed to decrypt connection credential"), err)
	}
	return c, credential, nil
}

// MarkUsed 记录一次成功的连接，并更新最后使用时间。
func (s *Store) MarkUsed(id int64) error {
	result, err := s.db.Exec("UPDATE connections SET use_count=use_count+1,last_used_at=unixepoch() WHERE id=?", id)
	if err != nil {
		return err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if updated == 0 {
		return errors.New(i18n.T("Connection does not exist"))
	}
	return nil
}

// UpdateConnection 更新连接字段，并返回更新后的连接摘要。
func (s *Store) UpdateConnection(id int64, input NewConnection) (Connection, error) {
	if id <= 0 {
		return Connection{}, errors.New(i18n.T("Connection does not exist"))
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Host = strings.TrimSpace(input.Host)
	input.Username = strings.TrimSpace(input.Username)
	input.Remark = strings.TrimSpace(input.Remark)
	if input.Name == "" || input.Host == "" || input.Username == "" {
		return Connection{}, errors.New(i18n.T("Name, host, and user are required"))
	}
	if input.Port < 1 || input.Port > 65535 {
		return Connection{}, errors.New(i18n.T("Port must be between 1 and 65535"))
	}
	if input.CredentialID <= 0 {
		return Connection{}, errors.New(i18n.T("You must select a credential"))
	}
	var c Connection
	if err := s.db.QueryRow("SELECT use_count,last_used_at FROM connections WHERE id=?", id).Scan(&c.UseCount, &c.LastUsedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Connection{}, errors.New(i18n.T("Connection does not exist"))
		}
		return Connection{}, fmt.Errorf("%s: %w", i18n.T("Failed to read connection usage"), err)
	}
	if err := s.db.QueryRow(`SELECT cr.type, cr.name FROM credentials cr WHERE cr.id=?`, input.CredentialID).Scan(&c.Credential, &c.CredentialName); err != nil {
		return Connection{}, fmt.Errorf("%s: %w", i18n.T("Failed to read credential"), err)
	}
	result, err := s.db.Exec(`UPDATE connections SET name=?,host=?,port=?,username=?,credential_id=?,remark=? WHERE id=?`, input.Name, input.Host, input.Port, input.Username, input.CredentialID, input.Remark, id)
	if err != nil {
		return Connection{}, err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return Connection{}, err
	}
	if updated == 0 {
		return Connection{}, errors.New(i18n.T("Connection does not exist"))
	}
	c.ID, c.Name, c.Host, c.Port, c.Username, c.CredentialID, c.Remark = id, input.Name, input.Host, input.Port, input.Username, input.CredentialID, input.Remark
	return c, nil
}

// DeleteConnection 删除指定连接。
func (s *Store) DeleteConnection(id int64) error {
	if id <= 0 {
		return errors.New(i18n.T("Connection does not exist"))
	}
	result, err := s.db.Exec("DELETE FROM connections WHERE id=?", id)
	if err != nil {
		return err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if deleted == 0 {
		return errors.New(i18n.T("Connection does not exist"))
	}
	return nil
}

// CreateConnection 创建引用已有凭据的连接。
func (s *Store) CreateConnection(input NewConnection) (Connection, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Host = strings.TrimSpace(input.Host)
	input.Username = strings.TrimSpace(input.Username)
	input.Remark = strings.TrimSpace(input.Remark)
	if input.Name == "" || input.Host == "" || input.Username == "" {
		return Connection{}, errors.New(i18n.T("Name, host, and user are required"))
	}
	if input.Port < 1 || input.Port > 65535 {
		return Connection{}, errors.New(i18n.T("Port must be between 1 and 65535"))
	}
	if input.CredentialID <= 0 {
		return Connection{}, errors.New(i18n.T("You must select a credential"))
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Connection{}, err
	}
	defer tx.Rollback()
	var credentialType, credentialName string
	if err := tx.QueryRow("SELECT type,name FROM credentials WHERE id=?", input.CredentialID).Scan(&credentialType, &credentialName); err != nil {
		return Connection{}, fmt.Errorf("%s: %w", i18n.T("Failed to read credential"), err)
	}
	result, err := tx.Exec("INSERT INTO connections(name,host,port,username,credential_id,remark) VALUES(?,?,?,?,?,?)", input.Name, input.Host, input.Port, input.Username, input.CredentialID, input.Remark)
	if err != nil {
		return Connection{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Connection{}, err
	}
	if err := tx.Commit(); err != nil {
		return Connection{}, err
	}
	return Connection{ID: id, Name: input.Name, Host: input.Host, Port: input.Port, Username: input.Username, Credential: credentialType, CredentialID: input.CredentialID, CredentialName: credentialName, Remark: input.Remark}, nil
}

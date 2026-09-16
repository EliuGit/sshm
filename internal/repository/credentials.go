package repository

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ListCredentials 返回可供连接选择的凭据摘要及其关联连接数量，绝不读取密文。
func (s *Store) ListCredentials() ([]Credential, error) {
	rows, err := s.db.Query(`SELECT cr.id,cr.name,cr.type,COUNT(c.id)
		FROM credentials cr LEFT JOIN connections c ON c.credential_id=cr.id
		GROUP BY cr.id ORDER BY cr.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Credential
	for rows.Next() {
		var credential Credential
		if err := rows.Scan(&credential.ID, &credential.Name, &credential.Type, &credential.ConnectionCount); err != nil {
			return nil, err
		}
		result = append(result, credential)
	}
	return result, rows.Err()
}

// UpdateCredential 更新凭据名称和类型；content 为空时保留原密文。
func (s *Store) UpdateCredential(id int64, name, credentialType string, content []byte) error {
	name = strings.TrimSpace(name)
	if id <= 0 {
		return errors.New("凭据不存在")
	}
	if name == "" {
		return errors.New("凭据名称不能为空")
	}
	if credentialType != "passwd" && credentialType != "key" {
		return errors.New("凭据类型必须是密码或私钥")
	}
	var result sql.Result
	var err error
	if len(bytes.TrimSpace(content)) == 0 {
		result, err = s.db.Exec("UPDATE credentials SET name=?,type=? WHERE id=?", name, credentialType, id)
	} else {
		result, err = s.db.Exec("UPDATE credentials SET name=?,type=?,content=? WHERE id=?", name, credentialType, content, id)
	}
	if err != nil {
		return err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if updated == 0 {
		return errors.New("凭据不存在")
	}
	return nil
}

// DeleteCredential 删除未被任何连接引用的凭据。
func (s *Store) DeleteCredential(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRow("SELECT COUNT(*) FROM connections WHERE credential_id=?", id).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("凭据仍关联 %d 个连接，不能删除", count)
	}
	result, err := tx.Exec("DELETE FROM credentials WHERE id=?", id)
	if err != nil {
		return err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if deleted == 0 {
		return errors.New("凭据不存在")
	}
	return tx.Commit()
}

// CreateCredential 将密码或私钥保存到加密数据库。
func (s *Store) CreateCredential(input NewCredential) (Credential, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len(bytes.TrimSpace(input.Content)) == 0 {
		return Credential{}, errors.New("凭据名称和内容不能为空")
	}
	if input.Type != "passwd" && input.Type != "key" {
		return Credential{}, errors.New("凭据类型必须是密码或私钥")
	}
	result, err := s.db.Exec("INSERT INTO credentials(name,type,content) VALUES(?,?,?)", input.Name, input.Type, input.Content)
	if err != nil {
		return Credential{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Credential{}, err
	}
	return Credential{ID: id, Name: input.Name, Type: input.Type}, nil
}

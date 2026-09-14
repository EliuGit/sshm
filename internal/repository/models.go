package repository

import (
	"database/sql"
	"errors"
)

// Connection 表示可在连接列表中展示的 SSH 连接信息。
type Connection struct {
	ID             int64
	Name           string
	Host           string
	Port           int
	Username       string
	Credential     string
	CredentialID   int64
	CredentialName string
	UseCount       int64
	LastUsedAt     sql.NullInt64
	Remark         string
}

// Credential 表示可供连接复用的凭据摘要，不包含加密内容。
type Credential struct {
	ID              int64
	Name            string
	Type            string
	ConnectionCount int
}

// NewCredential 包含待加密保存的新凭据。
type NewCredential struct {
	Name    string
	Type    string
	Content []byte
}

// NewConnection 包含连接信息及其引用的已有凭据。
type NewConnection struct {
	Name         string
	Host         string
	Port         int
	Username     string
	CredentialID int64
	Remark       string
}

// ErrInvalidPassword 表示主密码无法解封数据密钥。
var ErrInvalidPassword = errors.New("主密码不正确")

// Status 表示数据库是否已完成主密钥初始化。
type Status int

const (
	// NotFound 表示数据库文件不存在。
	NotFound Status = iota
	// Uninitialized 表示数据库存在，但尚未创建 SSHM schema。
	Uninitialized
	// Ready 表示数据库结构和主密钥记录均可用于解锁。
	Ready
)

// Store 持有数据库连接及当前进程解锁后的数据密钥。
type Store struct {
	db      *sql.DB
	dataKey []byte
}

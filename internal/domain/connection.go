package domain

import "time"

type AuthType string

const (
	AuthTypePassword   AuthType = "password"
	AuthTypePrivateKey AuthType = "private_key"
)

type Connection struct {
	ID             int64
	GroupID        *int64
	GroupName      string
	Name           string
	Host           string
	Port           int
	Username       string
	AuthType       AuthType
	PrivateKeyPath string
	Description    string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	LastUsedAt     *time.Time
}

type ConnectionSecret struct {
	ConnectionID       int64
	PasswordCiphertext string
}

type ConnectionInput struct {
	GroupID        *int64
	Name           string
	Host           string
	Port           int
	Username       string
	AuthType       AuthType
	PrivateKeyPath string
	Description    string
	Password       string
}

type ConnectionUpdateInput struct {
	GroupID        *int64
	Name           string
	Host           string
	Port           int
	Username       string
	AuthType       AuthType
	PrivateKeyPath string
	Description    string
	Password       string
	KeepPassword   bool
}

type ConnectionListOptions struct {
	Query   string
	Scope   ConnectionListScope
	GroupID int64
}

type ConnectionListScope int

const (
	ConnectionListScopeAll ConnectionListScope = iota
	ConnectionListScopeUngrouped
	ConnectionListScopeGroup
)

type ConnectionGroup struct {
	ID        int64
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ConnectionGroupListItem struct {
	ID              int64
	Name            string
	ConnectionCount int
	Ungrouped       bool
}

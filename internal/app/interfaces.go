package app

import "sshm/internal/domain"

// Repository 定义 app 层对存储的最小依赖集合，避免直接绑定具体实现。
type Repository interface {
	ConnectionRepository
	SecretRepository
	GroupRepository
}

type ConnectionRepository interface {
	ListConnections(opts domain.ConnectionListOptions) ([]domain.Connection, error)
	GetConnection(id int64) (domain.Connection, error)
	CreateConnection(conn domain.Connection, secret *domain.ConnectionSecret) (int64, error)
	UpdateConnection(id int64, conn domain.Connection, secret *domain.ConnectionSecret, deleteSecret bool) error
	DeleteConnection(id int64) error
	MarkUsed(id int64) error
	SetConnectionGroup(connectionID int64, groupID *int64) error
}

type SecretRepository interface {
	GetSecret(connectionID int64) (domain.ConnectionSecret, error)
}

type GroupRepository interface {
	ListGroups() ([]domain.ConnectionGroupListItem, error)
	CreateGroup(name string) (domain.ConnectionGroup, error)
	FindGroupByName(name string) (domain.ConnectionGroup, bool, error)
	RenameGroup(id int64, name string) error
	DeleteGroup(id int64) error
}

// CryptoProvider 定义 app 层对密文处理的最小依赖集合。
type CryptoProvider interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

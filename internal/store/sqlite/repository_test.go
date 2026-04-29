package sqlite

import (
	"database/sql"
	"path/filepath"
	"sshm/internal/domain"
	"testing"
)

func TestOpenMigratesConnectionGroupColumn(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	_, err = db.Exec(`
CREATE TABLE connections (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	host TEXT NOT NULL,
	port INTEGER NOT NULL,
	username TEXT NOT NULL,
	auth_type TEXT NOT NULL,
	private_key_path TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	last_used_at TEXT
);
INSERT INTO connections (name, host, port, username, auth_type, private_key_path, description, created_at, updated_at)
VALUES ('prod', 'example.com', 22, 'root', 'private_key', '~/.ssh/id_rsa', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
`)
	if err != nil {
		t.Fatalf("seed old schema error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	repo, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer repo.Close()

	items, err := repo.ListConnections(domain.ConnectionListOptions{})
	if err != nil {
		t.Fatalf("ListConnections() error = %v", err)
	}
	if len(items) != 1 || items[0].GroupID != nil {
		t.Fatalf("items = %#v", items)
	}
}

func TestListConnectionsFiltersInSQLite(t *testing.T) {
	t.Parallel()

	repo, err := Open(filepath.Join(t.TempDir(), "sshm.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer repo.Close()

	prodGroup, err := repo.CreateGroup("生产")
	if err != nil {
		t.Fatalf("CreateGroup(prod) error = %v", err)
	}
	devGroup, err := repo.CreateGroup("开发")
	if err != nil {
		t.Fatalf("CreateGroup(dev) error = %v", err)
	}

	prodAPIID := mustCreateConnection(t, repo, domain.Connection{
		GroupID:        &prodGroup.ID,
		Name:           "prod-api",
		Host:           "api.example.com",
		Port:           22,
		Username:       "root",
		AuthType:       domain.AuthTypePrivateKey,
		PrivateKeyPath: "~/.ssh/id_ed25519",
		Description:    "核心 API",
	})
	_ = prodAPIID
	mustCreateConnection(t, repo, domain.Connection{
		GroupID:        &prodGroup.ID,
		Name:           "prod-worker",
		Host:           "worker.example.com",
		Port:           22,
		Username:       "deploy",
		AuthType:       domain.AuthTypePrivateKey,
		PrivateKeyPath: "~/.ssh/id_ed25519",
		Description:    "异步任务",
	})
	mustCreateConnection(t, repo, domain.Connection{
		GroupID:        &devGroup.ID,
		Name:           "dev-web",
		Host:           "web.dev.local",
		Port:           22,
		Username:       "dev",
		AuthType:       domain.AuthTypePrivateKey,
		PrivateKeyPath: "~/.ssh/id_ed25519",
		Description:    "前端联调",
	})
	mustCreateConnection(t, repo, domain.Connection{
		Name:           "local-bastion",
		Host:           "127.0.0.1",
		Port:           2222,
		Username:       "tester",
		AuthType:       domain.AuthTypePrivateKey,
		PrivateKeyPath: "~/.ssh/id_ed25519",
		Description:    "未分组入口",
	})

	t.Run("group scope", func(t *testing.T) {
		items, err := repo.ListConnections(domain.ConnectionListOptions{
			Scope:   domain.ConnectionListScopeGroup,
			GroupID: prodGroup.ID,
		})
		if err != nil {
			t.Fatalf("ListConnections(group) error = %v", err)
		}
		if len(items) != 2 {
			t.Fatalf("len(items) = %d, want 2", len(items))
		}
		for _, item := range items {
			if item.GroupID == nil || *item.GroupID != prodGroup.ID {
				t.Fatalf("item = %#v, want prod group only", item)
			}
		}
	})

	t.Run("ungrouped scope", func(t *testing.T) {
		items, err := repo.ListConnections(domain.ConnectionListOptions{
			Scope: domain.ConnectionListScopeUngrouped,
		})
		if err != nil {
			t.Fatalf("ListConnections(ungrouped) error = %v", err)
		}
		if len(items) != 1 || items[0].Name != "local-bastion" || items[0].GroupID != nil {
			t.Fatalf("items = %#v, want only ungrouped bastion", items)
		}
	})

	t.Run("query matches joined group name", func(t *testing.T) {
		items, err := repo.ListConnections(domain.ConnectionListOptions{
			Query: "生产",
		})
		if err != nil {
			t.Fatalf("ListConnections(query group) error = %v", err)
		}
		if len(items) != 2 {
			t.Fatalf("len(items) = %d, want 2", len(items))
		}
	})

	t.Run("query and scope combined", func(t *testing.T) {
		items, err := repo.ListConnections(domain.ConnectionListOptions{
			Query:   "api",
			Scope:   domain.ConnectionListScopeGroup,
			GroupID: prodGroup.ID,
		})
		if err != nil {
			t.Fatalf("ListConnections(query+group) error = %v", err)
		}
		if len(items) != 1 || items[0].Name != "prod-api" {
			t.Fatalf("items = %#v, want only prod-api", items)
		}
	})
}

func mustCreateConnection(t *testing.T, repo *Repository, conn domain.Connection) int64 {
	t.Helper()

	id, err := repo.CreateConnection(conn, nil)
	if err != nil {
		t.Fatalf("CreateConnection(%s) error = %v", conn.Name, err)
	}
	return id
}

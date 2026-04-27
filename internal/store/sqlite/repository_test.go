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

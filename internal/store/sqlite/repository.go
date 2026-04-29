package sqlite

import (
	"database/sql"
	"fmt"
	"sshm/internal/domain"
	"time"

	_ "modernc.org/sqlite"
)

type Repository struct {
	db *sql.DB
}

func Open(path string) (*Repository, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		_ = db.Close()
		return nil, err
	}
	repo := &Repository{db: db}
	if err := repo.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return repo, nil
}

func (r *Repository) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

func (r *Repository) initSchema() error {
	schema := `
CREATE TABLE IF NOT EXISTS connections (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	group_id INTEGER,
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
CREATE TABLE IF NOT EXISTS connection_groups (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL COLLATE NOCASE UNIQUE,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS connection_secrets (
	connection_id INTEGER PRIMARY KEY,
	password_ciphertext TEXT NOT NULL,
	FOREIGN KEY(connection_id) REFERENCES connections(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_connections_name ON connections(name);
CREATE INDEX IF NOT EXISTS idx_connections_last_used_at ON connections(last_used_at);
`
	if _, err := r.db.Exec(schema); err != nil {
		return err
	}
	if err := r.ensureColumn("connections", "group_id", "INTEGER"); err != nil {
		return err
	}
	_, err := r.db.Exec(`CREATE INDEX IF NOT EXISTS idx_connections_group_id ON connections(group_id);`)
	return err
}

func (r *Repository) ListConnections(opts domain.ConnectionListOptions) ([]domain.Connection, error) {
	// 设计说明：
	// 1. 连接列表的 Scope / Query 过滤必须在数据库侧完成，避免“全量拉取后再由 Go 逐条过滤”的线性放大。
	// 2. 当前阶段先使用普通 SQL 下推条件，优先根治应用层全表扫描问题。
	// 3. 查询条件只负责列表筛选，不改变现有排序语义，确保 UI / CLI 展示结果与现有交互保持一致。
	baseSQL := `
SELECT c.id, c.group_id, COALESCE(g.name, ''), c.name, c.host, c.port, c.username, c.auth_type, c.private_key_path, c.description, c.created_at, c.updated_at, c.last_used_at
FROM connections AS c
LEFT JOIN connection_groups AS g ON g.id = c.group_id`
	whereClauses := make([]string, 0, 2)
	args := make([]any, 0, 6)
	switch opts.Scope {
	case domain.ConnectionListScopeUngrouped:
		whereClauses = append(whereClauses, "c.group_id IS NULL")
	case domain.ConnectionListScopeGroup:
		whereClauses = append(whereClauses, "c.group_id = ?")
		args = append(args, opts.GroupID)
	}
	query := buildConnectionQueryPattern(opts.Query)
	if query != "" {
		whereClauses = append(whereClauses, `(
	c.name LIKE ? COLLATE NOCASE OR
	c.host LIKE ? COLLATE NOCASE OR
	c.username LIKE ? COLLATE NOCASE OR
	c.description LIKE ? COLLATE NOCASE OR
	COALESCE(g.name, '') LIKE ? COLLATE NOCASE
)`)
		for range 5 {
			args = append(args, query)
		}
	}
	sqlText := baseSQL
	if len(whereClauses) > 0 {
		sqlText += "\nWHERE " + joinClauses(whereClauses, " AND ")
	}
	sqlText += "\nORDER BY CASE WHEN c.last_used_at IS NULL THEN 1 ELSE 0 END, c.last_used_at DESC, c.name COLLATE NOCASE ASC"
	rows, err := r.db.Query(sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.Connection
	for rows.Next() {
		conn, err := scanConnection(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, conn)
	}
	return items, rows.Err()
}

func (r *Repository) GetConnection(id int64) (domain.Connection, error) {
	row := r.db.QueryRow(`
SELECT c.id, c.group_id, COALESCE(g.name, ''), c.name, c.host, c.port, c.username, c.auth_type, c.private_key_path, c.description, c.created_at, c.updated_at, c.last_used_at
FROM connections AS c LEFT JOIN connection_groups AS g ON g.id = c.group_id
WHERE c.id = ?`, id)
	conn, err := scanConnection(row)
	if err == sql.ErrNoRows {
		return domain.Connection{}, domain.ErrConnectionNotFound
	}
	return conn, err
}

func (r *Repository) GetSecret(connectionID int64) (domain.ConnectionSecret, error) {
	row := r.db.QueryRow(`SELECT connection_id, password_ciphertext FROM connection_secrets WHERE connection_id = ?`, connectionID)
	var secret domain.ConnectionSecret
	if err := row.Scan(&secret.ConnectionID, &secret.PasswordCiphertext); err != nil {
		if err == sql.ErrNoRows {
			return domain.ConnectionSecret{}, domain.ErrConnectionSecretNotFound
		}
		return domain.ConnectionSecret{}, err
	}
	return secret, nil
}

func (r *Repository) CreateConnection(conn domain.Connection, secret *domain.ConnectionSecret) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`
INSERT INTO connections (group_id, name, host, port, username, auth_type, private_key_path, description, created_at, updated_at, last_used_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nullableInt64(conn.GroupID), conn.Name, conn.Host, conn.Port, conn.Username, string(conn.AuthType), conn.PrivateKeyPath, conn.Description, now, now, nil)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if secret != nil {
		if _, err := tx.Exec(`INSERT INTO connection_secrets (connection_id, password_ciphertext) VALUES (?, ?)`, id, secret.PasswordCiphertext); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *Repository) UpdateConnection(id int64, conn domain.Connection, secret *domain.ConnectionSecret, deleteSecret bool) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`
UPDATE connections
SET group_id = ?, name = ?, host = ?, port = ?, username = ?, auth_type = ?, private_key_path = ?, description = ?, updated_at = ?
WHERE id = ?`,
		nullableInt64(conn.GroupID), conn.Name, conn.Host, conn.Port, conn.Username, string(conn.AuthType), conn.PrivateKeyPath, conn.Description, now, id)
	if err != nil {
		return err
	}
	if deleteSecret {
		if _, err := tx.Exec(`DELETE FROM connection_secrets WHERE connection_id = ?`, id); err != nil {
			return err
		}
	}
	if secret != nil {
		if _, err := tx.Exec(`
INSERT INTO connection_secrets (connection_id, password_ciphertext) VALUES (?, ?)
ON CONFLICT(connection_id) DO UPDATE SET password_ciphertext = excluded.password_ciphertext`,
			id, secret.PasswordCiphertext); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) DeleteConnection(id int64) error {
	_, err := r.db.Exec(`DELETE FROM connections WHERE id = ?`, id)
	return err
}

func (r *Repository) MarkUsed(id int64) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.Exec(`UPDATE connections SET last_used_at = ?, updated_at = ? WHERE id = ?`, now, now, id)
	return err
}

func (r *Repository) ListGroups() ([]domain.ConnectionGroupListItem, error) {
	rows, err := r.db.Query(`
SELECT g.id, g.name, COUNT(c.id)
FROM connection_groups AS g
LEFT JOIN connections AS c ON c.group_id = g.id
GROUP BY g.id, g.name
ORDER BY g.name COLLATE NOCASE ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.ConnectionGroupListItem{}
	for rows.Next() {
		var item domain.ConnectionGroupListItem
		if err := rows.Scan(&item.ID, &item.Name, &item.ConnectionCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	ungroupedCount, err := r.countUngroupedConnections()
	if err != nil {
		return nil, err
	}
	return append([]domain.ConnectionGroupListItem{{
		Name:            "未分组",
		ConnectionCount: ungroupedCount,
		Ungrouped:       true,
	}}, items...), nil
}

func (r *Repository) CreateGroup(name string) (domain.ConnectionGroup, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := r.db.Exec(`INSERT INTO connection_groups (name, created_at, updated_at) VALUES (?, ?, ?)`, name, now, now)
	if err != nil {
		return domain.ConnectionGroup{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.ConnectionGroup{}, err
	}
	return r.GetGroup(id)
}

func (r *Repository) GetGroup(id int64) (domain.ConnectionGroup, error) {
	row := r.db.QueryRow(`SELECT id, name, created_at, updated_at FROM connection_groups WHERE id = ?`, id)
	return scanGroup(row)
}

func (r *Repository) FindGroupByName(name string) (domain.ConnectionGroup, bool, error) {
	row := r.db.QueryRow(`SELECT id, name, created_at, updated_at FROM connection_groups WHERE name = ? COLLATE NOCASE`, name)
	group, err := scanGroup(row)
	if err == sql.ErrNoRows {
		return domain.ConnectionGroup{}, false, nil
	}
	if err != nil {
		return domain.ConnectionGroup{}, false, err
	}
	return group, true, nil
}

func (r *Repository) RenameGroup(id int64, name string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.Exec(`UPDATE connection_groups SET name = ?, updated_at = ? WHERE id = ?`, name, now, id)
	return err
}

func (r *Repository) DeleteGroup(id int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.Exec(`UPDATE connections SET group_id = NULL, updated_at = ? WHERE group_id = ?`, now, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM connection_groups WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) SetConnectionGroup(connectionID int64, groupID *int64) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.Exec(`UPDATE connections SET group_id = ?, updated_at = ? WHERE id = ?`, nullableInt64(groupID), now, connectionID)
	return err
}

func (r *Repository) countUngroupedConnections() (int, error) {
	row := r.db.QueryRow(`SELECT COUNT(*) FROM connections WHERE group_id IS NULL`)
	var count int
	err := row.Scan(&count)
	return count, err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanConnection(s scanner) (domain.Connection, error) {
	var conn domain.Connection
	var groupID sql.NullInt64
	var authType string
	var createdAt string
	var updatedAt string
	var lastUsedAt sql.NullString
	if err := s.Scan(
		&conn.ID,
		&groupID,
		&conn.GroupName,
		&conn.Name,
		&conn.Host,
		&conn.Port,
		&conn.Username,
		&authType,
		&conn.PrivateKeyPath,
		&conn.Description,
		&createdAt,
		&updatedAt,
		&lastUsedAt,
	); err != nil {
		return domain.Connection{}, err
	}
	if groupID.Valid {
		conn.GroupID = &groupID.Int64
	}
	conn.AuthType = domain.AuthType(authType)
	conn.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	conn.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	if lastUsedAt.Valid && lastUsedAt.String != "" {
		t, err := time.Parse(time.RFC3339Nano, lastUsedAt.String)
		if err == nil {
			conn.LastUsedAt = &t
		}
	}
	return conn, nil
}

func scanGroup(s scanner) (domain.ConnectionGroup, error) {
	var group domain.ConnectionGroup
	var createdAt string
	var updatedAt string
	if err := s.Scan(&group.ID, &group.Name, &createdAt, &updatedAt); err != nil {
		return domain.ConnectionGroup{}, err
	}
	group.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	group.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return group, nil
}

func (r *Repository) ensureColumn(table string, name string, columnType string) error {
	rows, err := r.db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var columnName string
		var valueType string
		var notNull int
		var defaultValue any
		var primaryKey int
		if err := rows.Scan(&cid, &columnName, &valueType, &notNull, &defaultValue, &primaryKey); err != nil {
			return err
		}
		if columnName == name {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = r.db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, name, columnType))
	return err
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func buildConnectionQueryPattern(query string) string {
	query = trimSQLSearchQuery(query)
	if query == "" {
		return ""
	}
	return "%" + query + "%"
}

func trimSQLSearchQuery(query string) string {
	start := 0
	for start < len(query) && isSQLSpace(query[start]) {
		start++
	}
	end := len(query)
	for end > start && isSQLSpace(query[end-1]) {
		end--
	}
	return query[start:end]
}

func isSQLSpace(ch byte) bool {
	switch ch {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

func joinClauses(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, part := range parts[1:] {
		result += sep + part
	}
	return result
}

PRAGMA foreign_keys = ON;

-- credentials 的内容由加密数据库统一保护，不再单独加密。
CREATE TABLE credentials (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL COLLATE NOCASE UNIQUE CHECK (length(trim(name)) > 0),
    type TEXT NOT NULL CHECK (type IN ('passwd', 'key')),
    content BLOB NOT NULL CHECK (typeof(content) = 'blob' AND length(content) > 0),
    create_at INTEGER NOT NULL DEFAULT (unixepoch()) CHECK (create_at >= 0)
) STRICT;

-- 多条连接可以复用同一凭据；仍被引用的凭据不能删除。
CREATE TABLE connections (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL COLLATE NOCASE UNIQUE CHECK (length(trim(name)) > 0),
    host TEXT NOT NULL CHECK (length(trim(host)) > 0),
    port INTEGER NOT NULL DEFAULT 22 CHECK (port BETWEEN 1 AND 65535),
    username TEXT NOT NULL CHECK (length(trim(username)) > 0),
    credential_id INTEGER NOT NULL,
    use_count INTEGER NOT NULL DEFAULT 0 CHECK (use_count >= 0),
    last_used_at INTEGER CHECK (last_used_at IS NULL OR last_used_at >= 0),
    remark TEXT NOT NULL DEFAULT '',
    create_at INTEGER NOT NULL DEFAULT (unixepoch()) CHECK (create_at >= 0),
    FOREIGN KEY (credential_id) REFERENCES credentials(id) ON DELETE RESTRICT
) STRICT;

CREATE INDEX idx_connections_credential_id ON connections(credential_id);
CREATE INDEX idx_connections_last_used_at ON connections(last_used_at);
PRAGMA user_version = 1;

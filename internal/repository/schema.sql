PRAGMA foreign_keys = ON;

-- master_key 只保存由主密码加密后的数据密钥，Argon2id 参数固定在程序中。
CREATE TABLE master_key (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    kdf_salt BLOB NOT NULL CHECK (typeof(kdf_salt) = 'blob' AND length(kdf_salt) = 16),
    nonce BLOB NOT NULL CHECK (typeof(nonce) = 'blob' AND length(nonce) = 24),
    encrypted_data_key BLOB NOT NULL CHECK (typeof(encrypted_data_key) = 'blob' AND length(encrypted_data_key) = 48),
    create_at INTEGER NOT NULL DEFAULT (unixepoch()) CHECK (create_at >= 0)
) STRICT;

-- credentials 中的密文由内存中的数据密钥使用 XChaCha20-Poly1305 加密。
CREATE TABLE credentials (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL COLLATE NOCASE UNIQUE CHECK (length(trim(name)) > 0),
    type TEXT NOT NULL CHECK (type IN ('passwd', 'key')),
    nonce BLOB NOT NULL CHECK (typeof(nonce) = 'blob' AND length(nonce) = 24),
    ciphertext BLOB NOT NULL CHECK (typeof(ciphertext) = 'blob' AND length(ciphertext) >= 16),
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

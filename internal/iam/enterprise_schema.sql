-- 企业登录配置和稳定身份绑定；秘密由服务端加密后存储。
CREATE TABLE IF NOT EXISTS enterprise_oidc (
 connection_id TEXT PRIMARY KEY REFERENCES directory_connections(id) ON DELETE RESTRICT, name TEXT NOT NULL CHECK(name <> ''),
 issuer TEXT NOT NULL, client_id TEXT NOT NULL, secret TEXT NOT NULL DEFAULT '',
 redirect_uri TEXT NOT NULL, return_uri TEXT NOT NULL, identity_claim TEXT NOT NULL DEFAULT 'sub',
 enabled BOOLEAN NOT NULL DEFAULT false, revision BIGINT NOT NULL DEFAULT 1
);
CREATE TABLE IF NOT EXISTS enterprise_login_transactions (
 state_hash TEXT PRIMARY KEY, connection_id TEXT NOT NULL REFERENCES enterprise_oidc(connection_id),
 revision BIGINT NOT NULL, browser_hash TEXT NOT NULL, nonce TEXT NOT NULL, verifier TEXT NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE IF NOT EXISTS enterprise_login_identities (
 connection_id TEXT NOT NULL REFERENCES enterprise_oidc(connection_id), issuer TEXT NOT NULL, subject TEXT NOT NULL,
 user_id TEXT NOT NULL REFERENCES users(id), PRIMARY KEY(connection_id,issuer,subject),
 UNIQUE(connection_id,user_id)
);

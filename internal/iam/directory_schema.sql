-- 企业目录关系与有效团队归属分离，保持现有单团队与账务约束。
CREATE TABLE IF NOT EXISTS directory_connections (
 id TEXT PRIMARY KEY, organization_id TEXT NOT NULL UNIQUE REFERENCES organizations(id) ON DELETE RESTRICT,
 name TEXT NOT NULL CHECK(name <> ''), enabled BOOLEAN NOT NULL DEFAULT false,
 token_hash TEXT NOT NULL UNIQUE, default_team_id TEXT REFERENCES teams(id) ON DELETE RESTRICT,
 revision BIGINT NOT NULL DEFAULT 1, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 scim_configured BOOLEAN NOT NULL DEFAULT false
);
CREATE TABLE IF NOT EXISTS directory_users (
 id TEXT PRIMARY KEY, connection_id TEXT NOT NULL REFERENCES directory_connections(id) ON DELETE RESTRICT,
 external_id TEXT NOT NULL CHECK(external_id <> ''), user_id TEXT NOT NULL UNIQUE REFERENCES users(id) ON DELETE RESTRICT,
 active BOOLEAN NOT NULL DEFAULT true, mode TEXT NOT NULL DEFAULT 'auto' CHECK(mode IN ('auto','manual')),
 manual_team_id TEXT REFERENCES teams(id) ON DELETE RESTRICT,
 selected_mapping_id TEXT NOT NULL DEFAULT '', assignment_reason TEXT NOT NULL DEFAULT '',
 UNIQUE(connection_id,external_id), UNIQUE(id,connection_id)
);
CREATE TABLE IF NOT EXISTS directory_groups (
 id TEXT PRIMARY KEY, connection_id TEXT NOT NULL REFERENCES directory_connections(id) ON DELETE RESTRICT,
 external_id TEXT NOT NULL CHECK(external_id <> ''), name TEXT NOT NULL CHECK(name <> ''),
 UNIQUE(connection_id,external_id), UNIQUE(id,connection_id)
);
CREATE TABLE IF NOT EXISTS directory_group_members (
 connection_id TEXT NOT NULL, group_id TEXT NOT NULL, directory_user_id TEXT NOT NULL,
 PRIMARY KEY(group_id,directory_user_id),
 FOREIGN KEY(group_id,connection_id) REFERENCES directory_groups(id,connection_id) ON DELETE CASCADE,
 FOREIGN KEY(directory_user_id,connection_id) REFERENCES directory_users(id,connection_id) ON DELETE RESTRICT
);
CREATE TABLE IF NOT EXISTS directory_team_mappings (
 id TEXT PRIMARY KEY, connection_id TEXT NOT NULL REFERENCES directory_connections(id) ON DELETE RESTRICT,
 group_id TEXT NOT NULL UNIQUE, team_id TEXT NOT NULL REFERENCES teams(id) ON DELETE RESTRICT,
 priority INT NOT NULL CHECK(priority >= 0), enabled BOOLEAN NOT NULL DEFAULT true,
 FOREIGN KEY(group_id,connection_id) REFERENCES directory_groups(id,connection_id) ON DELETE CASCADE
);

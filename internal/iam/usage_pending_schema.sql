-- 直写账单先保存不可变快照，独立于用量/汇总事务；故障恢复和新实例均可安全重放。
CREATE TABLE IF NOT EXISTS usage_pending (
    request_id TEXT PRIMARY KEY,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS usage_pending_created_idx ON usage_pending (created_at, request_id);

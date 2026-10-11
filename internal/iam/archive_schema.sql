-- 配置版本不可变，旧积压写入原目标，凭据只存密文。
CREATE TABLE IF NOT EXISTS archive_configs (
 revision BIGSERIAL PRIMARY KEY, bucket TEXT NOT NULL, region TEXT NOT NULL,
 prefix TEXT NOT NULL, credentials TEXT NOT NULL, enabled BOOLEAN NOT NULL DEFAULT false,
 interval_seconds INTEGER NOT NULL DEFAULT 2 CHECK (interval_seconds BETWEEN 2 AND 86400),
 batch_size INTEGER NOT NULL DEFAULT 20 CHECK (batch_size BETWEEN 1 AND 200)
);
CREATE TABLE IF NOT EXISTS archive_events (
 id BIGSERIAL PRIMARY KEY, request_id TEXT NOT NULL, revision BIGINT NOT NULL REFERENCES archive_configs(revision),
 payload TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 state TEXT NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','processing','failed','done')),
 lease TEXT NOT NULL DEFAULT '', lease_until TIMESTAMPTZ, attempts INT NOT NULL DEFAULT 0,
 object_key TEXT NOT NULL DEFAULT '', sha256 TEXT NOT NULL DEFAULT '', byte_size BIGINT NOT NULL DEFAULT 0,
 error TEXT NOT NULL DEFAULT '', completed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS archive_events_queue ON archive_events(state,lease_until,id);
-- 白名单账务快照与账单原子提交，无正文、密钥或原始响应；异步任务更新产生独立版本。
CREATE OR REPLACE FUNCTION xhub_archive_usage() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE cfg archive_configs; snapshot JSONB;
BEGIN
 SELECT * INTO cfg FROM archive_configs ORDER BY revision DESC LIMIT 1;
 IF cfg.revision IS NULL OR NOT cfg.enabled THEN RETURN NEW; END IF;
 -- 仅白名单账务字段变化才生成新版本，迁移别名或正文更新不重复归档。
 IF TG_OP = 'UPDATE' AND
 ROW(NEW.request_id,NEW.ts,NEW.organization_id,NEW.team_id,NEW.project_id,NEW.user_id,
 NEW.model,NEW.provider,NEW.call_type,NEW.status,NEW.http_status,NEW.prompt_tokens,
 NEW.completion_tokens,NEW.cost,NEW.duration_ms,NEW.task_settled)
 IS NOT DISTINCT FROM
 ROW(OLD.request_id,OLD.ts,OLD.organization_id,OLD.team_id,OLD.project_id,OLD.user_id,
 OLD.model,OLD.provider,OLD.call_type,OLD.status,OLD.http_status,OLD.prompt_tokens,
 OLD.completion_tokens,OLD.cost,OLD.duration_ms,OLD.task_settled) THEN RETURN NEW; END IF;
 snapshot := jsonb_build_object('schema_version',1,'request_id',NEW.request_id,'ts',NEW.ts,
 'organization_id',NEW.organization_id,'team_id',NEW.team_id,'project_id',NEW.project_id,
 'user_id',NEW.user_id,'model',NEW.model,'provider',NEW.provider,'call_type',NEW.call_type,
 'status',NEW.status,'http_status',NEW.http_status,'prompt_tokens',NEW.prompt_tokens,
 'completion_tokens',NEW.completion_tokens,'cost',NEW.cost,'duration_ms',NEW.duration_ms,
 'task_settled',NEW.task_settled);
 INSERT INTO archive_events(request_id,revision,payload) VALUES(NEW.request_id,cfg.revision,snapshot::text);
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS usage_archive_outbox ON usage_events;
CREATE TRIGGER usage_archive_outbox AFTER INSERT OR UPDATE ON usage_events FOR EACH ROW EXECUTE FUNCTION xhub_archive_usage();

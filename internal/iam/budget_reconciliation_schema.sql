-- 人工对账是追加事实：一个尝试只允许一份结论，载荷保留原预占和原账单，不覆盖上游证据。
CREATE TABLE IF NOT EXISTS budget_reconciliations (
 reservation_id TEXT PRIMARY KEY,
 call_id TEXT NOT NULL,
 payload JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS budget_reconciliations_call_idx ON budget_reconciliations(call_id, created_at, reservation_id);

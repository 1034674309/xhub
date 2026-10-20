#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIR="${E2E_RUN_DIR:-$ROOT/.e2e/run-$$-$RANDOM}"
mkdir -p "$DIR"
UP_PORT="${E2E_UP_PORT:-4110}"
GW_PORT="${E2E_GW_PORT:-4100}"
MASTER="${E2E_MASTER_KEY:-sk-e2e-master}"
# portable模式同时让真实UI配置报告相对代理，避免hydrate前后在/gw和直连地址间变化。
E2E_REPORTED_ORIGIN="http://127.0.0.1:$GW_PORT"
if [[ "${E2E_PORTABLE_CONSOLE:-0}" == 1 ]]; then E2E_REPORTED_ORIGIN="/gw"; fi
DATABASE="${E2E_DATABASE:-xhub}"
# 数据库名称只允许 SQL 标识符；专属库由调用方创建，脚本只回收自己的 schema。
[[ "$DATABASE" =~ ^[a-z][a-z0-9_]*$ ]] || { echo "invalid E2E_DATABASE" >&2; exit 1; }
SCHEMA="e2e_$$_$RANDOM"
UP_PID=""
GW_PID=""
SCHEMA_CREATED=0
sql() { docker exec xhub-postgres psql -X -U xhub -d "$DATABASE" -v ON_ERROR_STOP=1 -q -c "$1"; }
cleanup() {
  trap - EXIT INT TERM
  [[ -z "$GW_PID" ]] || { kill "$GW_PID" 2>/dev/null || true; wait "$GW_PID" 2>/dev/null || true; }
  [[ -z "$UP_PID" ]] || { kill "$UP_PID" 2>/dev/null || true; wait "$UP_PID" 2>/dev/null || true; }
  rm -f "$DIR/c.yaml"
  # 只删除本进程创建的 schema；清理失败必须影响退出码并留下可重试证据。
  if [[ "$SCHEMA_CREATED" == 1 ]]; then
    if ! sql "DROP SCHEMA IF EXISTS $SCHEMA CASCADE" >"$DIR/cleanup.log" 2>&1; then
      cat "$DIR/cleanup.log" >&2
      exit 1
    fi
  fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
sql "CREATE SCHEMA $SCHEMA"
SCHEMA_CREATED=1
printf '%s\n' "$SCHEMA" > "$DIR/schema.txt"
E2E_UP_PORT="$UP_PORT" E2E_MASTER_KEY="$MASTER" E2E_SCHEMA="$SCHEMA" E2E_RUN_DIR="$DIR" python3 "$ROOT/e2e/config.py"
if [[ "${E2E_PREBUILT:-0}" != 1 ]]; then
  (cd "$ROOT" && go build -o "$DIR/xhub" ./cmd/gateway && go build -o "$DIR/livesweep" ./e2e/livesweep)
fi
# 专属运行目录可能尚无产物；PREBUILT 只复用存在的文件，不能让目录扫描测试缺少执行程序。
if [[ ! -x "$DIR/livesweep" ]]; then
  (cd "$ROOT" && go build -o "$DIR/livesweep" ./e2e/livesweep)
fi
if [[ "${E2E_ENTERPRISE_FIXTURE:-0}" != 1 && ! -x "$DIR/xhub" ]]; then
  (cd "$ROOT" && go build -o "$DIR/xhub" ./cmd/gateway)
fi
if [[ "${E2E_ENTERPRISE_FIXTURE:-0}" == 1 ]]; then
  (cd "$ROOT" && go test -c -o "$DIR/enterprise.test" ./internal/gateway)
fi
python3 "$ROOT/e2e/fake_upstream.py" --port "$UP_PORT" >"$DIR/up.log" 2>&1 &
UP_PID=$!
if [[ "${E2E_ENTERPRISE_FIXTURE:-0}" == 1 ]]; then
  XHUB_PRICE_FEED_URL="http://127.0.0.1:$UP_PORT/v1/market/models" XHUB_PUBLIC_ORIGIN="$E2E_REPORTED_ORIGIN" "$DIR/enterprise.test" -test.run '^TestBrowserEnterpriseFixture$' -test.timeout 0 >"$DIR/gw.log" 2>&1 &
else
  XHUB_PRICE_FEED_URL="http://127.0.0.1:$UP_PORT/v1/market/models" XHUB_PUBLIC_ORIGIN="$E2E_REPORTED_ORIGIN" "$DIR/xhub" -config "$DIR/c.yaml" -addr "127.0.0.1:$GW_PORT" >"$DIR/gw.log" 2>&1 &
fi
GW_PID=$!
wait "$GW_PID"

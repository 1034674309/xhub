#!/usr/bin/env bash
# 验证注册表与对话编解码的单元测试及各自 95% 语句覆盖率下限。
# 参数：可选报告目录；默认 .e2e/core-codec-coverage。调用：本地或 CI。
# 返回：测试失败、报告缺失或任一包低于下限时非零；仅写报告，不访问外部服务。
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
REPORT_DIR="${1:-.e2e/core-codec-coverage}"
mkdir -p "$REPORT_DIR"
go test ./internal/provider/registry ./internal/llm/dialogue -count=1 \
  -coverprofile="$REPORT_DIR/target.cover" -json > "$REPORT_DIR/target.jsonl" 2>&1
python3 - "$REPORT_DIR/target.cover" <<'PY'
import sys
from pathlib import Path

prefix = "github.com/sunqirui1987/xhub/"
packages = {prefix + name: [0, 0] for name in ("internal/provider/registry", "internal/llm/dialogue")}
for line in Path(sys.argv[1]).read_text().splitlines()[1:]:
    location, statements, hits = line.rsplit(" ", 2)
    package = location.rsplit(":", 1)[0].rsplit("/", 1)[0]
    if package in packages:
        count = int(statements)
        packages[package][0] += count
        packages[package][1] += count if int(hits) > 0 else 0
failed = False
for package, (total, covered) in packages.items():
    passed = total > 0 and covered * 100 >= total * 95
    percent = covered / total * 100 if total else 0
    print(f'{package.removeprefix(prefix)}: {percent:.2f}% ({covered}/{total}) {"PASS" if passed else "FAIL"}; minimum 95%')
    failed |= not passed
sys.exit(1 if failed else 0)
PY

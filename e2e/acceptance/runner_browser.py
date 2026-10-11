"""验收编排模块 runner_browser；仅运行入口选择的工作，失败交给上层清理。"""
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import time

from .runner_runtime import ROOT, command_stream, ready


def run_browser(directory, env, gateway, ui_port, processes, handles, logger):
    """用途：启动真实前端并执行 Playwright 验收；参数为运行状态，返回浏览器报告；进程和日志句柄由调用方统一清理。"""
    frontend = ROOT / "frontend"
    tsconfig = json.loads((frontend / "tsconfig.json").read_text())
    tsconfig["include"] = [i for i in tsconfig["include"] if not i.startswith(".next")] + [".next-real-dataset/types/**/*.ts"]
    (frontend / "tsconfig.e2e.json").write_text(json.dumps(tsconfig, indent=2) + "\n")
    ui_env = {**env, "E2E_BUILD_DIR": ".next-real-dataset", "XHUB_GATEWAY_ORIGIN": gateway,
              "NEXT_PUBLIC_BASE_URL": gateway, "E2E_DATASET_GATEWAY": gateway,
              "E2E_DATASET_UI": "http://127.0.0.1:" + str(ui_port), "E2E_DATASET_DIR": str(directory)}
    log = open(directory / "console.log", "a")
    handles.append(log)
    ui = subprocess.Popen([str(frontend / "node_modules/.bin/next"), "dev", "-p", str(ui_port), "-H", "127.0.0.1"],
                          cwd=frontend, env=ui_env, stdout=log, stderr=log)
    processes.append(ui)
    logger.log(f"    [浏览器] 等待前端启动，地址=http://127.0.0.1:{ui_port}")
    ready(ui_env["E2E_DATASET_UI"] + "/ui/login/", ui, timeout=120)
    logger.log("    [浏览器] 前端就绪，启动 Chromium 真实页面验收")
    command_stream(["node", str(ROOT / "e2e/real_dataset_browser.cjs")], logger, env=ui_env)
    return json.loads((directory / "browser-report.json").read_text())

def run_business_browser(directory, logger):
    """用途：补齐去重浏览器业务；参数为报告目录和日志器，返回本轮统计；精确标题与浏览器用例同步，等待共享锁并保存专属证据，隔离库和模拟供应商由 e2e.sh 清理，失败、缺报告或选中数量不符抛异常。"""
    titles = [
        "virtual key update, regenerate, block, and delete",
        "model update, test connection, and delete",
        "team member add is listed", "router fallback update lists the mapping",
        "admin panel saves prompt storage and hides the unused settings",
        "users can be edited and their team access follows membership changes",
        "关键词正则完成草稿调试、保存、编辑、真实拦截与删除",
        "XGo editor executes, persists, reloads, and edits real scripts",
        "chat explains missing credentials and unavailable deployment, then recovers",
        "single workspace has no comparison controls and preserves bilingual chat",
        "cancel and clear ignore delayed real responses; mobile chat remains usable",
        "错误日志详情完整显示上游正文并可返回普通日志",
        "异步任务原日志展示生命周期且轮询不增加日志：完成",
        "deployment deletion clears stale weights and preserves public inference",
        "deployment deletion clears default and template weights for inherited routing",
        "browser cache flush restores a billed miss after a free hit",
        "team session inheritance and explicitly bound key preview match inference without consuming rpm",
        "every page route renders without a dashboard error",
        "the visibility chain holds at every tier",
    ]
    # 文件与精确标题同时约束，避免相近名称或参数化变体重复执行同一业务类型。
    files = ["writes.spec.ts", "user-edit.spec.ts", "guardrails.spec.ts", "xgo-guardrails.spec.ts",
             "playground-workspace.spec.ts", "error-logs.spec.ts", "task-request-logs.spec.ts",
             "route-diagnostic.spec.ts", "weighted-routing.spec.ts", "product-fixes.spec.ts",
             "coverage.spec.ts", "visibility-chain.spec.ts"]
    pattern = "(" + "|".join(re.escape(title) + "$" for title in titles) + ")"
    env = dict(os.environ)
    for key in ("E2E_LIVE", "E2E_CREDENTIAL_SOURCE"):
        env.pop(key, None)
    env["E2E_FULL_COVERAGE"] = "0"
    env["E2E_BROWSER_REPORT_DIR"] = str((directory / "business-browser").resolve())
    result_path = Path(env["E2E_BROWSER_REPORT_DIR"]) / "results.json"
    started = time.time()
    try:
        command_stream(["bash", "scripts/e2e.sh", *files, "--grep", pattern], logger, env=env)
    finally:
        # 即使失败也保存已执行及未执行的浏览器证据；e2e.sh 启动时删除旧报告。
        if result_path.exists() and result_path.stat().st_mtime >= started:
            shutil.copyfile(result_path, directory / "business-browser-results.json")
    if not result_path.exists() or result_path.stat().st_mtime < started:
        raise AssertionError("缺少本轮浏览器报告")
    stats = json.loads(result_path.read_text())["stats"]
    if stats["expected"] != len(titles) or any(stats[field] for field in ("unexpected", "flaky", "skipped")):
        raise AssertionError("去重浏览器流程未全部执行通过: " + str(stats))
    return dict(status="passed", cases=len(titles), stats=stats, report="business-browser-results.json")


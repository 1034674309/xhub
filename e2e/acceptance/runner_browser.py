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

def business_browser_selection(catalog):
    """用途：将去重业务映射到当前独立场景；参数为场景清单，返回按验收顺序排列的条目。

    浏览器编排与契约测试调用；缺失、重复、非法路径或非 .test.ts 绑定抛异常，无副作用。
    稳定 ID 保留迁移前完整范围：拆分后的密钥/模型生命周期、所有页面与四层可见性均独立执行。
    """
    ids = ["SC-252", "SC-252-rotation", "SC-252-block", "SC-252-unblock", "SC-252-delete",
           "SC-253", "SC-253-connection", "SC-253-delete", "SC-254", "SC-255", "SC-256",
           "SC-245", "SC-027", "SC-257", "SC-108", "SC-109", "SC-110", "SC-020",
           "SC-240", "SC-122", "SC-247", "SC-113", "SC-114",
           *[f"SC-{number:03d}" for number in range(162, 208)],
           "SC-226", "SC-221", "SC-231", "SC-214"]
    selected = []
    files = set()
    for scenario_id in ids:
        matches = [row for row in catalog if row.get("id") == scenario_id]
        if len(matches) != 1:
            raise ValueError("去重浏览器场景缺失或 ID 重复: " + scenario_id)
        row = matches[0]
        file = row.get("file", "")
        if (not file.startswith("e2e/scenarios/") or not file.endswith(".test.ts")
                or ".." in Path(file).parts or file in files or not row.get("title")):
            raise ValueError("去重浏览器场景绑定无效: " + scenario_id)
        files.add(file)
        selected.append(row)
    return selected


def run_business_browser(directory, logger):
    """用途：补齐去重浏览器业务；参数为报告目录和日志器，返回本轮统计。

    共享验收调用；从场景清单解析独立文件与标题，等待共享锁并保存专属证据；
    隔离库和本地供应商由 e2e.sh 清理，失败、缺报告或选中数量不符抛异常。
    """
    frontend = ROOT / "frontend"
    selected = business_browser_selection(json.loads((frontend / "e2e/scenario-catalog.json").read_text()))
    for row in selected:
        if not (frontend / row["file"]).is_file():
            raise ValueError("去重浏览器场景文件不存在: " + row["file"])
    titles = [row["title"] for row in selected]
    # 文件与精确标题同时约束；路径需按正则转义并锚定，防止 sc-252 误选其生命周期变体。
    files = [r"(^|/)" + re.escape(row["file"]) + "$" for row in selected]
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

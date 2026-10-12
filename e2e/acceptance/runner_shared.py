"""验收编排模块 runner_shared；仅运行入口选择的工作，失败交给上层清理。"""
import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import sys
import time

from . import dataset as module
from .support import write_private
from .runner_runtime import ROOT, SHARED_DATABASE_URL, SHARED_REDIS_URL, RunLogger, command, command_stream, free_port, ready, shared_run_lock, validate_providers, validate_shared_target, reset_shared_target, shared_config, E2E_CATEGORIES
from .runner_database import recover_shared_chat_checkpoints, recover_media_log_ids, verify_database
from .runner_browser import run_browser, run_business_browser
from . import observer as observer_module


def restore_supplier_addresses(dataset, data):
    """用途：停止观察器前恢复仍存在部署的真实 HTTPS 地址；参数为数据集和清单，返回无；逐项按供应商 ID 恢复并打印结果，清除旧错误，仅把本次真实失败留在报告。"""
    if not dataset or not dataset.admin:
        return
    dataset.report.pop("retention_error", None)
    failures = []
    for deployment in dataset.state.get("deployments", []):
        if deployment.get("kind") != "chat" or not deployment.get("observed"):
            continue
        provider = next((p for p in data["providers"] if p["id"] == deployment.get("provider")), None)
        if not provider:
            failures.append(deployment.get("id", "unknown") + " 引用了未知供应商")
            continue
        try:
            dataset.api("/model/update", {"model_info": {"id": deployment["id"]},
                "litellm_params": {"api_base": os.environ.get(provider["base_env"], provider["base"])}})
            dataset.action_ok(deployment.get("public_name", deployment["id"]), "bypass_openai_chat",
                              "恢复供应商地址", f"部署 ID={deployment['id']}，供应商={provider['id']}", record=False)
        except Exception as error:
            # 生命周期和故障注入部署可能已在 finally 中删除；只有这类明确的临时部署可忽略 404。
            if deployment.get("temporary") and "状态 404" in str(error):
                dataset.action_ok(deployment.get("public_name", deployment["id"]), "bypass_openai_chat",
                                  "恢复供应商地址", f"临时部署 {deployment['id']} 已删除，无需恢复", record=False)
                continue
            failures.append(deployment.get("id", "unknown") + " 恢复失败: " + type(error).__name__)
            dataset.action_fail(deployment.get("public_name", deployment.get("id", "unknown")),
                                "bypass_openai_chat", "恢复供应商地址", error)
    if failures:
        dataset.report["retention_error"] = "；".join(failures)

def attach_chat_observer(dataset, observer):
    """用途：把聊天部署接入观察器；参数为数据集和观察器，返回无；只切换上游地址，保留创建时声明的端点，媒体仍直连真实供应商。"""
    for deployment in dataset.state.get("deployments", []):
        if not deployment.get("observed"):
            continue
        dataset.api("/model/update", {"model_info": {"id": deployment["id"]},
            "litellm_params": {"api_base": observer.base + "/" + deployment["provider"]}})

def shared_main(args, data):
    """用途：分阶段构建并验收 public 真实数据；参数为 CLI 和清单，返回退出码；seed 清空目标，verify 保留基线并执行浏览器/计量/回归。"""
    try:
        with shared_run_lock():
            return shared_main_locked(args, data)
    except RuntimeError as error:
        print(str(error), file=sys.stderr)
        return 1

def shared_main_locked(args, data):
    """用途：持锁执行共享数据构建或验收；参数为 CLI 和清单，返回退出码；仅由 shared_main 调用并覆盖完整服务生命周期。"""
    try:
        validate_providers(data)
        validate_shared_target(SHARED_DATABASE_URL, SHARED_REDIS_URL)
    except ValueError as error:
        raise SystemExit(str(error))
    directory = Path(args.directory).resolve()
    if args.phase == "seed":
        if directory.exists():
            shutil.rmtree(directory)
        directory.mkdir(parents=True)
        os.chmod(directory, 0o700)
        logger = RunLogger(directory / "testdata.log")
        logger.log("目标：PostgreSQL xhub/public；Redis DB 1；阶段=testdata")
        reset_shared_target(logger=logger.log)
        password = secrets.token_urlsafe(24)
        config = shared_config(password)
        write_private(directory / "c.yaml", config)
        (directory / "schema.txt").write_text("public\n")
    else:
        required = [directory / name for name in ("c.yaml", "access.json", "report.json", "schema.txt")]
        if any(not path.exists() for path in required):
            raise SystemExit("缺少 make testdata 基线，请先运行 make testdata")
        config = json.loads((directory / "c.yaml").read_text())
        validate_shared_target(config["general_settings"].get("database_url"), config["general_settings"].get("redis_url"))
        if (directory / "schema.txt").read_text().strip() != "public":
            raise SystemExit("共享验收状态不是 public schema")
        password = config["general_settings"]["admin_password"]
        logger = RunLogger(directory / "e2e.log")
        logger.log("目标：PostgreSQL xhub/public；Redis DB 1；阶段=e2e")
    gateway_port, ui_port = free_port(), free_port()
    gateway = "http://127.0.0.1:" + str(gateway_port)
    env = {**os.environ, "E2E_DATASET_PASSWORD": password, "E2E_DATASET_ADMIN": "admin", "XHUB_PUBLIC_ORIGIN": gateway}
    processes, handles, observer, dataset = [], [], None, None
    try:
        logger.log("[testdata 3/5] 构建并启动真实 xhub 网关" if args.phase == "seed" else "[e2e 1/4] 构建并启动真实 xhub 网关")
        build_started = time.monotonic()
        command(["go", "build", "-o", str(directory / "xhub"), "./cmd/gateway"])
        logger.log(f"    网关编译完成，耗时={time.monotonic() - build_started:.3f}s")
        log = open(directory / "gateway.log", "a")
        handles.append(log)
        gw = subprocess.Popen([str(directory / "xhub"), "-config", str(directory / "c.yaml"), "-addr", f"127.0.0.1:{gateway_port}"], cwd=ROOT, env=env, stdout=log, stderr=log)
        processes.append(gw)
        ready(gateway + "/health/liveliness", gw)
        logger.log(f"    网关健康检查通过，地址={gateway}")
        if args.phase == "verify":
            observer = observer_module.Observer(data["providers"], data["limits"]["max_upstream_attempts"],
                                       logger=lambda message: logger.log("    [真实上游] " + message))
        os.environ["E2E_DATASET_PASSWORD"] = password
        os.environ["E2E_DATASET_ADMIN"] = "admin"
        dataset = module.Dataset(gateway, directory, data, observer, logger=logger.log)
        if args.phase == "seed":
            logger.log("[testdata 4/5] 仅通过管理接口构建 real-acceptance 数据，不调用模型")
            dataset.seed()
            dataset.report["counts"] = {name: len(dataset.state[name]) for name in ("organizations", "teams", "users", "projects", "keys")}
            dataset.report["status"] = "seeded"
            dataset.report["target"] = {"database_url": SHARED_DATABASE_URL, "redis_url": SHARED_REDIS_URL, "schema": "public"}
            dataset.save()
            logger.log("[testdata 5/5] 构建完成：3组织 / 9团队 / 27成员 / 9项目 / 81成员密钥 + 1管理员个人密钥（共82把）/ 模型、路由和护栏配置；模型调用=0。Codex 三轮会话、图片及视频由 make e2e 执行")
        else:
            saved = json.loads((directory / "access.json").read_text())
            dataset.state = {key: value for key, value in saved.items() if key not in ("admin", "gateway")}
            dataset.report = json.loads((directory / "report.json").read_text())
            categories = getattr(args, "category", None)
            dataset.report.update(status="running", phase="verify",
                                  selected_categories=categories or list(E2E_CATEGORIES),
                                  verification_scope="selected" if categories else "full")
            dataset.report.pop("error", None)
            dataset.representative = True
            dataset.start_checklist(args.with_regression, categories)
            login, _ = dataset.api("/v2/login", {"username": "admin", "password": password})
            dataset.admin = login["key"]
            if categories is None or "chat-bills" in categories or "database" in categories:
                recover_shared_chat_checkpoints(dataset)
            if categories is None or any(category in categories for category in ("media-logs", "database", "browser")):
                recover_media_log_ids(dataset)
            attach_chat_observer(dataset, observer)
            logger.log("[e2e 2/4] 保留81密钥基线，按4种路由各选1把验收；逐项显示内容和结果")
            dataset.verify()
            # API 复验完成后先用数据库精确恢复媒体终态日志，再让浏览器按 ID 打开同一证据。
            if dataset.case_enabled("media-logs"):
                recover_media_log_ids(dataset)
            dataset.run_case("media-logs", dataset.verify_saved_media_logs)
            if any(dataset.case_enabled(category) for category in ("database", "browser", "business-browser")):
                logger.log("[e2e 3/4] 执行 PostgreSQL 计量核对与真实浏览器验收")
            if dataset.case_enabled("database"):
                dataset.report["database"] = dataset.run_case("database", verify_database, dataset)
            dataset.save()
            if dataset.case_enabled("browser"):
                dataset.report["browser"] = dataset.run_case("browser", run_browser, directory, env, gateway, ui_port, processes, handles, logger)
            if dataset.case_enabled("business-browser"):
                dataset.report["business_browser"] = dataset.run_case("business-browser", run_business_browser, directory, logger)
            dataset.save()
            if args.with_regression and dataset.case_enabled("regression"):
                logger.log("[e2e 4/4] 执行后台 regression 测试")
                dataset.run_case("regression", command_stream, ["bash", "scripts/regression.sh", "-v"], logger)
                dataset.report["backend_regression"] = {"command": "bash scripts/regression.sh -v", "status": "passed"}
            dataset.report["status"] = "passed"
            dataset.save()
            logger.log("真实验收通过，报告: " + str(directory / "report.json"))
    except Exception as error:
        if dataset:
            dataset.report.update(status="failed", error=type(error).__name__ + ": " + str(error))
            dataset.save()
        logger.log("❌ [real-acceptance / runner / 验收失败] " + type(error).__name__ + ": " + str(error) + "；日志目录: " + str(directory))
        return 1
    finally:
        if observer:
            restore_supplier_addresses(dataset, data)
        if dataset:
            dataset.save()
            if dataset.checklist:
                dataset.checklist.summary()
        for process in reversed(processes):
            process.terminate()
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
        if observer:
            evidence = directory / "upstream-observations.json"
            earlier = json.loads(evidence.read_text()) if evidence.exists() else []
            evidence.write_text(json.dumps(earlier + observer.rows, ensure_ascii=False, indent=2) + "\n")
            observer.close()
        for handle in handles:
            handle.close()
        (directory / "retained.txt").write_text("保留 PostgreSQL xhub/public 与 Redis DB 1，供 make e2e 和人工复核。\n")
    return 0

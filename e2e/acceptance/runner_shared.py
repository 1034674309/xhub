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
from .local_auth import local_admin_credentials
from .support import write_private, http
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
    if not getattr(dataset, "isolated_upstreams", False):
        raise ValueError("请求观察器只允许连接隔离验收副本，禁止改写共享部署地址")
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
    """用途：持锁分发建数和真实网关验收；参数为 CLI 和清单，返回退出码；verify 不编译、不启动服务、不改写上游地址。"""
    if args.phase == "seed":
        return shared_main_environment(args, data)
    return verify_running_gateway(args, data)


def verify_running_gateway(args, data):
    """用途：验收已经运行的真实网关；参数为 CLI 和清单，返回退出码，供 make e2e 调用。

    仅读取现有基线；认证使用显式环境或基线会话，不启动服务，不创建观察器。
    需要故障注入或自启服务的分类明确记为 unsupported；网关不可达和认证失败立即失败。
    """
    directory = Path(args.directory).resolve()
    directory.mkdir(parents=True, exist_ok=True)
    logger = RunLogger(directory / "e2e.log")
    gateway = os.environ.get("E2E_GATEWAY", "http://127.0.0.1:4000").rstrip("/")
    dataset = None
    unsupported = {"profile-isolation", "cache-weight", "fallback", "business-browser", "regression"}
    try:
        status, _, _ = http(gateway, "/health/liveliness")
        if status != 200:
            raise RuntimeError("真实网关健康检查失败")
        saved = json.loads((directory / "access.json").read_text())
        dataset = module.Dataset(gateway, directory, data, logger=logger.log)
        dataset.state = {key: value for key, value in saved.items() if key not in ("admin", "gateway")}
        baseline_report = json.loads((directory / "report.json").read_text())
        # 兼容此前错误地重复编码的失败报告；未知格式保留默认结构并给出业务错误。
        if isinstance(baseline_report, str):
            baseline_report = json.loads(baseline_report)
        if not isinstance(baseline_report, dict):
            raise ValueError("基线 report.json 必须为 JSON 对象")
        dataset.report = baseline_report
        dataset.report.update(status="running", gateway=gateway, phase="verify", gateway_mode="existing",
                              unsupported_categories=[], verification_scope="selected" if args.category else "real-gateway")
        dataset.report.pop("error", None)
        dataset.media_jobs = int(os.environ.get("E2E_JOBS", "4"))
        if not 1 <= dataset.media_jobs <= 16:
            raise ValueError("E2E_JOBS 必须在 1 到 16 之间")
        dataset.representative = True
        categories = args.category or [name for name in E2E_CATEGORIES if name not in unsupported]
        dataset.start_checklist(True, categories)
        dataset.admin = os.environ.get("E2E_DATASET_TOKEN") or saved.get("admin")
        auth = local_admin_credentials(gateway, ROOT)
        if auth.get("E2E_DATASET_PASSWORD"):
            login, _ = dataset.api("/v2/login", {"username": auth.get("E2E_DATASET_ADMIN", "admin@xhub.local"),
                                               "password": auth["E2E_DATASET_PASSWORD"]})
            dataset.admin = login["key"]
        if not dataset.admin:
            raise ValueError("缺少真实网关管理员会话，请设置 E2E_DATASET_TOKEN 或 E2E_DATASET_PASSWORD")
        dataset.api("/v2/model/info?size=200")
        requested = sorted(set(categories) & unsupported)
        dataset.report["unsupported_categories"] = requested
        dataset.report["isolated_only_categories"] = sorted(unsupported)
        if requested:
            raise ValueError("真实网关模式不支持自启服务或故障注入分类: " + ", ".join(requested) +
                             "；请显式选择真实业务分类，隔离测试使用 make e2e-offline 或 make regression")
        logger.log("连接已经运行的真实网关：" + gateway)
        dataset.verify()
        dataset.run_case("media-logs", dataset.verify_saved_media_logs)
        if dataset.case_enabled("database"):
            dataset.report["database"] = dataset.run_case("database", verify_database, dataset)
        if dataset.case_enabled("browser"):
            if not auth.get("E2E_DATASET_PASSWORD"):
                raise ValueError("浏览器登录需要真实网关管理员密码，请设置 E2E_DATASET_ADMIN 和 E2E_DATASET_PASSWORD")
            dataset.validate_browser_codex_sessions()
            env = {**os.environ, **auth, "E2E_DATASET_GATEWAY": gateway,
                   "E2E_DATASET_UI": os.environ.get("E2E_UI", "http://127.0.0.1:3000"),
                   "E2E_DATASET_DIR": str(directory)}
            with dataset.case("browser"):
                command_stream(["node", str(ROOT / "e2e/real_dataset_browser.cjs")], logger, env=env)
                dataset.report["browser"] = json.loads((directory / "browser-report.json").read_text())
        dataset.report["status"] = "passed"
        return 0
    except Exception as error:
        if dataset:
            dataset.report.update(status="failed", error=type(error).__name__ + ": " + str(error))
        else:
            # 健康检查在读取基线前失败时也覆盖本轮状态，避免旧的通过报告被误认为本次结果。
            write_private(directory / "report.json", {
                "status": "failed", "gateway": gateway, "phase": "verify", "gateway_mode": "existing",
                "error": type(error).__name__ + ": " + str(error),
            })
        logger.log("真实网关验收失败：" + type(error).__name__ + ": " + str(error))
        return 1
    finally:
        if dataset:
            dataset.save()
            if dataset.checklist:
                dataset.checklist.summary()


def shared_main_environment(args, data):
    """用途：通过已有真实网关构建基线；参数为 seed CLI 和清单，返回退出码，供 make testdata 调用。

    使用现有管理员密码，不启动进程、不生成配置、不直接清库；空租户检查由 seed 通过管理接口执行。
    健康或凭据失败在修改基线文件前返回，业务失败保留本轮检查点以供排查。
    """
    try:
        validate_providers(data)
    except ValueError as error:
        raise SystemExit(str(error))
    gateway = os.environ.get("E2E_GATEWAY", "http://127.0.0.1:4000").rstrip("/")
    auth = local_admin_credentials(gateway, ROOT)
    if not auth.get("E2E_DATASET_PASSWORD"):
        raise SystemExit("建数需要现有网关管理员密码，请设置 E2E_DATASET_PASSWORD；账号由 E2E_DATASET_ADMIN 指定，默认 admin@xhub.local")
    directory = Path(args.directory).resolve()
    dataset = None
    try:
        status, _, _ = http(gateway, "/health/liveliness")
        if status != 200:
            raise RuntimeError("真实网关健康检查失败")
        logger = RunLogger(directory / "testdata.log")
        dataset = module.Dataset(gateway, directory, data, logger=logger.log)
        dataset.login_credentials = auth
        dataset.seed()
        dataset.report["counts"] = {name: len(dataset.state[name]) for name in ("organizations", "teams", "users", "projects", "keys")}
        dataset.report.update(status="seeded", gateway=gateway, gateway_mode="existing")
        dataset.save()
        logger.log("构建完成，模型调用=0；make testdata 和 make e2e 均复用已经运行的真实网关")
        return 0
    except Exception as error:
        if dataset:
            dataset.report.update(status="failed", error=type(error).__name__ + ": " + str(error))
            dataset.save()
        print("建数失败：" + type(error).__name__ + ": " + str(error), file=sys.stderr)
        return 1

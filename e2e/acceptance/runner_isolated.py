"""私有 schema/Redis 的完整验收生命周期；正常结束和异常均清理本轮创建的资源。"""
import json
import os
import re
from pathlib import Path
import secrets
import subprocess
import sys

from . import dataset as module
from .support import load_manifest, write_private
from urllib.parse import urlsplit
from . import observer as observer_module
from .runner_runtime import ROOT, command, sql, free_port, ready
from .runner_shared import restore_supplier_addresses, attach_chat_observer


def run_isolated(args):
    """用途：运行隔离全套验收；参数为已校验 CLI，返回退出码；供统一入口调用，不改 public，keep-data 才保留本轮 schema。"""
    data = load_manifest()
    for provider in data["providers"]:
        if not os.environ.get(provider["key_env"]):
            raise SystemExit("缺少环境凭据: " + provider["key_env"])
        base = os.environ.get(provider["base_env"], provider["base"])
        url = urlsplit(base)
        if url.scheme != "https" or not url.hostname or url.username or url.password or any(c in base for c in "[]() \n\r\t"):
            raise SystemExit("供应商地址必须是无 Markdown 的 HTTPS URL")
    directory = Path(args.directory).resolve()
    if not args.resume and (directory / "c.yaml").exists():
        raise SystemExit("目录已包含运行基线；请使用 --resume --keep-data 或新目录")
    directory.mkdir(parents=True, exist_ok=True)
    os.chmod(directory, 0o700)
    suffix = secrets.token_hex(6)
    schema, redis_name = "e2e_real_" + suffix, "xhub-real-" + suffix
    gateway_port, ui_port, redis_port = free_port(), free_port(), free_port()
    password = secrets.token_urlsafe(24)
    env = {**os.environ, "E2E_DATASET_PASSWORD": password, "E2E_DATASET_ADMIN": "admin"}
    gateway = "http://127.0.0.1:" + str(gateway_port)
    env["XHUB_PUBLIC_ORIGIN"] = gateway
    config = {"model_list": [], "general_settings": {"master_key": "sk-" + secrets.token_hex(24), "admin_email": "admin", "admin_name": "admin",
              "admin_password": password, "store_model_in_db": True, "store_prompts_in_spend_logs": True,
              "database_url": "postgres://xhub:xhub_dev_password@127.0.0.1:5433/xhub?sslmode=disable&search_path=" + schema,
              "redis_url": "redis://127.0.0.1:" + str(redis_port) + "/0"}}
    if args.resume:
        config = json.loads((directory / "c.yaml").read_text())
        schema = (directory / "schema.txt").read_text().strip()
        if not re.fullmatch(r"e2e_real_[a-f0-9]{12}", schema):
            raise ValueError("续跑只允许本入口创建的隔离schema")
        config["general_settings"]["redis_url"] = "redis://127.0.0.1:" + str(redis_port) + "/0"
        password = config["general_settings"]["admin_password"]
        env["E2E_DATASET_PASSWORD"] = password
    processes, observer, dataset = [], None, None
    schema_created = redis_created = False
    handles = []
    try:
        command(["go", "build", "-o", str(directory / "xhub"), "./cmd/gateway"])
        if not args.resume:
            sql("CREATE SCHEMA " + schema)
        schema_created = True
        command(["docker", "run", "--rm", "-d", "--name", redis_name, "-p", f"127.0.0.1:{redis_port}:6379", "redis:7-alpine"], capture_output=True)
        redis_created = True
        write_private(directory / "c.yaml", config)
        (directory / "schema.txt").write_text(schema + "\n")
        log = open(directory / "gateway.log", "a")
        handles.append(log)
        gw = subprocess.Popen([str(directory / "xhub"), "-config", str(directory / "c.yaml"), "-addr", f"127.0.0.1:{gateway_port}"],
                              cwd=ROOT, env=env, stdout=log, stderr=log)
        processes.append(gw)
        ready(gateway + "/health/liveliness", gw)
        observer = observer_module.Observer(data["providers"], data["limits"]["max_upstream_attempts"])
        if args.resume and (directory / "upstream-observations.json").exists():
            observer.attempts = len(json.loads((directory / "upstream-observations.json").read_text()))
        os.environ["E2E_DATASET_PASSWORD"] = password
        os.environ["E2E_DATASET_ADMIN"] = "admin"
        dataset = module.Dataset(gateway, directory, data, observer)
        dataset.isolated_upstreams = True
        if args.resume:
            saved = json.loads((directory / "access.json").read_text())
            dataset.state = {k: v for k, v in saved.items() if k not in ("admin", "gateway")}
            dataset.report = json.loads((directory / "report.json").read_text())
            dataset.report["status"] = "running"
            dataset.report.pop("error", None)
            if not any(c["name"] == "81-personal-keys-real-calls-and-billing" and c["passed"] for c in dataset.report["checks"]):
                raise ValueError("续跑要求已有81把真实请求及账单证据")
            login, _ = dataset.api("/v2/login", {"username": "admin", "password": password})
            dataset.admin = login["key"]
            # 中断的回退验收可能已完成请求而尚未清理，续跑先删除明确标记的临时故障部署。
            for deployment in list(dataset.state["deployments"]):
                if deployment.get("temporary_fault"):
                    dataset.api("/model/fallback", {"model_name": deployment["public_name"], "policy": {}}, method="PUT")
                    dataset.api("/model/delete", {"id": deployment["id"]})
                    dataset.state["deployments"].remove(deployment)
            allowed_models = [deployment["public_name"] for deployment in dataset.state["deployments"]]
            for team in dataset.state["teams"]:
                dataset.api("/team/update", {"team_id": team["id"], "models": allowed_models})
            attach_chat_observer(dataset, observer)
            dataset.verify_acceptance_models()
            if not all(dataset.checked("permissions-" + role["role"]) for role in data["personas"]):
                dataset.verify_permissions()
            key = dataset.state["keys"][0]
            dataset.verify_limits(key)
            dataset.verify_model_lifecycle(key)
            dataset.verify_guardrail_lifecycle(key)
            dataset.save()
        else:
            dataset.seed()
            print("真实数据创建完成：3组织 / 9团队 / 27成员 / 81密钥；开始请求验收", flush=True)
            dataset.verify()
        if args.api_only:
            dataset.report["browser"] = "not-run (--api-only)"
        else:
            frontend = ROOT / "frontend"
            # 独立运行不依赖其他E2E脚本预先生成的忽略文件。
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
            ready(ui_env["E2E_DATASET_UI"] + "/ui/login/", ui, timeout=120)
            command(["node", str(ROOT / "e2e/real_dataset_browser.cjs")], env=ui_env)
            dataset.report["browser"] = json.loads((directory / "browser-report.json").read_text())
        # SQL 验收覆盖每日汇总与实体累计金额，失败时仍保留报告；不读取或输出原始请求/凭据。
        statement = f"""SET search_path TO {schema};
SELECT json_build_object('events',count(*),'unique_calls',count(distinct request_id),'cost',coalesce(sum(cost),0)) FROM usage_events WHERE status='success';
SELECT coalesce(sum(cost),0) FROM usage_daily;
"""
        lines = sql(statement).strip().splitlines()
        dataset.report["database"] = json.loads(lines[0])
        if abs(float(lines[1]) - dataset.report["database"]["cost"]) > 1e-9:
            raise AssertionError("每日聚合金额与真实请求事件不一致")
        counters = sql(f"SET search_path TO {schema}; SELECT json_build_object('requests',count(*),'prompt_tokens',coalesce(sum(prompt_tokens),0),'completion_tokens',coalesce(sum(completion_tokens),0)) FROM usage_events; SELECT json_build_object('requests',coalesce(sum(requests),0),'prompt_tokens',coalesce(sum(prompt_tokens),0),'completion_tokens',coalesce(sum(completion_tokens),0)) FROM usage_daily;").strip().splitlines()
        if json.loads(counters[0]) != json.loads(counters[1]):
            raise AssertionError("事件与每日汇总的请求数或token不一致")
        dataset.report["database"]["metering"] = json.loads(counters[0])
        # 续跑也重新读取历史真实响应：81个已有证据不能只依赖上一轮的通过标记。
        dataset.report["database"]["response_usage"] = audit_response_usage(schema)
        for table, column, dimension in (("organizations", "id", "organization_id"), ("teams", "id", "team_id"),
                                         ("users", "id", "user_id"), ("projects", "id", "project_id"), ("api_keys", "id", "key_id")):
            mismatch = sql(f"SET search_path TO {schema}; SELECT count(*) FROM {table} x WHERE abs(x.spend - coalesce((SELECT sum(cost) FROM usage_events u WHERE u.{dimension}=x.{column}),0)) > 0.000000001;").strip()
            if mismatch != "0":
                raise AssertionError("累计计费不一致: " + table)
        dataset.report["checks"].append({"name": "postgres-events-daily-five-owner-spend", "passed": True})
        dataset.report["status"] = "passed"
        dataset.save()
        print("真实验收通过，报告: " + str(directory / "report.json"), flush=True)
    except Exception as error:
        if dataset:
            dataset.report.update(status="failed", error=str(error))
            dataset.save()
        print("验收失败: " + str(error) + "；日志目录: " + str(directory), file=sys.stderr)
        return 1
    finally:
        if dataset and args.keep_data:
            # 观察器退出后基线仍可调用；只恢复曾经改写到观察器的聊天部署。
            restore_supplier_addresses(dataset, data)
            dataset.save()
        for process in reversed(processes):
            process.terminate()
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
        if observer:
            evidence = directory / "upstream-observations.json"
            earlier = json.loads(evidence.read_text()) if args.resume and evidence.exists() else []
            evidence.write_text(json.dumps(earlier + observer.rows, ensure_ascii=False, indent=2))
            observer.close()
        for handle in handles:
            handle.close()
        if redis_created:
            subprocess.run(["docker", "stop", redis_name], capture_output=True)
        if schema_created and not args.keep_data:
            sql("DROP SCHEMA " + schema + " CASCADE")
            (directory / "cleanup.txt").write_text("隔离 PostgreSQL schema 和 Redis 容器已删除；保留报告。\n")
        if args.keep_data:
            (directory / "retained.txt").write_text("保留 schema: " + schema + "\n重新启动前为 c.yaml 配置可用 Redis 并导出供应商环境凭据。\n")
    return 0

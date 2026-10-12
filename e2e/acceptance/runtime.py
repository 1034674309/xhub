"""真实验收的 runtime 模块；共享上下文由 Dataset 静态组合，资源与失败证据归本轮所有。"""
from contextlib import nullcontext
import base64
import hashlib
import json
import os
from pathlib import Path
import threading
import time

from checklist import Checklist
from .support import acceptance_run_id, write_private, http


class RuntimeCases:
    """用途：封装 runtime 场景；通过 self 访问本轮状态、管理 API 和证据，供 Dataset 调用，不自行启动全套测试。"""

    def __init__(self, gateway, directory, data, observer=None, logger=None):
        """用途：准备运行状态；参数为网关/私有目录/清单/观察器/日志回调，返回实例；保存密钥独立于公开证据。"""
        self.gateway, self.directory, self.data, self.observer = gateway, Path(directory), data, observer
        self.logger = logger or (lambda message: print(message, flush=True))
        self.log_lock, self.api_sequence = threading.Lock(), 0
        self.directory.mkdir(parents=True, exist_ok=True)
        os.chmod(self.directory, 0o700)
        self.state = {"keys": [], "organizations": [], "teams": [], "users": [], "projects": [], "deployments": []}
        self.report = {"checks": [], "probe_attempts": [], "calls": [], "models": [], "status": "running"}
        self.run = acceptance_run_id()
        self.admin = None
        self.representative = False
        self.checklist = None

    def acceptance_keys(self):
        """用途：选择不同路由规则的代表密钥；无参数，返回稳定列表；共享验收每类一把，其他入口保留全量，不修改基线，空列表由会话验收拒绝。"""
        if not self.representative:
            return self.state["keys"]
        chosen = {}
        for key in self.state["keys"]:
            profile = key["profile"]
            category = ("organization" if key["team_index"] == 0 else "team") if profile == "inherit" else profile
            chosen.setdefault(category, key)
        if set(chosen) != {"organization", "team", "fenno-only", "qiniu-only"}:
            raise AssertionError("代表密钥必须覆盖组织继承、团队继承和两种单供应商配置")
        return list(chosen.values())

    def case(self, ident):
        """用途：进入编号场景；参数为清单 ID，返回上下文；shared runner 保存结果，其他调用兼容原行为，无业务副作用。"""
        return self.checklist.case(ident) if self.checklist else nullcontext({})

    def run_case(self, ident, callback, *args):
        """用途：执行选中的独立业务场景；参数为 ID、函数及位置参数，返回函数结果或未选中的 None；失败原样抛出，不执行未选中回调。"""
        if not self.case_enabled(ident):
            return None
        with self.case(ident):
            return callback(*args)

    def case_enabled(self, ident):
        """用途：判断业务分类是否选中；参数为清单 ID，返回布尔；完整验收默认全部开启，分类验收不把跳过项记为通过，无副作用。"""
        selected = getattr(self, "selected_categories", None)
        return selected is None or ident in selected

    def start_checklist(self, with_regression=True, categories=None):
        """用途：公布去重验收计划；参数为回归开关和可选分类列表，无返回；分类须全部存在，过滤后才创建报告，未知/空分类拒绝。"""
        plan = [("gpt-" + str(index), "GPT 三轮上下文、粘滞与五级账单：" + key["profile"] +
                 (" / 组织继承" if key.get("team_index") == 0 else " / 团队继承") if key["profile"] == "inherit"
                 else "GPT 三轮上下文、粘滞与五级账单：" + key["profile"])
                for index, key in enumerate(self.acceptance_keys(), 1)]
        plan.insert(0, ("entities", "实体基线及团队成员"))
        plan += [("glm", "GLM 三轮上下文与账单"), ("image", "图片生成、文件与精确请求日志"),
                 ("ark-video", "Ark 视频创建、终态、日志与重复结算"),
                 ("fal-video", "FAL Seedance 视频创建、终态、日志与重复结算"),
                 ("kling-video", "FAL Kling 视频创建、终态、日志与重复结算"),
                 ("inheritance", "组织/团队/个人路由继承"),
                 ("chat-bills", "Chat 入口代表请求与价格快照"),
                 ("permissions", "四类角色权限与跨组织拒绝"), ("xgo", "XGo 持久化、调试、拦截、脱敏与放行"),
                 ("limits", "五级预算、RPM/TPM、模型白名单与密钥生命周期"),
                 ("model-lifecycle", "模型创建、下架、恢复与删除"), ("guard-lifecycle", "护栏编辑、默认启用与删除"),
                 ("global-guards", "全局拦截无外发与个人信息转发前脱敏"), ("profile-isolation", "单供应商路由隔离"),
                 ("cache-weight", "缓存零费用、跨密钥隔离、封禁及权重更新失效"),
                 ("fallback", "429 回退、成功目标复用、真实调用与目标权限"),
                 ("media-logs", "精确媒体日志与终态证据复验"), ("database", "事件、日报、token、五级金额与结算去重"),
                 ("browser", "真实数据的控制台、日志、护栏与角色页面"),
                 ("business-browser", "去重浏览器业务：密钥、模型、用户、护栏、Playground、错误及任务日志")]
        if with_regression:
            plan.append(("regression", "后台接口契约、权限、预算、协议与持久化回归"))
        if categories is not None:
            unknown = set(categories) - {ident for ident, _ in plan}
            if not categories or unknown:
                raise ValueError("未知或空 E2E 分类: " + ", ".join(sorted(unknown)))
            plan = [(ident, name) for ident, name in plan if ident in categories]
        self.selected_categories = None if categories is None else frozenset(categories)
        self.checklist = Checklist(self.directory, self.logger, plan)

    def save(self):
        """用途：落盘检查点和脱敏报告；无参数/返回，供每个阶段调用；访问材料保存在忽略目录 0600 文件。"""
        write_private(self.directory / "access.json", {**self.state, "admin": self.admin, "gateway": self.gateway})
        (self.directory / "report.json").write_text(json.dumps(self.report, ensure_ascii=False, indent=2) + "\n")

    def progress(self, message):
        """用途：把不含凭据的构建与验收进度立即输出到终端；参数为日志正文，无返回值；供长时间真实请求显示当前步骤。"""
        self.logger(message)

    def action_start(self, model, protocol, action, detail):
        """用途：记录单项验收开始；参数为模型、协议、动作和脱敏说明，无返回值；调用方在真实操作前使用。"""
        self.progress(f"    ▶ [{model} / {protocol} / {action}] {detail}")

    def action_wait(self, model, protocol, action, detail):
        """用途：记录异步任务等待状态；参数为模型、协议、动作和脱敏说明，无返回值；不会把轮询当作成功。"""
        self.progress(f"    ⏳ [{model} / {protocol} / {action}] {detail}")

    def action_ok(self, model, protocol, action, detail, record=True):
        """用途：记录一个已验证成功项；参数含是否写报告，返回无；只允许在断言完成后调用。"""
        self.progress(f"    ✅ [{model} / {protocol} / {action}] {detail}")
        if record:
            self.report["checks"].append({"name": f"{model}:{protocol}:{action}", "model": model,
                                          "protocol": protocol, "action": action, "passed": True})

    def action_fail(self, model, protocol, action, error):
        """用途：记录单项验收失败；参数为模型、协议、动作和异常，无返回值；不输出请求正文与密钥。"""
        self.progress(f"    ❌ [{model} / {protocol} / {action}] {type(error).__name__}: {error}")

    def probe_warning(self, model, protocol, action, detail, retry):
        """用途：输出候选探测的非致命警告；参数为模型、协议、动作、脱敏诊断及是否还有重试，返回无；供探测失败调用，仅打印下一步，不写正式 checks 或改变用例状态。"""
        next_step = "将重试当前候选" if retry else "将继续下一候选（如有）；全部候选不可用时用例失败"
        self.progress(f"    ⚠️ [{model} / {protocol} / {action}] 候选探测未成功：{detail}；{next_step}；本次探测不计入用例失败")

    def result_json(self, filename, value):
        """用途：把 bypass 返回结果保存为可复核 JSON；参数为文件名和脱敏后的响应，返回绝对路径；文件权限为0600。"""
        directory = self.directory / "artifacts"
        directory.mkdir(parents=True, exist_ok=True)
        path = directory / filename
        write_private(path, value)
        return str(path.resolve())

    def image_result(self, model, answer):
        """用途：提取并落盘真实图片 bypass 结果；参数为模型和响应，返回可打印证据；base64 图片保存为二进制且 JSON 不重复保存大正文。"""
        first = answer["data"][0]
        compact = {key: value for key, value in answer.items() if key != "data"}
        compact["data"] = [{key: value for key, value in first.items() if key != "b64_json"}]
        if first.get("b64_json"):
            raw = base64.b64decode(first["b64_json"], validate=True)
            if raw.startswith(b"\x89PNG\r\n\x1a\n"):
                extension = ".png"
            elif raw.startswith(b"\xff\xd8\xff"):
                extension = ".jpg"
            else:
                extension = ".bin"
            image_path = self.directory / "artifacts" / (model.replace("/", "_") + extension)
            image_path.parent.mkdir(parents=True, exist_ok=True)
            image_path.write_bytes(raw)
            os.chmod(image_path, 0o600)
            evidence = {"type": "b64_json", "bytes": len(raw), "sha256": hashlib.sha256(raw).hexdigest(),
                        "artifact": str(image_path.resolve())}
            compact["data"][0]["b64_json"] = evidence
        else:
            evidence = {"type": "url", "url": first["url"]}
        evidence["response_json"] = self.result_json(model.replace("/", "_") + "-bypass-result.json", compact)
        return evidence

    def api(self, route, body=None, token=None, method=None, expected=200, include_status=False, request_headers=None):
        """用途：验证管理或数据接口；参数为请求、期望状态、状态码开关及可选客户端头，返回 JSON/头或 JSON/头/状态；正文不出现在失败日志，避免凭据泄露。"""
        verb = method or ("GET" if body is None else "POST")
        safe_route = route.split("?", 1)[0]
        with self.log_lock:
            self.api_sequence += 1
            sequence = self.api_sequence
        started = time.monotonic()
        expected_values = (expected,) if isinstance(expected, int) else tuple(expected)
        expected_label = "/".join(str(value) for value in expected_values)
        self.progress(f"    [接口 {sequence:04d}] {verb} {safe_route} 开始，预期={expected_label}")
        try:
            options = {"request_headers": request_headers} if request_headers else {}
            status, value, headers = http(self.gateway, route, body, token or self.admin, method, **options)
        except Exception as error:
            self.progress(f"    [接口 {sequence:04d}] {verb} {safe_route} 网络失败，耗时={time.monotonic() - started:.3f}s，错误={type(error).__name__}")
            raise
        self.progress(f"    [接口 {sequence:04d}] {verb} {safe_route} 完成，状态={status}，耗时={time.monotonic() - started:.3f}s")
        if status not in expected_values:
            raise AssertionError(f"{verb} {safe_route} 状态 {status}，预期 {expected_label}")
        if include_status:
            return value, headers, status
        return value, headers

    def checked(self, name):
        """用途：判断续跑场景已有成功证据；参数为唯一场景名，返回布尔；只跳过明确通过项，失败和未执行项继续运行。"""
        # 编号清单表示本轮执行，不能把旧通过记录当成本轮执行结果；非清单续跑仍保留历史兼容。
        return not self.checklist and any(c["name"] == name and c.get("passed") for c in self.report["checks"])

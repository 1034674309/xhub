"""真实验收的 catalog 模块；共享上下文由 Dataset 静态组合，资源与失败证据归本轮所有。"""
import json
import os
import random
import time

from .support import chat_probe_result, resolve_document, http, curl_json, chat_candidates


class CatalogCases:
    """用途：封装 catalog 场景；通过 self 访问本轮状态、管理 API 和证据，供 Dataset 调用，不自行启动全套测试。"""

    def discover_catalog(self, provider, attempts=3, delay=5):
        """用途：优先通过已持久化连接读取真实目录，代理失败后从同一供应商只读接口实时回退；参数为供应商定义、最大次数和间隔，返回非空目录；两条路径都不接受静态或伪造模型。"""
        name = provider["name"]
        for attempt in range(1, attempts + 1):
            self.action_start(name, "provider-catalog", f"第 {attempt}/{attempts} 次读取真实模型目录",
                              f"连接名={provider['id']}，POST /model/builtin/models")
            catalog, _ = self.api("/model/builtin/models", {"credential_name": provider["id"]})
            model_ids = catalog.get("model_ids", []) if isinstance(catalog, dict) else []
            error = catalog.get("error") if isinstance(catalog, dict) else "响应不是 JSON 对象"
            if not error and model_ids:
                self.action_ok(name, "provider-catalog", "读取真实模型目录",
                               f"连接名={provider['id']}，模型数={len(model_ids)}，第 {attempt}/{attempts} 次成功")
                return catalog
            detail = str(error or "HTTP=200 但 model_ids 为空")[:300]
            self.action_fail(name, "provider-catalog", f"第 {attempt}/{attempts} 次读取真实模型目录",
                             RuntimeError(detail))
            if attempt < attempts:
                self.action_wait(name, "provider-catalog", "真实模型目录重试",
                                 f"{delay} 秒后进行第 {attempt + 1}/{attempts} 次只读查询")
                time.sleep(delay)
        self.action_wait(name, "provider-catalog", "供应商目录直连回退",
                         "已保存连接的目录代理连续失败，改用同一连接凭据读取供应商官方 /v1/models")
        try:
            return self.discover_supplier_catalog(provider, attempts=attempts, delay=delay)
        except Exception as error:
            raise RuntimeError("真实目录发现失败: " + provider["id"] +
                               f"；连接代理和供应商官方目录各连续 {attempts} 次失败") from error

    def discover_supplier_catalog(self, provider, attempts=3, delay=5):
        """用途：从供应商官方 OpenAI 目录读取实时模型 ID；参数为供应商定义、最大次数和间隔，返回标准 model_ids；仅在已保存连接的目录代理失败后调用，所有尝试均打印结果。"""
        name = provider["name"]
        base = os.environ.get(provider["base_env"], provider["base"]).rstrip("/")
        route = "/models" if base.endswith("/v1") else "/v1/models"
        for attempt in range(1, attempts + 1):
            self.action_start(name, "provider-catalog-direct", f"第 {attempt}/{attempts} 次读取供应商官方目录",
                              f"连接名={provider['id']}，GET {route}")
            try:
                status, response, _ = curl_json(base, route, None, os.environ[provider["key_env"]], timeout=90)
                rows = response.get("data", []) if isinstance(response, dict) else []
                model_ids = [row.get("id") for row in rows if isinstance(row, dict) and row.get("id")]
                if status == 200 and model_ids:
                    catalog = {"model_ids": list(dict.fromkeys(model_ids)), "source": "supplier-official-live"}
                    self.action_ok(name, "provider-catalog-direct", "读取供应商官方目录",
                                   f"连接名={provider['id']}，HTTP=200，模型数={len(catalog['model_ids'])}，第 {attempt}/{attempts} 次成功")
                    return catalog
                error = response.get("error") if isinstance(response, dict) else None
                raise RuntimeError(f"HTTP={status}，模型数={len(model_ids)}，错误={str(error)[:200]}")
            except Exception as error:
                self.action_fail(name, "provider-catalog-direct", f"第 {attempt}/{attempts} 次读取供应商官方目录", error)
                if attempt < attempts:
                    self.action_wait(name, "provider-catalog-direct", "供应商官方目录重试",
                                     f"{delay} 秒后进行第 {attempt + 1}/{attempts} 次只读查询")
                    time.sleep(delay)
        raise RuntimeError("供应商官方目录连续失败: " + provider["id"])

    def probe_chat_candidate(self, provider, upstream, probe_index, probe_total, attempts=1, delay=3,
                             action="真实模型候选探测", timeout=180):
        """用途：直接调用供应商探测一个真实聊天模型；参数为供应商、模型 ID、候选进度、重试次数、间隔、日志动作和超时，返回是否取得非空回答和非零 usage；正文为空且因长度截断时下一次提高输出上限至 1024，每次尝试保存无正文诊断，有限重试耗尽后淘汰候选且不污染正式验收 checks。"""
        protocol = "bypass_openai_chat"
        base = os.environ.get(provider["base_env"], provider["base"]).rstrip("/")
        label = f"{action} {probe_index}/{probe_total}"
        max_tokens = 32
        for attempt in range(1, attempts + 1):
            attempt_label = label if attempts == 1 else f"{label}，尝试 {attempt}/{attempts}"
            self.action_start(upstream, protocol, attempt_label,
                              f"供应商={provider['id']}，直接调用供应商聊天端点，超时={timeout}秒，max_tokens={max_tokens}")
            try:
                status, reply, _ = curl_json(
                    base, "/chat/completions" if base.endswith("/v1") else "/v1/chat/completions",
                    {"model": upstream, "messages": [{"role": "user",
                     "content": "Reply only OK. probe " + self.run}], "max_tokens": max_tokens},
                    os.environ[provider["key_env"]], timeout=timeout)
            except Exception as error:
                self.report.setdefault("probe_attempts", []).append({"name": "real-model-probe", "provider": provider["id"],
                    "model": upstream, "attempt": attempt, "attempts": attempts, "status": None,
                    "error": type(error).__name__, "max_tokens": max_tokens, "passed": False})
                # 网络异常可能携带地址或凭据；警告只展示异常类型，完整状态仍由探测证据记录。
                self.probe_warning(upstream, protocol, attempt_label, type(error).__name__, attempt < attempts)
                if attempt < attempts:
                    self.action_wait(upstream, protocol, label,
                                     f"{delay} 秒后重试当前真实模型，随后仍失败则继续下一候选")
                    time.sleep(delay)
                continue
            result = chat_probe_result(status, reply)
            self.report.setdefault("probe_attempts", []).append({"name": "real-model-probe", "provider": provider["id"],
                "model": upstream, "attempt": attempt, "attempts": attempts, "status": status,
                "max_tokens": max_tokens, **result})
            detail = (f"HTTP={status}，输入={result['prompt_tokens']}，输出={result['completion_tokens']}，"
                      f"结束原因={result['finish_reason']}，正文字符数={result['content_chars']}，"
                      f"推理字符数={result['reasoning_chars']}，响应错误={result['response_error']}")
            if result["passed"]:
                self.action_ok(upstream, protocol, attempt_label, detail, record=False)
                return True
            self.probe_warning(upstream, protocol, attempt_label, detail, attempt < attempts)
            if attempt < attempts:
                # 推理模型的输出预算可能先被思考耗尽；仅在上游明确截断时扩容，仍要求最终回答，不能把已计费当作成功。
                truncated = status == 200 and result["finish_reason"] == "length" and result["content_chars"] == 0 and not result["response_error"]
                if truncated:
                    max_tokens = 1024
                self.action_wait(upstream, protocol, label,
                                 ("正文为空且输出被长度限制截断，提高 max_tokens 至 1024；" if truncated else "") +
                                 f"{delay} 秒后重试当前真实模型，随后仍失败则继续下一候选")
                time.sleep(delay)
        return False

    def select_lifecycle_model(self, provider, candidates, attempts=2, delay=3):
        """用途：为模型生命周期验收选择额外的真实聊天模型；参数为供应商、去重候选、单候选重试次数和间隔，返回首个真实响应且 usage 非零的模型 ID；单候选失败会继续，全部失败时明确报错。"""
        total = len(candidates)
        for index, candidate in enumerate(candidates, 1):
            if self.probe_chat_candidate(provider, candidate, index, total, attempts=attempts, delay=delay,
                                         action="生命周期探测", timeout=120):
                return candidate
        raise RuntimeError(f"{total} 个额外真实聊天模型均未通过生命周期探测")

    def deployment(self, definition, provider, upstream=None, public_name=None, fault=False, temporary=False, route_tag=None):
        """用途：按真实模型定义注册部署；参数为定义、供应商、可选上游模型、可选公开模型名、故障开关、临时标记和观察标签，返回部署 ID；允许两个供应商的精确上游 ID 归入同一公开模型，标签区分部署，临时标记供异常退出清理。"""
        rates = self.data["billing"]
        upstream = upstream or definition["upstream_model"]
        public_name = public_name or (definition.get("public_name") if upstream == definition.get("upstream_model") else upstream) or upstream
        stored_model = definition.get("stored_model", upstream) if upstream == definition.get("upstream_model") else upstream
        info = {"transport": definition["transport"], "endpoint_types": definition["endpoint_types"]}
        params = {"model": stored_model, "custom_llm_provider": "openai",
                  "litellm_credential_name": provider["id"]}
        if definition["kind"] == "chat":
            params.update(max_tokens=self.data["limits"]["max_output_tokens"],
                          input_cost_per_token=rates["input_cost_per_token"],
                          output_cost_per_token=rates["output_cost_per_token"])
            info["pricing_source"] = "manual"
        else:
            info.update(pricing_source="catalog", base_model=definition["base_model"])
        if self.observer and definition["kind"] == "chat":
            # 不依赖退出清理保证共享部署安全：任何写入前必须确认目标已隔离。
            if not getattr(self, "isolated_upstreams", False):
                raise ValueError("请求观察器只允许连接隔离验收副本，禁止改写共享部署地址")
            tag = "fault" if fault else route_tag
            params["api_base"] = self.observer.base + "/" + provider["id"] + ("/" + tag if tag else "")
        self.action_start(public_name, definition["transport"], "添加模型与端点",
                          f"供应商连接={provider['id']}，上游模型={stored_model}")
        row, _ = self.api("/model/new", {"model_name": public_name, "litellm_params": params, "model_info": info})
        ident = row["model_info"]["id"]
        saved = {"id": ident, "public_name": public_name, "provider": provider["id"], "model": stored_model,
                 "kind": definition["kind"], "transport": definition["transport"],
                 "endpoint_types": definition["endpoint_types"], "observed": definition["kind"] == "chat",
                 "route_tag": "fault" if fault else route_tag,
                 "temporary_fault": fault, "temporary": temporary or fault or bool(route_tag)}
        self.state["deployments"].append(saved)
        self.save()
        self.action_ok(public_name, definition["transport"], "添加模型与端点", f"部署 ID={ident}")
        return ident

    def template(self, name, ids, profile, **scope):
        """用途：实例化清单整体路由文档；参数为名称/部署ID/策略/作用域，返回模板ID；阈值零禁用被动熔断，避免用例互相污染。"""
        body = resolve_document(self.data["routing"]["templates"][profile], {
            "shared_chat_model": self.data["shared_chat_model"]["public_name"],
            "fenno_deployment": ids[0], "qiniu_deployment": ids[1]})
        row, _ = self.api("/route_template/new", {"name": name, "body": body, **scope})
        return row["id"]

    def verify_model_lifecycle(self, key):
        """用途：读取真实目录并验证模型创建、下架拒绝、恢复调用与删除；参数为个人密钥，返回无，供共享和隔离验收调用。

        目录失败即停止；按真实网关契约检查下架错误，不依赖上游观察器；finally 清理临时密钥和部署并恢复团队模型列表。
        """
        if self.checked("model-add-disable-refuse-enable-real-request"):
            return
        provider = self.state["selected"]["qiniu"]["provider"]
        # 建数阶段只依赖清单；额外模型的实时目录发现归属于有供应商凭据的验收阶段。
        catalog = self.discover_catalog(provider)
        (self.directory / "qiniu-catalog.json").write_text(json.dumps(catalog, ensure_ascii=False, indent=2))
        deployed = {row["public_name"] for row in self.state["deployments"]}
        candidates = [row for row in chat_candidates(catalog, random.Random(self.run + "-lifecycle"))
                      if row not in deployed][:self.data["limits"]["max_probe_models_per_provider"]]
        if not candidates:
            raise RuntimeError("真实目录中没有未部署的额外聊天模型可用于生命周期验收")
        model = self.select_lifecycle_model(provider, candidates)
        self.action_ok(model, "bypass_openai_chat", "前置探测完成",
                       f"选中 {model}；真实供应商响应包含非空回答和非零 usage，开始创建、下架、恢复和删除流程", record=False)
        self.progress(f"    [模型生命周期] 真实模型={model} 创建、调用、下架、恢复、删除")
        definition = next(row for row in self.data["models"] if row["id"] == "qiniu-chat")
        ident = self.deployment(definition, provider, model, temporary=True)
        allowed = [row["public_name"] for row in self.state["deployments"] if row["id"] != ident]
        temp = None
        try:
            self.api("/team/update", {"team_id": key["team_id"], "models": allowed + [model]})
            temp, _ = self.api("/key/generate", {"owner_type": "personal", "user_id": key["user_id"],
                "team_id": key["team_id"], "project_id": key["project_id"], "key_alias": "模型生命周期", "models": [model]})
            subject = {**key, **temp}
            self.call(subject, "model-listed-" + self.run, model=model)
            self.api("/model/disable", {"model_name": model})
            # 下架必须由网关明确拒绝；供应商故障或权限错误不能冒充下架成功，统一 API 包装保留动作日志。
            answer, _, status = self.api("/v1/chat/completions", {"model": model,
                "messages": [{"role": "user", "content": "model-unlisted-" + self.run}]},
                token=subject["key"], expected=400, include_status=True)
            if "model_disabled" not in json.dumps(answer):
                raise AssertionError("下架请求未返回 model_disabled 错误")
            self.api("/model/enable", {"model_name": model})
            self.call(subject, "model-relisted-" + self.run, model=model)
            self.report["checks"].append({"name": "model-add-disable-refuse-enable-real-request", "disabled_status": status, "passed": True})
        finally:
            if temp:
                self.api("/key/delete", {"keys": [temp["key"]]})
            self.api("/model/delete", {"id": ident})
            self.state["deployments"] = [d for d in self.state["deployments"] if d["id"] != ident]
            self.api("/team/update", {"team_id": key["team_id"], "models": allowed})

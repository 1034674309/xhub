"""真实验收的 chat 模块；共享上下文由 Dataset 静态组合，资源与失败证据归本轮所有。"""
import json
import math

from .support import acceptance_run_id, header_value, bill_check


class ChatCases:
    """用途：封装 chat 场景；通过 self 访问本轮状态、管理 API 和证据，供 Dataset 调用，不自行启动全套测试。"""

    def codex_conversation(self, key, revalidate=False):
        """用途：执行或复验一把个人密钥的 Codex 三轮 Responses 会话；参数为密钥及只复验开关，返回完整会话证据或空；逐轮检查上下文、会话头、五级归属、usage 和价格，完整成功后保存，失败不写完整成功回执。"""
        config = self.data["agent_conversation"]
        saved = next((row for row in self.report.get("agent_conversations", [])
                      if row.get("key_id") == key["token_id"] and row.get("model") == key["call_model"]), None)
        if revalidate:
            if not saved or saved.get("agent_type") != "codex" or saved.get("model") != key["call_model"] or saved.get("profile") != key["profile"] or len(saved.get("turns", [])) != config["turns"]:
                return None
            try:
                previous = None
                ids = set()
                response_ids = set()
                for index, turn in enumerate(saved["turns"], 1):
                    if turn["call_id"] in ids or turn["response_id"] in response_ids or turn["turn"] != index or turn["previous_response_id"] != previous:
                        return None
                    bill, _, status = self.api("/spend/logs/ui/" + turn["call_id"], expected=(200, 404), include_status=True)
                    if status != 200:
                        return None
                    self.check_codex_turn(key, saved, turn, bill)
                    ids.add(turn["call_id"])
                    response_ids.add(turn["response_id"])
                    previous = turn["response_id"]
                return saved
            except (AssertionError, AttributeError, KeyError, TypeError, ValueError):
                return None
        session = "codex-" + acceptance_run_id(18)
        marker = "project-" + acceptance_run_id(12)
        conversation = {"agent_type": "codex", "session_id": session, "marker": marker,
                        "key_id": key["token_id"], "profile": key["profile"],
                        "model": key["call_model"], "turns": []}
        prompts = ["Remember the project marker " + marker + ". Reply with only that marker.",
                   "What project marker did I give you? Reply with only the marker.",
                   "Confirm the same project marker once more. Reply with only the marker."]
        previous = None
        provider = None
        for index, prompt in enumerate(prompts, 1):
            self.progress(f"    [会话第 {index}/3 轮] {key['call_model']} / {key['profile']}：" +
                          ("建立上下文" if index == 1 else "续接历史、核对上下文与独立账单"))
            body = {"model": key["call_model"], "instructions": config["instructions"],
                    "input": [{"role": "user", "content": prompt}], "store": False,
                    "stream": False, "max_output_tokens": config["max_output_tokens"]}
            if previous:
                body["previous_response_id"] = previous
            start = len(self.observer.rows) if self.observer else 0
            answer, headers = self.api(config["endpoint"], body, token=key["key"], request_headers={
                "User-Agent": config["user_agent"], "Session_id": session, "Originator": "codex_cli_rs"})
            call_id = header_value(headers, "x-litellm-call-id")
            if (not call_id or not answer.get("id")
                    or any(row["call_id"] == call_id or row["response_id"] == answer["id"]
                           for row in conversation["turns"])):
                raise AssertionError("Codex 第 %d 轮缺少独立请求或响应 ID" % index)
            bill = self.wait_bill(call_id)
            turn = {"turn": index, "call_id": call_id, "response_id": answer["id"],
                    "previous_response_id": previous, "prompt": prompt,
                    "request_body": body,
                    "prompt_tokens": bill["prompt_tokens"], "completion_tokens": bill["completion_tokens"],
                    "cost": bill_check(bill, key, self.data["billing"])}
            self.check_codex_turn(key, conversation, turn, bill, answer)
            if self.observer:
                observed = self.observed_since(start, marker)
                successful = [row for row in observed if row["status"] == 200 and row.get("forwarded")]
                if len(successful) != 1:
                    raise AssertionError("Codex 每轮必须具有一次真实供应商转发证据")
                row = successful[0]
                messages = json.loads(row["messages"])
                if len(messages) != 2 * index or messages[0].get("role") != "system" or messages[-1].get("content") != prompt:
                    raise AssertionError("Codex 上游历史条数、指令或当前输入错误")
                for prior_index, prior in enumerate(conversation["turns"]):
                    if messages[2 * prior_index + 1].get("content") != prior["prompt"] or marker not in messages[2 * prior_index + 2].get("content", ""):
                        raise AssertionError("Codex 上游丢失前轮用户或助手历史")
                if provider is not None and provider != row["provider"]:
                    raise AssertionError("Codex 同一会话未保持供应商粘滞")
                if any(int(row.get("usage", {}).get(field, -1)) != int(turn[field])
                       for field in ("prompt_tokens", "completion_tokens")):
                    raise AssertionError("Codex 网关计量与真实供应商不一致")
                provider = row["provider"]
                expected_provider = {"fenno-only": "fennoai", "qiniu-only": "qiniu"}.get(key["profile"])
                if expected_provider and provider != expected_provider:
                    raise AssertionError("Codex 会话违反单供应商路由配置")
            conversation["turns"].append(turn)
            self.progress(f"    [会话第 {index}/3 轮通过] 上下文、usage、归属、费用与供应商粘滞已核对")
            previous = answer["id"]
        conversation["provider"] = provider
        self.report["agent_conversations"] = [row for row in self.report.get("agent_conversations", [])
                                               if (row.get("key_id"), row.get("model")) != (key["token_id"], key["call_model"])] + [conversation]
        self.save()
        return conversation

    def check_codex_turn(self, key, conversation, turn, bill, answer=None):
        """用途：核对 Codex 一轮持久化证据；参数为身份、会话、轮次、账单和可选实时响应，无返回值；执行及断点复验共用，缺失上下文、客户端头、关联或计费时抛断言，无写入副作用。"""
        response = bill.get("response", {})
        if not conversation.get("session_id") or not conversation.get("marker") or not isinstance(response, dict):
            raise AssertionError("Codex 会话或响应证据缺失")
        usage = response.get("usage", {})
        text = "".join(part.get("text", "") for item in response.get("output", [])
                       for part in item.get("content", []) if part.get("type") == "output_text")
        proxy = bill.get("proxy_server_request", {})
        body = proxy.get("body", {})
        headers = proxy.get("headers", {})
        if bill.get("session_id") != conversation["session_id"] or bill.get("model") != key["call_model"]:
            raise AssertionError("Codex 会话或模型归属错误")
        if response.get("id") != turn["response_id"] or response.get("status") != "completed" or conversation["marker"] not in text or response.get("error"):
            raise AssertionError("Codex 第 %d 轮响应未完成或无法记住前轮项目标记：status=%s，response_id=%s，text=%s" %
                                 (turn["turn"], response.get("status"), response.get("id"), text[:160]))
        if answer is not None and answer != response:
            raise AssertionError("Codex 持久化响应与客户端收到的响应不一致")
        if not str(header_value(headers, "user-agent") or "").startswith("codex_cli_rs/") or header_value(headers, "session_id") != conversation["session_id"] or header_value(headers, "originator") != "codex_cli_rs":
            raise AssertionError("Codex 客户端身份或稳定会话头缺失")
        request = turn.get("request_body", {})
        if (request.get("input") != [{"role": "user", "content": turn["prompt"]}]
                or request.get("previous_response_id") != turn["previous_response_id"]
                or request.get("store") is not False or request.get("stream") is not False
                or request.get("model") != key["call_model"]
                or request.get("instructions") != self.data["agent_conversation"]["instructions"]):
            raise AssertionError("Codex 请求必须只发送本轮输入并正确续接响应")
        # 网关日志记录经过历史恢复与护栏处理的请求，因此必须验证完整历史；
        # 客户端增量输入另外保存在本轮检查点，不把两种正文混为同一协议。
        inputs = body.get("input", [])
        if (not isinstance(inputs, list) or len(inputs) != 2 * turn["turn"] - 1
                or inputs[-1] != request["input"][0] or "previous_response_id" in body
                or "store" in body or body.get("model") != key["call_model"]
                or body.get("instructions") != request["instructions"]):
            raise AssertionError("Codex 持久化请求未恢复完整历史或泄漏代理字段")
        for index, prior in enumerate(conversation["turns"][:turn["turn"] - 1]):
            if (inputs[index * 2] != {"role": "user", "content": prior["prompt"]}
                    or inputs[index * 2 + 1].get("role") != "assistant"
                    or conversation["marker"] not in json.dumps(inputs[index * 2 + 1].get("content", ""))):
                raise AssertionError("Codex 持久化请求丢失前轮上下文")
        for stored, native in (("prompt_tokens", "input_tokens"), ("completion_tokens", "output_tokens")):
            if int(bill.get(stored, 0)) != int(usage.get(native, -1)) or bill.get(stored) != turn[stored]:
                raise AssertionError("Codex 逐轮 usage 与持久化账单不一致")
        if not math.isclose(bill_check(bill, key, self.data["billing"]), turn["cost"], rel_tol=0, abs_tol=1e-9):
            raise AssertionError("Codex 逐轮费用与检查点不一致")

    def verify_codex_agents(self):
        """用途：生成或严格复验 Codex 三轮数据；无参数/返回，由模型验收调用；共享入口每种路由选一把代表，三轮请求必须独立，保留全部基线和历史会话。"""
        if not self.state["keys"]:
            raise AssertionError("Codex 验收不能使用空密钥列表")
        executed = 0
        ids = set()
        keys = self.acceptance_keys()
        for index, key in enumerate(keys, 1):
            if not self.case_enabled("gpt-" + str(index)):
                continue
            with self.case("gpt-" + str(index)) as result:
                conversation = self.codex_conversation(key, revalidate=True)
                if conversation is None:
                    conversation = self.codex_conversation(key)
                    executed += 1
                else:
                    result["status"] = "revalidated"
                for turn in conversation["turns"]:
                    if turn["call_id"] in ids:
                        raise AssertionError("Codex 不同会话复用了请求 ID")
                    ids.add(turn["call_id"])
        if not ids:
            return
        self.report["acceptance_key_ids"] = [key["token_id"] for index, key in enumerate(keys, 1)
                                             if self.case_enabled("gpt-" + str(index))]
        self.report["checks"] = [row for row in self.report["checks"] if row.get("name") != "codex-agent-multi-turn"]
        self.report["checks"].append({"name": "codex-agent-multi-turn", "agent_type": "codex",
"sessions": len(self.report["acceptance_key_ids"]), "requests": len(ids),
                                      "executed_this_run": executed, "passed": True})
        self.save()

    def verify_glm_model(self):
        """用途：验收 GLM 三轮会话；无参数，返回是否严格复验旧会话，供模型阶段调用；旧基线补齐模型和白名单，复验失败才发真实请求，目录缺模型抛断言。"""
        definition = next(row for row in self.data["models"] if row["id"] == "qiniu-glm")
        if not any(row["public_name"] == definition["public_name"] for row in self.state["deployments"]):
            # 旧保留基线缺少 GLM 时只新增明确的模型和白名单，不重建租户或清理原数据。
            provider = next(row for row in self.data["providers"] if row["id"] == definition["provider"])
            catalog = self.provider_catalog(provider)
            if definition["upstream_model"] not in catalog["model_ids"]:
                raise AssertionError("七牛目录缺少指定 GLM 模型：" + definition["upstream_model"])
            self.deployment(definition, provider)
            models = list(dict.fromkeys(row["public_name"] for row in self.state["deployments"]
                                        if not row.get("temporary")))
            for team in self.state["teams"]:
                self.api("/team/update", {"team_id": team["id"], "models": models})
            for owner in self.state["keys"]:
                self.api("/key/update", {"key": owner["token_id"], "models": models})
        key = {**self.state["keys"][0], "call_model": definition["public_name"], "profile": "qiniu-only"}
        self.progress("  [模型阶段 2/6] GLM：Codex 三轮会话")
        revalidated = self.codex_conversation(key, revalidate=True) is not None
        if not revalidated:
            self.codex_conversation(key)
        self.report["checks"] = [row for row in self.report["checks"] if row.get("name") != "glm-codex-multi-turn"]
        self.report["checks"].append({"name": "glm-codex-multi-turn", "model": key["call_model"], "passed": True})
        self.save()
        return revalidated


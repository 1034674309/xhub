"""真实验收的 billing 模块；共享上下文由 Dataset 静态组合，资源与失败证据归本轮所有。"""
import json
import math
import time

from .support import temporary_retry_delay, header_value, http, bill_check


class BillingCases:
    """用途：封装 billing 场景；通过 self 访问本轮状态、管理 API 和证据，供 Dataset 调用，不自行启动全套测试。"""

    def wait_log(self, call_id, require_cost=False):
        """用途：等待请求日志详情持久化；参数为请求 ID 和是否要求价格快照，返回完整日志；媒体创建日志允许零费用，正式结算必须含价格证据。"""
        end = time.monotonic() + 90
        while time.monotonic() < end:
            status, bill, _ = http(self.gateway, "/spend/logs/ui/" + call_id, token=self.admin)
            if status == 200 and (not require_cost or bill.get("metadata", {}).get("cost_breakdown")):
                return bill
            time.sleep(0.3)
        requirement = "及价格快照" if require_cost else ""
        raise AssertionError("请求日志未持久化" + requirement + ": " + call_id)

    def wait_bill(self, call_id):
        """用途：等待可计费请求完整落库；参数为请求 ID，返回含价格快照的账单；供聊天、图片和终态结算验收使用。"""
        return self.wait_log(call_id, require_cost=True)

    def call_checkpoint(self, key):
        """用途：复验某把密钥已经落盘的真实聊天回执；参数为密钥记录，返回有效回执或空；调用方据此安全续跑，静态身份、模型、响应、usage、五级归属和价格快照任一不符都不跳过。"""
        candidates = [row for row in self.report.get("calls", []) if row.get("key_id") == key.get("token_id")]
        if len(candidates) != 1:
            return None
        row = candidates[0]
        if row.get("profile") != key.get("profile") or row.get("model") != key.get("call_model"):
            return None
        call_id, marker = row.get("call_id"), row.get("marker")
        if not call_id or not marker:
            return None
        try:
            bill, _, status = self.api("/spend/logs/ui/" + call_id, expected=(200, 404), include_status=True)
            if status != 200 or bill.get("model") != key.get("call_model"):
                return None
            cost = bill_check(bill, key, self.data["billing"])
            response, messages = bill.get("response", {}), bill.get("messages", [])
            usage = response.get("usage", {}) if isinstance(response, dict) else {}
            if not response.get("choices") or marker not in json.dumps(messages, ensure_ascii=False):
                return None
            if any(int(bill.get(field, 0)) != int(usage.get(field, -1))
                   for field in ("prompt_tokens", "completion_tokens")):
                return None
            if any(int(row.get(field, 0)) != int(bill.get(field, -1))
                   for field in ("prompt_tokens", "completion_tokens")):
                return None
            if not math.isclose(float(row.get("cost", -1)), cost, rel_tol=0, abs_tol=1e-9):
                return None
            return row
        except (AssertionError, TypeError, ValueError):
            return None

    def record_call(self, key, marker, call_id, model, bill, cost):
        """用途：原子替换一把密钥的聊天验收检查点；参数为密钥、标记、请求、模型、账单和金额，返回回执；每次真实成功核账后立即保存，后续失败不会丢失已完成项。"""
        row = {"call_id": call_id, "marker": marker, "key_id": key["token_id"],
               "profile": key["profile"], "model": model,
               "prompt_tokens": bill["prompt_tokens"],
               "completion_tokens": bill["completion_tokens"], "cost": cost}
        self.report["calls"] = [item for item in self.report.get("calls", [])
                                  if item.get("key_id") != key["token_id"]] + [row]
        self.save()
        return row

    def call(self, key, marker, extra="", model=None, expected=200, guardrails=None, defer_bill=False,
             attempts=5, retry_delay=3, rate_limit_delay=65):
        """用途：个人密钥真实请求对账；参数含标记/模型/状态/护栏/延迟对账及临时错误重试策略，返回正文或四元回执；429 先等待供应商窗口恢复，其他临时错误短退避，失败不计成功证据。"""
        model = model or key.get("call_model") or self.state["chat_models"]["fennoai"]
        body = {"model": model, "messages": [{"role": "user", "content": "Reply only OK. " + marker + " " + extra}], "max_tokens": 32}
        if guardrails is not None:
            body["guardrails"] = guardrails
        if expected == 200:
            retryable = (429, 502, 503, 504)
            for attempt in range(1, attempts + 1):
                self.action_start(model, "bypass_openai_chat", f"第 {attempt}/{attempts} 次真实 AI 请求",
                                  f"profile={key.get('profile', 'temporary')}，预期 HTTP=200")
                answer, headers, status = self.api("/v1/chat/completions", body, token=key["key"],
                                                    expected=(200, *retryable), include_status=True)
                if status == 200:
                    break
                error = answer.get("error", answer) if isinstance(answer, dict) else {}
                summary = (str(error.get("message") or error.get("code") or "无错误正文")
                           if isinstance(error, dict) else str(error))[:300]
                self.action_fail(model, "bypass_openai_chat", f"第 {attempt}/{attempts} 次真实 AI 请求",
                                 RuntimeError(f"HTTP={status}，上游临时错误={summary}"))
                if attempt < attempts:
                    wait_seconds = temporary_retry_delay(summary, retry_delay, rate_limit_delay, attempt)
                    self.action_wait(model, "bypass_openai_chat", "真实 AI 请求重试",
                                     f"{wait_seconds} 秒后进行第 {attempt + 1}/{attempts} 次")
                    time.sleep(wait_seconds)
            else:
                raise AssertionError(f"{model} 连续 {attempts} 次真实请求均遇到临时上游错误")
        else:
            self.action_start(model, "bypass_openai_chat", "真实 AI 请求",
                              f"profile={key.get('profile', 'temporary')}，预期 HTTP={expected}")
            answer, headers = self.api("/v1/chat/completions", body, token=key["key"], expected=expected)
        if expected == 200:
            call_id = next((v for k, v in headers.items() if k.lower() == "x-litellm-call-id"), None)
            if not call_id or not answer.get("choices"):
                raise AssertionError("真实响应缺少 request_id 或 choices")
            usage = answer.get("usage", {})
            if int(usage.get("prompt_tokens", 0)) <= 0 or int(usage.get("completion_tokens", 0)) <= 0:
                raise AssertionError("真实响应必须包含非零输入和输出 usage")
            if self.observer:
                observed = [r for r in self.observer.rows if marker in r["messages"] and r["status"] == 200]
                if observed and observed[-1].get("usage") and any(observed[-1]["usage"].get(k) != answer.get("usage", {}).get(k)
                        for k in ("prompt_tokens", "completion_tokens")):
                    raise AssertionError("网关响应计量与真实供应商不一致")
            if defer_bill:
                self.action_ok(model, "bypass_openai_chat", "真实 AI 响应",
                               f"输入={answer.get('usage', {}).get('prompt_tokens', 0)}，输出={answer.get('usage', {}).get('completion_tokens', 0)}，等待批量计量核对", record=False)
                return (key, marker, call_id, answer.get("usage", {}))
            bill = self.wait_bill(call_id)
            if any(bill.get(k) != answer.get("usage", {}).get(k) for k in ("prompt_tokens", "completion_tokens")):
                raise AssertionError("账单计量与真实响应用量不一致")
            cost = bill_check(bill, key, self.data["billing"])
            self.record_call(key, marker, call_id, model, bill, cost)
            self.action_ok(model, "bypass_openai_chat", "请求日志、计量与计费",
                           f"输入={bill['prompt_tokens']}，输出={bill['completion_tokens']}，金额={cost:.9f}")
        else:
            self.action_ok(model, "bypass_openai_chat", "本地拒绝且无上游调用", f"HTTP={expected}")
        return answer

    def chat_request(self, key, body, expected=200):
        """用途：发送正文完全固定的聊天请求；参数为密钥、请求正文和期望状态，返回正文、响应头、状态和可空请求 ID；成功响应必须具有 choices 与非零 usage，供缓存重放、权重和回退链使用。"""
        answer, headers, status = self.api("/v1/chat/completions", body, token=key["key"],
                                            expected=expected, include_status=True)
        call_id = header_value(headers, "x-litellm-call-id")
        if status == 200:
            usage = answer.get("usage", {}) if isinstance(answer, dict) else {}
            if not call_id or not answer.get("choices"):
                raise AssertionError("真实响应缺少 request_id 或 choices")
            if int(usage.get("prompt_tokens", 0)) <= 0 or int(usage.get("completion_tokens", 0)) <= 0:
                raise AssertionError("真实响应必须包含非零输入和输出 usage")
        return answer, headers, status, call_id

    def refuse(self, key, marker, expected, needle=None, model=None):
        """用途：验证本地拒绝无外发；参数为密钥、标记、状态及错误关键词，返回错误；用于额度/撤销/下架，不产生收费调用。"""
        before = len(self.observer.rows) if self.observer else 0
        answer = self.call(key, marker, model=model, expected=expected)
        if needle and needle not in json.dumps(answer, ensure_ascii=False):
            raise AssertionError("拒绝响应未指明原因: " + needle)
        if self.observer and len(self.observer.rows) != before:
            raise AssertionError("本地拒绝发生上游外发: " + marker)
        return answer


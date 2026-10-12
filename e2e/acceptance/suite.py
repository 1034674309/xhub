"""真实验收的 suite 模块；共享上下文由 Dataset 静态组合，资源与失败证据归本轮所有。"""
import json



class SuiteCases:
    """用途：封装 suite 场景；通过 self 访问本轮状态、管理 API 和证据，供 Dataset 调用，不自行启动全套测试。"""

    def verify_acceptance_models(self):
        """用途：执行有序模型验收；无参数/返回；GPT 按路由代表、GLM 三轮及四类媒体各测一次，严格复验检查点，前阶段失败停止后续付费任务。"""
        if any(self.case_enabled("gpt-" + str(index)) for index in range(1, len(self.acceptance_keys()) + 1)):
            self.progress("  [模型阶段 1/6] GPT-5.6-sol：Codex 三轮会话")
        self.verify_codex_agents()
        if self.case_enabled("glm"):
            with self.case("glm") as result:
                if self.verify_glm_model():
                    result["status"] = "revalidated"
        if any(self.case_enabled(category) for category in ("image", "ark-video", "fal-video", "kling-video")):
            self.progress("  [模型阶段 3–6/6] 图片 → Ark Seedance → FAL Seedance → Kling")
        self.verify_media_models_parallel(self.state["keys"][0]) if getattr(self, "media_jobs", 1) > 1 else self.verify_media_models(self.state["keys"][0])

    def verify(self):
        """用途：验收实体基线与不同业务类型；无参数/返回，由 runner 调用；共享模式按路由去重，逐项断言和保存，失败中止，历史成功不能代替本轮业务执行。"""
        if self.case_enabled("entities"):
            with self.case("entities"):
                counts = {name: len(self.state[name]) for name in ("organizations", "teams", "users", "projects", "keys")}
                if list(counts.values()) != [3, 9, 27, 9, 81]:
                    raise AssertionError("实体数量错误")
                for team in self.state["teams"]:
                    members, _ = self.api("/team/member_list?team_id=" + team["id"])
                    if len(members["members"]) != 3:
                        raise AssertionError("团队必须恰好包含三名成员")
                self.report["counts"] = counts
        self.verify_acceptance_models()
        keys = self.acceptance_keys()
        if self.case_enabled("inheritance"):
            with self.case("inheritance"):
                for key in keys:
                    binding, _ = self.api("/route_template/binding?scope=key&scope_id=" + key["token_id"])
                    expected = "key" if key["profile"] != "inherit" else ("organization" if key["team_index"] == 0 else "team")
                    if binding["effective"]["scope_type"] != expected:
                        raise AssertionError("路由继承层级错误: " + expected)
                    self.progress("    [继承通过] " + key["profile"] + " / " + expected)
        if self.case_enabled("chat-bills"):
            with self.case("chat-bills") as result:
                executed = 0
                for index, key in enumerate(keys, 1):
                    self.progress(f"    [Chat {index}/{len(keys)}] {key['profile']}：真实响应、usage、五级归属与价格快照")
                    if self.call_checkpoint(key):
                        self.progress("    [Chat 检查点已复验] 持久化请求及账单一致")
                        continue
                    self.report["calls"] = [row for row in self.report.get("calls", []) if row.get("key_id") != key["token_id"]]
                    self.call(key, f"dataset-{self.run}-{index - 1}")
                    executed += 1
                    self.save()
                evidence = [row for row in self.report.get("calls", []) if row.get("key_id") in {key["token_id"] for key in keys}]
                if len(evidence) != len(keys) or len({row.get("key_id") for row in evidence}) != len(keys):
                    raise AssertionError("代表密钥的唯一真实调用证据不完整")
                name = "representative-keys-real-calls-and-billing" if self.representative else "81-personal-keys-real-calls-and-billing"
                self.report["checks"] = [row for row in self.report["checks"] if row.get("name") != name]
                self.report["checks"].append(dict(name=name, keys=len(keys), executed_this_run=executed, resumed=len(keys)-executed, passed=True))
                if executed == 0:
                    result["status"] = "revalidated"
                self.save()
        key = keys[0]
        self.run_case("permissions", self.verify_permissions)
        self.run_case("xgo", self.verify_xgo, key)
        self.run_case("limits", self.verify_limits, key)
        self.run_case("model-lifecycle", self.verify_model_lifecycle, key)
        self.run_case("guard-lifecycle", self.verify_guardrail_lifecycle, key)
        if self.case_enabled("global-guards"):
            with self.case("global-guards"):
                before = len(self.observer.rows) if self.observer else None
                blocked = self.call(key, "block-" + self.run, "验收禁止外发", expected=400)
                if "guardrail_failed" not in json.dumps(blocked) or (self.observer and len(self.observer.rows) != before):
                    raise AssertionError("护栏拦截必须在上游调用前发生")
                self.report["checks"].append({"name": "global-guardrail-block-no-egress", "passed": True})
                marker = "redact-" + self.run
                self.call(key, marker, "电话13800138000 邮箱qa@example.invalid")
                if self.observer:
                    rows = [r for r in self.observer.rows if marker in r["messages"]]
                    if not rows or any("13800138000" in r["messages"] or "qa@example.invalid" in r["messages"] for r in rows):
                        raise AssertionError("个人信息在真实供应商转发前没有脱敏")
                    self.report["checks"].append({"name": "redaction-before-real-supplier", "passed": True})
        if self.observer:
            self.run_case("profile-isolation", self.verify_profile_route_isolation)
            self.run_case("cache-weight", self.verify_cache_weight_chain, key)
            self.run_case("fallback", self.verify_fallback_policy_chain, key)
        self.report["status"] = "passed"
        self.save()


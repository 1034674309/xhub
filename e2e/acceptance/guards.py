"""真实验收的 guards 模块；共享上下文由 Dataset 静态组合，资源与失败证据归本轮所有。"""



class GuardsCases:
    """用途：封装 guards 场景；通过 self 访问本轮状态、管理 API 和证据，供 Dataset 调用，不自行启动全套测试。"""

    def verify_xgo(self, key):
        """用途：验收清单XGo源码及真实执行；参数为个人密钥，返回无；验证源码往返、调试、无外发拦截、真实脱敏和审计放行。"""
        guards, _ = self.api("/guardrails/list")
        for definition in self.data["guardrails"]:
            scenario = definition.get("acceptance")
            if not scenario:
                continue
            name = definition["guardrail_name"]
            self.progress(f"    [XGo] 护栏={name} 动作={scenario['action']} 开始")
            stored = next(g for g in guards["guardrails"] if g["guardrail_name"] == name)
            detail, _ = self.api("/guardrails/" + stored["guardrail_id"])
            if detail["litellm_params"]["custom_code"] != definition["litellm_params"]["custom_code"]:
                raise AssertionError("XGo源码持久化不一致")
            trial, _ = self.api("/guardrails/apply_guardrail", {"guardrail_name": name, "text": scenario["text"]})
            if trial.get("action") != scenario["action"]:
                raise AssertionError(name + " 调试动作不一致")
            if scenario["action"] == "modify":
                trial_text = trial.get("response_text", trial.get("text", ""))
                if scenario["text"] in trial_text or scenario["replacement"] not in trial_text:
                    raise AssertionError(name + " 调试响应没有返回脱敏后的正文")
            marker = "xgo-" + self.run + "-" + scenario["action"]
            before = len(self.observer.rows) if self.observer else 0
            self.call(key, marker, scenario["text"], expected=scenario["expected_http_status"], guardrails=[name])
            rows = self.observed_since(before, marker)
            if self.observer and scenario["action"] == "block" and rows:
                raise AssertionError("XGo拦截发生了上游外发")
            if self.observer and scenario["action"] == "modify" and (not rows or any(
                    scenario["text"] in r["messages"] or scenario["replacement"] not in r["messages"] for r in rows)):
                original = sum(scenario["text"] in row.get("messages", "") for row in rows)
                replaced = sum(scenario["replacement"] in row.get("messages", "") for row in rows)
                raise AssertionError(
                    f"XGo脱敏没有作用于真实上游正文: 当前请求观察记录={len(rows)}，"
                    f"仍含原文={original}，含替换文本={replaced}")
            self.report["checks"].append({"name": name + "-persist-debug-real-request", "passed": True})
            result = ("本地阻断且上游增量=0" if scenario["action"] == "block" else
                      f"真实上游正文已替换为 {scenario['replacement']}" if scenario["action"] == "modify" else
                      "审计标记后真实上游继续响应")
            self.action_ok(key["call_model"], "xgo_guardrail", name + " / " + scenario["action"],
                           "源码持久化、调试响应和真实请求一致；" + result, record=False)

    def verify_guardrail_lifecycle(self, key):
        """用途：临时XGo编辑启用删除验收；参数为个人密钥，返回无；调试和真实脱敏均验证，finally删除临时规则。"""
        if self.checked("xgo-edit-default-enable-delete"):
            return
        definition = next(g for g in self.data["guardrails"] if g.get("acceptance", {}).get("action") == "modify")
        self.progress("    [护栏生命周期] 创建临时 XGo、编辑源码、默认启用、真实脱敏并删除")
        guard, _ = self.api("/guardrails", {"guardrail_name": "临时-XGo-编辑与默认启用",
            "litellm_params": {**definition["litellm_params"], "default_on": False}})
        ident = guard["guardrail_id"]
        try:
            source = definition["litellm_params"]["custom_code"].replace("[工单已隐藏]", "[编辑后已隐藏]")
            self.api("/guardrails/" + ident, {"litellm_params": {"custom_code": source, "default_on": True}}, method="PATCH")
            stored, _ = self.api("/guardrails/" + ident)
            if stored["litellm_params"]["custom_code"] != source or not stored["litellm_params"]["default_on"]:
                raise AssertionError("XGo编辑或默认启用未持久化")
            marker = "xgo-edited-" + self.run
            self.call(key, marker, "XGO_PRIVATE")
            if self.observer:
                rows = [r for r in self.observer.rows if marker in r["messages"]]
                if not rows or any("XGO_PRIVATE" in r["messages"] or "[编辑后已隐藏]" not in r["messages"] for r in rows):
                    raise AssertionError("编辑启用后的XGo未改写真实外发正文")
        finally:
            self.api("/guardrails/" + ident, method="DELETE")
        self.api("/guardrails/" + ident, expected=404)
        self.report["checks"].append({"name": "xgo-edit-default-enable-delete", "passed": True})
        self.save()


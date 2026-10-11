"""真实验收的 routing 模块；共享上下文由 Dataset 静态组合，资源与失败证据归本轮所有。"""
import json

from .support import fallback_acceptance_deployments, fallback_acceptance_route, weighted_acceptance_route, response_cache_hit, cache_bill_check, bill_check


class RoutingCases:
    """用途：封装 routing 场景；通过 self 访问本轮状态、管理 API 和证据，供 Dataset 调用，不自行启动全套测试。"""

    def observed_since(self, start, marker):
        """用途：读取某一步之后属于固定请求标记的观察记录；参数为起始下标和标记，返回匹配行副本；供断言缓存未外发、权重命中与回退顺序。"""
        if not self.observer:
            return []
        with self.observer.lock:
            return [dict(row) for row in self.observer.rows[start:] if marker in row.get("messages", "")]

    def verify_cache_weight_chain(self, key):
        """用途：组合验证固定请求缓存、跨密钥隔离、封禁权限和 100:0 到 0:100 权重切换；参数为基线密钥，返回无；创建两部署、一路由和两把临时密钥，finally 全部删除。"""
        model = self.state["chat_models"]["fennoai"]
        protocol = "bypass_openai_chat"
        provider = self.state["selected"]["fennoai"]["provider"]
        definition = next(row for row in self.data["models"] if row["id"] == "fenno-chat")
        weights = self.data["logic_chains"]["weighted_route_switch"]
        timeout = self.data["routing"]["timeout_seconds"]
        deployment_ids, route_id, temporary_keys = [], None, []
        try:
            first_id = self.deployment(definition, provider, model, temporary=True, route_tag="weight-a")
            second_id = self.deployment(definition, provider, model, temporary=True, route_tag="weight-b")
            deployment_ids.extend((first_id, second_id))
            initial = weighted_acceptance_route(model, first_id, second_id,
                                                weights["initial_weights"], timeout)
            route, _ = self.api("/route_template/new", {
                "name": "real-acceptance-cache-weight-" + self.run, "body": initial})
            route_id = route["id"]
            for suffix in ("primary", "isolation"):
                created, _ = self.api("/key/generate", {"key_alias": "real-cache-weight-" + suffix,
                    "owner_type": "personal", "user_id": key["user_id"], "team_id": key["team_id"],
                    "project_id": key["project_id"], "models": [model], "route_template_id": route_id})
                temporary_keys.append({**key, **created, "profile": "logic-chain", "call_model": model})
            primary, isolated = temporary_keys
            marker = "cache-weight-" + self.run
            body = {"model": model, "messages": [{"role": "user",
                    "content": "Reply only OK. " + marker}], "max_tokens": 32}

            before = len(self.observer.rows)
            self.action_start(model, protocol, "缓存首次未命中与权重 100:0",
                              "固定正文首次请求，要求只调用 weight-a 的真实上游")
            first, first_headers, _, first_call = self.chat_request(primary, body)
            rows = self.observed_since(before, marker)
            if (response_cache_hit(first_headers) or len(rows) != 1
                    or rows[0].get("route_tag") != "weight-a" or rows[0].get("status") != 200):
                raise AssertionError("首次请求没有按 100:0 只进入 weight-a 真实部署")
            first_bill = self.wait_bill(first_call)
            first_cost = bill_check(first_bill, primary, self.data["billing"])
            self.action_ok(model, protocol, "缓存首次未命中与权重 100:0",
                           f"cache_hit=false，weight-a 上游=1，weight-b 上游=0，金额={first_cost:.9f}")

            before = len(self.observer.rows)
            second, second_headers, _, second_call = self.chat_request(primary, body)
            if not response_cache_hit(second_headers) or self.observed_since(before, marker):
                raise AssertionError("相同请求没有由缓存回答或仍发生真实上游调用")
            if second != first or second_call == first_call:
                raise AssertionError("缓存响应不一致或缓存请求没有独立 call ID")
            second_bill = self.wait_bill(second_call)
            cache_bill_check(second_bill, primary, first["usage"])
            self.action_ok(model, protocol, "相同请求缓存命中与响应一致",
                           "响应 JSON 与首次完全一致，上游增量=0，独立请求日志 cache_hit=true，spend=0，usage 一致")

            before = len(self.observer.rows)
            _, isolated_headers, _, isolated_call = self.chat_request(isolated, body)
            rows = self.observed_since(before, marker)
            if response_cache_hit(isolated_headers) or len(rows) != 1 or rows[0].get("route_tag") != "weight-a":
                raise AssertionError("另一把密钥错误复用了第一把密钥的缓存")
            isolated_cost = bill_check(self.wait_bill(isolated_call), isolated, self.data["billing"])
            self.action_ok(model, protocol, "跨密钥缓存隔离",
                           f"第二把密钥 cache_hit=false，weight-a 真实上游=1，独立计费={isolated_cost:.9f}")

            self.api("/key/block", {"key": primary["key"]})
            before = len(self.observer.rows)
            _, blocked_headers, blocked_status, _ = self.chat_request(primary, body, expected=401)
            if response_cache_hit(blocked_headers) or self.observed_since(before, marker):
                raise AssertionError("封禁密钥读取了缓存或发生上游调用")
            self.action_ok(model, protocol, "封禁权限先于缓存读取",
                           f"HTTP={blocked_status}，cache_hit=false，上游增量=0")
            self.api("/key/unblock", {"key": primary["key"]})

            updated = weighted_acceptance_route(model, first_id, second_id,
                                                weights["updated_weights"], timeout)
            self.api("/route_template/" + route_id + "/update", {"body": updated})
            before = len(self.observer.rows)
            switched, switched_headers, _, switched_call = self.chat_request(primary, body)
            rows = self.observed_since(before, marker)
            if response_cache_hit(switched_headers) or len(rows) != 1 or rows[0].get("route_tag") != "weight-b":
                raise AssertionError("路由更新未使旧缓存失效并切换到 weight-b")
            switched_cost = bill_check(self.wait_bill(switched_call), primary, self.data["billing"])
            self.action_ok(model, protocol, "路由更新使缓存失效并切换权重 0:100",
                           f"cache_hit=false，weight-a 上游=0，weight-b 上游=1，金额={switched_cost:.9f}")

            before = len(self.observer.rows)
            repeated, repeated_headers, _, repeated_call = self.chat_request(primary, body)
            if repeated != switched or not response_cache_hit(repeated_headers) or self.observed_since(before, marker):
                raise AssertionError("切换后相同请求没有命中新路由缓存")
            cache_bill_check(self.wait_bill(repeated_call), primary, switched["usage"])
            self.action_ok(model, protocol, "新权重缓存命中与零权重排除",
                           "响应一致，weight-a 上游=0，weight-b 上游增量=0，cache_hit=true，spend=0")
        finally:
            for subject in temporary_keys:
                self.api("/key/delete", {"keys": [subject["key"]]})
            if route_id:
                self.api("/route_template/" + route_id + "/delete", {})
            for deployment_id in deployment_ids:
                self.api("/model/delete", {"id": deployment_id})
            if deployment_ids:
                self.state["deployments"] = [row for row in self.state["deployments"]
                                               if row["id"] not in deployment_ids]
                self.save()

    def verify_fallback_policy_chain(self, key):
        """用途：组合验证 429 跨真实模型回退、重复请求禁用缓存、回退目标白名单复核及 disable_fallbacks；参数为基线密钥，返回无；临时故障部署、模板和密钥均 finally 删除。"""
        model = self.state["chat_models"]["fennoai"]
        # chat_models 记录两家供应商共用的公开模型，不能作为跨模型回退目标；
        # 使用清单中独立的七牛 GLM 模型，并把主模型两家的健康部署全部排除。
        fallback_definition = next(row for row in self.data["models"] if row["id"] == "qiniu-glm")
        fallback_model = fallback_definition["public_name"]
        protocol = "bypass_openai_chat"
        provider = self.state["selected"]["fennoai"]["provider"]
        definition = next(row for row in self.data["models"] if row["id"] == "fenno-chat")
        healthy_ids, fallback_id = fallback_acceptance_deployments(
            self.state["deployments"], model, fallback_model, fallback_definition["provider"])
        fault_id = self.deployment(definition, provider, model, fault=True, temporary=True)
        route_id, temporary_keys = None, []
        try:
            route, _ = self.api("/route_template/new", {"name": "real-acceptance-fallback-" + self.run,
                "body": fallback_acceptance_route(model, fault_id, healthy_ids, fallback_model, fallback_id,
                                                    self.data["routing"]["timeout_seconds"])})
            route_id = route["id"]
            for suffix, models in (("allowed", [model, fallback_model]), ("primary-only", [model])):
                created, _ = self.api("/key/generate", {"key_alias": "real-fallback-" + suffix,
                    "owner_type": "personal", "user_id": key["user_id"], "team_id": key["team_id"],
                    "project_id": key["project_id"], "models": models, "route_template_id": route_id})
                temporary_keys.append({**key, **created, "profile": "logic-chain", "call_model": model})
            allowed, primary_only = temporary_keys
            marker = "fallback-repeat-" + self.run
            body = {"model": model, "messages": [{"role": "user",
                    "content": "Reply only OK. " + marker}], "max_tokens": 32}
            repeat_count = self.data["logic_chains"]["fallback_cache"]["repeat_requests"]
            for repeat in range(1, repeat_count + 1):
                before = len(self.observer.rows)
                answer, headers, _, call_id = self.chat_request(allowed, body)
                rows = self.observed_since(before, marker)
                route = [(row.get("provider"), row.get("route_tag"), row.get("status")) for row in rows]
                if response_cache_hit(headers) or route != [("fennoai", "fault", 429), ("qiniu", "default", 200)]:
                    raise AssertionError("回退请求没有按 Fenno 429 到七牛真实 200 执行，或错误命中缓存")
                cost = bill_check(self.wait_bill(call_id), allowed, self.data["billing"])
                self.action_ok(model + " → " + fallback_model, protocol,
                               f"第 {repeat}/{repeat_count} 次 429 跨真实模型回退",
                               f"Fenno fault=429，Fenno 健康零权重=0 次，Qiniu 真实=200，cache_hit=false，金额={cost:.9f}，usage={answer['usage']}")
            self.action_ok(model + " → " + fallback_model, protocol, "配置回退禁用响应缓存",
                           f"完全相同正文执行={repeat_count} 次，每次均重新发生 fault 429 和真实回退 200")

            denied_marker = "fallback-denied-" + self.run
            denied_body = {"model": model, "messages": [{"role": "user",
                "content": "Reply only OK. " + denied_marker}], "max_tokens": 32}
            before = len(self.observer.rows)
            denied, denied_headers, denied_status, _ = self.chat_request(primary_only, denied_body, expected=401)
            rows = self.observed_since(before, denied_marker)
            if response_cache_hit(denied_headers) or [(r.get("provider"), r.get("route_tag"), r.get("status")) for r in rows] != [("fennoai", "fault", 429)]:
                raise AssertionError("回退目标白名单拒绝前发生了备用模型外发")
            if "model not in allowed model list" not in json.dumps(denied, ensure_ascii=False):
                raise AssertionError("回退目标白名单拒绝没有返回明确原因")
            self.action_ok(fallback_model, protocol, "回退目标重新执行模型白名单",
                           f"主模型故障后 HTTP={denied_status}，Qiniu 上游增量=0，拒绝原因已核对")

            disabled_marker = "fallback-disabled-" + self.run
            disabled_body = {"model": model, "messages": [{"role": "user",
                "content": "Reply only OK. " + disabled_marker}], "max_tokens": 32,
                "disable_fallbacks": True}
            before = len(self.observer.rows)
            _, disabled_headers, disabled_status, _ = self.chat_request(allowed, disabled_body, expected=502)
            rows = self.observed_since(before, disabled_marker)
            if response_cache_hit(disabled_headers) or [(r.get("provider"), r.get("route_tag"), r.get("status")) for r in rows] != [("fennoai", "fault", 429)]:
                raise AssertionError("disable_fallbacks=true 后仍执行了备用模型")
            self.action_ok(model, protocol, "disable_fallbacks 停止回退链",
                           f"HTTP={disabled_status}，仅 Fenno fault=429，Qiniu 上游增量=0，cache_hit=false")
        finally:
            for subject in temporary_keys:
                self.api("/key/delete", {"keys": [subject["key"]]})
            if route_id:
                self.api("/route_template/" + route_id + "/delete", {})
            self.api("/model/delete", {"id": fault_id})
            self.state["deployments"] = [row for row in self.state["deployments"] if row["id"] != fault_id]
            self.save()

    def verify_profile_route_isolation(self):
        """用途：用当前运行的新请求验收两个单供应商密钥配置；无参数/返回；分别调用真实英文模型 ID，并要求观察器中只有声明的供应商收到该标记，证明不同上游模型没有再被合并到业务别名。"""
        if self.checked("profile-routes-real-models-and-vendors"):
            return
        for profile, provider in (("fenno-only", "fennoai"), ("qiniu-only", "qiniu")):
            key = next(row for row in self.state["keys"] if row["profile"] == profile)
            marker = f"route-isolation-{profile}-{self.run}"
            self.call(key, marker, model=key["call_model"])
            rows = [row for row in self.observer.rows if marker in row["messages"]]
            successes = [row for row in rows if row["status"] == 200]
            if not successes or any(row["provider"] != provider for row in rows):
                raise AssertionError(f"{profile} 没有隔离到真实模型和供应商 {provider}")
            self.action_ok(key["call_model"], "bypass_openai_chat", "真实模型路由隔离",
                           f"profile={profile}，仅供应商={provider} 收到请求，响应包含非零 usage")
        self.report["checks"].append({"name": "profile-routes-real-models-and-vendors", "passed": True})
        self.save()


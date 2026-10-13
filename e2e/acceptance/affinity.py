"""现有真实网关的会话粘性验收；使用本轮独立部署、模板和密钥，不注入供应商故障。"""
from concurrent.futures import ThreadPoolExecutor

from .support import bill_check, header_value, response_cache_hit, weighted_acceptance_route


class AffinityCases:
    """供 Dataset 组合的完整会话、模板和端点场景；共享用户基线只读，临时管理资源最终清理。"""

    def affinity_request(self, subject, body, path="/v1/chat/completions", request_headers=None, expected=200):
        """用途：发送一轮独立真实请求；参数为密钥、正文、端点、头和预期状态，返回正文、头与请求 ID。

        供粘性场景与并发工作线程调用；成功必须有非空回答、正 usage 和独立请求 ID，缓存不能代替真实外发。
        不保存报告，线程仅发送请求；网络或协议错误直接失败，由外层清理资源。
        """
        answer, headers, status = self.api(path, body, token=subject["key"], request_headers=request_headers,
                                          expected=expected, include_status=True)
        call_id = header_value(headers, "x-litellm-call-id")
        if status == 200:
            usage = answer.get("usage", {})
            choices = answer.get("choices", [])
            if not call_id or not choices or not choices[0].get("message", {}).get("content"):
                raise AssertionError("Affinity 真实响应缺少请求 ID 或非空回答")
            if any(int(usage.get(field, 0)) <= 0 for field in ("prompt_tokens", "completion_tokens")):
                raise AssertionError("Affinity 真实响应缺少正输入/输出用量")
            if response_cache_hit(headers):
                raise AssertionError("Affinity 不同轮正文不能由响应缓存替代真实调用")
        return answer, headers, call_id

    def affinity_receipt(self, subject, response, session, source, definition, label):
        """用途：核对一轮会话、供应商、上游型号、外发路径及五级费用；参数为回执和预期身份，返回无。

        场景主线程调用，按独立请求 ID 等待持久化；来源、身份或路由不符失败；保存脱敏证据，不记录密钥。
        """
        _, headers, call_id = response
        if header_value(headers, "x-session-id") != session:
            raise AssertionError(label + "：响应会话 ID 未保持")
        if call_id in {row["call_id"] for row in self.report.get("affinity_evidence", [])}:
            raise AssertionError(label + "：请求 ID 重复")
        bill = self.wait_bill(call_id)
        for field, value in (("session_id", session), ("session_source", source), ("model", subject["call_model"])):
            if bill.get(field) != value:
                raise AssertionError(f"{label}：日志 {field}={bill.get(field)!r}，预期={value!r}")
        cost = bill_check(bill, subject, self.data["billing"])
        route = self.route_evidence(call_id, definition["provider"], definition.get("stored_model", definition["upstream_model"]))
        self.report.setdefault("affinity_evidence", []).append({"label": label, "call_id": call_id,
            "session_id": session, "session_source": source, "model": subject["call_model"],
            "supplier": route["supplier"], "upstream_model": route["upstream_model"],
            "outbound_endpoint": route["outbound_endpoint"], "spend": cost})
        self.save()
        self.action_ok(subject["call_model"], "bypass_openai_chat", label,
                       f"session={session}，供应商={route['supplier']}，独立真实请求，五级账单已核对")

    def verify_affinity_chain(self, key):
        """用途：验收会话粘性与模板、模型、端点当前资格的联动；参数为基线归属密钥，无返回。

        创建同名 Fenno/七牛部署、两个模板和两把密钥；先用单目标建立可控偏好，再恢复双目标检验保持。
        覆盖多轮、密钥/会话/端点/模型隔离、并发、模板更新与重新绑定、能力/停用/删除失效、推断和非法身份。
        不修改已有模板、默认权重或供应商；不制造上游故障，跨模型故障回退由 fallback 分类独立验收。
        每轮不同英文正文排除响应缓存；供应商唯一映射本轮候选部署，避免把品牌当作部署身份。finally 清理本轮资源。
        """
        model = self.state["chat_models"]["fennoai"]
        definitions = [dict(next(row for row in self.data["models"] if row["id"] == ident))
                       for ident in ("fenno-chat", "qiniu-chat")]
        for row in definitions:
            row["endpoint_types"] = ["chat", "bypass:openai-chat"]
        deployments, templates, subjects = [], [], []
        deleted = set()
        sequence = 0
        session = "affinity-" + self.run
        try:
            for definition in definitions:
                deployments.append(self.deployment(definition, self.state["selected"][definition["provider"]]["provider"],
                                                   public_name=model, temporary=True))
            excluded = [r["id"] for r in self.state["deployments"] if r["public_name"] == model and r["id"] not in deployments]

            def template_body(weights):
                """构造本轮完整模板；参数为两项权重，返回禁用回退且排除基线部署的正文，无数据写入。"""
                return weighted_acceptance_route(model, *deployments, weights, self.data["routing"]["timeout_seconds"], excluded)

            for weights in ([100, 0], [0, 100]):
                row, _ = self.api("/route_template/new", {"name": session + str(len(templates)), "body": template_body(weights)})
                templates.append(row["id"])
            for index in range(2):
                created, _ = self.api("/key/generate", {"key_alias": session + str(index), "owner_type": "personal",
                    "user_id": key["user_id"], "team_id": key["team_id"], "project_id": key["project_id"],
                    "models": [model], "route_template_id": templates[index]})
                subjects.append({**key, **created, "call_model": model})
            primary, isolated = subjects

            def weights(values):
                """更新主密钥模板的两部署分配；参数为有限权重，无返回；只修改本轮模板。"""
                self.api("/route_template/" + templates[0] + "/update", {"body": template_body(values)})

            def request(subject=primary, ident=session, path="/v1/chat/completions", messages=None, explicit=True):
                """生成不同英文正文并发送一轮；参数控制密钥、会话、入口和历史，返回回执；递增局部序号以排除缓存。"""
                nonlocal sequence
                sequence += 1
                body = {"model": subject["call_model"], "messages": messages or [{"role": "user", "content": f"Reply only OK. {session} round {sequence}"}], "max_tokens": 32}
                if explicit:
                    body["session_id"] = ident
                return self.affinity_request(subject, body, path)

            def check(response, index, label, subject=primary, ident=session, source="explicit"):
                """核对指定供应商部署的回执；参数为响应和期望身份，无返回；调用统一账单与持久化路由核对。"""
                self.affinity_receipt(subject, response, ident, source, definitions[index], label)

            check(request(), 0, "单目标模板建立 Fenno 偏好")
            weights([1, 1])
            for turn in range(3):
                check(request(), 0, f"双目标下同会话第 {turn + 1} 轮保持原部署")
            check(request(isolated), 1, "同会话不同密钥按独立模板选择七牛", subject=isolated)
            weights([0, 100])
            alternate = session + "-other"
            check(request(ident=alternate), 1, "另一会话建立七牛偏好", ident=alternate)
            native = "/bypass/openai/v1/chat/completions"
            check(request(path=native), 1, "同会话原生端点建立独立七牛偏好")
            weights([1, 1])
            check(request(), 0, "统一端点保留 Fenno，未被原生端点覆盖")
            check(request(path=native), 1, "原生端点保留七牛，未被统一端点覆盖")
            check(request(ident=alternate), 1, "另一会话偏好独立保持", ident=alternate)
            # 配置恢复双目标后，三个同时发出的首次请求必须协调为同一部署；不要求上游执行串行。
            concurrent = session + "-concurrent"
            bodies = [{"model": model, "session_id": concurrent, "messages": [{"role": "user", "content": f"Reply only OK. {concurrent} round {i}"}], "max_tokens": 32} for i in range(3)]
            with ThreadPoolExecutor(max_workers=3) as pool:
                futures = [pool.submit(self.affinity_request, primary, body) for body in bodies]
                responses = [future.result() for future in futures]
            # 实际赢家由持久化供应商与上游型号确认；外发路径可能相同，不能仅凭路径识别部署。
            first_bill = self.wait_bill(responses[0][2])
            endpoint = first_bill["outbound_endpoint"]
            winner = None
            for index, definition in enumerate(definitions):
                try:
                    self.route_evidence(responses[0][2], definition["provider"], definition.get("stored_model", definition["upstream_model"]))
                    winner = index
                    break
                except AssertionError as error:
                    if "真实路由与预期不符" not in str(error):
                        raise
            if winner is None:
                raise AssertionError("并发首次请求未进入任一本轮真实候选")
            for turn, response in enumerate(responses):
                check(response, winner, f"并发首次请求 {turn + 1} 共享赢家", ident=concurrent)
                if self.wait_bill(response[2])["outbound_endpoint"] != endpoint:
                    raise AssertionError("并发同会话外发端点不一致")
            weights([0, 100])
            check(request(), 1, "模板归零使旧 Fenno 偏好即时失效")
            weights([1, 1])
            check(request(), 1, "恢复 Fenno 正权重不抢走合法七牛偏好")
            # 只改变临时七牛部署当前资格，必须切换并建立新的合法偏好。
            self.api("/model/update", {"id": deployments[1], "model_info": {"endpoint_types": ["responses"]}})
            check(request(path=native), 0, "移除原生聊天端点后重新选择兼容部署")
            self.api("/model/update", {"id": deployments[1], "model_info": {"endpoint_types": definitions[1]["endpoint_types"]}})
            check(request(path=native), 0, "恢复原生聊天端点不覆盖当前合法偏好")
            self.api("/model/update", {"id": deployments[0], "model_info": {"disabled": True}})
            check(request(path=native), 1, "停用粘性部署后选择仍启用部署")
            self.api("/model/update", {"id": deployments[0], "model_info": {"disabled": False}})
            check(request(path=native), 1, "恢复部署后保持新偏好")
            self.api("/route_template/" + templates[1] + "/update", {"body": template_body([100, 0])})
            self.api("/key/update", {"key": primary["key"], "route_template_id": templates[1]})
            check(request(), 0, "重新绑定模板后不沿用不合法偏好")
            self.api("/key/update", {"key": primary["key"], "route_template_id": templates[0]})
            check(request(), 0, "重新绑定双目标模板保留合法偏好")
            # 同密钥、同会话改用不同公开模型，必须落在另一模型的候选池；原模型偏好随后仍保持。
            other = next(row for row in self.data["models"] if row["id"] == "qiniu-glm")
            self.api("/key/update", {"key": primary["key"], "models": [model, other["public_name"]]})
            other_subject = {**primary, "call_model": other["public_name"]}
            other_response = request(subject=other_subject)
            self.affinity_receipt(other_subject, other_response, session, "explicit", other, "同会话不同模型使用独立候选池")
            check(request(), 0, "切回原模型保持原部署偏好")
            self.api("/key/update", {"key": primary["key"], "models": [model]})
            # 无显式 ID 的完整历史必须稳定推断；新增对话不改变首轮指纹。
            history = [{"role": "user", "content": "Reply only OK. " + session + " inferred"}]
            first = request(messages=history, explicit=False)
            inferred = header_value(first[1], "x-session-id")
            if not inferred or not inferred.startswith("prompt:v1:"):
                raise AssertionError("无显式会话未生成稳定 prompt 指纹")
            # 用真实回答追加历史；目标由首轮持久化事实确定，不假定随机调度结果。
            inferred_index = 0
            try:
                self.route_evidence(first[2], "fennoai")
            except AssertionError as error:
                if "真实路由与预期不符" not in str(error):
                    raise
                inferred_index = 1
            check(first, inferred_index, "首轮推断会话身份", ident=inferred, source="prompt")
            history += [first[0]["choices"][0]["message"], {"role": "user", "content": "Reply only OK again."}]
            check(request(messages=history, explicit=False), inferred_index, "完整真实历史保持推断会话与部署", ident=inferred, source="prompt")
            for invalid, headers in (({"session_id": "other"}, {"X-Session-Id": session}),
                                     ({"session_id": " "}, None), ({"session_id": "x" * 257}, None),
                                     ({"previous_response_id": session + "-unknown"}, None)):
                body = {"model": model, "messages": [{"role": "user", "content": "Reply only OK."}], **invalid}
                self.affinity_request(primary, body, request_headers=headers, expected=400)
                self.action_ok(model, "bypass_openai_chat", "非法或冲突会话明确拒绝", "HTTP=400")
            # 模型权限必须先于旧粘性，不能用历史成功绕过新的白名单。
            self.api("/key/update", {"key": primary["key"], "models": [next(row["public_name"] for row in self.data["models"] if row["id"] == "qiniu-glm")]})
            self.affinity_request(primary, {"model": model, "session_id": session, "messages": [{"role": "user", "content": "Reply only OK."}]}, expected=401)
            self.api("/key/update", {"key": primary["key"], "models": [model]})
            self.api("/model/delete", {"id": deployments[0]})
            deleted.add(deployments[0])
            check(request(), 1, "删除旧粘性部署后选择当前合法部署")
            self.report["checks"].append({"name": "Affinity-session-template-model-endpoint", "passed": True})
            self.save()
        finally:
            # 无论创建或请求在哪一步失败，都继续尝试清理其余本轮资源；清理错误不能静默吞掉。
            errors = []
            for subject in subjects:
                try:
                    self.api("/key/delete", {"keys": [subject["key"]]})
                except Exception as error:
                    errors.append(error)
            for ident in templates:
                try:
                    self.api("/route_template/" + ident + "/delete", {})
                except Exception as error:
                    errors.append(error)
            for ident in deployments:
                if ident in deleted:
                    continue
                try:
                    self.api("/model/delete", {"id": ident})
                except Exception as error:
                    errors.append(error)
            self.state["deployments"] = [r for r in self.state["deployments"] if r["id"] not in deployments]
            self.save()
            if errors:
                raise AssertionError(f"Affinity 临时资源清理失败 {len(errors)} 项") from errors[0]

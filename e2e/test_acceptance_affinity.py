"""Affinity 验收单元测试：确定性协议断言、完整场景分发和异常资源清理。"""
import tempfile
import unittest
from unittest import mock
from urllib.parse import unquote

from acceptance import Dataset, load_manifest


class AffinityTests(unittest.TestCase):
    """各场景使用临时目录与内存夹具，无真实供应商或共享数据副作用。"""

    def test_affinity_request_validates_real_answer_usage_and_cache(self):
        """目的：成功必须是真实非空回答与正用量；前置 API 夹具，覆盖正常、空回答、缺 ID、零用量、缓存及预期拒绝；目录自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://localhost:1", directory, load_manifest())
            valid = {"choices": [{"message": {"role": "assistant", "content": "OK"}}], "usage": {"prompt_tokens": 2, "completion_tokens": 1}}
            fixtures = [(valid, {"x-litellm-call-id": "id"}, 200, True),
                        ({**valid, "choices": [{"message": {"content": ""}}]}, {"x-litellm-call-id": "id"}, 200, False),
                        (valid, {}, 200, False), (valid, {"x-litellm-call-id": "id", "x-litellm-cache-hit": "true"}, 200, False),
                        ({**valid, "usage": {"prompt_tokens": 0}}, {"x-litellm-call-id": "id"}, 200, False),
                        ({"error": "invalid session"}, {}, 400, True)]
            for answer, headers, status, passes in fixtures:
                with self.subTest(status=status, headers=headers), mock.patch.object(dataset, "api", return_value=(answer, headers, status)) as api:
                    if passes:
                        result = dataset.affinity_request({"key": "secret"}, {}, expected=status, step="原生入口")
                        self.assertIn("原生入口", result[1]["_label"])
                        self.assertEqual(unquote(api.call_args.kwargs["request_headers"]["X-Xhub-Label"]), result[1]["_label"])
                        self.assertNotIn("secret", result[1]["_label"])
                    else:
                        with self.assertRaises(AssertionError):
                            dataset.affinity_request({"key": "secret"}, {})

    def test_affinity_receipt_checks_identity_route_and_unique_id(self):
        """目的：账单或会话错误不能通过粘性验收；前置独立账单与路由夹具，覆盖有效、头冲突、来源错误、模型错误和重复 ID；目录自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://localhost:1", directory, load_manifest())
            dataset.logger = mock.Mock()
            subject = {"call_model": "model"}
            definition = {"provider": "qiniu", "upstream_model": "upstream"}
            response = ({}, {"x-session-id": "session", "_label": "note"}, "call")
            bill = {"label": "note", "model": "model", "session_id": "session", "session_source": "explicit"}
            route = {"supplier": "qiniu", "upstream_model": "upstream", "outbound_endpoint": "https://supplier/v1/chat/completions"}
            with mock.patch.object(dataset, "wait_bill", return_value=bill), mock.patch.object(dataset, "route_evidence", return_value=route), mock.patch("acceptance.affinity.bill_check", return_value=0.1):
                for field in ("model", "session_source", "session_id", "label"):
                    original = bill[field]
                    bill[field] = "wrong"
                    with self.subTest(field=field), self.assertRaises(AssertionError):
                        dataset.affinity_receipt(subject, response, "session", "explicit", definition, "round")
                    bill[field] = original
                with self.assertRaises(AssertionError):
                    dataset.affinity_receipt(subject, response, "conflict", "explicit", definition, "round")
                dataset.affinity_receipt(subject, response, "session", "explicit", definition, "round")
                self.assertEqual(len(dataset.report["affinity_evidence"]), 1)
                self.assertEqual(dataset.report["affinity_evidence"][0]["step"], "round")
                self.assertEqual(dataset.report["affinity_evidence"][0]["label"], "note")
                with self.assertRaisesRegex(AssertionError, "重复"):
                    dataset.affinity_receipt(subject, response, "session", "explicit", definition, "round")

    def test_affinity_chain_cleans_partial_creation_and_request_failure(self):
        """目的：创建或首轮供应商故障都必须清理已成功创建的资源；前置管理接口夹具，验证部署、模板、密钥和内存状态清理；目录自动清理。"""
        for failure in ("second-deployment", "request"):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as directory:
                dataset = Dataset("http://localhost:1", directory, load_manifest())
                dataset.logger = mock.Mock()
                dataset.state.update(chat_models={"fennoai": "gpt-5.6-sol"}, selected={
                    "fennoai": {"provider": {"id": "fennoai"}}, "qiniu": {"provider": {"id": "qiniu"}}}, deployments=[])
                created, routes = [], []
                def deployment(*args, **kwargs):
                    """记录成功创建的本轮部署；第二次可抛故障，返回 ID，内存状态供清理核对。"""
                    if created and failure == "second-deployment":
                        raise RuntimeError("creation failed")
                    ident = "dep" + str(len(created))
                    created.append(ident)
                    dataset.state["deployments"].append({"id": ident, "public_name": "gpt-5.6-sol"})
                    return ident
                def api(path, body, **kwargs):
                    """记录管理写入并返回唯一 ID/密钥；参数同真实接口，不访问外部服务。"""
                    routes.append((path, body))
                    return {"id": "r" + str(len(routes)), "key": "k" + str(len(routes))}, {}
                with mock.patch.object(dataset, "deployment", side_effect=deployment), mock.patch.object(dataset, "api", side_effect=api), mock.patch.object(dataset, "affinity_request", side_effect=RuntimeError("supplier failed")):
                    with self.assertRaises(RuntimeError):
                        dataset.verify_affinity_chain({"user_id": "u", "team_id": "t", "project_id": "p"})
                for ident in created:
                    self.assertIn(("/model/delete", {"id": ident}), routes)
                self.assertEqual(dataset.state["deployments"], [])
                if failure == "request":
                    self.assertEqual(sum(path == "/key/delete" for path, _ in routes), 2)
                    self.assertEqual(sum(path.startswith("/route_template/") and path.endswith("/delete") for path, _ in routes), 2)

    def test_affinity_full_chain_covers_scope_mutations_and_cleans_success(self):
        """目的：完整编排必须执行隔离、配置失效、并发及拒绝流程；前置供应商回执夹具，验证端点声明修改、模板绑定、模型隔离和成功清理；目录自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://localhost:1", directory, load_manifest())
            dataset.logger = mock.Mock()
            dataset.state.update(chat_models={"fennoai": "gpt-5.6-sol"}, selected={
                "fennoai": {"provider": {"id": "fennoai"}}, "qiniu": {"provider": {"id": "qiniu"}}},
                deployments=[{"id": "baseline", "public_name": "gpt-5.6-sol"}])
            mutations, requests, created = [], [], []
            def deployment(*args, **kwargs):
                """模拟临时部署创建；返回唯一 ID 并写入内存状态，供成功清理断言。"""
                ident = "dep" + str(len(created))
                created.append(ident)
                dataset.state["deployments"].append({"id": ident, "public_name": "gpt-5.6-sol"})
                return ident
            def api(path, body, **kwargs):
                """记录管理操作并返回 ID/密钥；保持本轮资源独立，不读写外部系统。"""
                mutations.append((path, body))
                return {"id": "resource" + str(len(mutations)), "key": "key" + str(len(mutations))}, {}
            def request(subject, body, path="/v1/chat/completions", **kwargs):
                """提供不同请求 ID 和显式或推断会话响应；参数同实际发送方法，不模拟路由结论。"""
                requests.append((subject, body, path, kwargs))
                session = body.get("session_id", "prompt:v1:fixture")
                return {"choices": [{"message": {"role": "assistant", "content": "OK"}}]}, {"x-session-id": session}, "call" + str(len(requests))
            with mock.patch.object(dataset, "deployment", side_effect=deployment), mock.patch.object(dataset, "api", side_effect=api), \
                 mock.patch.object(dataset, "affinity_request", side_effect=request), mock.patch.object(dataset, "affinity_receipt") as receipt, \
                 mock.patch.object(dataset, "route_evidence", return_value={}), mock.patch.object(dataset, "wait_bill", return_value={"outbound_endpoint": "/v1/chat/completions"}):
                dataset.verify_affinity_chain({"user_id": "u", "team_id": "t", "project_id": "p"})
            labels = [call.args[-1] for call in receipt.call_args_list]
            self.assertIn("同会话不同模型使用独立候选池", labels)
            self.assertIn("原生端点保留七牛，未被统一端点覆盖", labels)
            # 同一会话切换入口的备注必须解释偏好范围和权重，避免日志归组被误解为全入口共享粘性。
            for index, entry, expected in ((6, "/bypass/openai/v1/chat/completions", "预期建立七牛偏好"),
                                           (7, "/v1/chat/completions", "统一入口已建立的 Fenno 偏好"),
                                           (8, "/bypass/openai/v1/chat/completions", "不继承第8步统一入口的 Fenno")):
                with self.subTest(step=index + 1):
                    self.assertEqual(requests[index][2], entry)
                    self.assertEqual(requests[index][1]["session_id"], requests[0][1]["session_id"])
                    self.assertIn("粘性按密钥+模型+session+入口隔离", requests[index][3]["step"])
                    self.assertIn(expected, requests[index][3]["step"])
            for call in receipt.call_args_list:
                if call.args[-1] in ("停用粘性部署后选择仍启用部署", "恢复部署后保持新偏好"):
                    request_index = int(call.args[1][2].removeprefix("call")) - 1
                    self.assertEqual(requests[request_index][2], "/bypass/openai/v1/chat/completions",
                                     "停用流程必须验证当前绑定被停用部署的原生端点")
            self.assertEqual(sum("并发首次请求" in label for label in labels), 3)
            self.assertIn("删除旧粘性部署后选择当前合法部署", labels)
            self.assertEqual(sum(item[3].get("expected") == 400 for item in requests), 4)
            self.assertEqual(sum(item[3].get("expected") == 401 for item in requests), 1)
            for path, body in mutations:
                if path == "/route_template/new":
                    self.assertIn({"deployment_id": "baseline", "weight": 0}, body["body"]["model_routes"][0]["allocations"])
            self.assertIn(("/model/update", {"id": "dep1", "model_info": {"endpoint_types": ["responses"]}}), mutations)
            self.assertEqual(sum(path == "/model/delete" for path, _ in mutations), 2)
            self.assertEqual(sum(path == "/key/delete" for path, _ in mutations), 2)
            self.assertEqual(dataset.state["deployments"], [{"id": "baseline", "public_name": "gpt-5.6-sol"}])

"""真实验收 providers 模块的单元测试；各用例独立管理夹具与清理。"""
import copy
import json
import random
import tempfile
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from unittest import mock
from acceptance import Dataset, Observer, chat_candidates, chat_probe_result, curl_json, forward_supplier_chat, http, load_manifest


class AcceptanceTests(unittest.TestCase):
    """验证所属业务模块的正常、边界和失败路径，资源由各测试自行隔离清理。"""

    def test_candidates(self):
        """目的：真实目录优先稳定真实 ID 且只保留聊天候选；前置混合目录，验证优先级、去重、空目录边界，无副作用。"""
        rows = chat_candidates({"model_ids": ["gpt-real", "gpt-real", "qwen-live", "gpt-image", "qwen-embedding", "tts"]},
                               random.Random(1), ["qwen-live", "missing-model"])
        self.assertEqual(set(rows), {"gpt-real", "qwen-live"})
        self.assertEqual(rows[0], "qwen-live")
        self.assertEqual(chat_candidates({}, random.Random(1)), [])


    def test_provider_catalog_retries_empty_read_only_response(self):
        """目的：真实供应商目录偶发返回空列表时有限重试且不伪造模型；前置一次空目录后取得两个真实 ID，验证结果、调用次数和等待次数，模拟调用无需清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest())
            dataset.admin = "admin-token"
            provider = dataset.data["providers"][0]
            responses = [({"model_ids": []}, {}), ({"model_ids": ["gpt-6-sol", "gpt-image-2"]}, {})]
            with mock.patch.object(dataset, "api", side_effect=responses) as api, \
                    mock.patch("acceptance.support.time.sleep") as wait:
                catalog = dataset.discover_catalog(provider, attempts=3, delay=1)
            self.assertEqual(catalog["model_ids"], ["gpt-6-sol", "gpt-image-2"])
            self.assertEqual(api.call_count, 2)
            wait.assert_called_once_with(1)


    def test_provider_catalog_fails_after_declared_attempts(self):
        """目的：连接代理和官方目录都失败时停止构建且不能退回自定义模型；前置三次错误代理及直连失败，验证明确失败和固定代理次数，模拟调用无需清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest())
            dataset.admin = "admin-token"
            provider = dataset.data["providers"][0]
            with mock.patch.object(dataset, "api", return_value=({"error": "temporary unavailable"}, {})) as api, \
                    mock.patch.object(dataset, "discover_supplier_catalog", side_effect=RuntimeError("direct unavailable")), \
                    mock.patch("acceptance.support.time.sleep") as wait:
                with self.assertRaisesRegex(RuntimeError, "连接代理和供应商官方目录"):
                    dataset.discover_catalog(provider, attempts=3, delay=1)
            self.assertEqual(api.call_count, 3)
            self.assertEqual(wait.call_count, 2)


    def test_provider_catalog_falls_back_to_live_supplier_directory(self):
        """目的：连接目录代理持续失败时仍从同一供应商官方接口取得真实模型；前置代理三次错误和官方目录成功，验证回退结果与来源，不访问外网且无需清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest())
            provider = dataset.data["providers"][1]
            expected = {"model_ids": ["google/gemini-3.6-flash"], "source": "supplier-official-live"}
            with mock.patch.object(dataset, "api", return_value=({"error": "context deadline exceeded"}, {})), \
                    mock.patch.object(dataset, "discover_supplier_catalog", return_value=expected) as direct, \
                    mock.patch("acceptance.support.time.sleep"):
                catalog = dataset.discover_catalog(provider, attempts=3, delay=1)
            self.assertEqual(catalog, expected)
            direct.assert_called_once_with(provider, attempts=3, delay=1)


    def test_direct_supplier_catalog_normalizes_openai_models(self):
        """目的：官方 /v1/models 响应只提取真实模型 ID 并去重；前置重复 OpenAI 目录响应，验证来源、数量和 GET 调用，不访问外网且无需清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest())
            provider = dataset.data["providers"][1]
            response = {"object": "list", "data": [{"id": "google/gemini-3.6-flash"}, {"id": "google/gemini-3.6-flash"}, {"id": "gpt-image"}]}
            with mock.patch.dict("os.environ", {provider["key_env"]: "secret"}), \
                    mock.patch("acceptance.catalog.curl_json", return_value=(200, response, {})) as request:
                catalog = dataset.discover_supplier_catalog(provider, attempts=1)
            self.assertEqual(catalog["model_ids"], ["google/gemini-3.6-flash", "gpt-image"])
            self.assertEqual(catalog["source"], "supplier-official-live")
            self.assertIsNone(request.call_args.args[2])


    def test_chat_probe_records_network_failure_without_stopping_candidates(self):
        """目的：单个真实聊天候选网络断开时继续候选选择；前置供应商连接被远端关闭，验证失败证据和返回值，不访问真实外网且无需清理。"""
        with tempfile.TemporaryDirectory() as directory:
            lines = []
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lines.append)
            provider = dataset.data["providers"][0]
            with mock.patch.dict("os.environ", {provider["key_env"]: "secret"}), \
                    mock.patch("acceptance.catalog.curl_json", side_effect=ConnectionError("remote closed")):
                passed = dataset.probe_chat_candidate(provider, "gpt-6-sol", 1, 5)
            self.assertFalse(passed)
            self.assertEqual(dataset.report["probe_attempts"][-1]["error"], "ConnectionError")
            self.assertFalse(any(row.get("name") == "real-model-probe" for row in dataset.report["checks"]))
            output = "\n".join(lines)
            self.assertIn("⚠️ [gpt-6-sol / bypass_openai_chat / 真实模型候选探测 1/5]", output)
            self.assertIn("本次探测不计入用例失败", output)
            self.assertNotIn("❌", output)
            self.assertNotIn("remote closed", output)


    def test_chat_probe_marks_only_nonzero_real_usage_as_success(self):
        """目的：真实聊天候选必须同时返回回答和非零计量；前置成功供应商响应，验证打勾日志和通过证据，不访问真实外网且无需清理。"""
        with tempfile.TemporaryDirectory() as directory:
            lines = []
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lines.append)
            provider = dataset.data["providers"][0]
            reply = {"choices": [{"message": {"content": "OK"}}], "usage": {"prompt_tokens": 8, "completion_tokens": 1}}
            with mock.patch.dict("os.environ", {provider["key_env"]: "secret"}), \
                    mock.patch("acceptance.catalog.curl_json", return_value=(200, reply, {})):
                passed = dataset.probe_chat_candidate(provider, "gpt-6-sol", 2, 5)
            self.assertTrue(passed)
            self.assertTrue(dataset.report["probe_attempts"][-1]["passed"])
            self.assertFalse(any(row.get("name") == "real-model-probe" for row in dataset.report["checks"]))
            self.assertIn("✅ [gpt-6-sol / bypass_openai_chat / 真实模型候选探测 2/5]", "\n".join(lines))


    def test_lifecycle_model_selection_continues_after_failed_candidate(self):
        """目的：模型生命周期探测遇到首个候选网络失败时继续选择；前置首个返回失败且第二个返回成功，验证选中第二个真实模型和完整候选顺序，模拟调用无需清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest())
            provider = dataset.data["providers"][1]
            with mock.patch.object(dataset, "probe_chat_candidate", side_effect=[False, True]) as probe:
                selected = dataset.select_lifecycle_model(provider, ["first-real", "second-real"], attempts=2, delay=0)
            self.assertEqual(selected, "second-real")
            self.assertEqual([call.args[1] for call in probe.call_args_list], ["first-real", "second-real"])
            self.assertEqual(probe.call_args_list[0].kwargs["action"], "生命周期探测")


    def test_chat_probe_result_handles_success_boundaries_and_malformed_responses(self):
        """目的：探测判定不能把计费、推理或畸形结构当作回答；前置正常、空白、错误和计量边界响应，验证安全失败及无正文证据；纯内存测试无需清理。"""
        good = {"choices": [{"message": {"content": "OK"}, "finish_reason": "stop"}],
                "usage": {"prompt_tokens": 19, "completion_tokens": 1}}
        self.assertTrue(chat_probe_result(200, good)["passed"])
        for status, reply in [(502, good), (200, None), (200, []), (200, {"choices": [None]}),
                              (200, {"choices": "invalid"}), (200, {**good, "error": {"message": "secret"}})]:
            with self.subTest(status=status, reply=reply):
                self.assertFalse(chat_probe_result(status, reply)["passed"])
        for content in [None, "", " \n", ["OK"], {"text": "OK"}]:
            with self.subTest(content=content):
                reply = copy.deepcopy(good)
                reply["choices"][0]["message"] = {"content": content, "reasoning_content": "private reasoning"}
                result = chat_probe_result(200, reply)
                self.assertFalse(result["passed"])
                self.assertNotIn("private reasoning", json.dumps(result))
                self.assertEqual(result["reasoning_chars"], 17)
        for usage in [None, [], {"prompt_tokens": 19, "completion_tokens": 0},
                      {"prompt_tokens": -1, "completion_tokens": 1},
                      {"prompt_tokens": True, "completion_tokens": 1},
                      {"prompt_tokens": "secret", "completion_tokens": 1},
                      {"prompt_tokens": float("nan"), "completion_tokens": 1}]:
            with self.subTest(usage=usage):
                result = chat_probe_result(200, {**good, "usage": usage})
                self.assertFalse(result["passed"])
                json.dumps(result, allow_nan=False)
                self.assertNotIn("secret", json.dumps(result))


    def test_chat_probe_retries_truncated_reasoning_with_larger_budget(self):
        """目的：推理耗尽32 token后应自适应重试而非重复相同请求；前置首轮仅有推理且 length、次轮有回答，验证预算32→1024和结构诊断；临时目录自动清理且不访问外网。"""
        with tempfile.TemporaryDirectory() as directory:
            lines = []
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lines.append)
            provider = dataset.data["providers"][1]
            truncated = {"choices": [{"message": {"content": None, "reasoning_content": "private reasoning"},
                                      "finish_reason": "length"}],
                         "usage": {"prompt_tokens": 19, "completion_tokens": 32}}
            good = {"choices": [{"message": {"content": "OK"}, "finish_reason": "stop"}],
                    "usage": {"prompt_tokens": 19, "completion_tokens": 60}}
            with mock.patch.dict("os.environ", {provider["key_env"]: "secret"}), \
                    mock.patch("acceptance.catalog.curl_json", side_effect=[(200, truncated, {}), (200, good, {})]) as request:
                self.assertTrue(dataset.probe_chat_candidate(provider, "moonshotai/kimi-k2.6", 1, 2, attempts=2, delay=0))
            self.assertEqual([call.args[2]["max_tokens"] for call in request.call_args_list], [32, 1024])
            probes = dataset.report["probe_attempts"]
            self.assertEqual([row["passed"] for row in probes], [False, True])
            self.assertEqual(probes[0]["finish_reason"], "length")
            self.assertEqual(probes[0]["content_chars"], 0)
            self.assertIn("提高 max_tokens 至 1024", "\n".join(lines))
            self.assertNotIn("private reasoning", json.dumps(probes) + "\n".join(lines))


    def test_chat_probe_does_not_expand_budget_for_unrelated_failures(self):
        """目的：网络失败、非截断空回答和错误响应不能触发预算增加；前置失败响应及两次重试，验证始终32 token且失败单列；临时目录清理，无外部调用。"""
        for status, reply in [(502, {}), (200, {"choices": [{"message": {"content": ""}, "finish_reason": "stop"}]}),
                              (200, {"error": "private", "choices": [{"message": {}, "finish_reason": "length"}]})]:
            with self.subTest(status=status, reply=reply), tempfile.TemporaryDirectory() as directory:
                dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lambda line: None)
                provider = dataset.data["providers"][1]
                with mock.patch.dict("os.environ", {provider["key_env"]: "secret"}), \
                        mock.patch("acceptance.catalog.curl_json", return_value=(status, reply, {})) as request:
                    self.assertFalse(dataset.probe_chat_candidate(provider, "candidate", 1, 1, attempts=2, delay=0))
                self.assertEqual([call.args[2]["max_tokens"] for call in request.call_args_list], [32, 32])
                self.assertFalse(dataset.report["checks"])


    def test_lifecycle_probe_uses_real_http_and_continues_after_truncation_limit(self):
        """目的：真实HTTP探测扩容后仍无正文必须继续候选；前置本地供应商端点，验证curl载荷、32→1024预算上限和最终选中可用模型；服务、线程及临时目录均自动清理。"""
        requests = []

        class Supplier(BaseHTTPRequestHandler):
            """为探测提供本地协议边界；请求记录留在内存，服务由测试finally关闭。"""

            def do_POST(self):
                """用途：按候选返回截断推理或最终回答；无参数或返回值，供HTTP服务调用，读取请求并写JSON，正文和凭据不落盘。"""
                body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                requests.append((self.path, body))
                truncated = body["model"] == "truncated-model"
                reply = {"choices": [{"message": {"content": None if truncated else "OK",
                                                     "reasoning_content": "thinking" if truncated else None},
                                       "finish_reason": "length" if truncated else "stop"}],
                         "usage": {"prompt_tokens": 19, "completion_tokens": body["max_tokens"] if truncated else 1}}
                raw = json.dumps(reply).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(raw)))
                self.end_headers()
                self.wfile.write(raw)

            def log_message(self, format, *args):
                """用途：关闭服务默认日志以保持测试报告干净；参数为格式及值，无返回，无副作用。"""
                return

        server = ThreadingHTTPServer(("127.0.0.1", 0), Supplier)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            with tempfile.TemporaryDirectory() as directory:
                dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lambda line: None)
                provider = dataset.data["providers"][1]
                with mock.patch.dict("os.environ", {provider["key_env"]: "local-only",
                                                     provider["base_env"]: f"http://127.0.0.1:{server.server_port}/v1"}):
                    selected = dataset.select_lifecycle_model(provider, ["truncated-model", "working-model"], delay=0)
                self.assertEqual(selected, "working-model")
                self.assertEqual([body["max_tokens"] for path, body in requests], [32, 1024, 32])
                self.assertTrue(all(path == "/v1/chat/completions" for path, body in requests))
                self.assertEqual([row["passed"] for row in dataset.report["probe_attempts"]], [False, False, True])
        finally:
            server.shutdown()
            server.server_close()
            thread.join(timeout=5)


    def test_lifecycle_model_selection_fails_when_all_candidates_fail(self):
        """目的：模型生命周期不得把全部供应商失败误报为通过；前置两个候选均失败，验证明确异常和每个候选均已探测，模拟调用无需清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest())
            provider = dataset.data["providers"][1]
            with mock.patch.object(dataset, "probe_chat_candidate", return_value=False) as probe:
                with self.assertRaisesRegex(RuntimeError, "2 个额外真实聊天模型"):
                    dataset.select_lifecycle_model(provider, ["first-real", "second-real"], attempts=2, delay=0)
            self.assertEqual(probe.call_count, 2)


    def test_curl_json_parses_status_and_rejects_transport_failure(self):
        """目的：供应商直连使用兼容长连接的 curl 且正确分离 JSON 与状态；前置模拟成功及网络失败进程，验证解析和异常，不执行真实命令且无需清理。"""
        success = mock.MagicMock()
        success.__enter__.return_value = success
        success.__exit__.return_value = False
        success.communicate.return_value = (b'{"choices":[{"message":{"content":"OK"}}]}\n200', b"")
        success.returncode = 0
        with mock.patch("acceptance.support.subprocess.Popen", return_value=success) as popen:
            status, value, headers = curl_json("https://example.invalid", "/v1/chat/completions", {"model": "gpt-real"}, "secret")
        self.assertEqual((status, headers), (200, {}))
        self.assertTrue(value["choices"])
        self.assertEqual(popen.call_count, 1)

        failed = mock.MagicMock()
        failed.__enter__.return_value = failed
        failed.__exit__.return_value = False
        failed.communicate.return_value = (b"", b"remote closed")
        failed.returncode = 52
        with mock.patch("acceptance.support.subprocess.Popen", return_value=failed):
            with self.assertRaisesRegex(ConnectionError, "退出码=52") as raised:
                curl_json("https://example.invalid", "/v1/chat/completions", {}, "secret")
        self.assertNotIn("secret", str(raised.exception))


    def test_forward_supplier_chat_uses_curl_and_normalizes_v1_route(self):
        """目的：81 密钥观察器使用与构建探测相同的 curl 网络路径；前置带/v1及不带/v1的供应商地址，验证路由、令牌和超时，模拟调用不访问外网且无需清理。"""
        provider = {"base_env": "TEST_ACCEPTANCE_BASE", "base": "https://fallback.invalid",
                    "key_env": "TEST_ACCEPTANCE_KEY"}
        reply = (200, {"choices": [{}], "usage": {"prompt_tokens": 1, "completion_tokens": 1}}, {})
        with mock.patch.dict("os.environ", {"TEST_ACCEPTANCE_BASE": "https://provider.invalid/v1",
                                                "TEST_ACCEPTANCE_KEY": "secret"}), \
                mock.patch("acceptance.support.curl_json", return_value=reply) as request:
            self.assertEqual(forward_supplier_chat(provider, {"model": "gpt-real"}), reply)
        request.assert_called_once_with("https://provider.invalid/v1", "/chat/completions",
                                        {"model": "gpt-real"}, "secret", timeout=330)

        with mock.patch.dict("os.environ", {"TEST_ACCEPTANCE_BASE": "https://provider.invalid",
                                                "TEST_ACCEPTANCE_KEY": "secret"}), \
                mock.patch("acceptance.support.curl_json", return_value=reply) as request:
            forward_supplier_chat(provider, {"model": "gpt-real"})
        self.assertEqual(request.call_args.args[1], "/v1/chat/completions")


    def test_forward_supplier_chat_preserves_transport_failure(self):
        """目的：真实供应商网络失败不能伪造成成功响应；前置 curl 连接错误，验证异常原样上抛供观察器记录 502，不访问外网且无需清理。"""
        provider = {"base_env": "TEST_ACCEPTANCE_BASE", "base": "https://provider.invalid",
                    "key_env": "TEST_ACCEPTANCE_KEY"}
        with mock.patch.dict("os.environ", {"TEST_ACCEPTANCE_KEY": "secret"}), \
                mock.patch("acceptance.support.curl_json", side_effect=ConnectionError("remote closed")):
            with self.assertRaisesRegex(ConnectionError, "remote closed"):
                forward_supplier_chat(provider, {"model": "gpt-real"})


    def test_observer_records_route_tags_and_only_fault_tag_injects_429(self):
        """目的：观察器必须区分同一真实模型的权重部署且只为 fault 标签注入故障；前置模拟真实供应商 200，验证 weight-a 被转发、fault 不转发并分别保存 route_tag，finally 关闭本地服务。"""
        providers = [{"id": "fennoai", "base": "https://example.invalid",
                      "base_env": "TEST_BASE", "key_env": "TEST_KEY"}]
        reply = (200, {"choices": [{"message": {"content": "OK"}}],
                       "usage": {"prompt_tokens": 3, "completion_tokens": 1}}, {})
        with mock.patch("acceptance.observer.forward_supplier_chat", return_value=reply) as forward:
            observer = Observer(providers, 10)
            try:
                body = {"model": "gpt-5.6-sol", "messages": [{"role": "user", "content": "tagged"}]}
                self.assertEqual(http(observer.base, "/fennoai/weight-a", body)[0], 200)
                self.assertEqual(http(observer.base, "/fennoai/fault", body)[0], 429)
            finally:
                observer.close()
        self.assertEqual(forward.call_count, 1)
        self.assertEqual([row["route_tag"] for row in observer.rows], ["weight-a", "fault"])
        self.assertEqual([row["forwarded"] for row in observer.rows], [True, False])


    def test_observer_normalizes_untagged_openai_protocol_path(self):
        """目的：无观察标签的真实部署必须记录为 default；前置模拟真实供应商成功并请求 /provider/v1/chat/completions，验证协议版本不会被误认成部署标签，finally 关闭本地服务。"""
        providers = [{"id": "qiniu", "base": "https://example.invalid",
                      "base_env": "TEST_BASE", "key_env": "TEST_KEY"}]
        reply = (200, {"choices": [{"message": {"content": "OK"}}],
                       "usage": {"prompt_tokens": 3, "completion_tokens": 1}}, {})
        with mock.patch("acceptance.observer.forward_supplier_chat", return_value=reply):
            observer = Observer(providers, 10)
            try:
                body = {"model": "z-ai/glm-5", "messages": [{"role": "user", "content": "fallback"}]}
                self.assertEqual(http(observer.base, "/qiniu/v1/chat/completions", body)[0], 200)
            finally:
                observer.close()
        self.assertEqual(observer.rows[0]["route_tag"], "default")
        self.assertTrue(observer.rows[0]["forwarded"])



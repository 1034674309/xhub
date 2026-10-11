"""真实验收 billing_chat 模块的单元测试；各用例独立管理夹具与清理。"""
from contextlib import redirect_stdout
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock
from acceptance import Dataset, bill_check, cache_bill_check, header_value, load_manifest, response_cache_hit
from acceptance import runner_database as DATABASE


class AcceptanceTests(unittest.TestCase):
    """验证所属业务模块的正常、边界和失败路径，资源由各测试自行隔离清理。"""

    def test_bill(self):
        """目的：验证五级账单、派生计费归属与价格快照；前置固定费率，核对成功、归属错、零用量和错误金额，不产生数据库数据。"""
        rates = load_manifest()["billing"]
        key = dict(token_id="k", user_id="u", team_id="t", project_id="p", organization_id="o")
        bill = dict(api_key="k", user="u", team_id="t", project_id="p", organization_id="o",
                    prompt_tokens=11, completion_tokens=7, spend=0.000025, metadata={"cost_breakdown": {"source": "snapshot"}})
        self.assertAlmostEqual(bill_check(bill, key, rates), 0.000025)
        derived = {**key, "team_id": "", "organization_id": "",
                   "billing_team_id": "t", "billing_organization_id": "o"}
        self.assertAlmostEqual(bill_check(bill, derived, rates), 0.000025)
        for field, value in (("user", "foreign"), ("prompt_tokens", 0), ("spend", 1), ("metadata", {})):
            with self.subTest(field=field), self.assertRaises(AssertionError):
                bill_check({**bill, field: value}, key, rates)


    def test_cache_headers_and_zero_cost_bill(self):
        """目的：缓存命中兼容两种大小写不敏感响应头并核对零费用日志；前置完整五级身份和首次 usage，验证合法账单通过及非零费用、错误归属、usage、命中标记和价格明细失败，无持久化数据。"""
        self.assertTrue(response_cache_hit({"Cache_Hit": "TRUE"}))
        self.assertTrue(response_cache_hit({"X-LiteLLM-Cache-Hit": "true"}))
        self.assertFalse(response_cache_hit({"cache_hit": "false"}))
        self.assertFalse(response_cache_hit({}))
        self.assertEqual(header_value({"X-Test": "value"}, "x-test"), "value")
        key = dict(token_id="k", user_id="u", team_id="t", project_id="p", organization_id="o")
        usage = {"prompt_tokens": 11, "completion_tokens": 7}
        bill = dict(api_key="k", user="u", team_id="t", project_id="p", organization_id="o",
                    **usage, spend=0, cache_hit=True, metadata={"cost_breakdown": {"source": "cache"}})
        self.assertIsNone(cache_bill_check(bill, key, usage))
        for field, value in (("spend", 0.1), ("team_id", "wrong"), ("completion_tokens", 6),
                             ("cache_hit", False), ("metadata", {})):
            with self.subTest(field=field), self.assertRaises(AssertionError):
                cache_bill_check({**bill, field: value}, key, usage)


    def test_resume_only_skips_proven_checks(self):
        """目的：续跑只跳过成功场景；前置私有临时目录，验证失败/未运行边界仍需执行，目录自动清理，无网络。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest())
            self.assertFalse(dataset.checked("quota"))
            dataset.report["checks"] = [{"name": "quota", "passed": False}, {"name": "role", "passed": True}]
            self.assertFalse(dataset.checked("quota"))
            self.assertTrue(dataset.checked("role"))
            self.assertFalse(dataset.checked("missing"))


    def test_verify_starts_ordered_model_acceptance_after_count_check(self):
        """目的：模型验收前先核对基线；前置错误数量和完整实体，验证数量错误先拒绝、成员关系核对后模型失败中止，临时目录自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lambda message: None)
            with mock.patch.object(dataset, "verify_acceptance_models", side_effect=RuntimeError("模型阶段失败")) as models, \
                    mock.patch.object(dataset, "api") as api:
                with self.assertRaisesRegex(AssertionError, "实体数量错误"):
                    dataset.verify()
                models.assert_not_called()
                for name, count in {"organizations": 3, "teams": 9, "users": 27, "projects": 9, "keys": 81}.items():
                    dataset.state[name] = [{"id": str(index)} for index in range(count)]
                api.return_value = ({"members": [{}, {}, {}]}, {})
                with self.assertRaisesRegex(RuntimeError, "模型阶段失败"):
                    dataset.verify()
                models.assert_called_once_with()
                self.assertEqual(api.call_count, 9)


    def test_progress_log_is_visible_and_contains_no_state(self):
        """目的：长时间真实验收会立即显示阶段日志；前置内存输出，验证正文可见且状态不被序列化，无外部数据需清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest())
            output = io.StringIO()
            with redirect_stdout(output):
                dataset.progress("构建阶段可见")
            self.assertEqual(output.getvalue(), "构建阶段可见\n")
            self.assertNotIn("keys", output.getvalue())


    def test_response_usage_audit_accepts_only_complete_consistent_rows(self):
        """目的：普通成功响应必须是合法 JSON 且 usage 与计量一致；前置模拟数据库分类结果，验证正常、空证据、非法 JSON、非法 token 和数值不一致，模拟查询无需清理。"""
        valid = {"successful_responses": 12, "invalid_json": 0, "with_usage": 8,
                 "invalid_usage": 0, "mismatches": 0}
        with mock.patch.object(DATABASE, "sql", return_value=json.dumps(valid)) as query:
            self.assertEqual(DATABASE.audit_response_usage(), valid)
            statement = query.call_args.args[0]
            self.assertIn("pg_input_is_valid(l.response_body, 'jsonb')", statement)
            self.assertIn("CASE WHEN pg_input_is_valid", statement)
            self.assertIn("u.status='success'", statement)
            self.assertIn("u.call_type IN ('chat', 'responses')", statement)
            self.assertIn("response->'usage'->>'input_tokens'", statement)
            self.assertIn("response->'usage'->>'output_tokens'", statement)
            self.assertIn("official-settlement:%", statement)
        failures = (
            ({**valid, "with_usage": 0}, "没有可核对的 usage"),
            ({**valid, "invalid_json": 1}, "非法 JSON"),
            ({**valid, "invalid_usage": 1}, "非法 usage token"),
            ({**valid, "mismatches": 1}, "持久化计量不一致"),
        )
        for row, message in failures:
            with self.subTest(message=message), mock.patch.object(DATABASE, "sql", return_value=json.dumps(row)):
                with self.assertRaisesRegex(AssertionError, message):
                    DATABASE.audit_response_usage()


    def test_api_log_records_method_status_and_time_without_secrets(self):
        """目的：每个管理和数据接口都有可定位日志且不泄露参数；前置模拟 HTTP 成功，验证方法/路径/状态/耗时并隐藏查询和令牌，无外部数据需清理。"""
        with tempfile.TemporaryDirectory() as directory:
            lines = []
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lines.append)
            with mock.patch("acceptance.runtime.http", return_value=(200, {"ok": True}, {})):
                value, _ = dataset.api("/team/list?private=value", token="secret-token")
            output = "\n".join(lines)
            self.assertEqual(value, {"ok": True})
            self.assertIn("GET /team/list 开始", output)
            self.assertIn("状态=200", output)
            self.assertIn("耗时=", output)
            self.assertNotIn("private=value", output)
            self.assertNotIn("secret-token", output)


    def test_real_chat_call_retries_temporary_upstream_and_requires_nonzero_usage(self):
        """目的：81密钥真实调用可从明确的临时502恢复且只把非零usage计为成功；前置一次502和一次真实结构200，验证重试日志与回执，模拟接口无需清理。"""
        with tempfile.TemporaryDirectory() as directory:
            lines = []
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lines.append)
            key = {"key": "secret", "profile": "inherit", "call_model": "gpt-real"}
            responses = [
                ({"error": {"message": "upstream closed"}}, {}, 502),
                ({"choices": [{"message": {"content": "OK"}}],
                  "usage": {"prompt_tokens": 8, "completion_tokens": 1}},
                 {"x-litellm-call-id": "call-real"}, 200),
            ]
            with mock.patch.object(dataset, "api", side_effect=responses) as api, \
                    mock.patch("acceptance.support.time.sleep") as sleep:
                receipt = dataset.call(key, "retry-marker", defer_bill=True, retry_delay=1)
            self.assertEqual(receipt[2], "call-real")
            self.assertEqual(receipt[3], {"prompt_tokens": 8, "completion_tokens": 1})
            self.assertEqual(api.call_count, 2)
            sleep.assert_called_once_with(1)
            output = "\n".join(lines)
            self.assertIn("❌ [gpt-real / bypass_openai_chat / 第 1/5 次真实 AI 请求]", output)
            self.assertIn("✅ [gpt-real / bypass_openai_chat / 真实 AI 响应]", output)


    def test_real_chat_call_uses_long_cooldown_for_upstream_rate_limit(self):
        """目的：供应商429必须等待完整冷却窗口后再试；前置网关转译的上游429和随后成功响应，验证65秒退避及最终真实usage，模拟接口无需清理。"""
        with tempfile.TemporaryDirectory() as directory:
            lines = []
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lines.append)
            key = {"key": "secret", "profile": "inherit", "call_model": "gpt-real"}
            responses = [
                ({"error": {"message": "upstream 429"}}, {}, 502),
                ({"choices": [{"message": {"content": "OK"}}],
                  "usage": {"prompt_tokens": 8, "completion_tokens": 1}},
                 {"x-litellm-call-id": "call-rate-recovered"}, 200),
            ]
            with mock.patch.object(dataset, "api", side_effect=responses), \
                    mock.patch("acceptance.support.time.sleep") as sleep:
                receipt = dataset.call(key, "rate-marker", defer_bill=True, rate_limit_delay=65)
            self.assertEqual(receipt[2], "call-rate-recovered")
            sleep.assert_called_once_with(65)
            self.assertIn("65 秒后进行第 2/5 次", "\n".join(lines))


    def test_real_chat_call_rejects_zero_usage_after_success_status(self):
        """目的：HTTP成功但没有真实输出用量时不能打勾；前置带choices但completion为零的响应，验证立即失败且不写成功回执，模拟接口无需清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest())
            key = {"key": "secret", "profile": "inherit", "call_model": "gpt-real"}
            response = ({"choices": [{"message": {"content": ""}}],
                         "usage": {"prompt_tokens": 8, "completion_tokens": 0}},
                        {"x-litellm-call-id": "call-empty"}, 200)
            with mock.patch.object(dataset, "api", return_value=response):
                with self.assertRaisesRegex(AssertionError, "非零输入和输出 usage"):
                    dataset.call(key, "zero-marker", defer_bill=True)


    def test_chat_checkpoint_revalidates_complete_real_evidence(self):
        """目的：断点续跑只跳过完整的真实聊天证据；前置匹配模型、身份、请求标记、响应usage和价格快照，验证日志接口复验后返回原回执，临时目录自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest())
            key = {"token_id": "key-1", "profile": "inherit", "call_model": "gpt-5.6-sol",
                   "user_id": "user-1", "team_id": "team-1", "project_id": "project-1",
                   "organization_id": "org-1"}
            marker = "dataset-a1b2-3"
            row = {"call_id": "call-1", "marker": marker, "key_id": "key-1",
                   "profile": "inherit", "model": "gpt-5.6-sol",
                   "prompt_tokens": 11, "completion_tokens": 7, "cost": 0.000025}
            bill = {"api_key": "key-1", "user": "user-1", "team_id": "team-1",
                    "project_id": "project-1", "organization_id": "org-1",
                    "model": "gpt-5.6-sol", "prompt_tokens": 11, "completion_tokens": 7,
                    "spend": 0.000025, "metadata": {"cost_breakdown": {"source": "snapshot"}},
                    "messages": [{"role": "user", "content": marker}],
                    "response": {"choices": [{"message": {"content": "OK"}}],
                                 "usage": {"prompt_tokens": 11, "completion_tokens": 7}}}
            dataset.report["calls"] = [row]
            with mock.patch.object(dataset, "api", return_value=(bill, {}, 200)) as api:
                self.assertEqual(dataset.call_checkpoint(key), row)
            api.assert_called_once_with("/spend/logs/ui/call-1", expected=(200, 404), include_status=True)


    def test_chat_checkpoint_rejects_incomplete_or_mismatched_evidence(self):
        """目的：模型、身份、usage、价格或真实响应任一不符都必须补跑；前置逐项破坏已保存证据，验证检查点返回空且不会误判通过，临时目录自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest())
            key = {"token_id": "key-1", "profile": "inherit", "call_model": "gpt-5.6-sol",
                   "user_id": "user-1", "team_id": "team-1", "project_id": "project-1",
                   "organization_id": "org-1"}
            marker = "dataset-a1b2-3"
            base_row = {"call_id": "call-1", "marker": marker, "key_id": "key-1",
                        "profile": "inherit", "model": "gpt-5.6-sol",
                        "prompt_tokens": 11, "completion_tokens": 7, "cost": 0.000025}
            base_bill = {"api_key": "key-1", "user": "user-1", "team_id": "team-1",
                         "project_id": "project-1", "organization_id": "org-1",
                         "model": "gpt-5.6-sol", "prompt_tokens": 11, "completion_tokens": 7,
                         "spend": 0.000025, "metadata": {"cost_breakdown": {"source": "snapshot"}},
                         "messages": [{"content": marker}],
                         "response": {"choices": [{}],
                                      "usage": {"prompt_tokens": 11, "completion_tokens": 7}}}
            cases = {
                "模型": ({**base_row, "model": "wrong-model"}, base_bill),
                "身份": (base_row, {**base_bill, "team_id": "wrong-team"}),
                "usage": (base_row, {**base_bill, "response": {"choices": [{}],
                           "usage": {"prompt_tokens": 11, "completion_tokens": 6}}}),
                "价格": (base_row, {**base_bill, "metadata": {}}),
                "响应": (base_row, {**base_bill, "response": {"choices": [],
                           "usage": {"prompt_tokens": 11, "completion_tokens": 7}}}),
            }
            for name, (row, bill) in cases.items():
                with self.subTest(name=name):
                    dataset.report["calls"] = [row]
                    with mock.patch.object(dataset, "api", return_value=(bill, {}, 200)):
                        self.assertIsNone(dataset.call_checkpoint(key))


    def test_completed_checkpoint_survives_a_later_call_failure(self):
        """目的：后续供应商失败不能丢失此前已核账的密钥；前置先保存成功回执再模拟下一次调用失败，验证磁盘报告仍保留首条证据，临时目录自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest())
            first = {"token_id": "key-1", "profile": "inherit"}
            dataset.record_call(first, "dataset-a1-1", "call-1", "gpt-5.6-sol",
                                {"prompt_tokens": 8, "completion_tokens": 1}, 0.000010)
            failing = {"key": "secret", "token_id": "key-2", "profile": "inherit",
                       "call_model": "z-ai/glm-5"}
            with mock.patch.object(dataset, "api", side_effect=AssertionError("upstream failed")):
                with self.assertRaisesRegex(AssertionError, "upstream failed"):
                    dataset.call(failing, "dataset-a1-2")
            saved = json.loads((Path(directory) / "report.json").read_text())
            self.assertEqual([row["call_id"] for row in saved["calls"]], ["call-1"])



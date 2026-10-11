"""真实验收 media 模块的单元测试；各用例独立管理夹具与清理。"""
import copy
import base64
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock
from acceptance import Dataset, load_manifest
from acceptance import runner_database as DATABASE


class AcceptanceTests(unittest.TestCase):
    """验证所属业务模块的正常、边界和失败路径，资源由各测试自行隔离清理。"""

    def test_live_route_timeout_covers_synchronous_media_generation(self):
        """目的：给同步图片生成保留完整供应商处理窗口；前置真实清单，验证所有作用域均为300秒，不访问外网或写库。"""
        data = load_manifest()
        self.assertEqual(data["routing"]["timeout_seconds"], 300)
        for name, template in data["routing"]["templates"].items():
            with self.subTest(template=name):
                self.assertEqual(template["retry_policy"]["timeout_seconds"], 300)


    def test_video_settlement_audit_is_scoped_to_exact_task(self):
        """目的：视频结算去重必须按具体任务而非模型历史总数判断；前置固定 Ark 任务和模拟数据库结果，验证任务 ID、终态调用类型、价格及重复结算失败，无外部数据需清理。"""
        task = {"model": "bytedance/doubao-seedance-2-0-mini-260615",
                "transport": "qiniu_contents_generation", "task_id": "qvideo-exact-123"}
        proof = {"rows": 1, "priced": 1, "positive": 1, "request_id": "official-settlement:exact"}
        with mock.patch.object(DATABASE, "sql", return_value=json.dumps(proof)) as query:
            self.assertEqual(DATABASE.audit_video_settlement(task), proof)
            statement = query.call_args.args[0]
            self.assertIn("u.call_type='qiniu_contents_generation:get'", statement)
            self.assertIn("l.response_body::jsonb->>'request_id')='qvideo-exact-123'", statement)
            self.assertIn("u.status='completed' AND u.task_settled", statement)
            self.assertIn("min(u.request_id)", statement)
        with mock.patch.object(DATABASE, "sql", return_value=json.dumps({**proof, "rows": 2})):
            with self.assertRaisesRegex(AssertionError, "结算未去重"):
                DATABASE.audit_video_settlement(task)


    def test_wait_log_accepts_zero_cost_media_creation(self):
        """目的：媒体创建日志无需价格快照也能被读取，正式账单仍要求价格证据；前置模拟零费用详情，验证普通日志立即返回而账单继续轮询，模拟无外部数据需清理。"""
        dataset = Dataset("http://127.0.0.1:1", tempfile.mkdtemp(), load_manifest())
        empty_cost = (200, {"model": "video-model", "metadata": {}}, {})
        with mock.patch("acceptance.billing.http", return_value=empty_cost) as request:
            self.assertEqual(dataset.wait_log("create-id"), empty_cost[1])
            request.assert_called_once()
        with mock.patch("acceptance.billing.http", side_effect=[empty_cost, (200, {"metadata": {"cost_breakdown": {"source": "catalog"}}}, {})]), \
             mock.patch("acceptance.support.time.sleep"):
            self.assertIn("cost_breakdown", dataset.wait_bill("settlement-id")["metadata"])


    def test_image_log_requires_real_response_body(self):
        """目的：图片日志必须包含真实模型、精确 prompt、图片正文和价格快照；前置模拟完整详情，验证通过及缺图失败，临时目录自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lambda _: None)
            call = {"model": "gpt-image-2", "call_id": "image-log"}
            bill = {"model": "gpt-image-2", "proxy_server_request": {"body": {
                "model": "gpt-image-2", "prompt": "A red apple on a white background."}},
                "response": {"data": [{"b64_json": "real-base64"}]},
                "metadata": {"cost_breakdown": {"source": "catalog"}}}
            with mock.patch.object(dataset, "wait_bill", return_value=bill):
                self.assertEqual(dataset.verify_image_log(call), bill)
            with mock.patch.object(dataset, "wait_bill", return_value={**bill, "response": {"data": []}}):
                with self.assertRaisesRegex(AssertionError, "缺少真实图片响应"):
                    dataset.verify_image_log(call)


    def test_video_logs_require_exact_task_url_and_usage_boundary(self):
        """目的：视频日志分别验证创建与终态证据；前置 Ark 完整日志，验证精确 Task ID、URL、usage 通过及错误 URL 失败，临时目录自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lambda _: None)
            model = "bytedance/doubao-seedance-2-0-mini-260615"
            task = {"model": model, "transport": "qiniu_contents_generation", "task_id": "qvideo-exact",
                    "create_call_id": "create-id", "settlement_log_id": "settlement-id",
                    "bypass_result": {"video_url": "https://media.example/video.mp4"}}
            definition = next(row for row in load_manifest()["models"] if row.get("public_name") == model)
            created = {"model": model, "proxy_server_request": {"body": {"model": model, "content": [{
                "type": "text", "text": "A red apple on a white table, static camera, gentle natural light."}]}},
                "response": {"id": "qvideo-exact"}}
            settled = {"model": model, "response": {"id": "qvideo-exact", "status": "succeeded",
                "content": {"video_url": "https://media.example/video.mp4"},
                "usage": {"completion_tokens": 40594}}, "metadata": {"cost_breakdown": {"source": "catalog"}}}
            with mock.patch.object(dataset, "wait_log", return_value=created), mock.patch.object(dataset, "wait_bill", return_value=settled):
                self.assertEqual(dataset.verify_video_logs(task, definition), (created, settled))
            wrong = copy.deepcopy(settled)
            wrong["response"]["content"]["video_url"] = "https://media.example/wrong.mp4"
            with mock.patch.object(dataset, "wait_log", return_value=created), mock.patch.object(dataset, "wait_bill", return_value=wrong):
                with self.assertRaisesRegex(AssertionError, "真实视频 URL"):
                    dataset.verify_video_logs(task, definition)


    def test_saved_media_log_verification_ignores_dynamic_chat_definitions(self):
        """目的：续跑媒体复核兼容没有固定 public_name 的动态聊天定义；前置真实清单及一条图片和视频记录，验证只分派媒体日志且不误读聊天配置，临时目录自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lambda _: None)
            video_model = "bytedance/doubao-seedance-2-0-mini-260615"
            image = {"kind": "image", "model": "gpt-image-2", "call_id": "image-id"}
            task = {"kind": "video", "model": video_model, "task_id": "video-id"}
            dataset.report["media_calls"] = [image]
            dataset.report["media_tasks"] = [task]
            with mock.patch.object(dataset, "verify_image_log") as verify_image, \
                 mock.patch.object(dataset, "verify_video_logs") as verify_video:
                dataset.verify_saved_media_logs()
            verify_image.assert_called_once_with(image)
            definition = next(row for row in dataset.data["models"] if row.get("public_name") == video_model)
            verify_video.assert_called_once_with(task, definition)


    def test_recover_media_log_ids_uses_exact_task_and_call_types(self):
        """目的：续跑只按精确模型、Task ID 与协议恢复媒体日志；前置一个 FAL 任务和模拟查询，验证创建及终态 ID 回填和 SQL 约束，临时报告自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lambda _: None)
            task = {"model": "bytedance/seedance-2.0/mini/text-to-video", "transport": "qiniu_fal_doubao_20",
                    "task_id": "qvideo-exact", "kind": "video"}
            dataset.report["media_tasks"] = [task]
            with mock.patch.object(DATABASE, "sql", return_value="create-exact|official-settlement:exact") as query:
                self.assertEqual(DATABASE.recover_media_log_ids(dataset), 1)
            self.assertEqual(task["create_call_id"], "create-exact")
            self.assertEqual(task["settlement_log_id"], "official-settlement:exact")
            statement = query.call_args.args[0]
            self.assertIn("qiniu_fal_doubao_20:create", statement)
            self.assertIn("qiniu_fal_doubao_20:status", statement)
            self.assertIn("qvideo-exact", statement)


    def test_video_log_conditions_validate_and_escape_exact_identity(self):
        """目的：共享查询条件支持三类协议且拒绝不完整输入；前置正常及含引号任务，验证精确 JSON 匹配、转义和失败路径，无外部数据需清理。"""
        task = {"model": "model'quoted", "task_id": "task'quoted"}
        for transport in ("qiniu_contents_generation", "qiniu_fal_doubao_20", "qiniu_fal_kling"):
            with self.subTest(transport=transport):
                scope, create, settled = DATABASE.video_log_conditions({**task, "transport": transport})
                self.assertIn("model''quoted", scope)
                self.assertIn("task''quoted", scope)
                self.assertIn("pg_input_is_valid", scope)
                self.assertEqual(create, f"u.call_type='{transport}:create'")
                self.assertIn("u.status='completed' AND u.task_settled", settled)
                self.assertIn("official-settlement:%", settled)
        for invalid in ({}, {**task, "transport": "unknown"},
                        {**task, "task_id": "", "transport": "qiniu_fal_kling"}):
            with self.subTest(invalid=invalid), self.assertRaisesRegex(ValueError, "Task ID"):
                DATABASE.video_log_conditions(invalid)


    def test_recover_media_log_ids_supports_original_and_missing_evidence(self):
        """目的：原任务完成后恢复相同创建与结算 ID，缺失证据明确失败；前置临时报告及模拟查询，验证正常、空任务列表和两侧 ID 缺失，不调用供应商，目录自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lambda _: None)
            with mock.patch.object(DATABASE, "sql") as query:
                self.assertEqual(DATABASE.recover_media_log_ids(dataset), 0)
                query.assert_not_called()
            task = {"model": "video-model", "transport": "qiniu_contents_generation", "task_id": "exact"}
            dataset.report["media_tasks"] = [task]
            with mock.patch.object(DATABASE, "sql", return_value="original|original"):
                self.assertEqual(DATABASE.recover_media_log_ids(dataset), 1)
            self.assertEqual(task["create_call_id"], task["settlement_log_id"])
            for row in ("", "original|", "|settlement"):
                with self.subTest(row=row), mock.patch.object(DATABASE, "sql", return_value=row):
                    with self.assertRaisesRegex(AssertionError, "无法恢复视频创建或终态日志"):
                        DATABASE.recover_media_log_ids(dataset)


    def test_video_logs_revalidate_completed_original_and_reject_pending(self):
        """目的：复验同一日志的原始 prompt 与完成证据，待处理不可算通过；前置 FAL 详情及临时目录，验证完整 URL、usage 及状态失败，模拟接口不写外部数据，目录自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lambda _: None)
            task = {"model": "video-model", "transport": "qiniu_fal_doubao_20", "task_id": "exact",
                    "create_call_id": "original", "settlement_log_id": "original",
                    "bypass_result": {"video_url": "https://example.invalid/video.mp4"}}
            detail = {"model": task["model"], "status": "completed",
                      "proxy_server_request": {"body": {"prompt": "A red apple on a white table, static camera, gentle natural light."}},
                      "response": {"request_id": "exact", "status": "COMPLETED", "result": {
                          "video": {"url": task["bypass_result"]["video_url"]}, "usage": {"completion_tokens": 100}}}}
            with mock.patch.object(dataset, "wait_log", return_value=detail), mock.patch.object(dataset, "wait_bill", return_value=detail):
                self.assertEqual(dataset.verify_video_logs(task, {}), (detail, detail))
            with mock.patch.object(dataset, "wait_log", return_value={**detail, "status": "polling"}):
                with self.assertRaisesRegex(AssertionError, "原任务日志尚未完成"):
                    dataset.verify_video_logs(task, {})


    def test_verify_database_includes_completed_video_costs(self):
        """目的：视频原日志的 completed 费用必须计入日报对账；前置模拟聚合及空媒体列表，验证查询包括两种成功状态和金额不一致失败，临时报告自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lambda _: None)
            counter = json.dumps({"requests": 1, "prompt_tokens": 0, "completion_tokens": 100})
            summary = json.dumps({"events": 1, "unique_calls": 1, "cost": 0.1})
            with mock.patch.object(DATABASE, "sql", side_effect=[summary + "\n0.1", counter + "\n" + counter, *(["0"] * 5)]) as query, \
                 mock.patch.object(DATABASE, "audit_response_usage", return_value={"successful_responses": 1, "with_usage": 1}):
                self.assertEqual(DATABASE.verify_database(dataset)["cost"], 0.1)
                self.assertIn("status IN ('success', 'completed')", query.call_args_list[0].args[0])
            with mock.patch.object(DATABASE, "sql", return_value=summary + "\n0.2"):
                with self.assertRaisesRegex(AssertionError, "每日聚合金额"):
                    DATABASE.verify_database(dataset)


    def test_video_creation_checkpoint_keeps_original_settlement_id(self):
        """目的：新视频检查点立即保存原结算 ID，避免续跑前无终态引用；前置 Ark 创建及终态响应，验证成功路径、缺失创建 ID 与缺失任务 ID 失败，模拟调用不收费，临时文件自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lambda _: None)
            definition = next(row for row in dataset.data["models"] if row["id"] == "qiniu-ark-seedance")
            result = {"id": "exact", "content": {"video_url": "https://example.invalid/video.mp4", "duration": 4},
                      "usage": {"completion_tokens": 100}}
            with mock.patch.object(dataset, "api", return_value=({"id": "exact"}, {"x-litellm-call-id": "original"})), \
                 mock.patch.object(dataset, "poll_video", return_value=result), \
                 mock.patch.object(dataset, "video_result_query", return_value=result):
                dataset.verify_video_model({"key": "fake"}, definition)
            task = dataset.report["media_tasks"][0]
            self.assertEqual(task["create_call_id"], "original")
            self.assertEqual(task["settlement_log_id"], "original")
            for answer, headers, message in (({"id": "exact"}, {}, "创建响应缺少请求日志 ID"),
                                              ({}, {"x-litellm-call-id": "original"}, "视频创建响应缺少任务 ID")):
                with self.subTest(message=message), mock.patch.object(dataset, "api", return_value=(answer, headers)):
                    with self.assertRaisesRegex(AssertionError, message):
                        dataset.verify_video_model({"key": "fake"}, definition)


    def test_image_bypass_result_is_saved_as_binary_and_compact_json(self):
        """目的：图片 bypass 结果可直接查看且日志不塞入大段base64；前置微型PNG响应，验证文件、摘要和紧凑JSON，临时目录自动清理。"""
        raw = b"\x89PNG\r\n\x1a\nreal-image-bytes"
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest())
            result = dataset.image_result("gpt-image-2", {"created": 1, "data": [{"b64_json": base64.b64encode(raw).decode()}]})
            self.assertEqual(Path(result["artifact"]).read_bytes(), raw)
            self.assertEqual(result["bytes"], len(raw))
            saved = Path(result["response_json"]).read_text()
            self.assertIn(result["sha256"], saved)
            self.assertNotIn(base64.b64encode(raw).decode(), saved)


    def test_image_bypass_result_recognizes_jpeg_artifact(self):
        """目的：真实图片JPEG响应以可直接预览的扩展名保存；前置微型JPEG字节，验证文件后缀和内容，临时目录自动清理。"""
        raw = b"\xff\xd8\xff\xe1real-jpeg-bytes"
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest())
            result = dataset.image_result("gpt-image-2", {"data": [{"b64_json": base64.b64encode(raw).decode()}]})
            self.assertEqual(Path(result["artifact"]).suffix, ".jpg")
            self.assertEqual(Path(result["artifact"]).read_bytes(), raw)


    def test_image_generation_retries_only_documented_temporary_failures(self):
        """目的：真实图片生成遇到供应商临时错误时可恢复且不虚报通过；前置两次502后成功，验证三次独立调用和等待日志，使用模拟响应无需清理外部数据。"""
        with tempfile.TemporaryDirectory() as directory:
            lines = []
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lines.append)
            definition = next(row for row in dataset.data["models"] if row["id"] == "fenno-image")
            responses = [
                ({"error": {"message": "temporary upstream failure"}}, {}, 502),
                ({"error": {"message": "temporary upstream failure"}}, {}, 502),
                ({"data": [{"url": "https://example.invalid/image.png"}]}, {"x-litellm-call-id": "call-1"}, 200),
            ]
            with mock.patch.object(dataset, "api", side_effect=responses) as api, mock.patch("acceptance.support.time.sleep") as sleep:
                answer, headers = dataset.image_generation(definition, {"key": "secret"}, delay=1)
            self.assertEqual(answer["data"][0]["url"], "https://example.invalid/image.png")
            self.assertEqual(headers["x-litellm-call-id"], "call-1")
            self.assertEqual(api.call_count, 3)
            self.assertEqual(sleep.call_count, 2)
            output = "\n".join(lines)
            self.assertIn("❌ [gpt-image-2 / bypass_openai_image_generation / 第 1/3 次真实图片生成]", output)
            self.assertIn("✅ [gpt-image-2 / bypass_openai_image_generation / 第 3/3 次真实图片生成响应]", output)


    def test_image_generation_does_not_retry_non_temporary_failure(self):
        """目的：真实图片生成不能掩盖未声明的错误状态；前置供应商400响应，验证接口契约立即抛错且不等待，不访问或清理外部数据。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest())
            definition = next(row for row in dataset.data["models"] if row["id"] == "fenno-image")
            with mock.patch.object(dataset, "api", side_effect=AssertionError("状态 400")) as api, \
                    mock.patch("acceptance.support.time.sleep") as sleep:
                with self.assertRaisesRegex(AssertionError, "状态 400"):
                    dataset.image_generation(definition, {"key": "secret"}, delay=1)
            self.assertEqual(api.call_count, 1)
            sleep.assert_not_called()


    def test_record_call_replaces_same_key_and_persists_immediately(self):
        """目的：长批次中每把密钥成功后立即形成唯一检查点；前置同一密钥两次成功账单，验证第二次替换第一次且报告立刻落盘，临时目录自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest())
            key = {"token_id": "key-1", "profile": "inherit"}
            dataset.record_call(key, "dataset-a1-1", "call-1", "gpt-5.6-sol",
                                {"prompt_tokens": 8, "completion_tokens": 1}, 0.000010)
            dataset.record_call(key, "dataset-a1-2", "call-2", "gpt-5.6-sol",
                                {"prompt_tokens": 9, "completion_tokens": 2}, 0.000013)
            saved = json.loads((Path(directory) / "report.json").read_text())
            self.assertEqual(len(saved["calls"]), 1)
            self.assertEqual(saved["calls"][0]["call_id"], "call-2")
            self.assertEqual(saved["calls"][0]["model"], "gpt-5.6-sol")


    def test_fal_video_poll_accepts_202_until_completed(self):
        """目的：FAL 队列等待态的HTTP 202不会误判失败；前置一次202等待和一次200完成，验证轮询继续并读取最终结果，模拟调用不产生外部任务。"""
        with tempfile.TemporaryDirectory() as directory:
            lines = []
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lines.append)
            definition = next(row for row in dataset.data["models"] if row["id"] == "qiniu-fal-seedance")
            responses = [
                ({"status": "IN_PROGRESS"}, {}, 202),
                ({"status": "COMPLETED"}, {}, 200),
                ({"result": {"video": {"url": "https://example.invalid/video.mp4"}}}, {}, 200),
            ]
            with mock.patch.object(dataset, "api", side_effect=responses) as api, mock.patch("acceptance.support.time.sleep"):
                result = dataset.poll_video({"key": "secret"}, definition, "task-1",
                                            "/task-1/status", "/task-1", "COMPLETED", {"failed"})
            self.assertEqual(result["video"]["url"], "https://example.invalid/video.mp4")
            self.assertEqual(api.call_count, 3)
            self.assertIn("HTTP=202，状态=IN_PROGRESS", "\n".join(lines))


    def test_video_result_query_retries_idempotent_temporary_failure(self):
        """目的：视频终态GET遇到临时502后仍可完成去重核对；前置一次502后200，验证重试、等待和成功结果，模拟调用不产生外部任务。"""
        with tempfile.TemporaryDirectory() as directory:
            lines = []
            dataset = Dataset("http://127.0.0.1", directory, load_manifest(), logger=lines.append)
            definition = next(row for row in dataset.data["models"] if row["id"] == "qiniu-fal-seedance")
            responses = [({"error": {"message": "upstream timeout"}}, {}, 502),
                         ({"video": {"url": "https://example.invalid/video.mp4"}}, {}, 200)]
            with mock.patch.object(dataset, "api", side_effect=responses) as api, mock.patch("acceptance.support.time.sleep") as sleep:
                result = dataset.video_result_query({"key": "secret"}, definition, "/result", "终态查询", delay=1)
            self.assertEqual(result["video"]["url"], "https://example.invalid/video.mp4")
            self.assertEqual(api.call_count, 2)
            sleep.assert_called_once_with(1)
            self.assertIn("❌ [bytedance/seedance-2.0/mini/text-to-video / qiniu_fal_doubao_20 / 终态查询第 1/3 次]", "\n".join(lines))



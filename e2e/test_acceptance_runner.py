"""真实验收 runner 模块的单元测试；各用例独立管理夹具与清理。"""
from contextlib import redirect_stdout
import io
import json
from pathlib import Path
import tempfile
import subprocess
import unittest
from unittest import mock
from acceptance import Dataset, load_manifest
from acceptance import runner_runtime as RUNNER
from acceptance import runner_browser as BROWSER, runner_shared as SHARED


class AcceptanceTests(unittest.TestCase):
    """验证所属业务模块的正常、边界和失败路径，资源由各测试自行隔离清理。"""

    def test_category_make_and_cli_contract(self):
        """目的：make e2e 分类只发一个选中命令，未知分类在执行前失败；前置无凭据，验证单类、多类、全量、列表及 seed 禁用分类；只运行 dry-run/参数校验，不启动服务，无数据需清理。"""
        root = Path(__file__).parents[1]
        for categories in ([], ["limits"], ["limits", "permissions"], ["regression"], list(RUNNER.E2E_CATEGORIES)):
            with self.subTest(categories=categories):
                result = subprocess.run(["make", "-n", "e2e", *categories], cwd=root, capture_output=True, text=True)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(result.stdout.count("scripts/e2e.py"), 1)
                for category in categories:
                    self.assertIn("--category " + category, result.stdout)
                self.assertEqual(result.stdout.count("--category "), len(categories))
        invalid = subprocess.run(["make", "-n", "e2e", "limtis"], cwd=root, capture_output=True, text=True)
        self.assertNotEqual(invalid.returncode, 0)
        self.assertNotIn("scripts/e2e.py", invalid.stdout, "拼错分类不能意外运行全套付费用例")
        listed = subprocess.run(["make", "e2e-list"], cwd=root, capture_output=True, text=True)
        self.assertEqual(listed.returncode, 0, listed.stderr)
        self.assertEqual(tuple(listed.stdout.splitlines()[2:]), RUNNER.E2E_CATEGORIES)
        for args in (["--category", "limtis"], ["--category", "limits"],
                     ["--shared-target", "--phase", "seed", "--category", "limits"],
                     ["--shared-target", "--phase", "verify", "--category", "regression"]):
            invalid = subprocess.run(["python3", "scripts/e2e.py", *args], cwd=root, capture_output=True, text=True)
            self.assertEqual(invalid.returncode, 2, invalid.stderr)


    def test_selected_categories_skip_unrelated_calls_and_validate_plan(self):
        """目的：所有分类过滤与模型子类均只执行选中函数；前置内存夹具，验证空/未知拒绝、回调失败及未执行项不进入报告；不访问供应商，临时报告自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lambda _: None)
            dataset.representative = True
            dataset.state["keys"] = [dict(token_id=str(i), profile=profile, team_index=team, call_model="unit-model")
                for i, (profile, team) in enumerate([("inherit", 0), ("inherit", 1), ("fenno-only", 0), ("qiniu-only", 0)])]
            dataset.start_checklist(True, list(RUNNER.E2E_CATEGORIES))
            self.assertEqual(tuple(row["id"] for row in dataset.checklist.rows), RUNNER.E2E_CATEGORIES)
            for invalid in ([], ["unknown"], ["limits", "unknown"]):
                with self.assertRaisesRegex(ValueError, "E2E 分类"):
                    dataset.start_checklist(True, invalid)
            dataset.start_checklist(False, ["limits"])
            skipped = mock.Mock(side_effect=RuntimeError("must not run"))
            self.assertIsNone(dataset.run_case("permissions", skipped))
            skipped.assert_not_called()
            dataset.verify_limits = mock.Mock()
            dataset.codex_conversation = mock.Mock(side_effect=RuntimeError("unselected GPT"))
            dataset.verify_glm_model = mock.Mock(side_effect=RuntimeError("unselected GLM"))
            dataset.verify_image_model = mock.Mock(side_effect=RuntimeError("unselected image"))
            dataset.verify_video_model = mock.Mock(side_effect=RuntimeError("unselected video"))
            dataset.verify()
            dataset.verify_limits.assert_called_once_with(dataset.state["keys"][0])
            for name in ("codex_conversation", "verify_glm_model", "verify_image_model", "verify_video_model"):
                getattr(dataset, name).assert_not_called()
            self.assertEqual([(row["id"], row["status"]) for row in dataset.checklist.rows], [("limits", "passed")])
            dataset.start_checklist(False, ["limits"])
            dataset.verify_limits.side_effect = RuntimeError("selected failure")
            with self.assertRaisesRegex(RuntimeError, "selected failure"):
                dataset.verify()
            self.assertEqual(dataset.checklist.rows[0]["status"], "failed")


    def test_business_browser_reads_private_report_and_rejects_missing_or_failed_results(self):
        """目的：排队浏览器必须读取锁内保存的专属报告；前置模拟编排返回成功、失败或缺报告，验证路径和错误响应；临时目录清理，不启动外部服务。"""
        for mode in ["passed", "failed", "missing"]:
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as directory:
                report_dir = Path(directory)

                def command(command, logger, env):
                    """用途：模拟浏览器退出前保存专属报告；参数为命令、日志器和环境，无返回；供编排测试调用，仅写临时证据。"""
                    path = Path(env["E2E_BROWSER_REPORT_DIR"])
                    self.assertEqual(path, (report_dir / "business-browser").resolve())
                    if mode != "missing":
                        path.mkdir()
                        (path / "results.json").write_text(json.dumps({"stats": {
                            "expected": 19 if mode == "passed" else 18,
                            "unexpected": 0 if mode == "passed" else 1, "flaky": 0, "skipped": 0}}))

                with mock.patch.object(BROWSER, "command_stream", side_effect=command):
                    if mode == "passed":
                        result = BROWSER.run_business_browser(report_dir, mock.Mock())
                        self.assertEqual(result["cases"], 19)
                    else:
                        with self.assertRaisesRegex(AssertionError, "缺少本轮浏览器报告" if mode == "missing" else "未全部执行通过"):
                            BROWSER.run_business_browser(report_dir, mock.Mock())
                self.assertEqual((report_dir / "business-browser-results.json").exists(), mode != "missing")


    def test_shared_target_requires_exact_urls(self):
        """目的：破坏性清理只能命中用户指定目标；前置固定白名单，验证精确通过和数据库/Redis偏差失败，不连接或清理外部数据。"""
        RUNNER.validate_shared_target(RUNNER.SHARED_DATABASE_URL, RUNNER.SHARED_REDIS_URL)
        for database_url, redis_url in (
                (RUNNER.SHARED_DATABASE_URL.replace("/xhub?", "/postgres?"), RUNNER.SHARED_REDIS_URL),
                (RUNNER.SHARED_DATABASE_URL, "redis://127.0.0.1:6379/0")):
            with self.subTest(database_url=database_url, redis_url=redis_url), self.assertRaises(ValueError):
                RUNNER.validate_shared_target(database_url, redis_url)


    def test_shared_config_uses_public_database_and_redis_one(self):
        """目的：共享构建配置不带隔离schema且固定Redis DB 1；前置测试密码，验证配置契约，不写文件且无需清理。"""
        config = RUNNER.shared_config("test-password")["general_settings"]
        self.assertEqual(config["database_url"], RUNNER.SHARED_DATABASE_URL)
        self.assertEqual(config["redis_url"], RUNNER.SHARED_REDIS_URL)
        self.assertNotIn("search_path", config["database_url"])
        self.assertEqual(config["admin_password"], "test-password")


    def test_provider_validation_happens_without_database_access(self):
        """目的：缺少真实供应商密钥时必须在清库前失败；前置空环境，验证缺失和非法URL边界，本测试不调用重置函数。"""
        data = load_manifest()
        with self.assertRaisesRegex(ValueError, data["providers"][0]["key_env"]):
            RUNNER.validate_providers(data, {})
        environ = {provider["key_env"]: "secret" for provider in data["providers"]}
        environ[data["providers"][0]["base_env"]] = "http://insecure.invalid"
        with self.assertRaisesRegex(ValueError, "HTTPS"):
            RUNNER.validate_providers(data, environ)


    def test_seed_provider_validation_requires_no_credentials_but_validates_urls(self):
        """目的：建数允许完全没有供应商密钥，但拒绝非法地址；前置清单与空环境，验证默认地址、空密钥边界及非法 URL，无网络或数据需要清理。"""
        data = load_manifest()
        RUNNER.validate_providers(data, {}, require_credentials=False)
        with self.assertRaisesRegex(ValueError, "HTTPS"):
            RUNNER.validate_providers(data, {data["providers"][0]["base_env"]: "http://invalid"},
                                      require_credentials=False)


    def test_shared_seed_never_starts_observer_or_verification(self):
        """目的：共享 testdata 编排只启动网关和建数，无真实上游观察器或验收；前置替代进程及建数器，验证报告、零调用提示和进程退出，临时目录自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            args = type("Args", (), {"phase": "seed", "directory": directory})()
            dataset = mock.Mock()
            dataset.report = {}
            dataset.state = {name: [] for name in ("organizations", "teams", "users", "projects", "keys")}
            with mock.patch.object(SHARED, "reset_shared_target"), \
                    mock.patch.object(SHARED, "command"), \
                    mock.patch.object(SHARED, "ready"), \
                    mock.patch.object(RUNNER.subprocess, "Popen") as process, \
                    mock.patch.object(SHARED.module, "Dataset", return_value=dataset), \
                    mock.patch.object(SHARED.observer_module, "Observer") as observer, \
                    mock.patch.dict(RUNNER.os.environ, {}, clear=True), redirect_stdout(io.StringIO()):
                self.assertEqual(SHARED.shared_main_locked(args, load_manifest()), 0)
            dataset.seed.assert_called_once_with()
            dataset.verify.assert_not_called()
            observer.assert_not_called()
            process.return_value.terminate.assert_called_once_with()
            self.assertEqual(dataset.report["status"], "seeded")
            self.assertIn("模型调用=0", (Path(directory) / "testdata.log").read_text())


    def test_run_logger_writes_identical_timestamped_terminal_and_file_lines(self):
        """目的：完整过程日志同时显示并持久化；前置临时日志，验证多行均带时间且正文一致，临时目录自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "e2e.log"
            output = io.StringIO()
            logger = RUNNER.RunLogger(path)
            with redirect_stdout(output):
                logger.log("第一步\n第二步")
            self.assertEqual(output.getvalue(), path.read_text())
            self.assertRegex(path.read_text(), r"(?m)^\[\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\] 第一步$")
            self.assertIn("] 第二步\n", path.read_text())


    def test_shared_run_lock_rejects_concurrent_process(self):
        """目的：共享 xhub 构建与验收不能并发；前置临时锁文件，验证第二个持锁者立即失败，退出上下文后自动释放。"""
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "shared.lock"
            with RUNNER.shared_run_lock(path):
                with self.assertRaisesRegex(RuntimeError, "已有 make testdata"):
                    with RUNNER.shared_run_lock(path):
                        self.fail("竞争锁不应成功")
            with RUNNER.shared_run_lock(path):
                self.assertTrue(path.exists())


    def test_restore_supplier_addresses_uses_deployment_provider_ids(self):
        """目的：退出清理恢复仍存在的聊天部署并覆盖旧诊断；前置模拟部署和接口，验证按供应商还原地址、成功日志及未知供应商失败记录，不访问真实服务或数据库。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest())
            dataset.admin = "admin-token"
            dataset.state["deployments"] = [
                {"id": "deployment-a", "provider": "fennoai", "kind": "chat", "observed": True},
                {"id": "deployment-b", "provider": "qiniu", "kind": "chat", "observed": True},
                {"id": "deployment-media", "provider": "qiniu", "kind": "video", "observed": False},
                {"id": "deployment-unknown", "provider": "missing", "kind": "chat", "observed": True},
            ]
            calls = []
            lines = []
            dataset.logger = lines.append
            dataset.report["retention_error"] = "上一轮遗留错误"
            dataset.api = lambda route, body: calls.append((route, body))
            SHARED.restore_supplier_addresses(dataset, load_manifest())
            self.assertEqual([body["model_info"]["id"] for _, body in calls], ["deployment-a", "deployment-b"])
            self.assertEqual(calls[0][1]["litellm_params"]["api_base"], "https://api.fenno.ai")
            self.assertEqual(calls[1][1]["litellm_params"]["api_base"], "https://api.modelink.ai")
            self.assertIn("未知供应商", dataset.report["retention_error"])
            self.assertNotIn("上一轮遗留错误", dataset.report["retention_error"])
            self.assertEqual(sum("✅" in line and "恢复供应商地址" in line for line in lines), 2)


    def test_restore_supplier_addresses_clears_stale_error_after_success(self):
        """目的：上一轮恢复错误不得污染本轮成功报告；前置一个有效聊天部署和旧 retention_error，验证恢复成功后字段被删除并输出打勾日志，不访问真实服务或数据库。"""
        with tempfile.TemporaryDirectory() as directory:
            lines = []
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lines.append)
            dataset.admin = "admin-token"
            dataset.report["retention_error"] = "旧错误"
            dataset.state["deployments"] = [
                {"id": "deployment-a", "public_name": "gpt-5.6-sol", "provider": "fennoai",
                 "kind": "chat", "observed": True},
            ]
            dataset.api = lambda route, body: ({}, {})
            SHARED.restore_supplier_addresses(dataset, load_manifest())
            self.assertNotIn("retention_error", dataset.report)
            self.assertIn("✅ [gpt-5.6-sol / bypass_openai_chat / 恢复供应商地址]", "\n".join(lines))


    def test_restore_supplier_addresses_ignores_deleted_temporary_deployment(self):
        """目的：异常退出时已删除的生命周期部署不得造成恢复误报；前置一个带 temporary 标记且更新返回404的聊天部署，验证报告无 retention_error 并记录无需恢复，不访问真实服务或数据库。"""
        with tempfile.TemporaryDirectory() as directory:
            lines = []
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lines.append)
            dataset.admin = "admin-token"
            dataset.state["deployments"] = [
                {"id": "deployment-deleted", "public_name": "qwen/temporary-real-model",
                 "provider": "qiniu", "kind": "chat", "observed": True, "temporary": True},
            ]
            dataset.api = mock.Mock(side_effect=AssertionError("POST /model/update 状态 404，预期 200"))
            SHARED.restore_supplier_addresses(dataset, load_manifest())
            self.assertNotIn("retention_error", dataset.report)
            self.assertIn("临时部署 deployment-deleted 已删除，无需恢复", "\n".join(lines))


    def test_attach_chat_observer_leaves_native_media_on_supplier_addresses(self):
        """目的：验收观察器只接管聊天请求，防止图片和视频被改发聊天端点；前置聊天、图片和视频部署，验证仅 observed 部署更新，无网络或持久化数据需清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest())
            dataset.state["deployments"] = [
                {"id": "chat-a", "provider": "fennoai", "kind": "chat", "observed": True, "endpoint_types": ["chat"]},
                {"id": "image-a", "provider": "fennoai", "kind": "image", "observed": False},
                {"id": "video-a", "provider": "qiniu", "kind": "video", "observed": False},
            ]
            calls = []
            dataset.api = lambda route, body: calls.append((route, body))
            observer = type("ObserverStub", (), {"base": "http://127.0.0.1:4567"})()
            SHARED.attach_chat_observer(dataset, observer)
            self.assertEqual(len(calls), 1)
            self.assertEqual(calls[0][1]["model_info"]["id"], "chat-a")
            self.assertEqual(calls[0][1]["model_info"], {"id": "chat-a"})
            self.assertEqual(dataset.state["deployments"][0]["endpoint_types"], ["chat"])
            self.assertEqual(calls[0][1]["litellm_params"]["api_base"], "http://127.0.0.1:4567/fennoai")


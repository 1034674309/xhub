"""统一验收入口与业务模块边界单测；只使用临时目录和内存数据，不启动外部依赖。"""
import contextlib
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

from acceptance import cli, Dataset


class AcceptanceEntryTests(unittest.TestCase):
    """验证唯一入口、合法分发和失败证据；资源通过临时目录自动清理。"""

    def test_single_entry_and_business_module_ownership(self):
        """目的：阻止恢复双入口或把业务挤回总文件；前置为仓库文件，验证旧文件删除、入口短小及主要方法归属；只读，无清理需求。"""
        root = Path(__file__).resolve().parents[1]
        self.assertFalse((root / "e2e/real_dataset.py").exists())
        self.assertFalse((root / "scripts/e2e-real-dataset.py").exists())
        self.assertLess(len((root / "scripts/e2e.py").read_text().splitlines()), 30)
        methods = {
            "seed": "seed", "verify_limits": "limits",
            "verify_permissions": "permissions", "codex_conversation": "chat",
            "verify_image_model": "media", "wait_bill": "billing",
        }
        for method, module in methods.items():
            with self.subTest(method=method):
                self.assertEqual(getattr(Dataset, method).__module__, "acceptance." + module)

    def test_cli_dispatch_and_rejected_lifecycle_combinations(self):
        """目的：统一入口不混用调用方与 runner 的资源权限；前置模拟各生命周期，验证 seed/verify 分发、分类去重和错误组合；不联网、不建数，无清理需求。"""
        with mock.patch.object(cli, "run_existing", return_value=0) as existing, \
             mock.patch.object(cli, "shared_main", return_value=0) as shared, \
             mock.patch.object(cli, "run_isolated", return_value=0) as isolated, \
             mock.patch.object(cli.module, "load_manifest", return_value={}) as manifest:
            for seed in (False, True):
                args = ["e2e", "--gateway", "http://localhost:1"] + (["--seed-only"] if seed else [])
                with mock.patch("sys.argv", args):
                    self.assertEqual(cli.main(), 0)
                self.assertEqual(existing.call_args.args[0].seed_only, seed)
            with mock.patch("sys.argv", ["e2e", "--shared-target", "--phase", "verify",
                        "--category", "limits", "--category", "limits", "--category", "permissions", "--category", "profile-isolation", "--category", "cache-weight", "--category", "fallback"]):
                self.assertEqual(cli.main(), 0)
                self.assertEqual(shared.call_args.args[0].category, ["limits", "permissions", "profile-isolation", "cache-weight", "fallback"])
            with mock.patch("sys.argv", ["e2e"]):
                self.assertEqual(cli.main(), 0)
                isolated.assert_called_once()
            for incompatible in (["--shared-target"], ["--phase", "verify"],
                                 ["--category", "limits"], ["--with-regression"],
                                 ["--resume"], ["--keep-data"]):
                with self.subTest(incompatible=incompatible), \
                     mock.patch("sys.argv", ["e2e", "--gateway", "http://localhost:1", *incompatible]), \
                     contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit) as error:
                    cli.main()
                self.assertEqual(error.exception.code, 2)
            with mock.patch("sys.argv", ["e2e", "--seed-only"]), \
                 contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit) as error:
                cli.main()
            self.assertEqual(error.exception.code, 2)
            manifest.reset_mock()
            with mock.patch("sys.argv", ["e2e", "--list-categories"]), contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(cli.main(), 0)
            manifest.assert_not_called()

    def test_existing_gateway_seed_verify_and_failure_report(self):
        """目的：现成网关模式仅 seed-only 跳过验收且失败保存证据；前置临时目录与模拟 Dataset，覆盖建数/验收异常；不修改外部数据，目录自动清理。"""
        for mode in ("seed-only", "verify", "seed-error", "verify-error"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as directory:
                dataset = Dataset("http://localhost:1", directory, {"id": "unit"})
                dataset.seed = mock.Mock(side_effect=RuntimeError("seed rejected") if mode == "seed-error" else None)
                dataset.verify = mock.Mock(side_effect=RuntimeError("verify rejected") if mode == "verify-error" else None)
                args = mock.Mock(gateway="http://localhost:1", directory=directory, seed_only=mode == "seed-only")
                with mock.patch.object(cli, "Dataset", return_value=dataset), \
                     mock.patch.object(cli.module, "load_manifest", return_value={}), contextlib.redirect_stdout(io.StringIO()):
                    if mode.endswith("error"):
                        with self.assertRaisesRegex(SystemExit, "report.json"):
                            cli.run_existing(args)
                        report = json.loads((Path(directory) / "report.json").read_text())
                        self.assertEqual(report["status"], "failed")
                        self.assertIn("rejected", report["error"])
                    else:
                        self.assertEqual(cli.run_existing(args), 0)
                dataset.seed.assert_called_once()
                if mode in ("seed-only", "seed-error"):
                    dataset.verify.assert_not_called()
                else:
                    dataset.verify.assert_called_once()

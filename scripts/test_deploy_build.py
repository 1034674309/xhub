"""部署构建辅助函数测试；仅使用临时文件，不编译镜像或访问网络。"""
import os
import re
from pathlib import Path
import subprocess
import tempfile
import unittest

LIBRARY = Path(__file__).resolve().parent.parent / "deploy" / "build-lib.sh"


class ConsoleStagingTests(unittest.TestCase):
    """验证两种 Next 追踪目录布局、资源位置、启动入口和无效产物保护；所有产物自动清理。"""

    def stage(self, root, *, destination=None):
        """用途：调用打包函数；参数为临时根目录及可选输出目录，返回进程结果。

        供目录布局和失败测试使用，输出仅写入测试的临时目录；调用失败由返回码表达。
        """
        return subprocess.run(
            ["bash", "-e", "-c", 'source "$1"; shift; stage_console "$@"',
             "console-stage-test", str(LIBRARY), str(root / "standalone"),
             str(root / "static"), str(root / "public"), str(destination or root / "stage")],
            capture_output=True, text=True,
        )

    def fixture(self, root, *, nested=False, public=True):
        """用途：创建两种可启动的产物夹具；参数指定根目录、嵌套布局及 public 是否存在。

        返回 None；供正常和边界测试调用，依赖测试临时目录清理，不调用网络或真实后台。
        """
        app = root / "standalone" / "frontend" if nested else root / "standalone"
        app.mkdir(parents=True)
        (app / "server.js").write_text(
            'const fs = require("node:fs"); const path = require("node:path");\n'
            'const asset = fs.readFileSync(path.join(__dirname, ".next/static/asset.txt"), "utf8");\n'
            'console.log(require("fixture-dependency") + ":" + asset);\n'
        )
        dependency = root / "standalone/node_modules/fixture-dependency"
        dependency.mkdir(parents=True)
        (dependency / "index.js").write_text('module.exports = "ready";\n')
        (root / "static").mkdir()
        (root / "static/asset.txt").write_text("static")
        if public:
            (root / "public").mkdir()
            (root / "public/favicon.ico").write_text("icon")

    def test_both_layouts_start_and_keep_assets_with_the_application(self):
        """前置为根入口或嵌套入口；验证 Node 真正执行、依赖解析和资源位置；临时目录自动清理。"""
        for nested in (False, True):
            with self.subTest(nested=nested), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                self.fixture(root, nested=nested)
                result = self.stage(root)
                self.assertEqual(result.returncode, 0, result.stderr)
                app = root / "stage/frontend" if nested else root / "stage"
                self.assertEqual((app / "public/favicon.ico").read_text(), "icon")
                started = subprocess.run(
                    ["node", str(root / "stage/server.js")], cwd=root,
                    capture_output=True, text=True,
                )
                self.assertEqual(started.returncode, 0, started.stderr)
                self.assertEqual(started.stdout.strip(), "ready:static")

    def test_public_directory_is_optional(self):
        """边界输入不含 public 时仍可打包静态资源和入口；不生成多余 public，临时目录自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.fixture(root, nested=True, public=False)
            result = self.stage(root)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertFalse((root / "stage/frontend/public").exists())

    def test_missing_entry_or_static_assets_preserve_previous_stage(self):
        """前置有旧产物，新产物缺入口或静态目录；验证明确失败且旧产物保留，临时目录自动清理。"""
        for missing in ("server", "static"):
            with self.subTest(missing=missing), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                self.fixture(root)
                (root / "stage").mkdir()
                (root / "stage/previous.txt").write_text("previous")
                if missing == "server":
                    (root / "standalone/server.js").unlink()
                else:
                    (root / "static/asset.txt").unlink()
                    (root / "static").rmdir()
                result = self.stage(root)
                self.assertEqual(result.returncode, 1)
                self.assertIn("not produced", result.stderr)
                self.assertEqual((root / "stage/previous.txt").read_text(), "previous")

    def test_root_destination_is_rejected_before_cleanup(self):
        """危险边界参数为根目录时应在任何清理前拒绝；验证错误与返回码，临时夹具自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.fixture(root)
            result = self.stage(root, destination=Path("/"))
            self.assertEqual(result.returncode, 1)
            self.assertIn("invalid console staging directory", result.stderr)


class CertificateDiscoveryTests(unittest.TestCase):
    """验证不同主机的 CA 路径选择、显式覆盖和失败结果。"""

    def discover(self, candidates, *, override=None):
        """调用证书发现函数。

        candidates 是测试候选路径，override 可设置 XHUB_CA_CERT_FILE；返回完成进程。
        调用方用它验证正常、边界和失败输入；函数在临时环境运行，无持久副作用。
        """
        env = dict(os.environ)
        if override is None:
            env.pop("XHUB_CA_CERT_FILE", None)
        else:
            env["XHUB_CA_CERT_FILE"] = override
        command = 'source "$1"; shift; find_ca_certificate "$@"'
        return subprocess.run(
            ["bash", "-c", command, "build-lib-test", str(LIBRARY), *map(str, candidates)],
            env=env, capture_output=True, text=True,
        )

    def test_selects_first_readable_nonempty_linux_candidate(self):
        """正常场景跳过缺失路径并选择首个非空 Linux 风格文件；临时目录自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            bundle = root / "etc/ssl/certs/ca-certificates.crt"
            bundle.parent.mkdir(parents=True)
            bundle.write_text("fixture certificate\n")
            result = self.discover([root / "missing.pem", bundle])
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout.strip(), str(bundle))

    def test_empty_bundle_is_rejected(self):
        """边界场景中文件可读但为空时拒绝使用，避免打包无效信任库；临时文件自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            bundle = Path(directory) / "empty.crt"
            bundle.touch()
            result = self.discover([bundle])
            self.assertEqual(result.returncode, 1)
            self.assertEqual(result.stdout, "")

    def test_missing_candidates_fail_without_output(self):
        """失败场景中所有候选均缺失时返回 1 且不伪造路径；临时目录自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            result = self.discover([root / "missing-a", root / "missing-b"])
            self.assertEqual(result.returncode, 1)
            self.assertEqual(result.stdout, "")

    def test_environment_override_is_used_by_default_search(self):
        """显式覆盖路径应优先用于无参数调用，支持非常见发行版；临时文件自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            bundle = Path(directory) / "custom.crt"
            bundle.write_text("custom certificate\n")
            env = dict(os.environ, XHUB_CA_CERT_FILE=str(bundle))
            result = subprocess.run(
                ["bash", "-c", f'source "{LIBRARY}"; find_ca_certificate'],
                env=env, capture_output=True, text=True,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout.strip(), str(bundle))


if __name__ == "__main__":
    unittest.main()


class ConsoleBuildAddressTests(unittest.TestCase):
    """验证真实构建环境赋值：默认可移植，显式覆盖保留，构建失败不被吞掉；无持久数据。"""

    def test_browser_gateway_address_default_override_and_failure(self):
        """前置真实 build.sh 中的 npm 构建语句；执行空/显式地址及失败路径，临时命令夹具自动清理。"""
        script = (LIBRARY.parent / "build.sh").read_text()
        command = re.search(r"  NEXT_TELEMETRY_DISABLED=1 .*?npm run build", script, re.S).group(0)
        with tempfile.TemporaryDirectory() as directory:
            npm = Path(directory) / "npm"
            npm.write_text('#!/bin/sh\nprintf "%s" "$NEXT_PUBLIC_BASE_URL"\nexit "$BUILD_STATUS"\n')
            npm.chmod(0o755)
            for value, status in [(None, 0), ("https://api.example.com", 0), (None, 7)]:
                environment = dict(os.environ, PATH=directory + ":" + os.environ["PATH"], BUILD_STATUS=str(status))
                environment.pop("NEXT_PUBLIC_BASE_URL", None)
                if value is not None:
                    environment["NEXT_PUBLIC_BASE_URL"] = value
                result = subprocess.run(["bash", "-e", "-c", command], env=environment, capture_output=True, text=True)
                self.assertEqual(result.returncode, status, result.stderr)
                self.assertEqual(result.stdout, value or "", "默认镜像不能固化访问者的 localhost")

"""部署构建辅助函数测试；仅使用临时文件，不编译镜像或访问网络。"""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

LIBRARY = Path(__file__).resolve().parent.parent / "deploy" / "build-lib.sh"


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

"""隔离网关配置测试，验证可选 Redis，不访问数据库或供应商。"""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


class GatewayConfigTests(unittest.TestCase):
    """验证浏览器夹具配置的可选异步落库路径。"""

    def test_optional_redis(self):
        """目的：隔离环境生成配置，验证未配置、空值及专属 Redis 的原样传递；临时目录自动清理，无外部副作用。"""
        for value in (None, "", "redis://127.0.0.1:6397/1"):
            with self.subTest(redis=value), tempfile.TemporaryDirectory() as directory:
                env = {"PATH": os.environ.get("PATH", ""), "E2E_UP_PORT": "4137",
                       "E2E_MASTER_KEY": "test-only-key", "E2E_SCHEMA": "test_config",
                       "E2E_RUN_DIR": directory}
                if value is not None:
                    env["E2E_REDIS_URL"] = value
                subprocess.run([sys.executable, str(Path(__file__).with_name("config.py"))],
                               env=env, check=True, capture_output=True)
                settings = json.loads((Path(directory) / "c.yaml").read_text())["general_settings"]
                if value:
                    self.assertEqual(settings["redis_url"], value, "专属 Redis 配置必须原样保留")
                else:
                    self.assertNotIn("redis_url", settings, "默认配置不能依赖共享 Redis")

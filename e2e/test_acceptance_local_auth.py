"""本地真实网关认证配置的确定性测试。"""
from pathlib import Path
import tempfile
import unittest
from acceptance.local_auth import local_admin_credentials


class LocalAuthTests(unittest.TestCase):
    """用途：验证本地凭据选择及远程边界，不连接网关，临时配置自动删除。"""

    def test_config_environment_precedence_and_remote_boundary(self):
        """目的：默认本地配置可登录，显式变量优先，远程不带本地秘密；前置临时配置，验证引用、禁用及异常配置；临时目录清理。"""
        with tempfile.TemporaryDirectory() as directory:
            config = Path(directory, "configs/config.yaml")
            config.parent.mkdir()
            # 使用实际换行生成简单 YAML，避免测试依赖外部解析库。
            config.write_text("general_settings:\n  admin_email: admin@example.com\n  admin_password: os.environ/LOCAL_PASSWORD\n")
            auth = local_admin_credentials("http://127.0.0.1:4000", directory, {"LOCAL_PASSWORD": "secret"})
            self.assertEqual(auth, {"E2E_DATASET_ADMIN": "admin@example.com", "E2E_DATASET_PASSWORD": "secret"})
            self.assertEqual(local_admin_credentials("https://example.com", directory, {"LOCAL_PASSWORD": "secret"}), {})
            self.assertEqual(local_admin_credentials("http://127.0.0.1:5000", directory, {}), {})
            self.assertEqual(local_admin_credentials("http://localhost:4000", directory,
                {"E2E_DATASET_PASSWORD": "explicit-password"}),
                {"E2E_DATASET_ADMIN": "admin@xhub.local", "E2E_DATASET_PASSWORD": "explicit-password"})
            explicit = {"E2E_DATASET_ADMIN": "override", "E2E_DATASET_PASSWORD": "override-password"}
            self.assertEqual(local_admin_credentials("http://localhost:4000", directory, explicit), explicit)
            self.assertEqual(local_admin_credentials("http://localhost:4000", directory, {}), {})
            config.write_text("general_settings:\n  admin_password: secret\n  disable_env_credential_login: true\n")
            self.assertEqual(local_admin_credentials("http://localhost:4000", directory, {}), {})
            config.write_text("general_settings:\n  admin_password: |\n")
            with self.assertRaisesRegex(ValueError, "简单标量"):
                local_admin_credentials("http://localhost:4000", directory, {})

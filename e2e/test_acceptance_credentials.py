"""供应商实际凭据解析单测；不访问外部服务或持久化秘密。"""
import unittest
from acceptance.credentials import resolve_provider_credentials
from acceptance import load_manifest


class CredentialTests(unittest.TestCase):
    """验证建数配置的正常、边界与失败路径；每例仅使用内存环境映射。"""

    def test_actual_keys_and_base_override_are_preserved(self):
        """目的：实际密钥与地址覆盖原样返回；前置完整凭据，验证默认值、覆盖值和空清单边界，不落盘无需清理。"""
        data = load_manifest()
        env = {p["key_env"]: "actual-" + p["id"] for p in data["providers"]}
        first = data["providers"][0]
        env[first["base_env"]] = "https://supplier.example/v1"
        resolved = resolve_provider_credentials(data, env)
        self.assertEqual(resolved[first["id"]], {"api_base": env[first["base_env"]], "api_key": env[first["key_env"]]})
        second = data["providers"][1]
        self.assertEqual(resolved[second["id"]]["api_base"], second["base"])
        self.assertEqual(resolve_provider_credentials({"providers": []}, {}), {})

    def test_invalid_keys_are_rejected_without_leaking_secret(self):
        """目的：拒绝缺失、空白及引用密钥；前置完整清单并逐项替换错误值，验证错误仅指出变量名，不落盘无需清理。"""
        data = load_manifest()
        for provider in data["providers"]:
            for value in (None, "", " ", "sensitive\n", "sensitive key", "os.environ/SECRET"):
                with self.subTest(provider=provider["id"], value=value):
                    env = {p["key_env"]: "valid" for p in data["providers"]}
                    if value is None:
                        del env[provider["key_env"]]
                    else:
                        env[provider["key_env"]] = value
                    with self.assertRaises(ValueError) as failure:
                        resolve_provider_credentials(data, env)
                    self.assertIn(provider["key_env"], str(failure.exception))
                    self.assertNotIn("sensitive", str(failure.exception))
                    self.assertNotIn("os.environ/SECRET", str(failure.exception))

    def test_invalid_bases_fail_without_secret_in_error(self):
        """目的：拒绝不安全地址与 Markdown；前置有效密钥和多种非法地址，验证错误不复述地址及秘密，不落盘无需清理。"""
        data = load_manifest()
        for base in ("", "http://supplier.example", "https://", "https://user:secret@supplier.example", "[https://supplier.example]", "https://supplier.example/path with space"):
            with self.subTest(base=base):
                env = {p["key_env"]: "sensitive" for p in data["providers"]}
                env[data["providers"][0]["base_env"]] = base
                with self.assertRaisesRegex(ValueError, "HTTPS") as failure:
                    resolve_provider_credentials(data, env)
                self.assertNotIn("secret", str(failure.exception))

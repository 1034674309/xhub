"""建数与验收共用的供应商凭据解析；秘密只返回给管理请求，不写报告。"""
import os
from urllib.parse import urlsplit


def resolve_provider_credentials(data, environ=None):
    """用途：在任何数据变更前解析全部供应商的实际密钥；参数为清单和可选环境映射，返回按连接 ID 索引的 api_base/api_key；供 seed 和 runner 使用，缺失、空白、环境引用或非法 HTTPS 地址抛 ValueError，异常不含密钥，无外部调用。"""
    environ = os.environ if environ is None else environ
    resolved = {}
    for provider in data["providers"]:
        name = provider["key_env"]
        key = environ.get(name, "")
        # 环境引用不能证明凭据已配置，直接保存它会让停止建数进程后的连接失效。
        if not key or any(char.isspace() for char in key) or key.startswith("os.environ/"):
            raise ValueError("缺少有效实际凭据: " + name)
        base = environ.get(provider["base_env"], provider["base"])
        url = urlsplit(base)
        if url.scheme != "https" or not url.hostname or url.username or url.password or any(c in base for c in "[]() \n\r\t"):
            raise ValueError("供应商地址必须是无 Markdown 的 HTTPS URL")
        resolved[provider["id"]] = {"api_base": base, "api_key": key}
    return resolved

"""真实本地网关的管理员配置读取；秘密只用于登录，不输出到日志或报告。"""
import os
from pathlib import Path
import shlex
from urllib.parse import urlsplit


def local_admin_credentials(gateway, root, environ=None):
    """用途：读取本地网关配置中的管理员账号和初始密码；参数为目标、项目根及可选环境，返回账号/密码字典。

    显式 E2E 环境优先；仅默认本地 4000 自动读取 configs/config.yaml，其他目标需显式 E2E_CONFIG。
    只支持 general_settings 的简单标量和 os.environ 引用；复杂 YAML 拒绝，避免错误解析或泄露凭据。
    供共享建数与验收调用，不启动服务、不修改配置、不打印秘密。
    """
    env = os.environ if environ is None else environ
    explicit = {key: env[key] for key in ("E2E_DATASET_ADMIN", "E2E_DATASET_PASSWORD") if env.get(key)}
    if explicit.get("E2E_DATASET_PASSWORD"):
        return {"E2E_DATASET_ADMIN": env.get("E2E_DATASET_ADMIN", "admin@xhub.local"), **explicit}
    target = urlsplit(gateway)
    if not env.get("E2E_CONFIG") and (target.scheme != "http" or target.hostname not in ("127.0.0.1", "localhost", "::1") or target.port != 4000):
        return explicit
    path = Path(env.get("E2E_CONFIG", str(Path(root) / "configs/config.yaml")))
    if not path.is_file():
        return explicit
    values = {}
    active = False
    for line in path.read_text().splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        if not line.startswith((" ", "\t")):
            active = line.strip() == "general_settings:"
            continue
        if not active:
            continue
        key, separator, raw = line.strip().partition(":")
        if not separator or key not in ("admin_email", "admin_password", "disable_env_credential_login"):
            continue
        if raw.strip().startswith(('"', "'")):
            parts = shlex.split(raw, comments=True)
            if len(parts) != 1:
                raise ValueError("管理员配置必须为简单标量，请显式设置 E2E_DATASET_PASSWORD")
            value = parts[0]
        else:
            value = raw.split(" #", 1)[0].strip()
        if value[:1] in ("|", ">", "{", "[", "&", "*", "!"):
            raise ValueError("管理员配置必须为简单标量，请显式设置 E2E_DATASET_PASSWORD")
        if value.startswith("os.environ/"):
            value = env.get(value.split("/", 1)[1], "")
        values[key] = value
    if values.get("disable_env_credential_login", "false").lower() == "true":
        return explicit
    return {**({"E2E_DATASET_ADMIN": values.get("admin_email", "admin@xhub.local"),
                "E2E_DATASET_PASSWORD": values["admin_password"]} if values.get("admin_password") else {}), **explicit}

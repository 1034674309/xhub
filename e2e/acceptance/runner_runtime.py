"""进程、日志、锁和共享目标基础设施；不执行业务场景。"""
from contextlib import contextmanager
import fcntl
import os
from pathlib import Path
import secrets
import socket
import subprocess
import threading
import time

from . import support as module

ROOT = Path(__file__).resolve().parents[2]
SHARED_DATABASE_URL = "postgres://xhub:xhub_dev_password@127.0.0.1:5433/xhub?sslmode=disable"
SHARED_REDIS_URL = "redis://127.0.0.1:6379/1"
SHARED_DIRECTORY = ROOT / ".e2e/real-acceptance-current"
SHARED_LOCK = ROOT / ".e2e/real-acceptance-shared.lock"
E2E_CATEGORIES = ("entities", "gpt-1", "gpt-2", "gpt-3", "gpt-4", "glm", "image",
    "ark-video", "fal-video", "kling-video", "inheritance", "chat-bills", "permissions",
    "xgo", "limits", "model-lifecycle", "guard-lifecycle", "global-guards", "profile-isolation",
    "cache-weight", "fallback", "media-logs", "database", "browser", "business-browser", "regression")


class RunLogger:
    """把脱敏后的过程日志同步写到终端和运行目录。"""

    def __init__(self, path):
        """用途：初始化本轮日志；参数为日志路径，返回实例；覆盖同阶段旧日志并允许多线程供应商回调安全写入。"""
        self.path, self.lock = Path(path), threading.Lock()
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self.path.write_text("")

    def log(self, message):
        """用途：输出带时间的单行日志；参数为脱敏正文，无返回值；多行会拆分，立即刷新终端和文件。"""
        lines = str(message).splitlines() or [""]
        with self.lock:
            with self.path.open("a") as stream:
                for line in lines:
                    rendered = time.strftime("[%Y-%m-%d %H:%M:%S] ") + line
                    print(rendered, flush=True)
                    stream.write(rendered + "\n")
                stream.flush()

def command(args, **kwargs):
    """用途：执行不含秘密的子命令；参数为 argv 和运行选项，返回完成结果；失败抛异常，调用方 finally 清理进程。"""
    return subprocess.run(args, cwd=ROOT, check=True, **kwargs)

def command_stream(args, logger, **kwargs):
    """用途：运行长命令并把完整输出实时写入统一日志；参数为 argv/日志器/进程选项，返回无；非零退出码抛异常。"""
    logger.log("    [命令] " + " ".join(args))
    started = time.monotonic()
    process = subprocess.Popen(args, cwd=kwargs.pop("cwd", ROOT), stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                               text=True, bufsize=1, **kwargs)
    for line in process.stdout:
        logger.log("    [命令输出] " + line.rstrip("\n"))
    status = process.wait()
    logger.log(f"    [命令] 退出={status}，耗时={time.monotonic() - started:.3f}s")
    if status:
        raise subprocess.CalledProcessError(status, args)

@contextmanager
def shared_run_lock(path=SHARED_LOCK):
    """用途：串行化共享数据库的构建与验收；参数为锁文件路径，返回上下文；竞争时立即失败，进程退出由系统释放锁。"""
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    handle = open(path, "a+")
    try:
        try:
            fcntl.flock(handle.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as error:
            raise RuntimeError("已有 make testdata 或 make e2e 正在操作共享 xhub 数据") from error
        handle.seek(0)
        handle.truncate()
        handle.write(str(os.getpid()) + "\n")
        handle.flush()
        yield
    finally:
        try:
            fcntl.flock(handle.fileno(), fcntl.LOCK_UN)
        finally:
            handle.close()

def sql(statement):
    """用途：操作隔离 PostgreSQL schema；参数为脚本，返回标准输出；仅 runner 生成标识，密码不进命令行。"""
    return command(["docker", "exec", "-i", "xhub-postgres", "psql", "-X", "-U", "xhub", "-d", "xhub", "-qAt", "-v", "ON_ERROR_STOP=1"],
                   input=statement, text=True, capture_output=True).stdout

def free_port():
    """用途：选择可用回环端口；无参数，返回整数；供隔离服务启动，绑定竞争由健康检查和进程退出发现。"""
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]

def ready(url, process, timeout=90):
    """用途：等待服务可访问；参数为 URL/子进程/秒数，返回无；服务提前退出或超时明确失败，不复用其他任务服务。"""
    end = time.monotonic() + timeout
    while time.monotonic() < end:
        if process.poll() is not None:
            raise RuntimeError("隔离服务启动失败，请查看运行目录日志")
        try:
            if module.http(url, "")[0] == 200:
                return
        except Exception:
            pass
        time.sleep(0.5)
    raise TimeoutError("隔离服务健康检查超时")

def validate_providers(data, environ=None, require_credentials=True):
    """用途：在清库或验收前验证供应商配置；参数为清单、可选环境及是否需要真实凭据，返回无；建数只校验地址，验收缺少密钥或任何阶段地址非法均立即失败，无外部请求。"""
    environ = os.environ if environ is None else environ
    for provider in data["providers"]:
        if require_credentials and not environ.get(provider["key_env"]):
            raise ValueError("缺少环境凭据: " + provider["key_env"])
        base = environ.get(provider["base_env"], provider["base"])
        url = module.urlsplit(base)
        if url.scheme != "https" or not url.hostname or url.username or url.password or any(c in base for c in "[]() \n\r\t"):
            raise ValueError("供应商地址必须是无 Markdown 的 HTTPS URL")

def validate_shared_target(database_url, redis_url):
    """用途：把破坏性清理限制到用户指定的 xhub 与 Redis DB 1；参数为两个 URL，返回无；任何差异都拒绝执行。"""
    if database_url != SHARED_DATABASE_URL:
        raise ValueError("共享验收只允许清理指定的 PostgreSQL xhub 数据库")
    if redis_url != SHARED_REDIS_URL:
        raise ValueError("共享验收只允许清理指定的 Redis DB 1")

def redis_command(*parts):
    """用途：向本机 Redis DB 1 发送受控命令；参数为命令片段，返回 RESP 值；仅供 SELECT/FLUSHDB/DBSIZE 清理核对。"""
    def encode(command_parts):
        """用途：编码单条 Redis RESP 命令；参数为命令片段，返回字节；只处理当前 runner 内部常量。"""
        request = b"*" + str(len(command_parts)).encode() + b"\r\n"
        for part in command_parts:
            value = str(part).encode()
            request += b"$" + str(len(value)).encode() + b"\r\n" + value + b"\r\n"
        return request

    def reply(stream):
        """用途：读取 Redis 简单字符串或整数响应；参数为二进制流，返回值；错误响应转为异常且不包含秘密。"""
        prefix, line = stream.read(1), stream.readline().rstrip(b"\r\n")
        if prefix == b"-":
            raise RuntimeError("Redis 命令失败: " + line.decode(errors="replace"))
        if prefix == b":":
            return int(line)
        if prefix == b"+":
            return line.decode()
        raise RuntimeError("Redis 返回了未支持的响应类型")

    with socket.create_connection(("127.0.0.1", 6379), timeout=10) as connection:
        stream = connection.makefile("rb")
        connection.sendall(encode(("SELECT", "1")) + encode(parts))
        if reply(stream) != "OK":
            raise RuntimeError("Redis DB 1 选择失败")
        return reply(stream)

def reset_shared_target(database_url=SHARED_DATABASE_URL, redis_url=SHARED_REDIS_URL, logger=None):
    """用途：重建指定 xhub 数据库并清空 Redis DB 1；参数为受白名单保护的 URL，返回无；会删除目标中的全部既有数据。"""
    validate_shared_target(database_url, redis_url)
    emit = logger or (lambda message: print(message, flush=True))
    emit("[testdata 1/5] 清空 PostgreSQL xhub 全部数据")
    psql = ["docker", "exec", "-i", "xhub-postgres", "psql", "-X", "-U", "xhub", "-d", "postgres", "-v", "ON_ERROR_STOP=1", "-c"]
    command(psql + ["DROP DATABASE IF EXISTS xhub WITH (FORCE)"])
    command(psql + ["CREATE DATABASE xhub OWNER xhub"])
    emit("    PostgreSQL xhub 已重建，owner=xhub")
    emit("[testdata 2/5] 清空 Redis 127.0.0.1:6379/1 全部数据")
    if redis_command("FLUSHDB") != "OK":
        raise RuntimeError("Redis DB 1 清空失败")
    if redis_command("DBSIZE") != 0:
        raise RuntimeError("Redis DB 1 清空后仍有数据")
    emit("    Redis DB 1 已清空，DBSIZE=0")

def shared_config(password):
    """用途：生成直连用户指定数据库与 Redis 的网关配置；参数为管理员密码，返回配置字典；密钥仅写入私有运行目录。"""
    return {"model_list": [], "router_settings": {"num_retries": 1, "timeout": 90},
            "general_settings": {"master_key": "sk-" + secrets.token_hex(24), "admin_email": "admin", "admin_name": "admin",
            "admin_password": password, "store_model_in_db": True, "store_prompts_in_spend_logs": True,
            "database_url": SHARED_DATABASE_URL, "redis_url": SHARED_REDIS_URL}}


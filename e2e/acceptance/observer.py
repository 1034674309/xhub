"""供应商转发观察器：记录外发证据，并提供受控故障。"""
import json
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from .support import forward_supplier_chat

class Observer:
    """真实供应商转发及可控故障观察器；正常响应均来自外网，不生成模拟补全。"""

    def __init__(self, providers, limit, port=0, logger=None):
        """用途：建立隔离转发服务；参数为供应商配置/尝试上限/端口/日志回调，返回实例；密钥仅从环境读取，服务需 close 清理。"""
        self.providers = {p["id"]: p for p in providers}
        self.limit, self.attempts, self.rows = limit, 0, []
        self.logger = logger or (lambda message: None)
        self.lock = threading.Lock()
        owner = self

        class Handler(BaseHTTPRequestHandler):
            """只转发列入清单的路径，并记录正文中可核对的请求标记。"""

            def log_message(self, *args):
                """用途：关闭默认访问日志；任意日志参数不输出，避免路径或网络错误携带私密材料。"""
                pass

            def do_POST(self):
                """用途：处理网关补全；无参数，返回 HTTP 响应；第二段路径作为部署标签写入证据，只有 fault 注入故障，其余标签仍真实转发。"""
                parts = self.path.strip("/").split("/")
                if not parts or parts[0] not in owner.providers:
                    self.send_error(404)
                    return
                provider = owner.providers[parts[0]]
                body = json.loads(self.rfile.read(int(self.headers.get("Content-Length", 0))))
                text = json.dumps(body.get("messages", []), ensure_ascii=False)
                # 未配置观察标签时，OpenAI 客户端会把协议路径（例如 /v1/chat/completions）
                # 直接追加到供应商前缀。这里把协议段归一为 default，避免把 v1 误当成
                # 部署标签，导致真实回退已经成功却被证据断言错误拒绝。
                candidate_tag = parts[1] if len(parts) > 1 else "default"
                route_tag = "default" if candidate_tag in {"v1", "chat", "responses"} else candidate_tag
                fault = route_tag == "fault"
                with owner.lock:
                    owner.attempts += 1
                    allowed = owner.attempts <= owner.limit
                status = 429 if fault else 0
                usage = {}
                content = b'{"error":{"message":"acceptance injected rate limit","type":"rate_limit"}}'
                if not allowed:
                    status = 429
                elif not fault:
                    try:
                        # 系统 curl 与构建期真实模型探测使用同一网络路径，避免 urllib
                        # 经本机代理建立 HTTPS 隧道时被远端提前断开。
                        status, answer, _ = forward_supplier_chat(provider, body)
                    except (ConnectionError, OSError) as error:
                        status = 502
                        answer = {"error": {"message": "acceptance supplier transport failed",
                                            "type": "upstream_transport"}}
                        owner.logger("供应商=%s 模型=%s 真实转发失败=%s" % (
                            provider["id"], body.get("model"), type(error).__name__))
                    usage = answer.get("usage", {})
                    content = json.dumps(answer).encode()
                with owner.lock:
                    owner.rows.append({"provider": provider["id"], "model": body.get("model"),
                                       "route_tag": route_tag,
                                       "messages": text, "fault": fault, "forwarded": allowed and not fault, "status": status, "usage": usage})
                owner.logger("供应商=%s 部署标签=%s 模型=%s 状态=%s 输入=%s 输出=%s%s" % (
                    provider["id"], route_tag, body.get("model"), status, usage.get("prompt_tokens", 0),
                    usage.get("completion_tokens", 0), "（注入限流）" if fault else ""))
                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(content)

        self.server = ThreadingHTTPServer(("127.0.0.1", port), Handler)
        self.base = "http://127.0.0.1:" + str(self.server.server_port)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()

    def close(self):
        """用途：结束当前观察器；无参数/返回，供 finally 调用；释放端口并等待服务器退出。"""
        self.server.shutdown()
        self.server.server_close()
        self.thread.join()


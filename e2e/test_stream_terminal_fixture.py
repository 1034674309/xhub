"""流终态夹具的正常/边界/失败验证；只用本地HTTP，逐例关闭服务。"""
import json
import threading
import unittest
from http.server import HTTPServer
from urllib.error import HTTPError
from urllib.request import Request, urlopen
from e2e.fake_upstream import Handler
from e2e.stream_terminal_fixture import terminal_payload


class StreamTerminalFixtureTest(unittest.TestCase):
    def setUp(self):
        """启动本例专属本地上游并注册清理；无外部网络和业务数据。"""
        self.server = HTTPServer(("127.0.0.1", 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.base = "http://127.0.0.1:" + str(self.server.server_port)

    def tearDown(self):
        """关闭本例HTTP服务和线程，确保held连接也已由客户端释放。"""
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(2)

    def request(self, model, stream=True, path="/v1/chat/completions", key="sk-fake"):
        """构造本地真实HTTP请求，返回需调用方关闭的响应；错误状态由urllib抛出。"""
        body = json.dumps({"model": model, "stream": stream, "messages": [{"role": "user", "content": "Verify terminal handling."}]}).encode()
        return urlopen(Request(self.base + path, data=body, headers={"Authorization": "Bearer " + key}), timeout=2)

    def test_missing_terminal_fields(self):
        """验证两类故障夹具保留真实内容和用量，但确实缺finish_reason/response对象；关闭HTTP响应。"""
        with self.request("e2e-stream-terminal-missing") as response:
            raw = response.read().decode()
            self.assertIn('"finish_reason": null', raw)
            self.assertIn("[DONE]", raw)
        with self.request("e2e-stream-terminal-response", path="/v1/responses") as response:
            lines = [json.loads(line[5:]) for line in response.read().decode().splitlines() if line.startswith("data:")]
            self.assertEqual(lines[-1], {"type": "response.completed"})

    def test_length_json_sse_and_held(self):
        """验证完整工具+length在JSON/SSE一致，held终态先可读取再释放连接；无真实模型调用。"""
        with self.request("e2e-stream-terminal-length", stream=False) as response:
            payload = json.load(response)
            self.assertEqual(payload["choices"][0]["finish_reason"], "length")
            self.assertEqual(payload["choices"][0]["message"]["tool_calls"][0]["function"]["arguments"], "{}")
        with self.request("e2e-stream-terminal-length") as response:
            self.assertIn('"finish_reason": "length"', response.read().decode())
        with self.request("e2e-stream-terminal-held") as response:
            while response.readline().strip() != b"data: [DONE]":
                pass
        self.assertEqual(terminal_payload("e2e-stream-terminal-held")[0]["usage"]["completion_tokens"], 2)

    def test_reject_wrong_configuration(self):
        """错误凭据、路径和非流模式必须4xx，防止浏览器成功来自错误夹具分支；关闭错误响应。"""
        for kwargs, status in [({"key": "wrong"}, 401), ({"path": "/v1/responses"}, 400), ({"stream": False}, 400)]:
            with self.assertRaises(HTTPError) as error:
                self.request("e2e-stream-terminal-held", **kwargs)
            self.assertEqual(error.exception.code, status)
            error.exception.close()

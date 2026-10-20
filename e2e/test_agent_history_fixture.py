"""原生历史夹具的确定性单元验证；不启动供应商或写数据库。"""
import unittest
from e2e.agent_history_fixture import handle_agent_history, messages_events, validate_chat_history


class Capture:
    """捕获夹具的状态和输出，供断言成功/拒绝路径；实例为测试私有，无网络。"""
    def __init__(self, path):
        """以路径初始化捕获器；无返回，默认成功状态，仅保存内存。"""
        self.path, self.status, self.body = path, 200, None

    def _json(self, body):
        """记录同步正文；参数为对象，无返回，无外部副作用。"""
        self.body = body

    def _sse(self, body):
        """记录事件数组；参数为事件，无返回，无外部副作用。"""
        self.body = body

    def _response(self, status, body, content_type):
        """记录错误HTTP结果；参数为状态/字节/类型，无返回，无外部副作用。"""
        self.status, self.body = status, body


class AgentHistoryFixtureTest(unittest.TestCase):
    """检查夹具正常、边界与失败行为，避免假上游接受损坏历史造成假通过。"""
    def test_chat_parallel(self):
        """完整并行消息与逆序结果必须通过，拆组必须失败；所有数据内存销毁。"""
        calls = [{"id": "call_a"}, {"id": "call_b"}]
        results = [{"role": "tool", "tool_call_id": "call_b", "content": "beta-value"}, {"role": "tool", "tool_call_id": "call_a", "content": "alpha-value"}]
        self.assertFalse(validate_chat_history([{"role": "user", "content": "Hello"}]))
        self.assertTrue(validate_chat_history([{"role": "assistant", "tool_calls": calls}] + results))
        with self.assertRaises(ValueError):
            validate_chat_history([{"role": "assistant", "tool_calls": [call]} for call in calls] + results)
        with self.assertRaises(ValueError):
            validate_chat_history([{"role": "assistant", "tool_calls": calls}] + results[::-1])

    def test_native_messages(self):
        """原生签名/密文/caller完整回放通过，损坏签名返回400；同步流式均有合法终态，不留数据。"""
        for stream in (False, True):
            handler = Capture("/v1/messages")
            request = {"model": "e2e-history-messages", "stream": stream, "messages": [], "tools": [{}]}
            self.assertTrue(handle_agent_history(handler, request))
            self.assertEqual(handler.status, 200)
            if stream:
                self.assertEqual(handler.body[-1], {"type": "message_stop"})
                continue
            content = handler.body["content"]
            request["messages"] = [{"role": "assistant", "content": content}, {"role": "user", "content": [{"type": "tool_result", "tool_use_id": "call_read", "content": "file-value"}]}]
            handle_agent_history(handler, request)
            self.assertEqual(handler.body["content"], [{"type": "text", "text": "history-ok"}])
            content[0]["signature"] = "corrupt"
            handle_agent_history(handler, request)
            self.assertEqual(handler.status, 400)

    def test_dispatch_boundaries(self):
        """未知模型不截获、错误协议拒绝、普通Chat与流式工具输出合法；捕获器退出即清理。"""
        handler = Capture("/v1/chat/completions")
        self.assertFalse(handle_agent_history(handler, {"model": "other"}))
        self.assertTrue(handle_agent_history(handler, {"model": "e2e-history-chat"}))
        self.assertEqual(handler.body["choices"][0]["message"]["content"], "history-ok")
        handle_agent_history(handler, {"model": "e2e-history-chat", "stream": True, "tools": [{}]})
        self.assertEqual(handler.body[-1], "[DONE]")
        self.assertEqual([call["index"] for call in handler.body[0]["choices"][0]["delta"]["tool_calls"]], [0, 1])
        handle_agent_history(handler, {"model": "e2e-history-messages"})
        self.assertEqual(handler.status, 400)
        self.assertEqual(messages_events([], "model")[-2]["delta"]["stop_reason"], "end_turn")


if __name__ == "__main__":
    unittest.main()

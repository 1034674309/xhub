"""签名历史夹具单元验证；全部合成数据，无外网或持久资源。"""
import copy
import unittest
from e2e.messages_signed_fixture import signed_content, signed_payload, serve_signed_history


def request_for(round_number):
    """构造给定轮次的合法原生请求，返回独立字典供成功与篡改测试，无外部副作用。"""
    messages = [{"role": "user", "content": "Read both synthetic values using lookup."}]
    for number in range(1, round_number):
        messages.extend([{"role": "assistant", "content": signed_content(number)},
                         {"role": "user", "content": [{"type": "tool_result", "tool_use_id": f"call_{number}", "content": f"VALUE_{number}"}]}])
    return {"thinking": {"type": "adaptive"}, "output_config": {"effort": "low"}, "max_tokens": 64, "messages": messages}


class SignedHistoryFixtureTest(unittest.TestCase):
    """覆盖有序历史、分片、终态及错误输入，内存对象随每例结束回收。"""

    def test_three_rounds_and_fragmentation(self):
        """完整两次依赖工具与最终文本有效，签名和参数确实分片，输入不可变；无资源清理。"""
        for number in (1, 2, 3):
            request = request_for(number)
            original = copy.deepcopy(request)
            payload, events = signed_payload(request)
            self.assertEqual(request, original)
            self.assertEqual(payload["usage"], {"input_tokens": 8, "output_tokens": 4})
            self.assertEqual(events[-1]["type"], "message_stop")
            if number < 3:
                self.assertEqual(payload["content"], signed_content(number))
                signatures = [event["delta"]["signature"] for event in events if event.get("delta", {}).get("type") == "signature_delta"]
                self.assertEqual(len(signatures), 2)
                self.assertEqual("".join(signatures), f"PRIVATE_SIGNATURE_{number}")
            else:
                self.assertEqual(payload["content"][0]["text"], "VALUE_1|VALUE_2")
                self.assertEqual(payload["stop_reason"], "end_turn")

    def test_invalid_parameters(self):
        """缺省/错误推理和输出上限明确拒绝，不能制造虚假的兼容成功；仅内存数据。"""
        for updates in ({"thinking": None}, {"thinking": {"type": "disabled"}}, {"output_config": {"effort": "high"}}, {"max_tokens": 0}, {"messages": []}):
            with self.subTest(updates=updates), self.assertRaises(ValueError):
                signed_payload({**request_for(1), **updates})

    def test_tampered_history(self):
        """缺签名、重排块、错误结果和缺助手轮都必须失败；避免夹具只看终态文本，无外部资源。"""
        cases = []
        body = request_for(2); del body["messages"][1]["content"][0]["signature"]; cases.append(body)
        body = request_for(2); body["messages"][1]["content"].reverse(); cases.append(body)
        body = request_for(3); body["messages"][3]["content"][3]["input"]["previous"] = "WRONG"; cases.append(body)
        body = request_for(2); body["messages"][2]["content"][0]["content"] = "WRONG"; cases.append(body)
        body = request_for(2); body["messages"].pop(1); cases.append(body)
        for body in cases:
            with self.subTest(body=body), self.assertRaises(ValueError):
                signed_payload(body)

    def test_unrelated_model(self):
        """无关型号不读取handler或写响应，允许与其他离线夹具共存。"""
        self.assertFalse(serve_signed_history(None, {"model": "other"}))

    def test_http_dispatch_success_and_failures(self):
        """专属HTTP处理器覆盖同步/流式、鉴权、路径及请求错误；仅Mock响应写入，不启动网络或保存数据。"""
        from unittest.mock import Mock
        for stream in (False, True):
            handler = Mock(headers={"x-api-key": "sk-fake"}, path="/v1/messages")
            self.assertTrue(serve_signed_history(handler, {**request_for(1), "model": "e2e-claude-signed", "stream": stream}))
            (handler._sse if stream else handler._json).assert_called_once()
            handler.send_error.assert_not_called()
        for headers, path, updates, status in [({}, "/v1/messages", {}, 401), ({"x-api-key": "sk-fake"}, "/v1/responses", {}, 400), ({"Authorization": "Bearer sk-fake"}, "/v1/messages", {"thinking": None}, 400)]:
            handler = Mock(headers=headers, path=path)
            self.assertTrue(serve_signed_history(handler, {**request_for(1), "model": "e2e-claude-signed", **updates}))
            self.assertEqual(handler.send_error.call_args.args[0], status)
            handler._json.assert_not_called()
            handler._sse.assert_not_called()

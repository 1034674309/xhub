"""Codex 验收模拟器的标准会话协议与持久化证据单元测试。"""
import copy
import tempfile
import unittest
from unittest import mock

from acceptance import Dataset, load_manifest


def codex_fixture(directory):
    """用途：为模拟器测试创建三轮响应与完整账单；参数为临时目录，返回数据集、密钥、会话和账单。

    调用方用于执行及断点复验；固定英文输入、五级身份与价格，不访问网络，目录由调用方清理。
    """
    dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lambda _: None)
    key = dict(key="secret", token_id="k", user_id="u", team_id="t",
               project_id="p", organization_id="o", profile="inherit", call_model="gpt-test")
    conversation = dict(agent_type="codex", session_id="codex-test", marker="project-test",
                        key_id="k", profile="inherit", model="gpt-test", turns=[])
    config = dataset.data["agent_conversation"]
    history, bills = [], []
    for index in range(1, 4):
        prompt = ["Remember the project marker project-test. Reply with only that marker.",
                  "What project marker did I give you? Reply with only the marker.",
                  "Confirm the same project marker once more. Reply with only the marker."][index - 1]
        request = dict(model="gpt-test", instructions=config["instructions"],
                       input=[dict(role="user", content=prompt)], store=False, stream=False,
                       max_output_tokens=config["max_output_tokens"])
        previous = "resp-%d" % (index - 1) if index > 1 else None
        if previous:
            request["previous_response_id"] = previous
        history += request["input"]
        stored = {name: value for name, value in request.items()
                  if name not in ("previous_response_id", "store")}
        stored["input"] = copy.deepcopy(history)
        response = dict(id="resp-%d" % index, status="completed",
                        output=[dict(content=[dict(type="output_text", text="project-test")])],
                        usage=dict(input_tokens=11, output_tokens=7))
        bill = dict(api_key="k", user="u", team_id="t", project_id="p", organization_id="o",
                    model="gpt-test", session_id="codex-test", prompt_tokens=11, completion_tokens=7,
                    spend=0.000025, metadata=dict(cost_breakdown=dict(source="snapshot")),
                    response=response, proxy_server_request=dict(body=stored, headers={
                        "User-Agent": config["user_agent"], "X-Session-Id": "codex-test",
                        "Originator": "codex_cli_rs"}))
        conversation["turns"].append(dict(turn=index, call_id="call-%d" % index,
                                          response_id=response["id"], previous_response_id=previous,
                                          prompt=prompt, request_body=request, prompt_tokens=11,
                                          completion_tokens=7, cost=0.000025))
        bills.append(bill)
        history.append(dict(role="assistant", content="project-test"))
    return dataset, key, conversation, bills


class CodexSessionTests(unittest.TestCase):
    """验证三轮标准会话协议和失败证据；每项使用自动清理的私有目录，无外部调用。"""

    def test_three_turns_send_standard_header_and_revalidate(self):
        """目的：三轮均发送标准头并可复验；前置完整响应账单，验证增量请求、稳定 ID 和检查点，临时目录自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset, key, conversation, bills = codex_fixture(directory)
            replies = [(bill["response"], {"X-Session-Id": "codex-test",
                        "x-litellm-call-id": "call-%d" % index})
                       for index, bill in enumerate(bills, 1)]
            with mock.patch("acceptance.chat.acceptance_run_id", side_effect=["test", "test"]), \
                    mock.patch.object(dataset, "api", side_effect=replies) as api, \
                    mock.patch.object(dataset, "wait_bill", side_effect=bills):
                saved = dataset.codex_conversation(key)
            self.assertEqual(saved["turns"], conversation["turns"])
            for call, turn in zip(api.call_args_list, conversation["turns"]):
                self.assertEqual(call.args[1], turn["request_body"])
                self.assertEqual(call.kwargs["request_headers"]["X-Session-Id"], "codex-test")
                self.assertNotIn("Session_id", call.kwargs["request_headers"])
            with mock.patch.object(dataset, "api", side_effect=[(bill, {}, 200) for bill in bills]):
                self.assertEqual(dataset.codex_conversation(key, revalidate=True), saved)

    def test_missing_or_wrong_response_session_header_rejects_checkpoint(self):
        """目的：缺失或错误响应头不能算成功；前置 HTTP 成功响应，验证核账前失败且无检查点，临时目录自动清理。"""
        for actual in (None, "prompt:v1:wrong"):
            with self.subTest(actual=actual), tempfile.TemporaryDirectory() as directory:
                dataset, key, _, bills = codex_fixture(directory)
                headers = {"x-litellm-call-id": "call-1"}
                if actual is not None:
                    headers["X-Session-Id"] = actual
                with mock.patch.object(dataset, "api", return_value=(bills[0]["response"], headers)), \
                        mock.patch.object(dataset, "wait_bill") as wait:
                    with self.assertRaisesRegex(AssertionError, "第 1 轮响应会话头错误：call_id=call-1"):
                        dataset.codex_conversation(key)
                wait.assert_not_called()
                self.assertFalse(dataset.report.get("agent_conversations"))

    def test_bill_identity_diagnostics_and_legacy_header_are_rejected(self):
        """目的：分别定位会话、模型和旧头错误；前置正常首轮账单后逐项破坏，验证执行拒绝及复验补跑，目录自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset, key, conversation, bills = codex_fixture(directory)
            turn = conversation["turns"][0]
            dataset.check_codex_turn(key, conversation, turn, bills[0])
            for field in ("session_id", "model"):
                for actual in (None, "wrong"):
                    with self.subTest(field=field, actual=actual):
                        changed = copy.deepcopy(bills)
                        changed[0][field] = actual
                        with self.assertRaisesRegex(AssertionError, "call_id=call-1，字段=" + field):
                            dataset.check_codex_turn(key, conversation, turn, changed[0])
                        dataset.report["agent_conversations"] = [conversation]
                        with mock.patch.object(dataset, "api", return_value=(changed[0], {}, 200)):
                            self.assertIsNone(dataset.codex_conversation(key, revalidate=True))
            for name in ("x-session-id", "X-SESSION-ID"):
                valid = copy.deepcopy(bills[0])
                valid["proxy_server_request"]["headers"] = {
                    "user-agent": dataset.data["agent_conversation"]["user_agent"],
                    name: "codex-test", "originator": "codex_cli_rs"}
                dataset.check_codex_turn(key, conversation, turn, valid)
            legacy = copy.deepcopy(bills[0])
            headers = legacy["proxy_server_request"]["headers"]
            headers["Session_id"] = headers.pop("X-Session-Id")
            with self.assertRaisesRegex(AssertionError, "客户端身份或稳定会话头缺失"):
                dataset.check_codex_turn(key, conversation, turn, legacy)

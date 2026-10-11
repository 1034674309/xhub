"""真实验收 routing 模块的单元测试；各用例独立管理夹具与清理。"""
import tempfile
import unittest
from unittest import mock
from acceptance import Dataset, fallback_acceptance_deployments, fallback_acceptance_route, load_manifest, weighted_acceptance_route


class AcceptanceTests(unittest.TestCase):
    """验证所属业务模块的正常、边界和失败路径，资源由各测试自行隔离清理。"""

    def test_weighted_acceptance_route_switches_only_the_active_deployment(self):
        """目的：确定性权重链只在两个真实部署间切换；前置同一模型和两个部署 ID，验证 100:0、0:100、无回退及单次尝试，纯内存结果无需清理。"""
        first = weighted_acceptance_route("gpt-5.6-sol", "a", "b", [100, 0], 300)
        second = weighted_acceptance_route("gpt-5.6-sol", "a", "b", [0, 100], 300)
        self.assertEqual(first["model_routes"][0]["allocations"], [
            {"deployment_id": "a", "weight": 100}, {"deployment_id": "b", "weight": 0}])
        self.assertEqual(second["model_routes"][0]["allocations"], [
            {"deployment_id": "a", "weight": 0}, {"deployment_id": "b", "weight": 100}])
        self.assertEqual(first["fallbacks"], [])
        self.assertEqual(first["retry_policy"]["max_attempts"], 1)


    def test_fallback_acceptance_route_pins_zero_weight_and_real_fallback(self):
        """目的：零权重验收不能被现有组织或团队模板覆盖；前置主模型故障/健康部署及真实备用模型，验证临时密钥模板显式写入100:0、备用部署和回退链，纯内存结果无需清理。"""
        body = fallback_acceptance_route("gpt-5.6-sol", "fault", "healthy", "z-ai/glm-5", "qiniu", 300)
        self.assertEqual(body["model_routes"][0]["allocations"], [
            {"deployment_id": "fault", "weight": 100},
            {"deployment_id": "healthy", "weight": 0},
        ])
        self.assertEqual(body["model_routes"][1]["allocations"], [
            {"deployment_id": "qiniu", "weight": 100},
        ])
        self.assertEqual(body["fallbacks"], [{"gpt-5.6-sol": ["z-ai/glm-5"]}])
        self.assertEqual(body["retry_policy"]["max_attempts"], 1)


    def test_fallback_acceptance_deployments_excludes_all_shared_model_suppliers(self):
        """目的：同一公开模型跨供应商时全部健康部署必须置零；前置七牛部署排在 Fenno 之前及临时部署，验证选择独立 GLM 目标并排除临时记录，纯内存无需清理。"""
        deployments = [
            {"id": "qiniu-shared", "public_name": "gpt-5.6-sol", "provider": "qiniu"},
            {"id": "fenno-shared", "public_name": "gpt-5.6-sol", "provider": "fennoai"},
            {"id": "other-glm", "public_name": "z-ai/glm-5", "provider": "fennoai"},
            {"id": "qiniu-glm", "public_name": "z-ai/glm-5", "provider": "qiniu"},
            {"id": "old-fault", "public_name": "gpt-5.6-sol", "provider": "fennoai", "temporary_fault": True},
            {"id": "temp-glm", "public_name": "z-ai/glm-5", "provider": "qiniu", "temporary": True},
        ]
        healthy, target = fallback_acceptance_deployments(deployments, "gpt-5.6-sol", "z-ai/glm-5", "qiniu")
        self.assertEqual(healthy, ["qiniu-shared", "fenno-shared"])
        self.assertEqual(target, "qiniu-glm")
        body = fallback_acceptance_route("gpt-5.6-sol", "fault", healthy, "z-ai/glm-5", target, 300)
        self.assertEqual(body["model_routes"][0]["allocations"], [
            {"deployment_id": "fault", "weight": 100},
            {"deployment_id": "qiniu-shared", "weight": 0},
            {"deployment_id": "fenno-shared", "weight": 0},
        ])


    def test_fallback_acceptance_deployments_rejects_invalid_baseline(self):
        """目的：无效基线须在创建临时数据前明确失败；前置同名主备、缺失主备或重复备用，验证抛 ValueError，纯内存无需清理。"""
        primary = {"id": "healthy", "public_name": "primary", "provider": "fennoai"}
        backup = {"id": "backup", "public_name": "backup", "provider": "qiniu"}
        for rows, source, target in (([primary], "primary", "primary"),
                                     ([primary], "primary", "backup"),
                                     ([backup], "primary", "backup"),
                                     ([primary, backup, {**backup, "id": "duplicate"}], "primary", "backup")):
            with self.subTest(rows=rows, source=source, target=target), self.assertRaises(ValueError):
                fallback_acceptance_deployments(rows, source, target, "qiniu")
        with self.assertRaisesRegex(ValueError, "不同的公开模型"):
            fallback_acceptance_route("shared", "fault", ["healthy"], "shared", "backup", 300)


    def test_fallback_policy_chain_uses_manifest_glm_and_cleans_up_on_failure(self):
        """目的：验收入口不能重用两家共用的 chat_models，且异常必须清理临时数据；前置真实清单及两家同名基线，在模板失败与聊天失败处断言 GLM 目标、全部零权重与删除调用，临时目录自动清理。"""
        for fail_at in ("template", "chat"):
            with self.subTest(fail_at=fail_at), tempfile.TemporaryDirectory() as directory:
                dataset = Dataset("http://127.0.0.1:1", directory, load_manifest())
                dataset.state["chat_models"] = {"fennoai": "gpt-5.6-sol", "qiniu": "gpt-5.6-sol"}
                dataset.state["selected"] = {"fennoai": {"provider": {"id": "fennoai"}}}
                baseline = [
                    {"id": "qiniu", "public_name": "gpt-5.6-sol", "provider": "qiniu"},
                    {"id": "fenno", "public_name": "gpt-5.6-sol", "provider": "fennoai"},
                    {"id": "glm", "public_name": "z-ai/glm-5", "provider": "qiniu"},
                ]
                dataset.state["deployments"] = baseline + [
                    {"id": "fault", "public_name": "gpt-5.6-sol", "provider": "fennoai", "temporary": True}]
                dataset.observer = mock.MagicMock(rows=[])

                def api(route, body):
                    """模拟管理接口并核对完整路由参数；参数为路径和正文，返回创建结果或抛故障，所有状态仅在当前临时实例内。"""
                    if route == "/route_template/new":
                        self.assertEqual(body["body"]["fallbacks"], [{"gpt-5.6-sol": ["z-ai/glm-5"]}])
                        self.assertEqual(body["body"]["model_routes"][0]["allocations"], [
                            {"deployment_id": "fault", "weight": 100},
                            {"deployment_id": "qiniu", "weight": 0},
                            {"deployment_id": "fenno", "weight": 0}])
                        if fail_at == "template":
                            raise AssertionError("injected failure")
                        return {"id": "route"}, {}
                    return {"key": "temporary-" + body.get("key_alias", "")}, {}

                with mock.patch.object(dataset, "deployment", return_value="fault"), \
                     mock.patch.object(dataset, "api", side_effect=api) as requests, \
                     mock.patch.object(dataset, "chat_request", side_effect=AssertionError("injected failure")):
                    with self.assertRaisesRegex(AssertionError, "injected failure"):
                        dataset.verify_fallback_policy_chain({"user_id": "user", "team_id": "team", "project_id": "project"})
                self.assertEqual(dataset.state["deployments"], baseline)
                requests.assert_any_call("/model/delete", {"id": "fault"})
                if fail_at == "chat":
                    requests.assert_any_call("/route_template/route/delete", {})
                    self.assertEqual(sum(call.args[0] == "/key/delete" for call in requests.call_args_list), 2)


    def test_profile_route_isolation_calls_both_real_models_and_rejects_cross_provider(self):
        """目的：真实模型 ID 分离后仍主动验证两种单供应商路由；前置两个配置密钥和模拟观察记录，验证两次真实调用目标及跨供应商记录会失败，临时目录自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest())
            dataset.state["keys"] = [
                {"profile": "fenno-only", "call_model": "gpt-5.6-sol"},
                {"profile": "qiniu-only", "call_model": "z-ai/glm-5"},
            ]
            dataset.observer = mock.MagicMock(rows=[])

            def observed_call(key, marker, model=None):
                """模拟当前调用进入与配置匹配的供应商；参数为密钥、标记和模型，无返回；仅写入内存观察记录。"""
                provider = {"fenno-only": "fennoai", "qiniu-only": "qiniu"}[key["profile"]]
                dataset.observer.rows.append({"provider": provider, "messages": marker, "status": 200})

            with mock.patch.object(dataset, "call", side_effect=observed_call) as call:
                dataset.verify_profile_route_isolation()
            self.assertEqual([row.kwargs["model"] for row in call.call_args_list], ["gpt-5.6-sol", "z-ai/glm-5"])
            self.assertTrue(dataset.checked("profile-routes-real-models-and-vendors"))

            dataset.report["checks"] = []
            dataset.observer.rows = []

            def crossed_call(key, marker, model=None):
                """模拟错误跨供应商转发；参数为密钥、标记和模型，无返回；用于证明隔离断言会拒绝错误路径。"""
                dataset.observer.rows.append({"provider": "qiniu", "messages": marker, "status": 200})

            with mock.patch.object(dataset, "call", side_effect=crossed_call):
                with self.assertRaisesRegex(AssertionError, "fenno-only"):
                    dataset.verify_profile_route_isolation()


    def test_deployment_route_tag_builds_distinct_observer_address(self):
        """目的：同模型临时部署必须生成可观察的独立地址；前置模拟观察器和模型创建接口，验证 weight-b 写入 api_base、状态保存标签及临时资源标记，不创建真实部署。"""
        with tempfile.TemporaryDirectory() as directory:
            observer = type("ObserverStub", (), {"base": "http://127.0.0.1:4567"})()
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), observer=observer)
            definition = next(row for row in dataset.data["models"] if row["id"] == "fenno-chat")
            provider = dataset.data["providers"][0]
            captured = []

            def api(route, body):
                """记录部署请求并返回固定 ID；参数为路由和正文，返回模拟管理响应；仅用于内存断言。"""
                captured.append((route, body))
                return {"model_info": {"id": "deployment-tagged"}}, {}

            dataset.api = api
            ident = dataset.deployment(definition, provider, "gpt-5.6-sol", route_tag="weight-b")
            self.assertEqual(ident, "deployment-tagged")
            self.assertEqual(captured[0][1]["litellm_params"]["api_base"],
                             "http://127.0.0.1:4567/fennoai/weight-b")
            self.assertEqual(dataset.state["deployments"][0]["route_tag"], "weight-b")
            self.assertTrue(dataset.state["deployments"][0]["temporary"])



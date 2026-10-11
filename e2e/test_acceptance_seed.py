"""真实验收 seed 模块的单元测试；各用例独立管理夹具与清理。"""
import os
import tempfile
import unittest
from unittest import mock
from acceptance import Dataset, load_manifest


class AcceptanceTests(unittest.TestCase):
    """验证所属业务模块的正常、边界和失败路径，资源由各测试自行隔离清理。"""

    def test_seed_rejects_existing_tenants_before_supplier_or_model_calls(self):
        """目的：建数不能覆盖已有租户或进入供应商执行；前置登录成功但已有组织，验证失败后无目录、探测、会话和媒体调用，临时目录自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lambda message: None)
            dataset.api = mock.Mock(side_effect=[({"key": "admin"}, {}), ([{"organization_id": "existing"}], {})])
            with mock.patch.dict(os.environ, {"E2E_DATASET_PASSWORD": "password"}), \
                    mock.patch.object(dataset, "discover_catalog") as catalog, \
                    mock.patch.object(dataset, "probe_chat_candidate") as probe, \
                    mock.patch.object(dataset, "verify_acceptance_models") as models:
                with self.assertRaisesRegex(RuntimeError, "已有组织"):
                    dataset.seed()
            catalog.assert_not_called()
            probe.assert_not_called()
            models.assert_not_called()


    def test_seed_creates_admin_key_without_running_models(self):
        """目的：建数只走管理接口并额外创建管理员本人的个人密钥；前置内存管理 API 与空租户，验证 81+1 归属、报告及零供应商调用，临时检查点自动清理。"""
        with tempfile.TemporaryDirectory() as directory:
            dataset = Dataset("http://127.0.0.1:1", directory, load_manifest(), logger=lambda message: None)

            def management_api(route, body=None, **options):
                """模拟确定性管理响应；参数为路径、正文及请求选项，返回记录和空响应头；仅供建数单测，未声明路径立即失败，无外部副作用。"""
                if route == "/v2/login":
                    return {"key": "session", "user_id": "admin-id"}, {}
                if route == "/organization/list":
                    return [], {}
                if route == "/credentials":
                    return {"credentials": [{"credential_name": p["id"]} for p in dataset.data["providers"]]}, {}
                if route == "/model/groups?size=200":
                    groups = {}
                    for row in dataset.state["deployments"]:
                        group = groups.setdefault(row["public_name"], {"model_name": row["public_name"], "deployments": []})
                        group["deployments"].append({"id": row["id"], "model_name": row["public_name"],
                         "litellm_params": {"litellm_credential_name": row["provider"], "model": row["model"]},
                         "model_info": {"transport": row["transport"], "endpoint_types": row["endpoint_types"]}})
                    return {"data": list(groups.values())}, {}
                fields = {"/organization/new": "organization_id", "/team/new": "team_id",
                          "/user/new": "user_id", "/project/new": "project_id"}
                if route in fields:
                    return {fields[route]: route + str(dataset.api.call_count)}, {}
                if route == "/key/generate":
                    return {**body, "key": "plain", "token_id": str(dataset.api.call_count)}, {}
                if route in ("/model/default", "/route_template/binding", "/guardrails"):
                    return {}, {}
                raise AssertionError("建数出现未声明管理路径或数据面调用: " + route)

            def register_model(definition, provider, upstream=None, public_name=None):
                """模拟部署登记并填充状态；参数与 deployment 一致，返回唯一 ID；只隔离持久化细节，管理模型结构仍由 seed 验证。"""
                row = {**definition, "id": str(len(dataset.state["deployments"])),
                       "provider": provider["id"], "public_name": public_name or definition["public_name"],
                       "model": definition.get("stored_model", upstream or definition["upstream_model"])}
                dataset.state["deployments"].append(row)
                return row["id"]

            dataset.api = mock.Mock(side_effect=management_api)
            with mock.patch.dict(os.environ, {"E2E_DATASET_PASSWORD": "password"}), \
                    mock.patch.object(dataset, "deployment", side_effect=register_model), \
                    mock.patch.object(dataset, "template", return_value="template"), \
                    mock.patch.object(dataset, "discover_catalog") as catalog, \
                    mock.patch.object(dataset, "probe_chat_candidate") as probe, \
                    mock.patch.object(dataset, "verify_acceptance_models") as models:
                dataset.seed()
            self.assertEqual(len(dataset.state["keys"]), 81)
            self.assertEqual(dataset.state["admin_key"]["user_id"], "admin-id")
            self.assertEqual(dataset.state["admin_key"]["owner_type"], "personal")
            self.assertNotIn("team_id", dataset.state["admin_key"])
            self.assertEqual(dataset.report["total_key_count"], 82)
            self.assertEqual(dataset.report["calls"], [])
            catalog.assert_not_called()
            probe.assert_not_called()
            models.assert_not_called()



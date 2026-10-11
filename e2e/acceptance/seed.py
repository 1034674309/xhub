"""真实验收的 seed 模块；共享上下文由 Dataset 静态组合，资源与失败证据归本轮所有。"""
import os
import secrets

from .support import hierarchy


class SeedCases:
    """用途：封装 seed 场景；通过 self 访问本轮状态、管理 API 和证据，供 Dataset 调用，不自行启动全套测试。"""

    def seed(self):
        """用途：仅通过管理 API 构建清单声明的保留基线；无参数、无返回值；供 testdata 和验收准备调用，要求空租户库，不读取供应商目录、不探测或运行模型，异常保留检查点。"""
        login, _ = self.api("/v2/login", {"username": os.environ.get("E2E_DATASET_ADMIN", "admin"),
                                          "password": os.environ["E2E_DATASET_PASSWORD"]})
        self.admin = login["key"]
        self.progress("  [构建 1/5] 管理员登录成功，检查空数据库")
        orgs, _ = self.api("/organization/list")
        if orgs:
            raise RuntimeError("数据集要求空租户环境；已有组织时拒绝覆盖，请使用隔离运行入口")
        selected, ids = {}, []
        self.state["chat_models"] = {}
        self.progress("  [构建 2/5] 先添加 2 个真实模型提供商连接")
        for provider in self.data["providers"]:
            self.action_start(provider["name"], "provider", "添加模型提供商", f"连接名={provider['id']}")
            self.api("/credentials", {"credential_name": provider["id"],
                "credential_info": {"catalog_id": provider["id"], "custom_llm_provider": "openai", "provider_id": "OpenAI"},
                "credential_values": {"api_base": os.environ.get(provider["base_env"], provider["base"]),
                                      "api_key": "os.environ/" + provider["key_env"]}})
            listed, _ = self.api("/credentials")
            if provider["id"] not in {row.get("credential_name") for row in listed.get("credentials", [])}:
                raise AssertionError("模型提供商连接未持久化: " + provider["id"])
            self.action_ok(provider["name"], "provider", "添加模型提供商", "连接已持久化，凭据值保持脱敏")
        self.progress("  [构建 3/5] 按清单添加模型与端点，仅保存配置，不调用供应商")
        for provider in self.data["providers"]:
            chat_definition_id = {"fennoai": "fenno-chat", "qiniu": "qiniu-chat"}[provider["id"]]
            definition = next(row for row in self.data["models"] if row["id"] == chat_definition_id)
            found = definition["upstream_model"]
            selected[provider["id"]] = {"provider": provider, "model": found}
            self.state["chat_models"][provider["id"]] = self.data["shared_chat_model"]["public_name"]
            ids.append(self.deployment(definition, provider, found, self.data["shared_chat_model"]["public_name"]))
            self.report["models"].append({"provider": provider["id"], "upstream_model": found, "source": "manifest"})
            required = [row for row in self.data["models"] if row["provider"] == provider["id"]
                        and row["id"] not in ("fenno-chat", "qiniu-chat")]
            for definition in required:
                self.deployment(definition, provider)
                self.report["models"].append({"provider": provider["id"], "upstream_model": definition["upstream_model"],
                                               "kind": definition["kind"], "source": "manifest"})
        self.api("/model/default", {"model_name": self.data["shared_chat_model"]["public_name"],
            "weights": {"allocations": [
                {"deployment_id": ids[0], "weight": self.data["shared_chat_model"]["default_weights"][0]},
                {"deployment_id": ids[1], "weight": self.data["shared_chat_model"]["default_weights"][1]}]}}, method="PUT")
        groups, _ = self.api("/model/groups?size=200")
        group_rows = groups.get("data", groups if isinstance(groups, list) else [])
        for deployment in self.state["deployments"]:
            group = next((row for row in group_rows if row.get("model_name") == deployment["public_name"]), None)
            if not group:
                raise AssertionError("模型分组未显示真实模型 ID: " + deployment["public_name"])
            actual = next((row for row in group.get("deployments", [])
                           if row.get("id") == deployment["id"]), None)
            if not actual:
                raise AssertionError("模型分组缺少部署: " + deployment["id"])
            params, info = actual.get("litellm_params", {}), actual.get("model_info", {})
            expected = {
                "公开模型名": (actual.get("model_name"), deployment["public_name"]),
                "供应商连接": (params.get("litellm_credential_name"), deployment["provider"]),
                "上游模型": (params.get("model"), deployment["model"]),
                "传输协议": (info.get("transport"), deployment["transport"]),
                "用户入口": (info.get("endpoint_types"), deployment["endpoint_types"]),
            }
            wrong = [name for name, (got, wanted) in expected.items() if got != wanted]
            if wrong:
                raise AssertionError(deployment["public_name"] + " 部署结构错误: " + ",".join(wrong))
            self.action_ok(deployment["public_name"], deployment["transport"], "结构核对",
                           f"公开名={deployment['public_name']}，连接={deployment['provider']}，"
                           f"上游={deployment['model']}，入口={','.join(deployment['endpoint_types'])}")
        self.progress("  [构建 4/5] 创建独立真实模型路由及组织/团队/个人结构")
        fenno = self.template("route-fennoai-only", ids, "fenno-only")
        qiniu = self.template("route-qiniu-only", ids, "qiniu-only")
        allowed_models = list(dict.fromkeys(deployment["public_name"] for deployment in self.state["deployments"]))
        password = "Acceptance-" + secrets.token_urlsafe(18)
        self.state["member_password"] = password
        expanded = hierarchy(self.data)
        self.progress("    创建 3 个组织、9 个团队、27 名成员、9 个项目和 81 把密钥")
        for org in self.data["organizations"]:
            row, _ = self.api("/organization/new", {"organization_alias": org["name"], "max_budget": self.data["budgets"]["organization"]})
            oid = row["organization_id"]
            self.state["organizations"].append({"id": oid, "name": org["name"]})
            ot = self.template(org["slug"] + "-organization-models", ids, "organization-default", organization_id=oid)
            self.api("/route_template/binding", {"scope": "organization", "scope_id": oid, "route_template_id": ot})
            for ti, team in enumerate(self.data["teams"]):
                members = [m for m in expanded if m["organization"]["slug"] == org["slug"] and m["team"]["slug"] == team["slug"]]
                first = members[0]
                user, _ = self.api("/user/new", {"user_email": first["email"], "user_alias": first["name"], "password": password,
                    "user_role": "user", "max_budget": 10, "admin_organization_ids": [oid] if ti == 0 else []})
                teamrow, _ = self.api("/team/new", {"organization_id": oid, "team_alias": team["name"],
                    "admin_user_id": user["user_id"], "models": allowed_models, "max_budget": 30})
                tid = teamrow["team_id"]
                self.state["teams"].append({"id": tid, "organization_id": oid, "name": team["name"]})
                if ti:
                    tt = self.template(org["slug"] + "-" + team["slug"] + "-team-models", ids, "team-default", organization_id=oid, team_id=tid)
                    self.api("/route_template/binding", {"scope": "team", "scope_id": tid, "route_template_id": tt})
                project, _ = self.api("/project/new", {"team_id": tid, "project_alias": self.data["project_profiles"][ti]["name"], "max_budget": self.data["budgets"]["project"]})
                pid = project["project_id"]
                self.state["projects"].append({"id": pid, "team_id": tid})
                for mi, member in enumerate(members):
                    if mi:
                        user, _ = self.api("/user/new", {"user_email": member["email"], "user_alias": member["name"],
                            "password": password, "user_role": "user", "team_id": tid, "team_role": "user", "max_budget": 10})
                    uid = user["user_id"]
                    self.state["users"].append({"id": uid, "email": member["email"], "name": member["name"], "team_id": tid,
                        "organization_id": oid, "organization_slug": org["slug"], "team_slug": team["slug"], "member_index": mi})
                    for profile in self.data["key_profiles"]:
                        body = {"key_alias": org["name"] + "/" + team["name"] + "/" + member["name"] + "/" + profile,
                            "owner_type": "personal", "user_id": uid, "team_id": tid, "project_id": pid,
                            "models": allowed_models, "max_budget": 3}
                        if profile != "inherit":
                            body["route_template_id"] = fenno if profile == "fenno-only" else qiniu
                        key, _ = self.api("/key/generate", body)
                        call_model = self.data["shared_chat_model"]["public_name"]
                        self.state["keys"].append({"key": key["key"], "token_id": key["token_id"], "profile": profile,
                            "user_id": uid, "team_id": tid, "project_id": pid, "organization_id": oid, "team_index": ti})
                        self.state["keys"][-1]["call_model"] = call_model
                        self.save()
                self.progress(f"    [{org['name']}/{team['name']}] 已创建 3 名成员、1 个项目、9 把密钥；累计团队={len(self.state['teams'])}/9 密钥={len(self.state['keys'])}/81")
            self.progress(f"    [{org['name']}] 组织构建完成；累计成员={len(self.state['users'])}/27")
        self.progress(f"  [构建 5/5] 创建 {len(self.data['guardrails'])} 条护栏并保存数据集检查点")
        for guardrail in self.data["guardrails"]:
            self.api("/guardrails", {k: v for k, v in guardrail.items() if k != "acceptance"})
        # 管理员额外拥有一把无团队的个人虚拟密钥；与 81 把成员密钥分开保存，保持成员会话矩阵稳定。
        admin_key, _ = self.api("/key/generate", {"owner_type": "personal", "user_id": login["user_id"],
            "key_alias": "管理员个人密钥", "models": allowed_models, "max_budget": 3})
        self.state["admin_key"] = admin_key
        self.state["selected"] = selected
        self.report.update(status="seeded", phase="seed", admin_key_count=1, total_key_count=82)
        self.save()


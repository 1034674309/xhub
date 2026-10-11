"""真实验收的 permissions 模块；共享上下文由 Dataset 静态组合，资源与失败证据归本轮所有。"""



class PermissionsCases:
    """用途：封装 permissions 场景；通过 self 访问本轮状态、管理 API 和证据，供 Dataset 调用，不自行启动全套测试。"""

    def verify_permissions(self):
        """用途：按清单登录四角色验收权限；无参数/返回，供真实验收调用；分别检查资料、金额/RPM/TPM、模型、状态和护栏权限。额度及模型使用平台读取的原值，避免改变保留数据的分配和路由，临时护栏 finally 删除；状态码不符立即失败。"""
        personas = []
        for scenario in self.data["permission_scenarios"]:
            role = scenario["role"]
            self.progress(f"    [权限] 角色={role} 开始核对登录、团队可见范围、编辑边界与护栏权限")
            definition = next(p for p in self.data["personas"] if p["role"] == role)
            if role == "platform_admin":
                token, user = self.admin, self.state["users"][0]
            else:
                user = next(u for u in self.state["users"] if u["organization_slug"] == definition["organization"]
                            and u["team_slug"] == definition["team"] and u["member_index"] == definition["member_index"])
                login, _ = self.api("/v2/login", {"username": user["email"], "password": self.state["member_password"]})
                token = login["key"]
            teams, _ = self.api("/team/list", token=token)
            if len(teams) != scenario["visible_teams"]:
                raise AssertionError(role + " 团队可见范围不正确")
            foreign = next(t for t in self.state["teams"] if t["organization_id"] != user["organization_id"])
            for tid, field in ((user["team_id"], "own_team_update"), (foreign["id"], "foreign_team_update")):
                self.api("/team/update", {"team_id": tid, "team_description": "真实验收权限验证"},
                         token=token, expected=scenario[field])
            # 三项额度共用上级分配权限，模型和封禁状态仍由平台管理，不能合并成同一权限断言。
            # 原值写回仍会经过真实鉴权和分配校验；避免给共享验收数据添加分钟限制或清空模型。
            for tid, field in ((user["team_id"], "own_team_limits"),
                               (foreign["id"], "foreign_team_limits")):
                stored, _ = self.api("/team/info?team_id=" + tid)
                info = stored["team_info"]
                for limit in ("max_budget", "rpm_limit", "tpm_limit"):
                    self.api("/team/update", {"team_id": tid, limit: info.get(limit)},
                             token=token, expected=scenario[field])
                if tid == user["team_id"]:
                    for setting, expected_field in (("models", "team_models_update"),
                                                    ("status", "team_status_update")):
                        self.api("/team/update", {"team_id": tid, setting: info[setting]},
                                 token=token, expected=scenario[expected_field])
            created = None
            try:
                created, _ = self.api("/guardrails", {"guardrail_name": "权限临时-" + role,
                    "litellm_params": {"guardrail": "local", "mode": "pre_call", "default_on": False,
                                       "blocked_words": ["权限标记"], "action": "block"}},
                    token=token, expected=scenario["global_guardrail_create"])
            finally:
                if created and created.get("guardrail_id"):
                    self.api("/guardrails/" + created["guardrail_id"], method="DELETE")
            personas.append({"role": role, "email": "admin" if role == "platform_admin" else user["email"],
                             "team_id": user["team_id"], "visible_teams": len(teams)})
            self.report["checks"].append({"name": "permissions-" + role, "passed": True})
            self.action_ok("gpt-5.6-sol + z-ai/glm-5", "identity-rbac", role + " 权限矩阵",
                           f"登录、可见团队={len(teams)}、资料、金额/RPM/TPM分配、跨组织、模型/状态和全局护栏权限均已核对",
                           record=False)
        self.state["personas"] = personas
        self.save()


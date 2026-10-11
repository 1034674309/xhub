# 浏览器 E2E

## 免费隔离入口

~~~bash
make e2e-offline
make e2e-model-endpoints
bash scripts/e2e.sh provider-forms.spec.ts model-credential-protocol.spec.ts
~~~

[脚本](../../../scripts/e2e.sh)使用真实浏览器、控制台、网关和数据库，本地上游不消耗供应商额度。使用稳定角色/标签，不依赖脆弱 CSS；finally 删除自建对象，runner 清理 schema。

| 流程 | 用例 |
| --- | --- |
| 供应商 → 模型发现 → 部署保存 | provider-forms / model-discovery |
| 密钥 → Playground → 日志 | keys-playground |
| 模板 → 绑定 → 实际选路 | product-fixes / route-diagnostic |
| 额度 → 拒绝 → 恢复 | quota-hierarchy / rate-hierarchy |
| 护栏 → 拦截 / 修改 / 放行 | guardrails 相关 spec |

## 付费和共享数据入口

make e2e 与 make e2e-all 会调用真实供应商。make testdata 会清空共享 xhub/public 与 Redis DB 1；它不是隔离入口。未经明确确认不要在工作实例运行。

## 看报告

.e2e/runs/<timestamp>-<pid>/ 保存运行证据，.e2e/latest 指向最近运行。报告必须含完整命令、筛选范围、pass/fail/skip 与清理；只截页面不能证明接口、数据库与数据面通过。

[测试环境](testing.md) · [真实供应商](regression-live.md)

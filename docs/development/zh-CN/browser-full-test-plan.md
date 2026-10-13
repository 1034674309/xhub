# E2E 回归

浏览器场景使用真实控制台、网关、PostgreSQL 和本地上游协议夹具。

```bash
docker compose up -d postgres
cd frontend
npx playwright install chromium
cd ..
make e2e-offline
```

定向执行使用实际场景路径：

```bash
bash scripts/e2e.sh scenarios/pages/docs.test.ts scenarios/docs
```

scripts/e2e.sh 构建服务，检查端口，创建本轮独立 schema，执行 Playwright，最后清理本轮数据。默认端口为控制台 3100、网关 4100、上游 4110，可通过 E2E_UI_PORT、E2E_GW_PORT、E2E_UP_PORT 修改。无路径参数时启用全量覆盖检查。

make testdata 清空配置目标的共享 xhub 数据与 Redis DB 1，启动临时网关建数，结束后关闭临时网关并保留基线。仅用于专用测试环境。make e2e 复用已运行服务验证共享基线，模型场景可能产生费用。make e2e-list 显示分类、前提与费用边界。

E2E_RUN_DIR 指定隔离运行目录，保存 results.json、junit.xml、scenario-coverage.json；E2E_BROWSER_REPORT_DIR 可指定独立证据归档目录。离线协议夹具不证明真实供应商、身份租户和云存储账号互操作结果。

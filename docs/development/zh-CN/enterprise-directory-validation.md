# 企业目录验证

[English](../en/enterprise-directory-validation.md)

后台测试使用隔离 PostgreSQL 与本地 TLS OIDC、JWKS、token 和 Kodo 协议夹具，覆盖 SCIM 生命周期、PATCH 回滚、身份校验及归档 outbox。浏览器场景覆盖目录配置、团队映射、凭据轮换、撤权、SSO 配置、归档持久化与重试。

```bash
make regression
bash scripts/e2e.sh scenarios/directory scenarios/enterprise scenarios/auth
```

前提与清理见[E2E 指南](e2e-regression.md)。通过数量以实际运行报告为准。本地夹具不证明真实 Entra、Okta、Casdoor 或七牛账号互操作。参见[配置边界](enterprise-identity-log-archive-design.md)。

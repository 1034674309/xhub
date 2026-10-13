# 企业身份与 Kodo 归档

[English](../en/enterprise-identity-log-archive-design.md)

当前实现包括 SCIM 目录同步、OIDC Code + PKCE 登录和 Kodo 归档。PostgreSQL 是身份与计费的权威数据源。

1. 部署公开 HTTPS origin，配置至少 32 字符且稳定的服务主密钥，用于加密企业秘密。
2. SSO 与 SCIM 配置独立管理，创建或删除其中一项不创建或删除另一项。需要同步时，在管理面板创建绑定组织的目录，保存一次性 SCIM 凭据，将生成地址配置到身份平台。
3. 同步用户与组，组映射只能选择同组织团队。
4. 注册 OIDC 应用及准确回调地址，稳定身份声明与 SCIM externalId 对齐；先停用保存，再启用测试。
5. 配置 Kodo bucket、region、凭据与 prefix，先停用保存，测试上传和清理，再启用归档。

OIDC 不创建未同步账号，不按邮箱合并身份。团队选择依次考虑手工覆盖、当前有效团队、排序映射。SCIM 提供有限过滤与属性，不提供 Bulk 或嵌套组。归档按计费事件产生对象，仅保存元数据；计费提交后由 outbox 写入 JSONL.gz 对象及 manifest。

实现见 internal/gateway/directory.go 和 internal/iam 中 directory、enterprise、archive 模块。真实租户和 Kodo 账号需独立验证，参见[验证入口](enterprise-directory-validation.md)。

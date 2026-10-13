# Casdoor 接入边界

当前企业身份入口为 OIDC Code + PKCE 和 SCIM 用户、组同步。没有 Casdoor 通讯录轮询、专用扫码执行器或部门快照对账。

接入前确认实例提供满足 XHub 校验要求的 issuer、discovery、JWKS 与 token 端点，并通过 SCIM 同步用户。OIDC 只绑定已同步身份，不按邮箱自动创建或合并账号；用户必须满足当前目录字段约束。稳定身份声明与 SCIM externalId 对齐，回调地址以控制台生成值为准。

组映射只选择目录所属组织内团队。来源系统停用或删除需要实际投递并验证 XHub 撤权结果，不能假定自动传播。

配置见[企业身份与归档](enterprise-identity-log-archive-design.md)，测试见[企业目录验证](enterprise-directory-validation.md)。真实 Casdoor 发行版、权限与投递能力需在目标实例独立验证。

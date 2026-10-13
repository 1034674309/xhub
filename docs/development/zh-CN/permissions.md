# 权限与可见范围

网关按当前身份和成员关系授权。页面隐藏按钮不等于后台拒绝访问。

## 验证入口

~~~bash
go test ./internal/authz -count=1
go test ./internal/gateway -run 'TestVisibilityChain|TestPermission' -count=1
~~~

| 对象 | 规则 |
| --- | --- |
| 平台、组织、团队角色 | 不互相替代 |
| 列表 | 查询层限制范围；客户过滤只能缩小 |
| 个人密钥 | 属于用户；绑定团队时必须验证当前归属，未绑定团队时按用户当前归属和个人计费授权校验 |
| 服务密钥 | 使用声明的服务范围，不冒用成员归属 |
| 模型权限 | 沿身份链收窄 |
| 路由模板 | 不能扩大模型权限或绕过预算 |
| 历史用量 | 成员迁移、密钥变化不重写调用时归属 |

缺失范围不是全局范围；读取失败不能放行。

## 源码

[授权判断](../../../internal/authz/decide.go) · [IAM 模块](../../../internal/iam/readme_cn.md) · [额度链](quota-chain.md)

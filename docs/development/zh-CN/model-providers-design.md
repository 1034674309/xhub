# 供应商扩展的未实现能力

这是待实现能力索引，不是当前接入教程。当前接入用[供应商页面与 Go 示例](provider-catalog.md)。

| 能力 | 当前边界 | 开发前要确定 |
| --- | --- | --- |
| OAuth / 订阅登录 | 未实现通用链 | 授权、刷新、撤销、秘密存储 |
| 新厂商云签名 | 表单目录不等于执行器 | 签名、会话凭据、错误及时间偏差 |
| Suno 音乐 | 无原生执行器 | 创建、状态、结果/回调、真实用量 |
| 独立供应商新资源 API | 不按旧设计稿声称存在 | CRUD、迁移、部署引用与权限 |

~~~bash
rg 'RegisterTransport|RegisterSupplier' internal/provider
rg 'provider_id|catalog_id' internal/catalog internal/llm
~~~

先核对注册和调用链，再提出接口变更；新方案与已实现产品文档分开。

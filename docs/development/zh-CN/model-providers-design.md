# 供应商接入能力边界

当前接入方式见[供应商页面与 Go 示例](provider-catalog.md)。能力由凭据目录、公开端点与 transport 注册共同决定，表单中可选择供应商不代表所有协议都有执行器。

| 能力 | 当前行为 |
| --- | --- |
| OAuth / 订阅登录 | 没有通用供应商授权、刷新与撤销执行链 |
| 云签名 | 仅已注册的供应商签名与凭据方式可执行 |
| Suno 音乐 | 无原生执行器，不能作为 speech 或视频协议调用 |
| 供应商连接管理 | 使用已注册凭据接口；部署引用命名连接 |

~~~bash
rg 'RegisterTransport|RegisterSupplier' internal/provider
rg 'provider_id|catalog_id' internal/catalog internal/llm
~~~

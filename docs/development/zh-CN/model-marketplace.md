# 模型广场与本地目录

广场模型不是部署。点击添加模型，确认供应商、上游协议和价格，保存后才能调用。

## 管理接口片段

需要有权限的管理会话；不是虚拟密钥推理接口。

~~~http
GET /price/catalog
POST /price/model/listing
Content-Type: application/json

{"id":"catalog-model","delisted":true}
~~~

| 记录 | 存储键 |
| --- | --- |
| 完整价格源快照 | price_snapshot/catalog |
| 人工覆盖 | price_model/<model ID> |
| 上下架状态 | price_listing/<model ID> |

默认源为 https://api.modelink.ai/v1/market/models；XHUB_PRICE_FEED_URL 可指定镜像。刷新失败保留已有快照。下架隐藏广场条目，不停用已保存部署；撤回型号保留本地历史。无基础价格的自定义条目恢复默认时返回 409，不抹掉唯一价格。

## 源码与验证

[价格刷新](../../../internal/gateway/models/schedule.go) · [计价](pricing.md)

~~~bash
go test ./internal/catalog ./internal/gateway/models -count=1
~~~

# 内容审核

## 接口行为
供应商内容审核不同于 XHub 护栏，分类依模型。

## 接口与认证

~~~http
POST /v1/moderations
~~~

使用 XHub API 密钥：Authorization: Bearer <XHUB_API_KEY>。不要把供应商密钥当作网关密钥。

Base URL 使用调试台显示的网关根地址，不额外追加 /v1；实际请求使用上面的完整接口路径。

## 请求示例

~~~shell
curl "$XHUB_BASE_URL/v1/moderations" \
  -H "Authorization: Bearer $XHUB_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"YOUR_MODEL_NAME","input":"Hello world."}'
~~~

## 请求参数

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| model | string · required | 公开模型名，部署必须支持此协议。 |
| input | string or array · required | 待分类内容。 |

## 成功响应

~~~json
{
  "results": [
    {
      "flagged": false,
      "categories": {},
      "category_scores": {}
    }
  ]
}
~~~

| 字段 | 类型 |
| --- | --- |
| results | object[] |
| results[].flagged | boolean |
| results[].categories | object |
| results[].category_scores | object |

## 错误与排查

| HTTP | 说明 |
| --- | --- |
| 400 | 请求无效，请检查 JSON、参数和查询。 |
| 401 | XHub 凭据缺失或无效。 |
| 403 | 权限拒绝，请检查模型范围或策略。 |
| 429 | 超过限制，请检查响应详情与 Retry-After。 |

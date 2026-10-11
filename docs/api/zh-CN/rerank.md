# 检索重排

## 接口行为
按原始 index 关联结果，响应字段依登记的重排协议。

## 接口与认证

~~~http
POST /v1/rerank
~~~

使用 XHub API 密钥：Authorization: Bearer <XHUB_API_KEY>。不要把供应商密钥当作网关密钥。

Base URL 使用调试台显示的网关根地址，不额外追加 /v1；实际请求使用上面的完整接口路径。

## 请求示例

~~~shell
curl "$XHUB_BASE_URL/v1/rerank" \
  -H "Authorization: Bearer $XHUB_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"YOUR_MODEL_NAME","query":"What is XHub?","documents":["XHub is a model gateway."],"top_n":1}'
~~~

## 请求参数

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| model | string · required | 公开模型名，部署必须支持此协议。 |
| query / documents | string / array · required | 检索问题与候选文档。 |
| top_n | integer | 返回条数。 |

## 成功响应

~~~json
{
  "results": [
    {
      "index": 0,
      "relevance_score": 0.95
    }
  ]
}
~~~

| 字段 | 类型 |
| --- | --- |
| results | object[] |
| results[].index | number |
| results[].relevance_score | number |

## 错误与排查

| HTTP | 说明 |
| --- | --- |
| 400 | 请求无效，请检查 JSON、参数和查询。 |
| 401 | XHub 凭据缺失或无效。 |
| 403 | 权限拒绝，请检查模型范围或策略。 |
| 429 | 超过限制，请检查响应详情与 Retry-After。 |

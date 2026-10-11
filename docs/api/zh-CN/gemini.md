# Gemini / Vertex 原生协议

## 接口行为
路径使用 URL 编码的公开模型名，还支持 :streamGenerateContent、:countTokens。Vertex 使用 /vertex/v1/models/{model} 及相同操作后缀。上游 OAuth 凭据保留在部署设置。

## 接口与认证

~~~http
POST /v1beta/models/YOUR_MODEL_NAME:generateContent
~~~

使用 XHub API 密钥：Authorization: Bearer <XHUB_API_KEY>。不要把供应商密钥当作网关密钥。

Base URL 使用调试台显示的网关根地址，不额外追加 /v1；实际请求使用上面的完整接口路径。

## 请求示例

~~~shell
curl "$XHUB_BASE_URL/v1beta/models/YOUR_MODEL_NAME:generateContent" \
  -H "Authorization: Bearer $XHUB_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"contents":[{"role":"user","parts":[{"text":"Hello, introduce yourself."}]}]}'
~~~

## 请求参数

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| contents | object[] · required | role 与文本或支持的媒体 parts。 |
| generationConfig / tools | object / array | 生成设置及工具定义。 |

## 成功响应

~~~json
{
  "candidates": [
    {
      "content": {
        "role": "model",
        "parts": [
          {
            "text": "Hello!"
          }
        ]
      }
    }
  ],
  "usageMetadata": {
    "promptTokenCount": 10,
    "candidatesTokenCount": 5
  }
}
~~~

| 字段 | 类型 |
| --- | --- |
| candidates | object[] |
| candidates[].content | object |
| candidates[].content.role | string |
| candidates[].content.parts | object[] |
| candidates[].content.parts[].text | string |
| usageMetadata | object |
| usageMetadata.promptTokenCount | number |
| usageMetadata.candidatesTokenCount | number |

## 错误与排查

| HTTP | 说明 |
| --- | --- |
| 400 | 请求无效，请检查 JSON、参数和查询。 |
| 401 | XHub 凭据缺失或无效。 |
| 403 | 权限拒绝，请检查模型范围或策略。 |
| 429 | 超过限制，请检查响应详情与 Retry-After。 |

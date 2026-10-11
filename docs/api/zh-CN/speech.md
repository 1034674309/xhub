# 语音生成

## 接口行为
实际响应是二进制音频，不是此 JSON 示意，保存并核对 Content-Type。

## 接口与认证

~~~http
POST /v1/audio/speech
~~~

使用 XHub API 密钥：Authorization: Bearer <XHUB_API_KEY>。不要把供应商密钥当作网关密钥。

Base URL 使用调试台显示的网关根地址，不额外追加 /v1；实际请求使用上面的完整接口路径。

## 请求示例

~~~shell
curl "$XHUB_BASE_URL/v1/audio/speech" \
  -H "Authorization: Bearer $XHUB_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"YOUR_MODEL_NAME","input":"Hello, welcome to XHub.","voice":"alloy"}' --output speech.mp3
~~~

## 请求参数

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| model | string · required | 公开模型名，部署必须支持此协议。 |
| input / voice | string · required | 文本与支持的音色 ID。 |
| response_format / speed | string / number | 输出格式与支持的语速。 |

## 成功响应

~~~json
{
  "note": "Binary audio response"
}
~~~

| 字段 | 类型 |
| --- | --- |
| note | string |

## 错误与排查

| HTTP | 说明 |
| --- | --- |
| 400 | 请求无效，请检查 JSON、参数和查询。 |
| 401 | XHub 凭据缺失或无效。 |
| 403 | 权限拒绝，请检查模型范围或策略。 |
| 429 | 超过限制，请检查响应详情与 Retry-After。 |

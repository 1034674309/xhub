# 模型列表

## 接口概述

GET /models：无需密钥发现已启用的公开模型。

### 认证与路径

GET /models 无需密钥，列出已启用公开模型名。显式提供密钥时按权限过滤；GET /v1/models 保留鉴权契约，推理调用始终需要有效密钥。

### 响应与选择

响应的 data 数组列出模型，用 id 作为请求的 model。列表受调用方权限约束，能力需核对部署协议。

### 权限与失败

失效密钥或无权使用推理入口的身份返回 401；没有模型授权时返回空列表。缺少模型时检查允许范围、启用状态和公开名称。

## 协议行为
GET /models 可匿名发现已启用公开模型名。显式凭据按权限过滤，无效密钥返回 401；GET /v1/models 保留鉴权。id 是可调用名，created 固定为 1677610602，owned_by 固定为 openai，不代表实际供应商。空 data 可能表示没有授权模型。

## 接口与认证

~~~http
GET /models
~~~

GET /models 无凭据可公开查询已启用的模型名称。携带显式密钥时按权限过滤，错误密钥返回 401。GET /v1/models 要求鉴权。

Base URL 使用调试台显示的网关根地址，不额外追加 /v1；实际请求使用上面的完整接口路径。

## 请求示例

~~~shell
curl "$XHUB_BASE_URL/models"
~~~

## 请求参数

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| scope | string | 空或 expand；其他值返回 400，expand 扩展管理员可见范围。 |
| team_id | string | 已鉴权会话的团队筛选；密钥保持绑定范围。匿名发现忽略此参数。 |
| return_wildcard_routes | boolean | 保留已配置通配名并展开目录匹配；推理密钥也可包含匹配可见模型的授权通配名。默认 false，匿名发现忽略此参数。 |
| only_model_access_groups | boolean | true 时返回空列表。 |

## 成功响应

~~~json
{
  "object": "list",
  "data": [
    {
      "id": "YOUR_MODEL_NAME",
      "object": "model",
      "created": 1677610602,
      "owned_by": "openai"
    }
  ]
}
~~~

| 字段 | 类型 |
| --- | --- |
| object | string |
| data | object[] |
| data[].id | string |
| data[].object | string |
| data[].created | number |
| data[].owned_by | string |

## 错误与排查

| HTTP | 说明 |
| --- | --- |
| 400 | scope 无效；仅接受空值或 expand。 |
| 401 | XHub 凭据缺失或无效。 |

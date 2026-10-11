# HTTP 响应、校验与模块契约

## 说明

httpx 统一三个网关边界：JSON 与协议专用错误响应、预算和速率请求字段校验、功能模块路由登记。它不做认证、授权、持久化，也不自行挂载路由。网关处理函数用它校验已解码 JSON 并写响应；各功能包用它绑定模块，再由网关路由注册表统一挂载。

## 对外接口

### 响应与调用标识

- `CallID() string` 读取 16 个随机字节并编码为 32 位十六进制。随机源错误被忽略，因此熵源故障时可能得到弱的全零或部分标识；调用 ID 只用于关联，不用于鉴权。
- `SetCallID(w, id)` 设置 `x-litellm-call-id`，会覆盖已有值。
- `WriteJSON(w, status, value)` 补齐调用 ID 和 `x-litellm-version: xhub-dev`，保留已有 Content-Type，写状态码后编码 JSON，并带换行。响应头提交后发生的编码错误会被丢弃，无法改写状态。
- `WriteError` 以空路径调用 `WriteTypedError`。`WriteTypedError` 总是设置 JSON 类型、关联头和版本头，429 额外设置 `Retry-After: 1`。非 chat、非 threads 的 messages 路径使用 Anthropic 结构；生成、流式生成和计数 token 路径使用 Gemini 原生结构；其余使用 OpenAI 结构。Google RPC 状态明确映射 400、401、403、404、429、502、503 和其他 5xx，其他状态归为 INVALID_ARGUMENT。

### 预算与速率字段

- `ValidateBudgetFields(body, fields...)` 将缺字段、null 和空串视为共享或未设置；接受有限非负 float64 或数字字符串；类型错误、解析失败、NaN、无穷和负数返回带字段名的错误。
- `ValidateRateFields(body)` 对 rpm_limit 与 tpm_limit 执行上述校验，并要求整数且不超过 2,147,483,647。
- `RateValue(raw)` 解析 float64 或字符串。`OptionalRate` 用双重指针区分缺字段、显式清空和数值更新；`Rate` 在创建场景把缺失与清空都折叠为 nil。转换函数假设输入已校验，调用方必须先运行校验。

### 模块登记

`Registrar.Handle` 登记“方法加路径”模式。`Module` 提供稳定名字和挂载操作。`Bind(name, mount)` 即使名字为空也会构造模块，进程注册表负责拒绝空名与重名。绑定后的 Mount 在回调或 registrar 为 nil 时直接返回，否则同步执行回调。

## 内部核心实现

响应函数先提交响应头再编码正文，因此后续失败不能改变状态。路径判断先转小写，再按子串选择协议，调用方传入的已登记请求路径属于协议契约。数值校验先处理 JSON 解码得到的 float64 和数字字符串，再检查有限性、范围和整数条件，最后才转 int。模块绑定只保存名字和闭包，使功能包无需依赖具体网关进程类型。

### 测试依赖

`httpx_test.go` 验证协议错误结构和响应头保留；`budget_test.go` 验证预算非法输入与清空；`rates_test.go` 验证小数、边界和指针语义。运行 `go test ./internal/util -count=1`；不依赖外部服务。

## 源码职责与入口

- `httpx.go` 负责调用 ID、通用响应头、JSON 输出、协议识别和 OpenAI/Anthropic/Gemini 错误结构。
- `budget.go` 负责预算与速率校验和转换，包括 PATCH 字段三态语义。
- `module.go` 定义 Registrar、Module、Bind 和所有功能挂载文件使用的私有适配器。
- `httpx_test.go`、`budget_test.go`、`rates_test.go` 分别验证三个生产区域。

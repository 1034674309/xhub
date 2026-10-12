# 内嵌公共目录资源

[English](readme.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

本目录是构建期资源，不提供 Go API。修改 JSON 后由父包 embed 载入；保持数据格式、来源与许可证，重新构建/重启才影响内嵌基线。运行时覆盖走 catalog 管理接口。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| pricedata.json | 价格/供应商内嵌基线 |
| cn_holidays.json | 北京时间计费日历 |
| provider_fields.json | 供应商凭据字段描述 |

### 接入 demo

在仓库根目录执行以下命令。

```sh
python3 - <<'PY'
import json
from pathlib import Path
for name in ("pricedata.json", "cn_holidays.json", "provider_fields.json"):
    json.loads((Path("internal/catalog/publicdata") / name).read_text())
    print(name, "valid JSON")
PY
go test ./internal/catalog -count=1
```

在仓库根目录运行，先显示三个 valid JSON，再执行父包测试；不更新线上资源。

### 错误、状态与副作用

JSON 语法通过不证明费率、假期或供应商字段正确；还要检查消费方语义测试。修改文件不会热更新已运行进程。

## 说明

`internal/catalog/publicdata` 保存父级 `catalog` 包使用的构建期数据。本目录没有 Go package、可变状态或 HTTP handler。资源包括基础价格/供应商目录、北京时间计费日历、供应商凭据字段快照及许可证，以及保留的自动路由预设。`go:embed` 把字节编入二进制，修改文件不会改变运行中的进程；运行时市场刷新和运营覆盖是独立层。

## 对外接口

本目录不导出 Go 符号，父包消费者构成实际接口：

- `pricedata.json` 在 catalog 初始化时解码为 `PriceDocument`；`CostAt`、`CostMap`、`Providers`、模型/供应商修改及 gateway 模型价格接口消费其内存结果。
- `cn_holidays.json` 由 `holiday.go` 解析给 `IsPeakHour`，影响峰谷计费快照。
- `provider_fields.json` 由 `CredentialProviders` 解析，再通过 model-hub/供应商表单使用的 catalog 公共响应返回。
- `autorouter_presets.json` 为兼容保留；文件存在不表示已退役的自动路由被启用。

实际 HTTP 消费者在上层注册：公共模型目录包括 `GET /public/v1/model_hub`、`/public/v1/model_hub/{facet}`、`/public/model_hub`、`/public/model_hub/info`、`/public/endpoints`、`/model_hub`、`/model_hub/{facet}`；管理价格和供应商修改位于 `/price/catalog`、`/price/model`、`/price/provider`。解析或格式错误通过父包初始化/刷新行为暴露，本目录自身不能返回错误。

## 内部核心实现

### 资源契约

`pricedata.json` 是含来源元数据、生成时间、供应商行和模型行的 JSON 文档。模型 ID 是稳定查找键。费率行保留计量单位、输入输出侧、变体、时段、单位大小、来源键及每基础单位 USD；显式零价是真实价格，不能当成缺失。父包复制内嵌基线后再叠加 provider contribution、市场 feed 替换和运营覆盖。

`cn_holidays.json` 区分法定节假日与周末调休工作日。`holiday.go` 先按 Asia/Shanghai 时区和半开区间 09:00–12:00、14:00–18:00 判断，再应用例外。更新日历需重建后才影响未来计算，也不会重写历史价格快照。

`provider_fields.json` 是供应商描述及凭据字段数组，字段包含 key、label、类型、必填、默认值和 placeholder 等。`CredentialProviders` 将其与本地供应商合并，补已知默认 API base，规范 DeepSeek，按 provider/slug 去重，深拷贝新增行并按显示名排序。这些表单只描述配置方式，不保证适配器或外部账户可用。

`provider_fields.LICENSE` 记录凭据字段快照的许可证和来源义务，更新派生数据时必须一并保留。`autorouter_presets.json` 仅为兼容保留。所有 JSON 必须有效；损坏的内嵌资源可能导致进程启动时目录为空或进入回退/错误路径。

### 验证

资源目录没有直接测试。`internal/catalog` 测试覆盖 feed 解码、凭据供应商合并与隔离、节假日/时段边界、费率选择、显式零价、别名及图片/视频变体。运行 `go test ./internal/catalog -count=1`；修改内嵌字节后必须重新构建验证。外部供应商可用性和市场源新鲜度不在这些 fixture 的保证范围。

## 源码职责与入口

### `pricedata.json`

生成的基础价格目录，含元数据、供应商下拉行、模型属性、别名、兼容平铺字段和标准费率表。由 `pricedata.go`/`model_cost.go` 嵌入解码，由 `cost_at.go` 计费，并经 gateway 模型 handler 展示和修改。

### `cn_holidays.json`

中国节假日与调休工作日数据，由 `holiday.go` 用于判断北京时间峰值窗口；它是日历数据，不是调度器。

### `provider_fields.json`

凭据表单快照，由 `credential_providers.go` 消费；在合并本地供应商和默认 API base 前提供显示元数据与字段 schema。

### `provider_fields.LICENSE`

`provider_fields.json` 的许可证声明。运行时不解析，但重新分发和更新时必须与数据一起保留。

### `autorouter_presets.json`

catalog 资源包内嵌的保留预设 JSON；仅有此文件不会启用任何当前路由选择入口。

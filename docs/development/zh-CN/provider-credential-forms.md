# 模型供应商认证表单

打开模型供应商 → 添加提供商，先选择类型，再填写地址和认证。切换类型清除旧认证字段；编辑已保存连接不改变类型。

![中文模型供应商连接表单](../../assets/provider-guide/provider-form-zh.jpg)

## 字段来源

| 定义 | 来源 |
| --- | --- |
| 表单字段 | internal/catalog/publicdata/provider_fields.json |
| 合并后的公开定义 | /public/providers/fields |
| 表单身份 | credential_info.provider_id |
| 连接协议 | custom_llm_provider |

字段快照不代表每个供应商都有执行适配器。表单条目不证明 OAuth、云平台签名或服务账号认证链已实现；没有支持的认证字段不能保存为普通 Key 连接。

## 源码与验证

[表单目录](../../../internal/catalog/credential_providers.go) · [新增供应商](provider-catalog.md)

~~~bash
go test ./internal/catalog -count=1
bash scripts/e2e.sh scenarios/legacy/provider-forms scenarios/legacy/model-credential-protocol scenarios/legacy/model-discovery
~~~

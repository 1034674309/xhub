# 七牛 FAL 视频接入与扩展

先保存七牛模型供应商连接；添加部署时选择对应上游型号、FAL 协议、公开名与实际价格。地址和密钥只在供应商表单填写。

![模型供应商表单](../../assets/provider-guide/provider-form-zh.jpg)

![模型部署表单](../../assets/provider-guide/deployment-form-zh.jpg)

## 创建与查询

会产生供应商费用；kling-25 必须是已保存且获授权的部署。

~~~bash
curl "$XHUB_BASE_URL/queue/fal-ai/kling-video/v2.5-turbo/pro/text-to-video" \
  -H "Authorization: Bearer $XHUB_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"kling-25","prompt":"A red apple on a white table.","duration":"5"}'
~~~

创建返回 request_id。用同一 XHub 身份请求改写后的 status_url / response_url；查询路径属于公共队列，不能直接在具体创建路径后加 requests。多别名共享路径时用 model 指定，发上游前删除该字段。

| 系列 | transport | 查询前缀 |
| --- | --- | --- |
| Seedance 2.0 | qiniu_fal_doubao_20 | /queue/bytedance/seedance-2.0/requests/ |
| Dreamina 2.0 | qiniu_fal_dreamina_20 | /queue/byteplus/seedance-2.0/requests/ |
| Kling | qiniu_fal_kling | /queue/fal-ai/kling-video/requests/ |

完整路径见 [fal.go](../../../internal/provider/qiniu/fal.go)。

## 二开位置

| 修改 | 文件 / 函数 |
| --- | --- |
| 新型号与固定创建路径 | fal.go 的 falEndpoint / registerFal |
| 新队列与认证 | RegisterTransport 的 Actions、TaskID、Auth |
| 实际用量 / 计价规格 | internal/provider/measurement |
| 成功终态结算与任务钉 | internal/dataplane/execution/official.go |

Seedance 使用实际 completion_tokens；Kling/Veo 使用最终 video.duration。Vidu Q2 区间价、MiniMax 复合用量不能用首条费率替代，缺完整价格时保留 unpriced。等待/业务失败不结算；成功重复查询由 PostgreSQL 去重。没有后台补查、预算预占或创建时锁价。未知创建结果不能重复生成。

~~~bash
go test ./internal/provider/... ./internal/catalog ./internal/dataplane/... -count=1
XHUB_REGRESSION_STRICT=1 go test ./cmd/regression -run 'Test(FalSettlement|SeedanceSettlement)' -count=1 -v
~~~

[新协议完整代码](provider-catalog.md) · [任务接口](../../api/zh-CN/fal-kling.md) · [测试前置条件](testing.md)

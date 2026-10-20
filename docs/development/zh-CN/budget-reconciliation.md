# 未知费用的人工对账

硬额度请求已经提交上游、但费用不能可靠确认时，保留 `uncertain` 预占。超时、HTTP 400、额度周期重置和进程重启都不是自动退款依据。

平台管理员使用 `GET /budget/reservations` 查看分页摘要，然后通过 `POST /budget/reservations/{reservation_id}/reconcile` 提交核实结果。普通用户、组织/团队管理员、虚拟 Key 不具备此管理权限。

请求字段：

- `outcome`：`no_charge` 或 `charged`。
- `verified_cost_usd`：这个 **attempt** 的实际费用，不是整个 call 的累计金额；`no_charge` 必须为零，`charged` 必须大于零。
- `evidence`：1–2000 字符的核实依据，不能填写凭据或用户请求正文。
- `expected_fingerprint`：列表返回的预占指纹，避免处理查看后已改变的记录。

列表支持 `key_id`、`state`、`page`、`page_size`；每页最多100行。缺失的账单费用为 `null`，不代表免费。`can_reconcile=false` 和 `blocked_reason` 表示仍缺少可靠核对条件：预占不是 `uncertain`、原调用事件缺失/尚未结束、身份冲突、同 call 仍有在途尝试，或 `usage_pending` 尚未结算。应先恢复持久账单，不能删除待结算记录绕过限制。

一次事务追加不可变 `budget_reconciliations` 证据、按实际增量更新调用费用/日汇总/监控/五层消费、记录 `budget.reconcile` 审计，并删除该 attempt 的预占。请求数、Token 和性能样本不增加。原上游价格与观察值保留，原预占和修改前账单另存证据；原调用费用是原收费加所有已核实尝试费用。实际费用允许超过原预占，因为不能截断已经发生的费用；它可能使现有余额不足，后续请求会正常被额度检查拒绝。

同一尝试重复提交完全相同结论返回 `already_resolved=true`；不同结论、陈旧指纹或条件不满足返回409 `budget_reconciliation_conflict`。非法输入返回400 `budget_reconciliation_invalid`。不支持自动批量退款或直接修改既有核实结论。

核对按原请求时间和冻结身份计费。旧周期费用不占组织当前周期；Key撤销或用户删除不改变历史归属。一个 call 多个尝试可能连接不同供应商，新预占冻结本尝试供应商/上游型号；旧记录缺少供应商时保持未知，不能套用最终成功尝试的供应商。

Redis、PG pending 和独立磁盘队列最终均通过 `RecordUsage` 恢复。已人工核销的尝试，只允许身份及费用一致的迟到重放确认；冲突返回 `ErrConflict` 并保留载荷，不再次收费。未关联人工结论的晚到记录若试图释放已有 call 事件的另一个 hold，也返回冲突，防止事件去重吞掉新费用。

尚无原调用事件、只有崩溃残留 `submitted/reserved` 的记录不在此版本自动处理范围。系统也不会自行查询供应商账单；管理员必须先取得可靠依据。历史生产预占不会因升级自动修改。

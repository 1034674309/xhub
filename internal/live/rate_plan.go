package live

// RateNode 定义限流树；Parent 为父节点数组下标，-1 为根，配置来自 IAM 唯一归属树。
type RateNode struct {
	RateScope
	Parent int
}

// RatePlan 包含计数树和调用路径；Path 从 API Key/个人到根，各层均为合计上限。
type RatePlan struct {
	Nodes []RateNode
	Path  []int
}

// CheckRatePlan 只读校验各层当前分钟合计用量，不为闲置兄弟预留速率容量。
// 参数 plan 为合法无环树，rpm/tpm 与 Nodes 等长，est 为入站估算；返回最窄拒绝字段或空串。
// 调用：本地和 Redis 对照测试；零上限始终拒绝，负估算按零处理，不修改窗口。
func CheckRatePlan(plan RatePlan, rpm, tpm []int64, est int) string {
	for _, i := range plan.Path {
		n := plan.Nodes[i]
		if n.RPM != nil && (*n.RPM == 0 || rpm[i]+1 > int64(*n.RPM)) {
			return n.Kind + " rpm_limit"
		}
		if n.TPM != nil && (*n.TPM == 0 || tpm[i]+int64(max(0, est)) > int64(*n.TPM)) {
			return n.Kind + " tpm_limit"
		}
	}
	return ""
}

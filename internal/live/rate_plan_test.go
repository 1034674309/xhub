package live

import (
	"fmt"
	"reflect"
	"testing"
)

// TestRatePlanLimitChangeKeepsUsage 验证同一分钟从共享改成固定限额不能消除已累计用量。
// 前置纯内存单密钥窗口；覆盖共享、恰好用完、增大后恢复、显式零及 TPM 边界。
// 参数 t 为测试上下文，无返回；检查失败不修改计划或用量，场景无外部资源需要清理。
func TestRatePlanLimitChangeKeepsUsage(t *testing.T) {
	for _, tc := range []struct {
		name     string
		rpm, tpm *int
		want     string
	}{
		{"共享仍累计用量", nil, nil, ""},
		{"RPM低于历史用量", ratePlanTestLimit(1), nil, "key rpm_limit"},
		{"RPM正好耗尽", ratePlanTestLimit(2), nil, "key rpm_limit"},
		{"RPM提高后恢复", ratePlanTestLimit(3), nil, ""},
		{"RPM零始终禁用", ratePlanTestLimit(0), nil, "key rpm_limit"},
		{"TPM下一请求超出", nil, ratePlanTestLimit(29), "key tpm_limit"},
		{"TPM含下一请求正好够用", nil, ratePlanTestLimit(30), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := RatePlan{Nodes: []RateNode{{RateScope: RateScope{ID: "key/historical", Kind: "key", RPM: tc.rpm, TPM: tc.tpm}, Parent: -1}}, Path: []int{0}}
			rpm, tpm := []int64{2}, []int64{20}
			if got := CheckRatePlan(plan, rpm, tpm, 10); got != tc.want {
				t.Fatalf("修改限额必须沿用窗口消费: got=%q want=%q", got, tc.want)
			}
			if !reflect.DeepEqual(rpm, []int64{2}) || !reflect.DeepEqual(tpm, []int64{20}) {
				t.Fatalf("只读准入校验修改窗口: rpm=%v tpm=%v", rpm, tpm)
			}
		})
	}
}

// ratePlanTestLimit 为分钟限额边界表提供独立指针；参数 value 为非负上限，返回指针，无外部副作用。
func ratePlanTestLimit(value int) *int { return &value }

// TestRatePlanAllOrders 穷举小容量完整四层树的全部调用顺序，验证固定 2+2 与共享 1 的充分性和上界。
// 前置纯内存配置；期望来自各叶分配，不复用实现算法。容量内都放行，任一池多一次都拒绝且没有副作用，无需清理。
func TestRatePlanAllOrders(t *testing.T) {
	for _, tokens := range []bool{false, true} {
		t.Run(fmt.Sprint(tokens), func(t *testing.T) {
			total, fixed := 5, 2
			plan := RatePlan{Nodes: []RateNode{
				{RateScope: RateScope{ID: "org/a", Kind: "org"}, Parent: -1},
				{RateScope: RateScope{ID: "team/a", Kind: "team"}, Parent: 0},
				{RateScope: RateScope{ID: "user/a", Kind: "user"}, Parent: 1},
				{RateScope: RateScope{ID: "key/a", Kind: "key"}, Parent: 2},
				{RateScope: RateScope{ID: "user/b", Kind: "user"}, Parent: 1},
				{RateScope: RateScope{ID: "key/b", Kind: "key"}, Parent: 4},
				{RateScope: RateScope{ID: "user/shared", Kind: "user"}, Parent: 1},
				{RateScope: RateScope{ID: "key/shared", Kind: "key"}, Parent: 6},
			}}
			if tokens {
				plan.Nodes[0].TPM = &total
				plan.Nodes[3].TPM = &fixed
				plan.Nodes[4].TPM = &fixed
			} else {
				plan.Nodes[0].RPM = &total
				plan.Nodes[3].RPM = &fixed
				plan.Nodes[4].RPM = &fixed
			}
			paths := [][]int{{3, 2, 1, 0}, {5, 4, 1, 0}, {7, 6, 1, 0}}
			capacities := []int{2, 2, 1}
			orders := 0
			// 每个前缀都尝试三种下一请求，包含超限失败分支；仅沿容量内的分支继续枚举。
			var visit func([]int, []int64, []int64)
			visit = func(used []int, rpm, tpm []int64) {
				if used[0]+used[1]+used[2] == total {
					orders++
				}
				for candidate, path := range paths {
					plan.Path = path
					rejected := CheckRatePlan(plan, rpm, tpm, 1)
					if used[candidate] == capacities[candidate] {
						if rejected == "" {
							t.Fatalf("已用%v，第%d池多一次仍放行", used, candidate)
						}
						continue
					}
					if rejected != "" {
						t.Fatalf("已用%v，第%d池保留容量被挤占: %s", used, candidate, rejected)
					}
					nextRPM, nextTPM := append([]int64(nil), rpm...), append([]int64(nil), tpm...)
					for _, i := range path {
						nextRPM[i]++
						nextTPM[i]++
					}
					used[candidate]++
					visit(used, nextRPM, nextTPM)
					used[candidate]--
				}
			}
			visit(make([]int, 3), make([]int64, 8), make([]int64, 8))
			// 5!/(2!×2!×1!) = 30，枚举总数必须匹配，否则可能漏掉最坏调用次序。
			if orders != 30 {
				t.Fatalf("完整顺序数 %d，预期 30", orders)
			}
		})
	}
}

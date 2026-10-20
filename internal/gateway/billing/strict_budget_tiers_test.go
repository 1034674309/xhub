package billing

import (
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sunqirui1987/xhub/internal/catalog"
	"github.com/sunqirui1987/xhub/internal/config"
)

// tierUnit 提供纯内存合成USD/token报价；参数是每Token价格，故意使用千Token单位验证正确换算，无需清理。
func tierUnit(price float64) map[string]any {
	return map[string]any{"unit_name": "token", "unit_size": 1000, "unit_price_usd": price * 1000}
}

// tierRule 创建隔离上下文分档；参数为输入下/上界与倍数，输出范围覆盖测试上限，返回完整缓存读写报价。
func tierRule(low, high int, multiplier float64) map[string]any {
	return map[string]any{"input_range": []int{low, high}, "output_range": []int{0, 100}, "details_v2": map[string]any{"ncache": tierUnit(multiplier * 0.001), "output": tierUnit(multiplier * 0.002), "cache": tierUnit(multiplier * 0.0001), "c_cache": tierUnit(multiplier * 0.00125)}}
}

// tierDeployment 创建纯内存分档部署；参数为独立规则数组，返回有完整可信窗口的部署，不访问外部目录。
func tierDeployment(rules ...map[string]any) config.ModelEntry {
	d := strictBudgetDeployment()
	d.ModelInfo["max_input_tokens"] = 100
	d.ModelInfo["max_output_tokens"] = 10
	d.LiteLLMParams = map[string]any{"model": "tier-unit", "pricing_rules_v2": rules}
	return d
}

// tierPlan 对合成分档创建实际Chat计划；返回冻结计划，正常输入失败时立即定位；无外部资源。
func tierPlan(t *testing.T, d config.ModelEntry) BudgetPlan {
	t.Helper()
	p, _, err := StrictBudgetPlan(d, []byte(`{"messages":[{"role":"user","content":"Test tiers."}],"max_tokens":10}`), "openai-chat", time.Date(2026, 10, 20, 2, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestStrictBudgetTierFrozenCharges 验证全窗最坏预占、阈值两侧实扣和共享边界不猜价格；目录/配置热更新不能改变已冻结账单。
func TestStrictBudgetTierFrozenCharges(t *testing.T) {
	low, high := tierRule(0, 50, 1), tierRule(50, 200, 2)
	d := tierDeployment(low, high)
	p := tierPlan(t, d)
	if math.Abs(p.Amount-0.49) > 2e-12 {
		t.Fatalf("未预留高档与缓存写入最坏价格: %v", p.Amount)
	}
	high["details_v2"].(map[string]any)["output"] = tierUnit(99)
	for _, tc := range []struct {
		name                 string
		in, out, read, write int
		want                 float64
		bad                  bool
	}{
		{"below", 49, 2, 0, 0, 0.053, false}, {"above", 51, 2, 0, 0, 0.110, false}, {"boundary", 50, 2, 0, 0, 0, true},
		{"cached read selects total context", 51, 2, 30, 0, 0.056, false}, {"cache write selects total context", 45, 2, 0, 6, 0.113, false},
		{"cache write boundary", 45, 2, 0, 5, 0, true}, {"max input", 100, 10, 0, 0, 0.24, false}, {"write over declared input", 100, 2, 0, 1, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := p.Charge(catalog.Usage{PromptTokens: tc.in, CompletionTokens: tc.out, CachedTokens: tc.read, CacheWriteTokens: tc.write})
			if tc.bad {
				strictBudgetExpectError(t, err, "invalid_usage")
				return
			}
			if err != nil || math.Abs(c.Total-tc.want) > 1e-12 {
				t.Fatalf("实际分档结算错误: %+v %v want %v", c, err, tc.want)
			}
		})
	}
	var snapshot map[string]any
	if json.Unmarshal([]byte(p.PriceSnapshot), &snapshot) != nil || len(snapshot["tiers"].([]any)) != 2 || snapshot["interval_semantics"] != "shared_boundaries_require_equal_rates" {
		t.Fatalf("未保存完整冻结规则: %s", p.PriceSnapshot)
	}
}

// TestStrictBudgetTierInvalidSource 验证区间、USD、单位、条件、完整覆盖和费率缺失失败输入；每个子例独立构造，不写全局目录。
func TestStrictBudgetTierInvalidSource(t *testing.T) {
	for _, name := range []string{"gap", "overlap", "negative", "fractional", "missing range", "reverse range", "missing USD", "only CNY", "negative USD", "NaN USD", "zero unit", "fractional unit", "missing unit", "wrong unit", "missing output", "missing input", "empty details", "condition", "unknown metadata", "item order scalar", "item order mixed", "item order null", "unknown unit metadata", "legacy details", "unknown price", "duplicate input", "empty rules", "too many rules", "output gap", "output overlap", "incomplete window", "unpriced cache hint"} {
		t.Run(name, func(t *testing.T) {
			a, b := tierRule(0, 50, 1), tierRule(50, 200, 2)
			d := tierDeployment(a, b)
			details := a["details_v2"].(map[string]any)
			unit := details["ncache"].(map[string]any)
			body := []byte(`{"messages":[{"role":"user","content":"Test invalid pricing."}],"max_tokens":10}`)
			switch name {
			case "gap":
				b["input_range"] = []int{51, 200}
			case "overlap":
				b["input_range"] = []int{49, 200}
			case "negative":
				a["input_range"] = []int{-1, 50}
			case "fractional":
				a["input_range"] = []float64{0, 50.5}
			case "missing range":
				delete(a, "output_range")
			case "reverse range":
				a["input_range"] = []int{50, 0}
			case "missing USD":
				delete(unit, "unit_price_usd")
			case "only CNY":
				delete(unit, "unit_price_usd")
				unit["unit_price"] = 0.123
			case "negative USD":
				unit["unit_price_usd"] = -1
			case "NaN USD":
				unit["unit_price_usd"] = math.NaN()
			case "zero unit":
				unit["unit_size"] = 0
			case "fractional unit":
				unit["unit_size"] = 0.5
			case "missing unit":
				delete(unit, "unit_name")
			case "wrong unit":
				unit["unit_name"] = "second"
			case "missing output":
				delete(details, "output")
			case "missing input":
				delete(details, "ncache")
			case "empty details":
				a["details_v2"] = map[string]any{}
			case "condition":
				a["condition"] = "premium"
			case "unknown metadata":
				a["region"] = "US"
			case "item order scalar":
				a["item_order"] = "input"
			case "item order mixed":
				a["item_order"] = []any{"ncache", 42}
			case "item order null":
				a["item_order"] = nil
			case "unknown unit metadata":
				unit["condition"] = "premium"
			case "legacy details":
				a["details"] = map[string]any{"input": 1}
			case "unknown price":
				details["image_input"] = tierUnit(1)
			case "duplicate input":
				details["input"] = tierUnit(1)
			case "empty rules":
				d.LiteLLMParams["pricing_rules_v2"] = []any{}
			case "too many rules":
				rules := []any{}
				for i := 0; i < 65; i++ {
					rules = append(rules, tierRule(i, i+1, 1))
				}
				d.LiteLLMParams["pricing_rules_v2"] = rules
			case "output gap":
				a["input_range"] = []int{0, 200}
				b["input_range"] = []int{0, 200}
				a["output_range"] = []int{0, 5}
				b["output_range"] = []int{6, 100}
			case "output overlap":
				a["input_range"] = []int{0, 200}
				b["input_range"] = []int{0, 200}
				a["output_range"] = []int{0, 6}
				b["output_range"] = []int{5, 100}
			case "incomplete window":
				b["input_range"] = []int{50, 99}
			case "unpriced cache hint":
				delete(details, "c_cache")
				body = []byte(`{"messages":[{"role":"user","content":"Test cache."}],"cache_control":{"type":"ephemeral"},"max_tokens":10}`)
			}
			_, _, err := StrictBudgetPlan(d, body, "openai-chat", time.Now())
			strictBudgetExpectError(t, err, "unsupported_budget")
		})
	}
}

// TestStrictBudgetTierEqualBoundaryAndOutput 验证等价共边界可结算、输出分档与二维矩形覆盖；所有价格为本地合成数据。
func TestStrictBudgetTierEqualBoundaryAndOutput(t *testing.T) {
	t.Run("same rates", func(t *testing.T) {
		p := tierPlan(t, tierDeployment(tierRule(0, 50, 1), tierRule(50, 200, 1)))
		c, err := p.Charge(catalog.Usage{PromptTokens: 50, CompletionTokens: 2})
		if err != nil || math.Abs(c.Total-0.054) > 1e-12 {
			t.Fatalf("等价边界错误: %+v %v", c, err)
		}
	})
	t.Run("output tiers", func(t *testing.T) {
		a, b := tierRule(0, 200, 1), tierRule(0, 200, 2)
		a["output_range"] = []int{0, 5}
		b["output_range"] = []int{5, 100}
		p := tierPlan(t, tierDeployment(a, b))
		for _, n := range []int{4, 5, 6} {
			c, err := p.Charge(catalog.Usage{PromptTokens: 10, CompletionTokens: n})
			if n == 5 {
				strictBudgetExpectError(t, err, "invalid_usage")
			} else if err != nil || c.Total <= 0 {
				t.Fatalf("输出分档%d错误: %+v %v", n, c, err)
			}
		}
	})
	t.Run("zero price", func(t *testing.T) {
		p := tierPlan(t, tierDeployment(tierRule(0, 200, 0)))
		c, err := p.Charge(catalog.Usage{PromptTokens: 50, CompletionTokens: 2})
		if err != nil || p.Amount != 0 || c.Total != 0 {
			t.Fatalf("显式免费不是缺失价: %+v %v", c, err)
		}
	})
	t.Run("2d coverage", func(t *testing.T) {
		rules := []map[string]any{}
		for _, in := range [][2]int{{0, 50}, {50, 200}} {
			for _, out := range [][2]int{{0, 5}, {5, 100}} {
				r := tierRule(in[0], in[1], 1)
				r["output_range"] = out
				rules = append(rules, r)
			}
		}
		p := tierPlan(t, tierDeployment(rules...))
		if _, err := p.Charge(catalog.Usage{PromptTokens: 50, CompletionTokens: 5}); err != nil {
			t.Fatal(err)
		}
	})
}

// TestStrictBudgetTierCatalogProductionShapes 使用仓库真实目录结构验证阶梯价格不会退回首档；不连接供应商、不改变部署或目录。
func TestStrictBudgetTierCatalogProductionShapes(t *testing.T) {
	for _, model := range []string{"openai/gpt-5.6-sol", "openai/gpt-6.1-sol", "x-ai/grok-4.5", "deepseek/deepseek-v4.1-flash"} {
		t.Run(model, func(t *testing.T) {
			d := config.ModelEntry{ModelName: model, LiteLLMParams: map[string]any{"model": model}, ModelInfo: map[string]any{"base_model": model}}
			p, _, err := StrictBudgetPlan(d, []byte(`{"input":"Verify catalog tiers.","max_output_tokens":32}`), "openai-responses", time.Date(2026, 10, 20, 2, 0, 0, 0, time.UTC))
			if err != nil {
				t.Fatal(err)
			}
			if len(p.frozen.Tiers) == 0 {
				t.Fatal("原始价格档位被丢失")
			}
			c, err := p.Charge(catalog.Usage{PromptTokens: 1000, CompletionTokens: 2})
			if err != nil || c.Total <= 0 {
				t.Fatalf("正常小请求不能按目录结算: %+v %v", c, err)
			}
			if strings.Contains(model, "gpt-5.6-sol") && p.frozen.Input.USD != 0.00001 {
				t.Fatalf("仍用首档预占: %+v", p.frozen.Input)
			}
		})
	}
}

// TestStrictBudgetTrustedLegacyReasoningCap 验证只有管理员明确bool认证和来源可解锁旧cap推理；无外部依赖，保留原输出cap值。
func TestStrictBudgetTrustedLegacyReasoningCap(t *testing.T) {
	for _, tc := range []struct {
		name          string
		value, source any
		valid         bool
	}{{"trusted", true, "https://example.test/provider-contract", true}, {"false", false, "contract", false}, {"missing", nil, "contract", false}, {"string", "true", "contract", false}, {"no source", true, nil, false}, {"empty source", true, " ", false}, {"invalid source", true, 123, false}} {
		t.Run(tc.name, func(t *testing.T) {
			d := strictBudgetDeployment()
			d.ModelInfo["max_tokens_includes_reasoning"] = tc.value
			d.ModelInfo["max_tokens_semantics_source"] = tc.source
			p, out, err := StrictBudgetPlan(d, []byte(`{"messages":[{"role":"user","content":"Reason within the limit."}],"max_tokens":32,"reasoning_effort":"high"}`), "openai-chat", time.Now())
			if !tc.valid {
				strictBudgetExpectError(t, err, "unsupported_budget")
				return
			}
			if err != nil || p.OutputCap != 32 || !strings.Contains(p.PriceSnapshot, "cap_semantics_source") || !strings.Contains(string(out), `"max_tokens":32`) {
				t.Fatalf("认证的总输出cap未保留: %s %v", out, err)
			}
		})
	}
}

// TestStrictBudgetCatalogClearedOverrides 验证UI切回目录的null清除语义；显式manual与非null错误覆盖不得偷偷回落目录，纯内存无需清理。
func TestStrictBudgetCatalogClearedOverrides(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		value        any
		valid        bool
	}{{"catalog null", "catalog", nil, true}, {"implicit catalog null", "", nil, true}, {"manual null", "manual", nil, false}, {"invalid string", "catalog", "invalid", false}, {"invalid bool", "catalog", false, false}, {"empty rates", "catalog", []any{}, false}} {
		t.Run(tc.name, func(t *testing.T) {
			d := config.ModelEntry{ModelName: "deepseek/deepseek-v4.1-flash", LiteLLMParams: map[string]any{"model": "deepseek/deepseek-v4.1-flash", "input_cost_per_token": nil, "output_cost_per_token": nil, "cache_read_input_token_cost": nil, "rates": nil}, ModelInfo: map[string]any{"pricing_source": tc.source, "base_model": "deepseek/deepseek-v4.1-flash"}}
			if tc.name == "empty rates" {
				d.LiteLLMParams["rates"] = tc.value
			} else {
				d.LiteLLMParams["input_cost_per_token"] = tc.value
			}
			p, _, err := StrictBudgetPlan(d, []byte(`{"input":"Verify cleared catalog overrides.","max_output_tokens":32}`), "openai-responses", time.Now())
			if !tc.valid {
				strictBudgetExpectError(t, err, "unsupported_budget")
				return
			}
			if err != nil || p.Amount <= 0 || len(p.frozen.Tiers) == 0 {
				t.Fatalf("目录null覆盖未恢复原始费率: %+v %v", p, err)
			}
		})
	}
}

// TestStrictBudgetRuntimeTierShapes 使用2026-10-20运行时公开报价快照验证GPT/Claude展示字段、完整窗口和真实USD；无网络，临时目录行清理。
func TestStrictBudgetRuntimeTierShapes(t *testing.T) {
	raw, err := os.ReadFile("testdata/runtime-tier-prices-20261020.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		t.Run(row["id"].(string), func(t *testing.T) {
			name := "runtime-tier-fixture-" + row["id"].(string)
			catalog.SetModel(name, row)
			t.Cleanup(func() { catalog.RemoveModel(name) })
			dep := config.ModelEntry{ModelName: name, LiteLLMParams: map[string]any{"model": name}, ModelInfo: map[string]any{"base_model": name, "pricing_source": "catalog"}}
			p, _, err := StrictBudgetPlan(dep, []byte(`{"input":"Verify runtime price snapshot.","max_output_tokens":32}`), "openai-responses", time.Date(2026, 10, 20, 2, 0, 0, 0, time.UTC))
			if err != nil {
				t.Fatal(err)
			}
			expectedInput, expectedCharge := 1000000, .00202
			if strings.Contains(name, "gpt") {
				expectedInput, expectedCharge = 1050000, .0101
				if p.frozen.Input.USD != .00002 {
					t.Fatalf("高档预占价错误: %v", p.frozen.Input.USD)
				}
			}
			c, err := p.Charge(catalog.Usage{PromptTokens: 1000, CompletionTokens: 2})
			if err != nil || p.InputBound != expectedInput || math.Abs(c.Total-expectedCharge) > 1e-12 {
				t.Fatalf("运行时目录计价错误: %+v bound=%d err=%v", c, p.InputBound, err)
			}
		})
	}
}

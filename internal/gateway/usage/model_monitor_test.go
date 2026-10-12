package usage

import (
	"github.com/sunqirui1987/xhub/internal/config"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

// TestModelMonitorOptions 使用无账单配置验证目录去重、连接隔离、许可别名与供应商壳；无持久化及清理需求。
func TestModelMonitorOptions(t *testing.T) {
	list := []config.ModelEntry{
		{ModelName: "a", LiteLLMParams: map[string]any{"model": "actual", "litellm_credential_name": "supplier-a"}},
		{ModelName: "alias-a", LiteLLMParams: map[string]any{"model": "actual", "litellm_credential_name": "supplier-a"}},
		{ModelName: "b", LiteLLMParams: map[string]any{"model": "actual", "litellm_credential_name": "supplier-b"}},
		{ModelName: "shell", LiteLLMParams: map[string]any{"litellm_credential_name": "supplier-c"}, ModelInfo: map[string]any{"role": "provider"}},
		{ModelName: " "},
	}
	for _, tc := range []struct {
		name    string
		list    []config.ModelEntry
		allowed []string
		want    []modelMonitorOption
	}{
		{"unrestricted", list, nil, []modelMonitorOption{{"supplier-a", "actual"}, {"supplier-b", "actual"}, {"supplier-c", ""}}},
		{"restricted alias", list, []string{"b"}, []modelMonitorOption{{"supplier-b", "actual"}}},
		{"denied", list, []string{}, []modelMonitorOption{}},
		{"unknown grant", list, []string{"missing"}, []modelMonitorOption{}},
		{"empty config", nil, nil, []modelMonitorOption{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := modelMonitorOptions(tc.list, tc.allowed); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("配置筛选目录错误: %+v，期望 %+v", got, tc.want)
			}
		})
	}
}

// TestModelMonitorQuery 验证默认七日、四分之一小时时区、筛选和错误日期；无写入及清理需求。
func TestModelMonitorQuery(t *testing.T) {
	now := time.Date(2026, 1, 2, 20, 0, 0, 0, time.UTC)
	q, tz, err := modelMonitorQuery(httptest.NewRequest("GET", "/?timezone=-345&supplier=a&model=m", nil), now)
	if err != nil || tz != 345 || q.Supplier != "a" || q.Model != "m" || q.To.Sub(q.From) != 7*24*time.Hour || q.To.UTC().Format(time.RFC3339) != "2026-01-03T18:15:00Z" {
		t.Fatalf("默认日期/筛选错误: %+v %d %v", q, tz, err)
	}
	for _, raw := range []string{"timezone=x", "timezone=1", "timezone=900", "start_date=no", "end_date=no", "start_date=2026-02-01&end_date=2026-01-01", "start_date=2026-01-01&end_date=2026-04-01"} {
		if _, _, err := modelMonitorQuery(httptest.NewRequest("GET", "/?"+raw, nil), now); err == nil {
			t.Fatal("非法查询被接受", raw)
		}
	}
}

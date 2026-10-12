package iam

import "testing"

// TestSelectRouteTemplate 验证密钥、团队、组织优先级及空白边界；纯内存输入，无清理或数据库依赖。
func TestSelectRouteTemplate(t *testing.T) {
	for _, tc := range []struct {
		ids        []string
		id, source string
	}{
		{nil, "", "builtin"}, {[]string{" ", "", "org"}, "org", "organization"},
		{[]string{"", "team", "org"}, "team", "team"}, {[]string{" key ", "team", "org"}, "key", "key"},
		{[]string{"", "", "", "ignored"}, "", "builtin"},
	} {
		id, source := SelectRouteTemplate(tc.ids...)
		if id != tc.id || source != tc.source {
			t.Fatalf("模板优先级错误: ids=%v got=%s/%s", tc.ids, id, source)
		}
	}
}

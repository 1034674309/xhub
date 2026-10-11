package registry

import (
	"strings"

	providercontract "github.com/sunqirui1987/xhub/internal/provider/contract"
)

// Match 找出这个方法和路径命中的 bypass 动作，只在已登记的转发方式里找。
//
// 它刻意不读部署上的自定义文档：bypass 是后台登记的形状，不是运维在界面上
// 随手填的一份路径表。一份填错的路径表发不出请求，也就拿不到上游的返回值，
// 预选、日志和用量都无从谈起。
//
// 参数 method（string）：HTTP 方法，例如 GET 或 POST；path（string）：URL 路径；
// 返回 Hit（Hit）：命中的转发方式和动作；bool（bool）：命中时为真。
// 调用：gateway/bypass.go
// 测试：bypass_logic_test.go、match_test.go
func Match(method, path string) (providercontract.Hit, bool) {
	method = strings.ToUpper(method)
	path = strings.TrimSuffix(path, "/")
	for _, t := range Transports() {
		if t.Kind != providercontract.KindBypass {
			continue
		}
		for _, action := range t.Actions {
			if action.Method != method {
				continue
			}
			names, ok := matchPath(action.PublicPath, path)
			if !ok {
				continue
			}
			if strings.HasPrefix(action.PublicPath, "/bypass/") && (t.Protocol == "gemini" || t.Protocol == "vertex") {
				t.EndpointID = "bypass:" + t.Protocol
			}
			return providercontract.Hit{Transport: t, Action: action, Names: names}, true
		}
	}
	return providercontract.Hit{}, false
}

// 把路径模板和真实路径比较，抽出花括号里的参数；Google 的末尾模型占位符允许包含多段路径。
// 参数 pattern（string）：路径模板；path（string）：真实路径。
// 返回 map[string]string（map[string]string）：抽出的参数；bool（bool）：逐段对上时为真。
// 调用：仅在 registry.go 内使用
// 测试：无直接单测
func matchPath(pattern, path string) (map[string]string, bool) {
	pattern = strings.TrimSuffix(pattern, "/")
	pp := strings.Split(strings.Trim(pattern, "/"), "/")
	aa := strings.Split(strings.Trim(path, "/"), "/")
	if pattern == "" || path == "" {
		return nil, false
	}
	// 只扩展末尾 {model}:操作，其他任务 ID 仍必须逐段匹配，避免扩大任意队列路径的匹配范围。
	if len(pp) > 0 && strings.HasPrefix(pp[len(pp)-1], "{model}:") && len(aa) >= len(pp) {
		start := len(pp) - 1
		model := strings.Join(aa[start:], "/")
		suffix := strings.TrimPrefix(pp[start], "{model}")
		if !strings.HasSuffix(model, suffix) {
			return nil, false
		}
		for _, segment := range strings.Split(strings.TrimSuffix(model, suffix), "/") {
			if strings.TrimSpace(segment) == "" || segment == "." || segment == ".." {
				return nil, false
			}
		}
		aa = append(aa[:start], model)
	}
	if len(pp) != len(aa) {
		return nil, false
	}
	names := map[string]string{}
	for i := range pp {
		// 原生 Gemini 将模型占位符与操作后缀放在同一段，后缀必须精确匹配。
		if strings.HasPrefix(pp[i], "{") && strings.Contains(pp[i], "}") {
			end := strings.Index(pp[i], "}")
			suffix := pp[i][end+1:]
			if !strings.HasSuffix(aa[i], suffix) {
				return nil, false
			}
			value := strings.TrimSuffix(aa[i], suffix)
			if value == "" || value == "." || value == ".." {
				return nil, false
			}
			names[pp[i][1:end]] = value
			continue
		}
		if pp[i] != aa[i] {
			return nil, false
		}
	}
	return names, true
}

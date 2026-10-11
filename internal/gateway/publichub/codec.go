package publichub

// asFloat converts a JSON number to float64. An int is accepted. Any other type returns 0.
// 参数 v（any）：JSON 里读出的动态值。数字、字符串和对象都要接住，类型不符时按零值而不是 panic。
// 返回 float64（float64）：作为小数。缺失时为 0，不要把它理解成免费除非调用方另有约定。
// 调用：gateway/publichub/public_hub.go。
// 测试：无直接单测
func asFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	default:
		return 0
	}
}

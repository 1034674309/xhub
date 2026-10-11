package execution

// FragmentedErrorReader 模拟输出后失败的上游流；只用于执行器测试。
type FragmentedErrorReader struct {
	Parts [][]byte
	Err   error
}

// Read 分段返回测试流；参数为读缓冲，返回字节数或预设错误。仅内存测试，无清理副作用。
func (r *FragmentedErrorReader) Read(p []byte) (int, error) {
	if len(r.Parts) == 0 {
		return 0, r.Err
	}
	part := r.Parts[0]
	r.Parts = r.Parts[1:]
	return copy(p, part), nil
}

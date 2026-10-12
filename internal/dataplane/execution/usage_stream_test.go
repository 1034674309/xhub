// 本文件验证 execution 的相关执行契约，阅读顺序为测试宿主/辅助方法，再到各个 Test 场景。
// 具体前置、期望、边界和清理方式见每个测试方法；夹具的空实现仅用于隔离当前测试。
package execution

// FragmentedErrorReader 按 Parts 顺序返回片段，耗尽后返回 Err，模拟 SSE 在任意读取边界分块或断流。
// 每次 Read 弹出整个片段；缓冲区比片段小时未拷贝部分会丢失，因此测试应提供足够大的读取缓冲。
type FragmentedErrorReader struct {
	// 尚未返回的字节片段，Read 会弹出首项，测试需保证读取缓冲足够大。
	Parts [][]byte
	// 片段耗尽后返回的预设错误，通常为断流错误。
	Err error
}

// Read 为断流测试按顺序返回片段。
// 参数 p 为目标缓冲区；返回拷贝字节数与 nil，Parts 耗尽后返回 0 与 Err。
// 读取会弹出一个完整片段，p 太小时未拷贝尾部不保留；只适用于本测试的大缓冲读取，无外部资源。
func (r *FragmentedErrorReader) Read(p []byte) (int, error) {
	if len(r.Parts) == 0 {
		return 0, r.Err
	}
	part := r.Parts[0]
	r.Parts = r.Parts[1:]
	return copy(p, part), nil
}

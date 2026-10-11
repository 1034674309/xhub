package registry

import (
	"sync"
	"testing"

	"github.com/sunqirui1987/xhub/internal/provider/contract"
)

// TestTransportRegistryIsolation 验证注册与读取均隔离可变配置，方法规范化不修改输入；前置临时传输与失败输入。
// 参数 t 为单测上下文；返回无，验证切片、映射和回调容器；清理恢复进程内注册表，无网络依赖。
func TestTransportRegistryIsolation(t *testing.T) {
	mu.Lock()
	saved := transports
	transports = nil
	mu.Unlock()
	t.Cleanup(func() { mu.Lock(); transports = saved; mu.Unlock() })
	source := contract.Transport{ID: " local ", Kind: contract.KindBypass, Providers: []string{"trusted"}, Headers: map[string]string{"version": "original"}, SupplierPrefixes: map[string]string{"trusted": "/v1"}, Actions: []contract.Action{{Name: "create", Method: " post ", PublicPath: "/local", UpstreamPath: "/v1/local"}}, Billing: &contract.TaskBilling{}}
	RegisterTransport(contract.Transport{})
	RegisterTransport(contract.Transport{ID: "missing-actions"})
	RegisterTransport(source)
	if source.Actions[0].Method != " post " {
		t.Fatal("注册改变了输入动作")
	}
	source.Actions[0].PublicPath = "/forged"
	source.Providers[0] = "forged"
	source.Headers["version"] = "forged"
	source.SupplierPrefixes["trusted"] = "/forged"
	source.Billing.Context = func(map[string]any) contract.TaskContext { return contract.TaskContext{Known: true} }
	check := func() {
		got := Transports()
		if len(got) != 1 {
			t.Fatalf("无效注册污染目录: %d", len(got))
		}
		tr := got[0]
		if tr.ID != "local" || tr.Actions[0].Method != "POST" || tr.Actions[0].PublicPath != "/local" || tr.Providers[0] != "trusted" || tr.Headers["version"] != "original" || tr.SupplierPrefixes["trusted"] != "/v1" || tr.Billing.Context != nil {
			t.Fatalf("注册表配置被调用方修改: %+v", tr)
		}
	}
	check()
	copy := Transports()[0]
	copy.Actions[0].PublicPath = "/forged"
	copy.Providers[0] = "forged"
	copy.Headers["version"] = "forged"
	copy.SupplierPrefixes["trusted"] = "/forged"
	copy.Billing.Context = source.Billing.Context
	check()
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range 20 {
				tr := Transports()[0]
				tr.Headers["version"] = "concurrent"
				tr.Actions[0].PublicPath = "/concurrent"
			}
		}()
	}
	workers.Wait()
	check()
}

package openai_test

import (
	"testing"

	_ "github.com/sunqirui1987/xhub/internal/provider/all"
	providercapability "github.com/sunqirui1987/xhub/internal/provider/capability"
	providercontract "github.com/sunqirui1987/xhub/internal/provider/contract"
	providerregistry "github.com/sunqirui1987/xhub/internal/provider/registry"
)

// TestAdaptedTypesAreNotRegistered 验证注册表仅登记真实上游执行协议，公开适配入口由能力目录与 family 处理。
// 参数 t 为单测上下文；前置全部内置供应商注册，返回无；只读目录，无清理。
func TestAdaptedTypesAreNotRegistered(t *testing.T) {
	for _, transport := range providerregistry.Transports() {
		if transport.Kind == providercontract.KindAdapted {
			t.Fatalf("an adapted transport was registered: %+v; adaptations are described by the capability table", transport)
		}
	}
	// 适配的入口不进 bypass 匹配：chat 由 family 循环接走。
	if _, ok := providerregistry.Match("POST", "/v1/chat/completions"); ok {
		t.Fatal("chat is adapted and must stay out of bypass matching")
	}
}

// TestCapabilityTableDescribesTheAdaptedEntrypoints 证明七条适配类型变成了能力表。
//
// 对应关系：chat、completion、embedding、image、video、audio_speech、rerank
// 这几条能力都在，而且 image 把生图和改图收成了一条。
// 参数 t（*testing.T）：当前测试。
// 返回：无。
func TestCapabilityTableDescribesTheAdaptedEntrypoints(t *testing.T) {
	want := []string{"chat", "completion", "embedding", "image", "video", "audio_speech", "audio_transcription", "rerank", "moderation"}
	got := map[string]bool{}
	for _, c := range providercapability.Capabilities() {
		got[c.ID] = true
	}
	for _, id := range want {
		if !got[id] {
			t.Fatalf("capability %s is missing from the table", id)
		}
		delete(got, id)
	}
	if len(got) != 0 {
		t.Fatalf("unexpected capabilities %v", got)
	}
}

// TestRegisterTransportRejectsAnEntryWithNoActions 证明一个没有动作的转发方式不会被登记。
//
// 它无处可转：Match 靠动作的路径命中，没有动作就永远匹配不上。
// 参数 t（*testing.T）：当前测试。
// 返回：无。
func TestRegisterTransportRejectsAnEntryWithNoActions(t *testing.T) {
	before := len(providerregistry.Transports())
	providerregistry.RegisterTransport(providercontract.Transport{ID: "no-actions", Kind: providercontract.KindBypass})
	providerregistry.RegisterTransport(providercontract.Transport{Kind: providercontract.KindBypass, Actions: []providercontract.Action{{Name: "x", Method: "POST", PublicPath: "/x"}}})
	if after := len(providerregistry.Transports()); after != before {
		t.Fatalf("a transport with no id or no actions was registered: %d -> %d", before, after)
	}
}

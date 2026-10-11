package capability

import (
	"github.com/sunqirui1987/xhub/internal/provider/contract"
	"testing"
)

// TestEndpointDirectoryBoundaries 验证操作映射、未知端点及返回值隔离；前置静态能力目录。
// 参数 t 为单测上下文，返回无；修改的只是副本，无清理或外部服务。
func TestEndpointDirectoryBoundaries(t *testing.T) {
	for op, want := range map[string]string{"images": "image_generation", "images_edits": "image_edit", "audio_translation": "audio_translation", "responses": "responses", "messages": "messages", "gemini": "gemini", "vertex": "vertex", "chat": "chat", "embeddings": "embedding", "unknown": ""} {
		if got := EndpointForOp(op); got != want {
			t.Fatalf("操作 %s 映射到 %s，期望 %s", op, got, want)
		}
	}
	if KnownEndpoint("unknown") {
		t.Fatal("未知端点被接受")
	}
	copy := EndpointTypes()
	copy[0].Paths[0] = "/forged"
	if EndpointTypes()[0].Paths[0] == "/forged" {
		t.Fatal("端点副本改变了目录")
	}
	caps := Capabilities()
	caps[0].Ops[0] = "forged"
	caps[0].Paths[0] = "/forged"
	if id, ok := CapabilityForOp("forged"); ok {
		t.Fatalf("能力副本污染注册: %s", id)
	}
}

// TestEndpointCompatibility 验证跨对话转换和原生操作严格绑定；参数 t 为单测上下文。
// 前置纯协议数据，返回无，包含正常与错误协议；无外部状态，无需清理。
func TestEndpointCompatibility(t *testing.T) {
	for _, tc := range []struct {
		endpoint  contract.EndpointDescriptor
		transport contract.Transport
		want      bool
	}{
		{contract.EndpointDescriptor{Kind: contract.KindAdapted, Protocol: "openai-chat"}, contract.Transport{Protocol: "anthropic-messages"}, true},
		{contract.EndpointDescriptor{Kind: contract.KindBypass, ID: "image_generation", Protocol: "openai-images"}, contract.Transport{EndpointID: "image_edit", Protocol: "openai-images"}, false},
		{contract.EndpointDescriptor{Kind: contract.KindBypass, ID: "image_generation", Protocol: "openai-images"}, contract.Transport{EndpointID: "image_generation", Protocol: "openai-images"}, true},
		{contract.EndpointDescriptor{Kind: contract.KindAdapted, ID: "embedding", Protocol: "openai-embeddings"}, contract.Transport{EndpointID: "embedding", Protocol: "openai-images"}, false},
		{contract.EndpointDescriptor{Kind: contract.KindAdapted, ID: "embedding", Protocol: "openai-embeddings"}, contract.Transport{EndpointID: "embedding", Protocol: "openai-embeddings"}, true},
	} {
		if got := CompatibleEndpoint(tc.endpoint, tc.transport); got != tc.want {
			t.Fatalf("端点=%+v 传输=%+v 兼容=%v", tc.endpoint, tc.transport, got)
		}
	}
}

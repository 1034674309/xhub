package registry_test

import (
	"testing"

	"github.com/sunqirui1987/xhub/internal/config"
	providerdeployment "github.com/sunqirui1987/xhub/internal/provider/deployment"
	providerregistry "github.com/sunqirui1987/xhub/internal/provider/registry"
)

// TestGoogleNativeActions 验证带操作后缀的模型路径及独立动作解析。
// 参数 t 为测试上下文；前置注册原生协议；验证正常、空别名、错误操作与模型替换，无数据清理。
func TestGoogleNativeActions(t *testing.T) {
	for _, protocol := range []string{"gemini", "vertex"} {
		root := "/v1beta/models/"
		if protocol == "vertex" {
			root = "/vertex/v1/models/"
		}
		for _, op := range []string{"generateContent", "streamGenerateContent", "countTokens"} {
			hit, ok := providerregistry.Match("POST", root+"alias:"+op)
			if !ok || hit.Names["model"] != "alias" {
				t.Fatalf("未命中原生操作: %s %+v", op, hit)
			}
			m := config.ModelEntry{ModelName: "alias", LiteLLMParams: map[string]any{"model": "real"}, ModelInfo: map[string]any{"transport": protocol + "_generate_content", "endpoint_types": []string{protocol}}}
			resolved, err := providerdeployment.ResolveHit(hit, m)
			if err != nil || resolved.Names["model"] != "real" || resolved.Action.UpstreamPath[len(resolved.Action.UpstreamPath)-len(op):] != op {
				t.Fatalf("操作或模型错配: %+v %v", resolved, err)
			}
		}
		for _, path := range []string{root + ":generateContent", root + "alias:unknown", root + "../:generateContent"} {
			if _, ok := providerregistry.Match("POST", path); ok {
				t.Fatalf("非法路径被接受: %s", path)
			}
		}
	}
}

// TestGooglePathAliases 验证 Google 模型占位符解析完整路径和版本后缀，拒绝空段、点段和未知操作。
// 参数 t 为单测上下文；前置编译时注册，返回无；只读目录，无网络和持久数据清理。
func TestGooglePathAliases(t *testing.T) {
	for _, root := range []string{"/v1beta/models/", "/vertex/v1/models/", "/bypass/gemini/v1beta/models/", "/bypass/vertex/v1/models/"} {
		for _, op := range []string{"generateContent", "streamGenerateContent", "countTokens"} {
			for _, name := range []string{"group/model:latest", "group/sub/model", "model:generateContent", "名称/版本：最新"} {
				hit, ok := providerregistry.Match("POST", root+name+":"+op)
				if !ok || hit.Names["model"] != name {
					t.Errorf("完整模型名解析失败 %s%s:%s：%+v", root, name, op, hit)
				}
			}
		}
		for _, name := range []string{"", ".", "..", "group//model", "group/../model", "/model"} {
			if _, ok := providerregistry.Match("POST", root+name+":generateContent"); ok {
				t.Errorf("非法模型路径被接受 %s%s", root, name)
			}
		}
		if _, ok := providerregistry.Match("POST", root+"group/model:unknown"); ok {
			t.Errorf("未知 Google 操作被接受 %s", root)
		}
	}
	// 多段解析仅用于末尾模型，不得放宽已登记的异步任务 ID。
	if _, ok := providerregistry.Match("GET", "/queue/bytedance/seedance-2.0/requests/group/task"); ok {
		t.Fatal("Fal 任务 ID 被错误扩展为多段路径")
	}
}

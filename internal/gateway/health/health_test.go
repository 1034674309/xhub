package health

import (
	"errors"
	"net/http/httptest"
	"testing"
)

// TestProbeBoundaries 前置成功或失败的数据库探测，验证存活不依赖数据库、就绪失败返回 503。
// 参数 t 为测试上下文；内存处理器不建立连接，测试环境变量由 t.Setenv 恢复。
func TestProbeBoundaries(t *testing.T) {
	for _, failed := range []bool{false, true} {
		h := handler{ping: func() error {
			if failed {
				return errors.New("offline")
			}
			return nil
		}}
		live := httptest.NewRecorder()
		h.healthLive(live, httptest.NewRequest("GET", "/health/liveness", nil))
		if live.Code != 200 {
			t.Fatalf("存活探测错误: %d", live.Code)
		}
		ready := httptest.NewRecorder()
		h.healthReady(ready, httptest.NewRequest("GET", "/health/readiness", nil))
		want := 200
		if failed {
			want = 503
		}
		if ready.Code != want {
			t.Fatalf("就绪探测错误: got=%d want=%d", ready.Code, want)
		}
	}
	t.Setenv("XHUB_PUBLIC_ORIGIN", " https://gateway.example/// ")
	if PublicOrigin() != "https://gateway.example" {
		t.Fatalf("公开地址未规范化: %s", PublicOrigin())
	}
	t.Setenv("XHUB_PUBLIC_ORIGIN", "")
	if PublicOrigin() != LocalGatewayOrigin {
		t.Fatal("缺省地址错误")
	}
}

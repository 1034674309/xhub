package affinity

import (
	"testing"
	"time"

	providercontract "github.com/sunqirui1987/xhub/internal/provider/contract"
)

// TestOfficialTaskPinLastsSevenDays 验证任务部署保存与七天 TTL 常量，前置为独立内存 State。
// 参数 t 为测试上下文，无返回值；断言任务可找回原运行部署，失败信息包含 TTL 或部署值。
// 不访问 Redis 或外部上游，测试结束后对象由 Go 释放，无需删除用户数据。
func TestOfficialTaskPinLastsSevenDays(t *testing.T) {
	if officialPinTTL != 7*24*time.Hour {
		t.Fatalf("official pin ttl %s", officialPinTTL)
	}
	s := &State{}
	s.PinOfficial("cgt-1", "base|model")
	if got := s.OfficialDeployment("cgt-1"); got != "base|model" {
		t.Fatalf("pin %q", got)
	}
}

// TestOfficialContextSurvivesHostReplacement 验证替换 State 后仍能从同一缓存恢复创建时计费事实。
// 参数 t 为测试上下文，无返回值；前置独立内存缓存，验证快照一致、七天有效期及其他作用域未命中。
// 仅在测试内部复用缓存，无并发访问和外部数据，结束后对象由 Go 释放。
func TestOfficialContextSurvivesHostReplacement(t *testing.T) {
	s := &State{}
	facts := providercontract.TaskContext{StartedAt: time.Now().UTC(), Model: "bytedance/model", Resolution: "1080p", HasVideo: true, Known: true}
	s.PinOfficialContext("scope", facts)
	restored := &State{affinity: s.affinity}
	if got := restored.OfficialContext("scope"); got != facts {
		t.Fatalf("context lost: %+v", got)
	}
	pin := s.affinity["official_context:v1:scope"]
	if time.Until(pin.until) < officialPinTTL-time.Minute {
		t.Fatal("wrong context TTL")
	}
	if restored.OfficialContext("other").Known {
		t.Fatal("cross-task context leak")
	}
}

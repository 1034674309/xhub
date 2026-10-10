package regression

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sunqirui1987/xhub/internal/config"
	"github.com/sunqirui1987/xhub/internal/router"
)

// TestMalformedSuccessTripsCooldown 验证 HTTP 200 的损坏协议会回退并触发部署冷却，下一请求直接跳过故障部署。
// 前置独立 Redis、隔离 schema 和本地假上游；部署身份每次唯一，测试前后只删除本用例的失败、冷却、延迟和路由用量键。
func TestMalformedSuccessTripsCooldown(t *testing.T) {
	redisURL := os.Getenv("XHUB_REGRESSION_REDIS_URL")
	if redisURL == "" {
		t.Skip("malformed response cooldown needs XHUB_REGRESSION_REDIS_URL")
	}
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	public := "regression-malformed-cooldown-" + suffix
	malformed := deployment(public, "malformed-a-"+suffix, map[string]any{"input_cost_per_token": 0.0000001})
	healthy := deployment(public, "healthy-b-"+suffix, map[string]any{"input_cost_per_token": 0.01})
	h := newHarness(t, malformed, healthy)
	cleanupProtocolRuntimeKeys(t, redisURL, h.gw.Cfg.ModelList)
	admin := h.adminSession()
	c := h.openScope(t, admin, "malformed-cooldown")
	h.modelRouteTemplate(t, admin, c, "malformed cooldown route", public, "cost-based-routing", nil, 1, 1, 60)
	h.scriptBody("malformed-a-"+suffix, []byte("{}"))
	first := h.ok(http.MethodPost, "/v1/chat/completions", c.key, chatRequest(public, "fallback after malformed 200"))
	if !strings.Contains(first.text(), defaultReply.Content) || strings.Join(h.upstreamSince(0), ",") != "malformed-a-"+suffix+",healthy-b-"+suffix {
		t.Fatalf("损坏 200 未回退到健康部署: %s calls=%v", first.describe(), h.upstreamSince(0))
	}

	h.scriptBody("malformed-a-"+suffix, nil)
	h.resetUpstream()
	h.ok(http.MethodPost, "/v1/chat/completions", c.key, chatRequest(public, "skip malformed deployment during cooldown"))
	if got := h.upstreamSince(0); strings.Join(got, ",") != "healthy-b-"+suffix {
		t.Fatalf("损坏协议部署未进入冷却: %v", got)
	}
}

// cleanupProtocolRuntimeKeys 精确清理本用例部署产生的 Redis 路由状态。
// 参数 t 为测试上下文，redisURL 为专用实例地址，deployments 为本用例部署；开始先清残留并注册结束清理，不执行 FLUSHDB。
func cleanupProtocolRuntimeKeys(t *testing.T, redisURL string, deployments []config.ModelEntry) {
	t.Helper()
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("解析 regression Redis 地址失败: %v", err)
	}
	client := redis.NewClient(opt)
	keys := make([]string, 0, len(deployments)*4)
	for _, dep := range deployments {
		id := router.CooldownID(dep)
		keys = append(keys, "xhub:fails:"+id, "xhub:cooldown:"+id, "xhub:latency:"+id, "xhub:routetpm:"+id)
	}
	remove := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.Del(ctx, keys...).Err(); err != nil {
			t.Errorf("清理 protocol regression Redis 键失败: %v", err)
			return
		}
		remaining, err := client.Exists(ctx, keys...).Result()
		if err != nil {
			t.Errorf("确认 protocol regression Redis 清理失败: %v", err)
		} else if remaining != 0 {
			t.Errorf("protocol regression Redis 键仍有 %d 个残留", remaining)
		}
	}
	remove()
	t.Cleanup(func() {
		remove()
		_ = client.Close()
	})
}

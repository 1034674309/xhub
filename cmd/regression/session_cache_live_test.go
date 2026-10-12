package regression

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sunqirui1987/xhub/internal/config"
)

// sessionCacheAttempt 保存一次真实供应商调用的脱敏观测，不保存地址、认证、提示词或回答。
// 用于证明每轮确实访问了上游，并把供应商缓存与网关响应缓存分开统计。
type sessionCacheAttempt struct {
	// Vendor 是由已配置主机映射出的供应商标识，不能由模型回答推测。
	Vendor string `json:"vendor"`
	// Status 是上游 HTTP 状态；0 表示连接失败，不能作为缓存未命中样本。
	Status int `json:"status"`
	// Milliseconds 包含收到完整响应的耗时，仅用于辅助观察，不能作为缓存命中证据。
	Milliseconds int64 `json:"milliseconds"`
	// Usage 是供应商原始响应的数值用量，保留 cached_tokens 缺失与明确为零的区别。
	Usage map[string]any `json:"usage,omitempty"`
}

// sessionCacheTransport 在真实 HTTP 链外记录缓存证据；响应正文读后恢复，不改变网关计量输入。
// 仅用于串行 live 测试，锁同时保证内部 HTTP 回调与测试读取之间没有竞争。
type sessionCacheTransport struct {
	// next 是实际联网的 transport，不能替换为模拟上游。
	next http.RoundTripper
	// vendors 把允许访问的供应商主机映射为报告标识，报告不保存主机地址。
	vendors map[string]string
	// mu 保护 attempts，防止 HTTP 回调与证据读取同时操作切片。
	mu sync.Mutex
	// attempts 按完成顺序保存本次隔离测试的全部真实上游调用。
	attempts []sessionCacheAttempt
}

// RoundTrip 执行 req 并记录 HTTP 状态、完整响应耗时与原始 usage；返回值和错误交还网关。
// 请求由真实网关构建；读取失败继续返回原错误，不把错误文本或可能含凭据的正文写入报告。
func (o *sessionCacheTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	started := time.Now()
	resp, err := o.next.RoundTrip(req)
	a := sessionCacheAttempt{Vendor: o.vendors[req.URL.Host]}
	if resp != nil {
		a.Status = resp.StatusCode
		raw, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(raw))
		if readErr == nil {
			var body map[string]any
			if json.Unmarshal(raw, &body) == nil {
				a.Usage, _ = body["usage"].(map[string]any)
			}
		} else if err == nil {
			err = readErr
		}
	}
	a.Milliseconds = time.Since(started).Milliseconds()
	o.mu.Lock()
	o.attempts = append(o.attempts, a)
	o.mu.Unlock()
	return resp, err
}

// TestLiveSessionSupplierPromptCache 测量同模型在两家真实供应商间保持、切换与切回时的缓存用量。
// 前置为显式 live 开关及恰好两家供应商、各一个对应型号；每轮生成少量输出，会产生真实费用。
// 使用独立 PostgreSQL schema、临时账号、模板和动态偏好；测试结束删除 schema，不修改现有产品配置。
// 缓存未命中或字段缺失都是合法实验结果；测试断言完整调用链、供应商粘性及用量日志的真实性。
func TestLiveSessionSupplierPromptCache(t *testing.T) {
	if os.Getenv("XHUB_CACHE_LIVE") != "1" {
		t.Skip("set XHUB_CACHE_LIVE=1 to measure paid supplier prompt caching")
	}
	vendors := liveCredentials(t)
	if len(vendors) != 2 || len(vendors[0].Models) != 1 || len(vendors[1].Models) != 1 {
		t.Fatal("缓存实验需要两家供应商，各配置一个相同逻辑型号")
	}
	const public = "session-cache-live"
	var entries []config.ModelEntry
	hosts := map[string]string{}
	for _, v := range vendors {
		e := liveModelDeployment(v, v.Models[0])
		e.ModelName = public
		e.LiteLLMParams["deployment_id"] = "cache-" + v.ID
		e.ModelInfo["id"] = "cache-" + v.ID
		e.ModelInfo["endpoint_types"] = []string{"chat", "bypass:openai-chat"}
		u, err := url.Parse(v.Base)
		if err != nil || u.Host == "" || hosts[u.Host] != "" {
			t.Fatal("供应商必须配置不同且有效的主机地址")
		}
		hosts[u.Host] = v.ID
		entries = append(entries, e)
	}
	h := newHarness(t, entries...)
	h.live = true
	next := h.gw.Client.Transport
	if next == nil {
		next = http.DefaultTransport
	}
	observer := &sessionCacheTransport{next: next, vendors: hosts}
	h.gw.Client.Transport = observer
	admin := h.adminSession()
	owner := h.openScope(t, admin, "live-cache")
	body := routeTemplateBody([]any{}, 1, 90, 0, 0)
	template := routeTemplate(t, h, admin, "live cache measurement", body)
	bindTemplate(t, h, admin, "key", owner.keyID, template)
	// 每次实验在前缀开头加入唯一标识，避免与历史测试共享已热缓存；后续各轮保持整段前缀不变。
	session := fmt.Sprintf("cache-live-%d", time.Now().UnixNano())
	var prefix strings.Builder
	prefix.WriteString("Synthetic cache measurement " + session + ". Read this reference and reply only OK to the final request.\n")
	for i := 0; i < 160; i++ {
		fmt.Fprintf(&prefix, "Reference item %03d: A gateway validates identity, selects an eligible deployment, preserves the conversation preference, and records supplier usage after completion.\n", i)
	}
	calls := []map[string]any{}
	defer func() {
		if dir := os.Getenv("E2E_EVIDENCE_DIR"); dir != "" {
			_ = os.MkdirAll(dir, 0700)
			evidence := map[string]any{"experiment": "session_supplier_prompt_cache", "session": session, "prefix_bytes": prefix.Len(), "calls": calls, "passed": !t.Failed(), "gateway_response_cache_enabled": false}
			raw, _ := json.MarshalIndent(evidence, "", "  ")
			if err := os.WriteFile(filepath.Join(dir, "session-supplier-cache.json"), raw, 0600); err != nil {
				t.Errorf("写入缓存实验报告失败: %v", err)
			}
		}
	}()
	// 分四阶段：A 预热、开放双候选验证仍保持 A、强制 B 并预热、切回 A 检查缓存是否保留。
	for stage, weights := range [][2]int{{1, 0}, {1, 1}, {0, 1}, {1, 0}} {
		body["model_routes"] = []any{map[string]any{"model": public, "strategy": "traffic-split", "allocations": []any{
			map[string]any{"deployment_id": "cache-" + vendors[0].ID, "weight": weights[0]},
			map[string]any{"deployment_id": "cache-" + vendors[1].ID, "weight": weights[1]},
		}}}
		h.ok("POST", "/route_template/"+template+"/update", admin, map[string]any{"body": body})
		want := vendors[0].ID
		if stage == 2 {
			want = vendors[1].ID
		}
		for round := 0; round < 3; round++ {
			index := len(calls) + 1
			req := map[string]any{"model": public, "messages": []any{
				map[string]any{"role": "system", "content": prefix.String()},
				map[string]any{"role": "user", "content": fmt.Sprintf("Round %d. Reply only OK.", index)},
			}, "max_completion_tokens": 32, "stream": false}
			// 原生透传入口保留供应商支持的 max_completion_tokens，避免统一协议默认 token 上限影响实验。
			r := h.doHeaders("POST", "/bypass/openai/v1/chat/completions", owner.key, req, map[string]string{"X-Session-Id": session})
			observer.mu.Lock()
			attempts := append([]sessionCacheAttempt(nil), observer.attempts...)
			observer.mu.Unlock()
			if len(attempts) != index {
				t.Fatalf("第 %d 轮真实上游尝试累计=%d，预期每轮一次", index, len(attempts))
			}
			a := attempts[index-1]
			call := map[string]any{"request": index, "stage": stage + 1, "attempt": a, "gateway_status": r.status}
			calls = append(calls, call)
			if r.status != 200 || a.Status != 200 || a.Vendor != want {
				t.Fatalf("第 %d 轮网关状态=%d，上游状态=%d，实际供应商=%s，预期=%s", index, r.status, a.Status, a.Vendor, want)
			}
			u := liveUsageCounts(a.Usage)
			if u.prompt < 2048 || r.header("X-Session-Id") != session {
				t.Fatal("长前缀用量不足或会话身份丢失")
			}
			h.flushSpend()
			row := logDetail(t, h, admin, r.header("x-litellm-call-id"))
			metadata, _ := row["metadata"].(map[string]any)
			call["log_prompt_tokens"] = row["prompt_tokens"]
			call["log_cached_tokens"] = metadata["cached_tokens"]
			call["gateway_response_cache_hit"] = row["cache_hit"]
			call["catalog_estimated_usd"] = parseFloatOrZero(r.header("x-litellm-response-cost"))
			if row["cache_hit"] != "false" || numberOrZero(row["prompt_tokens"]) != float64(u.prompt) || numberOrZero(metadata["cached_tokens"]) != float64(u.cacheRead) {
				t.Fatal("网关响应缓存介入或日志未准确记录供应商用量")
			}
			t.Logf("round=%d stage=%d vendor=%s prompt=%d cached=%d elapsed_ms=%d", index, stage+1, a.Vendor, u.prompt, u.cacheRead, a.Milliseconds)
		}
	}
}

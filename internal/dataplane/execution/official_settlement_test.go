// 本文件验证 execution 的相关执行契约，阅读顺序为测试宿主/辅助方法，再到各个 Test 场景。
// 具体前置、期望、边界和清理方式见每个测试方法；夹具的空实现仅用于隔离当前测试。
package execution

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sunqirui1987/xhub/internal/auth"
	dpcontract "github.com/sunqirui1987/xhub/internal/dataplane/contract"
	providerregistry "github.com/sunqirui1987/xhub/internal/provider/registry"
	"github.com/sunqirui1987/xhub/internal/router"
)

// concurrentPollRecord 保存并发轮询的调用 ID、输入标记、响应、备注与用量，验证按调用隔离暂存。
type concurrentPollRecord struct {
	// 本轮调用 ID、入站 X-Poll 哨兵和原始响应，用于发现并发串记录。
	callID, poll, response string
	// 本轮的调用备注，包括共享最终结算 ID。
	note dpcontract.CallNote
	// 本轮提交的协议用量映射。
	usage map[string]any
}

// concurrentSettlementHost 用互斥锁保护每轮交互和备注，再用屏障让多轮轮询同时到达结算边界。
// 用于验证暂存不会串请求且最终结算身份稳定；消费记录后删除临时映射，无真实持久化。
type concurrentSettlementHost struct {
	// 复用原生身份和任务目录，暂存操作在本宿主加锁。
	*logicHost
	// 保护交互、备注及输出记录的互斥锁。
	mu sync.Mutex
	// 按本轮 callID 隔离的交互暂存。
	exchanges map[string]concurrentPollRecord
	// 按本轮 callID 隔离的备注暂存。
	annotations map[string]dpcontract.CallNote
	// 已消费的完整轮询记录。
	records []concurrentPollRecord
	// 让全部轮询先暂存，再同时进入记账的屏障。
	ready sync.WaitGroup
}

// RememberExchange 是 concurrentSettlementHost 的测试接口实现。
// 在锁内按 id 保存 X-Poll 标记和 response 文本，再通过 ready 屏障等待全部轮询；无返回，制造同时存在的暂存记录。
// 调用方为本文件或同包的执行测试；仅模拟所述边界，不能据此推断生产宿主的行为。
func (h *concurrentSettlementHost) RememberExchange(id string, r *http.Request, _, response []byte) {
	h.mu.Lock()
	h.exchanges[id] = concurrentPollRecord{callID: id, poll: r.Header.Get("X-Poll"), response: string(response)}
	h.mu.Unlock()
	// 先让所有调用同时保留元数据，再开始轮询，确保测试能暴露相互覆盖的问题。
	h.ready.Done()
	h.ready.Wait()
}

// AnnotateCall 是 concurrentSettlementHost 的测试接口实现。
// 在锁内把 note 保存到 annotations[id]；无返回，按独立 callID 隔离并发备注，不写数据库。
// 调用方为本文件或同包的执行测试；仅模拟所述边界，不能据此推断生产宿主的行为。
func (h *concurrentSettlementHost) AnnotateCall(id string, note dpcontract.CallNote) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.annotations[id] = note
}

// RecordSpend 是 concurrentSettlementHost 的测试接口实现。
// 在锁内按 id 取交互和备注，关联 usage 后追加 records 并删除两份暂存；无返回，模拟生产消费边界而不真实扣费。
// 调用方为本文件或同包的执行测试；仅模拟所述边界，不能据此推断生产宿主的行为。
func (h *concurrentSettlementHost) RecordSpend(_ http.ResponseWriter, _ *auth.Principal, id, _, _ string, usage map[string]any, _ time.Time, _ bool, _ int, _ string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	row := h.exchanges[id]
	row.note, row.usage = h.annotations[id], usage
	h.records = append(h.records, row)
	delete(h.exchanges, id)
	delete(h.annotations, id)
}

// TestOfficialConcurrentCompletedPollsIsolateMetadata 验证 16 个并发完成轮询各自保留调用 ID、正文与备注，同时共享稳定结算 ID 且消费后清空暂存。
// 参数 t 为 Go 测试上下文；返回值无，断言失败定位具体协议、路由或日志行为。
// 前置使用独立测试配置及本地 HTTP 假上游，不依赖外部供应商凭据。
// 服务器通过 defer/Cleanup 关闭，测试状态只存在本例内存，不产生持久化数据。
func TestOfficialConcurrentCompletedPollsIsolateMetadata(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "succeeded", "poll": r.Header.Get("X-Poll"), "usage": map[string]any{"completion_tokens": 12}})
	}))
	defer up.Close()
	dep := Deployment("video", "volcengine/model", "key", up.URL, "ark_contents_generation", nil)
	h := &concurrentSettlementHost{logicHost: officialHost(up, dep), exchanges: map[string]concurrentPollRecord{}, annotations: map[string]dpcontract.CallNote{}}
	h.pins[officialTaskScope(&auth.Principal{UserID: "test-user"}, "ark_contents_generation", "task")] = router.CooldownID(dep)
	const polls = 16
	h.ready.Add(polls)
	var wg sync.WaitGroup
	for i := 0; i < polls; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := httptest.NewRequest(http.MethodGet, "/api/v3/contents/generations/tasks/task", nil)
			r.Header.Set("X-Poll", fmt.Sprint(i))
			hit, ok := providerregistry.Match(r.Method, r.URL.Path)
			if !ok {
				t.Error("missing official route")
				return
			}
			w := httptest.NewRecorder()
			ServeBypass(h, w, r, hit)
			if w.Code != http.StatusOK {
				t.Errorf("poll %d: %d %s", i, w.Code, w.Body.String())
			}
		}(i)
	}
	wg.Wait()
	if len(h.records) != polls {
		t.Fatalf("recorded %d polls", len(h.records))
	}
	seen := map[string]bool{}
	settlement := h.records[0].note.SettlementID
	for _, row := range h.records {
		var response struct {
			Poll string `json:"poll"`
		}
		if err := json.Unmarshal([]byte(row.response), &response); err != nil {
			t.Fatal(err)
		}
		if row.callID == "" || seen[row.callID] || response.Poll != row.poll || settlement == "" || row.note.SettlementID != settlement || !row.note.SkipRouteUsage || !positiveOfficialUsage(row.usage) {
			t.Fatalf("metadata collision or lost settlement: %+v response=%+v", row, response)
		}
		seen[row.callID] = true
	}
	if len(h.exchanges) != 0 || len(h.annotations) != 0 {
		t.Fatal("metadata was not consumed")
	}
}

// retrySettlementHost 模拟首次结算未持久化、后续结算成功的宿主，捕获每次稳定结算 ID；验证轮询可再次提交相同结算。
type retrySettlementHost struct {
	// 复用原生宿主，覆写 RecordSpend 模拟首轮丢失。
	*logicHost
	// 每次结算尝试提交的稳定标识，验证重试不变。
	attempts []string
}

// RecordSpend 是 retrySettlementHost 的测试接口实现。
// 把最近备注的 SettlementID 记入 attempts，首轮故意不持久化，其余委托 logicHost.RecordSpend；参数沿用生产接口，无返回。
// 调用方为本文件或同包的执行测试；仅模拟所述边界，不能据此推断生产宿主的行为。
func (h *retrySettlementHost) RecordSpend(w http.ResponseWriter, p *auth.Principal, id, alias, op string, usage map[string]any, start time.Time, cached bool, status int, dep string) {
	h.attempts = append(h.attempts, h.notes[len(h.notes)-1].SettlementID)
	if len(h.attempts) == 1 {
		return
	}
	h.logicHost.RecordSpend(w, p, id, alias, op, usage, start, cached, status, dep)
}

// TestOfficialSettlementRetriesAfterPersistenceFailure 验证首次模拟结算未持久化后可再次提交相同结算 ID，并按调用方与部署隔离该 ID。
// 参数 t 为 Go 测试上下文；返回值无，断言失败定位具体协议、路由或日志行为。
// 前置使用独立测试配置及本地 HTTP 假上游，不依赖外部供应商凭据。
// 服务器通过 defer/Cleanup 关闭，测试状态只存在本例内存，不产生持久化数据。
func TestOfficialSettlementRetriesAfterPersistenceFailure(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"status":"succeeded","usage":{"completion_tokens":12}}`)
	}))
	defer up.Close()
	dep := Deployment("video", "volcengine/model", "key", up.URL, "ark_contents_generation", nil)
	h := &retrySettlementHost{logicHost: officialHost(up, dep)}
	scope := officialTaskScope(&auth.Principal{UserID: "test-user"}, "ark_contents_generation", "task")
	h.pins[scope] = router.CooldownID(dep)
	for i := 0; i < 2; i++ {
		r := httptest.NewRequest(http.MethodGet, "/api/v3/contents/generations/tasks/task", nil)
		hit, ok := providerregistry.Match(r.Method, r.URL.Path)
		if !ok {
			t.Fatal("missing official route")
		}
		w := httptest.NewRecorder()
		ServeBypass(h, w, r, hit)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if len(h.attempts) != 2 || h.attempts[0] != h.attempts[1] || !strings.HasPrefix(h.attempts[0], "official-settlement:") || len(h.spend) != 1 || h.spend[0].usage == nil {
		t.Fatalf("lost settlement retry: attempts=%v persisted=%+v", h.attempts, h.spend)
	}
	if h.attempts[0] == officialSettlementID(scope, "other-deployment") || h.attempts[0] == officialSettlementID(officialTaskScope(&auth.Principal{UserID: "other"}, "ark_contents_generation", "task"), router.CooldownID(dep)) {
		t.Fatal("settlement identity lacks caller/deployment isolation")
	}
}

// TestOfficialZeroAndPendingPollsKeepUniqueIDs 验证明确零用量、进行中和失败轮询各自记录独立调用 ID，不生成最终结算或路由用量。
// 参数 t 为 Go 测试上下文；返回值无，断言失败定位具体协议、路由或日志行为。
// 前置使用独立测试配置及本地 HTTP 假上游，不依赖外部供应商凭据。
// 服务器通过 defer/Cleanup 关闭，测试状态只存在本例内存，不产生持久化数据。
func TestOfficialZeroAndPendingPollsKeepUniqueIDs(t *testing.T) {
	response := `{"status":"succeeded","usage":{"completion_tokens":0}}`
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, response) }))
	defer up.Close()
	dep := Deployment("video", "volcengine/model", "key", up.URL, "ark_contents_generation", nil)
	h := officialHost(up, dep)
	h.pins[officialTaskScope(&auth.Principal{UserID: "test-user"}, "ark_contents_generation", "task")] = router.CooldownID(dep)
	for _, body := range []string{response, response, `{"status":"running","usage":{"completion_tokens":12}}`, `{"status":"failed","usage":{"completion_tokens":12}}`} {
		response = body
		h.call(t, http.MethodGet, "/api/v3/contents/generations/tasks/task", "")
	}
	seen := map[string]bool{}
	for _, note := range h.notes {
		if note.SettlementID != "" || !note.SkipRouteUsage {
			t.Fatalf("invalid poll note: %+v", note)
		}
	}
	for _, row := range h.spend {
		if row.usage != nil || strings.HasPrefix(row.callID, "official-settlement:") || seen[row.callID] {
			t.Fatalf("poll collapsed or charged: %+v", row)
		}
		seen[row.callID] = true
	}
}

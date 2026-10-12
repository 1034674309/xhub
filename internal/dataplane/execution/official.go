package execution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sunqirui1987/xhub/internal/auth"
	"github.com/sunqirui1987/xhub/internal/catalog"
	"github.com/sunqirui1987/xhub/internal/config"
	dpcontract "github.com/sunqirui1987/xhub/internal/dataplane/contract"
	dpdiagnostics "github.com/sunqirui1987/xhub/internal/dataplane/diagnostics"
	dprouting "github.com/sunqirui1987/xhub/internal/dataplane/routing"
	dptransport "github.com/sunqirui1987/xhub/internal/dataplane/transport"
	dpusage "github.com/sunqirui1987/xhub/internal/dataplane/usage"
	providercontract "github.com/sunqirui1987/xhub/internal/provider/contract"
	providerdeployment "github.com/sunqirui1987/xhub/internal/provider/deployment"
	"github.com/sunqirui1987/xhub/internal/router"
	"github.com/sunqirui1987/xhub/internal/util"
)

// ServeBypass 转发一次已经匹配到的官方调用。正文按登记的模型规则转发；Fal 队列链接改为网关相对路径。
// usage 在后续查询第一次出现时扣一次。
// 参数 h：Bypass 接口。w：调用方响应。r：入站请求。hit：路径匹配到的端点类型和动作。
// 返回：无。错误和上游正文都写在 w 上。
// 调用：gateway/bypass.go serveBypass。测试：bypass_logic_test.go TestEndpointAndModelLogic。
func ServeBypass(h dpcontract.Bypass, w http.ResponseWriter, r *http.Request, hit providercontract.Hit) {
	start := time.Now()
	callID := util.CallID()
	util.SetCallID(w, callID)
	p := h.RequireLLMPrincipal(w, r)
	if p == nil {
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, (64<<20)+1))
	if err != nil || len(raw) > 64<<20 {
		util.WriteTypedError(w, r.URL.Path, 400, "invalid_request", "could not read request body within 64 MiB")
		return
	}
	requestBody, err := dptransport.ParseBypassBody(raw, r.Header.Get("Content-Type"))
	if err != nil {
		util.WriteTypedError(w, r.URL.Path, 400, "invalid_request", err.Error())
		return
	}
	body := requestBody.Fields
	if hit.Action.Name == "create" || hit.Transport.ModelField != "" && hit.Action.Name == "" {
		serveBypassCreate(h, w, r, hit, p, body, raw, callID, start)
		return
	}
	id := hit.Names["id"]
	if id == "" {
		id = hit.Names["task_id"]
	}
	if id == "" {
		id = hit.Names["request_id"]
	}
	if hit.Action.TaskQuery != "" {
		id = strings.TrimSpace(r.URL.Query().Get(hit.Action.TaskQuery))
	}
	serveBypassFollow(h, w, r, hit, id, callID, start, p)
}

// serveBypassCreate 处理创建。按模型字段找勾了该类型的部署，换成官方模型 id 和上游密钥。
// 创建响应里的任务 id 钉住这条部署；同步对话按实际 usage 计费，异步任务沿用查询结算。
// 仅显式配置回退的同步对话可在失败后重放；继续请求、已输出流和异步创建保持原部署。
//
// 参数 h（Bypass）：官方转发需要的能力，不含聊天缓存和护栏；w（http.ResponseWriter）：创建结果或错误写在这里；r（*http.Request）：入站请求，用来读路径和头；hit（provider.Hit）：这次匹配到的端点类型和动作。
// 参数 principal（*auth.Principal）：已通过鉴权的调用方。body（map[string]any）：解析后的 JSON。raw（[]byte）：原始正文。
// 参数 callID（string）：用量行 id。start（time.Time）：请求开始时间。
// 返回：无。
// 调用：ServeBypass 在动作名是 create 时。测试：逻辑测试中的方舟创建、重试及任务归属。
func serveBypassCreate(h dpcontract.Bypass, w http.ResponseWriter, r *http.Request, hit providercontract.Hit, principal *auth.Principal, body map[string]any, raw []byte, callID string, start time.Time) {
	requestBody, _ := dptransport.ParseBypassBody(raw, r.Header.Get("Content-Type"))
	field := hit.Transport.ModelField
	if field == "" {
		field = "model"
	}
	alias, _ := body[field].(string)
	alias = strings.TrimSpace(alias)
	if hit.Transport.Protocol == "gemini" || hit.Transport.Protocol == "vertex" {
		// Gemini 原厂正文没有 model；公开路径是路由别名，部署模型决定上游路径。
		alias = hit.Names["model"]
	}
	if alias == "" && hit.Action.Model != "" {
		// A Fal body normally has no model. Resolve the registered path to its
		// configured public alias, including names created by the model editor.
		names := map[string]bool{}
		list, _ := dprouting.DropDisabled(eligible(h, hit))
		for _, dep := range list {
			names[dep.ModelName] = true
		}
		if len(names) > 1 {
			util.WriteTypedError(w, r.URL.Path, 400, "invalid_request", "multiple public aliases for this endpoint; model required")
			return
		}
		alias = hit.Transport.StripPrefix + "/" + hit.Action.Model
		for name := range names {
			alias = name
		}
	}
	if alias == "" {
		util.WriteTypedError(w, r.URL.Path, 400, "invalid_request", field+" required")
		return
	}
	plan := h.PlanRoute(r, alias, body, principal)
	w.Header().Set("X-Session-Id", plan.SessionID)
	if plan.SessionID != "" {
		h.AnnotateCall(callID, dpcontract.CallNote{SessionID: plan.SessionID, SessionSource: plan.SessionSource})
	}
	if plan.Err != nil {
		util.WriteTypedError(w, r.URL.Path, 400, "invalid_request", plan.Err.Error())
		return
	}
	if !h.EnforceIdentityLimits(w, r.URL.Path, principal, alias, dpusage.EstimateTokens(body)) {
		return
	}
	settings := h.RouteSettingsFor(principal)
	if settings.Err != nil {
		util.WriteTypedError(w, r.URL.Path, 503, "unavailable", "route template unavailable")
		return
	}
	baseSettings := settings
	settings = settings.ForModel(alias)
	if settings.Err != nil {
		util.WriteTypedError(w, r.URL.Path, 400, "invalid_request", settings.Err.Error())
		return
	}
	if err := router.ValidateStrategy(settings.Strategy()); err != nil {
		util.WriteTypedError(w, r.URL.Path, 400, "invalid_request", err.Error())
		return
	}
	list := eligible(h, hit)
	list, _ = dprouting.DropDisabled(list)
	state := h.RouteState()
	state.Allocations = settings.Policy.Shares()
	runtimePin := dprouting.RuntimePin(list, plan.Pinned)
	pool := settings.Schedule(list, alias, state, runtimePin)
	disableFallbacks, _ := body["disable_fallbacks"].(bool)
	// 原生异步任务的创建结果可能不明确，保持禁止重放；对话创建可以按模型回退。
	background, _ := body["background"].(bool)
	dialogueFallback := !background && (hit.Transport.Protocol == "openai-responses" || hit.Transport.Protocol == "openai-chat" || hit.Transport.Protocol == "anthropic-messages" || hit.Transport.Protocol == "gemini" || hit.Transport.Protocol == "vertex")
	fallbacks := dprouting.NewFallbackQueue(baseSettings, alias, plan.Required || disableFallbacks || !dialogueFallback)
	if len(pool) == 0 {
		pool, settings = fallbacks.Next("general", list, state)
	}
	pool, settings = fallbacks.SelectAffinity(&plan, pool, alias, list, state, settings)
	if plan.Required && len(pool) == 0 {
		util.WriteTypedError(w, r.URL.Path, 409, "unavailable", "response deployment unavailable")
		return
	}
	if settings.Err != nil {
		util.WriteTypedError(w, r.URL.Path, 503, "unavailable", "fallback route unavailable")
		return
	}
	if len(pool) == 0 {
		util.WriteTypedError(w, r.URL.Path, 400, "invalid_request", "model not found: "+alias)
		return
	}
	var dep config.ModelEntry
	var respBody []byte
	var response dptransport.ForwardResponse
	var status int
	var selectedID string
	tried := false
	originalHit := hit
	for len(pool) > 0 {
		kind := "general"
		for _, rawDep := range pool {
			// 组内尝试仍保留成员身份，错误分类及后续回退使用最后尝试成员。
			fallbacks.UseDeployment(rawDep.ModelName)
			plan.Target = fallbacks.Target(rawDep)
			// 回退目标可能是另一个组；组别名与真实成员都必须通过权限和预算检查。
			if fallbacks.PoolAlias != alias && fallbacks.PoolAlias != rawDep.ModelName && !h.EnforceIdentityLimits(w, r.URL.Path, principal, fallbacks.PoolAlias, dpusage.EstimateTokens(body)) {
				return
			}
			// 同步对话先用完整 max_attempts 重试同一供应商，不能因未配置跨模型回退而放弃重试。
			// disable_fallbacks 只控制跨模型队列；异步创建和固定响应归属不扩大重放范围。
			replayDialogue := dialogueFallback && !plan.Required
			if rawDep.ModelName != alias && !h.EnforceIdentityLimits(w, r.URL.Path, principal, rawDep.ModelName, dpusage.EstimateTokens(body)) {
				return
			}
			candidate, err := h.AttachCredential(rawDep)
			if err != nil {
				continue
			}
			candidateHit, err := providerdeployment.ResolveHit(originalHit, candidate)
			if err != nil {
				continue
			}
			hit = candidateHit
			base, key := dptransport.BypassAuth(candidate)
			if base == "" || key == "" {
				continue
			}
			upstreamModel := providercontract.OfficialID(candidateHit.Transport.StripPrefix, candidate.ParamString("model", ""))
			payload, err := requestBody.Payload(candidateHit.Transport.ModelField, upstreamModel, candidateHit.Action.Model != "" || candidateHit.Transport.Protocol == "gemini" || candidateHit.Transport.Protocol == "vertex")
			if err != nil {
				util.WriteTypedError(w, r.URL.Path, 400, "invalid_request", err.Error())
				return
			}
			url := dptransport.WithQuery(dptransport.BypassDeploymentURL(base, candidateHit), r)
			for attempt := 0; attempt < settings.Retries(); attempt++ {
				if tried {
					fresh, err := h.ResolveRequest(r)
					if err != nil || fresh == nil || !fresh.CanInfer() {
						util.WriteTypedError(w, r.URL.Path, 401, "invalid_api_key", "invalid api key")
						return
					}
					principal = fresh
					if !h.EnforceIdentityLimits(w, r.URL.Path, principal, alias, dpusage.EstimateTokens(body)) {
						return
					}
					if rawDep.ModelName != alias && !h.EnforceIdentityLimits(w, r.URL.Path, principal, rawDep.ModelName, dpusage.EstimateTokens(body)) {
						return
					}
				}
				tried = true
				ctx, cancel := context.WithTimeout(r.Context(), time.Duration(settings.TimeoutSeconds()*float64(time.Second)))
				response, err = dptransport.ForwardOfficial(h.HTTPClient(), r.WithContext(ctx), candidateHit.Action.Method, url, key, payload, r.Header, candidateHit.Transport, w)
				respBody, status = response.Body, response.StatusCode
				cancel()
				dep = candidate
				selectedID = router.CooldownID(rawDep)
				if err != nil {
					if response.Streamed {
						h.RememberExchange(callID, r, raw, []byte(dpdiagnostics.SafeErr(err)))
						note := dataplaneNote(hit, dep, plan.SessionID, time.Since(start))
						applyStreamTiming(&note, response, start)
						note.Upstream = response.Diagnostics
						h.AnnotateCall(callID, note)
						h.RecordSpend(w, principal, callID, alias, hit.Transport.ID+":"+hit.Action.Name, nil, start, false, 502, selectedID)
						return
					}
					util.Error("bypass forward path=%s err=%s", r.URL.Path, dpdiagnostics.SafeErr(err))
					h.NoteFailure(router.CooldownID(rawDep), settings)
					if replayDialogue {
						continue
					}
					// 创建响应丢失时，上游可能已受理付费任务；结果不明确的创建请求不能重放。
					h.RememberExchange(callID, r, raw, []byte(dpdiagnostics.SafeErr(err)))
					note := dataplaneNote(hit, dep, plan.SessionID, time.Since(start))
					applyStreamTiming(&note, response, start)
					note.Upstream = response.Diagnostics
					h.AnnotateCall(callID, note)
					h.RecordSpend(w, principal, callID, alias, hit.Transport.ID+":"+hit.Action.Name, nil, start, false, http.StatusBadGateway, selectedID)
					util.WriteTypedError(w, r.URL.Path, 502, "upstream_error", "upstream request failed")
					return
				}
				if status >= 500 || status == http.StatusTooManyRequests {
					h.NoteFailure(router.CooldownID(rawDep), settings)
					// 服务端可能在受理付费任务后才返回错误，不能因此创建第二个任务。
					if status >= 500 && !replayDialogue {
						goto forwarded
					}
					if r.Context().Err() != nil {
						return
					}
					continue
				}
				errorKind := dprouting.FallbackKind(status, respBody)
				if errorKind != "" && !fallbacks.Disabled && len(baseSettings.ModelFallbacks[fallbacks.Current].Targets(errorKind)) > 0 {
					kind = errorKind
					goto nextFallbackModel
				}
				hit = candidateHit
				goto forwarded
			}
		}
	nextFallbackModel:
		pool, settings = fallbacks.Next(kind, list, state)
		if settings.Err != nil {
			util.WriteTypedError(w, r.URL.Path, 503, "unavailable", "fallback route unavailable")
			return
		}
	}
	if !tried {
		util.WriteTypedError(w, r.URL.Path, 401, "upstream_auth", "This model has no upstream API key configured.")
		return
	}
forwarded:
	if status == 0 {
		h.RememberExchange(callID, r, raw, []byte("upstream request failed"))
		h.RecordSpend(w, principal, callID, alias, hit.Transport.ID+":"+hit.Action.Name, nil, start, false, http.StatusBadGateway, selectedID)
		util.WriteTypedError(w, r.URL.Path, 502, "upstream_error", "upstream request failed")
		return
	}
	var doc map[string]any
	_ = json.Unmarshal(respBody, &doc)
	depID := selectedID
	taskRequestID := ""
	if taskID := providercontract.ReadTaskID(doc, hit.Transport.TaskID); taskID != "" && status >= 200 && status < 300 && doc["error"] == nil && doc["detail"] == nil {
		scope := officialTaskScope(principal, hit.Transport.ID, taskID)
		if billing := hit.Transport.Billing; billing != nil {
			// 创建请求身份随调用方隔离的任务钉保存，后续查询更新同一日志。
			taskRequestID = callID
			contextBody := body
			if hit.Action.Model != "" {
				contextBody = make(map[string]any, len(body)+1)
				for k, v := range body {
					contextBody[k] = v
				}
				contextBody["model"] = hit.Action.Model
			}
			facts := billing.Context(contextBody)
			facts.StartedAt = start
			facts.RequestID = callID
			h.PinOfficialContext(scope, facts)
		}
		h.PinOfficial(scope, selectedID)
	}
	respBody = rewriteQueueURLs(hit, doc, respBody)
	if status >= 200 && status < 300 {
		responseID, _ := doc["id"].(string)
		if response.Streamed {
			responseID = response.ResponseID
		}
		h.CommitRoute(plan, router.DeploymentID(dep), responseID)
	}
	h.RememberExchange(callID, r, raw, respBody)
	note := dataplaneNote(hit, dep, plan.SessionID, time.Since(start))
	if taskRequestID != "" {
		note.TaskRequestID, note.TaskInitial = taskRequestID, true
		note.TaskStatus = officialTaskStatus(status, doc, true)
	}
	applyStreamTiming(&note, response, start)
	note.Upstream = response.Diagnostics
	h.AnnotateCall(callID, note)
	var usage map[string]any
	if status >= 200 && status < 300 && hit.Transport.ResponseUsage != nil {
		usage = hit.Transport.ResponseUsage(doc, body)
		if response.Streamed {
			usage = response.Usage
		}
	}
	h.RecordSpend(w, principal, callID, alias, hit.Transport.ID+":"+hit.Action.Name, usage, start, false, status, depID)
	response.Body = respBody
	dptransport.WriteThrough(w, response)
}

// serveBypassFollow 处理查询和列表。id 非空时走任务钉，并要求部署仍属于这个端点类型。
// id 为空时是列表：只有一个上游密钥才转发，多个则写 400 和模型名。
// 参数 id：路径或查询串里的任务 id，列表时为空。其余参数与创建相同。
// 返回：无。异步查询更新原日志状态与正文，成功实测用量只结算一次。
// 调用：ServeBypass。测试：逻辑测试的重复扣费、跨供应商 404、列表和 Suno 查询参数。
func serveBypassFollow(h dpcontract.Bypass, w http.ResponseWriter, r *http.Request, hit providercontract.Hit, id, callID string, start time.Time, principal *auth.Principal) {
	var dep config.ModelEntry
	var ok bool
	scopedID := officialTaskScope(principal, hit.Transport.ID, id)
	var depID string
	if id != "" {
		pinned := h.OfficialDeployment(scopedID)
		if pinned == "" {
			util.WriteTypedError(w, r.URL.Path, 404, "not_found", "unknown task")
			return
		}
		dep, ok = h.FindDeployment(pinned)
		depID = pinned
		if !ok || !sameEndpoint(dep, hit) {
			util.WriteTypedError(w, r.URL.Path, 404, "not_found", "unknown task")
			return
		}
	} else {
		var names []string
		dep, names, ok = oneUpstream(h, hit)
		if !ok {
			if len(names) == 0 {
				util.WriteTypedError(w, r.URL.Path, 400, "invalid_request", "model not found")
				return
			}
			util.WriteTypedError(w, r.URL.Path, 400, "invalid_request", "model required: "+strings.Join(names, ", "))
			return
		}
	}
	if depID == "" {
		depID = router.CooldownID(dep)
	}
	if !h.EnforceIdentityLimits(w, r.URL.Path, principal, dep.ModelName, 0) {
		return
	}
	settings := h.RouteSettingsFor(principal)
	if settings.Err != nil {
		util.WriteTypedError(w, r.URL.Path, 503, "unavailable", "route template unavailable")
		return
	}
	settings = settings.ForModel(dep.ModelName)
	if settings.Err != nil {
		util.WriteTypedError(w, r.URL.Path, 400, "invalid_request", settings.Err.Error())
		return
	}
	dep, err := h.AttachCredential(dep)
	if err != nil {
		util.WriteTypedError(w, r.URL.Path, 401, "upstream_auth", "This model has no upstream API key configured.")
		return
	}
	hit, err = providerdeployment.ResolveHit(hit, dep)
	if err != nil {
		util.WriteTypedError(w, r.URL.Path, 409, "unavailable", err.Error())
		return
	}
	base, key := dptransport.BypassAuth(dep)
	if base == "" || key == "" {
		util.WriteTypedError(w, r.URL.Path, 401, "upstream_auth", "This model has no upstream API key configured.")
		return
	}
	url := dptransport.WithQuery(dptransport.BypassDeploymentURL(base, hit), r)
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(settings.TimeoutSeconds()*float64(time.Second)))
	defer cancel()
	response, err := dptransport.ForwardOfficial(h.HTTPClient(), r.WithContext(ctx), r.Method, url, key, nil, r.Header, hit.Transport, nil)
	respBody, status := response.Body, response.StatusCode
	if err != nil {
		h.RememberExchange(callID, r, nil, []byte(dpdiagnostics.SafeErr(err)))
		note := dataplaneNote(hit, dep, "", time.Since(start))
		facts := h.OfficialContext(scopedID)
		errorStart := start
		if id != "" && hit.Transport.Billing != nil && (hit.Action.Name == "get" || hit.Action.Name == "status") {
			note.TaskRequestID, note.TaskStatus = facts.RequestID, "failed"
			note.SkipRouteUsage = true
			if !facts.StartedAt.IsZero() {
				errorStart = facts.StartedAt
			}
		}
		applyStreamTiming(&note, response, start)
		note.Upstream = response.Diagnostics
		h.AnnotateCall(callID, note)
		h.RecordSpend(w, principal, callID, dep.ModelName, hit.Transport.ID+":"+hit.Action.Name, nil, errorStart, false, http.StatusBadGateway, depID)
		util.WriteTypedError(w, r.URL.Path, 502, "upstream_error", "upstream request failed")
		return
	}
	var doc map[string]any
	_ = json.Unmarshal(respBody, &doc)
	usage := usageFromOfficial(doc)
	billingStart := start
	if billing := hit.Transport.Billing; billing != nil {
		facts := h.OfficialContext(scopedID)
		usage = billing.Usage(doc, facts)
		if usage != nil && facts.Model != "" {
			usage["pricing_model"] = hit.Transport.StripPrefix + "/" + facts.Model
		}
		if positiveOfficialUsage(usage) && !facts.StartedAt.IsZero() {
			billingStart = facts.StartedAt
		}
	}
	settlementID := ""
	if id != "" && status >= 200 && status < 300 && positiveOfficialUsage(usage) && !officialPending(doc) && officialTaskStatus(status, doc, false) != "failed" {
		// 每次完成查询都允许重试持久化；新任务由原日志事务标记去重，旧任务保留稳定结算 ID 兼容。
		settlementID = officialSettlementID(scopedID, depID)
	} else {
		usage = nil
	}
	respBody = rewriteQueueURLs(hit, doc, respBody)
	plan := h.PlanRoute(r, dep.ModelName, nil, principal)
	h.RememberExchange(callID, r, nil, respBody)
	note := dataplaneNote(hit, dep, plan.SessionID, time.Since(start))
	if id != "" && hit.Transport.Billing != nil && (hit.Action.Name == "get" || hit.Action.Name == "status") {
		facts := h.OfficialContext(scopedID)
		note.TaskRequestID = facts.RequestID
		note.TaskStatus = officialTaskStatus(status, doc, false)
		note.TaskSettlement = settlementID != ""
		// FAL 结果端点直接返回产物而不带 status；计量适配器确认完成后才允许结算。
		if note.TaskSettlement {
			note.TaskStatus = "completed"
		}
		if !facts.StartedAt.IsZero() {
			billingStart = facts.StartedAt
		}
	}
	note.SettlementID = settlementID
	note.SkipRouteUsage = true
	applyStreamTiming(&note, response, start)
	note.Upstream = response.Diagnostics
	h.AnnotateCall(callID, note)
	h.RecordSpend(w, principal, callID, dep.ModelName, hit.Transport.ID+":"+hit.Action.Name, usage, billingStart, false, status, depID)
	response.Body = respBody
	dptransport.WriteThrough(w, response)
}

// officialTaskStatus 将供应商响应转换为原请求生命周期状态。
// 参数 status、doc、initial：HTTP 状态、响应对象、是否创建；返回执行中、轮询中、完成或失败的协议值。
// 调用创建与查询路径；未知或缺失状态保持活动态，HTTP 错误及供应商失败显示失败，无副作用。
func officialTaskStatus(status int, doc map[string]any, initial bool) string {
	if status >= 400 || doc["error"] != nil || doc["detail"] != nil {
		return "failed"
	}
	state, _ := doc["status"].(string)
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "succeeded", "completed", "success":
		return "completed"
	case "failed", "expired", "cancelled", "canceled", "error":
		return "failed"
	}
	if initial {
		return "executing"
	}
	return "polling"
}

// dataplaneNote 组装 Bypass 写入用量行的备注。供应商优先用部署上的 custom_llm_provider，
// 否则用端点类型的前缀，再否则用类型 id。
// 参数 hit：端点类型。dep：选中的部署。sessionID：PlanRoute 给出的会话，可空。
// 参数 elapsed：从请求开始到上游响应结束。
// 返回：CallNote。TTFTMs 在耗时大于 0 时至少为 1 毫秒。
// 调用：创建和查询在 RecordSpend 之前。测试：逻辑测试断言 Provider 为 volcengine。
func dataplaneNote(hit providercontract.Hit, dep config.ModelEntry, sessionID string, elapsed time.Duration) dpcontract.CallNote {
	name := dep.ParamString("custom_llm_provider", "")
	if name == "" {
		name = hit.Transport.StripPrefix
	}
	if name == "" {
		name = hit.Transport.ID
	}
	return dpcontract.CallNote{
		TTFTMs:    dpusage.TtftMillis(elapsed),
		Provider:  name,
		SessionID: sessionID,
	}
}

// sameEndpoint 确认钉住的部署和当前路径属于同一类端点。防止跨协议查询任务。
// 参数 dep：钉上找回的部署。hit：当前路径匹配到的类型。
// 返回：true 才继续用这行部署转发。
// 调用：serveBypassFollow。测试：native_bypass_test.go 的任务隔离与重复查询。
func sameEndpoint(dep config.ModelEntry, hit providercontract.Hit) bool {
	return providerdeployment.Includes(dep, hit.Transport.ID)
}

// eligible 列出模型表里属于当前传输方式的部署。
// 参数 h、hit：读模型表和当前匹配。
// 返回：未再过滤暂停状态的部署。暂停由创建和 oneUpstream 去掉。
// 调用：serveBypassCreate、oneUpstream。测试：bypass_logic_test.go。
func eligible(h dpcontract.Bypass, hit providercontract.Hit) []config.ModelEntry {
	var out []config.ModelEntry
	for _, m := range h.Models() {
		if providerdeployment.Includes(m, hit.Transport.ID) && (hit.Action.Model == "" ||
			providercontract.OfficialID(hit.Transport.StripPrefix, m.ParamString("model", m.ModelName)) == hit.Action.Model) {
			out = append(out, m)
		}
	}
	return out
}

// oneUpstream 为没有任务 id 的列表选部署。按 api_base 加 api_key 分组。 只有一组时返回那条部署。多组不能合成一家的官方响应。
// 参数 h（Bypass）：h；hit（provider.Hit）：hit。
// 调用：仅在 official.go 内使用
// 测试：无直接单测
func oneUpstream(h dpcontract.Bypass, hit providercontract.Hit) (config.ModelEntry, []string, bool) {
	list, _ := dprouting.DropDisabled(eligible(h, hit))
	seen := map[string]config.ModelEntry{}
	var names []string
	for _, m := range list {
		names = append(names, m.ModelName)
		attached, err := h.AttachCredential(m)
		if err != nil {
			continue
		}
		base, key := dptransport.BypassAuth(attached)
		id := base + "|" + key
		if _, ok := seen[id]; !ok {
			seen[id] = attached
		}
	}
	if len(seen) == 1 {
		for _, dep := range seen {
			return dep, nil, true
		}
	}
	return config.ModelEntry{}, names, false
}

// officialTaskScope 隔离不同调用方和不同供应商端点的任务编号。
// 参数 p、transport、id：调用方、端点类型和上游任务编号。
// 返回：固定长度、不含凭证的任务范围键。
// 调用：创建和查询。测试：official_template_test.go。
func officialTaskScope(p *auth.Principal, transport, id string) string {
	caller := ""
	if p != nil {
		if p.Hash != "" {
			caller = "key:" + p.Hash
		} else {
			caller = "user:" + p.UserID
		}
	}
	raw, _ := json.Marshal([]string{caller, transport, id})
	sum := sha256.Sum256(raw)
	return "v2:" + hex.EncodeToString(sum[:])
}

// officialSettlementID 由固定部署与调用方任务范围生成唯一结算身份。
// 等待或零用量轮询保留各自的请求身份，不占用成功结果的结算身份。
// 参数 scopedID：隔离调用主体和任务的身份。depID：钉住的部署身份。
// 返回：稳定的持久层结算 request_id。
// 调用：serveBypassFollow。测试：official_settlement_test.go。
func officialSettlementID(scopedID, depID string) string {
	raw, _ := json.Marshal([]string{scopedID, depID})
	sum := sha256.Sum256(raw)
	return "official-settlement:" + hex.EncodeToString(sum[:])
}

// positiveOfficialUsage 判断规范化后是否有正用量。
// 参数 usage：上游用量对象。返回：任一计费维度为正时为真。
// 调用：serveBypassFollow。测试：official_settlement_test.go。
func positiveOfficialUsage(usage map[string]any) bool {
	u := catalog.NormalizeUsage(usage)
	return u.PromptTokens > 0 || u.CompletionTokens > 0 || u.CachedTokens > 0 ||
		u.CacheWriteTokens > 0 || u.Images > 0 || u.Seconds > 0 || u.Searches > 0
}

// officialPending 判断显式未完成或失败的任务，避免把进度用量当作最终账单。
// 参数 doc：上游任务响应。返回：任务尚未成功完成时为真。
// 调用：serveBypassFollow。测试：official_template_test.go。
func officialPending(doc map[string]any) bool {
	status, _ := doc["status"].(string)
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "in_queue", "in_progress", "queued", "pending", "running", "processing", "submitted", "failed", "error", "cancelled", "canceled":
		return true
	}
	return false
}

// usageFromOfficial 从官方响应里取出 usage 对象。没有用量字段时返回 nil，创建请求因此不计费。
// 参数 doc：已解析的上游响应。
// 返回：usage 映射，或 nil。
// 调用：serveBypassFollow。测试：逻辑测试第一次查询带 completion_tokens，创建响应没有 usage。
func usageFromOfficial(doc map[string]any) map[string]any {
	usage, _ := doc["usage"].(map[string]any)
	if len(usage) == 0 {
		return nil
	}
	return usage
}

// rewriteQueueURLs 将任务轮询地址改写为登记的网关相对路径。
// 参数 hit：固定传输与动作；doc：上游对象；raw：原始正文。
// 返回：改写后的 JSON；无任务、无变更或编码失败时返回原字节。
// 调用：创建与后续查询。测试：fal_test.go。
// 不信任上游 URL 的主机；未实现取消时清空 cancel_url。
func rewriteQueueURLs(hit providercontract.Hit, doc map[string]any, raw []byte) []byte {
	if !hit.Transport.QueueURLs || doc == nil {
		return raw
	}
	id := providercontract.ReadTaskID(doc, hit.Transport.TaskID)
	if id == "" {
		id = hit.Names["request_id"]
	}
	if id == "" {
		return raw
	}
	names := map[string]string{"request_id": url.PathEscape(id)}
	changed := false
	for _, a := range hit.Transport.Actions {
		field := ""
		switch a.Name {
		case "get":
			field = "response_url"
		case "status":
			field = "status_url"
		}
		if field != "" {
			if _, exists := doc[field]; exists {
				doc[field] = providercontract.Expand(a.PublicPath, names)
				changed = true
			}
		}
	}
	if _, exists := doc["cancel_url"]; exists {
		doc["cancel_url"] = ""
		changed = true
	}
	if !changed {
		return raw
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return raw
	}
	return out
}

// applyStreamTiming 将实测 SSE 首数据时间写入调用备注；参数 note 为待更新快照、response 为转发结果、start 为开始时间。
// 原生调用计费前调用，无返回值；非流式保持原备注，未收到 SSE 数据时不产生性能样本。
func applyStreamTiming(note *dpcontract.CallNote, response dptransport.ForwardResponse, start time.Time) {
	note.Streamed = response.Streamed
	if response.Streamed {
		note.TTFTMs = nil
		if !response.FirstByteAt.IsZero() {
			note.TTFTMs = dpusage.TtftMillis(response.FirstByteAt.Sub(start))
		}
	}
}

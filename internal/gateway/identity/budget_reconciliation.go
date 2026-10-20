package identity

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/sunqirui1987/xhub/internal/iam"
	"github.com/sunqirui1987/xhub/internal/util"
)

// BudgetReservations 为平台管理员列出安全的待核对占用摘要；参数为管理宿主和HTTP上下文，无写入副作用。
// key_id可空用于全局核对，分页严格限1..100；普通用户和虚拟Key不能读取其他人的账务状态。
func BudgetReservations(g Gate, w http.ResponseWriter, r *http.Request) {
	util.SetCallID(w, util.CallID())
	if g.RequireManage(w, r) == nil {
		return
	}
	q := r.URL.Query()
	page, size := 1, 50
	var err error
	if q.Has("page") {
		page, err = strconv.Atoi(q.Get("page"))
		if err != nil {
			writeBudgetReconciliationError(g, w, r, iam.ErrInvalid)
			return
		}
	}
	if q.Has("page_size") {
		size, err = strconv.Atoi(q.Get("page_size"))
		if err != nil {
			writeBudgetReconciliationError(g, w, r, iam.ErrInvalid)
			return
		}
	}
	if page < 1 || page > 1000000 || size < 1 || size > 100 {
		writeBudgetReconciliationError(g, w, r, iam.ErrInvalid)
		return
	}
	rows, total, err := g.Identity().ListBudgetReservations(r.Context(), q.Get("key_id"), q.Get("state"), size, (page-1)*size)
	if err != nil {
		writeBudgetReconciliationError(g, w, r, err)
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{"reservations": rows, "total": total, "page": page, "page_size": size})
}

// BudgetReconcile 接收一个明确且带依据的人工对账结论，供Key详情管理流程调用。
// 仅管理session可调用；严格JSON校验、防止遗漏金额或额外对象，结果及审计由IAM原子持久化，失败不释放。
func BudgetReconcile(g Gate, w http.ResponseWriter, r *http.Request) {
	util.SetCallID(w, util.CallID())
	p := g.RequireManage(w, r)
	if p == nil {
		return
	}
	var body struct {
		Outcome             string   `json:"outcome"`
		VerifiedCost        *float64 `json:"verified_cost_usd"`
		Evidence            string   `json:"evidence"`
		ExpectedFingerprint string   `json:"expected_fingerprint"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
	decoder.DisallowUnknownFields()
	var trailing any
	if err := decoder.Decode(&body); err != nil || body.VerifiedCost == nil {
		writeBudgetReconciliationError(g, w, r, iam.ErrInvalid)
		return
	}
	if err := decoder.Decode(&trailing); err != io.EOF {
		writeBudgetReconciliationError(g, w, r, iam.ErrInvalid)
		return
	}
	resolution, already, err := g.Identity().ReconcileBudgetReservation(r.Context(), actorOf(p), r.PathValue("reservation_id"), iam.BudgetReconciliationInput{Outcome: body.Outcome, VerifiedCost: *body.VerifiedCost, Evidence: body.Evidence, ExpectedFingerprint: body.ExpectedFingerprint})
	if err != nil {
		writeBudgetReconciliationError(g, w, r, err)
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{"resolution": resolution, "already_resolved": already})
}

// writeBudgetReconciliationError 将领域失败映射到稳定机器错误码，前端按i18n翻译；不把存储诊断或证据泄露给客户端。
// 参数g/w/r用于既有存储错误处理，err不能为空；仅本文件处理函数调用，无账务副作用。
func writeBudgetReconciliationError(g Gate, w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, iam.ErrInvalid):
		util.WriteError(w, 400, "budget_reconciliation_invalid", "invalid budget reconciliation input")
	case errors.Is(err, iam.ErrConflict):
		util.WriteError(w, 409, "budget_reconciliation_conflict", "budget reconciliation evidence changed or is incomplete")
	default:
		g.WriteIAMError(w, r, err)
	}
}

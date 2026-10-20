package iam

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"xorm.io/xorm"
)

// BudgetReconciliationInput 是管理员核实后的单次尝试结论；调用方必须传入当前预占指纹与非空依据。
// 实际费用可以超过预占；它是新增账单事实，不修改额度上限或伪造Token用量。
type BudgetReconciliationInput struct {
	Outcome             string  `json:"outcome"`
	VerifiedCost        float64 `json:"verified_cost_usd"`
	Evidence            string  `json:"evidence"`
	ExpectedFingerprint string  `json:"expected_fingerprint"`
}

// BudgetResolution 是可公开的不可变人工对账结果；原始证据仅保存在数据库内部载荷。
type BudgetResolution struct {
	ReservationID string    `json:"reservation_id"`
	CallID        string    `json:"call_id"`
	Outcome       string    `json:"outcome"`
	VerifiedCost  float64   `json:"verified_cost_usd"`
	CostDelta     float64   `json:"cost_delta_usd"`
	PreviousCost  float64   `json:"previous_cost_usd"`
	UpdatedCost   float64   `json:"updated_cost_usd"`
	Evidence      string    `json:"evidence"`
	ActorID       string    `json:"actor_id"`
	ResolvedAt    time.Time `json:"resolved_at"`
}

// savedBudgetResolution 冻结人工结论、原预占与修改前账单，供审计及迟到事件校验，不经管理API直接返回。
type savedBudgetResolution struct {
	Resolution  BudgetResolution  `json:"resolution"`
	Fingerprint string            `json:"fingerprint"`
	Hold        BudgetReservation `json:"original_reservation"`
	Event       UsageEvent        `json:"original_usage"`
}

// BudgetReservationView 是管理页面所需的安全摘要；缺失账单用nil表示，不能被展示为已确认零费用。
type BudgetReservationView struct {
	ReservationID string    `json:"reservation_id"`
	CallID        string    `json:"call_id"`
	KeyID         string    `json:"key_id"`
	UserID        string    `json:"user_id"`
	TeamID        string    `json:"team_id"`
	OrgID         string    `json:"organization_id"`
	ProjectID     string    `json:"project_id"`
	Amount        float64   `json:"amount"`
	State         string    `json:"state"`
	Reason        string    `json:"reason"`
	StartedAt     time.Time `json:"started_at"`
	Fingerprint   string    `json:"fingerprint"`
	Model         string    `json:"model"`
	Supplier      string    `json:"supplier"`
	UpstreamModel string    `json:"upstream_model"`
	LedgerCost    *float64  `json:"ledger_cost"`
	CanReconcile  bool      `json:"can_reconcile"`
	BlockedReason string    `json:"blocked_reason"`
}

// budgetReservationFingerprint 为完整预占生成稳定指纹，防止管理员在查看之后处理已变化的记录。
// 参数为数据库快照，返回64位十六进制摘要；时间统一UTC微秒，不修改原对象，不包含授权能力。
func budgetReservationFingerprint(h BudgetReservation) string {
	h.StartedAt = h.StartedAt.UTC().Truncate(time.Microsecond)
	h.CreatedAt = h.CreatedAt.UTC().Truncate(time.Microsecond)
	raw, _ := json.Marshal(h)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// validateBudgetReconciliation 校验并规范管理员输入，供HTTP和IAM调用共用；非法金额/结论/依据返回ErrInvalid。
// 只trim依据，不截断证据或接受NaN/Inf；不使用TTL、HTTP状态或目录价格推断退款。
func validateBudgetReconciliation(in BudgetReconciliationInput) (BudgetReconciliationInput, error) {
	in.Evidence = strings.TrimSpace(in.Evidence)
	fingerprint, err := hex.DecodeString(in.ExpectedFingerprint)
	if err != nil || len(fingerprint) != 32 || in.ExpectedFingerprint != strings.ToLower(in.ExpectedFingerprint) || in.Evidence == "" || utf8.RuneCountInString(in.Evidence) > 2000 || math.IsNaN(in.VerifiedCost) || math.IsInf(in.VerifiedCost, 0) || in.VerifiedCost < 0 {
		return in, ErrInvalid
	}
	if (in.Outcome != "no_charge" && in.Outcome != "charged") || (in.Outcome == "no_charge" && in.VerifiedCost != 0) || (in.Outcome == "charged" && in.VerifiedCost <= 0) {
		return in, ErrInvalid
	}
	return in, nil
}

// readBudgetResolution 在当前事务读取一个不可变结论；id是尝试ID，返回载荷/是否存在/存储错误。
// 对账与自动重放共用；损坏JSON失败关闭，不能把损坏数据当作尚未核销。
func readBudgetResolution(s *xorm.Session, id string) (savedBudgetResolution, bool, error) {
	var row struct {
		Payload string `xorm:"payload"`
	}
	found, err := s.Table("budget_reconciliations").Where("reservation_id = ?", id).Get(&row)
	var saved savedBudgetResolution
	if err == nil && found {
		err = json.Unmarshal([]byte(row.Payload), &saved)
	}
	if err == nil && found && (saved.Resolution.ReservationID != id || saved.Hold.RequestID != id) {
		err = ErrConflict
	}
	return saved, found, err
}

// reconciliationEvidence 读取原账单并说明不能核销的具体原因；参数会话和hold均由数据库提供。
// List用于显示，Resolve在串行事务中再次调用；尚无完整账单、仍有在途尝试或持久待结算时不释放额度。
func reconciliationEvidence(s *xorm.Session, h BudgetReservation) (UsageEvent, string, error) {
	var e UsageEvent
	found, err := s.Where("request_id = ?", h.CallID).Get(&e)
	if err != nil {
		return e, "", err
	}
	if h.State != "uncertain" {
		return e, "state_not_uncertain", nil
	}
	if !found {
		return e, "event_missing", nil
	}
	if e.Status == "executing" || e.Status == "polling" {
		return e, "event_not_terminal", nil
	}
	if e.KeyID != h.KeyID || e.UserID != h.UserID || e.TeamID != h.TeamID || e.OrganizationID != h.OrgID || e.ProjectID != h.ProjectID {
		return e, "identity_mismatch", nil
	}
	active, err := s.Where("call_id = ? AND state <> ?", h.CallID, "uncertain").Exist(new(BudgetReservation))
	if err != nil {
		return e, "", err
	}
	if active {
		return e, "active_attempt", nil
	}
	pending, err := s.Table("usage_pending").Where("request_id = ?", h.CallID).Exist()
	if err != nil {
		return e, "", err
	}
	if pending {
		return e, "pending_usage", nil
	}
	return e, "", nil
}

// ListBudgetReservations 按key/state分页读取待结算占用与安全账单摘要；调用方必须已验证平台管理员。
// 参数limit范围1..100、offset非负，state可空；返回行、总数、存储错误，不修改预占或消费。
func (db *DB) ListBudgetReservations(ctx context.Context, keyID, state string, limit, offset int) ([]BudgetReservationView, int64, error) {
	if limit < 1 || limit > 100 || offset < 0 || (state != "" && state != "uncertain" && state != "settling" && state != "submitted" && state != "reserved") {
		return nil, 0, ErrInvalid
	}
	s := db.session(ctx)
	defer s.Close()
	var holds []BudgetReservation
	query := s.Where("(? = '' OR key_id = ?) AND (? = '' OR state = ?)", keyID, keyID, state, state)
	total, err := query.Asc("created_at", "request_id").Limit(limit, offset).FindAndCount(&holds)
	if err != nil {
		return nil, 0, err
	}
	rows := make([]BudgetReservationView, 0, len(holds))
	for _, h := range holds {
		e, reason, err := reconciliationEvidence(s, h)
		if err != nil {
			return nil, 0, err
		}
		source, upstreamModel := budgetReservationSource(h)
		row := BudgetReservationView{ReservationID: h.RequestID, CallID: h.CallID, KeyID: h.KeyID, UserID: h.UserID, TeamID: h.TeamID, OrgID: h.OrgID, ProjectID: h.ProjectID, Amount: h.Amount, State: h.State, Reason: h.Reason, StartedAt: h.StartedAt, Fingerprint: budgetReservationFingerprint(h), Model: e.Model, Supplier: source, UpstreamModel: upstreamModel, CanReconcile: reason == "", BlockedReason: reason}
		if e.RequestID != "" {
			cost := e.Cost
			row.LedgerCost = &cost
		}
		rows = append(rows, row)
	}
	return rows, total, nil
}

// ReconcileBudgetReservation 在单一额度事务中追加人工结论、修正费用、审计并释放一个uncertain尝试。
// by必须是仍启用的平台管理员session；相同输入重放返回原结论，变化/正在结算/缺失证据返回冲突。
// 实际费用按该attempt新增，绝不替换同call其他尝试费用，不重算token或增加请求计数；失败全部回滚。
func (db *DB) ReconcileBudgetReservation(ctx context.Context, by Actor, id string, input BudgetReconciliationInput) (BudgetResolution, bool, error) {
	in, err := validateBudgetReconciliation(input)
	if err != nil || id == "" || by.ID == "" || by.Kind != "session" {
		return BudgetResolution{}, false, ErrInvalid
	}
	var result BudgetResolution
	already := false
	err = db.tx(ctx, func(s *xorm.Session) error {
		var admin User
		found, err := s.ID(by.ID).Get(&admin)
		if err != nil {
			return err
		}
		if !found || !admin.Admin() {
			return ErrInvalid
		}
		saved, found, err := readBudgetResolution(s, id)
		if err != nil {
			return err
		}
		if found {
			r := saved.Resolution
			if r.Outcome != in.Outcome || r.VerifiedCost != in.VerifiedCost || r.Evidence != in.Evidence || saved.Fingerprint != in.ExpectedFingerprint {
				return ErrConflict
			}
			result, already = r, true
			return nil
		}
		var hold BudgetReservation
		found, err = s.ID(id).Get(&hold)
		if err != nil {
			return err
		}
		if !found {
			return ErrNotFound
		}
		if budgetReservationFingerprint(hold) != in.ExpectedFingerprint {
			return ErrConflict
		}
		event, blocked, err := reconciliationEvidence(s, hold)
		if err != nil {
			return err
		}
		if blocked != "" {
			return ErrConflict
		}
		sum := event.Cost + in.VerifiedCost
		if math.IsInf(sum, 0) || math.IsNaN(sum) {
			return ErrInvalid
		}
		result = BudgetResolution{ReservationID: id, CallID: hold.CallID, Outcome: in.Outcome, VerifiedCost: in.VerifiedCost, CostDelta: in.VerifiedCost, PreviousCost: event.Cost, UpdatedCost: sum, Evidence: in.Evidence, ActorID: by.ID, ResolvedAt: time.Now().UTC()}
		saved = savedBudgetResolution{Resolution: result, Fingerprint: in.ExpectedFingerprint, Hold: hold, Event: event}
		payload, err := json.Marshal(saved)
		if err != nil {
			return err
		}
		if _, err = s.Exec("INSERT INTO budget_reconciliations(reservation_id,call_id,payload) VALUES (?, ?, ?::jsonb)", id, hold.CallID, string(payload)); err != nil {
			return err
		}
		if err = applyReconciliationCost(s, hold, event, in.VerifiedCost); err != nil {
			return err
		}
		if err = writeAudit(s, by, Audit{Action: "budget.reconcile", ObjectType: "budget_reservation", ObjectID: id, TeamID: hold.TeamID, Detail: map[string]any{"call_id": hold.CallID, "key_id": hold.KeyID, "outcome": in.Outcome, "verified_cost_usd": in.VerifiedCost, "previous_cost_usd": event.Cost, "updated_cost_usd": sum, "evidence": in.Evidence, "fingerprint": in.ExpectedFingerprint}}); err != nil {
			return err
		}
		_, err = s.ID(id).Delete(new(BudgetReservation))
		return err
	})
	return result, already, err
}

// applyReconciliationCost 仅累计人工核实的尝试费用；参数为已锁定hold、原调用事件及非负增量。
// 对账事务调用；保留原始价格和token，日汇总/监控不增加调用次数，组织消费按请求原周期处理。
func applyReconciliationCost(s *xorm.Session, h BudgetReservation, e UsageEvent, delta float64) error {
	if delta == 0 {
		return nil
	}
	if _, err := s.Exec("UPDATE usage_events SET cost = cost + ? WHERE request_id = ?", delta, e.RequestID); err != nil {
		return err
	}
	r := UsageRecord{TS: e.TS, KeyID: h.KeyID, UserID: h.UserID, TeamID: h.TeamID, ProjectID: h.ProjectID, OrganizationID: h.OrgID, OwnerType: e.OwnerType, Model: e.Model, Supplier: e.Supplier, UpstreamModel: e.UpstreamModel, Cost: delta}
	_, err := s.Exec(`INSERT INTO usage_daily(day,organization_id,team_id,project_id,user_id,key_id,owner_type,model,requests,prompt_tokens,completion_tokens,cost) VALUES (?,?,?,?,?,?,?,?,0,0,0,?) ON CONFLICT(day,organization_id,team_id,project_id,user_id,key_id,owner_type,model) DO UPDATE SET cost=usage_daily.cost+EXCLUDED.cost`, day(r.TS), r.OrganizationID, r.TeamID, r.ProjectID, r.UserID, r.KeyID, ownerType(r), r.Model, delta)
	if err != nil {
		return err
	}
	source, upstreamModel := budgetReservationSource(h)
	k := monitorKey{stamp(e.TS).Truncate(15 * time.Minute), h.OrgID, h.TeamID, h.ProjectID, h.UserID, h.KeyID, ownerType(r), source, upstreamModel}
	if err = flushMonitor(s, map[monitorKey]monitorDelta{k: {Cost: delta}}); err != nil {
		return err
	}
	r.TS = h.StartedAt
	return addScopeSpend(s, r)
}

// reconciledUsageFence 在自动结算之前核对人工终态，避免磁盘/Redis/PG迟到消息重复收费或释放其他尝试。
// 参数为当前串行事务及完整原始记录；返回true表示已被人工结论消费，一致重放可以确认，冲突返回ErrConflict并保留队列。
func reconciledUsageFence(s *xorm.Session, r UsageRecord) (bool, error) {
	if r.BudgetReservationID != "" {
		saved, found, err := readBudgetResolution(s, r.BudgetReservationID)
		if err != nil {
			return false, err
		}
		if found {
			h := saved.Hold
			if r.RequestID != h.CallID || r.KeyID != h.KeyID || r.UserID != h.UserID || r.TeamID != h.TeamID || r.OrganizationID != h.OrgID || r.ProjectID != h.ProjectID || r.Cost != saved.Resolution.VerifiedCost {
				return false, ErrConflict
			}
			return true, nil
		}
	}
	// 同call已有人工修正时，原零费用失败账单可继续幂等确认；新的非一致费用必须留待人工核对。
	var row struct {
		Payload string `xorm:"payload"`
	}
	found, err := s.Table("budget_reconciliations").Where("call_id = ?", r.RequestID).Asc("created_at", "reservation_id").Get(&row)
	if err != nil || !found {
		return false, err
	}
	var saved savedBudgetResolution
	if err = json.Unmarshal([]byte(row.Payload), &saved); err != nil {
		return false, err
	}
	e := saved.Event
	if r.KeyID != e.KeyID || r.UserID != e.UserID || r.TeamID != e.TeamID || r.OrganizationID != e.OrganizationID || r.ProjectID != e.ProjectID || r.Cost != e.Cost {
		return false, ErrConflict
	}
	if r.BudgetReservationID != "" {
		held, err := s.ID(r.BudgetReservationID).Exist(new(BudgetReservation))
		if err != nil {
			return false, err
		}
		if held {
			return false, ErrConflict
		}
	}
	return true, nil
}

// budgetReservationSource 仅读取本次尝试冻结的安全供应商/上游型号；旧记录缺字段返回空表示未知。
// 参数是预占行，返回连接标识和型号；列表和监控增量共用，不能用最终call的供应商冒充先前失败尝试。
func budgetReservationSource(h BudgetReservation) (string, string) {
	var snapshot struct {
		Supplier string `json:"supplier"`
		Model    string `json:"model"`
	}
	if json.Unmarshal([]byte(h.Snapshot), &snapshot) != nil {
		return "", ""
	}
	return snapshot.Supplier, snapshot.Model
}

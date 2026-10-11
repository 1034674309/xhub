package iam

import (
 "context"
 "errors"
 "math"

 "xorm.io/xorm"
)

// ErrBudgetUnavailable 表示本次预占超过某层可用余额；调用方返回429而不调用上游。
var ErrBudgetUnavailable = errors.New("budget reservation exceeds available balance")

// BudgetReservation 保存正在执行请求的计费路径快照，不设自动过期以避免仍执行的请求被提前释放。
// 崩溃后遗留占用必须由确认请求已终止的恢复流程释放，不能通过超时猜测释放资金。
type BudgetReservation struct {
 RequestID string `xorm:"pk 'request_id'"`
 Kind string `xorm:"'kind'"`
 NodeID string `xorm:"'node_id'"`
 KeyID string `xorm:"'key_id'"`
 UserID string `xorm:"'user_id'"`
 TeamID string `xorm:"'team_id'"`
 OrgID string `xorm:"'org_id'"`
 Amount float64 `xorm:"'amount'"`
}

// TableName 返回预占表名；无参数和副作用，供xorm查询使用。
func (BudgetReservation) TableName() string { return "budget_reservations" }

// applyBudgetHolds 将已占金额叠加到当前树的各层消费，避免固定额度和共享额度重复计数。
// 参数s/nodes为同一事务与额度树；返回数据库错误，供分配校验和请求准入调用，不修改持久消费。
func applyBudgetHolds(s *xorm.Session, nodes map[string]*quotaNode) error {
 var holds []BudgetReservation
 if err := s.Find(&holds); err != nil { return err }
 for _, h := range holds {
  for kind, id := range map[string]string{"key": h.KeyID, "user": h.UserID, "team": h.TeamID, "org": h.OrgID} {
   if n := nodes[kind+":"+id]; n != nil { n.Spend += h.Amount }
  }
 }
 return nil
}

// ReserveBudget 在事务锁内为请求预占金额；参数ctx/requestID/kind/id/amount为上下文、幂等ID、主体和费用上界。
// 返回持久快照或余额/存储错误；相同ID只允许相同主体金额重试，已结算ID不能重新占用。
// 调用方必须在转发前执行，结算用RecordUsage原子释放，未转发或失败可用ReleaseBudget释放。
func (db *DB) ReserveBudget(ctx context.Context, requestID, kind, id string, amount float64) (*BudgetReservation, error) {
 if requestID == "" || (kind != "key" && kind != "user") || id == "" || amount < 0 || math.IsNaN(amount) || math.IsInf(amount, 0) { return nil, ErrInvalid }
 var hold BudgetReservation
 err := db.tx(ctx, func(s *xorm.Session) error {
  found, err := s.ID(requestID).Get(&hold)
  if err != nil { return err }
  if found {
   if hold.Kind != kind || hold.NodeID != id || hold.Amount != amount { return ErrConflict }
   return nil
  }
  settled, err := s.Where("request_id = ?", requestID).Exist(new(UsageEvent))
  if err != nil { return err }; if settled { return ErrConflict }
  nodes, err := quotaTree(s)
  if err != nil { return err }
  n := nodes[kind+":"+id]
  if n == nil { return ErrNotFound }
  if err := applyBudgetHolds(s, nodes); err != nil { return err }
  hold = BudgetReservation{RequestID: requestID, Kind: kind, NodeID: id, Amount: amount}
  var child *quotaNode
  credit := 0.0
  for n != nil {
   switch n.Kind {
   case "key": hold.KeyID = n.ID
   case "user": hold.UserID = n.ID
   case "team": hold.TeamID = n.ID
   case "org": hold.OrgID = n.ID
   }
   if n.Parent == "ambiguous" { return ErrMultipleTeams }
   if n.Limit != nil {
    reserved := 0.0
    for _, c := range n.Children {
     value := quotaReserved(c)
     if c == child { value = math.Max(0, value-credit) }
     reserved += value
    }
    if n.Spend+reserved+amount > *n.Limit+1e-9 { return ErrBudgetUnavailable }
    credit = math.Max(0, *n.Limit-n.Spend)
   }
   child = n
   if n.Parent != "" && nodes[n.Parent] == nil { return ErrInvalid }
   n = nodes[n.Parent]
  }
  _, err = s.Insert(&hold)
  return err
 })
 if err != nil { return nil, err }; return &hold, nil
}

// ReleaseBudget 释放确认不再执行的请求占用；参数ctx/requestID为上下文和请求ID，返回存储错误。
// 调用：请求退出/恢复流程；重复释放无害，数据库不可用时保留占用而非允许超支。
func (db *DB) ReleaseBudget(ctx context.Context, requestID string) error {
 if requestID == "" { return ErrInvalid }
 return db.tx(ctx, func(s *xorm.Session) error { _, err := s.ID(requestID).Delete(new(BudgetReservation)); return err })
}

// settleBudgetHold 校验结算归属必须等于准入快照并删除占用；参数s/r为结算事务和账单，返回冲突或存储错误。
// RecordUsage在写实际消费的同一事务调用；上游异常超出预占仍记录真实账单，不能隐瞒实际费用。
func settleBudgetHold(s *xorm.Session, r UsageRecord) error {
 var h BudgetReservation
 found, err := s.ID(r.RequestID).Get(&h)
 if err != nil || !found { return err }
 if h.KeyID != r.KeyID || h.UserID != r.UserID || h.TeamID != r.TeamID || h.OrgID != r.OrganizationID { return ErrConflict }
 _, err = s.ID(r.RequestID).Delete(new(BudgetReservation))
 return err
}

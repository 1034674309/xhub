package iam

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"xorm.io/xorm"
)

// usageFingerprint 对完整冻结UsageRecord计算稳定SHA256，供账单写入与严格重放一致性确认。
// 参数须已通过用量校验/冻结TS；返回摘要或序列化错误，不将正文写入日志或调用外部服务。
func usageFingerprint(record UsageRecord) (string, error) {
	raw, err := json.Marshal(record)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// checkPendingUsageSnapshot 在当前结算事务核对同ID待结算证据，JSONB比较忽略对象字段顺序但保留全部值。
// 供strict分支在核销fence之后调用；已有不同快照返回冲突，不能结算旧载荷后确认当前磁盘文件。
func checkPendingUsageSnapshot(s *xorm.Session, record UsageRecord) error {
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}
	rows, err := s.QueryString("SELECT request_id FROM usage_pending WHERE request_id = ? AND payload <> CAST(? AS jsonb)", record.RequestID, string(payload))
	if err != nil {
		return err
	}
	if len(rows) > 0 {
		return fmt.Errorf("%w: pending usage snapshot differs", ErrConflict)
	}
	return nil
}

// checkCommittedUsageSnapshot 在insertEvent发现重复ID后严格核对完整指纹；SQL插入冲突等待并发事务完成，当前事务保证判重后才退款。
// 旧无指纹记录不能可靠重建完整正文/预占ID，因此返回ErrConflict保留磁盘证据，由人工核销fence处理。
func checkCommittedUsageSnapshot(s *xorm.Session, record UsageRecord) error {
	fingerprint, err := usageFingerprint(record)
	if err != nil {
		return err
	}
	var existing UsageEvent
	found, err := s.Where("request_id = ?", record.RequestID).Cols("usage_fingerprint").Get(&existing)
	if err != nil {
		return err
	}
	if !found || existing.UsageFingerprint == "" || existing.UsageFingerprint != fingerprint {
		return fmt.Errorf("%w: committed usage snapshot differs or has no fingerprint", ErrConflict)
	}
	return nil
}

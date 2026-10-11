package iam

import (
	"context"
	"xorm.io/xorm"
)

// ArchiveConfig 是不可变版本的 Kodo 目标，Credentials 为加密 JSON，不得通过管理接口回显。
type ArchiveConfig struct {
	Revision        int64  `xorm:"pk autoincr" json:"revision"`
	Bucket          string `json:"bucket"`
	Region          string `json:"region"`
	Prefix          string `json:"prefix"`
	Credentials     string `json:"-"`
	Enabled         bool   `json:"enabled"`
	IntervalSeconds int    `json:"interval_seconds"`
	BatchSize       int    `json:"batch_size"`
}

// TableName 返回配置表名供 xorm 映射，无参数和副作用。
func (ArchiveConfig) TableName() string { return "archive_configs" }

// ArchiveSchedule 校验并补齐调度参数；零值表示使用系统默认，返回默认化间隔、每轮上限及合法标记。
// 管理接口与 worker 共用，间隔限制为 2 秒至一天、上限为 1 至 200；无数据库或网络副作用。
func ArchiveSchedule(interval, batch int) (int, int, bool) {
	if interval == 0 {
		interval = 2
	}
	if batch == 0 {
		batch = 20
	}
	return interval, batch, interval >= 2 && interval <= 86400 && batch >= 1 && batch <= 200
}

// ArchiveEvent 保存不可变快照与 manifest，ID 是全局事件版本及对象幂等键。
type ArchiveEvent struct {
	ID        int64  `xorm:"pk autoincr 'id'" json:"id"`
	RequestID string `xorm:"'request_id'" json:"request_id"`
	Revision  int64  `json:"revision"`
	Payload   string `json:"-"`
	Lease     string `json:"-"`
	ObjectKey string `json:"object_key"`
	SHA256    string `xorm:"'sha256'" json:"sha256"`
	ByteSize  int64  `json:"byte_size"`
	State     string `json:"state"`
	Attempts  int    `json:"attempts"`
	Error     string `json:"error"`
}

// TableName 返回 outbox 表名供 xorm 映射，无参数和副作用。
func (ArchiveEvent) TableName() string { return "archive_events" }

// ArchiveConfiguration 读取指定版本，revision=0 取最新；供管理和 worker 使用，返回配置或错误。
func (db *DB) ArchiveConfiguration(ctx context.Context, revision int64) (*ArchiveConfig, error) {
	s := db.session(ctx)
	defer s.Close()
	if revision == 0 {
		s.Desc("revision")
	} else {
		s.ID(revision)
	}
	var c ArchiveConfig
	err := get(s, &c)
	if err != nil {
		return nil, err
	}
	return &c, err
}

// SaveArchiveConfiguration 默认化并校验调度后插入版本和审计；参数为管理员及密文目标配置。
// 管理接口及内部调用共用；非法调度返回 ErrInvalid，不创建版本，数据库失败全部回滚。
func (db *DB) SaveArchiveConfiguration(ctx context.Context, by Actor, c ArchiveConfig) error {
	interval, batch, valid := ArchiveSchedule(c.IntervalSeconds, c.BatchSize)
	if !valid {
		return ErrInvalid
	}
	c.IntervalSeconds, c.BatchSize = interval, batch
	c.Revision = 0
	return db.tx(ctx, func(s *xorm.Session) error {
		if _, err := s.Insert(&c); err != nil {
			return err
		}
		return writeAudit(s, by, Audit{Action: "archive.configure", ObjectType: "archive", ObjectID: "kodo"})
	})
}

// ClaimArchiveEvent 原子领取待处理或过期事件；lease 是随机 fencing token，返回事件或 nil。
// worker 重启可重领；失败十次后需管理员重试，网络调用不占用数据库事务。
func (db *DB) ClaimArchiveEvent(ctx context.Context, lease string) (*ArchiveEvent, error) {
	rows := []ArchiveEvent{}
	err := db.Engine.Context(ctx).SQL(`UPDATE archive_events SET state='processing',lease=?,lease_until=now()+interval '2 minutes',attempts=attempts+1
 WHERE id=(SELECT id FROM archive_events WHERE (state='pending' OR (state='processing' AND lease_until<now()) OR (state='failed' AND attempts<10 AND lease_until<now())) ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING *`, lease).Find(&rows)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}

// CompleteArchiveEvent 用租约提交成功 manifest 或固定错误；返回是否仍拥有租约及数据库错误。
// key/hash/size 描述核验对象，失败指数退避；旧 worker 不得覆盖新领取结果。
func (db *DB) CompleteArchiveEvent(ctx context.Context, e *ArchiveEvent, key, hash string, size int64, success bool) (bool, error) {
	state, detail := "done", ""
	if !success {
		state, detail = "failed", "archive_upload_failed"
	}
	result, err := db.Engine.Context(ctx).Exec(`UPDATE archive_events SET state=?,error=?,object_key=?,sha256=?,byte_size=?,completed_at=CASE WHEN ? THEN now() ELSE NULL END,lease_until=now()+make_interval(secs=>LEAST(3600,CAST(power(2,LEAST(attempts,10))*10 AS INT))) WHERE id=? AND lease=? AND state='processing' AND lease_until>now()`, state, detail, key, hash, size, success, e.ID, e.Lease)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}

// ArchiveStatus 返回状态计数及最近二十条结果，不返回正文或凭据；供管理员查看，无写副作用。
func (db *DB) ArchiveStatus(ctx context.Context) (map[string]int64, []ArchiveEvent, error) {
	rows := []struct {
		State string
		Count int64
	}{}
	err := db.Engine.Context(ctx).SQL("SELECT state,count(*) FROM archive_events GROUP BY state").Find(&rows)
	counts := map[string]int64{}
	for _, r := range rows {
		counts[r.State] = r.Count
	}
	events := []ArchiveEvent{}
	if err == nil {
		err = db.Engine.Context(ctx).Desc("id").Limit(20).Find(&events)
	}
	return counts, events, err
}

// RetryArchive 重置失败事件并审计；参数为管理员，返回错误，不改变内容或目标版本。
func (db *DB) RetryArchive(ctx context.Context, by Actor) error {
	return db.tx(ctx, func(s *xorm.Session) error {
		if _, err := s.Exec("UPDATE archive_events SET state='pending',attempts=0,error='' WHERE state='failed'"); err != nil {
			return err
		}
		return writeAudit(s, by, Audit{Action: "archive.retry", ObjectType: "archive", ObjectID: "kodo"})
	})
}

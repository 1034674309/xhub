package iam

import (
	"context"
	"strings"
	"time"
	"xorm.io/xorm"
)

// EnterpriseOIDC 保存单组织登录配置；Secret 是密文且永不 JSON 回显。
type EnterpriseOIDC struct {
	Name          string `json:"name"`
	ConnectionID  string `xorm:"pk 'connection_id'" json:"connection_id"`
	Issuer        string `json:"issuer"`
	ClientID      string `xorm:"'client_id'" json:"client_id"`
	Secret        string `json:"-"`
	RedirectURI   string `xorm:"'redirect_uri'" json:"redirect_uri"`
	ReturnURI     string `xorm:"'return_uri'" json:"return_uri"`
	IdentityClaim string `xorm:"'identity_claim'" json:"identity_claim"`
	Enabled       bool   `json:"enabled"`
	Revision      int64  `json:"revision"`
}

// TableName 返回登录配置表，由 xorm 调用，无参数或副作用。
func (EnterpriseOIDC) TableName() string { return "enterprise_oidc" }

// EnterpriseLoginTransaction 是单次浏览器绑定认证事务；数据库仅保存 state/browser 的哈希。
type EnterpriseLoginTransaction struct {
	StateHash    string `xorm:"pk 'state_hash'"`
	ConnectionID string `xorm:"'connection_id'"`
	Revision     int64
	BrowserHash  string `xorm:"'browser_hash'"`
	Nonce        string
	Verifier     string
	ExpiresAt    time.Time `xorm:"'expires_at'"`
}

// TableName 返回认证事务表名，由 xorm 调用，无副作用。
func (EnterpriseLoginTransaction) TableName() string { return "enterprise_login_transactions" }

// SaveEnterpriseOIDC 保存加密后的配置并提升版本；by 为管理员，in 的连接不能改变组织。
// 返回错误；关键配置改变使旧认证事务失效，成功记录审计，不删除稳定身份绑定。
func (db *DB) SaveEnterpriseOIDC(ctx context.Context, by Actor, in EnterpriseOIDC) error {
	return db.tx(ctx, func(s *xorm.Session) error {
		if err := get(s.SQL("SELECT * FROM directory_connections WHERE id=? FOR UPDATE", in.ConnectionID), new(DirectoryConnection)); err != nil {
			return err
		}
		if in.Name == "" {
			// 旧客户端省略名称时优先保留已有登录名称，不能从 SCIM 名称覆盖独立配置。
			var old EnterpriseOIDC
			found, err := s.ID(in.ConnectionID).Get(&old)
			if err != nil {
				return err
			}
			if found {
				in.Name = old.Name
			}
		}
		if in.Name == "" {
			var d DirectoryConnection
			if err := get(s.ID(in.ConnectionID), &d); err != nil {
				return err
			}
			in.Name = d.Name
		}
		if strings.TrimSpace(in.Name) == "" {
			return ErrInvalid
		}
		if err := saveEnterpriseOIDC(s, in); err != nil {
			return err
		}
		return writeAudit(s, by, Audit{Action: "enterprise.oidc.configure", ObjectType: "directory_connection", ObjectID: in.ConnectionID})
	})
}

// EnterpriseOIDCByID 读取服务端配置；调用方必须保护 Secret，返回配置或存储错误，无写入。
func (db *DB) EnterpriseOIDCByID(ctx context.Context, id string) (*EnterpriseOIDC, error) {
	s := db.session(ctx)
	defer s.Close()
	var out EnterpriseOIDC
	err := get(s.ID(id), &out)
	return &out, err
}

// EnterpriseOIDCOptions 只列出可用组织的已启用登录入口；返回无秘密的配置，供公开登录页使用。
func (db *DB) EnterpriseOIDCOptions(ctx context.Context) ([]EnterpriseOIDC, error) {
	out := []EnterpriseOIDC{}
	err := db.Engine.Context(ctx).SQL(`SELECT o.* FROM enterprise_oidc o JOIN directory_connections d ON d.id=o.connection_id JOIN organizations g ON g.id=d.organization_id WHERE o.enabled=true AND g.status='active' ORDER BY o.connection_id`).Find(&out)
	return out, err
}

// StartEnterpriseLogin 保存短时认证事务并清理过期记录；tx 已由网关随机生成，返回错误。
// 事务范围内校验配置版本，最多每连接 1000 个未完成事务，防止公开入口无界写入。
func (db *DB) StartEnterpriseLogin(ctx context.Context, tx EnterpriseLoginTransaction) error {
	return db.tx(ctx, func(s *xorm.Session) error {
		var c EnterpriseOIDC
		if err := get(s.SQL("SELECT * FROM enterprise_oidc WHERE connection_id=? FOR UPDATE", tx.ConnectionID), &c); err != nil {
			return err
		}
		if !c.Enabled || c.Revision != tx.Revision {
			return ErrInactive
		}
		var d DirectoryConnection
		if err := get(s.ID(tx.ConnectionID), &d); err != nil {
			return err
		}
		var org Organization
		if err := get(s.ID(d.OrganizationID), &org); err != nil {
			return err
		}
		// SCIM 仅管理同步，SSO 只受自己的开关和组织状态控制。
		if org.Status != StatusActive {
			return ErrInactive
		}
		if _, err := s.Exec("DELETE FROM enterprise_login_transactions WHERE expires_at < now()"); err != nil {
			return err
		}
		n, err := s.Where("connection_id=?", tx.ConnectionID).Count(new(EnterpriseLoginTransaction))
		if err != nil {
			return err
		}
		if n >= 1000 {
			return ErrConflict
		}
		_, err = s.Insert(&tx)
		return err
	})
}

// ConsumeEnterpriseLogin 原子消费 state 与浏览器凭据匹配的未过期事务；返回事务或统一拒绝错误。
// 错误浏览器不消费正确事务，重放不能再次兑换，配置版本在回调签发前再次检查。
func (db *DB) ConsumeEnterpriseLogin(ctx context.Context, state, browser string) (*EnterpriseLoginTransaction, error) {
	out := []EnterpriseLoginTransaction{}
	err := db.Engine.Context(ctx).SQL("DELETE FROM enterprise_login_transactions WHERE state_hash=? AND browser_hash=? AND expires_at>now() RETURNING *", HashKey(state), HashKey(browser)).Find(&out)
	if err != nil {
		return nil, err
	}
	if len(out) != 1 {
		return nil, ErrInactive
	}
	return &out[0], nil
}

// EnterpriseLoginUser 根据已验证 issuer/sub 和共同稳定标识绑定已同步用户；不使用邮箱匹配。
// cfgRevision 阻止配置变更后的旧回调；返回活跃普通用户或错误，绑定与审计在事务内完成。
func (db *DB) EnterpriseLoginUser(ctx context.Context, id, issuer, subject, externalID string, cfgRevision int64) (*User, error) {
	var u User
	err := db.tx(ctx, func(s *xorm.Session) error {
		var c EnterpriseOIDC
		if err := get(s.SQL("SELECT * FROM enterprise_oidc WHERE connection_id=? FOR UPDATE", id), &c); err != nil {
			return err
		}
		if !c.Enabled || c.Revision != cfgRevision || c.Issuer != issuer || subject == "" {
			return ErrInactive
		}
		var d DirectoryConnection
		if err := get(s.ID(id), &d); err != nil {
			return err
		}
		var org Organization
		if err := get(s.ID(d.OrganizationID), &org); err != nil {
			return err
		}
		if org.Status != StatusActive {
			return ErrInactive
		}
		var binding struct {
			UserID string `xorm:"'user_id'"`
		}
		found, err := s.SQL("SELECT user_id FROM enterprise_login_identities WHERE connection_id=? AND issuer=? AND subject=?", id, issuer, subject).Get(&binding)
		if err != nil {
			return err
		}
		var du DirectoryUser
		if found {
			err = get(s.Where("connection_id=? AND user_id=?", id, binding.UserID), &du)
		} else {
			if externalID == "" {
				return ErrInvalid
			}
			err = get(s.Where("connection_id=? AND external_id=?", id, externalID), &du)
		}
		if err != nil {
			return err
		}
		if !du.Active {
			return ErrInactive
		}
		if err := get(s.ID(du.UserID), &u); err != nil {
			return err
		}
		if u.Status != StatusActive || u.Role != RoleUser {
			return ErrInactive
		}
		if !found {
			if _, err := s.Exec("INSERT INTO enterprise_login_identities(connection_id,issuer,subject,user_id) VALUES(?,?,?,?)", id, issuer, subject, u.ID); err != nil {
				return err
			}
		}
		return writeAudit(s, Actor{Kind: "system", ID: u.ID}, Audit{Action: "enterprise.oidc.login", ObjectType: "user", ObjectID: u.ID})
	})
	return &u, err
}

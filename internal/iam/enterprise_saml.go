package iam

import (
	"context"
	"strings"
)

// EnterpriseSAMLTransaction 读取未过期的 SAML 请求关联；state 为随机 RelayState，返回事务或错误。
// ACS 尚未取得跨站 Cookie，只能验证签名和暂存身份，不能借此接口签发会话。
func (db *DB) EnterpriseSAMLTransaction(ctx context.Context, state string) (*EnterpriseLoginTransaction, error) {
	var out EnterpriseLoginTransaction
	s := db.session(ctx)
	defer s.Close()
	err := get(s.Where("state_hash=? AND expires_at>now()", HashKey(state)), &out)
	return &out, err
}

// StageEnterpriseSAML 在原请求上仅一次保存已验签的稳定身份；state/revision/subject 来自网关验证。
// 返回冲突或存储错误，不产生登录会话；配置修改、重复回调及过期请求均拒绝，最终仍需浏览器凭据。
func (db *DB) StageEnterpriseSAML(ctx context.Context, state string, revision int64, subject string) error {
	if strings.TrimSpace(subject) == "" || len(subject) > 1024 {
		return ErrInvalid
	}
	result, err := db.Engine.Context(ctx).Exec(`UPDATE enterprise_login_transactions t SET saml_subject=? FROM enterprise_oidc c WHERE t.state_hash=? AND t.expires_at>now() AND t.saml_subject='' AND t.revision=? AND c.connection_id=t.connection_id AND c.revision=t.revision AND c.enabled=true AND c.protocol='saml'`, subject, HashKey(state), revision)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrInactive
	}
	return nil
}

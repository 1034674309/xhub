package iam

import (
	"context"
	"xorm.io/xorm"
)

// DirectoryManagement 是管理员读取的连接快照；不包含同步凭据或本地用户秘密。
type DirectoryManagement struct {
	Connection DirectoryConnection    `json:"connection"`
	Groups     []DirectoryGroup       `json:"groups"`
	Mappings   []DirectoryMapping     `json:"mappings"`
	Users      []DirectoryManagedUser `json:"users"`
}

// DirectoryManagedUser 包含分配来源与实际团队；供人工覆盖页面使用，不暴露预算和密钥。
type DirectoryManagedUser struct {
	DirectoryUserResource `xorm:"extends"`
	TeamID                string `xorm:"'team_id'" json:"team_id"`
}

// DirectoryManagementSnapshot 在连接锁下读取一致的管理快照；ctx/id 指定生命周期与连接。
// 返回连接、组、映射和用户归属或错误；仅管理接口调用，不修改数据，不返回 TokenHash。
func (db *DB) DirectoryManagementSnapshot(ctx context.Context, id string) (*DirectoryManagement, error) {
	out := DirectoryManagement{Groups: []DirectoryGroup{}, Mappings: []DirectoryMapping{}, Users: []DirectoryManagedUser{}}
	err := db.tx(ctx, func(s *xorm.Session) error {
		if err := get(s.SQL("SELECT * FROM directory_connections WHERE id=? AND scim_configured=true FOR UPDATE", id), &out.Connection); err != nil {
			return err
		}
		if err := s.Where("connection_id=?", id).Asc("id").Find(&out.Groups); err != nil {
			return err
		}
		if err := s.Where("connection_id=?", id).Asc("priority", "id").Find(&out.Mappings); err != nil {
			return err
		}
		return s.SQL("SELECT d.*,u.email,u.name,COALESCE(tm.team_id,'') AS team_id FROM directory_users d JOIN users u ON u.id=d.user_id LEFT JOIN team_members tm ON tm.user_id=d.user_id WHERE d.connection_id=? ORDER BY d.id", id).Find(&out.Users)
	})
	return &out, err
}

// DirectoryUserResource 是 SCIM 可读快照；只含目录属性，不暴露密码、预算或密钥。
type DirectoryUserResource struct {
	DirectoryUser `xorm:"extends"`
	Email         string `json:"userName"`
	Name          string `json:"displayName"`
}

// DirectoryUsersPage 按连接查询稳定分页；field 仅允许 externalId/userName，value 为精确值。
// 参数 offset/count 控制分页，返回资源、总数或错误；供 SCIM 使用，无写副作用。
func (db *DB) DirectoryUsersPage(ctx context.Context, connectionID, field, value string, offset, count int) ([]DirectoryUserResource, int64, error) {
	if offset < 0 || count < 0 || count > 200 || (field != "" && field != "externalId" && field != "userName") {
		return nil, 0, ErrInvalid
	}
	// 使用固定 SQL 列名及绑定参数，禁止客户端过滤属性进入 SQL。
	where := "d.connection_id = ?"
	args := []any{connectionID}
	if field == "externalId" {
		where += " AND d.external_id = ?"
		args = append(args, value)
	}
	if field == "userName" {
		where += " AND u.email = ?"
		args = append(args, normEmail(value))
	}
	var totals []struct{ Total int64 }
	if err := db.Engine.Context(ctx).SQL("SELECT count(*) AS total FROM directory_users d JOIN users u ON u.id=d.user_id WHERE "+where, args...).Find(&totals); err != nil {
		return nil, 0, err
	}
	out := []DirectoryUserResource{}
	if count > 0 {
		pageArgs := append(append([]any{}, args...), count, offset)
		if err := db.Engine.Context(ctx).SQL("SELECT d.*,u.email,u.name FROM directory_users d JOIN users u ON u.id=d.user_id WHERE "+where+" ORDER BY d.id LIMIT ? OFFSET ?", pageArgs...).Find(&out); err != nil {
			return nil, 0, err
		}
	}
	return out, totals[0].Total, nil
}

// DirectoryUserByID 在连接范围读取资源；id 为 SCIM id，返回快照或不存在错误。
// 供详情和更新使用，跨连接访问与不存在等价，不修改状态。
func (db *DB) DirectoryUserByID(ctx context.Context, connectionID, id string) (*DirectoryUserResource, error) {
	s := db.session(ctx)
	defer s.Close()
	var out DirectoryUserResource
	err := get(s.SQL("SELECT d.*,u.email,u.name FROM directory_users d JOIN users u ON u.id=d.user_id WHERE d.connection_id=? AND d.id=?", connectionID, id), &out)
	return &out, err
}

// RotateDirectoryToken 原子轮换连接凭据；新 tokenHash 由管理端生成，仅保存哈希。
// 参数 by 为已授权管理主体，返回错误；旧凭据立即失效，成功产生审计。
func (db *DB) RotateDirectoryToken(ctx context.Context, by Actor, id, tokenHash string) error {
	if tokenHash == "" {
		return ErrInvalid
	}
	return db.tx(ctx, func(s *xorm.Session) error {
		var c DirectoryConnection
		if err := get(s.SQL("SELECT * FROM directory_connections WHERE id=? AND scim_configured=true FOR UPDATE", id), &c); err != nil {
			return err
		}
		if _, err := s.Exec("UPDATE directory_connections SET token_hash=?, revision=revision+1 WHERE id=?", tokenHash, id); err != nil {
			return err
		}
		return writeAudit(s, by, Audit{Action: "directory.token.rotate", ObjectType: "directory_connection", ObjectID: id})
	})
}

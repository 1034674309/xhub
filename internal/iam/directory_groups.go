package iam

import (
	"context"
	"xorm.io/xorm"
)

// DirectoryGroupResource 保存组与稳定排序的成员，供 SCIM 读取，不包含本地权限。
type DirectoryGroupResource struct {
	DirectoryGroup
	Members []string
}

// DirectoryGroupByID 按连接读取企业组与用户成员；参数为连接和组 id，返回快照或错误。
// SCIM 详情使用，事务内一致读取，跨连接等同不存在，无写副作用。
func (db *DB) DirectoryGroupByID(ctx context.Context, connectionID, id string) (*DirectoryGroupResource, error) {
	var out DirectoryGroupResource
	err := db.tx(ctx, func(s *xorm.Session) error {
		if err := get(s.SQL("SELECT * FROM directory_connections WHERE id=? AND scim_configured=true FOR UPDATE", connectionID), new(DirectoryConnection)); err != nil {
			return err
		}
		if err := get(s.Where("id=? AND connection_id=?", id, connectionID), &out.DirectoryGroup); err != nil {
			return err
		}
		var members []struct {
			ID string `xorm:"'id'"`
		}
		if err := s.SQL("SELECT directory_user_id AS id FROM directory_group_members WHERE connection_id=? AND group_id=? ORDER BY directory_user_id", connectionID, id).Find(&members); err != nil {
			return err
		}
		out.Members = []string{}
		for _, m := range members {
			out.Members = append(out.Members, m.ID)
		}
		return nil
	})
	return &out, err
}

// DirectoryGroupsPage 查询精确过滤与稳定分页；参数 field 仅 externalId/displayName，offset/count 为范围。
// 返回组、总数或错误；SCIM 列表调用，零 count 只返回总数，不改变目录。
func (db *DB) DirectoryGroupsPage(ctx context.Context, connectionID, field, value string, offset, count int) ([]DirectoryGroup, int64, error) {
	if offset < 0 || count < 0 || count > 200 || (field != "" && field != "externalId" && field != "displayName") {
		return nil, 0, ErrInvalid
	}
	where := "connection_id=?"
	args := []any{connectionID}
	if field == "externalId" {
		where += " AND external_id=?"
		args = append(args, value)
	}
	if field == "displayName" {
		where += " AND name=?"
		args = append(args, value)
	}
	var totals []struct{ Total int64 }
	if err := db.Engine.Context(ctx).SQL("SELECT count(*) AS total FROM directory_groups WHERE "+where, args...).Find(&totals); err != nil {
		return nil, 0, err
	}
	out := []DirectoryGroup{}
	if count > 0 {
		args = append(args, count, offset)
		if err := db.Engine.Context(ctx).SQL("SELECT * FROM directory_groups WHERE "+where+" ORDER BY id LIMIT ? OFFSET ?", args...).Find(&out); err != nil {
			return nil, 0, err
		}
	}
	return out, totals[0].Total, nil
}

// DeleteDirectoryGroup 删除指定连接的组并重新计算失效归属；参数为连接和组 id。
// 返回错误；SCIM 删除调用，成员与映射级联清除，人工覆盖保留，失败全部回滚并记录审计。
func (db *DB) DeleteDirectoryGroup(ctx context.Context, connectionID, id string) error {
	return db.tx(ctx, func(s *xorm.Session) error {
		var c DirectoryConnection
		if err := get(s.SQL("SELECT * FROM directory_connections WHERE id=? AND scim_configured=true FOR UPDATE", connectionID), &c); err != nil {
			return err
		}
		var g DirectoryGroup
		if err := get(s.Where("connection_id=? AND id=?", connectionID, id), &g); err != nil {
			return err
		}
		if _, err := s.ID(id).Delete(&DirectoryGroup{}); err != nil {
			return err
		}
		if err := reconcileDirectory(ctx, s, c, ""); err != nil {
			return err
		}
		if _, err := s.Exec("UPDATE directory_connections SET revision=revision+1 WHERE id=?", connectionID); err != nil {
			return err
		}
		return writeAudit(s, Actor{Kind: "system"}, Audit{Action: "directory.group.delete", ObjectType: "directory_group", ObjectID: id})
	})
}

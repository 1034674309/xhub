package iam

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"xorm.io/xorm"
)

// DirectoryPatchOperation 是 SCIM 原子补丁操作；Value 保持 JSON 类型，避免布尔值被字符串化。
type DirectoryPatchOperation struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

// PatchDirectoryUser 在连接锁内顺序应用用户补丁；id 为本连接资源，ops 是有限操作集。
// 返回提交身份或错误；只允许目录管理属性，任何操作失败全部回滚，不允许改权限或稳定身份。
func (db *DB) PatchDirectoryUser(ctx context.Context, connectionID, id string, ops []DirectoryPatchOperation) (*DirectoryUser, error) {
	d, _, err := db.provisionDirectoryUser(ctx, connectionID, "", "", "", false, func(s *xorm.Session) (string, string, string, bool, error) {
		var row DirectoryUserResource
		if err := get(s.SQL("SELECT d.*,u.email,u.name FROM directory_users d JOIN users u ON u.id=d.user_id WHERE d.connection_id=? AND d.id=?", connectionID, id), &row); err != nil {
			return "", "", "", false, err
		}
		if err := applyDirectoryUserPatch(&row, ops); err != nil {
			return "", "", "", false, err
		}
		return row.ExternalID, row.Email, row.Name, row.Active, nil
	})
	return d, err
}

// applyDirectoryUserPatch 解析用户字段补丁；row 是事务快照，ops 顺序执行，返回校验错误。
// 只由同步事务与单测调用；失败时调用方必须丢弃快照，不写入数据库。
func applyDirectoryUserPatch(row *DirectoryUserResource, ops []DirectoryPatchOperation) error {
	if len(ops) == 0 || len(ops) > 100 {
		return ErrInvalid
	}
	for _, op := range ops {
		kind := strings.ToLower(op.Op)
		path := strings.ToLower(op.Path)
		if kind != "add" && kind != "replace" && kind != "remove" {
			return ErrInvalid
		}
		if path == "" {
			if kind == "remove" {
				return ErrInvalid
			}
			var fields map[string]json.RawMessage
			if json.Unmarshal(op.Value, &fields) != nil || len(fields) == 0 {
				return ErrInvalid
			}
			nested := []DirectoryPatchOperation{}
			for key, value := range fields {
				nested = append(nested, DirectoryPatchOperation{Op: kind, Path: key, Value: value})
			}
			if err := applyDirectoryUserPatch(row, nested); err != nil {
				return err
			}
			continue
		}
		if string(op.Value) == "null" && kind != "remove" {
			return ErrInvalid
		}
		switch path {
		case "active":
			if kind == "remove" || json.Unmarshal(op.Value, &row.Active) != nil {
				return ErrInvalid
			}
		case "username":
			if kind == "remove" || json.Unmarshal(op.Value, &row.Email) != nil {
				return ErrInvalid
			}
		case "displayname":
			if kind == "remove" {
				row.Name = ""
			} else if json.Unmarshal(op.Value, &row.Name) != nil {
				return ErrInvalid
			}
		default:
			return ErrInvalid
		}
	}
	return nil
}

// PatchDirectoryGroup 在连接锁内读取成员并原子应用组补丁；只支持 User 成员，不支持嵌套组。
// 参数是连接、资源和操作，返回组或错误；失败回滚成员关系、映射归属与审计。
func (db *DB) PatchDirectoryGroup(ctx context.Context, connectionID, id string, ops []DirectoryPatchOperation) (*DirectoryGroup, error) {
	return db.replaceDirectoryGroup(ctx, connectionID, "", "", nil, func(s *xorm.Session) (string, string, []string, error) {
		var g DirectoryGroup
		if err := get(s.Where("connection_id=? AND id=?", connectionID, id), &g); err != nil {
			return "", "", nil, err
		}
		var rows []struct {
			ID string `xorm:"'id'"`
		}
		if err := s.SQL("SELECT directory_user_id AS id FROM directory_group_members WHERE connection_id=? AND group_id=? ORDER BY directory_user_id", connectionID, id).Find(&rows); err != nil {
			return "", "", nil, err
		}
		members := []string{}
		for _, r := range rows {
			members = append(members, r.ID)
		}
		name, members, err := applyDirectoryGroupPatch(g.Name, members, ops)
		return g.ExternalID, name, members, err
	})
}

// applyDirectoryGroupPatch 计算目标成员集；name/members 为快照，ops 支持成员添加、替换与精确删除。
// 返回新名称、成员或错误，不修改输入切片；不支持的筛选显式拒绝，防止误删整组。
func applyDirectoryGroupPatch(name string, members []string, ops []DirectoryPatchOperation) (string, []string, error) {
	members = append([]string{}, members...)
	if len(ops) == 0 || len(ops) > 100 {
		return "", nil, ErrInvalid
	}
	for _, op := range ops {
		kind := strings.ToLower(op.Op)
		path := strings.ToLower(op.Path)
		if kind != "add" && kind != "replace" && kind != "remove" {
			return "", nil, ErrInvalid
		}
		if path == "" {
			if kind == "remove" {
				return "", nil, ErrInvalid
			}
			var fields map[string]json.RawMessage
			if json.Unmarshal(op.Value, &fields) != nil || len(fields) == 0 {
				return "", nil, ErrInvalid
			}
			nested := []DirectoryPatchOperation{}
			for key, value := range fields {
				nested = append(nested, DirectoryPatchOperation{Op: kind, Path: key, Value: value})
			}
			var err error
			name, members, err = applyDirectoryGroupPatch(name, members, nested)
			if err != nil {
				return "", nil, err
			}
			continue
		}
		if path == "displayname" {
			if kind == "remove" || string(op.Value) == "null" || json.Unmarshal(op.Value, &name) != nil {
				return "", nil, ErrInvalid
			}
			continue
		}
		if strings.HasPrefix(path, "members[value eq ") {
			if kind != "remove" || !strings.HasSuffix(op.Path, "]") {
				return "", nil, ErrInvalid
			}
			var target string
			raw := op.Path[len("members[value eq ") : len(op.Path)-1]
			if json.Unmarshal([]byte(raw), &target) != nil || target == "" {
				return "", nil, ErrInvalid
			}
			kept := []string{}
			for _, id := range members {
				if id != target {
					kept = append(kept, id)
				}
			}
			members = kept
			continue
		}
		if path != "members" {
			return "", nil, ErrInvalid
		}
		if kind == "remove" {
			members = []string{}
			continue
		}
		var values []struct {
			Value string `json:"value"`
			Type  string `json:"type"`
		}
		if json.Unmarshal(op.Value, &values) != nil || string(op.Value) == "null" {
			return "", nil, ErrInvalid
		}
		if kind == "replace" {
			members = []string{}
		}
		seen := map[string]bool{}
		for _, id := range members {
			seen[id] = true
		}
		for _, v := range values {
			if v.Value == "" || (v.Type != "" && !strings.EqualFold(v.Type, "User")) {
				return "", nil, fmt.Errorf("%w: only User members are supported", ErrInvalid)
			}
			if !seen[v.Value] {
				members = append(members, v.Value)
				seen[v.Value] = true
			}
		}
	}
	return name, members, nil
}

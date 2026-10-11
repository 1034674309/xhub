package iam

import (
	"fmt"
	"sort"
)

// DirectoryCandidate 表示企业组命中的团队；仅在连接所属组织内使用。
// MappingID 用于同优先级稳定排序，Priority 越小越优先；不承载权限授予。
type DirectoryCandidate struct {
	MappingID string `xorm:"'mapping_id'" json:"mapping_id"`
	TeamID    string `xorm:"'team_id'" json:"team_id"`
	Priority  int    `json:"priority"`
}

// DirectoryAssignment 保存有效团队与人工/自动来源；空团队允许人工固定为无团队。
// 供同步服务和用户管理共用，Mode 仅允许 auto/manual。
type DirectoryAssignment struct {
	Mode      string `json:"mode"`
	TeamID    string `json:"team_id"`
	MappingID string `json:"mapping_id"`
	Reason    string `json:"reason"`
}

// SelectDirectoryTeam 根据已持久化候选决定单团队，不修改输入或执行数据库写入。
// 参数 current 为现有来源，candidates 已经过组织/启用状态过滤，fallback 为有效默认团队，
// force 仅供管理员确认的自动重算使用；返回选择结果或非法模式/候选错误。
// 人工覆盖永不被 force 覆盖；正常同步优先保留仍有效的团队，避免组顺序造成计费抖动。
func SelectDirectoryTeam(current DirectoryAssignment, candidates []DirectoryCandidate, fallback string, force bool) (DirectoryAssignment, error) {
	if current.Mode != "" && current.Mode != "auto" && current.Mode != "manual" {
		return DirectoryAssignment{}, fmt.Errorf("%w: invalid assignment mode", ErrInvalid)
	}
	if current.Mode == "manual" {
		current.MappingID = ""
		current.Reason = "manual_override"
		return current, nil
	}
	ordered := append([]DirectoryCandidate(nil), candidates...)
	for _, c := range ordered {
		if c.MappingID == "" || c.TeamID == "" || c.Priority < 0 {
			return DirectoryAssignment{}, fmt.Errorf("%w: invalid directory candidate", ErrInvalid)
		}
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Priority != ordered[j].Priority {
			return ordered[i].Priority < ordered[j].Priority
		}
		return ordered[i].MappingID < ordered[j].MappingID
	})
	if !force && current.TeamID != "" {
		for _, c := range ordered {
			if c.TeamID == current.TeamID {
				return DirectoryAssignment{Mode: "auto", TeamID: c.TeamID, MappingID: c.MappingID, Reason: "current_team_retained"}, nil
			}
		}
		if len(ordered) == 0 && current.TeamID == fallback {
			return DirectoryAssignment{Mode: "auto", TeamID: fallback, Reason: "default_team"}, nil
		}
	}
	if len(ordered) > 0 {
		c := ordered[0]
		return DirectoryAssignment{Mode: "auto", TeamID: c.TeamID, MappingID: c.MappingID, Reason: "mapping_priority"}, nil
	}
	if fallback != "" {
		return DirectoryAssignment{Mode: "auto", TeamID: fallback, Reason: "default_team"}, nil
	}
	return DirectoryAssignment{Mode: "auto", Reason: "no_matching_team"}, nil
}

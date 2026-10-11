package iam

import (
	"errors"
	"reflect"
	"testing"
)

// TestSelectDirectoryTeam 验证纯分配规则；前置有效/非法候选及当前来源，
// 覆盖首次、保留、人工无团队、重算、默认和失败；无外部数据，无需清理。
func TestSelectDirectoryTeam(t *testing.T) {
	candidates := []DirectoryCandidate{{MappingID: "office", TeamID: "beijing", Priority: 2}, {MappingID: "department", TeamID: "engineering", Priority: 1}}
	cases := []struct {
		name         string
		current      DirectoryAssignment
		fallback     string
		force        bool
		team, reason string
	}{
		{name: "首次按优先级", team: "engineering", reason: "mapping_priority"},
		{name: "现有团队保持", current: DirectoryAssignment{Mode: "auto", TeamID: "beijing"}, team: "beijing", reason: "current_team_retained"},
		{name: "人工覆盖优先", current: DirectoryAssignment{Mode: "manual", TeamID: "manual"}, force: true, team: "manual", reason: "manual_override"},
		{name: "人工固定无团队", current: DirectoryAssignment{Mode: "manual"}, team: "", reason: "manual_override"},
		{name: "管理员重算", current: DirectoryAssignment{Mode: "auto", TeamID: "beijing"}, force: true, team: "engineering", reason: "mapping_priority"},
		{name: "旧团队失效", current: DirectoryAssignment{Mode: "auto", TeamID: "gone"}, team: "engineering", reason: "mapping_priority"},
	}
	original := append([]DirectoryCandidate(nil), candidates...)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SelectDirectoryTeam(tc.current, candidates, tc.fallback, tc.force)
			if err != nil || got.TeamID != tc.team || got.Reason != tc.reason {
				t.Fatalf("团队选择错误: %+v %v", got, err)
			}
		})
	}
	if !reflect.DeepEqual(candidates, original) {
		t.Fatal("选择器修改输入候选")
	}
	for _, fallback := range []string{"", "default"} {
		got, err := SelectDirectoryTeam(DirectoryAssignment{}, nil, fallback, false)
		if err != nil || got.TeamID != fallback {
			t.Fatalf("空候选默认错误: %+v %v", got, err)
		}
	}
	reversed := []DirectoryCandidate{candidates[1], candidates[0]}
	a, _ := SelectDirectoryTeam(DirectoryAssignment{}, candidates, "", false)
	b, _ := SelectDirectoryTeam(DirectoryAssignment{}, reversed, "", false)
	if a != b {
		t.Fatal("候选顺序影响分配")
	}
	tie := []DirectoryCandidate{{MappingID: "z", TeamID: "z"}, {MappingID: "a", TeamID: "a"}}
	got, _ := SelectDirectoryTeam(DirectoryAssignment{}, tie, "", false)
	if got.TeamID != "a" {
		t.Fatal("同优先级没有固定排序")
	}
	invalid := []DirectoryCandidate{{MappingID: "x", TeamID: "y", Priority: -1}, {TeamID: "x"}, {MappingID: "x"}}
	for _, c := range invalid {
		if _, err := SelectDirectoryTeam(DirectoryAssignment{}, []DirectoryCandidate{c}, "", false); !errors.Is(err, ErrInvalid) {
			t.Fatalf("非法候选未拒绝: %+v %v", c, err)
		}
	}
	if _, err := SelectDirectoryTeam(DirectoryAssignment{Mode: "invalid"}, nil, "", false); !errors.Is(err, ErrInvalid) {
		t.Fatal("非法模式未拒绝")
	}
}

package iam

import (
	"encoding/json"
	"testing"
)

// TestDirectoryPatch 验证纯补丁计算的正常、边界和失败输入，无数据库前置或清理需求。
func TestDirectoryPatch(t *testing.T) {
	for _, tc := range []struct {
		path, value, op string
		valid           bool
	}{
		{"active", "false", "replace", true}, {"userName", `"member@example.com"`, "add", true}, {"displayName", `"Member"`, "replace", true},
		{"", `{"active":false,"displayName":"Updated"}`, "replace", true},
		{"active", "null", "replace", false}, {"active", `"false"`, "replace", false}, {"id", `"x"`, "replace", false},
		{"externalId", `"x"`, "replace", false}, {"role", `"admin"`, "add", false}, {"active", "true", "unknown", false}, {"userName", "", "remove", false},
	} {
		row := DirectoryUserResource{}
		err := applyDirectoryUserPatch(&row, []DirectoryPatchOperation{{Op: tc.op, Path: tc.path, Value: json.RawMessage(tc.value)}})
		if (err == nil) != tc.valid {
			t.Errorf("用户补丁 %s %s %s: %v", tc.op, tc.path, tc.value, err)
		}
	}
	if err := applyDirectoryUserPatch(new(DirectoryUserResource), nil); err == nil {
		t.Fatal("空补丁未拒绝")
	}
	name, members, err := applyDirectoryGroupPatch("Group", []string{"A"}, []DirectoryPatchOperation{
		{Op: "add", Path: "members", Value: json.RawMessage(`[{"value":"B","type":"User"},{"value":"B"}]`)},
		{Op: "remove", Path: `members[value eq "A"]`},
		{Op: "replace", Path: "displayName", Value: json.RawMessage(`"Updated"`)},
	})
	if err != nil || name != "Updated" || len(members) != 1 || members[0] != "B" {
		t.Fatalf("成员集计算错误 %s %v %v", name, members, err)
	}
	for _, ops := range [][]DirectoryPatchOperation{nil, {{Op: "add", Path: "members", Value: json.RawMessage(`[{"value":"B","type":"Group"}]`)}}, {{Op: "remove", Path: `members[value co "A"]`}}, {{Op: "replace", Path: "id", Value: json.RawMessage(`"x"`)}}} {
		if _, _, err := applyDirectoryGroupPatch("Group", nil, ops); err == nil {
			t.Fatalf("非法组补丁未拒绝 %+v", ops)
		}
	}
}

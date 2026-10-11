package contract

import (
	"reflect"
	"testing"
)

// TestContractHelpers 验证型号前缀、任务字段与路径展开的正常、空和失败输入；参数 t 为单测上下文。
// 前置纯值数据，返回无；不访问网络或注册表，无资源需清理。
func TestContractHelpers(t *testing.T) {
	for _, tc := range []struct{ prefix, stored, want string }{{"qiniu", "qiniu/org/model", "org/model"}, {"qiniu", "other/model", "other/model"}, {"", " org/model ", "org/model"}, {"qiniu", "", ""}, {"qiniu", "qiniu/qiniu/model", "qiniu/model"}} {
		if got := OfficialID(tc.prefix, tc.stored); got != tc.want {
			t.Fatalf("型号前缀: got=%q want=%q", got, tc.want)
		}
	}
	for _, tc := range []struct {
		doc        map[string]any
		path, want string
	}{{nil, "id", ""}, {map[string]any{"id": 3}, "id", ""}, {map[string]any{"data": "invalid"}, "data.id", ""}, {map[string]any{"data": map[string]any{"id": " task "}}, "data.id", "task"}, {map[string]any{}, "", ""}} {
		if got := ReadTaskID(tc.doc, tc.path); got != tc.want {
			t.Fatalf("任务字段: got=%q want=%q", got, tc.want)
		}
	}
	names := map[string]string{"id": "task", "model": "org/model"}
	before := map[string]string{"id": "task", "model": "org/model"}
	if got := Expand("/models/{model}/tasks/{id}", names); got != "/models/org/model/tasks/task" || !reflect.DeepEqual(names, before) {
		t.Fatalf("路径展开或输入隔离错误: %q", got)
	}
	if got := Expand("/tasks/{id}", nil); got != "/tasks/{id}" {
		t.Fatalf("未填参数被错误替换: %q", got)
	}
}

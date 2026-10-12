// Package prefs builds the router-settings page, the callback view, and the general-settings list. The numbers come from the merged document.
package prefs

import (
	"net/http"

	"github.com/sunqirui1987/xhub/internal/util"
)

// Callbacks returns general callback and cache configuration; routing policy belongs to templates.
// 参数 s（Host）：Callbacks使用的数据面宿主；w（http.ResponseWriter）：调用方的 HTTP 响应，状态和正文写在这里；r（*http.Request）：入站 HTTP 请求。
// 返回：无。状态码和正文写进调用方的响应。
// 调用：gateway/prefs/mount.go
// 测试：无直接单测
func Callbacks(s Host, w http.ResponseWriter, r *http.Request) {
	if s.RequireManage(w, r) == nil {
		return
	}
	cbs := []any{}
	for _, kind := range []string{"callback", "callbacks"} {
		list, _ := s.RecordStore().ListKV(kind)
		for _, row := range list {
			name := str(row["callback_name"])
			if name == "" {
				name = str(row["name"])
			}
			if name == "" {
				name = str(row["id"])
			}
			if name != "" {
				cbs = append(cbs, name)
			} else {
				cbs = append(cbs, row)
			}
		}
	}
	util.WriteJSON(w, 200, map[string]any{
		"status":                       "success",
		"callbacks":                    cbs,
		"success_callbacks":            cbs,
		"failure_callbacks":            []any{},
		"available_callbacks":          map[string]any{},
		"alerts":                       []any{},
		"active_alerting_destinations": []any{},
	})
}

// List returns settings for the requested config_type. general_settings includes stored_in_db.
// 参数 s（Host）：列出使用的数据面宿主；w（http.ResponseWriter）：调用方的 HTTP 响应，状态和正文写在这里；r（*http.Request）：入站 HTTP 请求。
// 返回：无。状态码和正文写进调用方的响应。
// 调用：gateway/keys/generate.go、gateway/keys/mount.go、gateway/models/list.go、gateway/models/mount.go
// 测试：无直接单测
func List(s Host, w http.ResponseWriter, r *http.Request) {
	if s.RequireManage(w, r) == nil {
		return
	}
	if r.URL.Query().Get("config_type") == "general_settings" || r.URL.Query().Get("config_type") == "" {
		util.WriteJSON(w, 200, GeneralList(s))
		return
	}
	util.WriteJSON(w, 200, []map[string]any{
		{
			"field_name": "mcp_internal_ip_ranges", "field_type": "List", "field_value": []any{},
			"field_description": "Internal IP ranges treated as private for MCP", "stored_in_db": false,
			"field_default_value": []any{},
		},
		{
			"field_name": "allow_requests_on_db_unavailable", "field_type": "Boolean", "field_value": false,
			"field_description": "Allow requests when the database is unavailable", "stored_in_db": false,
			"field_default_value": false,
		},
	})
}

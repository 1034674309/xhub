package gateway

import (
	"github.com/sunqirui1987/xhub/internal/iam"
	"net/http"
	"strings"
)

const scimGroupSchema = "urn:ietf:params:scim:schemas:core:2.0:Group"

// scimGroup 定义仅包含用户成员的标准企业组，不将所有组变成本地团队。
type scimGroup struct {
	Schemas     []string     `json:"schemas"`
	ID          string       `json:"id,omitempty"`
	ExternalID  string       `json:"externalId"`
	DisplayName string       `json:"displayName"`
	Members     []scimMember `json:"members"`
}

// scimMember 的 value 引用同连接用户资源，type 非 User 时拒绝嵌套组。
type scimMember struct {
	Value string `json:"value"`
	Type  string `json:"type,omitempty"`
}

// scimGroupPublic 转换目录组快照；参数为组与成员，返回 SCIM 资源，无副作用。
func scimGroupPublic(in iam.DirectoryGroupResource) scimGroup {
	out := scimGroup{Schemas: []string{scimGroupSchema}, ID: in.ID, ExternalID: in.ExternalID, DisplayName: in.Name, Members: []scimMember{}}
	for _, id := range in.Members {
		out.Members = append(out.Members, scimMember{Value: id, Type: "User"})
	}
	return out
}

// scimGroups 处理标准组列表、创建、替换和删除；参数是请求响应，无返回。
// 仅连接凭据可调用，成员校验与团队重算在同一事务；拒绝跨连接和嵌套组。
func (s *Server) scimGroups(w http.ResponseWriter, r *http.Request) {
	connectionID := r.PathValue("connection_id")
	db := s.Identity()
	auth := strings.Fields(r.Header.Get("Authorization"))
	if len(auth) != 2 || !strings.EqualFold(auth[0], "Bearer") {
		scimError(w, 401, "", "Invalid directory credential")
		return
	}
	if _, err := db.AuthenticateDirectory(r.Context(), connectionID, auth[1]); err != nil {
		scimError(w, 401, "", "Invalid directory credential")
		return
	}
	id := r.PathValue("id")
	if r.Method == http.MethodGet && id == "" {
		// 共用分页解析器，将唯一不同的公开精确字段转换为内部解析字段。
		clone := r.Clone(r.Context())
		urlCopy := *r.URL
		clone.URL = &urlCopy
		q := clone.URL.Query()
		filter := q.Get("filter")
		if strings.HasPrefix(filter, "userName") {
			scimError(w, 400, "invalidFilter", "Unsupported group filter")
			return
		}
		if strings.HasPrefix(filter, "displayName eq ") {
			q.Set("filter", "userName"+strings.TrimPrefix(filter, "displayName"))
			clone.URL.RawQuery = q.Encode()
		}
		start, count, field, value, err := scimPaging(clone)
		if err != nil {
			scimError(w, 400, "invalidFilter", "Unsupported filter or invalid pagination")
			return
		}
		if field == "userName" {
			field = "displayName"
		}
		rows, total, err := db.DirectoryGroupsPage(r.Context(), connectionID, field, value, start-1, count)
		if err != nil {
			scimStoreError(w, err)
			return
		}
		out := []scimGroup{}
		for _, row := range rows {
			resource, err := db.DirectoryGroupByID(r.Context(), connectionID, row.ID)
			if err != nil {
				scimStoreError(w, err)
				return
			}
			out = append(out, scimGroupPublic(*resource))
		}
		scimWrite(w, 200, map[string]any{"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:ListResponse"}, "totalResults": total, "startIndex": start, "itemsPerPage": len(out), "Resources": out})
		return
	}
	var existing *iam.DirectoryGroupResource
	if id != "" {
		var err error
		existing, err = db.DirectoryGroupByID(r.Context(), connectionID, id)
		if err != nil {
			scimStoreError(w, err)
			return
		}
		if r.Method == http.MethodGet {
			scimWrite(w, 200, scimGroupPublic(*existing))
			return
		}
		if r.Method == http.MethodDelete {
			if err := db.DeleteDirectoryGroup(r.Context(), connectionID, id); err != nil {
				scimStoreError(w, err)
				return
			}
			scimWrite(w, 204, nil)
			return
		}
	}
	var in scimGroup
	if !directoryDecode(r, &in) {
		scimError(w, 400, "invalidSyntax", "Invalid JSON")
		return
	}
	valid := false
	for _, schema := range in.Schemas {
		if schema == scimGroupSchema {
			valid = true
		}
	}
	if !valid || (in.ID != "" && in.ID != id) || (existing != nil && existing.ExternalID != in.ExternalID) {
		scimError(w, 400, "invalidValue", "Invalid schema or immutable identity")
		return
	}
	members := []string{}
	for _, m := range in.Members {
		if m.Value == "" || (m.Type != "" && m.Type != "User") {
			scimError(w, 400, "invalidValue", "Only user members are supported")
			return
		}
		members = append(members, m.Value)
	}
	g, err := db.ReplaceDirectoryGroup(r.Context(), connectionID, in.ExternalID, in.DisplayName, members)
	if err != nil {
		scimStoreError(w, err)
		return
	}
	// 写入响应使用本次提交的快照，避免并发重读后返回另一请求的内容。
	in.ID = g.ID
	in.Schemas = []string{scimGroupSchema}
	if in.Members == nil {
		in.Members = []scimMember{}
	}
	code := 200
	if r.Method == http.MethodPost {
		code = 201
	}
	w.Header().Set("Location", "/scim/v2/connections/"+connectionID+"/Groups/"+g.ID)
	scimWrite(w, code, in)
}

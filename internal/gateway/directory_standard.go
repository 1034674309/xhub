package gateway

import (
	"github.com/sunqirui1987/xhub/internal/iam"
	"net/http"
	"strings"
)

// scimAuthenticate 校验请求的连接范围凭据；参数为响应与请求，返回可否继续，不修改数据。
// 标准发现与 PATCH 共用；不暴露连接存在性，失败写 SCIM 401。
func (s *Server) scimAuthenticate(w http.ResponseWriter, r *http.Request) bool {
	a := strings.Fields(r.Header.Get("Authorization"))
	if len(a) != 2 || !strings.EqualFold(a[0], "Bearer") {
		scimError(w, 401, "", "Invalid directory credential")
		return false
	}
	if _, err := s.Identity().AuthenticateDirectory(r.Context(), r.PathValue("connection_id"), a[1]); err != nil {
		scimError(w, 401, "", "Invalid directory credential")
		return false
	}
	return true
}

// scimPatch 处理标准 PatchOp 请求；参数为 HTTP 请求响应，操作在一个数据库事务内执行。
// 仅已认证连接可调用，拒绝权限属性、跨连接资源与未知操作，失败不部分写入。
func (s *Server) scimPatch(w http.ResponseWriter, r *http.Request) {
	if !s.scimAuthenticate(w, r) {
		return
	}
	var in struct {
		Schemas    []string                      `json:"schemas"`
		Operations []iam.DirectoryPatchOperation `json:"Operations"`
	}
	if !directoryDecode(r, &in) || len(in.Schemas) != 1 || in.Schemas[0] != "urn:ietf:params:scim:api:messages:2.0:PatchOp" {
		scimError(w, 400, "invalidSyntax", "Invalid PatchOp request")
		return
	}
	c, id := r.PathValue("connection_id"), r.PathValue("id")
	if strings.Contains(r.URL.Path, "/Groups/") {
		if _, err := s.Identity().PatchDirectoryGroup(r.Context(), c, id, in.Operations); err != nil {
			scimStoreError(w, err)
			return
		}
		row, err := s.Identity().DirectoryGroupByID(r.Context(), c, id)
		if err != nil {
			scimStoreError(w, err)
			return
		}
		scimWrite(w, 200, scimGroupPublic(*row))
		return
	}
	if _, err := s.Identity().PatchDirectoryUser(r.Context(), c, id, in.Operations); err != nil {
		scimStoreError(w, err)
		return
	}
	row, err := s.Identity().DirectoryUserByID(r.Context(), c, id)
	if err != nil {
		scimStoreError(w, err)
		return
	}
	scimWrite(w, 200, scimPublic(*row))
}

// scimAttribute 创建标准属性声明；参数描述类型与约束，返回 schema 属性，无副作用。
func scimAttribute(name, typ, mutability string, required, multi bool) map[string]any {
	return map[string]any{"name": name, "type": typ, "multiValued": multi, "description": name, "required": required, "caseExact": false, "mutability": mutability, "returned": "default", "uniqueness": "none"}
}

// scimDiscovery 返回 Schemas 与 ResourceTypes 列表或详情；只描述当前实现的属性。
// 参数为 HTTP 请求响应，认证后读取静态协议信息，未知 id 返回标准 404，无数据库写入。
func (s *Server) scimDiscovery(w http.ResponseWriter, r *http.Request) {
	if !s.scimAuthenticate(w, r) {
		return
	}
	schema := strings.Contains(r.URL.Path, "/Schemas")
	out := []map[string]any{}
	for _, kind := range []string{"User", "Group"} {
		urn := scimUserSchema
		endpoint := "/Users"
		if kind == "Group" {
			urn = scimGroupSchema
			endpoint = "/Groups"
		}
		if !schema {
			out = append(out, map[string]any{"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:ResourceType"}, "id": kind, "name": kind, "endpoint": endpoint, "schema": urn, "schemaExtensions": []any{}})
			continue
		}
		attrs := []map[string]any{scimAttribute("id", "string", "readOnly", false, false), scimAttribute("externalId", "string", "immutable", true, false), scimAttribute("displayName", "string", "readWrite", kind == "Group", false)}
		if kind == "User" {
			u := scimAttribute("userName", "string", "readWrite", true, false)
			u["uniqueness"] = "server"
			attrs = append(attrs, u, scimAttribute("active", "boolean", "readWrite", false, false))
		} else {
			m := scimAttribute("members", "complex", "readWrite", false, true)
			m["subAttributes"] = []map[string]any{scimAttribute("value", "string", "readWrite", true, false), scimAttribute("type", "string", "readWrite", false, false)}
			attrs = append(attrs, m)
		}
		out = append(out, map[string]any{"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:Schema"}, "id": urn, "name": kind, "description": "XHub directory " + kind, "attributes": attrs})
	}
	if id := r.PathValue("id"); id != "" {
		for _, row := range out {
			if row["id"] == id {
				scimWrite(w, 200, row)
				return
			}
		}
		scimError(w, 404, "", "Resource not found")
		return
	}
	scimWrite(w, 200, map[string]any{"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:ListResponse"}, "totalResults": len(out), "startIndex": 1, "itemsPerPage": len(out), "Resources": out})
}

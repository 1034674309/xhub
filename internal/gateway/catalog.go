// catalog.go loads routes.json and answers public catalog reads. Matching and
// the price map live in internal/catalog. This file chooses which handler
// runs for a catalog path that ingress.go did not give to a family.

package gateway

import (
	"net/http"
	"strings"

	"github.com/sunqirui1987/xhub/internal/catalog"
	"github.com/sunqirui1987/xhub/internal/gateway/family"
	"github.com/sunqirui1987/xhub/internal/util"
)

// serveFamilyRoute handles a catalog route that was registered on Gin by path and has no earlier dedicated handler. It is not mounted on "/". An unregistered path gets a 404 from the engine NoRoute.
// 参数 w（http.ResponseWriter）：调用方的 HTTP 响应，状态码和正文写在这里；r（*http.Request）：入站 HTTP 请求，用来读路径、头和正文。
// 调用：gateway/ingress.go
// 测试：无直接单测
// 返回：无。状态码和正文写进调用方的响应。
func (s *Server) serveFamilyRoute(w http.ResponseWriter, r *http.Request) {
	if !s.matchCatalog(r.Method, r.URL.Path) {
		util.Debug("process %s %s step=catalog match=false", r.Method, r.URL.Path)
		util.WriteError(w, 404, "not_found", "Not Found")
		return
	}
	util.SetCallID(w, util.CallID())
	if catalog.IsPublicPath(r.Method, r.URL.Path) {
		util.Debug("process %s %s step=catalog class=public", r.Method, r.URL.Path)
		util.WriteJSON(w, 200, catalog.PublicBody(r.URL.Path))
		return
	}
	class := "manage"
	switch catalog.AuthOf(r.URL.Path) {
	case catalog.AuthData:
		class = "data"
	case catalog.AuthMixed:
		class = "mixed"
	}
	util.Debug("process %s %s step=catalog class=%s", r.Method, r.URL.Path, class)
	switch class {
	case "data":
		family.ServeDataPlane(s, w, r)
	case "mixed":
		family.ServeMixed(s, w, r)
	default:
		family.ServeMgmt(s, w, r)
	}
}

// matchCatalog reports whether this request hits one embedded catalog route. A mixed prefix counts as a match even when it is not in the list.
// 参数 method（string）：HTTP 方法，例如 GET 或 POST；path（string）：匹配目录要定位的路径。可能是 URL，也可能是字段路径。
// 返回 bool（bool）：请求命中一条内嵌目录路由时返回真。混合前缀即使不在清单里也算命中。
// 调用：仅在 catalog.go 内使用
// 测试：无直接单测
func (s *Server) matchCatalog(method, path string) bool {
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		path = "/"
	}
	for _, rt := range s.catalog {
		if !strings.EqualFold(rt.M, method) {
			continue
		}
		if catalog.PathMatch(rt.P, path) {
			return true
		}
	}
	// The OpenAPI list does not always include a child route such as /v1/mcp/server.
	return catalog.IsMixedPath(path)
}

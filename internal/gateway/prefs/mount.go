// Package prefs registers settings routes. The rule that a database key overrides YAML lives in the handlers, not in this registration.
package prefs

import (
	"net/http"
	"sync"

	"github.com/sunqirui1987/xhub/internal/util"
)

var logTraceOnceMount sync.Once

// Module reads and writes router settings and general settings.
// 参数 h（Host）：实现这一步所需能力的数据面宿主。聊天、直通和刷写各自只依赖自己的方法。
// 返回 util.Module（util.Module）：可挂到网关上的模块。
// 调用：gateway/family/mount.go、gateway/guard/mount.go、gateway/identity/mount.go、gateway/keys/mount.go
// 测试：无直接单测
func Module(h Host) util.Module {
	logTraceOnceMount.Do(func() { util.Trace("enter prefs.Module") })

	traceModule("prefs")
	return util.Bind("prefs", func(reg util.Registrar) {
		reg.Handle("GET /router/settings", func(w http.ResponseWriter, r *http.Request) { Page(h, w, r) })
		reg.Handle("GET /router/fields", func(w http.ResponseWriter, r *http.Request) { Page(h, w, r) })
		reg.Handle("GET /get/config/callbacks", func(w http.ResponseWriter, r *http.Request) { Callbacks(h, w, r) })
		reg.Handle("GET /config/list", func(w http.ResponseWriter, r *http.Request) { List(h, w, r) })
		reg.Handle("POST /config/update", func(w http.ResponseWriter, r *http.Request) { Update(h, w, r) })
		reg.Handle("POST /config/field/update", func(w http.ResponseWriter, r *http.Request) { FieldUpdate(h, w, r) })
		reg.Handle("POST /config/field/delete", func(w http.ResponseWriter, r *http.Request) { FieldDelete(h, w, r) })
	})
}

package contract

import (
	"net/http"
	"time"

	"github.com/sunqirui1987/xhub/internal/auth"
	"github.com/sunqirui1987/xhub/internal/config"
)

// BudgetHost 是生产网关在每次实际 HTTP 尝试前实施持久费用预占的扩展接口。
// 旧测试宿主可以只测试协议能力；生产 Server 必须实现此接口，两条推理执行器均调用。
type BudgetHost interface {
	// PrepareBudgetAttempt 校验最终出站 JSON、写入 cap 并原子预占；失败已写错误，不得外发。
	// 参数为响应、原请求、主体、请求 ID、实际部署、协议、正文、开始时间；返回修改后的正文及是否继续。
	PrepareBudgetAttempt(http.ResponseWriter, *http.Request, *auth.Principal, string, config.ModelEntry, string, []byte, time.Time) ([]byte, bool)
	// MarkBudgetAttemptSubmitted 在真正发送前持久标记已提交；失败已写错误，不得外发。
	// 请求取消且尚未提交时由 ReleaseCall 释放；提交后的未知费用必须保留，不按 TTL 退款。
	MarkBudgetAttemptSubmitted(http.ResponseWriter, *http.Request, string) bool
}

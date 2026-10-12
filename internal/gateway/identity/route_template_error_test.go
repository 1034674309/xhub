package identity

import (
	"github.com/sunqirui1987/xhub/internal/iam"
	"net/http/httptest"
	"testing"
	"xorm.io/xorm"
)

// templateErrorGate 仅替换解析器所用身份库；其余 Gate 方法不应在测试中调用。
type templateErrorGate struct {
	Gate
	db *iam.DB
}

// Identity 返回不可连接的测试数据库；无参数，无持久化副作用。
func (g templateErrorGate) Identity() *iam.DB { return g.db }

// TestTemplateResolutionDatabaseError 验证控制台不会将读取错误伪装成内置模板。
// 前置不可连接的本地数据库；应直接返回错误；测试结束关闭 engine，不产生数据。
func TestTemplateResolutionDatabaseError(t *testing.T) {
	engine, err := xorm.NewEngine("pgx", "postgres://127.0.0.1:1/unavailable")
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	_, err = resolveRouteTemplate(templateErrorGate{db: &iam.DB{Engine: engine}}, httptest.NewRequest("GET", "/route-template/binding", nil), "organization", "org")
	if err == nil {
		t.Fatal("数据库读取失败被吞掉")
	}
}

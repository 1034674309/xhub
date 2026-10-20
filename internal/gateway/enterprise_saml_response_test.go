package gateway

import (
	"github.com/crewjam/saml"
	"testing"
	"time"
)

// TestEnterpriseSAMLAssertionBounds 使用已验签断言的结构夹具验证浏览器必需字段；无网络或数据库。
// 正常认证通过；缺失/未来时间、无 bearer 和请求关联错误拒绝，纯内存无需清理。
func TestEnterpriseSAMLAssertionBounds(t *testing.T) {
	now := time.Now().UTC()
	for _, mode := range []string{"valid", "nil", "subject", "conditions", "confirmation", "data", "method", "request", "recipient", "expiry", "future", "authn", "authnFuture", "id", "version"} {
		t.Run(mode, func(t *testing.T) {
			a := &saml.Assertion{ID: "assertion", Version: "2.0", IssueInstant: now, Subject: &saml.Subject{SubjectConfirmations: []saml.SubjectConfirmation{{Method: "urn:oasis:names:tc:SAML:2.0:cm:bearer", SubjectConfirmationData: &saml.SubjectConfirmationData{InResponseTo: "request", Recipient: "https://console.example/acs", NotOnOrAfter: now.Add(time.Minute)}}}}, Conditions: &saml.Conditions{NotOnOrAfter: now.Add(time.Minute)}, AuthnStatements: []saml.AuthnStatement{{AuthnInstant: now}}}
			switch mode {
			case "nil":
				a = nil
			case "subject":
				a.Subject = nil
			case "conditions":
				a.Conditions = nil
			case "confirmation":
				a.Subject.SubjectConfirmations = nil
			case "data":
				a.Subject.SubjectConfirmations[0].SubjectConfirmationData = nil
			case "method":
				a.Subject.SubjectConfirmations[0].Method = "other"
			case "request":
				a.Subject.SubjectConfirmations[0].SubjectConfirmationData.InResponseTo = "other"
			case "recipient":
				a.Subject.SubjectConfirmations[0].SubjectConfirmationData.Recipient = "https://other.example/acs"
			case "expiry":
				a.Subject.SubjectConfirmations[0].SubjectConfirmationData.NotOnOrAfter = time.Time{}
			case "future":
				a.IssueInstant = now.Add(time.Hour)
			case "authn":
				a.AuthnStatements = nil
			case "authnFuture":
				a.AuthnStatements[0].AuthnInstant = now.Add(time.Hour)
			case "id":
				a.ID = ""
			case "version":
				a.Version = "1.0"
			}
			err := enterpriseSAMLAssertionBounds(a, "request", "https://console.example/acs", now)
			if (err == nil) != (mode == "valid") {
				t.Fatalf("浏览器断言边界 %s: %v", mode, err)
			}
		})
	}
}

// TestEnterpriseSAMLResponseMalformed 验证 XML 包装器失败关闭；无服务端事务或外部依赖。
// 空提供方、空请求标识和畸形响应必须返回错误且无断言，不传播 panic，纯内存无需清理。
func TestEnterpriseSAMLResponseMalformed(t *testing.T) {
	for _, raw := range [][]byte{nil, []byte("<invalid>"), []byte("not XML")} {
		if a, err := enterpriseSAMLResponse(nil, raw, "request"); err == nil || a != nil {
			t.Fatal("空provider被接受")
		}
		if a, err := enterpriseSAMLResponse(&saml.ServiceProvider{}, raw, "request"); err == nil || a != nil {
			t.Fatal("无效响应被接受")
		}
	}
}

// TestEnterpriseSAMLCookie 验证 HTTP 联调和 HTTPS Cookie 属性隔离；配置决定安全属性，避免客户端伪造转发头降级。
// 正常/删除年龄及浏览器绑定属性均精确断言，无外部资源需要清理。
func TestEnterpriseSAMLCookie(t *testing.T) {
	for _, secure := range []bool{true, false} {
		c, _, _ := enterpriseSAMLTestMaterial(t)
		if !secure {
			c.RedirectURI = "http://console.example/auth/enterprise/saml/acs"
			c.AllowHTTP = true
		}
		cookie := enterpriseSAMLCookie(&c, "state", "browser", 300)
		if cookie.Secure != secure || !cookie.HttpOnly || cookie.Value != "browser" || cookie.Path != "/" || cookie.MaxAge != 300 {
			t.Fatal("SAML Cookie属性错误")
		}
		if enterpriseSAMLCookie(&c, "state", "", -1).MaxAge != -1 {
			t.Fatal("Cookie未删除")
		}
	}
}

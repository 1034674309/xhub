package gateway

import (
	"errors"
	"time"

	"github.com/crewjam/saml"
)

// enterpriseSAMLResponse 通过库验签和解密，再收紧浏览器登录断言必需字段；raw 为有限大小响应，requestID 来自服务端事务。
// ACS 调用，返回经过签名和请求关联验证的断言或固定错误；库对缺失 Subject/Conditions 的 panic 也必须失败关闭。
func enterpriseSAMLResponse(sp *saml.ServiceProvider, raw []byte, requestID string) (assertion *saml.Assertion, err error) {
	defer func() {
		if recover() != nil {
			assertion = nil
			err = errors.New("invalid SAML response")
		}
	}()
	if sp == nil || len(raw) == 0 || len(raw) > 2<<20 || requestID == "" {
		return nil, errors.New("invalid SAML response")
	}
	assertion, err = sp.ParseXMLResponse(raw, []string{requestID}, sp.AcsURL)
	if err != nil {
		return nil, err
	}
	if err = enterpriseSAMLAssertionBounds(assertion, requestID, sp.AcsURL.String(), time.Now()); err != nil {
		return nil, err
	}
	return assertion, nil
}

// enterpriseSAMLAssertionBounds 要求浏览器 SSO 的 bearer 主体、时效和认证语句；a 只能是库已验签的断言。
// response 包装器调用，返回错误或 nil；补足库对空 SubjectConfirmation 集合及未来签发时间的宽松处理，不替代密码学校验。
func enterpriseSAMLAssertionBounds(a *saml.Assertion, requestID, recipient string, now time.Time) error {
	invalid := errors.New("invalid SAML authentication assertion")
	if a == nil || a.ID == "" || a.Version != "2.0" || a.Subject == nil || a.Conditions == nil || len(a.Subject.SubjectConfirmations) == 0 || len(a.AuthnStatements) == 0 {
		return invalid
	}
	if a.IssueInstant.IsZero() || a.IssueInstant.After(now.Add(saml.MaxClockSkew)) || a.Conditions.NotOnOrAfter.IsZero() {
		return invalid
	}
	for _, confirmation := range a.Subject.SubjectConfirmations {
		data := confirmation.SubjectConfirmationData
		if data == nil || confirmation.Method != "urn:oasis:names:tc:SAML:2.0:cm:bearer" || data.InResponseTo != requestID || data.Recipient != recipient || data.NotOnOrAfter.IsZero() {
			return invalid
		}
	}
	for _, statement := range a.AuthnStatements {
		if statement.AuthnInstant.IsZero() || statement.AuthnInstant.After(now.Add(saml.MaxClockSkew)) {
			return invalid
		}
	}
	return nil
}

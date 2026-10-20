package samltest

import (
	"encoding/base64"
	"encoding/xml"
	"net/url"
	"testing"

	"github.com/crewjam/saml"
	dsig "github.com/russellhaering/goxmldsig"
)

// TestIDPSignedExchange 在临时 TLS IdP 和内存 SP 上验证夹具真正签名并能检出篡改，不依赖数据库。
// 覆盖稳定属性、未知身份、缺签名和非法元数据；t.Cleanup 关闭服务且无持久化数据。
func TestIDPSignedExchange(t *testing.T) {
	f := New(t)
	var metadata saml.EntityDescriptor
	if err := xml.Unmarshal([]byte(f.Metadata()), &metadata); err != nil {
		t.Fatal(err)
	}
	acs, _ := url.Parse("https://console.example.test/auth/enterprise/saml/acs")
	sp := &saml.ServiceProvider{EntityID: "urn:test:sp", Key: f.idp.Signer, Certificate: f.idp.Certificate, AcsURL: *acs, MetadataURL: *acs, IDPMetadata: &metadata, SignatureMethod: dsig.RSASHA256SignatureMethod}
	exported, _ := xml.Marshal(sp.Metadata())
	if err := f.Register(string(exported)); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"valid", "unknown", "tamper", "unsigned"} {
		t.Run(mode, func(t *testing.T) {
			req, err := sp.MakeAuthenticationRequest(f.Server.URL+"/sso", saml.HTTPRedirectBinding, saml.HTTPPostBinding)
			if err != nil {
				t.Fatal(err)
			}
			redirect, err := req.Redirect("random-browser-state", sp)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "unsigned" {
				q := redirect.Query()
				q.Del("Signature")
				redirect.RawQuery = q.Encode()
			}
			form, err := f.Response(redirect.String(), "employee-sub", mode)
			if mode == "unsigned" {
				if err == nil {
					t.Fatal("本地 IdP 不得接受未签名请求")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := base64.StdEncoding.DecodeString(form.SAMLResponse)
			assertion, err := sp.ParseXMLResponse(raw, []string{req.ID}, sp.AcsURL)
			if mode == "tamper" {
				if err == nil {
					t.Fatal("篡改夹具没有破坏实际签名")
				}
				return
			}
			if err != nil {
				t.Fatal("有效夹具断言验签失败", err)
			}
			expected := "employee-sub"
			if mode == "unknown" {
				expected = "unknown-local-employee"
			}
			if assertion.AttributeStatements[0].Attributes[0].Values[0].Value != expected {
				t.Fatal("夹具没有生成指定稳定身份")
			}
		})
	}
	t.Run("invalid-metadata", func(t *testing.T) {
		if err := f.Register("<invalid>"); err == nil {
			t.Fatal("非法 SP XML 被注册")
		}
		if _, err := f.GetServiceProvider(nil, "missing"); err == nil {
			t.Fatal("未知 SP 被接受")
		}
	})
}

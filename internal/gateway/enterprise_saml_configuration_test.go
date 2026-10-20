package gateway

import (
	"context"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sunqirui1987/xhub/internal/iam"
)

// TestEnterpriseSAMLMetadataFetch 验证元数据读取边界；前置本地 TLS 身份提供方且显式注入测试客户端。
// 覆盖粘贴、HTTPS获取、HTTP拒绝、错误状态、超大响应和取消；不写数据库，服务器由Cleanup关闭。
func TestEnterpriseSAMLMetadataFetch(t *testing.T) {
	fixture := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/good":
			_, _ = w.Write([]byte("metadata"))
		case "/large":
			_, _ = w.Write([]byte(strings.Repeat("x", (2<<20)+1)))
		default:
			w.WriteHeader(503)
		}
	}))
	t.Cleanup(fixture.Close)
	s := &Server{enterpriseClient: fixture.Client()}
	if got, err := s.enterpriseSAMLMetadataXML(context.Background(), &iam.EnterpriseOIDC{IDPMetadata: "pasted"}); err != nil || got != "pasted" {
		t.Fatal("粘贴元数据读取失败", err)
	}
	for _, test := range []struct {
		path  string
		valid bool
	}{{fixture.URL + "/good", true}, {fixture.URL + "/large", false}, {fixture.URL + "/failed", false}, {"http://127.0.0.1/meta", false}} {
		got, err := s.enterpriseSAMLMetadataXML(context.Background(), &iam.EnterpriseOIDC{IDPMetadataURL: test.path})
		if (err == nil) != test.valid || test.valid && got != "metadata" {
			t.Fatalf("元数据获取边界 %s: %v", test.path, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.enterpriseSAMLMetadataXML(ctx, &iam.EnterpriseOIDC{IDPMetadataURL: fixture.URL + "/good"}); err == nil {
		t.Fatal("取消未终止元数据请求")
	}
}

// TestEnterpriseSAMLExpiredCertificate 检查配置页的可用证书提示；前置内存RSA材料，不依赖公司SSO。
// 当前证书不提示，过期/未来提示；畸形XML不误报到期，具体结构错误交给配置校验，纯内存无需清理。
func TestEnterpriseSAMLExpiredCertificate(t *testing.T) {
	_, key, current := enterpriseSAMLTestMaterial(t)
	now := time.Now()
	for _, test := range []struct {
		name, xml string
		expired   bool
	}{
		{"valid", enterpriseSAMLTestMetadata(current), false},
		{"expired", enterpriseSAMLTestMetadata(enterpriseSAMLTestCertificate(t, key, now.Add(-2*time.Hour), now.Add(-time.Hour), x509.KeyUsageDigitalSignature)), true},
		{"future", enterpriseSAMLTestMetadata(enterpriseSAMLTestCertificate(t, key, now.Add(time.Hour), now.Add(2*time.Hour), x509.KeyUsageDigitalSignature)), true},
		{"malformed", "<invalid>", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if enterpriseSAMLExpiredCertificate(test.xml) != test.expired {
				t.Fatal("元数据签名证书提示不准确")
			}
		})
	}
}

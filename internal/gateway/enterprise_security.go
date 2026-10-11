package gateway

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"time"
)

// enterpriseRandom 生成 256 位 URL 安全随机字符串；认证事务与一次性凭据调用，失败返回错误。
func enterpriseRandom() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b), err
}

// enterpriseSecret 使用 AES-GCM 加解密配置秘密；master 必须是受控且稳定的服务端密钥。
// value 是明文或密文，decrypt 指定方向；返回结果或错误，使用随机 nonce 与固定用途 AAD。
func enterpriseSecret(master, value string, decrypt bool) (string, error) {
	if len(master) < 32 {
		return "", errors.New("enterprise encryption key requires at least 32 characters")
	}
	key := sha256.Sum256([]byte(master))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	aad := []byte("xhub-enterprise-secret-v1")
	if decrypt {
		raw, err := base64.RawURLEncoding.DecodeString(value)
		if err != nil || len(raw) < g.NonceSize() {
			return "", errors.New("invalid encrypted secret")
		}
		plain, err := g.Open(nil, raw[:g.NonceSize()], raw[g.NonceSize():], aad)
		return string(plain), err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(g.Seal(nonce, nonce, []byte(value), aad)), nil
}

// enterpriseURL 校验远程认证地址；参数为 URL，返回解析地址或错误，不允许非 HTTPS、凭据、片段。
// SSRF 的实际 IP 校验由拨号时执行，避免只检查域名却遭 DNS 重绑定。
func enterpriseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return nil, errors.New("invalid enterprise HTTPS URL")
	}
	return u, nil
}

// enterprisePublicIP 判断地址是否为可访问公网；参数为解析地址，返回允许与否，无副作用。
func enterprisePublicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, p := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "64:ff9b::/96"} {
		if netip.MustParsePrefix(p).Contains(ip) {
			return false
		}
	}
	return true
}

// enterpriseTransport 对 OIDC 请求施加地址和响应限制；base 是带 DNS 钉扎的 transport。
type enterpriseTransport struct{ base http.RoundTripper }

// RoundTrip 校验远程 URL 并限制响应体；请求由 OIDC 库发起，返回响应或安全错误，无凭据日志。
func (t enterpriseTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if _, err := enterpriseURL(r.URL.String()); err != nil {
		return nil, err
	}
	resp, err := t.base.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	if resp.ContentLength > 2<<20 {
		resp.Body.Close()
		return nil, errors.New("enterprise response too large")
	}
	resp.Body = &enterpriseBody{ReadCloser: resp.Body, remaining: 2 << 20}
	return resp, nil
}

// enterpriseBody 在未知长度响应上强制上限；超过上限报错而不是静默截断。
type enterpriseBody struct {
	io.ReadCloser
	remaining int64
}

// Read 消耗有限响应字节；参数为调用方缓冲区，返回读取数或大小超限错误，Close 由调用方执行。
func (b *enterpriseBody) Read(p []byte) (int, error) {
	if b.remaining == 0 {
		var one [1]byte
		n, err := b.ReadCloser.Read(one[:])
		if n > 0 {
			return 0, errors.New("enterprise response too large")
		}
		return 0, err
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	n, err := b.ReadCloser.Read(p)
	b.remaining -= int64(n)
	return n, err
}

// enterpriseHTTPClient 创建限制公网 HTTPS 的 OIDC 客户端；无参数，返回带超时的客户端。
// 不使用环境代理；每次拨号解析并直接连接已检查 IP，重定向由 RoundTrip 再校验。
func enterpriseHTTPClient() *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = nil
	tr.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, errors.New("empty DNS result")
		}
		for _, ip := range ips {
			if !enterprisePublicIP(ip) {
				return nil, errors.New("enterprise endpoint is not public")
			}
		}
		var last error
		for _, ip := range ips {
			conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			last = err
		}
		return nil, last
	}
	return &http.Client{Transport: enterpriseTransport{base: tr}, Timeout: 15 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return errors.New("too many redirects")
		}
		_, err := enterpriseURL(r.URL.String())
		return err
	}}
}

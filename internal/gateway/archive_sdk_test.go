package gateway

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qiniu/go-sdk/v7/auth"
	qclient "github.com/qiniu/go-sdk/v7/client"
	"github.com/qiniu/go-sdk/v7/storage"
	"github.com/qiniu/go-sdk/v7/storagev2/uplog"
)

// TestKodoSDKProtocol 通过本地 TLS 服务验证官方 SDK 的上传、凭据策略、重试核验和删除。
// 前置为虚构 AK/SK 与内存对象，无真实七牛写入；测试结束关闭服务器并释放对象。
func TestKodoSDKProtocol(t *testing.T) {
	archiveSDKOnce.Do(uplog.DisableUplog)
	objects := map[string][]byte{}
	mimeTypes := map[string]string{}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Reqid", "local-protocol-fixture")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Error("上传不是标准 multipart", err)
				w.WriteHeader(400)
				return
			}
			defer r.MultipartForm.RemoveAll()
			key := r.FormValue("key")
			parts := strings.Split(r.FormValue("token"), ":")
			if len(parts) != 3 || parts[0] != "fake-ak" {
				t.Error("上传缺少限定凭据")
				w.WriteHeader(401)
				return
			}
			raw, err := base64.URLEncoding.DecodeString(parts[2])
			var policy storage.PutPolicy
			if err != nil || json.Unmarshal(raw, &policy) != nil || policy.Scope != "xhub-test:"+key || policy.InsertOnly != 1 {
				t.Error("凭据未限定对象或允许覆盖")
				w.WriteHeader(401)
				return
			}
			f, header, err := r.FormFile("file")
			if err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			defer f.Close()
			body, err := io.ReadAll(f)
			if err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			if _, found := objects[key]; found {
				w.WriteHeader(614)
				_, _ = w.Write([]byte(`{"error":"file exists"}`))
				return
			}
			objects[key] = body
			mimeTypes[key] = header.Header.Get("Content-Type")
			_ = json.NewEncoder(w).Encode(map[string]any{"key": key, "hash": kodoContentHash(body)})
		case strings.HasPrefix(r.URL.Path, "/stat/") || strings.HasPrefix(r.URL.Path, "/delete/"):
			valid, signatureErr := auth.New("fake-ak", "fake-sk").VerifyCallback(r)
			if !valid || signatureErr != nil {
				t.Error("管理接口缺少签名")
				w.WriteHeader(401)
				return
			}
			encoded := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			entry, _ := base64.URLEncoding.DecodeString(encoded)
			key := strings.TrimPrefix(string(entry), "xhub-test:")
			body, found := objects[key]
			if !found {
				w.WriteHeader(612)
				_, _ = w.Write([]byte(`{"error":"not found"}`))
				return
			}
			if strings.HasPrefix(r.URL.Path, "/delete/") {
				delete(objects, key)
				_, _ = w.Write([]byte("{}"))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"hash": kodoContentHash(body), "fsize": len(body)})
		default:
			t.Errorf("未知 SDK 请求 %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	host := strings.TrimPrefix(srv.URL, "https://")
	k := &kodoArchive{bucket: "xhub-test", mac: auth.New("fake-ak", "fake-sk"), cfg: &storage.Config{UseHTTPS: true, Region: &storage.Region{SrcUpHosts: []string{host}, RsHost: host}}, client: &qclient.Client{Client: srv.Client()}}
	ctx := context.Background()
	for key, body := range map[string][]byte{"data.jsonl.gz": []byte("gzip fixture"), "data.manifest.json": []byte("{}"), "probe": []byte("probe")} {
		if err := k.Put(ctx, key, body); err != nil {
			t.Fatalf("SDK 上传失败 %s: %v", key, err)
		}
		if err := k.Put(ctx, key, body); err != nil {
			t.Fatalf("同内容安全重试失败 %s: %v", key, err)
		}
		if err := k.Put(ctx, key, []byte("conflict")); err == nil {
			t.Fatal("不同内容被覆盖", key)
		}
	}
	if mimeTypes["data.jsonl.gz"] != "application/gzip" || mimeTypes["data.manifest.json"] != "application/json" || mimeTypes["probe"] != "application/octet-stream" {
		t.Fatal("对象 MIME 错误", mimeTypes)
	}
	if err := k.Delete(ctx, "probe"); err != nil {
		t.Fatal("探针清理失败", err)
	}
	if _, found := objects["probe"]; found {
		t.Fatal("探针残留")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := k.Delete(canceled, "data.jsonl.gz"); err == nil {
		t.Fatal("取消上下文未阻止删除")
	}
	if _, err := newKodoArchive(nil, ""); err == nil {
		t.Fatal("缺失配置未拒绝")
	}
}

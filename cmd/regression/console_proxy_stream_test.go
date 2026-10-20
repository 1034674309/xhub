package regression

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/sunqirui1987/xhub/internal/config"
)

// consoleProxyFixture 用仓库原始 Next 配置启动独立代理；参数为测试上下文和真实网关URL，返回本机代理URL。
// 只在显式 Next 回归运行；临时目录复制配置并链接已安装依赖，不修改工作树，退出回收进程和所有临时文件。
func consoleProxyFixture(t *testing.T, gateway string) string {
	t.Helper()
	frontend, err := filepath.Abs("../../frontend")
	if err != nil {
		t.Fatal(err)
	}
	modules, err := filepath.EvalSymlinks(filepath.Join(frontend, "node_modules"))
	if err != nil {
		t.Fatalf("Next代理回归需要已安装的前端依赖: %v", err)
	}
	dir := t.TempDir()
	configuration, err := os.ReadFile(filepath.Join(frontend, "next.config.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{"next.config.mjs": configuration, "package.json": []byte(`{"name":"xhub-proxy-regression","private":true}`), "app/page.js": []byte("export default function Page(){return null}"), "app/layout.js": []byte("export default function Layout({children}){return children}")} {
		if err = os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0700); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, name), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err = os.Symlink(modules, filepath.Join(dir, "node_modules")); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	_ = listener.Close()
	logPath := filepath.Join(dir, "next.log")
	log, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = log.Close() })
	cmd := exec.Command(filepath.Join(modules, ".bin/next"), "dev", "--webpack", "-H", "127.0.0.1", "-p", port)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "E2E_BUILD_DIR=.next", "NEXT_TELEMETRY_DISABLED=1", "XHUB_GATEWAY_ORIGIN="+gateway)
	cmd.Stdout, cmd.Stderr = log, log
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	})
	url := "http://127.0.0.1:" + port
	client := &http.Client{Timeout: 500 * time.Millisecond}
	for deadline := time.Now().Add(30 * time.Second); ; {
		resp, err := client.Get(url + "/gw/health/liveliness")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == 200 {
				return url
			}
		}
		if time.Now().After(deadline) {
			raw, _ := os.ReadFile(logPath)
			t.Fatalf("Next代理未就绪: %v\n%s", err, raw)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// TestConsoleProxyStreamsSmallFrames 通过真实 Next rewrite、Go HTTP 网关和私有数据库验证小帧无需终态即可到达。
// 显式 XHUB_REGRESSION_NEXT=1 才运行；三协议标准/原生及两种跨协议均使用小于1KiB首帧和保持连接的本地上游。
// 客户端默认协商gzip但不得收到压缩缓冲；读到标记即取消，上游必须断开；无付费调用，夹具回收进程、连接和schema。
func TestConsoleProxyStreamsSmallFrames(t *testing.T) {
	if os.Getenv("XHUB_REGRESSION_NEXT") != "1" {
		t.Skip("需要显式开启本地Next代理回归")
	}
	t.Setenv("XHUB_REGRESSION_REDIS_URL", "")
	stopped := make(chan struct{}, 10)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		protocol, cut := "chat", "data: {\"id\":\"chatcmpl_terminal\",\"choices\":[{\"index\":0,\"delta\":{},"
		if strings.HasSuffix(r.URL.Path, "/responses") {
			protocol, cut = "responses", "event: response.completed"
		}
		if strings.HasSuffix(r.URL.Path, "/messages") {
			protocol, cut = "messages", "event: content_block_stop"
		}
		prefix := strings.Split(streamTerminalFrames(protocol), cut)[0]
		if len(prefix) >= 1024 {
			t.Errorf("首帧夹具过大: %d", len(prefix))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "private, no-store")
		_, _ = io.WriteString(w, prefix)
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
		stopped <- struct{}{}
	}))
	t.Cleanup(up.Close)
	var models []config.ModelEntry
	for _, protocol := range []string{"chat", "responses", "messages"} {
		dep := chatDeployment("proxy-" + protocol)
		dep.LiteLLMParams["api_base"] = up.URL
		dep.ModelInfo["id"] = "proxy-" + protocol
		// 原生入口必须与该部署的传输协议一致；标准三入口则由真实桥接链转换。
		dep.ModelInfo["endpoint_types"] = []string{"chat", "responses", "messages", map[string]string{"chat": "bypass:openai-chat", "responses": "bypass:openai-responses", "messages": "bypass:anthropic-messages"}[protocol]}
		if protocol == "responses" {
			dep.ModelInfo["transport"] = "bypass_openai_responses"
		}
		if protocol == "messages" {
			dep.ModelInfo["transport"] = "bypass_anthropic_messages"
			dep.LiteLLMParams["custom_llm_provider"] = "anthropic"
		}
		models = append(models, dep)
	}
	h := newHarness(t, models...)
	owner := h.openScope(t, h.adminSession(), "console-proxy")
	proxy := consoleProxyFixture(t, h.server.URL)
	for _, tc := range []struct{ source, target, path string }{
		{"chat", "chat", "/v1/chat/completions"}, {"responses", "responses", "/v1/responses"}, {"messages", "messages", "/v1/messages"},
		{"chat", "chat", "/bypass/openai/v1/chat/completions"}, {"responses", "responses", "/bypass/openai/v1/responses"}, {"messages", "messages", "/bypass/anthropic/v1/messages"},
		{"chat", "responses", "/v1/responses"}, {"chat", "messages", "/v1/messages"},
	} {
		t.Run(tc.source+tc.path, func(t *testing.T) {
			body := map[string]any{"model": "proxy-" + tc.source, "stream": true, "messages": []any{map[string]any{"role": "user", "content": "Return a short marker."}}, "max_tokens": 32}
			if tc.target == "responses" {
				delete(body, "messages")
				delete(body, "max_tokens")
				body["input"] = "Return a short marker."
			}
			raw, _ := json.Marshal(body)
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			req, _ := http.NewRequestWithContext(ctx, "POST", proxy+"/gw"+tc.path, bytes.NewReader(raw))
			req.Header.Set("Authorization", "Bearer "+owner.key)
			req.Header.Set("Content-Type", "application/json")
			started := time.Now()
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				raw, _ := io.ReadAll(resp.Body)
				t.Fatalf("代理请求失败: %d %s", resp.StatusCode, raw)
			}
			if !strings.Contains(resp.Header.Get("Cache-Control"), "no-transform") || resp.Uncompressed || resp.Header.Get("Content-Encoding") != "" {
				t.Fatalf("SSE被代理压缩: %v uncompressed=%t", resp.Header, resp.Uncompressed)
			}
			scanner := bufio.NewScanner(resp.Body)
			found := false
			for scanner.Scan() {
				if strings.Contains(scanner.Text(), "TERMINAL_OK") {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("终态前未读到小帧: %v", scanner.Err())
			}
			t.Logf("小帧经真实Next代理到达: %s", time.Since(started))
			cancel()
			_ = resp.Body.Close()
			select {
			case <-stopped:
			case <-time.After(2 * time.Second):
				t.Fatal("代理取消没有传到上游")
			}
		})
	}
}

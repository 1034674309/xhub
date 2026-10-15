package regression

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestRealAgentClients 使用实际 Codex/Claude Code 二进制完成一次本地文件工具往返。
// 参数 t 为回归上下文；显式 XHUB_TEST_CODEX_BIN/XHUB_TEST_CLAUDE_BIN 才运行，未安装时明确跳过。
// 私有数据库和本地协议夹具隔离供应商波动；验证真实 CLI 请求/工具结果、日志及费用，不代表真实模型质量或上游兼容认证。
// XHUB_AGENT_LIVE=1 并提供 LIVE_KEY/BASE 时显式改用真实上游，会产生费用；默认不访问外部服务。
// CLI 禁用用户配置/外部 MCP/持久会话；仅授权读取测试目录，二进制由调用方提供，目录/schema/密钥由测试清理。
func TestRealAgentClients(t *testing.T) {
	for _, client := range []string{"codex", "claude"} {
		t.Run(client, func(t *testing.T) {
			variable := "XHUB_TEST_CODEX_BIN"
			if client == "claude" {
				variable = "XHUB_TEST_CLAUDE_BIN"
			}
			binary := os.Getenv(variable)
			if binary == "" {
				t.Skip(variable + " not configured; real CLI not exercised")
			}
			live := os.Getenv("XHUB_AGENT_LIVE") == "1"
			liveKey, liveBase := os.Getenv("XHUB_AGENT_LIVE_KEY"), strings.TrimRight(os.Getenv("XHUB_AGENT_LIVE_BASE"), "/")
			if live && (liveKey == "" || liveBase == "") {
				t.Fatal("真实供应商验收必须显式配置 XHUB_AGENT_LIVE_KEY/BASE")
			}
			directory := t.TempDir()
			sentinel := "xhub-real-agent-file-content-9317"
			file := filepath.Join(directory, "probe.txt")
			if err := os.WriteFile(file, []byte(sentinel), 0600); err != nil {
				t.Fatal(err)
			}
			var mu sync.Mutex
			requests := 0
			toolReturned := false
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					w.WriteHeader(400)
					return
				}
				mu.Lock()
				defer mu.Unlock()
				requests++
				raw, _ := json.Marshal(body["input"])
				done := (strings.Contains(string(raw), "function_call_output") || strings.Contains(string(raw), "custom_tool_call_output")) && strings.Contains(string(raw), sentinel)
				if done {
					toolReturned = true
				}
				if live {
					encoded, _ := json.Marshal(body)
					req, err := http.NewRequestWithContext(r.Context(), "POST", liveBase+"/responses", bytes.NewReader(encoded))
					if err != nil {
						t.Error("live request construction failed")
						w.WriteHeader(502)
						return
					}
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Authorization", "Bearer "+liveKey)
					httpClient := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
					response, err := httpClient.Do(req)
					if err != nil {
						t.Errorf("live upstream request failed: %T", err)
						w.WriteHeader(502)
						return
					}
					defer response.Body.Close()
					w.Header().Set("Content-Type", response.Header.Get("Content-Type"))
					w.WriteHeader(response.StatusCode)
					buffer := make([]byte, 8192)
					for {
						n, e := response.Body.Read(buffer)
						if n > 0 {
							if _, err := w.Write(buffer[:n]); err != nil {
								return
							}
							if f, ok := w.(http.Flusher); ok {
								f.Flush()
							}
						}
						if e != nil {
							if e != io.EOF {
								t.Errorf("live stream read failed: %T", e)
							}
							break
						}
					}
					return
				}
				if done {
					toolReturned = true
					writeAgentResponse(w, nativeAgentResponse(fmt.Sprintf("resp_cli_%d", requests), "AGENT_OK "+sentinel, false, nil), body["stream"] == true)
					return
				}
				toolName := ""
				argument := ""
				tools, _ := body["tools"].([]any)
				for _, value := range tools {
					tool, _ := value.(map[string]any)
					name, _ := tool["name"].(string)
					if client == "codex" && (name == "exec_command" || name == "shell_command" || name == "shell") {
						toolName = name
						args := map[string]any{"cmd": "cat " + file, "workdir": directory, "max_output_tokens": 1000}
						if name == "shell_command" {
							args = map[string]any{"command": "cat " + file, "workdir": directory}
						}
						if name == "shell" {
							args = map[string]any{"command": []string{"cat", file}, "workdir": directory}
						}
						encoded, _ := json.Marshal(args)
						argument = string(encoded)
						break
					}
					if client == "claude" && name == "Read" {
						toolName = name
						encoded, _ := json.Marshal(map[string]any{"file_path": file})
						argument = string(encoded)
						break
					}
				}
				// 桌面版 Codex 将代码执行工具放在 additional_tools/namespace 中，必须保留原生自定义工具语义。
				custom := false
				if client == "codex" && toolName == "" {
					inputs, _ := body["input"].([]any)
					for _, value := range inputs {
						item, _ := value.(map[string]any)
						if item["type"] != "additional_tools" {
							continue
						}
						groups, _ := item["tools"].([]any)
						for _, value := range groups {
							group, _ := value.(map[string]any)
							if group["name"] != "functions" {
								continue
							}
							definitions, _ := group["tools"].([]any)
							for _, value := range definitions {
								tool, _ := value.(map[string]any)
								if tool["name"] == "exec" && tool["type"] == "custom" {
									toolName = "exec"
									custom = true
									args, _ := json.Marshal(map[string]any{"cmd": "cat " + file, "workdir": directory, "max_output_tokens": 1000})
									argument = "text(await tools.exec_command(" + string(args) + "));"
								}
							}
						}
					}
				}
				if toolName == "" {
					if report := os.Getenv("XHUB_AGENT_REPORT_DIR"); report != "" {
						raw, _ := json.Marshal(map[string]any{"tools": body["tools"], "input": body["input"]})
						_ = os.WriteFile(filepath.Join(report, client+"-request.json"), raw, 0600)
					}
					t.Errorf("实际 %s 请求没有预期文件工具；收到 %d 个工具", client, len(tools))
					w.WriteHeader(400)
					writeJSON(w, map[string]any{"error": map[string]any{"message": "expected file tool missing"}})
					return
				}
				call := map[string]any{"id": "fc_cli", "type": "function_call", "status": "completed", "call_id": "call_cli", "name": toolName, "arguments": argument}
				if custom {
					call = map[string]any{"id": "ctc_cli", "type": "custom_tool_call", "status": "completed", "call_id": "call_cli", "name": "exec", "namespace": "functions", "input": argument}
				}
				writeAgentResponse(w, nativeAgentResponse(fmt.Sprintf("resp_cli_%d", requests), "", false, call), body["stream"] == true)
			}))
			t.Cleanup(upstream.Close)
			dep := chatDeployment("gpt-5.6-sol")
			dep.LiteLLMParams["api_base"] = upstream.URL + "/v1"
			dep.ModelInfo["transport"] = "bypass_openai_responses"
			dep.ModelInfo["endpoint_types"] = []string{"chat", "responses", "messages"}
			h := newHarness(t, dep)
			baseHandler := h.server.Config.Handler
			h.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1/messages" {
					raw, _ := io.ReadAll(r.Body)
					r.Body = io.NopCloser(bytes.NewReader(raw))
					var body map[string]any
					_ = json.Unmarshal(raw, &body)
					t.Logf("Claude 桥接能力参数 context_management=%s thinking=%s output_config=%s", mustJSON(body["context_management"]), mustJSON(body["thinking"]), mustJSON(body["output_config"]))
				}
				baseHandler.ServeHTTP(w, r)
			})
			admin := h.adminSession()
			owner := h.openScope(t, admin, "real-"+client)
			ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
			defer cancel()
			prompt := "Read " + file + " using your file or shell tool, then reply with AGENT_OK and its contents. Do not modify any files."
			var args, env []string
			if client == "codex" {
				args = []string{"exec", "--ignore-user-config", "--ignore-rules", "--ephemeral", "--skip-git-repo-check", "-C", directory, "-s", "read-only", "--json", "-m", dep.ModelName, "-c", `model_provider="xhub"`, "-c", fmt.Sprintf(`model_providers.xhub={name="XHub",base_url="%s/v1",env_key="XHUB_AGENT_TEST_KEY",wire_api="responses",requires_openai_auth=false,supports_websockets=false}`, h.server.URL), "-c", `model_reasoning_effort="medium"`, prompt}
				env = []string{"XHUB_AGENT_TEST_KEY=" + owner.key}
			} else {
				args = []string{"--bare", "--print", "--no-session-persistence", "--strict-mcp-config", "--mcp-config", `{"mcpServers":{}}`, "--tools", "Read", "--allowedTools", "Read", "--max-turns", "3", "--output-format", "json", "--model", dep.ModelName, "--effort", "low", prompt}
				env = []string{"ANTHROPIC_API_KEY=" + owner.key, "ANTHROPIC_BASE_URL=" + h.server.URL, "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1", "DISABLE_AUTOUPDATER=1", "DISABLE_TELEMETRY=1", "CLAUDE_CODE_DISABLE_AUTO_MEMORY=1"}
			}
			command := exec.CommandContext(ctx, binary, args...)
			command.Dir = directory
			// 子进程只拿一次性网关密钥，真实上游凭据仅留在测试转发器内存，不能暴露给 Agent 工具环境。
			for _, entry := range os.Environ() {
				name, _, _ := strings.Cut(entry, "=")
				if strings.Contains(name, "API_KEY") || strings.HasPrefix(name, "XHUB_AGENT_LIVE") || name == "ANTHROPIC_AUTH_TOKEN" {
					continue
				}
				command.Env = append(command.Env, entry)
			}
			command.Env = append(command.Env, env...)
			output, err := command.CombinedOutput()
			// 只有本地夹具与一次性虚拟密钥参与；失败诊断仍删除密钥后再输出。
			safe := strings.ReplaceAll(string(output), owner.key, "[REDACTED]")
			if report := os.Getenv("XHUB_AGENT_REPORT_DIR"); report != "" {
				if err := os.MkdirAll(report, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(report, client+".jsonl"), []byte(safe), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err != nil {
				t.Fatalf("实际 %s CLI 失败: %v\n%s", client, err, safe)
			}
			mu.Lock()
			count, returned := requests, toolReturned
			mu.Unlock()
			if count < 2 || !returned || (!strings.Contains(safe, "AGENT_OK") || !strings.Contains(safe, sentinel)) {
				t.Fatalf("%s 没有完成真实工具往返: requests=%d returned=%v\n%s", client, count, returned, safe)
			}
			h.flushSpend()
			rows := h.successRows(t, admin, dep.ModelName)
			if len(rows) != count {
				t.Fatalf("%s CLI 调用数与成功账单不符: calls=%d rows=%d", client, count, len(rows))
			}
			t.Logf("实际 %s CLI：%d 次网关请求、一次文件工具往返、%d 条账单", client, count, len(rows))
		})
	}
}

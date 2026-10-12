# 启动配置与模型定义

[English](readme.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

进程入口先 Load，再把同一 Config 交给 IAM/Store 初始化和 gateway.New。Load 只读 YAML 与环境变量，检查 PostgreSQL URL；它不证明数据库、Redis 或供应商可连接。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| Load(path) | 读取并返回 *Config,error |
| ModelEntry.ParamString(key, fallback) | 读取连接字符串参数 |
| ModelEntry.Disabled() | 读取明确的停用标记 |

### 接入 demo

将下面代码保存为仓库内的 `demo/main.go`，在仓库根目录执行 `go run ./demo /tmp/xhub-demo.yaml`；运行后删除本次创建的 demo 目录。

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/config"
	"os"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	if len(os.Args) != 2 {
		panic("usage: demo <config.yaml>")
	}
	cfg, err := config.Load(os.Args[1])
	if err != nil {
		panic(err)
	}
	fmt.Println(cfg.RouterSettings.RoutingStrategy, cfg.RouterSettings.NumRetries, cfg.RouterSettings.Timeout)
}
```

先将以下内容保存为 /tmp/xhub-demo.yaml：

```yaml
general_settings:
  database_url: postgres://localhost/demo
```

把配置路径作为参数。仅含 general_settings.database_url: postgres://localhost/demo 的 YAML 会输出 random 2 60；示例不连接数据库。

### 错误、状态与副作用

os.environ/NAME 未设置时展开为空；零值触发默认值。不要输出整个 Config，其中可能有密钥和管理员密码。

## 说明

config 把网关 YAML 启动文件加载为有类型的进程配置，同时保留路由和通用设置中未声明的键，供后续数据库覆盖按键合并。它定义模型部署、路由默认值、数据库与 Redis 地址、初始管理员、可选应急主密钥和花费日志开关。加载只读取文件和环境变量，不连接外部服务，也不改写配置文件。

## 对外接口

- `Load(path string) (*Config, error)` 读取 YAML，展开受支持的环境引用，校验数据库 URL，补默认值，并解析部署中的字符串参数。文件和 YAML 错误原样返回；数据库地址缺失或协议不是 postgres/postgresql 时返回校验错误。进程启动在组装网关、路由、IAM 和数据面前调用。
- `ModelEntry.Disabled() bool` 仅在 `model_info.disabled` 实际为布尔 true 时返回真；字段缺失或类型错误时仍视为启用。模型发现和路由挑选部署时使用。
- `ModelEntry.ParamString(key, fallback string) string` 从 `litellm_params` 读取非空字符串，否则返回 fallback，供路由和供应商选择避免不安全类型断言。
- `Config` 提供模型、路由、LiteLLM、护栏和通用设置。`RouterRaw` 与 `GeneralRaw` 保留 YAML 映射，让数据库覆盖逐键处理；保留某个键不表示运行时一定实现其语义。

本包没有 HTTP 边界。命令与测试负责加载，gateway、router、provider 等包消费返回值。

## 内部核心实现

Load 对 YAML 解码两次：第一次填充有类型结构，第二次捕获原始映射。环境引用格式为 `os.environ/NAME`，只在主密钥、数据库与 Redis 地址、初始管理员字段以及各部署 `litellm_params` 的字符串值中展开；变量未设置时结果为空。随后强制使用 PostgreSQL，并在字段为零值时设置默认值：路由策略 random、重试两次、超时 60 秒。因此这些字段无法区分“显式零”和“省略”。

管理员凭据只由网关启动流程在账号不存在时用于播种，以后修改 YAML 不会重写已保存密码。`disable_env_credential_login` 关闭播种。主密钥可以缺省，它是应急 API 凭据而非控制台登录。解析后的秘密保存在内存中，调用方不得写入日志。

### 测试依赖

`config_test.go` 覆盖默认值、原始映射、环境展开、管理员字段、可选主密钥和禁用部署；`url_test.go` 覆盖数据库协议。运行 `go test ./internal/config -count=1`；测试使用临时文件和环境变量，不连接真实数据库。

## 源码职责与入口

- `config.go` 包含全部生产类型与操作：Config、ModelEntry、RouterSettings、GeneralSettings、Load、环境解析、禁用部署判定和字符串参数读取。
- `config_test.go` 验证解析和默认值；`url_test.go` 专门验证 URL 校验。

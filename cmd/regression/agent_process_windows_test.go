//go:build windows

package regression

import (
	"os/exec"
	"time"
)

// isolateAgentCommand 保持 Windows 测试编译并限制管道等待；参数为尚未启动的命令，无返回值或进程副作用。
// 本轮真实 CLI 验收只运行于 POSIX，调用方在 Windows 上显式跳过；不宣称 Windows 子进程组清理能力。
func isolateAgentCommand(command *exec.Cmd) { command.WaitDelay = 3 * time.Second }

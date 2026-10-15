//go:build !windows

package regression

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// isolateAgentCommand 将 CLI 启动器及其子进程放入独立进程组，并在上下文超时后终止整组。
// 参数 command 为尚未启动的 CommandContext；无返回值；实际 CLI 回归调用，不启动进程或影响其他进程组。
// npm Codex 启动器会再派生 Rust/工具宿主，只杀启动器会留下持有 stdout 的子进程，使 CombinedOutput 永久等待。
// WaitDelay 限制管道清理等待；Cancel 只终止本命令创建的组，已退出的组按 ProcessDone 处理。
func isolateAgentCommand(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.WaitDelay = 3 * time.Second
	command.Cancel = func() error {
		if command.Process == nil || command.Process.Pid <= 0 {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}

// TestAgentProcessTimeoutTerminatesChildren 验证超时会同时清理包装器和持有 stdout 的子进程。
// 参数 t 为上下文；只启动本地 sleep 夹具，不调用模型或数据库；断言有界退出与无运行中的孤儿，defer 取消整组并 Wait 清理。
func TestAgentProcessTimeoutTerminatesChildren(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "sh", "-c", "sleep 30 & echo $!; wait")
	isolateAgentCommand(command)
	if err := command.Cancel(); !errors.Is(err, os.ErrProcessDone) {
		t.Fatalf("未启动命令的取消边界错误: %v", err)
	}
	pipe, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = command.Cancel() }()
	reader := bufio.NewReader(pipe)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	child, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || child <= 0 {
		t.Fatalf("未获取测试子进程: %q", line)
	}
	start := time.Now()
	cancel()
	finished := make(chan error, 1)
	go func() { finished <- command.Wait() }()
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("已取消的命令不应成功")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("超时后仍等待子进程管道")
	}
	if time.Since(start) > 4*time.Second {
		t.Fatal("CLI 超时清理超出上限")
	}
	// 已退出但尚待 init 回收的僵尸不再持有管道；不能误判为仍运行的工具宿主。
	state, err := exec.Command("ps", "-o", "stat=", "-p", fmt.Sprint(child)).Output()
	if err == nil && strings.TrimSpace(string(state)) != "" && !strings.HasPrefix(strings.TrimSpace(string(state)), "Z") {
		t.Fatalf("测试子进程未终止: pid=%d state=%s", child, state)
	}
}

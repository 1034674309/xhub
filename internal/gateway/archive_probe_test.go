package gateway

import (
	"context"
	"errors"
	"testing"
)

// probeArchive 记录探针清理上下文；仅单元测试使用，不访问网络或持久化。
type probeArchive struct {
	putErr, deleteErr error
	canceled          bool
	bounded           bool
	cancel            context.CancelFunc
}

// Put 模拟上传后客户端断开；参数为探针上下文、对象和正文，返回预设错误，供取消路径测试使用。
func (p *probeArchive) Put(ctx context.Context, key string, body []byte) error {
	if p.cancel != nil {
		p.cancel()
	}
	return p.putErr
}

// Delete 记录清理是否仍可执行且有时限；参数为清理上下文和对象，返回预设错误，无外部副作用。
func (p *probeArchive) Delete(ctx context.Context, key string) error {
	p.canceled = ctx.Err() != nil
	_, p.bounded = ctx.Deadline()
	return p.deleteErr
}

// TestArchiveProbeCleanup 前置内存存储；验证正常、上传失败、删除失败及客户端取消后的清理和错误保留，无数据需清理。
func TestArchiveProbeCleanup(t *testing.T) {
	for _, mode := range []string{"success", "upload", "delete", "both", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			upload, deletion := errors.New("upload"), errors.New("delete")
			p := &probeArchive{}
			if mode == "upload" || mode == "both" {
				p.putErr = upload
			}
			if mode == "delete" || mode == "both" {
				p.deleteErr = deletion
			}
			if mode == "cancel" {
				p.cancel = cancel
				p.putErr = context.Canceled
			}
			err := archiveProbe(ctx, p, "unique-probe")
			if p.canceled || !p.bounded {
				t.Fatal("清理上下文已取消或没有时限")
			}
			if (err == nil) != (mode == "success") || (p.putErr != nil && !errors.Is(err, p.putErr)) || (p.deleteErr != nil && !errors.Is(err, p.deleteErr)) {
				t.Fatal("上传或清理错误丢失", err)
			}
		})
	}
}

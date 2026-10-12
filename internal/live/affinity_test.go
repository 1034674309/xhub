package live

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestAffinityAtomicClaimAndExpiry 在专用真实 Redis 验证并发首次申请唯一赢家、迟到提交不覆盖、过期重新申请和存储错误。
// 无外部凭据，testRedis 结束时终止进程并清理私有数据；失败信息定位原子更新阶段。
func TestAffinityAtomicClaimAndExpiry(t *testing.T) {
	c := testRedis(t)
	ctx := context.Background()
	key := "private-affinity"
	var wg sync.WaitGroup
	results := make(chan string, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v, e := c.CompareAffinity(ctx, key, "", fmt.Sprint(i), time.Minute)
			if e != nil {
				t.Error(e)
			}
			results <- v
		}(i)
	}
	wg.Wait()
	close(results)
	winner := ""
	for v := range results {
		if winner == "" {
			winner = v
		}
		if v != winner {
			t.Fatalf("并发赢家不一致 %s/%s", winner, v)
		}
	}
	if got, e := c.CompareAffinity(ctx, key, winner, "new-version", time.Minute); e != nil || got != "new-version" {
		t.Fatalf("成功比较更新: %s %v", got, e)
	}
	if got, e := c.CompareAffinity(ctx, key, winner, "late", time.Minute); e != nil || got != "new-version" {
		t.Fatalf("迟到提交覆盖: %s %v", got, e)
	}
	c.rdb.PExpire(ctx, key, time.Millisecond)
	time.Sleep(5 * time.Millisecond)
	if got, e := c.ReadAffinity(ctx, key); e != nil || got != "" {
		t.Fatalf("过期未变为 miss: %s %v", got, e)
	}
	if got, e := c.CompareAffinity(ctx, key, "", "fresh", time.Minute); e != nil || got != "fresh" {
		t.Fatalf("过期无法重新申请: %s %v", got, e)
	}
	c.rdb.Close()
	if _, e := c.ReadAffinity(ctx, key); e == nil {
		t.Fatal("存储错误被当作 miss")
	}
}

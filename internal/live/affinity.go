package live

import (
	"context"
	"github.com/redis/go-redis/v9"
	"time"
)

// ReadAffinity 严格读取会话缓存；参数为上下文和键，返回值或存储错误，miss 返回空且无错误。
// affinity 调用；保留故障信息供普通偏好降级和响应归属失败关闭。
func (c *Client) ReadAffinity(ctx context.Context, key string) (string, error) {
	value, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return value, err
}

// CompareAffinity 原子比较更新偏好和 TTL；参数为上下文、键、旧值、新值及有效期，返回当前赢家或错误。
// 空旧值实现首次申请，版本不符不写入，不加会话执行锁；affinity 的申请与成功提交调用。
func (c *Client) CompareAffinity(ctx context.Context, key, old, value string, ttl time.Duration) (string, error) {
	return c.rdb.Eval(ctx, affinityCompareScript, []string{key}, old, value, ttl.Milliseconds()).Text()
}

const affinityCompareScript = `local current = redis.call('GET', KEYS[1]) or ''
if current == ARGV[1] then
 redis.call('SET', KEYS[1], ARGV[2], 'PX', ARGV[3])
 return ARGV[2]
end
return current`

// WriteAffinity 发布不可变响应归属或上下文；参数为上下文、键、值及 TTL，返回真实存储错误。
// CommitRoute 调用；失败由上层阻止发布后续归属，不隐藏错误。
func (c *Client) WriteAffinity(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

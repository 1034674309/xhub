// Package live is Redis for the hot path. Cooldown, latency, RPM, TPM, and spend deltas live here. The request itself does not write PostgreSQL.
package live

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sunqirui1987/xhub/internal/util"
)

// Client is the hot-path Redis client. A request only updates it. Spend logs are written to PostgreSQL by the timed flush.
// Client is the gateway hot path. Router cooldown, latency, and token counters
// live here, as do RPM/TPM buckets and the spend delta. Spend logs are queued
// and flushed to PostgreSQL by the server; the request does not insert them.
type Client struct {
	rdb *redis.Client
}

// SpendRef 拼出热花费计数的 Redis 键，按密钥、团队、用户、组织或项目分开。
// 参数 kind（string）：分类名，用来选择限额主体、日志类型或官方端点；id（string）：花费引用使用的主键。空串表示调用方没有指定记录。
// 返回 string（string）：热花费计数的 Redis 前缀，按密钥、团队、用户或组织分开。
// 调用：dataplane/live.go、gateway/limits.go、gateway/spend.go
// 测试：无直接单测
func SpendRef(kind, id string) string {
	if kind == "" || id == "" {
		return id
	}
	return kind + "/" + id
}

var logTraceOnceRedis sync.Once

// Open parses the Redis URL and pings it. On failure it closes the client and returns the error.
// 参数 url（string）：根地址或完整 URL。空串表示改用供应商默认根，末尾斜杠会去掉。
// 返回 *Client（*Client）：已 ping 通的 Redis 客户端。解析 URL 或 ping 失败时为 nil，并且会关掉半开的客户端；error（error）：URL 不合法或 ping 失败。nil 表示可以用。
// 调用：gateway/server.go、iam/db.go、store/engine.go、store/store.go
// 测试：authz_test.go、builtin_providers_test.go、chains_test.go
func Open(url string) (*Client, error) {
	logTraceOnceRedis.Do(func() { util.Trace("enter live.Open") })

	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	c := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		return nil, err
	}
	return &Client{rdb: c}, nil
}

// Close closes Redis. A nil client is not an error.
// 参数：无。
// 返回 error（error）：失败原因，nil 表示这一步成功。
// 调用：dataplane/official.go、dataplane/serve.go、dataplane/stream.go、gateway/ingress.go
// 测试：authz_test.go、builtin_providers_test.go、bypass_logic_test.go
func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}

// RecordFailure increments the failure count. After allowed failures it writes the cooldown key. An allowed below 1 does nothing.
// 参数 id（string）：记录失败使用的主键。空串表示调用方没有指定记录；allowed（int）：记录失败使用的整数。零表示没有这项或尚未计数；cooldown（time.Duration）：一段时间。零值表示改用调用方约定的默认时长，例如会话一小时、官方任务七天。
// 返回 error（error）：失败原因，nil 表示这一步成功。
// 调用：dataplane/live.go、gateway/wire.go
// 测试：无直接单测
func (c *Client) RecordFailure(id string, allowed int, cooldown time.Duration) error {
	if c == nil || id == "" || allowed < 1 {
		return nil
	}
	ctx := context.Background()
	n, err := c.rdb.Incr(ctx, "xhub:fails:"+id).Result()
	if err != nil {
		return err
	}
	_ = c.rdb.Expire(ctx, "xhub:fails:"+id, cooldown).Err()
	if int(n) < allowed {
		return nil
	}
	return c.rdb.Set(ctx, "xhub:cooldown:"+id, "1", cooldown).Err()
}

// Cooled reports which deployment ids are still cooling down. If Redis is unavailable it returns an empty map and the caller treats that as no cooldown.
// 参数 ids（[]string）：要保留或查询的一组 id。空切片表示没有可处理的记录。
// 返回 map[string]bool（map[string]bool）：Cooled。没有该键表示假，不要当成缺省 JSON。
// 调用：dataplane/live.go
// 测试：无直接单测
func (c *Client) Cooled(ids []string) map[string]bool {
	out := map[string]bool{}
	if c == nil || len(ids) == 0 {
		return out
	}
	ctx := context.Background()
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = "xhub:cooldown:" + id
	}
	vals, err := c.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return out
	}
	for i, v := range vals {
		if v != nil {
			out[ids[i]] = true
		}
	}
	return out
}

// AddLatency pushes one latency onto the left of the list, keeps the latest 20, and expires the key after one hour.
// 参数 id（string）：累加延迟使用的主键。空串表示调用方没有指定记录；ms（float64）：累加延迟使用的小数。0 表示没有费用或尚未计价。
// 返回 error（error）：失败原因，nil 表示这一步成功。
// 调用：dataplane/live.go
// 测试：无直接单测
func (c *Client) AddLatency(id string, ms float64) error {
	if c == nil || id == "" {
		return nil
	}
	ctx := context.Background()
	key := "xhub:latency:" + id
	pipe := c.rdb.TxPipeline()
	pipe.LPush(ctx, key, fmt.Sprintf("%g", ms))
	pipe.LTrim(ctx, key, 0, 19)
	pipe.Expire(ctx, key, time.Hour)
	_, err := pipe.Exec(ctx)
	return err
}

// Latencies returns the average of recent latencies for each deployment. An id with no sample is omitted.
// 参数 ids（[]string）：要保留或查询的一组 id。空切片表示没有可处理的记录。
// 返回 map[string]float64（map[string]float64）：Latencies的数值表，例如花费或 token。没有该键表示还没有发生过。
// 调用：dataplane/live.go
// 测试：无直接单测
func (c *Client) Latencies(ids []string) map[string]float64 {
	out := map[string]float64{}
	if c == nil {
		return out
	}
	ctx := context.Background()
	for _, id := range ids {
		vals, err := c.rdb.LRange(ctx, "xhub:latency:"+id, 0, 19).Result()
		if err != nil || len(vals) == 0 {
			continue
		}
		sum := 0.0
		n := 0
		for _, raw := range vals {
			v, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				continue
			}
			sum += v
			n++
		}
		if n > 0 {
			out[id] = sum / float64(n)
		}
	}
	return out
}

// AddUsage adds tokens to the current minute bucket.
// 参数 id（string）：累加用量使用的主键。空串表示调用方没有指定记录；tokens（int）：累加用量使用的整数。零表示没有这项或尚未计数。
// 返回 error（error）：失败原因，nil 表示这一步成功。
// 调用：dataplane/live.go
// 测试：无直接单测
func (c *Client) AddUsage(id string, tokens int) error {
	if c == nil || id == "" || tokens == 0 {
		return nil
	}
	if tokens < 0 {
		return fmt.Errorf("negative token increment")
	}
	return incrementWithTTL.Run(context.Background(), c.rdb, []string{"xhub:routetpm:" + id}, tokens, 60000).Err()
}

// Usages returns each deployment's token usage for the current minute.
// 参数 ids（[]string）：要保留或查询的一组 id。空切片表示没有可处理的记录。
// 返回 map[string]float64（map[string]float64）：Usages的数值表，例如花费或 token。没有该键表示还没有发生过。
// 调用：dataplane/live.go
// 测试：无直接单测
func (c *Client) Usages(ids []string) map[string]float64 {
	out := map[string]float64{}
	if c == nil || len(ids) == 0 {
		return out
	}
	ctx := context.Background()
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = "xhub:routetpm:" + id
	}
	vals, err := c.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return out
	}
	for i, v := range vals {
		s, _ := v.(string)
		n, err := strconv.ParseFloat(s, 64)
		if err == nil {
			out[ids[i]] = n
		}
	}
	return out
}

// minuteBucket is the current Unix minute, used as the suffix of RPM and TPM keys.
// 参数：无。
// 返回 int64（int64）：当前 Unix 分钟，即 time.Now().Unix()/60。用作 RPM 和 TPM 键的后缀。这一步不会失败，也不会返回 0 来表示缺失。
// 调用：仅在 redis.go 内使用
// 测试：无直接单测
func minuteBucket() int64 { return time.Now().Unix() / 60 }

// HitRPM increments the current minute's request count and returns the new value.
// 参数 id（string）：虚拟密钥的令牌哈希，即 Principal.Hash。不是部署 id。计数键是 xhub:rpm: 加上这个哈希，再加当前分钟。
// 返回 int64（int64）：这一分钟累加 1 之后的请求数。客户端为 nil 时返回 0 且 error 为 nil；Redis INCR 失败时返回 0 和 error。
// 调用：gateway/limits.go
// 测试：无直接单测
func (c *Client) HitRPM(id string) (int64, error) {
	return c.bump("xhub:rpm:"+id, 1)
}

// HitTPM adds tokens to the current minute and returns the new value.
// 参数 id（string）：虚拟密钥的令牌哈希，即 Principal.Hash。不是部署 id。计数键是 xhub:tpm: 加上这个哈希，再加当前分钟；tokens（int）：这次估算要累加的 token 数。
// 返回 int64（int64）：这一分钟累加后的 token 数。客户端为 nil 时返回 0 且 error 为 nil；Redis INCR 失败时返回 0 和 error。
// 调用：gateway/limits.go
// 测试：无直接单测
func (c *Client) HitTPM(id string, tokens int) (int64, error) {
	return c.bump("xhub:tpm:"+id, int64(tokens))
}

// bump adds n to a minute counter and returns the new value. The key includes the minute number, so the next minute starts at zero after expiry.
// 参数 prefix（string）：Redis 键的前半段。HitRPM 传入 xhub:rpm: 加上密钥哈希，HitTPM 传入 xhub:tpm: 加上密钥哈希。不是部署 id，也不是要剥掉的路径前缀；n（int64）：要加到这个分钟计数上的数量。
// 返回 int64（int64）：INCRBY 之后的新计数。客户端为 nil 时返回 0 且 error 为 nil。Redis 失败时返回 0 和 error。键两分钟后过期。
// 调用：仅在 redis.go 内使用
// 测试：无直接单测
func (c *Client) bump(prefix string, n int64) (int64, error) {
	if c == nil {
		return 0, nil
	}
	if n < 0 {
		return 0, fmt.Errorf("negative counter increment")
	}
	key := fmt.Sprintf("%s:%d", prefix, minuteBucket())
	return incrementWithTTL.Run(context.Background(), c.rdb, []string{key}, n, 120000).Int64()
}

var incrementWithTTL = redis.NewScript("local n = redis.call('INCRBY', KEYS[1], ARGV[1]); redis.call('PEXPIRE', KEYS[1], ARGV[2]); return n")

// HotSpend 读取尚未确认落库的花费，供网关限额展示和检查使用，不消费队列。
// 参数 id 为 SpendRef 生成的主体标识；返回对应金额，缺少客户端、标识或可读计数时返回零。
// 严格预算准入不依靠此遥测读取降级放行；写入和释放只经过 EnqueueSpend 与 AckFlushed。
// 测试：redis_test.go 验证各层计数、非法入队、并发去重和重复确认。
func (c *Client) HotSpend(id string) float64 {
	if c == nil || id == "" {
		return 0
	}
	v, err := c.rdb.Get(context.Background(), "xhub:spend:"+id).Float64()
	if err != nil {
		return 0
	}
	return v
}

// SpendLog is one spend log waiting in the queue to be written to PostgreSQL.
type SpendLog struct {
	RequestID string `json:"request_id"`
	CallType  string `json:"call_type"`
	Model     string `json:"model"`
	APIKey    string `json:"api_key"`
	// KeyID is the api_keys row id, which is what usage_events keys ownership
	// by. APIKey stays the token hash, because that is the live-spend reference.
	KeyID      string  `json:"key_id,omitempty"`
	Prompt     int     `json:"prompt_tokens"`
	Completion int     `json:"completion_tokens"`
	Spend      float64 `json:"spend"`
	SpendValid bool    `json:"spend_valid"`
	Start      string  `json:"start"`
	End        string  `json:"end"`
	CacheHit   bool    `json:"cache_hit"`
	Status     string  `json:"status"`
	// HTTPStatus 是网关最终返回给调用方的状态码；Error 是失败时保留的完整原始正文。
	// 两项随 Redis 队列一起传递，避免异步刷库丢失错误诊断信息。
	HTTPStatus int    `json:"http_status,omitempty"`
	Error      string `json:"error,omitempty"`
	OwnerType  string `json:"owner_type,omitempty"`
	TeamID     string `json:"team_id"`
	UserID     string `json:"user_id"`
	OrgID      string `json:"org_id"`
	ProjectID  string `json:"project_id,omitempty"`
	Messages   string `json:"messages,omitempty"`
	Response   string `json:"response,omitempty"`
	// UpstreamResponse 保存已脱敏的上游响应头和原始统计 JSON。
	UpstreamResponse string `json:"upstream_response,omitempty"`
	ProxyRequest     string `json:"proxy_server_request,omitempty"`
	TTFTMs           *int   `json:"ttft_ms,omitempty"`
	KeyHash          string `json:"key_hash,omitempty"`
	KeyAlias         string `json:"key_alias,omitempty"`
	TeamAlias        string `json:"team_alias,omitempty"`
	Provider         string `json:"provider,omitempty"`
	CachedTokens     *int   `json:"cached_tokens,omitempty"`
	SessionID        string `json:"session_id,omitempty"`
	CacheKey         string `json:"cache_key,omitempty"`
	Guardrail        string `json:"guardrail,omitempty"`
	// PriceSnapshot is the JSON of the rates this call was actually billed at,
	// with the quantities they applied to. Without it a log row can only be
	// explained by recomputing from the price table of the day it is read, so
	// the same row shows a different "original cost" after every price change.
	// Empty means the call was never priced - not that it was free.
	PriceSnapshot string `json:"price_snapshot,omitempty"`
}

// EnqueueSpend 将网关用量日志与各层未落库花费原子入队，是唯一的队列写入口。
// 参数 row 的 RequestID 必须是全局唯一的不可变事件标识；零费用日志仍须具有标识。
// 返回 error：客户端不可用、身份或费用/token 非法、Redis 写入失败时非 nil；重复请求无副作用。
// 首个 payload 生效，确认后重放也不重复扣费；去重身份不设置 TTL，依赖部署的 Redis 持久化。
// 调用：gateway/spend.go；不会等待磁盘 fsync 或副本 quorum。测试：redis_test.go 覆盖边界和失败原子性。
func (c *Client) EnqueueSpend(row SpendLog) error {
	if c == nil || c.rdb == nil {
		return fmt.Errorf("enqueue spend: Redis client is unavailable")
	}
	if row.Spend < 0 || math.IsNaN(row.Spend) || math.IsInf(row.Spend, 0) || row.Prompt < 0 || row.Completion < 0 || (row.CachedTokens != nil && *row.CachedTokens < 0) {
		return fmt.Errorf("enqueue spend: invalid cost or token count")
	}
	if row.RequestID == "" {
		return fmt.Errorf("enqueue spend: request_id is required")
	}
	raw, err := json.Marshal(row)
	if err != nil {
		return err
	}
	args := []any{raw, 0.0, row.RequestID}
	if row.SpendValid {
		args[1] = row.Spend
	}
	for _, ref := range []struct{ kind, id string }{
		{"key", row.APIKey}, {"team", row.TeamID}, {"user", row.UserID},
		{"org", row.OrgID}, {"project", row.ProjectID},
	} {
		id := SpendRef(ref.kind, ref.id)
		if id != "" {
			args = append(args, id)
		}
	}
	return enqueueSpendScript.Run(context.Background(), c.rdb, []string{"xhub:spendlog", "xhub:spend:requests"}, args...).Err()
}

var enqueueSpendScript = redis.NewScript(`
if redis.call('HEXISTS', KEYS[2], ARGV[3]) == 1 then
  return 0
end
local delta = tonumber(ARGV[2])
-- Lua 出错不会回滚此前写入，因此先检查队列类型与全部金额，再发布日志。
-- 不维护无人消费的旧主体索引；队列记录自身携带确认所需的全部归属。
-- OOM 等基础设施故障仍须运维恢复，Redis 并不是数据库事务 outbox。
local queueType = redis.call('TYPE', KEYS[1]).ok
if queueType ~= 'none' and queueType ~= 'list' then
  return redis.error_reply('spend queue is not a list')
end
for i = 4, #ARGV do
  if delta ~= 0 then
    local raw = redis.call('GET', 'xhub:spend:' .. ARGV[i])
    local current = raw and tonumber(raw) or 0
    if (raw and not tonumber(raw)) or current + delta ~= current + delta or
       math.abs(current + delta) == math.huge then
      return redis.error_reply('invalid hot spend counter')
    end
  end
end
for i = 4, #ARGV do
  if delta ~= 0 then
    redis.call('INCRBYFLOAT', 'xhub:spend:' .. ARGV[i], delta)
  end
end
redis.call('RPUSH', KEYS[1], ARGV[1])
redis.call('HSET', KEYS[2], ARGV[3], '1')
return 1
`)

// PeekLogs 为 dataplane Flush 查看最多 n 条花费日志而不删除，返回解码记录及对应原始字符串。
// 客户端为空、n 小于一、读取失败或队列为空时返回 nil；损坏事件只返回其前面的合法前缀，不能越过坏记录。
// 测试：redis_test.go 覆盖正常队列和损坏队头，cmd/regression/spend_queue_test.go 覆盖刷写失败保留记录。
func (c *Client) PeekLogs(n int) (rows []SpendLog, raw []string) {
	if c == nil || n < 1 {
		return nil, nil
	}
	vals, err := c.rdb.LRange(context.Background(), "xhub:spendlog", 0, int64(n-1)).Result()
	if err != nil || len(vals) == 0 {
		return nil, nil
	}
	out := make([]SpendLog, 0, len(vals))
	for i, item := range vals {
		var row SpendLog
		if json.Unmarshal([]byte(item), &row) != nil || row.RequestID == "" {
			// 不能确认无法解码的事件，否则会永久丢失用量；坏队头必须显式修复后才能继续刷写。
			util.Error("spend queue contains an invalid event at offset %d", i)
			return out, vals[:i]
		}
		out = append(out, row)
	}
	return out, vals
}

// AckFlushed 在数据库事务成功后，原子释放各主体热花费并确认已写入的队列前缀。
// 参数 deltas 为主体花费标识到本批金额的映射，n 为记录数，head 为 PeekLogs 返回的首条原文。
// 返回 error：非法金额或 Redis 失败时非 nil；空客户端、空批次和队头已经改变时无副作用。
// 调用：dataplane/runtime.Flush；队头校验防止成功确认后重试重复扣减，金额预检防止只释放部分主体。
// 测试：redis_test.go 和 cmd/regression/spend_queue_test.go 验证确认原子性、重复确认及故障恢复。
func (c *Client) AckFlushed(deltas map[string]float64, n int, head string) error {
	if c == nil || n < 1 || head == "" {
		return nil
	}
	for _, delta := range deltas {
		if delta < 0 || math.IsNaN(delta) || math.IsInf(delta, 0) {
			return fmt.Errorf("ack spend: invalid delta")
		}
	}
	args := make([]any, 0, 2+len(deltas)*2)
	args = append(args, n, head)
	for id, delta := range deltas {
		if id == "" || delta == 0 {
			continue
		}
		args = append(args, id, delta)
	}
	return ackFlushScript.Run(context.Background(), c.rdb, []string{"xhub:spendlog"}, args...).Err()
}

var ackFlushScript = redis.NewScript(`
local head = redis.call('LINDEX', KEYS[1], 0)
if (not head) or head ~= ARGV[2] then
  return 0
end
local n = tonumber(ARGV[1])
-- Lua 错误不回滚已经执行的写入，必须在扣减前预检全部主体计数。
for i = 3, #ARGV, 2 do
  local raw = redis.call('GET', 'xhub:spend:' .. ARGV[i])
  local current = raw and tonumber(raw) or 0
  local delta = tonumber(ARGV[i + 1])
  if not delta or (raw and not tonumber(raw)) or
     current - delta ~= current - delta or math.abs(current - delta) == math.huge then
    return redis.error_reply('invalid hot spend counter')
  end
end
local i = 3
while i <= #ARGV do
  local id = ARGV[i]
  local delta = tonumber(ARGV[i + 1])
  if id ~= '' and delta ~= 0 then
    redis.call('INCRBYFLOAT', 'xhub:spend:' .. id, -delta)
  end
  i = i + 2
end
redis.call('LTRIM', KEYS[1], n, -1)
return 1
`)

// GetString reads one Redis string. A missing key returns ok false.
// 参数 ctx（context.Context）：上下文，取消时停止；key（string）：缓存或配置表的键。
// 返回 string（string）：读到的字符串。键不存在时为空串，调用方要把空串当成没有钉住；bool（bool）：Redis 里存在这个字符串键时返回真。没有这个键时返回假。
// 调用：gateway/affinity.go
// 测试：无直接单测
func (c *Client) GetString(ctx context.Context, key string) (string, bool) {
	if c == nil || c.rdb == nil || key == "" {
		return "", false
	}
	v, err := c.rdb.Get(ctx, key).Result()
	if err != nil || v == "" {
		return "", false
	}
	return v, true
}

// SetString stores one Redis string with a TTL. A nil client does nothing.
// 参数 ctx（context.Context）：上下文，取消时停止；key（string）：缓存或配置表的键；value（string）：写入字符串使用的值。空串表示调用方没有提供这项；ttl（time.Duration）：一段时间。零值表示改用调用方约定的默认时长，例如会话一小时、官方任务七天。
// 返回：无。这条带 TTL 的字符串已写入 Redis。客户端或键为空时什么都不写。
// 调用：gateway/affinity.go
// 测试：无直接单测
func (c *Client) SetString(ctx context.Context, key, value string, ttl time.Duration) {
	if c == nil || c.rdb == nil || key == "" {
		return
	}
	_ = c.rdb.Set(ctx, key, value, ttl).Err()
}

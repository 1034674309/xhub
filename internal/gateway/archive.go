package gateway

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/qiniu/go-sdk/v7/auth"
	qclient "github.com/qiniu/go-sdk/v7/client"
	"github.com/qiniu/go-sdk/v7/storage"
	"github.com/qiniu/go-sdk/v7/storagev2/uplog"
	"github.com/sunqirui1987/xhub/internal/iam"
	"github.com/sunqirui1987/xhub/internal/util"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// archiveObjectStore 是不可覆盖的对象写入边界；实现必须核验已有对象而非覆盖其他内容。
type archiveObjectStore interface {
	Put(context.Context, string, []byte) error
	Delete(context.Context, string) error
}

// kodoContentHash 计算七牛公开内容哈希；body 为对象字节，返回 URL-safe etag，无副作用。
// 四 MiB 以下使用单块 SHA1，较大对象按四 MiB 分块再 SHA1，供重试核验使用。
func kodoContentHash(body []byte) string {
	const block = 4 << 20
	if len(body) <= block {
		sum := sha1.Sum(body)
		return base64.URLEncoding.EncodeToString(append([]byte{0x16}, sum[:]...))
	}
	hashes := []byte{}
	for len(body) > 0 {
		n := len(body)
		if n > block {
			n = block
		}
		sum := sha1.Sum(body[:n])
		hashes = append(hashes, sum[:]...)
		body = body[n:]
	}
	sum := sha1.Sum(hashes)
	return base64.URLEncoding.EncodeToString(append([]byte{0x96}, sum[:]...))
}

// kodoArchive 使用七牛官方 SDK 与受控 HTTPS 端点，不接受任意自定义上传域名。
type kodoArchive struct {
	bucket string
	mac    *auth.Credentials
	cfg    *storage.Config
	client *qclient.Client
}

// archiveSDKOnce 保证进程仅关闭一次 SDK 的可选上传遥测，避免归档信息写入本地遥测文件。
var archiveSDKOnce sync.Once

// newKodoArchive 解密指定版本配置并构建客户端；返回适配器或固定错误，不访问网络。
func newKodoArchive(c *iam.ArchiveConfig, master string) (archiveObjectStore, error) {
	archiveSDKOnce.Do(uplog.DisableUplog)
	if c == nil {
		return nil, errors.New("archive not configured")
	}
	plain, err := enterpriseSecret(master, c.Credentials, true)
	if err != nil {
		return nil, err
	}
	var keys struct {
		AK string
		SK string
	}
	if json.Unmarshal([]byte(plain), &keys) != nil || keys.AK == "" || keys.SK == "" {
		return nil, errors.New("invalid archive credentials")
	}
	region, ok := storage.GetRegionByID(storage.RegionID(c.Region))
	if !ok {
		return nil, errors.New("invalid archive region")
	}
	return &kodoArchive{bucket: c.Bucket, mac: auth.New(keys.AK, keys.SK), cfg: &storage.Config{Region: &region, UseHTTPS: true}, client: &qclient.Client{Client: enterpriseHTTPClient()}}, nil
}

// Put 使用 insertOnly 与作用域 token 上传；ctx/key/body 为受控对象，返回核验错误。
// 网络超时后重试可能遇到已有对象，使用七牛内容哈希和字节数比较，不覆盖不同内容。
func (k *kodoArchive) Put(ctx context.Context, key string, body []byte) error {
	policy := storage.PutPolicy{Scope: k.bucket + ":" + key, InsertOnly: 1, Expires: 300}
	var ret storage.PutRet
	mime := "application/octet-stream"
	if strings.HasSuffix(key, ".gz") {
		mime = "application/gzip"
	} else if strings.HasSuffix(key, ".json") {
		mime = "application/json"
	}
	err := storage.NewFormUploaderEx(k.cfg, k.client).Put(ctx, &ret, policy.UploadToken(k.mac), key, bytes.NewReader(body), int64(len(body)), &storage.PutExtra{MimeType: mime, TryTimes: 1})
	expected := kodoContentHash(body)
	if err == nil && ret.Hash == expected && ret.Key == key {
		return nil
	}
	// Stat 沿用有时限客户端，即使上传响应丢失也能安全确认相同内容。
	stat, statErr := storage.NewBucketManagerEx(k.mac, k.cfg, k.client).Stat(k.bucket, key)
	if statErr == nil && stat.Hash == expected && stat.Fsize == int64(len(body)) {
		return nil
	}
	return errors.New("archive object verification failed")
}

// Delete 仅用于管理员连接测试的随机探针，参数为 key，返回错误；不提供归档删除接口。
func (k *kodoArchive) Delete(ctx context.Context, key string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return storage.NewBucketManagerEx(k.mac, k.cfg, k.client).Delete(k.bucket, key)
}

// archiveEncode 生成确定性的 JSONL.gz 与 SHA-256；参数为不可变事件，返回对象和哈希或错误。
// 不收集请求正文；固定 gzip header 使不同 worker 生成同一内容。
func archiveEncode(e *iam.ArchiveEvent) ([]byte, string, error) {
	var payload any
	if json.Unmarshal([]byte(e.Payload), &payload) != nil {
		return nil, "", errors.New("invalid archive snapshot")
	}
	var raw bytes.Buffer
	if err := json.NewEncoder(&raw).Encode(map[string]any{"event_id": e.ID, "configuration_revision": e.Revision, "usage": payload}); err != nil {
		return nil, "", err
	}
	var out bytes.Buffer
	g := gzip.NewWriter(&out)
	if _, err := g.Write(raw.Bytes()); err != nil {
		return nil, "", err
	}
	if err := g.Close(); err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(out.Bytes())
	return out.Bytes(), hex.EncodeToString(sum[:]), nil
}

// processArchive 领取并处理单条事件；factory 是真实或测试存储适配器，返回是否有任务与错误。
// 成功上传数据和 manifest 后才确认；失败可重试且不影响账务事务，租约丢失不得确认。
func (s *Server) processArchive(ctx context.Context, factory func(*iam.ArchiveConfig) (archiveObjectStore, error)) (bool, error) {
	lease, err := enterpriseRandom()
	if err != nil {
		return false, err
	}
	e, err := s.Identity().ClaimArchiveEvent(ctx, lease)
	if err != nil || e == nil {
		return false, err
	}
	c, err := s.Identity().ArchiveConfiguration(ctx, e.Revision)
	var store archiveObjectStore
	if err == nil {
		store, err = factory(c)
	}
	body, hash, encodeErr := archiveEncode(e)
	if err == nil {
		err = encodeErr
	}
	key := ""
	if c != nil {
		key = fmt.Sprintf("%s/v%d/events/%020d.jsonl.gz", c.Prefix, e.Revision, e.ID)
	}
	if err == nil {
		err = store.Put(ctx, key, body)
	}
	if err == nil {
		manifest, _ := json.Marshal(map[string]any{"schema_version": 1, "event_id": e.ID, "object_key": key, "sha256": hash, "byte_size": len(body), "row_count": 1, "configuration_revision": e.Revision})
		err = store.Put(ctx, key+".manifest.json", manifest)
	}
	owned, finishErr := s.Identity().CompleteArchiveEvent(ctx, e, key, hash, int64(len(body)), err == nil)
	if finishErr != nil {
		return true, finishErr
	}
	if !owned {
		return true, errors.New("archive lease lost")
	}
	return true, err
}

// archiveCycleDue 判断本轮调度是否到期；now/last 为当前时间与上次开始时间，间隔须已校验。
// 供后台消费者和确定性测试使用，无副作用；时钟回退不会提前消费。
func archiveCycleDue(now, last time.Time, interval int) bool {
	return now.Sub(last) >= time.Duration(interval)*time.Second
}

// runArchiveBatch 串行消费最多 limit 个事件；factory 为对象存储边界，返回领取数量及首个错误。
// worker 与回归测试使用，队列为空或外部失败即停止；每次调用保留 45 秒时限及租约语义。
func (s *Server) runArchiveBatch(ctx context.Context, limit int, factory func(*iam.ArchiveConfig) (archiveObjectStore, error)) (int, error) {
	count := 0
	for i := 0; i < limit; i++ {
		job, cancel := context.WithTimeout(ctx, 45*time.Second)
		ok, err := s.processArchive(job, factory)
		cancel()
		if ok {
			count++
		}
		if !ok || err != nil {
			return count, err
		}
	}
	return count, nil
}

// runArchiveCycle 读取最新配置并执行到期轮次；now/last 为调度时间，factory 为对象存储边界。
// 返回下次使用的轮次起点和错误；供生命周期消费者及真实数据库回归共用，不到期不领取事件。
// 到期后即便上传失败也推进轮次起点，避免两秒轮询绕过管理员配置的处理间隔。
func (s *Server) runArchiveCycle(ctx context.Context, now, last time.Time, factory func(*iam.ArchiveConfig) (archiveObjectStore, error)) (time.Time, error) {
	c, err := s.Identity().ArchiveConfiguration(ctx, 0)
	if err != nil {
		return last, err
	}
	interval, batch, valid := iam.ArchiveSchedule(c.IntervalSeconds, c.BatchSize)
	if !valid {
		return last, iam.ErrInvalid
	}
	if !archiveCycleDue(now, last, interval) {
		return last, nil
	}
	_, err = s.runArchiveBatch(ctx, batch, factory)
	return now, err
}

// archiveLoop 是服务生命周期内的后台消费者；ctx 取消退出，每两秒读取已保存的最新调度。
// 间隔从上次轮次开始计时、每轮串行处理配置上限；上传耗时可能延长周期，不承诺准点完成。
// 停用仅阻止新事件入队，已入队事件仍用其不可变版本归档；失败保留 outbox 并退避。
func (s *Server) archiveLoop(ctx context.Context) {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	last := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			last, _ = s.runArchiveCycle(ctx, time.Now(), last, func(target *iam.ArchiveConfig) (archiveObjectStore, error) {
				return newKodoArchive(target, s.Cfg.GeneralSettings.MasterKey)
			})
		}
	}
}

// archiveAdmin 管理配置、状态和失败重试；参数为 HTTP 请求响应，必须平台管理员会话。
// secret 不回显，保存不依赖七牛在线；测试连接创建并删除随机探针，失败返回固定错误。
func (s *Server) archiveAdmin(w http.ResponseWriter, r *http.Request) {
	p := s.RequireManage(w, r)
	if p == nil {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	by := iam.Actor{ID: p.UserID, Kind: "session"}
	if strings.HasSuffix(r.URL.Path, "/retry") {
		if err := s.Identity().RetryArchive(r.Context(), by); err != nil {
			s.WriteIAMError(w, r, err)
			return
		}
		util.WriteJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	c, err := s.Identity().ArchiveConfiguration(r.Context(), 0)
	if strings.HasSuffix(r.URL.Path, "/test") {
		if err != nil {
			util.WriteError(w, 400, "archive_not_configured", "Archive not configured")
			return
		}
		k, err := newKodoArchive(c, s.Cfg.GeneralSettings.MasterKey)
		if err == nil {
			random, e := enterpriseRandom()
			err = e
			if err == nil {
				key := c.Prefix + "/probes/" + random
				err = k.Put(r.Context(), key, []byte("XHub archive connection probe"))
				cleanErr := k.Delete(r.Context(), key)
				if err == nil {
					err = cleanErr
				}
			}
		}
		if err != nil {
			util.WriteError(w, 502, "archive_test_failed", "Archive connection test failed")
			return
		}
		util.WriteJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	if r.Method == http.MethodGet {
		if err != nil && !errors.Is(err, iam.ErrNotFound) {
			s.WriteIAMError(w, r, err)
			return
		}
		counts, events, e := s.Identity().ArchiveStatus(r.Context())
		if e != nil {
			s.WriteIAMError(w, r, e)
			return
		}
		util.WriteJSON(w, 200, map[string]any{"configuration": c, "counts": counts, "events": events})
		return
	}
	var in struct {
		iam.ArchiveConfig
		AK string `json:"access_key"`
		SK string `json:"secret_key"`
	}
	if !directoryDecode(r, &in) || !regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}$`).MatchString(in.Bucket) || !regexp.MustCompile(`^[a-zA-Z0-9/_-]{1,120}$`).MatchString(in.Prefix) || strings.HasPrefix(in.Prefix, "/") || strings.HasSuffix(in.Prefix, "/") {
		util.WriteError(w, 400, "invalid_request", "Invalid archive configuration")
		return
	}
	if _, ok := storage.GetRegionByID(storage.RegionID(in.Region)); !ok {
		util.WriteError(w, 400, "invalid_request", "Invalid Kodo region")
		return
	}
	interval, batch, valid := iam.ArchiveSchedule(in.IntervalSeconds, in.BatchSize)
	if !valid {
		util.WriteError(w, 400, "invalid_request", "Invalid archive schedule")
		return
	}
	in.IntervalSeconds, in.BatchSize = interval, batch
	if in.AK == "" && in.SK == "" && err == nil {
		in.Credentials = c.Credentials
	} else {
		if in.AK == "" || in.SK == "" {
			util.WriteError(w, 400, "invalid_request", "Both archive credentials are required")
			return
		}
		plain, _ := json.Marshal(map[string]string{"AK": in.AK, "SK": in.SK})
		in.Credentials, err = enterpriseSecret(s.Cfg.GeneralSettings.MasterKey, string(plain), false)
		if err != nil {
			util.WriteError(w, 400, "invalid_request", "A stable encryption key is required")
			return
		}
	}
	if err := s.Identity().SaveArchiveConfiguration(r.Context(), by, in.ArchiveConfig); err != nil {
		s.WriteIAMError(w, r, err)
		return
	}
	util.WriteJSON(w, 200, map[string]bool{"ok": true})
}

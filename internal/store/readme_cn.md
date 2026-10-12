# 框架 PostgreSQL 存储

[English](readme.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

启动时 Open 创建框架表，宿主共享 Store。调用业务 handler 前先授权，然后读写对应命名空间；退出时关闭 Engine。身份、成员和账务使用 IAM。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| Open(dsn) | 连接 PostgreSQL 并同步框架表 |
| GetKV(kind, id) | 读取 JSON 文档；缺失 sql.ErrNoRows |
| PutKV(kind, id, json) | 写入文档并使缓存失效 |

### 接入 demo

这是供宿主调用的 Go 接入函数。将代码放在仓库内独立包中可编译；实际执行前须由调用方提供下述真实依赖。

```go
package demo

import "github.com/sunqirui1987/xhub/internal/store"

// ReadDemoRecord 演示已授权文档读取；传入专用数据库和命名空间，返回记录或错误。
// Open 会同步表结构；不修改业务记录，结束关闭 Engine。
func ReadDemoRecord(dsn, kind, id string) (map[string]any, error) {
	st, err := store.Open(dsn)
	if err != nil {
		return nil, err
	}
	defer st.Engine.Close()
	return st.GetKV(kind, id)
}
```

函数可编译；运行需专用 PostgreSQL，未存在的记录返回 sql.ErrNoRows。Open 有表结构写入副作用。

### 错误、状态与副作用

GetKV 绕过缓存以获得安全敏感的最新值；Store 不检查租户权限。不要用 demo 命名空间覆盖用户已有数据。

## 说明

`internal/store` 持久化 IAM 之外的网关框架记录：通用 JSON 文档、控制台管理的模型部署和带命名空间的配置。只接受 `postgres://` 或 `postgresql://`；空串、SQLite、file 和未知协议都在启动时失败。身份、密钥、成员关系、审计和用量由 `internal/iam` 管理。

Store 在 xorm 上配置 10,000 项内存 LRU。写入后同时清理带 schema 和不带 schema 的缓存项。安全敏感的 `GetKV` 绕过缓存，使其他进程刚撤销的会话或凭据立即可见。SQL 可能含密钥哈希和凭据，因此 xorm SQL 日志始终关闭。

## 对外接口

### 打开与通用文档

- `Open(databaseURL) (*Store, error)` 验证 PostgreSQL，可选创建安全的 `search_path` schema，执行 ping，并同步 `kv`、`proxy_models`、`proxy_config`。连接、建 schema、同步失败会关闭半成品引擎并返回错误；主要调用方是网关启动。
- `PutKV(kind, id, body)` 更新或插入原始 JSON 文本并清缓存。`GetKV` 返回对象，缺少 `id` 时补入；损坏 JSON 以 `body` 保留；nil store 或缺行返回 `sql.ErrNoRows`。`DeleteKV` 对缺行也返回 `sql.ErrNoRows`。`ListKV(kind)` 解码同类全部行，nil store 视为空。会话、凭据、邀请、护栏/配置文档和 UI 设置使用这些接口。

### 模型与命名空间配置

- `ProxyModel` 是公开模型记录。`UpsertProxyModel`、`ListProxyModels`、`DeleteProxyModel` 编解码 params/info 并清模型缓存；损坏 JSON 读为空 map。网关组装目录时数据库行覆盖同名 YAML 部署。
- `PutConfig(namespace, key, value)` JSON 编码并 upsert。`ListConfig(namespace)` 解码全部值，损坏 JSON 回退原始字符串；nil store 返回空 map，使 YAML 保持权威。`DeleteConfig` 删除一行。模型价格刷新、路由默认值/组和偏好设置是直接调用方。
- `SaveModelDirectory(entry, deletedID, directory)` 在管理接口事务中增删部署，并清理模型默认值、全局路由组及 IAM 路由模板中的失效引用。序列化、损坏 JSON、查询、锁或提交任一失败都回滚；提交后清缓存，不修改内存目录。启动加载只读当前结构，不执行旧数据升级或修复。

### 实际 HTTP 边界

本包不注册路由。`gateway/models` 通过 `POST /model/new`、`/model/update`、`/model/delete`、启停及内置模型路由，`PUT /model/default`、`PUT /model/fallback`，以及 `GET|POST|PUT|DELETE /routing/groups` 暴露其效果。`gateway/prefs` 使用配置行实现 `GET /config/list`、`POST /config/update`、`POST /config/field/update`、`POST /config/field/delete`。handler 负责鉴权、校验、发布内存状态和 HTTP 错误映射。

## 内部核心实现

### 引擎与缓存一致性

`ensureSchema` 读取 `search_path`，经 `quoteIdent` 拒绝空、含符号或超过 63 字节的名称，用管理连接创建 schema，再由 `dsnWithSearchPath` 写入 PostgreSQL `options=-c search_path=...`。xorm 映射私有行结构并同步表，不追加约束。`quietLog` 实现全部 xorm logger 方法但不输出 SQL。`bust` 在修改后清理两种表名的 ID 与 bean 缓存；`GetKV.NoCache()` 是保证跨进程撤销的例外。

### 模型目录原子清理

`SaveModelDirectory` 使用一个 xorm 事务。先写部署，再扫描 `model_defaults`：剩余部署为零或一时删除默认权重，多部署时清理分配；随后通过 router 清理每个 `routing_groups`。若 IAM 模板表存在，则用 PostgreSQL `FOR UPDATE` 锁定全部 `route_templates` 并重写权重。损坏的历史 JSON 是硬错误，避免部署删除已提交而引用只清了一部分。

### 依赖与验证

本包依赖 PostgreSQL/pgx、xorm、`internal/config`、`internal/router`、`internal/iam`、`internal/util`。`model_weights_test.go` 是直接单元/回归测试，覆盖事务清理、损坏 JSON 回滚、默认值/路由组/模板及缓存可见结果；更高层 gateway 测试覆盖 KV、模型和偏好调用。数据库用例需要测试 PostgreSQL，并通过事务或隔离 schema 清理数据。

## 源码职责与入口

### `store.go`

定义 `Store` 和 `Open`，执行 PostgreSQL-only 配置检查并委托创建完整引擎。

### `engine.go`

实现 `quietLog`、`openEngine`、schema 创建、search-path DSN 改写、ping、LRU 安装及表同步；错误路径关闭已打开资源。

### `pg.go`

提供私有 `quoteIdent`，在动态 `CREATE SCHEMA` 前严格校验 PostgreSQL 标识符。

### `beans.go`

定义 `kv`、`proxy_models`、`proxy_config` 的私有 xorm 行和表映射，包括复合主键顺序及 JSON/text 列。

### `crud.go`

实现通用 KV 增改查删、复合 ID、缺行归一化、JSON 回退和缓存失效。

### `models.go`

定义公开 `ProxyModel` 和读取控制台部署时的 trace 入口，持久层行结构保持私有。

### `keys.go`

实现模型与命名空间配置 CRUD、JSON 转换、nil-store 降级、时间戳及缓存失效。

### `model_weights.go`

实现 `SaveModelDirectory`：跨 store 与可选 IAM 表的事务式部署修改和引用清理、PostgreSQL 显式行锁、回滚边界及提交后清缓存。

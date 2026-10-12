// Package contract 数据面与网关之间的能力契约。执行器只依赖这些接口，具体鉴权、缓存、凭据与持久化仍由 gateway 实现。
// RoutePlan/RouteTarget 描述请求范围内的会话选择，CallNote/SpendLog 描述一次交互的日志和结算边界；归属、授权与结算不能混为一体。
package contract

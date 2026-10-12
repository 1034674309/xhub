/** 用途：验证鉴权错误契约；参数为 HTTP 状态、JSON 错误体、预期状态及可选密钥生命周期错误类型，返回无。
 * 浏览器权限场景调用；401/403 与隐藏资源的 404 必须由调用方明确选择，500、非 JSON 和错误类型不匹配均抛错，无副作用。 */
export function requireAuthorizationError(status: number, body: unknown, expected: 401 | 403 | 404, expectedType?: "key_blocked" | "key_revoked" | "key_expired"): void {
  const type = expectedType ?? ({401: "invalid_api_key", 403: "forbidden", 404: "not_found"})[expected];
  const error = (body as {error?: {type?: unknown; code?: unknown; message?: unknown}})?.error;
  if (status !== expected || error?.type !== type || String(error?.code) !== String(expected) || typeof error?.message !== "string" || !error.message) {
    throw new Error(`鉴权契约不符：预期 ${expected}/${type}，实际 ${status}/${JSON.stringify(body)}`);
  }
}

/** 用途：验证真实页面导航成功；参数为响应状态、访问路径与最终 URL，返回无。
 * 路由巡检调用；缺响应、404、500、未解析参数与登录降级均失败，登录页面允许自身 URL，无写副作用。 */
export function requirePageSuccess(status: number | undefined, route: string, finalURL: string): void {
  if (status !== 200 || /\[[^\]]+\]/.test(route) || (route !== "/login" && /\/login(?:[/?#]|$)/.test(finalURL))) {
    throw new Error(`页面契约不符：${route} HTTP ${status ?? "无响应"}，最终 ${finalURL}`);
  }
}

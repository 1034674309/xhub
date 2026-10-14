export interface ApiBaseInputs {
  /**
   * Absolute API origin to target instead of same-origin. Comes from the
   * NEXT_PUBLIC_BASE_URL build env (dev / split-origin) or the proxy_base_url
   * the backend reports in its UI config. Empty/unset means same-origin.
   */
  explicitBase?: string | null;
  /**
   * The proxy's mount path when it sits under a sub-path (e.g. "/litellm"
   * behind a reverse proxy). "/" or empty means mounted at the root.
   */
  serverRootPath?: string | null;
}

export const normalizeRootPath = (serverRootPath: string | null | undefined): string => {
  const trimmed = (serverRootPath ?? "").trim();
  if (trimmed === "" || trimmed === "/") return "";
  const withLeadingSlash = trimmed.startsWith("/") ? trimmed : `/${trimmed}`;
  return withLeadingSlash.replace(/\/+$/, "");
};

/**
 * Resolve the single API base string that every request is prefixed with.
 *
 * Same-origin is represented as "" so requests stay relative and work behind
 * any domain or reverse proxy without hardcoding an origin. An explicit base is
 * used verbatim (trailing slash trimmed). The server root path is appended
 * unless the base already ends with it.
 */
/** Local OpenAI-compatible API origin. The console on port 3000 is not this URL. */
export const LOCAL_GATEWAY_ORIGIN = "http://localhost:4000";

export interface GatewayApiBaseInputs {
  /** NEXT_PUBLIC_BASE_URL. Empty means unset. */
  envBase?: string | null;
  /** proxy_base_url from the gateway UI config. Empty means unset. */
  reportedBase?: string | null;
  /**
   * Origin of the page. Accepted so callers can pass it, and ignored:
   * the page origin is never the API base.
   */
  pageOrigin?: string | null;
}

/**
 * Choose the gateway API origin. A non-empty reported base wins, then a
 * non-empty env base, then the local gateway origin. The page origin is not
 * a candidate.
 */
export const resolveGatewayApiBase = ({ envBase, reportedBase }: GatewayApiBaseInputs): string => {
  const reported = (reportedBase ?? "").trim().replace(/\/+$/, "");
  if (reported) return reported;
  const env = (envBase ?? "").trim().replace(/\/+$/, "");
  if (env) return env;
  return LOCAL_GATEWAY_ORIGIN;
};

/** 解析控制台请求地址；参数为构建覆盖地址与服务端报告地址，返回外部地址或同源 /gw。
 * 网络层和 typed client 初始化调用；保留显式覆盖优先级，未配置时通过 Next 代理，避免发布镜像绑定访问者的 localhost。
 * 不用于生成客户端示例；示例继续使用对外网关地址，无状态副作用。
 */
export const resolveConsoleApiBase = ({ envBase, reportedBase }: GatewayApiBaseInputs): string => {
  if ((reportedBase ?? "").trim() || (envBase ?? "").trim()) {
    return resolveGatewayApiBase({ envBase, reportedBase });
  }
  return "/gw";
};

export interface ClientSampleSettings {
  PROXY_BASE_URL?: string | null;
  LITELLM_UI_API_DOC_BASE_URL?: string | null;
}

/**
 * Base URL copied into OpenAI and curl samples. A doc base wins, then an
 * explicit proxy base, then the public gateway default (never the console /gw proxy).
 */
export const clientSampleBaseUrl = (settings?: ClientSampleSettings | null, envBase?: string | null): string => {
  const doc = (settings?.LITELLM_UI_API_DOC_BASE_URL ?? "").trim();
  if (doc) return doc.replace(/\/+$/, "");
  const proxy = (settings?.PROXY_BASE_URL ?? "").trim();
  if (proxy) return proxy.replace(/\/+$/, "");
  return resolveGatewayApiBase({ envBase });
};

export const resolveApiBase = ({ explicitBase, serverRootPath }: ApiBaseInputs): string => {
  const base = (explicitBase ?? "").trim().replace(/\/+$/, "");
  const rootPath = normalizeRootPath(serverRootPath);
  if (rootPath === "" || base.endsWith(rootPath)) return base;
  return `${base}${rootPath}`;
};

export interface RequestUrlInputs {
  /** Base registered at runtime (a split-origin proxy or worker URL); empty means none. */
  registeredBase?: string | null;
  /** Origin of the page issuing the request; the same-origin fallback. */
  pageOrigin?: string | null;
}

/** 组合请求路径与运行时地址，供 typed client 创建 Request；参数为路径、注册地址和页面源。
 * 返回可用 URL；/gw 等相对代理需要页面源补全，避免原生 Request 拒绝相对 URL，无网络或存储副作用。
 */
export const resolveRequestUrl = (path: string, { registeredBase, pageOrigin }: RequestUrlInputs): string => {
  const base = (registeredBase || pageOrigin || "").replace(/\/+$/, "");
  const target = `${base}${path}`;
  return registeredBase?.startsWith("/") && pageOrigin ? `${pageOrigin.replace(/\/+$/, "")}${target}` : target;
};

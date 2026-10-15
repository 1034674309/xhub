import path from "path";
import { fileURLToPath } from "url";

/** @type {import('next').NextConfig} */
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const nextConfig = {
  output: "standalone",
  outputFileTracingRoot: path.resolve(__dirname, ".."),
  outputFileTracingIncludes: { "/docs/[[...slug]]": ["../docs/api/**/*.md"] },
  distDir: process.env.E2E_BUILD_DIR || ".next",
  typescript: { tsconfigPath: process.env.E2E_BUILD_DIR ? "tsconfig.e2e.json" : "tsconfig.json" },
  experimental: {
    useTypeScriptCli: false,
    // 推理请求可能长时间没有首字节；覆盖框架默认的 30 秒代理超时，网关和调用方仍各自执行自己的截止时间。
    proxyTimeout: 600_000,
  },
  compiler: {
    removeConsole: process.env.NODE_ENV === "production" ? { exclude: ["error", "warn"] } : false,
  },
  images: {
    unoptimized: true,
  },
  basePath: "",
  assetPrefix: "",
  trailingSlash: false,
  allowedDevOrigins: ["127.0.0.1", "localhost"],
  async rewrites() {
    return [{ source: "/gw/:path*", destination: `${process.env.XHUB_GATEWAY_ORIGIN || "http://127.0.0.1:4000"}/:path*` }];
  },
  turbopack: {
    root: __dirname,
  },
};

export default nextConfig;

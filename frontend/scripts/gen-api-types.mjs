#!/usr/bin/env node
/**
 * 检查控制台 API 类型的真实生成来源。
 * XHub 后端已经迁移到 Go；当前仓库只有 method/path 路由目录，没有包含请求体、
 * 响应体和状态码的 OpenAPI 文档或 Go 生成命令，因此不能安全生成 schema.d.ts。
 */
import { existsSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

/**
 * 计算生成器使用的仓库路径。
 * 参数 scriptUrl 为当前脚本或测试夹具的 file URL；返回前端目录、仓库根目录、
 * 路由目录与生成文件路径。调用方用这些路径检查真实来源；函数不访问或修改文件。
 */
export function resolveGeneratorPaths(scriptUrl) {
  const dashboardDir = resolve(dirname(fileURLToPath(scriptUrl)), "..");
  const repoRoot = resolve(dashboardDir, "..");
  return {
    dashboardDir,
    repoRoot,
    routeCatalogPath: join(repoRoot, "internal", "catalog", "routes.json"),
    outputPath: join(dashboardDir, "src", "lib", "http", "schema.d.ts"),
  };
}

/**
 * 检查仓库当前可观察到的合同来源。
 * 参数 paths 来自 resolveGeneratorPaths，exists 可由单元测试注入；返回来源能力说明。
 * routes.json 只证明路由目录存在，不能作为 OpenAPI schema；函数无写入副作用。
 */
export function inspectContractSources(paths, exists = existsSync) {
  return { routeCatalogExists: exists(paths.routeCatalogPath), hasSchemaSource: false };
}

/**
 * 执行 API 类型来源检查。
 * 参数 scriptUrl 定位仓库，exists 供测试隔离文件系统；当前没有真实 Go OpenAPI
 * 来源时抛出可操作错误。不会启动服务、调用网络或改写生成文件。
 */
export function runGeneratorCheck(scriptUrl = import.meta.url, exists = existsSync) {
  const paths = resolveGeneratorPaths(scriptUrl);
  const sources = inspectContractSources(paths, exists);
  const catalog = sources.routeCatalogExists
    ? `检测到 ${paths.routeCatalogPath}，但它只包含 method/path，不能生成请求和响应类型。`
    : `未检测到路由目录 ${paths.routeCatalogPath}。`;
  throw new Error([
    "无法生成 src/lib/http/schema.d.ts：仓库中没有由 Go 后端产出的 OpenAPI 合同。",
    catalog,
    "已停止使用退役的 litellm.proxy.proxy_server；现有 schema.d.ts 保持不变。",
    "请先在 Go 源码中建立包含请求体、响应体和错误状态的 OpenAPI 生成链，再接入 openapi-typescript。",
  ].join("\n"));
}

if (process.argv[1] && pathToFileURL(resolve(process.argv[1])).href === import.meta.url) {
  try {
    runGeneratorCheck();
  } catch (error) {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  }
}

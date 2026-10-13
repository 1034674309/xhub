#!/usr/bin/env bash

# find_ca_certificate 选择构建镜像所需的主机 CA bundle。
# 参数为可选候选路径，测试和特殊发行版可显式传入；无参数时先使用 XHUB_CA_CERT_FILE，
# 否则依次检查 Debian/Ubuntu、RHEL 系和 macOS/Alpine 的常见路径。
# 返回值：成功时输出第一个可读且非空的文件并返回 0；没有可用文件时不输出并返回 1。
# 调用场景：deploy/build.sh 在复制证书前调用；函数只读文件元数据，无写入副作用。
find_ca_certificate() {
  local candidates=() candidate
  if [[ $# -gt 0 ]]; then
    candidates=("$@")
  elif [[ -n "${XHUB_CA_CERT_FILE:-}" ]]; then
    candidates=("$XHUB_CA_CERT_FILE")
  else
    candidates=(
      /etc/ssl/certs/ca-certificates.crt
      /etc/pki/tls/certs/ca-bundle.crt
      /etc/ssl/cert.pem
    )
  fi
  for candidate in "${candidates[@]}"; do
    if [[ -r "$candidate" && -s "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done
  return 1
}

# stage_console 将 Next.js standalone 产物整理到 Docker 控制台的运行目录。
# 参数依次为 standalone 目录、静态资源目录、可选 public 目录和输出目录。
# 返回值：成功返回 0；入口或静态资源缺失、输出目录为空或为根目录时返回 1。
# 调用场景：deploy/build.sh 在前端生产构建后调用；会替换输出目录，但校验失败不改已有产物。
# 保留追踪目录结构，兼容根入口及 outputFileTracingRoot 指向仓库时的 frontend 子目录入口。
stage_console() {
  local standalone="$1" static="$2" public="$3" destination="$4" app_relative=""
  if [[ -z "$destination" || "$destination" == / ]]; then
    echo "invalid console staging directory" >&2
    return 1
  fi
  if [[ -f "$standalone/server.js" ]]; then
    app_relative=""
  elif [[ -f "$standalone/frontend/server.js" ]]; then
    app_relative="/frontend"
  else
    echo "frontend standalone server was not produced" >&2
    return 1
  fi
  if [[ ! -d "$static" ]]; then
    echo "frontend static assets were not produced" >&2
    return 1
  fi
  rm -rf "$destination"
  mkdir -p "$destination"
  cp -a "$standalone/." "$destination/"
  mkdir -p "$destination$app_relative/.next"
  cp -a "$static" "$destination$app_relative/.next/static"
  if [[ -d "$public" ]]; then
    cp -a "$public" "$destination$app_relative/public"
  fi
  if [[ -n "$app_relative" ]]; then
    # 不能直接扁平化 frontend：上层 node_modules 和追踪的文档仍需保持相对位置。
    # 根入口只转交启动，使现有 Docker ENTRYPOINT 无需感知 Next 的输出目录布局。
    printf '%s\n' '// 控制台运行入口：转交仓库追踪布局中的 Next.js 服务，无额外参数或返回值。' \
      'require("./frontend/server.js");' > "$destination/server.js"
  fi
}

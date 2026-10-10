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

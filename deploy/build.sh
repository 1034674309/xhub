#!/usr/bin/env bash
# Compile the gateway and the console on this machine, then pack those
# artifacts into images. The Dockerfile does not see the source tree.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
source "$ROOT/deploy/build-lib.sh"

TARGET_OS=linux
case "$(uname -m)" in
  arm64|aarch64) TARGET_ARCH=arm64; NODE_ARCH=arm64 ;;
  x86_64|amd64) TARGET_ARCH=amd64; NODE_ARCH=x64 ;;
  *) echo "unsupported arch: $(uname -m)" >&2; exit 1 ;;
esac
NODE_VERSION="${NODE_VERSION:-24.14.1}"
GATEWAY_BINARY="bin/xhub-${TARGET_OS}-${TARGET_ARCH}"
CONSOLE_STAGE="bin/console"
NODE_BINARY="bin/node-${TARGET_OS}-${TARGET_ARCH}"

echo "[build] gateway ${GATEWAY_BINARY}"
mkdir -p bin
CGO_ENABLED=0 GOOS="$TARGET_OS" GOARCH="$TARGET_ARCH" \
  go build -trimpath -ldflags="-s -w" -o "$GATEWAY_BINARY" ./cmd/gateway

echo "[build] console"
(
  cd frontend
  if [ ! -d node_modules ]; then
    npm ci
  fi
  NEXT_TELEMETRY_DISABLED=1 \
    NEXT_PUBLIC_BASE_URL="${NEXT_PUBLIC_BASE_URL:-}" \
    npm run build
)

# Next 追踪根目录可能生成根入口或 frontend 子目录入口；统一打包后交 Docker 启动。
stage_console frontend/.next/standalone frontend/.next/static frontend/public "$CONSOLE_STAGE"

if [ ! -s "$NODE_BINARY" ]; then
  echo "[build] node linux/${TARGET_ARCH} ${NODE_VERSION}"
  tmp="$(mktemp -d)"
  curl -fsSL "https://npmmirror.com/mirrors/node/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${NODE_ARCH}.tar.xz" \
    -o "$tmp/node.tar.xz"
  tar -xJf "$tmp/node.tar.xz" -C "$tmp"
  cp "$tmp/node-v${NODE_VERSION}-linux-${NODE_ARCH}/bin/node" "$NODE_BINARY"
  chmod 755 "$NODE_BINARY"
  rm -rf "$tmp"
fi

if CA_CERTIFICATE_SOURCE="$(find_ca_certificate)"; then
  cp "$CA_CERTIFICATE_SOURCE" bin/ca-certificates.crt
else
  echo "missing readable CA bundle; checked XHUB_CA_CERT_FILE or common Linux/macOS paths" >&2
  exit 1
fi

echo "[build] image xhub-gateway"
docker build --target gateway \
  --build-arg "GATEWAY_BINARY=${GATEWAY_BINARY}" \
  -t xhub-gateway .

echo "[build] image xhub-console"
docker build --target console \
  --build-arg "NODE_BINARY=${NODE_BINARY}" \
  -t xhub-console .

echo "built xhub-gateway and xhub-console from ${GATEWAY_BINARY} and ${CONSOLE_STAGE}"

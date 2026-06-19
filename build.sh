#!/usr/bin/env bash
# 把 server 與 client 交叉編譯成 Linux / macOS / Windows 的單一執行檔。
# Go 內建交叉編譯，不需要任何額外工具鏈。
set -euo pipefail
cd "$(dirname "$0")"

OUT=dist
rm -rf "$OUT"
mkdir -p "$OUT"

# 平台清單：GOOS/GOARCH。涵蓋常見桌面與伺服器架構。
platforms=(
  "linux amd64"
  "linux arm64"
  "darwin amd64"
  "darwin arm64"
  "windows amd64"
  "windows arm64"
)

for app in server client; do
  for p in "${platforms[@]}"; do
    set -- $p
    goos=$1; goarch=$2
    ext=""
    [ "$goos" = "windows" ] && ext=".exe"
    name="$OUT/${app}-${goos}-${goarch}${ext}"
    echo "編譯 $name"
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
      go build -trimpath -ldflags "-s -w" -o "$name" "./cmd/$app"
  done
done

echo "完成，輸出在 $OUT/"
ls -la "$OUT"

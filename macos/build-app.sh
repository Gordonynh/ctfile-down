#!/usr/bin/env bash
# 构建 macOS .app（通用二进制：arm64 + amd64）
set -euo pipefail

cd "$(dirname "$0")/.."   # 仓库根目录
ROOT="$(pwd)"
VERSION="${VERSION:-$(git describe --tags --always 2>/dev/null || echo 0.2.0)}"
VERSION="${VERSION#v}"

APP="$ROOT/dist/ctfile-down.app"
BUILD="$ROOT/.build"
mkdir -p "$BUILD/tmp" "$BUILD/modulecache"
# 临时目录与 Clang 模块缓存放在工作区内，避免沙箱/权限问题。
export TMPDIR="$BUILD/tmp"
MODCACHE="$BUILD/modulecache"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "==> 构建 Go 服务端 (darwin universal)"
LDFLAGS="-s -w -X main.version=$VERSION"
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o "$TMP/server-arm64" .
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o "$TMP/server-amd64" .
lipo -create -output "$TMP/ctfile-down-server" "$TMP/server-arm64" "$TMP/server-amd64"

echo "==> 编译 Swift 启动器 (universal)"
swiftc -O -module-cache-path "$MODCACHE" -target arm64-apple-macos11.0 \
    -framework Cocoa -framework WebKit -o "$TMP/launcher-arm64" macos/main.swift
swiftc -O -module-cache-path "$MODCACHE" -target x86_64-apple-macos11.0 \
    -framework Cocoa -framework WebKit -o "$TMP/launcher-amd64" macos/main.swift
lipo -create -output "$TMP/ctfile-down" "$TMP/launcher-arm64" "$TMP/launcher-amd64"

echo "==> 准备图标"
# 优先使用仓库内已提交的 icon.icns（CI 无需 Pillow），否则现场生成。
if [ -f "$ROOT/macos/icon.icns" ]; then
    cp "$ROOT/macos/icon.icns" "$TMP/icon.icns"
else
    PY="${PYTHON:-python3}"
    "$PY" macos/make_icon.py "$TMP/icon.icns" >/dev/null
fi

echo "==> 组装 .app"
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp "$TMP/ctfile-down"        "$APP/Contents/MacOS/ctfile-down"
cp "$TMP/ctfile-down-server" "$APP/Contents/Resources/ctfile-down-server"
cp "$TMP/icon.icns"          "$APP/Contents/Resources/icon.icns"
sed "s/__VERSION__/$VERSION/g" macos/Info.plist > "$APP/Contents/Info.plist"
printf 'APPL????' > "$APP/Contents/PkgInfo"

echo "==> Ad-hoc 签名"
codesign --force --deep --sign - "$APP" 2>&1 | sed 's/^/    /'

echo "==> 完成: $APP"
du -sh "$APP"
lipo -info "$APP/Contents/MacOS/ctfile-down" | sed 's/^/    /'

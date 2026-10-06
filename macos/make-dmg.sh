#!/usr/bin/env bash
# 把 dist/ctfile-down.app 打包成 DMG（含 Applications 软链接，拖拽即装）
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"
VERSION="${VERSION:-$(git describe --tags --always 2>/dev/null || echo 0.2.0)}"
VERSION="${VERSION#v}"

APP="$ROOT/dist/ctfile-down.app"
DMG="$ROOT/dist/ctfile-down_${VERSION}_darwin_universal.dmg"

[ -d "$APP" ] || { echo "缺少 $APP，请先运行 macos/build-app.sh" >&2; exit 1; }

STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

cp -R "$APP" "$STAGE/"
ln -s /Applications "$STAGE/Applications"

rm -f "$DMG"
echo "==> 打包 DMG"
hdiutil create \
    -volname "ctfile-down" \
    -srcfolder "$STAGE" \
    -ov -format UDZO \
    "$DMG" >/dev/null

echo "==> 完成: $DMG"
ls -lh "$DMG"

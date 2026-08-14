#!/usr/bin/env bash
# Assemble the macOS release: frontend build, Go binary with embedded assets,
# .app bundle, optional codesign/notarize, and the dmg installer.
#
# There is no `wails build` step: Wails v3 serves the embedded frontend/dist
# from the plain Go binary, so bundling is just "put the binary in a .app".
#
# Usage:
#   ./scripts/bundle.sh              # host arch, sign/notarize when env present
#   VERSION=1.2.3 ./scripts/bundle.sh
#   UNIVERSAL=1 ./scripts/bundle.sh  # arm64+amd64 via lipo
#
# Signing/notarization env (all optional; unsigned local builds just skip it):
#   CODESIGN_IDENTITY   e.g. "Developer ID Application" — enables codesigning
#   APPLE_ID APPLE_TEAM_ID APPLE_APP_PASSWORD — enable notarization+staple
set -euo pipefail
cd "$(dirname "$0")/.."

BIN_DIR="apps/desktop/build/bin"
APP="$BIN_DIR/option-tab.app"
PLIST="apps/desktop/build/darwin/Info.plist"
VERSION="${VERSION:-$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$PLIST")}"
# Asset name follows the @option-tab/shared releaseAssetName contract that the
# landing page builds download links with.
DMG_NAME="option-tab_${VERSION}_darwin_arm64.dmg"

echo "==> bundling option-tab $VERSION"

# 1. Frontend (embedded into the binary via //go:embed frontend/dist).
(cd apps/desktop/frontend && bun install --frozen-lockfile && bun run build)

# 2. Binary. Regenerate bindings when the wails3 CLI is available; the
#    committed frontend/bindings are the fallback.
cd apps/desktop
(wails3 generate bindings || echo 'wails3 not installed, using committed bindings')
mkdir -p build/bin
if [ "${UNIVERSAL:-0}" = "1" ]; then
  # CGO_ENABLED=1 is explicit: cgo defaults OFF when GOARCH differs from the
  # host, which would silently drop the AX/CGEventTap platform layer.
  CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -o build/bin/option-tab-arm64 .
  CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -o build/bin/option-tab-amd64 .
  lipo -create -output build/bin/option-tab build/bin/option-tab-arm64 build/bin/option-tab-amd64
  rm build/bin/option-tab-arm64 build/bin/option-tab-amd64
else
  CGO_ENABLED=1 go build -o build/bin/option-tab .
fi
cd ../..

# 3. Icon: render build/appicon.png into the .icns the bundle expects.
ICONSET="$(mktemp -d)/icon.iconset"
mkdir -p "$ICONSET"
for size in 16 32 128 256 512; do
  sips -z "$size" "$size" apps/desktop/build/appicon.png --out "$ICONSET/icon_${size}x${size}.png" >/dev/null
  sips -z "$((size * 2))" "$((size * 2))" apps/desktop/build/appicon.png --out "$ICONSET/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$ICONSET" -o "$BIN_DIR/iconfile.icns"

# 4. Bundle.
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp "$PLIST" "$APP/Contents/Info.plist"
cp "$BIN_DIR/option-tab" "$APP/Contents/MacOS/option-tab"
cp "$BIN_DIR/iconfile.icns" "$APP/Contents/Resources/"

# 5. Sign (hardened runtime) when an identity is available.
if [ -n "${CODESIGN_IDENTITY:-}" ]; then
  codesign --force --deep --options runtime --timestamp --sign "$CODESIGN_IDENTITY" "$APP"
  codesign --verify --deep --strict --verbose=2 "$APP"
else
  echo "==> CODESIGN_IDENTITY unset: skipping codesign"
fi

# 6. DMG (styled drag-to-Applications window; see build/darwin/dmg/appdmg.json).
(cd apps/desktop && npx --yes appdmg build/darwin/dmg/appdmg.json "build/bin/$DMG_NAME")

# 7. Sign, notarize, and staple the DMG when Apple credentials are available.
if [ -n "${CODESIGN_IDENTITY:-}" ] && [ -n "${APPLE_ID:-}" ]; then
  codesign --force --timestamp --sign "$CODESIGN_IDENTITY" "$BIN_DIR/$DMG_NAME"
  xcrun notarytool submit "$BIN_DIR/$DMG_NAME" --apple-id "$APPLE_ID" --team-id "$APPLE_TEAM_ID" \
    --password "$APPLE_APP_PASSWORD" --wait
  # staple fails if notarization did not fully succeed, failing the script.
  xcrun stapler staple "$BIN_DIR/$DMG_NAME"
  xcrun stapler validate "$BIN_DIR/$DMG_NAME"
else
  echo "==> APPLE_ID/CODESIGN_IDENTITY unset: skipping dmg notarization"
fi

echo "==> done: $BIN_DIR/$DMG_NAME"

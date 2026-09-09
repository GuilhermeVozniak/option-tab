#!/usr/bin/env bash
# Release: VERSION=1.2.3 ./scripts/bundle.sh (signing credentials required).
# Local: BUNDLE_MODE=unsigned ./scripts/bundle.sh (UNVERIFIED, never publish).
set -euo pipefail
cd "$(dirname "$0")/.."
BIN_DIR="apps/desktop/build/bin"
APP="$BIN_DIR/option-tab.app"
PLIST="apps/desktop/build/darwin/Info.plist"
BUNDLE_MODE="${BUNDLE_MODE:-release}"
case "$BUNDLE_MODE" in
  release)
    for required in VERSION CODESIGN_IDENTITY APPLE_ID APPLE_TEAM_ID APPLE_APP_PASSWORD; do
      [[ -n "${!required:-}" ]] || { echo "Release requires $required" >&2; exit 1; }
    done
    [[ "${UNIVERSAL:-1}" == 1 ]] || { echo "Releases require UNIVERSAL=1" >&2; exit 1; }
    UNIVERSAL=1
    ;;
  unsigned)
    VERSION="${VERSION:-$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$PLIST")}"
    UNIVERSAL="${UNIVERSAL:-0}"
    echo "UNVERIFIED LOCAL BUILD: unsigned and not notarized; do not distribute."
    ;;
  *) echo "BUNDLE_MODE must be release or unsigned" >&2; exit 1 ;;
esac
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([+-][A-Za-z0-9.-]+)?$ ]] || { echo "Invalid VERSION" >&2; exit 1; }
[[ "$UNIVERSAL" == 0 || "$UNIVERSAL" == 1 ]] || { echo "UNIVERSAL must be 0 or 1" >&2; exit 1; }
export MACOSX_DEPLOYMENT_TARGET=14.0
export CGO_CFLAGS="${CGO_CFLAGS:-} -mmacosx-version-min=14.0"
export CGO_CXXFLAGS="${CGO_CXXFLAGS:-} -mmacosx-version-min=14.0"
export CGO_LDFLAGS="${CGO_LDFLAGS:-} -mmacosx-version-min=14.0"

# Bindings must exist BEFORE the frontend embeds its generated bridge.
./scripts/generate-bindings.sh
(cd apps/desktop/frontend && bun install --frozen-lockfile && bun run build)

cd apps/desktop
mkdir -p build/bin
if [[ "$UNIVERSAL" == 1 ]]; then
  CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -o build/bin/option-tab-arm64 .
  CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -o build/bin/option-tab-amd64 .
  lipo -create -output build/bin/option-tab build/bin/option-tab-arm64 build/bin/option-tab-amd64
  expected_archs="arm64 x86_64"
  asset_arch=universal
else
  asset_arch="$(go env GOARCH)"
  case "$asset_arch" in
    arm64) expected_archs=arm64 ;;
    amd64) expected_archs=x86_64 ;;
    *) echo "Unsupported macOS architecture: $asset_arch" >&2; exit 1 ;;
  esac
  CGO_ENABLED=1 GOOS=darwin GOARCH="$asset_arch" go build -o build/bin/option-tab .
fi
# Inspect the produced binary rather than trusting requested compiler flags.
actual_archs="$(lipo -archs build/bin/option-tab | tr ' ' '\n' | LC_ALL=C sort | xargs)"
[[ "$actual_archs" == "$expected_archs" ]] || { echo "Architecture mismatch: $actual_archs (expected $expected_archs)" >&2; exit 1; }
for arch in $expected_archs; do
  build_info="$(xcrun vtool -arch "$arch" -show-build build/bin/option-tab)"
  [[ "$(awk '$1 == "minos" { print $2 }' <<< "$build_info")" == "14.0" ]] || { echo "Deployment floor mismatch for $arch" >&2; exit 1; }
done
cd ../..
DMG_NAME="option-tab_${VERSION}_darwin_${asset_arch}.dmg"
if [[ "$BUNDLE_MODE" == unsigned ]]; then DMG_NAME="option-tab_${VERSION}_darwin_${asset_arch}_UNVERIFIED.dmg"; fi

# 3. Icon: render build/appicon.png into the .icns the bundle expects.
ICON_TMP="$(mktemp -d)"
trap 'rm -rf "$ICON_TMP"' EXIT
ICONSET="$ICON_TMP/icon.iconset"
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
# Stamp the release version into the bundle. The committed plist is only the
# fallback VERSION source above; without this the bundle keeps whatever
# version was last hand-edited there (0.4.2 and 0.4.3 both shipped reporting
# 0.4.1 to Finder, Get Info and anything else reading the bundle).
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $VERSION" "$APP/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $VERSION" "$APP/Contents/Info.plist"
cp "$BIN_DIR/option-tab" "$APP/Contents/MacOS/option-tab"
cp "$BIN_DIR/iconfile.icns" "$APP/Contents/Resources/"
cp apps/desktop/build/darwin/OptionTab.sdef "$APP/Contents/Resources/OptionTab.sdef"

# Verify the assembled metadata, not just the committed template.
[[ "$(/usr/libexec/PlistBuddy -c 'Print :LSMinimumSystemVersion' "$APP/Contents/Info.plist")" == "14.0" ]] || { echo "Bundle minimum OS must be 14.0" >&2; exit 1; }
[[ "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$APP/Contents/Info.plist")" == "option-tab" ]] || { echo "Bundle executable mismatch" >&2; exit 1; }
[[ "$(/usr/libexec/PlistBuddy -c 'Print :OSAScriptingDefinition' "$APP/Contents/Info.plist")" == "OptionTab.sdef" && -s "$APP/Contents/Resources/OptionTab.sdef" ]] || { echo "Bundle scripting dictionary missing or mismatched" >&2; exit 1; }
for version_key in CFBundleShortVersionString CFBundleVersion; do
  [[ "$(/usr/libexec/PlistBuddy -c "Print :$version_key" "$APP/Contents/Info.plist")" == "$VERSION" ]] || { echo "Bundle version mismatch" >&2; exit 1; }
done

# 5. Sign the release app with hardened runtime.
if [[ "$BUNDLE_MODE" == release ]]; then
  codesign --force --deep --options runtime --entitlements apps/desktop/build/darwin/entitlements.plist --timestamp --sign "$CODESIGN_IDENTITY" "$APP"
  codesign --verify --deep --strict --verbose=2 "$APP"
else
  echo "==> UNVERIFIED: skipping codesign"
fi

# 6. DMG (styled drag-to-Applications window; see build/darwin/dmg/appdmg.json).
(cd apps/desktop && npx --yes appdmg build/darwin/dmg/appdmg.json "build/bin/$DMG_NAME")

# 7. Release success requires DMG signing, notarization and staple validation.
if [[ "$BUNDLE_MODE" == release ]]; then
  codesign --force --timestamp --sign "$CODESIGN_IDENTITY" "$BIN_DIR/$DMG_NAME"
  xcrun notarytool submit "$BIN_DIR/$DMG_NAME" --apple-id "$APPLE_ID" --team-id "$APPLE_TEAM_ID" \
    --password "$APPLE_APP_PASSWORD" --wait
  # staple fails if notarization did not fully succeed, failing the script.
  xcrun stapler staple "$BIN_DIR/$DMG_NAME"
  xcrun stapler validate "$BIN_DIR/$DMG_NAME"
else
  echo "==> UNVERIFIED: skipping notarization"
fi

echo "==> done: $BIN_DIR/$DMG_NAME"

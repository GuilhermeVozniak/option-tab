#!/bin/sh
set -eu
cd "$(dirname "$0")/../../.."
smoke_dir=$(mktemp -d /tmp/option-tab-dock-panel.XXXXXX)
fixture_pid=''
saved_pointer=''
cleanup() { if [ -n "$fixture_pid" ]; then kill "$fixture_pid" 2>/dev/null || true; wait "$fixture_pid" 2>/dev/null || true; fi; if [ -n "$saved_pointer" ]; then set -- $saved_pointer; "$smoke_dir/TextFixture.app/Contents/MacOS/fixture" --warp "$1" "$2" || true; fi; rm -rf "$smoke_dir"; }
trap cleanup EXIT INT TERM
mkdir -p "$smoke_dir/TextFixture.app/Contents/MacOS"
cat > "$smoke_dir/TextFixture.app/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>CFBundlePackageType</key><string>APPL</string><key>CFBundleExecutable</key><string>fixture</string><key>CFBundleIdentifier</key><string>com.optiontab.disposable-text-fixture</string><key>CFBundleName</key><string>Disposable Text Fixture</string><key>NSHighResolutionCapable</key><true/></dict></plist>
PLIST
clang -fobjc-arc -framework Cocoa -framework ApplicationServices internal/platform/testdata/dock_panel_text_fixture.m -o "$smoke_dir/TextFixture.app/Contents/MacOS/fixture"
mkdir -p "$smoke_dir/PanelSmoke.app/Contents/MacOS"
cat > "$smoke_dir/PanelSmoke.app/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>CFBundleExecutable</key><string>panel-smoke</string><key>CFBundleIdentifier</key><string>com.optiontab.disposable-panel-smoke</string><key>CFBundleName</key><string>Disposable Panel Smoke</string><key>NSHighResolutionCapable</key><true/><key>LSUIElement</key><true/></dict></plist>
PLIST
go build -o "$smoke_dir/PanelSmoke.app/Contents/MacOS/panel-smoke" ./internal/platform/testdata/dock_panel_smoke
saved_pointer=$("$smoke_dir/TextFixture.app/Contents/MacOS/fixture" --pointer)
"$smoke_dir/TextFixture.app/Contents/MacOS/fixture" >"$smoke_dir/fixture.log" 2>&1 &
fixture_pid=$!
DOCK_PANEL_SCREEN_INDEX="${DOCK_PANEL_SCREEN_INDEX:-0}" DOCK_PANEL_FIXTURE_PID="$fixture_pid" DOCK_PANEL_FIXTURE_LOG="$smoke_dir/fixture.log" "$smoke_dir/PanelSmoke.app/Contents/MacOS/panel-smoke"

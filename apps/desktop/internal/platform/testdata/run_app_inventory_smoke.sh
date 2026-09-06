#!/bin/sh
set -eu

desktop_dir=$(CDPATH= cd -- "$(dirname "$0")/../../.." && pwd)
smoke_dir=$(mktemp -d "${TMPDIR:-/tmp}/option-tab-app-smoke.XXXXXX")
app="$smoke_dir/InventorySmoke.app"
state="$smoke_dir/state"
trap 'if [ -f "$state" ]; then pid=$(awk "{print \$1}" "$state"); kill "$pid" 2>/dev/null || true; fi; rm -rf "$smoke_dir"' EXIT

mkdir -p "$app/Contents/MacOS"
cp "$desktop_dir/internal/platform/testdata/app_inventory_fixture.m" "$smoke_dir/fixture.m"
clang -fobjc-arc -framework Cocoa "$smoke_dir/fixture.m" -o "$app/Contents/MacOS/inventory-smoke"
cat > "$app/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>inventory-smoke</string>
<key>CFBundleIdentifier</key><string>com.optiontab.InventorySmoke</string>
<key>CFBundleName</key><string>Inventory Smoke</string>
<key>CFBundlePackageType</key><string>APPL</string>
</dict></plist>
PLIST
open -n "$app" --args "$state"
i=0
while [ ! -f "$state" ]; do
  i=$((i + 1)); [ "$i" -lt 100 ] || { echo "fixture did not close its window" >&2; exit 1; }
  sleep 0.05
done
cd "$desktop_dir"
OPTION_TAB_APP_SMOKE_STATE="$state" go test ./internal/platform -run TestApplicationNativeSmokeWindowlessInventoryAndExactActivation -count=1 -v

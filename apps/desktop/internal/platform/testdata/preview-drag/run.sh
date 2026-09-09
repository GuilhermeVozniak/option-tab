#!/bin/bash
set -euo pipefail
source_dir=$(cd "$(dirname "$0")" && pwd)
build_dir=$(mktemp -d /tmp/option-tab-preview-ax.XXXXXX)
fixture_pid=""
cleanup(){
 if [[ -n "$fixture_pid" ]];then kill "$fixture_pid" 2>/dev/null || true;wait "$fixture_pid" 2>/dev/null || true;fi
 rm -rf "$build_dir"
}
trap cleanup EXIT
app_dir="$build_dir/PreviewFixture.app"
mkdir -p "$app_dir/Contents/MacOS"
cat > "$app_dir/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>CFBundleIdentifier</key><string>com.optiontab.preview-drag-fixture</string><key>CFBundleExecutable</key><string>PreviewFixture</string><key>CFBundlePackageType</key><string>APPL</string><key>LSUIElement</key><true/></dict></plist>
PLIST
clang -fobjc-arc -framework Cocoa -framework ApplicationServices "$source_dir/fixture.m" -o "$app_dir/Contents/MacOS/PreviewFixture"
clang -fobjc-arc -framework Cocoa -framework ApplicationServices "$source_dir/driver.m" -o "$build_dir/driver"
foreground=$("$build_dir/driver" --foreground)
"$app_dir/Contents/MacOS/PreviewFixture" "$build_dir/state" > "$build_dir/fixture.log" 2>&1 &
fixture_pid=$!
for ((attempt=0;attempt<100;attempt++));do
 [[ -s "$build_dir/state" ]] && break
 kill -0 "$fixture_pid" 2>/dev/null || { cat "$build_dir/fixture.log";exit 1; }
 sleep 0.05
done
[[ -s "$build_dir/state" ]] || { echo "fixture readiness timed out" >&2;exit 1; }
read -r announced first second < "$build_dir/state"
[[ "$announced" == "$fixture_pid" ]] || { echo "fixture PID mismatch" >&2;exit 1; }
"$build_dir/driver" "$fixture_pid" "$first" "$second" "$foreground"
kill "$fixture_pid"
wait "$fixture_pid" 2>/dev/null || true
fixture_pid=""
[[ $("$build_dir/driver" --foreground) == "$foreground" ]] || { echo "fixture cleanup changed foreground" >&2;exit 1; }
echo "PASS fixture PID reaped; foreground preserved through launch and cleanup"

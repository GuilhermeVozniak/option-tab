#!/bin/sh
# Compile/package only. Never launches the resulting app.
set -eu
if [ "$#" -ne 1 ] || [ -e "$1" ]; then
  echo 'usage: build.sh /absolute/new/BadgeFixture.app' >&2
  exit 2
fi
case "$1" in /*.app) ;; *) exit 2 ;; esac
source_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
bundle=$1
identifier="org.optiontab.fixture.launcher-badges-live.$(/usr/bin/uuidgen | tr '[:upper:]' '[:lower:]')"
mkdir -p "$bundle/Contents/MacOS"
/usr/bin/xcrun clang -fobjc-arc -fblocks -mmacosx-version-min=14.0 -Wall -Wextra -Werror \
  -framework AppKit "$source_dir/fixture.m" -o "$bundle/Contents/MacOS/BadgeFixture"
cat > "$bundle/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>$identifier</string>
<key>CFBundleExecutable</key><string>BadgeFixture</string>
<key>CFBundleName</key><string>Option Tab Badge Fixture</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleVersion</key><string>1</string>
<key>LSMinimumSystemVersion</key><string>14.0</string>
<key>NSHighResolutionCapable</key><true/>
</dict></plist>
PLIST
/usr/bin/plutil -lint "$bundle/Contents/Info.plist" >/dev/null

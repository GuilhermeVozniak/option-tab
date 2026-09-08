# Manual launcher-input port probe — prepared, never automatically run

This test-only Wails app uses a real hidden host and production LauncherPanel,
LauncherGestureSource, and LauncherKeyboardSource. It is deliberately an interior
420×260 surface, **not** the production Dock route or edge geometry. It creates no
input taps of its own, never posts/warps input, activates no app, performs no app or
window action, and writes no preferences. Ordinary `go test ./...` ignores testdata.

## Compile only

From `apps/desktop` on macOS14+:

```sh
CGO_CFLAGS='-mmacosx-version-min=14.0' CGO_LDFLAGS='-mmacosx-version-min=14.0' go build -o /tmp/option-tab-launcher-input-live ./internal/platform/testdata/launcher-input-live
```

That command does not run the probe. No production capability flag is changed.

## Later coordinated manual run

Use an exact display UUID already established by the read-only launcher/display
inventory. No display is guessed. Do not run while another desktop acceptance
fixture owns the input slot. Run only when an interactive desktop session is
available; the --run-native flag is mandatory.

```sh
/tmp/option-tab-launcher-input-live --run-native --display DISPLAY-UUID --duration 90s
```

For reliable LaunchServices/Wails deployment, put that binary in a disposable
`LauncherInputProbe.app/Contents/MacOS/launcher-input-live` with Info.plist keys
`CFBundleExecutable=launcher-input-live`, `CFBundleIdentifier=com.optiontab.fixture.launcher-input-live`,
`CFBundlePackageType=APPL`, `LSUIElement=true`, `NSHighResolutionCapable=true`,
`LSMinimumSystemVersion=14.0`; invoke the bundled binary directly with the same
arguments. The app's activation policy is Accessory. Do not activate it through
`open -a`, add activation workarounds, or synthesize input to obtain a pass.

The observer requires an existing positively known ordinary display/Space and
cancels when its identity, bounds, scale or Space changes, becomes unknown, or
observation is unavailable/stale. A positively observed native Dock overlap also refuses/cancels the interior surface. Native physical panel admission is also checked
before policy setup and every250ms. A missing permission/unsupported display/failed
Wails readiness prints coarse REFUSED and cleans up; it never requests consent.

1. Wait for `READY portProbe=true ...`; confirm the visible surface is unobtrusive.
2. Manually scroll, pinch and swipe inside it. Output reports packet counts only.
3. Click **Enter keyboard mode** explicitly. Only this button asks the production
   native keyboard port for key permission, using a final current-context guard.
4. Type disposable text/IME composition. The bridge receives bounded UTF-8 but
   retains/logs no content; only admitted committed-event counts are reported.
   There is no Enter action and no selection/application integration.
5. Click **Leave keyboard mode**, blur or press native Escape; inspect coarse key
   permission status. Stop with **Stop probe**, Ctrl-C/SIGTERM, or lifetime expiry.

Duration is restricted to15–180seconds. Cleanup cancels and joins environment
observation, hides/closes the exact panel, joins its gesture worker, closes the
owned hidden Wails host and quits. Expected cleanup markers are
`panelClosed=true gestureJoined=true` and `environmentJoined=true`. A hard process
kill cannot provide join evidence. No cursor restoration is attempted because the
probe never moves it. It never activates a previous app to restore focus.

Wails internal logging is discarded to avoid logging RPC text; stdout contains
only coarse readiness/refusal, permission booleans, packet/event totals and cleanup.
No raw typed text, app/window names, paths, display identifiers or PIDs are output.
A successful run proves these native ports can deliver into a small Wails host on
that machine only. It does not prove full launcher action integration, physical
Dock edge layout, every device/system gesture, noninterference across all Spaces,
or justify turning on currently gated public capabilities by itself.

Focus preservation is **unverified** by this probe and is reported explicitly in its READY marker. The existing `ActiveApp` source intentionally substitutes the previous external application when this process is foreground, so it cannot prove foreground preservation. No new foreground observer is installed. During a separately coordinated manual run, record human observations of the foreground application before Show, after entering keyboard mode, after leaving/blur, and after cleanup; do not infer restoration from key permission or the cleanup marker. The probe never activates or restores another application.

Gesture counters require exact native gesture-token validation, current context/display admission, and fresh physical panel validation. Every fetched packet is acknowledged even when admission refuses it.

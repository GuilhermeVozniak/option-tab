# Replacement Dock foundation checkpoint

The optional replacement Dock now has an independently enabled runtime, per-display running-app launcher, explicit built-in clock grant, settings, exact-target activation and a permanent “Use native Dock” recovery action. It defaults off. Native Dock preferences and its process are not changed.

This is the first H01/H02/H08/H13 implementation slice. It does not complete the retained replacement Dock roadmap. Richer profiles, layouts and appearance, pinned items/groups, magnification, folder fan-out, app context menus, audio/widgets, community installation, gestures, supported badges and import/export remain planned. User confirmed continuation of the original full scope after the elapsed-time question; no scope reduction was approved.

## Behavior

- Logical display UUID bindings resolve independently; unknown/incomplete display, Space or native Dock evidence yields instead of placing an uncertain surface. An optional dynamically resolved read-only SkyLight adapter supplies per-display Space evidence.
- Native launcher hosts have their own nonactivating policy, token and display/Space checks. Activation resolves an opaque current item to exact PID/start/bundle identity and rechecks the App and native host immediately before dispatch.
- Native icon-input and monitor-lock owners retire admission and join before a replacement host can show. Pointer ownership yields native hover panels while keyboard switching and independently pinned media remain available.
- Disable, pause, preferences, switcher admission, host failure and shutdown retire old sessions. Recovery first hides/retires the launcher, then persists disabled; a failed save keeps a process-local disabled latch and visible error.
- The clock uses a digest-bound declarative row/text manifest and explicit per-instance grant. One shared timer runs only while a granted instance is visible. No downloaded code, capture or arbitrary provider dispatch is introduced.

## Verification

- Full Go race/coverage run passed across 22 packages. Focused launcher tests passed again after correcting two unchecked cancellation results; golangci-lint reports zero issues.
- Desktop Vitest: 294 tests passed; shared: 7; site: 4. The full run initially found three existing App fixtures missing the new launcher mock methods; those fixtures were corrected and the complete desktop suite rerun successfully.
- Desktop Chromium: 70 tests passed, including the new launcher route and settings/recovery scenarios. Repository Biome checks and frontend production builds passed.
- Native CGO builds passed for arm64 and x86_64; Mach-O inspection confirms macOS 14.0 minimum on both. These local unsigned binaries were not installed or distributed.
- App integration tests use real App/controller/host-wrapper wiring with fake native ports to verify source joins, exact activation, settings-publication gaps, host failure, pause/resume, recovery/shutdown and independent media pins. A separate narrow review found no concrete defect in source retirement receipt ordering.

## Native acceptance still outstanding

Native seam tests and compilation establish code paths, not physical desktop acceptance. A bounded host-only attempt created, showed, hid and closed its fixture with unchanged foreground, native Dock identity and preferences. Its physical visibility check failed and pointer context changed, so it stopped without retry or pointer restoration. Source inspection then found the fixture used `Prohibited` activation policy while production uses `Accessory`; the failed combined visibility result therefore does not establish production behavior. The ignored fixture was corrected and given individual visibility-gate logging, but was not rerun.

Actual ordinary-space visibility/click activation, two-display behavior, native Dock reachability, mixed scale, disconnect/reconnect, fullscreen/Mission Control and physical recovery remain unverified. The feature remains an unreleased draft checkpoint; no version, installed application or release was changed.

Detailed commands, original logs and limited native evidence are retained locally under the ignored `.superpowers/sdd/2026-09-07-replacement-dock/` directory.

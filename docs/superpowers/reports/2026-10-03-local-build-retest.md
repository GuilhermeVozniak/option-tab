# Local build retest

Native checks used `option-tab-local-cb3289a.app` on macOS 27.0 (26A428). They do not establish acceptance of subsequent source changes or complete the broader native matrix.

## Permission recovery

Accessibility became granted, but Screen Recording remained unavailable after restarting and toggling the existing Settings entry. Read-only native logs established that ScreenCapture still checked the current process against the previous ad-hoc build's designated requirement and failed with `-67050`. The Settings switch being on did not establish effective authorization.

With the user's explicit approval, the stale Screen Recording entry was removed and the exact current app bundle was added through System Settings. After Quit & Reopen, Preferences showed both permissions granted. TCC logs for the new process confirmed the current signature passed and ScreenCapture was Allowed. No code bypass or broad permission reset was used.

## Observed checks

- The replacement Dock displayed running application icons in its native panel.
- The Clock widget displayed the correct local time and advanced from 14:50 to 14:51. Its temporary enablement and local-time grant were restored to their original disabled state.
- Enabling launcher interactions made the pinch and swipe options available. Both settings persisted when enabled and were restored afterward. Physical gesture delivery remains waived by the user, not passed.
- Monitor lock reported Protected for the main display and Awaiting placement for the second display. The manual-placement instruction remained visible, and the unavailable automatic-placement hint was absent. Monitor lock was restored to disabled with the main-display target.

The only remaining settings-file difference after restoration was the Clock package digest refreshed by the current app. Evidence is retained privately under `.superpowers/sdd/native-retest-2026-10-03/`.

## Additional widget checks

A temporary profile exercised Audio, Battery and Network in the running app. Audio displayed the current output name. Its chooser was disabled without the optional capability; granting that capability exposed three available outputs. No output was selected or changed. Volume and mute readings remained unavailable for the current device, so those readings are not counted as passed.

Network displayed Connected and Ethernet with only status access. Enabling optional usage access produced changing transfer rates, including observed readings of 7087 B/s and 11136 B/s. Battery displayed unavailable; the Mac mini reported AC power without a battery, so actual battery readings remain untested.

Tall widget content extended beyond the native panel, and revealing the output chooser scrolled the page and clipped application icons. This provides a native reproduction for the widget-layout follow-up. The temporary profile was removed, its main-display assignment returned to the original profile, and the full settings JSON matched the pre-widget-test baseline exactly.

The source fix gives the launcher its native panel height and confines long content to each widget's scrolling area. Four-edge browser regressions cover multiple readings and a twelve-output chooser: controls remain reachable, document scrolling stays at zero, and application icons keep their positions. Existing narrow layouts, the four-slot viewport cap and magnification checks also passed. These checks do not replace a native retest of the rebuilt app.

## Launcher action visibility

Opening actions for the last running application exposed controls in the accessibility tree while leaving them outside the visible Dock strip. A browser regression reproduced this on all four edges before any automatic click scrolling. Opening actions now scrolls their container into the existing viewport. The test checks bounds and hit targets, invokes the visible Show all windows control, and verifies keyboard reopening and Escape dismissal.

This fix is a source change after `cb3289a`; it has not been tested in a rebuilt native application. The full desktop unit suite passed (596 tests), together with 30 related Chromium cases, all four new WebKit cases, TypeScript and Biome checks. The new geometry regression is also included in the existing WebKit CI configuration.

## Panel failures and language changes

An accepted Show all windows request could fail during asynchronous native-panel startup without reporting that failure to its parent Dock. The backend now publishes a scoped failure through the existing child update event before retiring the failed child. The parent displays a translated message and rejects updates from replaced items, different parents, closed children and retired sessions. A later clock/content revision does not immediately erase a valid failure. Normal dismissal remains distinct from startup failure.

The same message was clipped by a fixed height limit on vertical Docks. Its height now follows the available viewport. Four-edge browser tests deliver the real update/hide event shapes and verify that the entire message fits without moving the application strip. The combined widget/context/error browser configuration passed all 44 Chromium and WebKit cases.

Launcher errors retain reason keys and widget validation retains invalid-state flags, so changing language also translates errors that are already visible. Regression tests cover Portuguese, Spanish and English widget validation without saving invalid drafts. The focused launcher/child route suite passed 27 tests; child lifecycle tests passed with Go's race detector. Pinned Wails bindings, the production frontend build and repository JavaScript lint also passed. Independent review found no blocking issue in the widget layout and validation changes; parent-event retirement handling was checked separately.

The combined desktop frontend suite passed all 602 tests, and the full Go race/coverage suite passed. Existing React `act` warnings remain in broader application tests, and the production build retains its existing large-chunk warning; neither check failed.

## Remaining native checks

Current-build switcher thumbnails and window actions still require an accessible open switcher. Two disposable TextEdit windows were prepared; no current-build minimize/close result is claimed. The earlier build's bounded results remain documented in [the prior report](2026-10-02-live-bulk-acceptance.md).

The computer-control tool selected the replacement Dock's parent window after Show all windows. That does not establish whether the separate child window opened. Keyboard/IME focus acceptance and real media timing remain unverified.

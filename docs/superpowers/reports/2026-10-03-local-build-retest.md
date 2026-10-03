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

## Launcher action visibility

Opening actions for the last running application exposed controls in the accessibility tree while leaving them outside the visible Dock strip. A browser regression reproduced this on all four edges before any automatic click scrolling. Opening actions now scrolls their container into the existing viewport. The test checks bounds and hit targets, invokes the visible Show all windows control, and verifies keyboard reopening and Escape dismissal.

This fix is a source change after `cb3289a`; it has not been tested in a rebuilt native application. The full desktop unit suite passed (596 tests), together with 30 related Chromium cases, all four new WebKit cases, TypeScript and Biome checks. The new geometry regression is also included in the existing WebKit CI configuration.

## Remaining native checks

Current-build switcher thumbnails and window actions still require an accessible open switcher. Two disposable TextEdit windows were prepared; no current-build minimize/close result is claimed. The earlier build's bounded results remain documented in [the prior report](2026-10-02-live-bulk-acceptance.md).

The computer-control tool selected the replacement Dock's parent window after Show all windows. That does not establish whether the separate child window opened. Keyboard/IME focus acceptance and real media timing remain unverified.

# Option Tab Dock enhancement scope

Baseline: v0.4.8. Branch: `feat/dockdoor-parity`.
Status: approved narrowed scope; implementation in progress. Unchecked items have not passed all acceptance checks.

This scope was narrowed after reviewing the other projects in `~/Dev/pessoal`. It adds complementary Option Tab features rather than duplicating products we already maintain. Existing Option Tab features are retained. Native Dock enhancements come first. The subsequent instruction to work on all retained features includes the optional Dock replacement, delivered as a separate mode after the core enhancements.

## Removed overlaps and product ownership

| Removed from this roadmap | Existing owner |
|---|---|
| A01–A04: maximize/restore, centering, halves and quarters; no new tiling/edge-snap engine | Tiles Spliter |
| E07–E08: calendar panel, event feed, permissions and calendar-provider work | Calendium |
| H10: file staging shelf, pinned-folder shelf and AirDrop zone | DragZone |
| H11 saved-command execution | DragZone |
| H13 weather provider/widget | Calendium |

Removed IDs stay retired so existing references remain meaningful. These features are excluded, not marked completed in Option Tab. No automatic integration with sibling apps is implied.

Option Tab-specific interfaces remain: dragging a *window preview* is different from Tiles Spliter's desktop edge snapping; Folder Pop is different from DragZone's saved folder action; window automation is different from DragZone's file/action commands. Do not build another generic CLI/script-action platform. Native panel styling, localization and release compatibility remain requirements for Option Tab even when another project has similar infrastructure.

## First implementation checkpoint

The first batch adds explicit-target actions, configurable switcher interactions and live ScreenCaptureKit previews. Completed IDs below have code, focused checks and relevant native/browser evidence. App grouping, Dock surfaces and later stages remain in progress.

Bulk close/minimize and cache lifecycle are implemented with remaining native limitations: incomplete AX discovery is reported explicitly, and retained closed-window detection depends on the app supporting Accessibility destruction notifications. These broader roadmap items remain unchecked. See [validation and limitations](superpowers/reports/2026-09-06-dock-foundations.md).

## App grouping and native Dock checkpoint

App grouping, running windowless apps, independent mode settings, Dock hover previews, exact preview focus/actions, independent Dock appearance and scoped pointer/capture transport are implemented. Required lint, unit/race, production build and browser checks pass. Disposable native fixtures have proved real Dock hover, nonactivating Close against the exact second preview, windowless application activation, and bounded capture returning to idle.

Acceptance remains partial for the broader native matrix: actual shortcut delivery, other-Space focus, all physical Dock/display configurations, restart, lock and permission transitions are not established by the combined harness. C04 spacing implementation is in progress. A [positive window-retirement follow-up](superpowers/reports/2026-09-06-positive-window-retirement.md) fixes deliberately retained closed NSWindows when exact AX destruction was observed; broader C06 coverage remains incomplete. See [the app/Dock validation report](superpowers/reports/2026-09-06-app-groups-and-dock-previews.md). Unchecked IDs continue to distinguish implementation from completed acceptance.

## A. Window actions

- [x] A05 New Window for apps that expose a supported command; indicate unsupported apps.
- [x] A06 Force quit an app through an explicit action.
- [ ] A07 Close all windows belonging to an app, preserving native save dialogs.
- [ ] A08 Minimize all windows belonging to an app.
- [ ] A09 Expose existing focus, close, minimize/restore, hide, quit, and fullscreen actions consistently across every new surface.
- [ ] A10 Return visible action failures and partial bulk results instead of silently ignoring them.

## B. Switcher improvements

- [ ] B01 App-grouped Command+Tab mode: one icon per app and the selected app's window preview.
- [x] B02 Include running apps without windows in app mode.
- [ ] B03 Separate app-switcher and window-switcher appearance/behavior settings.
- [x] B04 Configurable physical-key window-action bindings with conflict validation.
- [x] B05 Configurable middle-click actions, defaulting to close where enabled.
- [x] B06 Automatic compact-list mode at a configurable window-count threshold, plus always-list mode.
- [x] B07 Horizontal/vertical layout direction and corresponding navigation.
- [x] B08 Configurable two-finger swipe actions inside the switcher.
- [x] B09 Complete the exposed app-badge and dismissal-animation settings.
- [ ] B10 Native translucent material background where supported, with a solid fallback.

## C. Previews

- [x] C01 Continuously refreshed previews while visible, with bounded resource use.
- [ ] C02 Larger selected/hovered-window previews across switcher and Dock surfaces.
- [ ] C03 Preserve aspect ratio and offer dynamic preview sizing.
- [ ] C04 Separate spacing, arrangement, sizing and appearance for Dock previews.
- [ ] C05 Configurable embedded controls and traffic-light styling.
- [ ] C06 Stop visible capture on dismissal and handle permission revocation, closed windows, stale frames and cache eviction.

## D. Native Dock integration

- [ ] D01 Hover an app icon to see that app's windows.
- [ ] D02 Click a preview to focus the exact window.
- [ ] D03 Manage windows from the hover panel without activating them first where the action permits it.
- [ ] D04 Configurable hover delay, dismissal delay and interaction thresholds.
- [ ] D05 Keep previews open while moving between the Dock icon and panel.
- [ ] D06 Support left, bottom and right Dock positions, auto-hide and multi-display placement.
- [ ] D07 Apply app exclusions and window filters to Dock previews.
- [ ] D08 Reconnect after Dock restart and adapt to display/Space changes.
- [ ] D09 Click an app icon to hide all of its windows when enabled.
- [ ] D10 Scroll up to show an app and down to hide it; handle wheel, trackpad and momentum.
- [ ] D11 Command-right-click to quit; Command-Option-right-click to force quit.
- [ ] D12 Configurable two-finger preview swipes, with directions relative to Dock position.
- [ ] D13 Drag a preview onto the desktop to reposition its underlying window; no edge snapping or tiling engine.
- [ ] D14 Aero Shake with configurable minimize/close-other-window action.
- [ ] D15 Lock the native Dock to a selected monitor, with bypass modifier and disconnect recovery.

## E. Folder and media panels

- [ ] E01 Folder Pop: hover a Dock folder to view contents.
- [ ] E02 Sort folder contents and open files/folders.
- [ ] E03 Request folder access on demand and handle missing/revoked access.
- [ ] E04 Spotify and Apple Music now-playing details, artwork and playback controls.
- [ ] E05 Synchronized lyrics when an available, permitted provider supplies them.
- [ ] E06 Pin media panels to the screen.

## F. Automation

- [ ] F01 AppleScript commands to open the switcher and show/hide app previews.
- [ ] F02 Resolve apps by name, bundle ID or PID; accept explicit preview coordinates.
- [ ] F03 Focus/close/minimize/hide/fullscreen by window ID or active window; no tiling commands.
- [ ] F04 JSON queries for running apps, window lists and active-window details.
- [ ] F05 Optional cached preview images in query responses.
- [ ] F06 Document Terminal/osascript and macro-tool integration.

## G. Distribution and support

- [ ] G01 Publish tested Intel/universal macOS downloads and align updater/download URLs.
- [ ] G02 Homebrew installation route.
- [ ] G03 Validate signed/notarized packages and supported macOS versions; macOS 13 capture needs a real fallback before claiming parity.
- [ ] G04 User-controlled diagnostic log export.
- [ ] G05 Document local data handling and any update/media-provider network activity.
- [ ] G06 Extend existing English, Brazilian Portuguese and Spanish strings to all new controls.

## H. Optional Dock replacement — later delivery stage

- [ ] H01 Optional replacement Dock with safe restoration of native Dock access.
- [ ] H02 Per-display Docks, edges, layouts and profiles.
- [ ] H03 Switch profiles according to the focused app.
- [ ] H04 Pinned apps, folders, files and links; app groups, spacers and separators.
- [ ] H05 Custom item icons and drag reordering/grouping.
- [ ] H06 Spring magnification, configurable scale/reach, and high-refresh animation.
- [ ] H07 Native materials, tint, borders, transparency, size and light/dark appearance.
- [ ] H08 Floating/full-width layouts, auto-hide, and Dock overlap avoidance; no general window-tiling controls.
- [ ] H09 Folder fan-out with list/grid presentation.
- [ ] H11 App context menus with show-all and relaunch; no saved-command runner.
- [ ] H12 Media scrubbing and audio-output device switching.
- [ ] H13 Clock, battery, network and audio widgets with stacks; weather stays in Calendium.
- [ ] H14 Community widget format, installation flow and documented extension capabilities.
- [ ] H15 Dock pinch/swipe gestures, haptics and letter navigation.
- [ ] H16 Notification badges where a supported source is available.
- [ ] H17 Export/import Dock profiles, items and widget settings.

## Current input checkpoint

D09–D14 implementation and C04 card spacing are available on the feature branch. Automated suites and disposable native role/action fixtures pass; physical gesture and off-panel drag acceptance remain open. See the [input/drag checkpoint report](superpowers/reports/2026-09-06-dock-input-and-preview-drag.md) for behavior, evidence and exact limitations. The roadmap's unchecked items are not promises of universal AX or device support.

## Current monitor checkpoint

Monitor protection (D15) is implemented with manual placement, independent settings and verified-position admission. Automatic placement remains disabled after a native outbound test succeeded but its return cancelled on input evidence. Broader physical protection acceptance remains open. See the [monitor-lock checkpoint report](superpowers/reports/2026-09-06-dock-monitor-lock.md).

## Existing features retained

Nine activation shortcuts; window-based switching; fuzzy search; Vim/arrow navigation; MRU and other sorting; app/Space/monitor filters; appearance settings; large selected preview; cursor following and haptics; menu-bar controls; login launch; permission onboarding; settings import/export/reset; updater; and three UI languages.

## Delivery order and acceptance

Implement A–B first, then C–D, E, F and G. H is independently scoped because it replaces the Dock instead of enhancing it. Each milestone gets focused tests and native smoke checks before being marked complete. A setting or demo-only UI does not count as implemented native behavior.

The removed-feature table takes precedence over the competitor inventory: full DockDoor parity is no longer the goal.

Architecture and milestone acceptance criteria: [design proposal](superpowers/specs/2026-09-06-dockdoor-parity-design.md).

Sources: supplied 2:09 screen recording; [DockDoor Free](https://dockdoor.net/); [automation documentation](https://dockdoor.net/docs.html); [DockDoor Pro](https://pro.dockdoor.net/). New Window and the app-icon switcher presentation are visible in the recording at approximately 01:24 and 01:42. Platform-dependent integrations need native validation before promising universal app support.


## Current Folder Pop checkpoint

E01–E03 implementation is available on the feature branch: exact Dock-folder previews, sorting, on-demand access and guarded opening. Automated and disposable native fixtures pass. Visible chooser approval, real default-app opening and distributed-app permission behavior remain acceptance checks; E01–E03 stay unchecked until that evidence is complete. See the [Folder Pop checkpoint report](superpowers/reports/2026-09-06-folder-pop.md). Media, automation, distribution and the optional replacement Dock remain retained work.

## Current media checkpoint

E04–E06 implementation adds independently enabled Music/Spotify panels, typed native transport, bounded artwork, explicit local timestamped lyrics and separate session-only media pins. Provider consent and remote artwork are opt-in. Spotify seek remains unavailable pending runtime unit verification. Automated/native fixture evidence and physical acceptance limits are recorded in the [media checkpoint report](superpowers/reports/2026-09-07-dock-media.md); E04–E06 stay unchecked. Automation, distribution and the optional replacement Dock remain retained work.

# DockDoor parity implementation scope

Baseline: v0.4.8. Branch: `feat/dockdoor-parity`.
Status: proposed scope for review; unchecked items are not implemented.

This inventory combines the supplied website recording, DockDoor Free's public feature page and automation documentation, and the separately identified DockDoor Pro scope. Existing Option Tab features are retained. The initial recommendation is Free parity first, with Pro replacement tracked separately pending the scope decision.

## A. Window actions

- [ ] A01 Maximize to usable screen bounds and restore the previous size/position.
- [ ] A02 Center a window without changing its size.
- [ ] A03 Position in left, right, top, or bottom half.
- [ ] A04 Position in any of the four screen quarters.
- [ ] A05 New Window for apps that expose a supported command; indicate unsupported apps.
- [ ] A06 Force quit an app through an explicit action.
- [ ] A07 Close all windows belonging to an app, preserving native save dialogs.
- [ ] A08 Minimize all windows belonging to an app.
- [ ] A09 Expose existing focus, close, minimize/restore, hide, quit, and fullscreen actions consistently across every new surface.
- [ ] A10 Return visible action failures and partial bulk results instead of silently ignoring them.

## B. Switcher improvements

- [ ] B01 App-grouped Command+Tab mode: one icon per app and the selected app's window preview.
- [ ] B02 Include running apps without windows in app mode.
- [ ] B03 Separate app-switcher and window-switcher appearance/behavior settings.
- [ ] B04 Configurable physical-key window-action bindings with conflict validation.
- [ ] B05 Configurable middle-click actions, defaulting to close where enabled.
- [ ] B06 Automatic compact-list mode at a configurable window-count threshold, plus always-list mode.
- [ ] B07 Horizontal/vertical layout direction and corresponding navigation.
- [ ] B08 Configurable two-finger swipe actions inside the switcher.
- [ ] B09 Complete the exposed app-badge and dismissal-animation settings.
- [ ] B10 Native translucent material background where supported, with a solid fallback.

## C. Previews

- [ ] C01 Continuously refreshed previews while visible, with bounded resource use.
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
- [ ] D13 Drag a preview onto the desktop to reposition its underlying window.
- [ ] D14 Aero Shake with configurable minimize/close-other-window action.
- [ ] D15 Lock the native Dock to a selected monitor, with bypass modifier and disconnect recovery.

## E. Folder, media and calendar panels

- [ ] E01 Folder Pop: hover a Dock folder to view contents.
- [ ] E02 Sort folder contents and open files/folders.
- [ ] E03 Request folder access on demand and handle missing/revoked access.
- [ ] E04 Spotify and Apple Music now-playing details, artwork and playback controls.
- [ ] E05 Synchronized lyrics when an available, permitted provider supplies them.
- [ ] E06 Pin media panels to the screen.
- [ ] E07 Calendar Dock hover showing today's events.
- [ ] E08 Calendar permission, timezone/day changes and empty/error states.

## F. Automation

- [ ] F01 AppleScript commands to open the switcher and show/hide app previews.
- [ ] F02 Resolve apps by name, bundle ID or PID; accept explicit preview coordinates.
- [ ] F03 Focus/close/minimize/maximize/hide/fullscreen/center/snap by window ID or active window.
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

## H. Separate Pro-style Dock replacement — scope decision pending

- [ ] H01 Optional replacement Dock with safe restoration of native Dock access.
- [ ] H02 Per-display Docks, edges, layouts and profiles.
- [ ] H03 Switch profiles according to the focused app.
- [ ] H04 Pinned apps, folders, files and links; app groups, spacers and separators.
- [ ] H05 Custom item icons and drag reordering/grouping.
- [ ] H06 Spring magnification, configurable scale/reach, and high-refresh animation.
- [ ] H07 Native materials, tint, borders, transparency, size and light/dark appearance.
- [ ] H08 Floating/full-width layouts, auto-hide, and overlap avoidance.
- [ ] H09 Folder fan-out with list/grid presentation.
- [ ] H10 File staging tray, pinned folders and AirDrop zone.
- [ ] H11 Rich context menus with show-all, relaunch and user-saved commands.
- [ ] H12 Media scrubbing and audio-output device switching.
- [ ] H13 Clock, weather, battery, network and audio widgets with stacks.
- [ ] H14 Community widget format, installation flow and documented extension capabilities.
- [ ] H15 Dock pinch/swipe gestures, haptics and letter navigation.
- [ ] H16 Notification badges where a supported source is available.
- [ ] H17 Export/import Dock profiles, items and widget settings.

## Existing features retained

Nine activation shortcuts; window-based switching; fuzzy search; Vim/arrow navigation; MRU and other sorting; app/Space/monitor filters; appearance settings; large selected preview; cursor following and haptics; menu-bar controls; login launch; permission onboarding; settings import/export/reset; updater; and three UI languages.

## Delivery order and acceptance

Implement A–B first, then C–D, E, F and G. H is independently scoped because it replaces the Dock instead of enhancing it. Each milestone gets focused tests and native smoke checks before being marked complete. A setting or demo-only UI does not count as implemented native behavior.

Architecture and milestone acceptance criteria: [design proposal](superpowers/specs/2026-09-06-dockdoor-parity-design.md).

Sources: supplied 2:09 screen recording; [DockDoor Free](https://dockdoor.net/); [automation documentation](https://dockdoor.net/docs.html); [DockDoor Pro](https://pro.dockdoor.net/). New Window and the app-icon switcher presentation are visible in the recording at approximately 01:24 and 01:42. Platform-dependent integrations need native validation before promising universal app support.

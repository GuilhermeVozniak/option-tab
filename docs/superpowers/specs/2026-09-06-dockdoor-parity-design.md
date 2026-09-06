# DockDoor parity design proposal

Status: proposed for review; no parity features are implemented by this document.
Baseline: Option Tab v0.4.8, with all reliability fixes from PRs #23–#26.

## Product direction

Keep Option Tab's existing keyboard switcher and settings. Add DockDoor Free parity in independently testable milestones on `feat/dockdoor-parity`. Track the separate Pro-style replacement as a later milestone unless the user chooses to include it in the first program. Nothing in this proposal changes v0.4.8.

Three approaches considered:

1. Extend the current Go/Wails application with focused native capabilities (recommended). Reuses switching, filtering, settings, and window identity; native integration stays behind platform interfaces.
2. Rewrite the desktop application in Swift. Offers a uniform native UI but repeats the existing switching and settings work and increases migration risk.
3. Build a separate Dock companion. Isolates the Dock lifecycle but duplicates permissions, configuration, previews, and action dispatch.

## Shared architecture

Create one typed window-action service used by keyboard commands, preview buttons, gestures, bulk actions, and automation. Resolve targets by window ID at execution time. Recheck ownership and availability before applying an action. Preserve the existing non-activating overlay and distinguish fullscreen from maximize/restore.

Pure Go computes geometry against `domain.Screen.Visible`; the native adapter reads/writes AX geometry and reports unsupported operations. UI and automation receive explicit errors. Bulk operations return successes and per-window failures; one closed window must not abort the rest.

Keep new native implementations in focused files alongside `internal/platform/darwin.m`, with declarations in `darwin.h` and narrow optional capabilities in `platform.go`. Extend fake backends for semantic tests. Windows/Linux remain explicit unsupported/demo backends until separately implemented.

Use versioned settings with migrations preserving existing shortcuts. New Dock behavior defaults off until enabled. Permission requests happen when enabling the associated capability. Native event callbacks enqueue lightweight events; window enumeration, AX queries, capture, and persistence stay off event-tap callbacks.

## Milestone 1: window actions and switcher controls

Deliver an app-aware New Window action where supported, maximize/restore, center, left/right/top/bottom halves and four quarters, force quit, close-all and minimize-all for an app. Restoration records belong to the window/process lifetime and are discarded on closure. Geometry uses points, handles negative monitor coordinates, respects usable screen bounds, and reports non-resizable windows.

Add configurable physical-key action bindings, middle-click action selection, automatic compact-list threshold (zero disables; trigger at or above configured count), and horizontal/vertical arrangement. Preserve type-to-search and reserved navigation keys; reject conflicting bindings in settings. Explicit window targets prevent hover or selection races. Bulk destructive actions require explicit invocation and keep ordinary close behavior, including application save dialogs.

Add an app-grouped Command+Tab mode with one app icon per running app and a selected-app window preview; retain the existing per-window mode. Include windowless apps in app mode and permit opening a new window when the app supports it. Unsupported New Window commands must be reported rather than silently emitting a global shortcut to the wrong app.

Fix existing exposed appearance controls: wire app badges, let dismissal finish before native hiding, and use an optional native material background with a solid fallback. Keep theme, reduced-motion behavior, and accessibility labels consistent across layouts.

Files: `internal/domain`, new `internal/actions`, `internal/platform/{platform.go,darwin.go,darwin.h,darwin_geometry.m}`, `internal/platform/fake`, `internal/config`, `internal/switcher`, `app_switcher.go`, frontend `lib/{keymap,layout,types,bridge}`, `overlay`, and settings tabs.

Acceptance: geometry/action tests cover offscreen displays, non-resizable and vanished windows; settings migration and binding conflict tests; UI tests prove middle-click never focuses, threshold transitions preserve selection, and error feedback is visible. Native smoke tests verify maximize/restore and snap on two displays.

## Milestone 2: bounded live previews

Replace snapshot-only refresh while visible with a capture-session manager. Subscribe to visible cards and selected preview; prioritize selection, cap concurrent streams, stop on hide/permission loss, and evict closed-window cache entries. Keep snapshot fallback when streaming is unavailable. No hidden recording unless the user explicitly enables background capture.

Files: new `internal/preview`, native capture adapter, `app_switcher.go`, frontend preview rendering. Capture lifecycle must have cancellation, bounded frame delivery, and no queued stale images after window reuse.

Acceptance: tests for unsubscribe, permission revocation, closing windows, bounded cache and stale frame rejection; native visible-video smoke test; record CPU/memory before and after repeated show/hide cycles.

## Milestone 3: native Dock previews

Observe the native Dock's accessibility elements and pointer context. Resolve app identity and icon bounds, then display a separate non-activating preview panel anchored to the Dock icon. Handle left/right/bottom Docks, auto-hide, multiple monitors, Spaces changes and Dock restart. Hover delays, dismissal thresholds, dynamic sizing, layouts, embedded controls, and traffic-light styling have separate settings from keyboard switching.

Reuse window enumeration, filtering, capture, and action services. Moving between an icon and its preview keeps the panel open. Leaving both dismisses it after the configured delay. Windowless apps show an explicit empty state.

Files: new `internal/dock`, native `darwin_dock.m`, `app_dock.go`, frontend `dock`, and a Dock settings tab.

Acceptance: deterministic pointer-state tests; panel placement tests for all Dock edges; native smoke tests prove hovering and managing windows do not steal focus, and Dock restart reconnects observation.

## Milestone 4: Dock mouse and trackpad workflows

Add icon click-to-hide, icon scroll-to-show/hide, Command-right-click quit, Command-Option-right-click force quit, and configurable preview swipe/middle-click actions. Map swipe directions relative to Dock edge; ignore momentum repeats and require movement thresholds.

Add preview dragging that repositions the underlying window with a clear drag affordance, and configurable Aero Shake acting on other windows. Exact drag semantics are our own specified behavior; the competitor's phrase about dragging between applications does not establish a transferable window ownership feature. Drag/gesture cancellation must leave no stuck capture or event interception.

Dock monitor locking is opt-in, handles unplugged monitors, and supports a bypass modifier. Do not modify persistent system Dock preferences to implement a transient lock.

Acceptance: gesture state tests cover threshold, momentum, cancellation and edge direction; native tests cover all Dock positions, modifiers, monitor disconnect, and ordinary input passing through when disabled.

## Milestone 5: folders, media, and calendar

Folder Pop lists/sorts/opens contents of a hovered Dock folder. Request access on demand, persist approved access appropriately, and show actionable denied/missing-folder states. File opening uses system APIs with paths as data.

Media adapters provide Spotify/Apple Music playback state and transport; a pinnable panel displays synchronized lyrics when a permitted provider supplies them. Missing lyrics, unavailable players, and offline providers get explicit empty states. Do not promise universal lyrics coverage or bundle unlicensed lyrics.

Calendar uses EventKit with on-demand permission and displays today's events in the user's timezone. Refresh on date/calendar changes and revoke subscriptions when disabled.

Files: new `internal/widgets` adapters, native folder/media/calendar files, frontend widget panels, settings, and required usage descriptions in Info.plist.

Acceptance: fake providers cover permissions, timezone/day transitions, missing folders, playback changes, unavailable lyrics and offline behavior; native checks exercise actual app integration and permission denial/revocation.

## Milestone 6: automation

Expose an AppleScript dictionary and local command entry points for showing/hiding previews, opening the switcher, window actions, and JSON app/window queries including optional cached preview images. Support app name, bundle ID and PID, window IDs and the active window, and explicit preview coordinates.

Route commands through the same services and error model as the UI. Document use from Terminal through osascript and macro tools. This is local automation, with no unauthenticated network listener.

Acceptance: integration tests execute real osascript commands against a test instance, validate returned JSON and errors, and prove actions cannot target stale/reused identities.

## Milestone 7: distribution and compatibility

Publish a tested Intel build or universal macOS asset; keep release asset names, website links, updater resolution, signing and notarization consistent. Add a maintained Homebrew distribution route. Either implement a working macOS 13 capture fallback or clearly require macOS 14 in bundle metadata, download copy, and release documentation; do not claim compatibility solely because the binary launches.

Provide log export with explicit user control. Keep preview data local and document update/provider network activity accurately.

Acceptance: inspect actual packaged architecture and signature, verify installed version and update paths on supported Macs, test cask install/uninstall, and verify download links against uploaded assets.

## Later milestone: optional Pro-style Dock replacement

This is a separate product mode, not a requirement for enhancing the native Dock. If included, add a dedicated Dock host with per-display layouts and profiles, focus-driven profile switching, pinned apps/files/folders/links, grouping, custom icons, separators, drag ordering, spring magnification, materials, auto-hide and overlap avoidance.

Add folder fan-out, file staging and AirDrop, richer context menus including relaunch and saved commands, audio-output switching, media scrubbing, widget stacks (clock/weather/battery/network), a documented community widget format and installation flow, letter navigation, pinch/swipe gestures, notification badges, and backup/restore of profiles and settings.

Require independent designs for Dock lifecycle, file staging and widget extensibility before coding them. User-installed widgets and saved commands need explicit capabilities and opt-in execution; there is no arbitrary downloaded code execution by default. Display disconnect and disabling replacement restore access to the native Dock.

## Delivery gates

Each milestone gets a focused implementation plan and commits on the feature branch. Update the coverage checklist only after its acceptance criteria pass. Run focused unit/integration/UI tests during implementation; before a milestone PR run lint, race tests, build, browser tests and relevant native smoke checks. Keep the branch unpublished as a release until the user requests a new release.

## Reference scope

Competitor capability inventory: https://dockdoor.net/ and https://dockdoor.net/docs.html; optional replacement scope: https://pro.dockdoor.net/. These are feature references, not a source-code import plan. Retain Option Tab's own implementation and identity.

## Recording evidence

Inspected the supplied 2:09 recording using frames sampled throughout and larger views of the interaction examples.

- 00:06–00:21: app-wide close/minimize, click-to-hide, scroll show/hide and Aero Shake.
- 00:24–00:39: dragging a preview onto the desktop, middle-click close and Dock quit.
- 00:42–01:00: enlarged previews, Dock locking and gesture action menus, including center and half/quarter positions.
- 01:03–01:18: calendar and media/lyrics panels.
- 01:24: preview controls explicitly label New window and Enter full screen. These are distinct from maximizing.
- 01:33–01:48: layouts, an app-icon Command+Tab presentation with selected-app preview, Folder Pop and Dock previews.
- 01:57–02:03: Pro teaser for magnification, folder fan-out, file tray and Dock appearance. The recording does not demonstrate the complete Pro product.

Reference file: `/Users/guilherme/Desktop/Screen Recording 2026-09-06 at 15.09.18.mov`.

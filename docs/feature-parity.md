# AltTab Feature Parity Audit

> Goal: make `apps/desktop` a 1:1 free clone of [AltTab](https://alt-tab.app/).
> This document enumerates **every** AltTab feature, marks its status in this repo,
> explains **why** the partial/missing ones don't work, and calls out UI gaps.
> Source of truth for AltTab's surface: its preferences UI / `Localizable.strings`.
>
> Legend: ✅ done · 🟡 partial (in model/code but not usable or incomplete) · ❌ missing

---

## 0. The root cause behind most 🟡 items — ✅ RESOLVED (2026-06-28, Task #1)

**Previously:** `internal/platform/darwin.m` (`ot_list_windows_json`) emitted only
`id, pid, app, bundle, title, x/y/w/h, onscreen` and queried with
`kCGWindowListOptionOnScreenOnly`, so `SpaceID`, `ScreenID`, `Minimized`,
`Hidden`, `Fullscreen`, and `LastFocused` were never populated and minimized /
other-Space windows were excluded entirely. The pure Go logic for these existed
and was unit-tested but was a no-op against the real backend.

**Now:** the macOS backend enumerates **all** windows (`kCGWindowListExcludeDesktopElements`,
no OnScreenOnly) and enriches each with:

| Field | Source |
|---|---|
| `ScreenID` | `CGGetDisplaysWithPoint` on the window-bounds center |
| `Minimized` | AX scan (`kAXMinimizedAttribute` + private `_AXUIElementGetWindow`) |
| `Hidden` | `NSRunningApplication.isHidden` |
| `Fullscreen` | window rect exactly covers its display |
| `SpaceID` | private CGS `CGSCopySpacesForWindows` (+ `ot_active_space`) |
| `LastFocused` | synthesized from CGWindowList z-order (frontmost = most recent), below the MRU reference point so explicit MRU still wins |

`ot_active_screen` / `ot_cursor_screen` / `ot_screens_json` now return real
displays. Everything degrades to safe zero values without Accessibility/CGS, so
Space/screen filters fall back to "all" rather than hiding windows. Mapping is
unit-tested via the pure `mapRawWindows` (`darwin_mapping_test.go`); the native
path is runtime-smoke-tested by `TestWindows_DoesNotError`.

**Remaining caveats:** the AX minimized scan + per-window CGS lookup run on every
`Windows()` call (no caching yet — a perf follow-up); fullscreen is a bounds
heuristic; `ot_active_screen` falls back to the main display when no AX focused
window is found.

---

## 1. Controls / Shortcuts

| AltTab feature | Status | Notes / why |
|---|---|---|
| Up to 9 independent shortcuts, each with own keys + filters | 🟡 | `config.MaxShortcuts = 9` and per-shortcut scope/style overrides exist, but `Settings.tsx` only renders the 2 seeded rows — no add/remove UI |
| Hold modifier, release to focus (hold-to-cycle) | ✅ | `Behavior.HoldToCycle` + CGEventTap modifier-release detection |
| Select next / previous (Tab / Shift+Tab) | ✅ | `keymap.ts` |
| Arrow-key navigation | ✅ | `keymap.ts` (Arrow* → advance/reverse) |
| Vim-key navigation (h/j/k/l) | ✅ | opt-in `Behavior.VimKeys`; matched on physical `e.code` (Task #3) |
| Select on mouse hover | ✅ | `onMouseEnter` → `Select` |
| Action: Focus selected window | ✅ | `Confirm` |
| Action: Close window | ✅ | hover button + key (modifier+W) (Task #3) |
| Action: Minimize/deminimize | ✅ | key (modifier+M); native now toggles min/deminimize (Task #3) |
| Action: Hide/show app | ✅ | key (modifier+H) (Task #3); still no hover button (minor) |
| Action: Quit app | ✅ | hover button + key (modifier+Q) (Task #3) |
| Action: Fullscreen/defullscreen window | ✅ | `FullscreenSelected` + key (modifier+F); native AXFullScreen toggle (Task #3) |
| Action: Preview selected window | ✅ | `Appearance.PreviewSelected` renders a large preview inside the overlay; a fresh 1024px capture is streamed per selection change via `switcher:preview` (Task #7), falling back to the grid thumbnail while it loads |
| Per-shortcut: minimum window count to show | ❌ | not modeled |
| Cursor follows focus | ✅ | `Behavior.CursorFollowFocus` → `CGWarpMouseCursorPosition` to the focused window's center on commit (Task #6) |
| Per-shortcut "when released: focus / do nothing" | ✅ | `Shortcut.WhenReleased` (Task #6): doNothing keeps the switcher open until Enter/Esc/click |
| Per-shortcut order / Spaces / Screens overrides in UI | ✅ | each shortcut row exposes order + spaces + screens selects, "global default" inherits (Task #6) |
| Select on mouse hover — toggle | ✅ | `Behavior.MouseHoverSelect` (default on) gates hover selection (Task #6) |

## 2. Gestures

| AltTab feature | Status | Notes |
|---|---|---|
| 3/4-finger horizontal/vertical swipe to trigger | ❌ | no trackpad/gesture port at all |
| Trackpad haptic feedback | ❌ | — |

## 3. Appearance

| AltTab feature | Status | Notes / why |
|---|---|---|
| Style: Thumbnails | ✅ | live captures via ScreenCaptureKit (`SCScreenshotManager`, macOS 14+) |
| Style: App Icons | ✅ | real `NSRunningApplication.icon` PNGs |
| Style: Titles | ✅ | text list |
| Theme: System / Light / Dark | ✅ | CSS `ot-theme-*` |
| Theme: extra skins (Sequoia / WinXP / Win11) | ❌ | AltTab Pro skins; out of scope unless wanted |
| Size presets Small/Medium/Large | 🟡 | numeric px in config, no preset selector |
| Auto-size switcher | ✅ | `Appearance.AutoSize` + `layout.ts` |
| Max rows / columns | ✅ | config; only Max columns exposed in UI |
| Background opacity / blur / corner radius | ✅ | exposed in Appearance tab (Task #4) |
| Accent / highlight color | ✅ | color input |
| Font / icon / thumbnail size | ✅ | all exposed in Appearance tab (Task #4) |
| Show app/window titles | ✅ | `ShowTitle` |
| Title truncation control | ✅ | `Appearance.TitleTruncation` end/middle/start (Task #6); end via CSS, middle/start char-based |
| Status icons (minimized/hidden/fullscreen badges) | ✅ | per-window markers for all three (Task #5); hideable via `ShowStatusIcons` (Task #6) |
| Space number label per window | ✅ | numbered badges (Task #6): ordinals derived from the sorted distinct Space ids among listed windows; toggle `ShowSpaceNumbers` |
| Hide window-control circles on hover toggle | ✅ | `ShowWindowControls` |
| Apparition delay before switcher shows | ✅ | `Appearance.ApparitionDelayMs` 0–2000ms slider (Task #6) |
| Fade in/out animations | ✅ | enter fade+scale (Task #5) and exit fade via `FadeOutAnimation` (Task #6), both reduced-motion aware |
| Animations toggle | ✅ | `FadeOutAnimation` checkbox (Task #6) |
| Menubar icon | ✅ | `NSStatusBar` tray; glyph styles default/outline/dot + hidden via General-tab radios (Task #6) |

## 4. Filtering & Ordering

| AltTab feature | Status | Notes / why |
|---|---|---|
| Order: Recently used/focused | 🟡 | works in-session via MRU; no real `LastFocused` (see §0) |
| Order: Recently created first | ✅ | `OrderRecentlyCreated` (Task #6): descending window-server id as creation proxy |
| Order: Alphabetical | ✅ | `order.go` |
| Order: Space order | 🟡 | code exists, no-op without `SpaceID` (see §0) |
| Show from all apps / active app only | ✅ | per-shortcut `appScope` |
| Show from all / active / cursor screen | 🟡 | code exists, no-op without `ScreenID` (see §0) |
| Show from visible / active / all Spaces | 🟡 | only active/all modeled; no-op without `SpaceID` |
| Group apps | ❌ | not implemented |
| Group tabs (one entry per tab vs per window) | ❌ | not implemented |
| Show minimized windows | ✅ | tristate Show / Hide / Show-at-end (Task #6); backend facts from §0 |
| Show hidden windows | ✅ | same tristate (Task #6) |
| Show fullscreen windows | ✅ | same tristate (Task #6) |
| Show windows with no title | ✅ | `ShowWindowsWithoutTitle` |
| Show apps with no open window | ❌ | not modeled |

## 5. Blacklists

| AltTab feature | Status | Notes / why |
|---|---|---|
| Per-app blacklist (bundle id / name) | ✅ | Blacklists tab editor (add/remove bundle id or name) (Task #4) |
| Blacklist modes (hide / don't show / disable shortcut) | ✅ | structured entries (Task #6): hide always / when-no-open-window, plus "ignore shortcuts when active" which suppresses activation while the app is frontmost. v1 string lists migrate automatically |

## 6. General / About

| AltTab feature | Status | Notes / why |
|---|---|---|
| Start at login | ✅ | `SMAppService` `LoginItem` + UI |
| Pause / resume switcher | ✅ | tray + `Controller.SetPaused` |
| Permissions: Accessibility prompt + open-settings | ✅ | Settings “Permissions” section shows live state + Grant / Open-Settings (Task #2, 2026-06-28) |
| Permissions: Screen Recording prompt + open-settings | ✅ | same section; Grant triggers `CGRequestScreenCaptureAccess`, Open-Settings deep-links the privacy pane |
| Language selection | ✅ | full i18n (Task #7): English, Português (Brasil), Español dictionaries in `lib/i18n.ts`; “System default” sniffs `navigator.language`; missing keys fall back to English; aria-labels stay English as stable identifiers |
| Check for updates / auto-update policy | ✅ | background checker (Task #7): `internal/update` + daily GitHub releases poll honoring the policy; “update available” banner with Download in General/About; manual check opens releases. No *silent* auto-install (needs a signed updater) — “auto” behaves like “check”, stated in the UI |
| Crash-reports policy | ✅ | real crash capture (Task #7): `internal/crash` + `debug.SetCrashOutput` write fatal panics to a local log; next launch shows a banner with Report (prefilled GitHub issue — user sees exactly what is shared) / Dismiss; policy “never” disables capture and clears leftovers |
| About tab (version, links, feedback, support) | ✅ | About tab with Go-reported version, website/GitHub, send-feedback (issues), support + check-for-updates links (Task #6) |
| Export / Import settings | ✅ | General tab: Export downloads JSON, Import reads a JSON file (Task #4) |
| Reset settings | ✅ | General tab “Reset to defaults” (Task #4) |
| Pro / licensing | N/A | intentionally all-free clone |

## 7. Core runtime (working)

| Capability | Status |
|---|---|
| Window enumeration (CGWindowList) | ✅ (but see §0 limits) |
| Focus / close / minimize via AX API | ✅ |
| Quit / hide app via NSRunningApplication | ✅ |
| Thumbnails via ScreenCaptureKit | ✅ (needs Screen Recording) |
| Real app icons | ✅ |
| Global hotkey (CGEventTap), modifier-release | ✅ |
| Menubar tray | ✅ |
| Multi-monitor placement modes | 🟡 (placement modeled; screen detection limited) |

---

## UI parity (overlay)

The overlay is close to AltTab (Task #5 polish applied):

- ✅ Translucent rounded panel, grid of cells, title-bar with app icon + title, selected highlight, hover controls.
- ✅ Window-control buttons are macOS traffic-light circles: red close, yellow minimize, green fullscreen.
- ✅ Status icons for minimized / hidden / fullscreen, plus an “on another Space” marker.
- ✅ Fade + subtle scale enter animation (honors `prefers-reduced-motion`).
- 🟡 Space marker is a flag, not the numeric Space ordinal (needs a space-enumeration map).
- ❌ No exit (fade-out) animation — overlay unmounts immediately.
- ❌ No live preview of the selected window behind the switcher.

## UI parity (preferences) — ✅ largely addressed (Task #4)

`Settings.tsx` is now a **tabbed** preferences surface (Controls / Appearance /
Filtering / Blacklists / General) mirroring AltTab's layout. Every config field is
reachable: all 9 shortcuts with add/remove + per-shortcut scope and style
override; full appearance (style, theme, accent, max rows/cols, thumbnail/icon/
title/font sizes, opacity, blur, corner radius, title/badge/controls/auto-size
toggles); ordering + placement; all window filters (spaces, screens, minimized/
hidden/fullscreen/no-title); a blacklist editor; and General (start-at-login,
menubar icon, import/export/reset, permissions). All panels stay mounted (inactive
ones hidden) so the form is one controlled surface. Remaining gap vs AltTab: it's
a web-styled form, not a native macOS preferences window (cosmetic), and there's
no size-preset shortcut (Small/Medium/Large) — raw px instead.

---

## Screenshot UI audit — 2026-07-04 (Task #6) ✅

Audited the five AltTab preference screenshots control-by-control against our
Settings UI and closed every gap. Tab layout now mirrors AltTab:
**General · Controls · Appearance · Filtering · Blacklists · About**.

| Screenshot | Controls covered |
|---|---|
| ① General | start at login · menubar-icon style radios (default/outline/dot/hidden) · language · updates policy + check-now · crash-reports policy · permissions · import/export/reset |
| ② Controls | up-to-9 shortcuts (chord, enable, app scope, style) · per-shortcut when-released, Spaces, Screens, order · hold-to-cycle · vim keys · mouse-hover select · cursor follows focus |
| ③ Appearance | style · theme · sizes · show-on-screen · apparition delay · fade-out animation · status icons · Space number labels · colored-circle controls · title truncation · preview selected window |
| ④ Blacklists | structured table: app matcher + hide mode (always / when no open window) + ignore-shortcuts-when-active |
| ⑤ About | version · website/GitHub · send feedback · support project · check for updates |

Settings schema bumped to **v2** (tristate window filters, structured
blacklist); v1 files — boolean filters and plain-string blacklists — still load
via custom `UnmarshalJSON`.

Remaining known gaps (not present in these screenshots or needing new native
work): trackpad gestures, group apps/tabs, show-apps-with-no-open-window,
silent auto-install of updates (needs a signed updater; checks + banner exist),
per-shortcut minimum window count, AltTab Pro skins.

### Caveat follow-up — 2026-07-04 (Task #7) ✅

The four caveats from the screenshot audit were closed:

1. **Translations** — `lib/i18n.ts` (en / pt-BR / es, English-keyed dictionaries,
   graceful fallback); every visible Settings string goes through `t()`.
2. **Update checks** — `internal/update` (parse + semver compare, unit-tested);
   app polls GitHub releases 10s after launch then daily when policy ≠ off and
   emits `update:available` → banner with Download button.
3. **Crash reports** — `internal/crash` (rotate/arm/pending/dismiss, unit-tested)
   wired to `debug.SetCrashOutput`; report opens a prefilled GitHub issue.
4. **Selected-window preview** — per-selection 1024px ScreenCaptureKit capture
   streamed to the overlay instead of reusing the small grid thumbnail.

## Prioritized roadmap (highest leverage first)

1. **Backend window facts** — populate `SpaceID` (CGS private APIs), `ScreenID`,
   `Minimized`/`Hidden`/`Fullscreen` (AX + `kCGWindowListOptionAll`), and a real
   focus timestamp. Unblocks ~8 features (§0).
2. ~~**Permissions UI**~~ — ✅ done (Task #2): live Accessibility + Screen Recording
   state with Grant / Open-Settings, polled so a grant reflects without restart.
3. ~~**Keyboard actions**~~ — ✅ done (Task #3): modifier+W/M/Q/H/F act on the
   selected window (matched on physical key so Option chars don't matter), new
   `Fullscreen` action (AXFullScreen toggle), and opt-in vim h/j/k/l navigation.
4. ~~**Preferences UI build-out**~~ — ✅ done (Task #4): tabbed Controls/Appearance/
   Filtering/Blacklists/General exposing every config field, blacklist editor,
   add/remove for all 9 shortcuts, and import/export/reset.
5. ~~**Overlay polish**~~ — ✅ done (Task #5): traffic-light controls (close/min/
   fullscreen), minimized/hidden/fullscreen + other-Space status icons, and a
   fade/scale enter animation. Remaining: numeric Space ordinal, exit animation,
   live preview.
6. **Ordering/grouping** — recently-created order, group apps, group tabs,
   show-apps-with-no-window.
7. **Gestures** — trackpad swipe trigger + haptics.
8. **General** — updater, language selection, crash-report policy.

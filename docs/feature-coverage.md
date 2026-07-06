# Feature test-coverage matrix

> Companion to [`feature-parity.md`](./feature-parity.md) (what the app does) and
> [`testing.md`](./testing.md) (how we test). This file is the **traceability
> map**: every shipped feature of `apps/desktop` and the test scenarios that lock
> it down. Goal: a test scenario for **every** feature.

Every feature below has at least one automated scenario. Each is verified against
the **current** behavior (the tests lock down what ships today, they do not
assert aspirations). Features AltTab has but this clone intentionally does not
(trackpad gestures, group apps/tabs, show-apps-with-no-open-window, per-shortcut
minimum window count, Pro skins, silent auto-install) are out of scope and carry
no tests — see `feature-parity.md`.

## Totals

| Suite | Files | Tests |
|-------|-------|-------|
| Go (`go test ./... -race`) | 13 packages | 227 |
| Frontend (Vitest + Testing Library) | 11 files | 164 |

Go statement coverage of the pure-logic packages: `domain` / `hotkey` / `mru` /
`order` / `update` 100%, `filter` 96%, `crash` 94%, `config` 94%, `search` 93%,
`switcher` 88%. `internal/platform` (30%) and `internal/platform/fake` (52%) are
low **by design** — the former is the CGO macOS backend (smoke-only, see below),
the latter is test infrastructure. The `app.go` Wails adapter sits at 42%; the
remainder is Wails runtime wiring that needs a live app context.

## Controls & Shortcuts

| ID | Feature | Test files |
|----|---------|-----------|
| C1 | Up to 9 independent shortcuts (model + add/remove UI + per-shortcut chord/enabled/appScope/style) | `Settings.test.tsx`, `app_test.go`, `config_test.go`, `validate_test.go` |
| C2 | Hold-to-cycle: activate → advance → release focuses selected | `Settings.test.tsx`, `app_test.go`, `switcher_test.go` |
| C3 | Tab / Shift+Tab next-previous with wrap | `Overlay.test.tsx`, `keymap.test.ts`, `bridge.test.ts`, `switcher_test.go` |
| C4 | Arrow-key navigation (opt-in `Behavior.ArrowKeys`) | `Overlay.test.tsx`, `keymap.test.ts`, `switcher_test.go` |
| C5 | Vim h/j/k/l navigation (opt-in, only without a modifier) | `Overlay.test.tsx`, `keymap.test.ts` |
| C6 | Select on mouse hover + `MouseHoverSelect` toggle | `Overlay.test.tsx`, `Settings.test.tsx`, `switcher_test.go` |
| C7 | Confirm focuses selected; click confirms; backdrop click cancels | `Overlay.test.tsx`, `keymap.test.ts`, `switcher_test.go` |
| C8 | Close window: hover button + modifier+W (physical code) | `Overlay.test.tsx`, `App.test.tsx`, `keymap.test.ts`, `switcher_test.go`, `fake_test.go` |
| C9 | Minimize / deminimize: modifier+M + yellow button | `Overlay.test.tsx`, `keymap.test.ts`, `switcher_test.go`, `fake_test.go` |
| C10 | Hide / show app: modifier+H | `Overlay.test.tsx`, `keymap.test.ts`, `switcher_test.go` |
| C11 | Quit app: modifier+Q | `Overlay.test.tsx`, `keymap.test.ts`, `switcher_test.go` |
| C12 | Fullscreen toggle: modifier+F + green button | `Overlay.test.tsx`, `keymap.test.ts`, `switcher_test.go` |
| C13 | Preview selected window (`PreviewSelected`, per-selection capture stream) | `App.test.tsx`, `Overlay.test.tsx`, `Settings.test.tsx`, `app_test.go`, `bridge.test.ts`, `layout.test.ts` |
| C14 | Cursor follows focus on commit (`CursorFollowFocus`) | `Settings.test.tsx`, `switcher_test.go` |
| C15 | Per-shortcut when-released: focus vs. do-nothing | `Settings.test.tsx`, `switcher_test.go` |
| C16 | Per-shortcut order/spaces/screens overrides with global-default inheritance | `Settings.test.tsx`, `switcher_test.go`, `filter_test.go` |
| C17 | Shortcut chord recorder (`lib/chord.ts` + `ShortcutRecorder`) | `Settings.test.tsx`, `ShortcutRecorder.test.tsx`, `chord.test.ts` |
| C18 | Hotkey chord parse/match grammar (`internal/hotkey`) | `hotkey_test.go` |

## Appearance

| ID | Feature | Test files |
|----|---------|-----------|
| A1 | Three visual styles render (thumbnails / appIcons / titles) + per-shortcut override | `App.test.tsx`, `Overlay.test.tsx`, `Settings.test.tsx`, `config_test.go`, `switcher_test.go` |
| A2 | Theme system / light / dark (`ot-theme-*`) | `Overlay.test.tsx`, `Settings.test.tsx`, `validate_test.go` |
| A3 | Size presets Small/Medium/Large → px mapping | `Settings.test.tsx`, `config_test.go` |
| A4 | Auto-size switcher + max rows/columns (`computeLayout`) | `layout.test.ts` |
| A5 | Appearance knobs: opacity, blur, radius, accent, font/icon/thumbnail size → CSS vars | `Overlay.test.tsx`, `Settings.test.tsx`, `config_test.go`, `validate_test.go` |
| A6 | Show titles + truncation end / middle / start | `Overlay.test.tsx`, `Settings.test.tsx`, `text.test.ts` |
| A7 | Status icons (min / hidden / fullscreen / other-Space) + `ShowStatusIcons` | `Overlay.test.tsx` |
| A8 | Space-number badges (ordinals from sorted Space ids) + toggle | `Overlay.test.tsx` |
| A9 | Window-control circles show/hide toggle | `Overlay.test.tsx` |
| A10 | Apparition delay before switcher shows (0–2000ms) | `Overlay.test.tsx`, `Settings.test.tsx`, `config_test.go` |
| A11 | Fade-in enter + `FadeOutAnimation` exit + reduced-motion | `Overlay.test.tsx` |
| A12 | Menubar icon styles default/outline/dot/hidden + tray sync | `Settings.test.tsx`, `app_test.go`, `config_test.go` |
| A13 | Show-on-screen placement modes (multi-monitor) | `Settings.test.tsx`, `switcher_test.go`, `validate_test.go` |

## Filtering & Ordering

| ID | Feature | Test files |
|----|---------|-----------|
| F1 | Order: recently used (MRU beats z-order `LastFocused`) | `mru_test.go`, `order_test.go`, `switcher_test.go`, `darwin_mapping_test.go` |
| F2 | Order: recently created (descending window-id proxy) | `Settings.test.tsx`, `order_test.go` |
| F3 | Order: alphabetical | `order_test.go`, `switcher_test.go` |
| F4 | Order: by Space | `order_test.go` |
| F5 | Filter: app scope all vs. active app (per shortcut) | `Settings.test.tsx`, `filter_test.go`, `switcher_test.go`, `config_test.go` |
| F6 | Filter: screen scope all / active / cursor | `Settings.test.tsx`, `filter_test.go` |
| F7 | Filter: Space scope active / all | `Settings.test.tsx`, `filter_test.go` |
| F8 | Show minimized tristate show / hide / show-at-end (`SendToBack`) | `Settings.test.tsx`, `filter_test.go`, `order_test.go`, `switcher_test.go` |
| F9 | Show hidden apps tristate | `Settings.test.tsx`, `filter_test.go`, `order_test.go` |
| F10 | Show fullscreen tristate | `Settings.test.tsx`, `filter_test.go`, `order_test.go`, `config_test.go` |
| F11 | Show windows without a title toggle | `Settings.test.tsx`, `filter_test.go` |
| F12 | Fuzzy search by title/app name + type-to-search routing | `Overlay.test.tsx`, `search_test.go`, `switcher_test.go`, `keymap.test.ts`, `bridge.test.ts` |

## Blacklists

| ID | Feature | Test files |
|----|---------|-----------|
| B1 | Match by bundle id or app name | `Settings.test.tsx`, `filter_test.go` |
| B2 | Hide modes: always vs. when-no-open-window | `Settings.test.tsx`, `filter_test.go` |
| B3 | Ignore-shortcuts: activation suppressed while blacklisted app frontmost | `Settings.test.tsx`, `filter_test.go`, `switcher_test.go` |
| B4 | v1 → v2 config migration (bool tristates, string blacklist entries) | `config_test.go` |

## General & About

| ID | Feature | Test files |
|----|---------|-----------|
| G1 | Start at login (LoginItem binding + UI) | `Settings.test.tsx`, `app_test.go`, `fake_test.go` |
| G2 | Pause / resume gates activation (`SetPaused`) | `app_test.go`, `switcher_test.go`, `bridge.test.ts` |
| G3 | Permissions: states + grant / open-settings + polling gate | `Settings.test.tsx`, `app_test.go`, `useBridge.test.tsx`, `bridge.test.ts`, `darwin_test.go`, `fake_test.go` |
| G4 | i18n: en / pt-BR / es, English-key fallback, `resolveLang` sniff, `t()` coverage | `Settings.test.tsx`, `i18n.test.ts` |
| G5 | Update checks: `ParseLatest`/`Newer`, policy, banner + Download | `Settings.test.tsx`, `update_test.go`, `useBridge.test.tsx`, `bridge.test.ts` |
| G6 | Crash reports: rotate/arm/pending/dismiss, never-policy, report, banner | `Settings.test.tsx`, `crash_test.go`, `app_crash_test.go`, `useBridge.test.tsx`, `bridge.test.ts` |
| G7 | About tab: version + website/GitHub/feedback/support/check-updates | `Settings.test.tsx`, `app_test.go`, `bridge.test.ts` |
| G8 | Export / import / reset settings | `Settings.test.tsx` |
| G9 | Config: Default/Normalize/Validate, paths, MaxShortcuts=9, never-null slices | `config_test.go`, `validate_test.go`, `paths_test.go` |
| G10 | First-run onboarding: onboarded gate, wizard, live permission status | `Settings.test.tsx` |
| G11 | Menubar tray: install/remove sync + command routing | `app_test.go` |
| G12 | Preferences window mode: open/close, window flip, close intercept, tab deep-link | `App.test.tsx`, `Settings.test.tsx`, `app_test.go`, `bridge.test.ts` |
| G13 | Bridge: every bound-method wrapper calls Wails / no-ops when absent | `bridge.test.ts` |

## Core runtime / platform

| ID | Feature | Test files |
|----|---------|-----------|
| P1 | darwin pure mapping: `mapRawWindows`, `chordFromCapture`, `modMask`, `keycodeFor`, degradation | `darwin_mapping_test.go`, `darwin_capture_test.go`, `darwin_test.go` |
| P2 | Cross-Space focus, first-show layout, faster previews (commit 6dee73e) | `switcher_test.go`, `Overlay.test.tsx`, `layout.test.ts`, `darwin_mapping_test.go` |

## Post-parity additions (beyond the AltTab surface)

| ID | Feature | Test files |
|----|---------|-----------|
| X1 | Haptic feedback tick on selection change (`Behavior.HapticFeedback`) | `app_test.go`, `config_test.go` |
| X2 | Background thumbnail capture opt-in (`Behavior.CaptureInBackground`) | `Settings.test.tsx`, `config_test.go` |

## Boundaries not unit-tested (by design)

These are deliberate gaps consistent with the layered philosophy in
`testing.md` — the decision logic behind each is tested; only the thin OS/IO
edge is not:

- **CGO macOS backend** (`darwin.go`/`.m`) — smoke-tested only
  (`darwin_test.go` asserts it constructs and enumerates without panicking). Its
  pure translation helpers (`mapRawWindows`, `chordFromCapture`, `modMask`,
  `keycodeFor`) *are* unit-tested.
- **CSS blur strength** — `appearance.blur` toggles the frosted-glass
  `backdrop-filter` on the switcher's in-page surfaces (search, selected cell,
  controls, preview, badges); the `ot-no-blur` root class it drives is asserted
  in `Overlay.test.tsx`, but the actual pixel blur is not jsdom-observable. Real
  *desktop* blur behind the panel is not possible via CSS in a transparent
  overlay window (needs a native `NSVisualEffectView`); the panel's glass is
  baked into its layered background instead.
- **`App.ReportCrash` issue-URL build** — unobservable: `OpenURL` no-ops when
  the Wails context is nil. Escaping/truncation lives one call deeper than the
  test seam; the crash lifecycle it draws from is covered in `crash_test.go`.
- **`updateLoop` / `checkForUpdateOnce`** — hit the real GitHub API via a
  package-const URL; the parse + semver-compare logic is covered by
  `update_test.go` (`ParseLatest`/`Newer`).
- **`backgroundCaptureLoop`** — a 4s-ticker goroutine; the opt-in flag that
  gates it (X2) is covered, the ticker itself is not.
- **CSS-only visuals** — fade-in and `prefers-reduced-motion` are pure CSS and
  not observable in jsdom; the `FadeOutAnimation` flag branch is tested.

### Quirks found while mapping coverage — resolved

Both were fixed after the coverage pass:

- `appearance.blur` was a dead knob (nothing read it). It now drives the
  `ot-no-blur` root class that gates the in-page frosted-glass `backdrop-filter`
  — a real, visible on/off toggle (see “CSS blur strength” above). Covered by
  `Overlay.test.tsx`.
- The hover controls now include **Hide** and **Quit** app buttons (slate and
  dark circles) alongside close / minimize / fullscreen, matching
  `feature-parity.md`. Covered by `Overlay.test.tsx`.

## Running the suite

```bash
task test       # Vitest (all JS/TS workspaces) + go test ./... -race -cover
task lint       # Biome + golangci-lint
```

Per layer, from `apps/desktop`: `go test ./internal/<pkg>/ -race -count=1`; and
from `apps/desktop/frontend`: `bunx vitest run <path>`. After any Go edit run
`golangci-lint fmt ./...` (never standalone `gofumpt`).

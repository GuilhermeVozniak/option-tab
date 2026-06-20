# option-tab — Window Switcher Product Design

**Date:** 2026-06-20
**Status:** Approved (autonomous goal-driven build)
**Type:** Product feature build (on top of completed monorepo scaffold)

## 1. Purpose

`option-tab` is a free, open-source, cross-platform (macOS-first) window switcher that
replicates and surpasses AltTab (`alt-tab-macos`). It brings the Windows-style
`Alt+Tab` experience to macOS: a hotkey-triggered overlay showing **every window**
(not just every app) with live thumbnails, icons, titles, search, and rich controls.
Every feature AltTab gates behind its "Pro" tier (visual styles, fuzzy search,
auto-sizing thumbnails, up to 9 independent shortcuts) ships here **100% free**.

The monorepo scaffold (tooling, CI, landing page, shared contract) is already complete.
This spec covers the **actual product**: the window-switching engine, the overlay UI,
configuration, and platform integration.

## 2. Feature parity target (from AltTab)

### 2.1 Switching & activation
- Global hotkey opens an overlay listing windows. Default chord: **⌥Tab** (Option+Tab) —
  the product's namesake.
- **Hold-and-release** mode: hold the modifier, press Tab to advance, Shift+Tab to go
  back, release the modifier to focus the highlighted window. Also a **press** (toggle)
  mode.
- Up to **9 independent shortcuts**, each with its own chord and its own filter scope
  (e.g. shortcut 1 = all windows; shortcut 2 = windows of the active app; shortcut 3 =
  windows on the active space; etc.).
- Navigation while open: Tab/Shift+Tab, arrow keys, mouse hover to highlight, click to
  focus, `Esc`/click-outside to cancel, `Q` to quit highlighted app, `W`/close button to
  close highlighted window, `M` to minimize, `H` to hide app, `Return` to focus.
- Cancel restores focus to the previously-focused window (no focus theft).

### 2.2 Window sourcing & filtering
- Enumerate all on-screen windows across all running apps.
- Filters (each independently configurable, and per-shortcut overridable):
  - active space only / all spaces / specific spaces
  - active screen only / all screens / screen under cursor
  - include/exclude minimized windows
  - include/exclude hidden apps' windows
  - include/exclude fullscreen windows
  - include/exclude windows without a title
  - app blacklist (never show these apps) and a "don't-show-the-app-itself" rule
- Display order: **most-recently-used** (default), recently-used-then-active-first,
  alphabetical by app, by space, by creation order.

### 2.3 Appearance / visual styles (all free)
- Three visual styles: **Thumbnails** (live previews), **App Icons** (dock-like row of
  large icons), **Titles** (compact text list).
- Live window thumbnails with app-icon badge, window title, app name, and a window-count
  badge per app.
- **Auto-sizing**: thumbnail size scales to the number of windows to stay readable.
- Customizable: max rows/columns, thumbnail max size, icon size, title max width &
  truncation, font size, light/dark/system theme, accent/highlight color, background
  blur/translucency & opacity, corner radius, show/hide app badges, show/hide title,
  show/hide window controls on hover.
- Overlay placement: centered on active screen / screen under cursor / screen with the
  focused window.

### 2.4 System integration
- Start at login, menubar (tray) icon with menu (Preferences, Pause, Quit), pause/resume.
- Multi-monitor and multi-Space aware.
- Permission onboarding for macOS Accessibility (focus control) and Screen Recording
  (thumbnails), with graceful degradation when not granted.
- Settings persisted to disk as JSON; live-applied without restart.

### 2.5 Out of scope (YAGNI for v1)
- Trackpad gestures (documented as future work).
- Code signing / notarization / auto-update (scaffold's `release.md` already defers this).
- Full Linux/Windows native parity — those platforms get a **compile-clean stub**
  implementation of the platform port plus the full pure-Go core, so the codebase builds
  and tests everywhere; only macOS ships a complete native backend in v1.

## 3. Architecture

Hexagonal-lite, matching the scaffold's established pattern: **all decision logic is pure
Go behind interfaces (fully unit-tested); all OS-specific work sits behind a single
`platform` port with a macOS CGO adapter and a portable stub.** The Wails `app.go` stays a
thin adapter; the React frontend is a pure rendering+input layer driven by events.

```
apps/desktop/
  main.go                      # Wails bootstrap: frameless transparent always-on-top overlay window
  app.go                       # thin Wails adapter: binds methods, wires controller, relays events
  internal/
    domain/                    # core value types: Window, App, Space, Screen, Bounds (PURE)
    config/                    # Settings model, defaults, JSON load/save, validation, migration (PURE)
    filter/                    # window filtering predicates from Settings/ShortcutScope (PURE)
    order/                     # display-ordering strategies (MRU, alphabetical, by-space, ...) (PURE)
    search/                    # fuzzy title/app search + ranking (PURE)
    hotkey/                    # chord parsing/formatting + match logic (PURE) and registry
    switcher/                  # Controller + selection state machine: orchestrates the cycle (PURE core,
                               #   depends only on the platform port interface — fully testable with fakes)
    platform/                  # the PORT
      platform.go              # interfaces: WindowSource, Focuser, Thumbnailer, HotkeyEngine,
                               #   Permissions, LoginItem, ScreenSource + shared structs + events
      darwin.go / *.m / *.h    # macOS CGO backend (build tag darwin)
      stub.go                  # portable no-op/synthetic backend (build tag !darwin)
      fake.go                  # in-memory fake used by tests (all platforms)
    mru/                       # most-recently-used window tracker (PURE)
  frontend/src/
    overlay/                   # the switcher overlay React UI (styles: thumbnails/icons/titles)
    settings/                  # preferences UI
    lib/                       # pure TS: keymap, layout math, formatting (unit-tested)
```

### 3.1 The platform port (key seam)

```go
type WindowSource interface { Windows() ([]domain.Window, error) }
type Focuser interface {
    Focus(domain.WindowID) error
    Close(domain.WindowID) error
    Minimize(domain.WindowID) error
    QuitApp(domain.AppID) error
    HideApp(domain.AppID) error
}
type Thumbnailer interface { Thumbnail(domain.WindowID, maxPx int) (image.Image, error) }
type HotkeyEngine interface {
    Register(id int, chord hotkey.Chord) error
    Unregister(id int) error
    // Events() emits press/repeat(Tab)/modifier-release so the controller can drive hold-to-cycle.
    Events() <-chan hotkey.Event
}
type Permissions interface { Accessibility() PermState; ScreenRecording() PermState; Request(PermKind) }
type ScreenSource interface { Screens() []domain.Screen; CursorScreen() domain.Screen }
type LoginItem interface { Enabled() bool; SetEnabled(bool) error }
```

The macOS adapter implements these with: `CGWindowListCopyWindowInfo` (enumeration),
`AXUIElement` (focus/close/minimize/raise + per-app windows), ScreenCaptureKit /
`CGWindowListCreateImage` (thumbnails), Carbon `RegisterEventHotKey` + a `CGEventTap`
flags-changed watcher (hotkeys + modifier-release), `AXIsProcessTrusted` /
`CGPreflightScreenCaptureAccess` (permissions), `SMAppService` (login item), `NSScreen`
(screens). The stub returns synthetic data so the app runs and the UI is demoable on any
OS.

### 3.2 The switching cycle (data flow)

1. `HotkeyEngine` emits a press event for shortcut N.
2. `switcher.Controller` asks `WindowSource` for windows, applies `filter` (using shortcut
   N's scope) → `order` → builds a `SwitcherList`. It seeds selection at index 1 (the
   previous window) for instant back-and-forth.
3. Controller emits a `switcher:show` Wails event with the serialized list; thumbnails are
   fetched lazily/async and pushed via `switcher:thumbnail` events.
4. Frontend renders the overlay in the configured style; user input (Tab, arrows, hover,
   typing-to-search) is sent back as `switcher:input` calls → Controller updates selection
   / search filter and emits `switcher:update`.
5. On modifier release (or click/Return) Controller calls `Focuser.Focus(selected)` and
   emits `switcher:hide`. On Esc/cancel it just hides and restores prior focus.
6. `mru` tracker observes focus changes to keep MRU order fresh.

## 4. Testing strategy (full coverage emphasis)

- **Pure-Go packages** (`domain`, `config`, `filter`, `order`, `search`, `hotkey`, `mru`,
  and `switcher`'s state machine) → exhaustive table-driven tests with testify. Target
  near-100% line coverage on these; they hold all real logic.
- **`switcher.Controller`** is tested end-to-end against `platform/fake.go` (a scripted
  in-memory platform): assert that a press→cycle→release sequence focuses the right window,
  that filters/order/search apply, that cancel restores focus, that per-shortcut scopes
  work — no OS calls.
- **CGO adapter (`darwin.go`)** holds no decision logic (just translation), so it is kept
  as thin as possible and excluded from the coverage bar; a small `//go:build darwin`
  smoke test asserts it constructs and that enumeration returns without error on CI macOS.
- **Frontend**: pure TS (`lib/` keymap, layout math, formatting) unit-tested with Vitest;
  overlay/settings components tested with Testing Library (render the three styles, key
  handling, search box). 
- **E2E**: a Playwright/dev-harness page mounts the overlay against a mock event bus
  (recorded `switcher:show`/`update` payloads) to verify the three visual styles, keyboard
  navigation, and search interactively — no native deps, runs in CI.
- All existing scaffold tests stay green; the placeholder `greeter` unit is removed once
  `switcher` replaces it.

## 5. Configuration model (sketch)

`config.Settings` (JSON at `~/Library/Application Support/option-tab/settings.json` on
macOS, XDG/AppData elsewhere): `Shortcuts []Shortcut` (chord + scope + style override),
global `Appearance` (style, theme, sizes, colors, blur), global `Filters`, `Order`,
`Placement`, `Behavior` (hold-vs-press, start-at-login, paused). Ships with sensible
AltTab-like defaults; invalid configs fall back to defaults with a logged warning;
versioned for forward migration.

## 6. Phasing (incremental, each phase independently green)

1. **Core domain + config** — `domain`, `config` with full tests. *(no UI)*
2. **Logic packages** — `filter`, `order`, `search`, `mru`, `hotkey` parsing, with full
   tests.
3. **Platform port + fake + stub** — interfaces, `fake.go`, `stub.go`; `switcher.Controller`
   driven entirely by the fake, fully tested. App runs on any OS with synthetic windows.
4. **Overlay UI** — React overlay (3 styles) + settings UI, wired to Controller events;
   frontend unit/component tests + e2e harness.
5. **macOS native backend** — CGO `darwin.go`: enumeration, focus, thumbnails, hotkeys,
   permissions, login item, panel-style overlay window. Manual + smoke verification.
6. **Polish & landing page** — update `apps/web` landing copy/screenshots to the real
   feature set; docs; final full-suite verification.

## 7. Success criteria

- `task test` and `task lint` pass repo-wide; pure-Go logic packages have comprehensive
  table-driven coverage; `switcher.Controller` is verified against the fake platform.
- `wails dev`/`wails build` produces a running macOS app where ⌥Tab opens the overlay,
  cycles windows with live thumbnails, searches by typing, and focuses on release.
- All three visual styles, up to 9 configurable shortcuts, the documented filters, and the
  ordering modes work and are configurable in the settings UI.
- Non-macOS builds compile and test green via the stub.
- The landing page reflects the real, fully-free feature set.

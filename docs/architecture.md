# Architecture

## Overview

option-tab is a monorepo containing three deployable units:

| Unit | Technology | Output |
|------|-----------|--------|
| `apps/desktop` | Go + React (Wails) | Native binary |
| `apps/web` | Next.js (static export) | GitHub Pages site |
| `packages/shared` | TypeScript (library) | Consumed by `apps/web` and the release pipeline |

The desktop app is the product: a window switcher with feature parity to AltTab. The web
app is a marketing site and is never bundled into the binary.

---

## Desktop app: ports-and-adapters

All decision logic is **pure Go behind interfaces** and is exhaustively unit-tested. Every
OS interaction lives behind a single **platform port**, with swappable backends.

```
┌──────────────────────────────────────────────┐
│  React overlay + settings (frontend/src)      │
│  Overlay/Settings ← state via Wails events    │  ←─ switcher:show / :update / :hide
│  actions → lib/bridge.ts → bound Go methods   │
└───────────────────────┬──────────────────────┘
                        │ Wails runtime (IPC)
┌───────────────────────▼──────────────────────┐
│  app.go  (Wails-bound adapter)                │  ←─ thin; owns platform + controller,
│  implements switcher.View via Wails events    │     relays hotkey events, no logic
└───────────────────────┬──────────────────────┘
                        │
┌───────────────────────▼──────────────────────┐
│  internal/switcher.Controller (state machine) │  ←─ activate, cycle, search, confirm,
│  depends only on the platform port + pure pkgs│     window controls — fully testable
└───────┬───────────────────────────────┬──────┘
        │ pure logic                     │ platform port (interfaces)
┌───────▼───────────────┐   ┌────────────▼─────────────────────────┐
│ domain, config, filter│   │ internal/platform                     │
│ order, search, mru,   │   │  darwin.go/.m/.h  (CGO macOS backend) │
│ hotkey                │   │  stub.go          (!darwin, synthetic)│
└───────────────────────┘   │  fake/            (in-memory, tests)  │
                            └───────────────────────────────────────┘
```

### Pure-Go packages (`internal/`)

| Package | Responsibility |
|---------|----------------|
| `domain` | Core value types: `Window`, `App`, `Screen`, `Space`, `Bounds` (+ geometry helpers). |
| `config` | `Settings` model, AltTab-like defaults, validation/normalization, versioned JSON load/save. |
| `filter` | Selects which windows to show from the global filters + a per-shortcut scope. |
| `order` | Display ordering: most-recently-used, alphabetical, by space. |
| `search` | Fuzzy, ranked subsequence matching for the type-to-filter box. |
| `mru` | Thread-safe most-recently-used window tracker. |
| `hotkey` | Parses/canonicalizes chords like `option+tab` into modifiers + key. |
| `switcher` | The `Controller` state machine and `View` interface; the heart of the app. |

### The platform port (`internal/platform`)

`platform.go` defines the required interfaces (`WindowSource`, `Focuser`, `Thumbnailer`,
`Environment`, `HotkeyEngine`, `Permissions`, `LoginItem`) and the aggregate `Platform`,
plus **optional capability ports** that only richer backends provide and consumers
type-assert for: `IconSource` (app icons as data URLs), `ThumbnailSource`
(ScreenCaptureKit window snapshots), `Tray` (menubar item), `CursorWarper`
(cursor-follows-focus), `SettingsOpener` (System Settings privacy panes),
`WindowModer` (overlay ↔ titled preferences window), and `DockHider` (accessory
activation policy). Adding a capability means adding an optional port, not widening
`Platform`. Three backends implement them:

- **`darwin.go` + `darwin.m` + `darwin.h`** — the macOS backend. Window enumeration via
  `CGWindowList` enriched with AX minimized state, CGS Space ids, and per-display facts;
  focus/close/minimize/fullscreen via the Accessibility API; app quit/hide via
  `NSRunningApplication`; live thumbnails and selected-window previews via
  ScreenCaptureKit (macOS 14+); app icons from `NSRunningApplication`; permission
  checks/requests; login item via `SMAppService`; a `CGEventTap`-based hotkey engine that
  detects modifier release for hold-to-cycle; an `NSStatusBar` menubar item behind the
  `Tray` port; and window-presentation control — `ot_window_set_prefs_mode` flips the
  single window between borderless overlay and titled preferences chrome, and
  `ot_hide_dock_icon` keeps the app out of the Dock (with `LSUIElement` in the bundle).
- **`stub.go`** (`//go:build !darwin`) — synthetic data so Windows/Linux builds compile and
  the UI is demoable; full native backends for those platforms are future work.
- **`fake/`** — a scriptable in-memory backend used to drive the controller in tests.

### Frontend (`apps/desktop/frontend/src`)

The UI is a "frosted aurora" glassmorphism system: Tailwind CSS v4 design tokens live in
`styles.css` (mapped to shadcn conventions via `@theme`), and `components/ui/` holds the
shadcn-style primitives. Select/Checkbox/Radio/Slider deliberately wrap **native** form
elements (styled, not Radix) so the controlled-form tests can drive them with real change
events.

| Area | Files |
|------|-------|
| Pure logic | `lib/keymap.ts` (key → action), `lib/layout.ts` (grid math + size presets), `lib/chord.ts` (keydown → hotkey chord), `lib/text.ts` (title truncation), `lib/i18n.ts` (en/pt-BR/es), `lib/types.ts` (state mirror) |
| Bridge | `lib/bridge.ts` — Wails events + bound methods, degrading gracefully without Wails; `hooks/useBridge.ts` — permission/about/crash hooks over it |
| Design system | `components/ui/` — glass Button, Input, Select, Checkbox, Radio, Slider, Card, Badge, Segmented, Separator |
| Overlay | `overlay/` — `Overlay.tsx` (keyboard + layout), `EntryItem.tsx` (one window in any style), `StatusIcons.tsx`, `types.ts` (`OverlayHandlers`) |
| Settings | `settings/` — `Settings.tsx` (shell + tab nav), `tabs/` (one file per tab), `shared.ts` (TabContext + control interfaces), `Onboarding.tsx` (first-run wizard), `ShortcutRecorder.tsx`, `PermissionRow.tsx` |
| Shell | `App.tsx` — routes between the overlay, the preferences panel, and the `#settings`/`#demo` windows |

The package-`main` `app*.go` files are the only Go code that imports `wails/v2`. `main.go`
configures the frameless, transparent, always-on-top, start-hidden window and embeds
`frontend/dist` via `//go:embed`.

### Single-window model

Wails v2 supports one window, so the overlay and the preferences UI share it. Opening
preferences (menubar item, or automatically on first launch while `behavior.onboarded` is
false) flips the window into titled chrome via the `WindowModer` port and emits
`prefs:open`; the frontend renders the settings panel — or the onboarding wizard on first
run. Closing (native close button, intercepted by `OnBeforeClose`) emits `prefs:close`
and restores the overlay chrome. If the hotkey fires while preferences are open, the
switcher takes over and preferences are dismissed first.

---

## Landing page: static Next.js site

`apps/web` is a static-export Next.js 15 site (`output: "export"`). Its download buttons:

1. `PrimaryDownload` calls `detectPlatform(navigator.userAgent)` from `lib/download.ts`.
2. Both `PrimaryDownload` and `DownloadButtons` call `downloadUrl(platform, arch, version)`
   from `@option-tab/shared` to build the GitHub Release asset URL.
3. The advertised version is `APP_VERSION` in `apps/web/lib/download.ts` — bump it in
   lockstep with a release tag.

---

## Shared contract: `@option-tab/shared`

`packages/shared/src/index.ts` is the single source of truth for `PRODUCT` metadata,
`releaseAssetName(platform, arch, version)`, `downloadUrl(...)`, and `latestReleaseUrl()`.
It is the seam between what `release.yml` publishes and what the landing page links to;
changing the release matrix means updating this contract in the same commit.

> **Current limitation:** `release.yml` builds an `amd64 tar.gz` for all three platforms,
> while the contract defines platform-specific extensions (`darwin → dmg`, `windows → zip`,
> `linux → tar.gz`). These are not yet aligned — see [docs/release.md](release.md).

---

## Build orchestration

**Turborepo** (`turbo.json`) orchestrates workspace scripts with dependency ordering
(`build` → `^build`; `lint`/`test` → `^build`; `e2e` → `build`). **Taskfile**
(`Taskfile.yml`) is the human-facing cross-language entrypoint (`task lint/test/build/e2e/
dev:desktop/dev:web`). Wails handles the desktop-specific steps (embedding the frontend,
CGO, native WebKit dependencies).

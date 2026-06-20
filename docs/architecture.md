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

`platform.go` defines the interfaces (`WindowSource`, `Focuser`, `Thumbnailer`,
`Environment`, `HotkeyEngine`, `Permissions`, `LoginItem`) and the aggregate `Platform`.
Three backends implement them:

- **`darwin.go` + `darwin.m` + `darwin.h`** — the macOS backend. Window enumeration via
  `CGWindowList`, focus/close/minimize via the Accessibility API, app quit/hide via
  `NSRunningApplication`, permission checks/requests, login item via `SMAppService`, and a
  `CGEventTap`-based hotkey engine that detects modifier release for hold-to-cycle.
- **`stub.go`** (`//go:build !darwin`) — synthetic data so Windows/Linux builds compile and
  the UI is demoable; full native backends for those platforms are future work.
- **`fake/`** — a scriptable in-memory backend used to drive the controller in tests.

### Frontend (`apps/desktop/frontend/src`)

| Area | Files |
|------|-------|
| Pure logic | `lib/keymap.ts` (key → action), `lib/layout.ts` (grid math), `lib/types.ts` (state mirror) |
| Bridge | `lib/bridge.ts` — Wails events + bound methods, degrading gracefully without Wails |
| Overlay | `overlay/Overlay.tsx` — the switcher in three visual styles, with keyboard + controls |
| Settings | `settings/Settings.tsx` — the controlled preferences form |
| Shell | `App.tsx` — routes between the overlay and the `#settings` window |

`app.go` is the only Go file that imports `wails/v2`. `main.go` configures the frameless,
transparent, always-on-top, start-hidden overlay window and embeds `frontend/dist` via
`//go:embed`.

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

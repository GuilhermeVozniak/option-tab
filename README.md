# option-tab

[![CI](https://github.com/GuilhermeVozniak/option-tab/actions/workflows/ci.yml/badge.svg)](https://github.com/GuilhermeVozniak/option-tab/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

A free, open-source **window switcher** for macOS (with Windows/Linux builds) — the
Windows-style `Alt`+`Tab` experience, rebuilt in [Wails](https://wails.io) (Go + React).
Switch by **window**, not just by app, with live thumbnails, fuzzy search, and up to nine
custom shortcuts. Every feature AltTab gates behind its "Pro" tier ships here **100% free**.

## Features

- **Window-level switching** across every app, triggered by a global hotkey (default ⌥Tab).
- **Hold-to-cycle**: hold the modifier, tap Tab to advance / Shift+Tab to go back, release to focus.
- **Three visual styles** — Thumbnails, App Icons, and Titles — switchable per shortcut.
- **Fuzzy search**: start typing to filter by window title or app name.
- **Auto-sizing** thumbnails that scale to the number of open windows.
- **Up to 9 independent shortcuts**, each with its own filter scope and style.
- **Filters**: by active space / all spaces, by screen or cursor screen, minimized, hidden
  apps, fullscreen, untitled windows, and an app blacklist.
- **Display order**: most-recently-used, alphabetical, or by space.
- **Window controls** from the switcher: focus, close, minimize, hide, quit.
- **Three sizes** — Small / Medium / Large presets for thumbnails and icons, plus
  fine-grained pixel knobs under Advanced.
- **Shortcut recorder**: focus the field and press the chord (e.g. ⌥⇥) — no typing chord names.
- **First-run onboarding** that walks through the Accessibility and Screen Recording grants,
  with live status that updates as you grant them in System Settings.
- **Menubar-only app**: no Dock icon (accessory app, like AltTab), with a menubar icon for
  Preferences, Pause/Resume, and Quit; preferences open in a regular titled window.
- **Glassmorphism UI** built on Tailwind CSS v4 + shadcn-style components: frosted panels
  over an aurora backdrop, light/dark themes, accent color, start at login.
- **Quality of life**: update checks against GitHub releases, opt-in crash reports via
  prefilled GitHub issues, and a UI translated to English, Portuguese, and Spanish.

## Architecture at a glance

| Piece | What it is |
|-------|------------|
| `apps/desktop` | The **product**: a Wails app — pure-Go switcher core behind a platform port, with a macOS CGO backend and a React overlay UI |
| `apps/web` | The **landing page**: a static Next.js site deployed to GitHub Pages with OS-aware download buttons |
| `packages/shared` | A TypeScript contract defining product metadata and release-asset naming — the seam between the release pipeline and the landing page |

The pure-Go core (`internal/{domain,config,filter,order,search,mru,hotkey,switcher}`) holds
all decision logic and is exhaustively unit-tested. Every OS interaction sits behind the
`internal/platform` port, with a macOS CGO adapter, a portable stub for other OSes, and an
in-memory fake for tests.

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.23+ | <https://go.dev/dl> |
| Bun | 1.1+ | <https://bun.sh> |
| Wails CLI | v2 | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |
| Task | latest | <https://taskfile.dev/installation> |
| golangci-lint | v2 | <https://golangci-lint.run/welcome/install> |
| gofumpt | latest | `go install mvdan.cc/gofumpt@latest` |

On macOS, the app needs **Accessibility** permission (global hotkey + window actions) and
**Screen Recording** permission (live thumbnails via ScreenCaptureKit, macOS 14+). The
first-run onboarding wizard walks through both grants.

## Quickstart

```bash
bun install            # install JS/TS dependencies (all workspaces)
task dev:desktop       # run the desktop app in Wails dev mode (hot-reload)
task dev:web           # or run the landing-page dev server
```

## Monorepo layout

```
option-tab/
├── apps/
│   ├── desktop/                  # Wails desktop app — THE PRODUCT
│   │   ├── app*.go               # Thin Wails adapter, split by concern (switcher view,
│   │   │                         #   prefs window/tray, settings, updates, crash, permissions)
│   │   ├── main.go               # Frameless, transparent, always-on-top overlay window
│   │   ├── internal/
│   │   │   ├── domain/           # Core value types (Window, App, Screen, Space, Bounds)
│   │   │   ├── config/           # Settings model, defaults, versioned JSON load/save
│   │   │   ├── filter/           # Window selection by filters + per-shortcut scope
│   │   │   ├── order/            # MRU / alphabetical / by-space ordering
│   │   │   ├── search/           # Fuzzy type-to-filter
│   │   │   ├── mru/              # Most-recently-used tracker
│   │   │   ├── hotkey/           # Chord parsing/canonicalization
│   │   │   ├── switcher/         # The controller state machine (View-driven)
│   │   │   ├── update/           # GitHub "latest release" parsing + version compare
│   │   │   ├── crash/            # Crash-log rotation for opt-in reports
│   │   │   └── platform/         # The OS port: darwin (CGO), stub (!darwin), fake (tests)
│   │   └── frontend/             # React UI: Tailwind v4 + shadcn-style glass components
│   │       └── src/
│   │           ├── components/ui/  # Button, Input, Select, Checkbox, Card, Segmented, …
│   │           ├── overlay/        # The switcher: Overlay, EntryItem, StatusIcons
│   │           ├── settings/       # Preferences shell + tabs/, onboarding, recorder
│   │           ├── hooks/          # Bridge-backed hooks (permissions, about, crash)
│   │           └── lib/            # Pure logic: keymap, layout, chord, i18n, bridge
│   └── web/                      # Static Next.js landing page
├── packages/
│   └── shared/                   # @option-tab/shared — release-asset naming contract
├── .github/workflows/           # ci.yml, release.yml, deploy-web.yml
├── Taskfile.yml                 # Cross-language build entrypoint
├── turbo.json                   # Turborepo pipeline
├── biome.json                   # JS/TS formatter + linter config
└── lefthook.yml                 # Git hooks (pre-commit, pre-push, commit-msg)
```

## Available tasks

```bash
task lint          # Biome (JS/TS) + golangci-lint (Go)
task test          # Vitest (JS/TS) + go test (Go), with race detector
task build         # next build + wails build (requires Wails CLI)
task e2e           # Playwright smoke tests against apps/web
task dev:desktop   # Wails dev mode with hot-reload
task dev:web       # Next.js dev server for the landing page
```

## Further reading

- [Architecture](docs/architecture.md)
- [Development guide](docs/development.md)
- [Testing guide](docs/testing.md)
- [Release process](docs/release.md)
- [Contributing](CONTRIBUTING.md)

## Credits

Inspired by [AltTab](https://alt-tab.app) ([lwouis/alt-tab-macos](https://github.com/lwouis/alt-tab-macos)),
reimplemented from scratch in Go & Wails with all paid features free for everyone.

## License

Apache-2.0 — see [LICENSE](LICENSE).

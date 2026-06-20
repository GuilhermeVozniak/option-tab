# Architecture

## Overview

option-tab is a monorepo containing three deployable units:

| Unit | Technology | Output |
|------|-----------|--------|
| `apps/desktop` | Go + React (Wails) | Native binary |
| `apps/web` | Next.js (static export) | GitHub Pages site |
| `packages/shared` | TypeScript (library) | Consumed by `apps/web` and the release pipeline |

---

## Repository layout

```
option-tab/
├── apps/
│   ├── desktop/               # Wails desktop application
│   │   ├── app.go             # Wails-bound adapter layer
│   │   ├── main.go            # Entry point (embeds frontend/dist)
│   │   ├── internal/
│   │   │   └── greeter/       # Business unit: interface, impl, tests
│   │   └── frontend/          # React UI (Vite, Vitest, Testing Library)
│   │       └── src/
│   │           ├── lib/       # Pure-TS helpers (desktop.ts, name.ts)
│   │           └── components/
│   └── web/                   # Static Next.js landing page
│       ├── app/               # Next.js App Router
│       ├── components/        # PrimaryDownload, DownloadButtons
│       ├── lib/               # download.ts (detectPlatform, APP_VERSION)
│       └── e2e/               # Playwright smoke tests
├── packages/
│   └── shared/                # @option-tab/shared
│       └── src/index.ts       # PRODUCT, releaseAssetName, downloadUrl, latestReleaseUrl
├── .github/workflows/
│   ├── ci.yml                 # PR + main branch checks
│   ├── release.yml            # Tag-triggered desktop build and GitHub Release
│   └── deploy-web.yml         # Landing page deploy to GitHub Pages
├── Taskfile.yml               # Cross-language entrypoint (wraps Turbo + direct commands)
├── turbo.json                 # Turborepo pipeline (build, lint, test, e2e, dev)
├── biome.json                 # Biome formatter + linter for JS/TS/JSON
└── lefthook.yml               # Git hooks
```

---

## Desktop app: hexagonal-lite pattern

The desktop app follows a hexagonal (ports-and-adapters) lite approach:

```
┌───────────────────────────────────────┐
│  React frontend (apps/desktop/frontend)│
│  greet() via lib/desktop.ts           │  <── reads Wails global go.main.App
└─────────────────┬─────────────────────┘
                  │ Wails runtime (IPC)
┌─────────────────▼─────────────────────┐
│  app.go  (Wails-bound adapter)        │  <── thin; no business logic
│  App.Greet(name) → greeter.Greet(name)│
└─────────────────┬─────────────────────┘
                  │ depends on interface
┌─────────────────▼─────────────────────┐
│  internal/greeter  (business unit)    │
│  Greeter interface + DefaultGreeter   │
└───────────────────────────────────────┘
```

**Key rules:**

- `app.go` is the only file that imports `wails/v2`. It holds service interfaces (e.g., `greeter.Greeter`), not concrete types, so it can be tested via mocks.
- All business logic lives in `internal/<unit>/`. Units are Go packages with a public interface and a production implementation.
- The React frontend calls Go through `apps/desktop/frontend/src/lib/desktop.ts`, which reads the Wails runtime global (`go.main.App`) at call time. This avoids importing the generated `wailsjs/` directory so the frontend builds standalone in CI without a running Wails process.
- `main.go` embeds `frontend/dist` via `//go:embed all:frontend/dist`. The frontend must be built before `wails build` (Wails handles this automatically via `wails dev` / `wails build`).

---

## Landing page: static Next.js site

`apps/web` is a static-export Next.js 15 site (`output: "export"`). It is a marketing site, not a web version of the desktop product.

The landing page's download buttons work as follows:

1. `PrimaryDownload` (client component) calls `detectPlatform(navigator.userAgent)` from `lib/download.ts` to detect the visitor's OS.
2. Both `PrimaryDownload` and `DownloadButtons` call `downloadUrl(platform, arch, version)` from `@option-tab/shared` to construct the GitHub Release asset URL.
3. The version advertised is `APP_VERSION` in `apps/web/lib/download.ts` — bump this in lockstep with a release tag.

---

## Shared contract: `@option-tab/shared`

`packages/shared/src/index.ts` is the single source of truth for:

- `PRODUCT` — name, display name, GitHub repo URL.
- `releaseAssetName(platform, arch, version)` — canonical asset filename (`option-tab_<version>_<platform>_<arch>.<ext>`).
- `downloadUrl(platform, arch, version)` — full GitHub Release asset URL.
- `latestReleaseUrl()` — URL of the latest release page.

This package is the **seam** between what `release.yml` publishes and what the landing page links to. When the release matrix changes (e.g., adding arm64 or switching from `.tar.gz` to `.dmg`), the extension map in `packages/shared/src/index.ts` must be updated in the same commit.

> **Current limitation:** `release.yml` builds an `amd64 tar.gz` for all three platforms. The `@option-tab/shared` contract defines platform-specific extensions (`darwin → dmg`, `windows → zip`, `linux → tar.gz`). These are not yet aligned. See [docs/release.md](release.md) for the documented follow-ups.

---

## Build orchestration

Two layers handle the build:

**Turborepo** (`turbo.json`) orchestrates workspace-level scripts with dependency ordering:

- `build` depends on `^build` (workspace deps built first).
- `lint` and `test` depend on `^build` (shared contract compiled before consumers lint/test against it).
- `e2e` depends on `build` (static site must exist before Playwright runs).
- `dev` is persistent with no cache.

**Taskfile** (`Taskfile.yml`) provides the human-facing cross-language entrypoint:

- `task lint` → `bun run lint` (Turbo → Biome across JS/TS workspaces) + `golangci-lint` (Go).
- `task test` → `bun run test` (Turbo → Vitest) + `go test ./... -race -cover` (Go).
- `task build` → `bun run build` (Turbo → Next.js) + `wails build` (Go + embedded frontend).
- `task e2e` → Playwright directly in `apps/web`.
- `task dev:desktop` / `task dev:web` → Wails dev mode / Next.js dev server.

Turborepo caches build/lint/test artifacts. Wails (`wails build`, `wails dev`) handles the desktop-specific build steps (embedding the frontend, CGo, native WebKit dependencies).

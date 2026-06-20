# option-tab — Monorepo Scaffold Design

**Date:** 2026-06-20
**Status:** Approved (pending spec review)
**Type:** Greenfield scaffold

## 1. Purpose

`option-tab` is an open-source, free desktop application distributed to end users. This
spec defines the **scaffold** for its monorepo: structure, tooling, testing patterns, git
hooks, and CI/CD. No product/domain features are implemented here — only the foundation
and representative example code that establishes every pattern.

Two deliverables live in the repo:

1. **The desktop app** (`apps/desktop`) — the actual product, built with Wails (Go
   backend + web frontend), distributed as cross-platform binaries. This is where the
   testability emphasis goes.
2. **The landing page** (`apps/web`) — a Next.js marketing site that advertises the
   product and provides per-OS download buttons. It is *not* an app that consumes desktop
   logic; it is a static, deployable brochure.

## 2. Decisions (locked)

| Area | Decision |
| --- | --- |
| App topology | Separate frontends. Desktop = Wails (Go + own Vite/React UI). Web = independent Next.js landing page. |
| JS orchestration | Turborepo + Bun workspaces |
| Package manager / runtime | Bun 1.x |
| Go linting | golangci-lint + gofumpt |
| JS/TS lint + format | Biome 2.x |
| Git hooks | Lefthook |
| CI/CD | GitHub Actions — full cross-platform desktop releases + landing-page deploy |
| Testing | Full pyramid: Go (stdlib + testify, table-driven, interface mocks); Vitest + Testing Library; Playwright E2E |
| License | Apache-2.0 |
| Wails version | v2 (stable); v3 noted as future migration |

## 3. Repository topology

```
option-tab/
├── apps/
│   ├── desktop/                 # Wails v2 — THE PRODUCT
│   │   ├── main.go              # Wails bootstrap
│   │   ├── app.go               # App struct: thin adapter, Wails-bound methods
│   │   ├── app_test.go          # binding layer test (mocked services)
│   │   ├── wails.json
│   │   ├── go.mod
│   │   ├── internal/            # Pure Go business logic behind interfaces
│   │   │   └── greeter/         #   example unit: greeter.go + greeter_test.go
│   │   ├── frontend/            # Vite 6 + React 19 + TS
│   │   │   ├── src/
│   │   │   │   ├── lib/         # pure logic (unit-tested)
│   │   │   │   └── components/  # React components (Testing Library)
│   │   │   ├── vitest.config.ts
│   │   │   └── package.json
│   │   └── build/               # Wails build assets (icons, etc.)
│   └── web/                     # Next.js 15 landing page — independent
│       ├── app/                 # App Router: hero, features, downloads
│       ├── components/
│       ├── lib/                 # pure logic (download-link resolution, etc.)
│       ├── e2e/                 # Playwright smoke tests
│       ├── vitest.config.ts
│       ├── playwright.config.ts
│       └── package.json
├── packages/
│   └── shared/                  # Product-metadata contract (pure TS, fully tested)
│       ├── src/                 #   product name, repo URL, version, per-OS asset naming
│       └── package.json
├── docs/
│   ├── architecture.md
│   ├── development.md
│   ├── testing.md
│   └── release.md
├── .github/
│   └── workflows/
│       ├── ci.yml               # PR checks: lint + test + build smoke
│       ├── release.yml          # tag v* : cross-platform desktop binaries → GitHub Release
│       └── deploy-web.yml        # landing page → GitHub Pages
├── biome.json                   # JS/TS lint + format config
├── turbo.json                   # JS task graph + caching
├── lefthook.yml                 # git hooks
├── Taskfile.yml                 # cross-language entrypoint (wraps turbo + go/wails)
├── go.work                      # Go workspace (future-proof multi-module)
├── .golangci.yml                # Go lint config (incl. gofumpt)
├── package.json                 # root, Bun workspaces
├── bun.lock
├── .gitignore
├── LICENSE                       # Apache-2.0
├── CONTRIBUTING.md
└── README.md
```

### Cross-language orchestration

`Taskfile.yml` (Taskfile.dev) is the single human-facing entrypoint. It provides uniform
verbs across both languages:

- `task dev` — run desktop (`wails dev`) or web (`turbo dev`) targets
- `task lint` — Biome (JS/TS) + golangci-lint (Go)
- `task test` — Vitest + `go test ./...`
- `task build` — `turbo build` + `wails build`
- `task e2e` — Playwright

Turborepo owns the JS task graph + caching; Taskfile delegates JS verbs to `turbo` and
runs Go/Wails commands directly. This keeps one obvious command set regardless of language.

## 4. Tech stack (fresh)

- **Desktop:** Wails v2, Go 1.23+, Vite 6, React 19, TypeScript 5
- **Web (landing):** Next.js 15 (App Router, static export), React 19, TypeScript 5
- **Tooling:** Bun 1.x, Turborepo 2.x, Biome 2.x, golangci-lint + gofumpt, Lefthook, Taskfile.dev
- **Testing:** Go stdlib `testing` + testify; Vitest 3 + @testing-library/react; Playwright

## 5. Testability pattern

The scaffold's primary job is to make the *right* way the *easy* way. Every package ships
at least one real example test demonstrating its pattern.

### Desktop Go (hexagonal-lite) — the emphasis

- All business logic lives in `internal/<unit>/`, expressed behind small interfaces.
- `app.go` (the Wails-bound layer) is a **thin adapter**: it holds service interfaces and
  delegates. It contains no business logic, so it needs minimal testing.
- Services receive their dependencies as interfaces → trivially mocked.
- Tests are **table-driven** with testify assertions. Example: `internal/greeter` ships
  `greeter.go` (a `Greeter` interface + impl) and a table-driven `greeter_test.go`.
- `app_test.go` shows testing the binding layer by injecting a mock `Greeter`.

### Frontend (desktop UI + landing page)

- Pure logic extracted into `lib/` → fast unit tests (Vitest).
- Components tested with @testing-library/react (behavior, not implementation).
- Web flows covered by Playwright: landing page renders and **download buttons resolve to
  valid release-asset URLs**.

### packages/shared

- Pure TypeScript: product constants + a function that, given an OS/arch, returns the
  expected release-asset filename and download URL. Fully unit-tested. This is the typed
  seam between what `release.yml` publishes and what the landing page links to.

## 6. Git hooks (Lefthook)

- **pre-commit:** Biome check+format on staged JS/TS; gofumpt + golangci-lint on staged
  Go; `tsc --noEmit` typecheck.
- **pre-push:** run unit/integration suites (`task test`).
- **commit-msg:** Conventional Commits validation (feeds clean release notes).

Hooks operate on staged files where possible to stay fast; CI re-runs everything
authoritatively.

## 7. CI/CD (GitHub Actions)

### ci.yml — on pull_request / push to main

Parallel jobs (Bun, Go, and Wails system deps cached):

- `lint` — Biome + golangci-lint
- `test-go` — `go test ./... -race -cover`
- `test-js` — Vitest (all JS workspaces) via Turbo
- `e2e` — Playwright against the built landing page
- `build` — `turbo build` + a `wails build` smoke on the runner OS

### release.yml — on tag `v*`

The desktop app is the product, so this is the important pipeline:

- Matrix `macos-latest / windows-latest / ubuntu-latest` → install Wails + system deps →
  `wails build` → upload platform binaries as assets to a GitHub Release.
- Asset names follow the convention defined in `packages/shared` so the landing page's
  download links resolve.
- Versioning is tag-driven; release notes derive from Conventional Commits.

### deploy-web.yml — on push to main (paths: apps/web) and after release

- `next build` static export → publish to GitHub Pages.
- Landing page download buttons point at the **latest** GitHub Release assets.

## 8. Documentation

- `README.md` — overview, prerequisites, quickstart (`task dev`), project map.
- `docs/architecture.md` — monorepo layout, the desktop/web split, the shared contract.
- `docs/development.md` — env setup (Go, Bun, Wails CLI), common `task` commands.
- `docs/testing.md` — the testability patterns and how to add tests at each layer.
- `docs/release.md` — how a tag becomes cross-platform binaries + a deployed landing page.
- `CONTRIBUTING.md` — workflow, commit conventions, hook expectations.
- `LICENSE` — Apache-2.0, with headers where appropriate.

## 9. Out of scope (YAGNI)

- Any product/domain feature of option-tab itself (only example/placeholder code).
- Auto-update mechanism, code signing/notarization (documented as future work in
  `docs/release.md`, not implemented).
- Web app backend / database — the landing page is fully static.
- Wails v3 migration.

## 10. Success criteria

- `task dev`, `task lint`, `task test`, `task build`, `task e2e` all run from a clean clone
  after documented setup.
- Each workspace has at least one passing example test demonstrating its pattern.
- A pushed `v*` tag produces a GitHub Release with macOS/Windows/Linux desktop binaries.
- Landing page builds, deploys, and its download buttons resolve to release assets.
- Lefthook hooks block unformatted / lint-failing commits.

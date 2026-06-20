# option-tab

[![CI](https://github.com/GuilhermeVozniak/option-tab/actions/workflows/ci.yml/badge.svg)](https://github.com/GuilhermeVozniak/option-tab/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

A cross-platform desktop application built with [Wails](https://wails.io) (Go + React), shipped via a monorepo that also hosts a static Next.js marketing site.

## What is option-tab?

| Piece | What it is |
|-------|------------|
| `apps/desktop` | The **product**: a Wails desktop app (Go backend + React frontend, distributed as a native binary) |
| `apps/web` | The **landing page**: a static Next.js site deployed to GitHub Pages with OS-aware download buttons |
| `packages/shared` | A TypeScript contract package defining product metadata and release-asset naming — the seam between the release pipeline and the landing page |

The desktop app is the distributed product. The web app is a marketing site and is never bundled into the binary.

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.23+ | <https://go.dev/dl> |
| Bun | 1.1+ | <https://bun.sh> |
| Wails CLI | v2 | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |
| Task | latest | <https://taskfile.dev/installation> |
| golangci-lint | v2 | <https://golangci-lint.run/welcome/install> |
| gofumpt | latest | `go install mvdan.cc/gofumpt@latest` |

## Quickstart

```bash
# 1. Install JS/TS dependencies (all workspaces)
bun install

# 2a. Run the desktop app in Wails dev mode (hot-reload)
task dev:desktop

# 2b. Or run the landing-page dev server
task dev:web
```

## Monorepo layout

```
option-tab/
├── apps/
│   ├── desktop/               # Wails desktop app
│   │   ├── app.go             # Thin Wails-bound adapter (no business logic)
│   │   ├── main.go            # Entry point; embeds frontend/dist
│   │   ├── internal/
│   │   │   └── greeter/       # Example business unit (interface + impl + tests)
│   │   └── frontend/          # React UI (Vite, Vitest, Testing Library)
│   └── web/                   # Static Next.js landing page
│       ├── app/               # Next.js App Router pages
│       ├── components/        # PrimaryDownload, DownloadButtons
│       ├── lib/               # download.ts (detectPlatform, APP_VERSION)
│       └── e2e/               # Playwright smoke tests
├── packages/
│   └── shared/                # @option-tab/shared — asset naming contract
├── .github/workflows/         # ci.yml, release.yml, deploy-web.yml
├── Taskfile.yml               # Cross-language build entrypoint
├── turbo.json                 # Turborepo pipeline
├── biome.json                 # JS/TS formatter + linter config
└── lefthook.yml               # Git hooks (pre-commit, pre-push, commit-msg)
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

## Testing

See [docs/testing.md](docs/testing.md) for the full testing strategy and per-layer examples.

## Further reading

- [Architecture](docs/architecture.md)
- [Development guide](docs/development.md)
- [Testing guide](docs/testing.md)
- [Release process](docs/release.md)
- [Contributing](CONTRIBUTING.md)

## License

Apache-2.0 — see [LICENSE](LICENSE).

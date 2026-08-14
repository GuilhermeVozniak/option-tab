# Development guide

## Tool installation

Install all prerequisites before running any `task` commands.

### Go 1.26+

```bash
# Download from https://go.dev/dl and follow the platform instructions.
go version  # should print go1.26.x or higher
```

### Bun 1.1+

```bash
curl -fsSL https://bun.sh/install | bash
bun --version  # should print 1.1.x or higher
```

### Wails CLI v3 (alpha)

The desktop app uses Wails v3 (`v3.0.0-alpha2.117`). The CLI is only needed for
`wails3 dev` and for regenerating the committed frontend bindings — builds are plain
`go build`:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
wails3 version  # should print v3.0.0-alpha...
```

On Linux, Wails also requires system WebKit headers:

```bash
sudo apt-get install libgtk-4-dev libwebkitgtk-6.0-dev
```

### Task (taskfile.dev)

```bash
# macOS/Linux via the install script:
sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b /usr/local/bin
task --version
```

### golangci-lint v2

```bash
# Use the official binary installer (do not use 'go install' — it may give an older version):
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
  | sh -s -- -b $(go env GOPATH)/bin v2.12.0
golangci-lint --version
```

### gofumpt

```bash
go install mvdan.cc/gofumpt@latest
gofumpt --version
```

---

## First-time setup

```bash
# Clone the repo
git clone https://github.com/GuilhermeVozniak/option-tab.git
cd option-tab

# Install JS/TS dependencies for all workspaces
bun install
# This also runs 'lefthook install' (via the prepare script) to wire up Git hooks.
```

---

## Available tasks

| Command | What it does |
|---------|-------------|
| `task lint` | Biome check (JS/TS/JSON) via Turbo + golangci-lint (Go) |
| `task test` | Vitest (all JS/TS workspaces) via Turbo + `go test ./... -race -cover` |
| `task build` | Turbo (landing page + frontend) + `go build` (desktop binary at `apps/desktop/bin/option-tab`) |
| `task bundle` | `scripts/bundle.sh` — assemble/sign/package the `.app` + dmg into `apps/desktop/build/bin` |
| `task e2e` | Playwright smoke tests against the built landing page |
| `task dev:desktop` | Frontend build + `wails3 dev` (restarts the Go process on Go changes) |
| `task dev:web` | Next.js dev server for the landing page |

> Note: `dev:desktop` and `dev:web` must be specified explicitly — there is no shorthand `dev` target.

---

## Desktop development workflow

### Running in dev mode

```bash
task dev:desktop
# Equivalent to: cd apps/desktop/frontend && bun run build && cd .. && wails3 dev
```

`wails3 dev` compiles and runs the Go binary, which serves the embedded frontend in the
app's two WebKit windows, and restarts the process when Go files change. There is no
frontend HMR: the app serves the embedded `frontend/dist`, so after editing frontend
files rebuild them (`cd apps/desktop/frontend && bun run build`, or keep
`vite build --watch` running) and let `wails3 dev` pick the change up. The overlay UI can
also be iterated on quickly in a plain browser (`cd apps/desktop/frontend && bun run dev`)
— the bridge degrades gracefully without a backend (DOM keydown replaces native key
events, bindings no-op).

### How Wails bindings work

`main.go` registers `*App` as a Wails v3 service (`application.NewService(app)`); every
exported method becomes callable from the frontend. The typed bindings are **generated and
committed** at `apps/desktop/frontend/bindings/`:

```bash
cd apps/desktop && wails3 generate bindings   # re-run after changing App's methods
```

The frontend never calls the bindings directly — it goes through
`apps/desktop/frontend/src/lib/bridge.ts`, which wraps them (plus `@wailsio/runtime`
`Events`) and degrades gracefully when no backend answers (browser dev, tests):

```ts
import { Events } from "@wailsio/runtime";
import * as AppService from "../../bindings/option-tab/app.js";

export const switcher = {
  advance: () => call(AppService.Advance()),
  // ...
};
```

This approach means:

- The frontend compiles and tests run without a live Wails process (the bindings module
  and `@wailsio/runtime` are mocked in tests).
- Binding method IDs are deterministic hashes of the Go method names; the e2e suite
  intercepts the `/wails/runtime` HTTP calls and answers them by id
  (`e2e/support/fakeWails.ts`).

### Go embed dependency

`main.go` uses `//go:embed all:frontend/dist` to bundle the React build into the binary.
The frontend must be built before `go build`; `task build` runs Turbo first, and
`scripts/bundle.sh` builds it again itself.

### Adding a new business unit

1. Create `apps/desktop/internal/<unit>/<unit>.go` with a public interface and production implementation.
2. Add the interface as a field on `App` in `app.go`; wire the production impl in `NewApp()`.
3. Expose the method(s) you need via receiver functions on `*App`.
4. Write tests in `internal/<unit>/<unit>_test.go` (table-driven; mock the interface for `app.go` tests).

---

## Landing page development workflow

```bash
task dev:web
# Equivalent to: cd apps/web && bun run dev
```

The landing page (`apps/web`) is a static-export Next.js 15 site. It uses the `@option-tab/shared` package (workspace dependency) for download URL construction. After changing `packages/shared`, run `bun install` or let Turbo's `^build` dependency propagate the update.

---

## Linting and formatting

| Layer | Tool | Config |
|-------|------|--------|
| JS/TS/JSON | Biome 2.x | `biome.json` (root) |
| Go | golangci-lint v2 + gofumpt | `.golangci.yml` (root) |

Lefthook runs Biome and gofumpt automatically on `git commit` (format-on-save for staged files). golangci-lint also runs on pre-commit for Go files.

To run manually:

```bash
task lint          # all layers
cd apps/desktop && golangci-lint run ./...   # Go only
bunx biome check --write .                   # JS/TS only (formats + lints)
```

# Development guide

## Tool installation

Install all prerequisites before running any `task` commands.

### Go 1.23+

```bash
# Download from https://go.dev/dl and follow the platform instructions.
go version  # should print go1.23.x or higher
```

### Bun 1.1+

```bash
curl -fsSL https://bun.sh/install | bash
bun --version  # should print 1.1.x or higher
```

### Wails CLI v2

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails version  # should print v2.x.x
```

On Linux, Wails also requires system WebKit headers:

```bash
sudo apt-get install libgtk-3-dev libwebkit2gtk-4.1-dev
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
| `task build` | `next build` (landing page) via Turbo + `wails build` (desktop binary) |
| `task e2e` | Playwright smoke tests against the built landing page |
| `task dev:desktop` | Wails dev mode — hot-reloads both Go and React |
| `task dev:web` | Next.js dev server for the landing page |

> Note: `dev:desktop` and `dev:web` must be specified explicitly — there is no shorthand `dev` target.

---

## Desktop development workflow

### Running in dev mode

```bash
task dev:desktop
# Equivalent to: cd apps/desktop && wails dev
```

`wails dev` does the following automatically:

1. Builds the React frontend (`apps/desktop/frontend`) and writes output to `frontend/dist`.
2. Starts the Vite dev server for the frontend (with hot module replacement).
3. Compiles and runs the Go binary, which serves the Wails WebKit window.

Changes to Go files restart the Go process. Changes to frontend files are hot-reloaded by Vite.

### How Wails bindings work

`main.go` binds `*App` to the Wails runtime via `Bind: []interface{}{app}`. Wails injects the bound methods on the JavaScript global `go.main.App` at runtime.

The frontend accesses these bindings through `apps/desktop/frontend/src/lib/desktop.ts`:

```ts
// desktop.ts reads the Wails global at call time rather than importing
// generated wailsjs/ files, so the frontend builds standalone in CI.
function wailsApp(): WailsApp {
  return (window as unknown as { go: { main: { App: WailsApp } } }).go.main.App;
}

export function greet(name: string): Promise<string> {
  return wailsApp().Greet(name);
}
```

This approach means:

- The frontend compiles and tests run without a live Wails process (the `wailsApp()` call is mocked in tests).
- Generated `wailsjs/` files are not checked in or imported.

### Go embed dependency

`main.go` uses `//go:embed all:frontend/dist` to bundle the React build into the binary. When running `wails build` directly (not via `wails dev`), the frontend must be built first. `task build` handles this via Turbo's dependency ordering (`bun run build` runs before `wails build`).

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

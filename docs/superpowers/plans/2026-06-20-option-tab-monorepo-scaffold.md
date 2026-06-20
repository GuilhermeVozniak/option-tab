# option-tab Monorepo Scaffold Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up an open-source monorepo scaffold for the `option-tab` Wails desktop app plus a Next.js marketing landing page, with fresh tooling, testability patterns, git hooks, and release/deploy CI.

**Architecture:** Bun-workspaces monorepo orchestrated by Turborepo (JS) and a top-level Taskfile (cross-language). The desktop app (`apps/desktop`) is a Wails v2 Go backend using a hexagonal-lite pattern (business logic behind interfaces in `internal/`, a thin Wails-bound adapter) with a Vite/React frontend. The landing page (`apps/web`) is a static-export Next.js site. `packages/shared` is a pure-TS product-metadata contract consumed by the web page and matched by the release pipeline.

**Tech Stack:** Go 1.23, Wails v2, Bun 1.x, Turborepo 2.x, Biome 2.x, golangci-lint + gofumpt, Next.js 15, React 19, Vite 6, TypeScript 5, Vitest 3, @testing-library/react, Playwright, Lefthook, Taskfile.dev, GitHub Actions, Apache-2.0.

## Global Constraints

- **Module / repo identity (single source of truth):** GitHub repo is `https://github.com/gui336699/option-tab`. The desktop Go module path is `option-tab`. The npm scope is `@option-tab`. The product slug used in release asset names is `option-tab`. These exact strings are used verbatim everywhere below; the web-facing copy of the repo URL lives only in `packages/shared/src/index.ts`.
- **Version floors:** Go `>= 1.23`, Bun `>= 1.1`, Node engines `>= 20` (for tooling that reads it), Wails CLI `v2` (latest v2.x), Next.js `15.x`, React `19.x`, Vite `6.x`, Biome `2.x`, Turborepo `2.x`, Vitest `3.x`, Playwright latest.
- **Package manager:** Bun only. Never generate `package-lock.json`/`pnpm-lock.yaml`/`yarn.lock`; the lockfile is `bun.lock`.
- **Commit convention:** Conventional Commits (`feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert`). Every commit message in this plan already conforms.
- **License:** Apache-2.0 for all source.
- **Next.js output:** static export (`output: "export"`) — no server runtime, deployable to GitHub Pages.
- **Wails frontend dir:** `apps/desktop/frontend`, build output `apps/desktop/frontend/dist` (embedded by Go via `//go:embed all:frontend/dist`).

---

## File Structure

| Path | Responsibility |
| --- | --- |
| `package.json`, `bun.lock` | Root Bun workspace manifest |
| `turbo.json` | JS task graph + caching |
| `biome.json` | JS/TS lint + format config |
| `Taskfile.yml` | Cross-language command entrypoint |
| `go.work` | Go workspace referencing `apps/desktop` |
| `.golangci.yml` | Go lint config (gofumpt + vet + staticcheck) |
| `lefthook.yml` | Git hooks |
| `.gitignore`, `LICENSE`, `README.md`, `CONTRIBUTING.md` | Repo hygiene + docs |
| `packages/shared/src/index.ts` (+ `.test.ts`) | Product metadata + release-asset URL resolver |
| `apps/desktop/go.mod`, `main.go`, `app.go` (+ `app_test.go`) | Wails bootstrap + thin bound adapter |
| `apps/desktop/internal/greeter/greeter.go` (+ `_test.go`) | Example business unit behind an interface |
| `apps/desktop/frontend/**` | Vite + React + TS UI, `lib/` pure logic + component |
| `apps/web/**` | Next.js static landing page, `lib/` download logic, Playwright e2e |
| `.github/workflows/{ci,release,deploy-web}.yml` | CI checks, desktop release, Pages deploy |
| `docs/{architecture,development,testing,release}.md` | Documentation |

---

## Task 1: Root monorepo foundation

**Files:**
- Create: `package.json`, `turbo.json`, `biome.json`, `Taskfile.yml`, `.gitignore`, `LICENSE`, `.npmrc`

**Interfaces:**
- Consumes: nothing.
- Produces: Bun workspaces (`apps/*`, `packages/*`); Taskfile verbs `dev|lint|test|build|e2e`; Turbo pipeline tasks `build|lint|test|dev`; Biome config used by every JS task.

- [ ] **Step 1: Create the root workspace manifest**

`package.json`:
```json
{
  "name": "option-tab",
  "private": true,
  "type": "module",
  "packageManager": "bun@1.1.38",
  "engines": { "bun": ">=1.1", "node": ">=20" },
  "workspaces": ["apps/*", "packages/*"],
  "scripts": {
    "build": "turbo run build",
    "lint": "turbo run lint",
    "test": "turbo run test",
    "dev": "turbo run dev"
  },
  "devDependencies": {
    "@biomejs/biome": "^2.0.0",
    "turbo": "^2.0.0",
    "typescript": "^5.6.0"
  }
}
```

- [ ] **Step 2: Create the Turbo pipeline**

`turbo.json`:
```json
{
  "$schema": "https://turbo.build/schema.json",
  "tasks": {
    "build": { "dependsOn": ["^build"], "outputs": ["dist/**", ".next/**", "out/**"] },
    "lint": { "dependsOn": ["^build"] },
    "test": { "dependsOn": ["^build"] },
    "e2e": { "dependsOn": ["build"] },
    "dev": { "cache": false, "persistent": true }
  }
}
```

- [ ] **Step 3: Create the Biome config**

`biome.json`:
```json
{
  "$schema": "https://biomejs.dev/schemas/2.0.0/schema.json",
  "vcs": { "enabled": true, "clientKind": "git", "useIgnoreFile": true },
  "files": { "ignoreUnknown": true },
  "formatter": { "enabled": true, "indentStyle": "space", "indentWidth": 2, "lineWidth": 100 },
  "linter": { "enabled": true, "rules": { "recommended": true } },
  "javascript": { "formatter": { "quoteStyle": "double" } }
}
```

- [ ] **Step 4: Create the Taskfile (cross-language entrypoint)**

`Taskfile.yml`:
```yaml
version: "3"

tasks:
  default:
    cmds: [task --list]
  lint:
    desc: Lint all code (Biome + golangci-lint)
    cmds:
      - bun run lint
      - cd apps/desktop && golangci-lint run ./...
  test:
    desc: Run all unit/integration tests (Vitest + go test)
    cmds:
      - bun run test
      - cd apps/desktop && go test ./... -race -cover
  build:
    desc: Build web + desktop
    cmds:
      - bun run build
      - cd apps/desktop && wails build
  e2e:
    desc: Run Playwright e2e for the landing page
    cmds:
      - cd apps/web && bunx playwright test
  dev:web:
    desc: Run the landing page dev server
    cmds: [cd apps/web && bun run dev]
  dev:desktop:
    desc: Run the desktop app in Wails dev mode
    cmds: [cd apps/desktop && wails dev]
```

- [ ] **Step 5: Create `.gitignore`, `.npmrc`, and the Apache-2.0 LICENSE**

`.gitignore`:
```
node_modules/
dist/
out/
.next/
.turbo/
apps/desktop/build/bin/
apps/desktop/frontend/dist/
coverage/
playwright-report/
test-results/
*.log
.DS_Store
```

`.npmrc`:
```
engine-strict=true
```

`LICENSE`: the full standard Apache License 2.0 text (fetch verbatim from https://www.apache.org/licenses/LICENSE-2.0.txt; copyright line: `Copyright 2026 option-tab contributors`).

- [ ] **Step 6: Install and verify the workspace resolves**

Run: `bun install`
Expected: completes, writes `bun.lock`, no workspace errors.

Run: `bunx turbo run build --dry=json | head`
Expected: prints a task graph (no packages yet is fine — exits 0).

- [ ] **Step 7: Commit**

```bash
git add package.json turbo.json biome.json Taskfile.yml .gitignore .npmrc LICENSE bun.lock
git commit -m "build: scaffold root bun/turbo/biome workspace with taskfile"
```

---

## Task 2: `packages/shared` — product-metadata contract (TDD)

**Files:**
- Create: `packages/shared/package.json`, `packages/shared/tsconfig.json`, `packages/shared/vitest.config.ts`
- Create: `packages/shared/src/index.ts`
- Test: `packages/shared/src/index.test.ts`

**Interfaces:**
- Consumes: nothing.
- Produces (imported as `@option-tab/shared`):
  - `PRODUCT: { name: string; displayName: string; repo: string }`
  - `type Platform = "darwin" | "windows" | "linux"`
  - `type Arch = "amd64" | "arm64"`
  - `releaseAssetName(platform: Platform, arch: Arch, version: string): string`
  - `downloadUrl(platform: Platform, arch: Arch, version: string): string`
  - `latestReleaseUrl(): string`

- [ ] **Step 1: Create the package manifest and configs**

`packages/shared/package.json`:
```json
{
  "name": "@option-tab/shared",
  "version": "0.0.0",
  "private": true,
  "type": "module",
  "main": "./src/index.ts",
  "types": "./src/index.ts",
  "exports": { ".": "./src/index.ts" },
  "scripts": {
    "lint": "biome check .",
    "test": "vitest run",
    "build": "tsc --noEmit"
  },
  "devDependencies": { "vitest": "^3.0.0" }
}
```

`packages/shared/tsconfig.json`:
```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "strict": true,
    "declaration": true,
    "skipLibCheck": true,
    "noEmit": true
  },
  "include": ["src"]
}
```

`packages/shared/vitest.config.ts`:
```ts
import { defineConfig } from "vitest/config";

export default defineConfig({
  test: { environment: "node", include: ["src/**/*.test.ts"] },
});
```

- [ ] **Step 2: Write the failing test**

`packages/shared/src/index.test.ts`:
```ts
import { describe, expect, it } from "vitest";
import { PRODUCT, downloadUrl, latestReleaseUrl, releaseAssetName } from "./index";

describe("releaseAssetName", () => {
  it.each([
    ["darwin", "arm64", "1.2.3", "option-tab_1.2.3_darwin_arm64.dmg"],
    ["windows", "amd64", "1.2.3", "option-tab_1.2.3_windows_amd64.zip"],
    ["linux", "amd64", "1.2.3", "option-tab_1.2.3_linux_amd64.tar.gz"],
  ] as const)("%s/%s -> %s", (platform, arch, version, expected) => {
    expect(releaseAssetName(platform, arch, version)).toBe(expected);
  });
});

describe("downloadUrl", () => {
  it("builds a tagged release asset URL", () => {
    expect(downloadUrl("darwin", "arm64", "1.2.3")).toBe(
      `${PRODUCT.repo}/releases/download/v1.2.3/option-tab_1.2.3_darwin_arm64.dmg`,
    );
  });
});

describe("latestReleaseUrl", () => {
  it("points at the latest release page", () => {
    expect(latestReleaseUrl()).toBe(`${PRODUCT.repo}/releases/latest`);
  });
});
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `cd packages/shared && bunx vitest run`
Expected: FAIL — cannot resolve `./index`.

- [ ] **Step 4: Write the minimal implementation**

`packages/shared/src/index.ts`:
```ts
export const PRODUCT = {
  name: "option-tab",
  displayName: "Option Tab",
  repo: "https://github.com/gui336699/option-tab",
} as const;

export type Platform = "darwin" | "windows" | "linux";
export type Arch = "amd64" | "arm64";

const EXTENSIONS: Record<Platform, string> = {
  darwin: "dmg",
  windows: "zip",
  linux: "tar.gz",
};

export function releaseAssetName(platform: Platform, arch: Arch, version: string): string {
  return `${PRODUCT.name}_${version}_${platform}_${arch}.${EXTENSIONS[platform]}`;
}

export function downloadUrl(platform: Platform, arch: Arch, version: string): string {
  return `${PRODUCT.repo}/releases/download/v${version}/${releaseAssetName(platform, arch, version)}`;
}

export function latestReleaseUrl(): string {
  return `${PRODUCT.repo}/releases/latest`;
}
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `cd packages/shared && bunx vitest run`
Expected: PASS (5 assertions).

- [ ] **Step 6: Commit**

```bash
git add packages/shared
git commit -m "feat(shared): add product metadata and release-asset URL resolver"
```

---

## Task 3: Desktop Go backend — hexagonal-lite (TDD)

**Files:**
- Create: `apps/desktop/go.mod`, `go.work`, `.golangci.yml`
- Create: `apps/desktop/internal/greeter/greeter.go`
- Test: `apps/desktop/internal/greeter/greeter_test.go`
- Create: `apps/desktop/app.go`, `apps/desktop/main.go`, `apps/desktop/wails.json`
- Test: `apps/desktop/app_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `greeter.Greeter` interface: `Greet(name string) string`
  - `greeter.New() *greeter.DefaultGreeter`
  - `main.App` struct with `NewApp() *App`, `(*App).startup(ctx context.Context)`, `(*App).Greet(name string) string`

- [ ] **Step 1: Initialize the Go module and workspace**

`apps/desktop/go.mod`:
```
module option-tab

go 1.23
```

`go.work` (repo root):
```
go 1.23

use ./apps/desktop
```

`.golangci.yml` (repo root) — golangci-lint **v2** schema (installed version is 2.12.x; in v2 formatters live in their own block and the file must declare `version: "2"`):
```yaml
version: "2"
run:
  timeout: 5m
linters:
  default: standard
  enable:
    - errcheck
    - ineffassign
    - misspell
    - staticcheck
    - unused
formatters:
  enable:
    - gofmt
    - gofumpt
  settings:
    gofumpt:
      extra-rules: true
```

Note: `govet` and `staticcheck` are part of golangci-lint v2's `standard` default set; `gofumpt` runs as a formatter (checked by `golangci-lint run`, auto-applied by `golangci-lint fmt`).

- [ ] **Step 2: Write the failing greeter test (table-driven)**

`apps/desktop/internal/greeter/greeter_test.go`:
```go
package greeter

import "testing"

func TestDefaultGreeter_Greet(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty defaults to World", "", "Hello, World!"},
		{"uses provided name", "Gui", "Hello, Gui!"},
		{"trims whitespace", "  Ana  ", "Hello, Ana!"},
	}
	g := New()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := g.Greet(tt.input); got != tt.want {
				t.Errorf("Greet(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `cd apps/desktop && go test ./internal/greeter/`
Expected: FAIL — undefined: `New`.

- [ ] **Step 4: Implement the greeter unit**

`apps/desktop/internal/greeter/greeter.go`:
```go
// Package greeter holds an example business unit. Logic lives behind the
// Greeter interface so the Wails binding layer depends on behavior, not a
// concrete type, and can be tested with a mock.
package greeter

import (
	"fmt"
	"strings"
)

// Greeter produces a greeting for a name.
type Greeter interface {
	Greet(name string) string
}

// DefaultGreeter is the production implementation.
type DefaultGreeter struct{}

// New returns a production Greeter.
func New() *DefaultGreeter { return &DefaultGreeter{} }

// Greet returns a friendly greeting, defaulting to "World" when empty.
func (g *DefaultGreeter) Greet(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "World"
	}
	return fmt.Sprintf("Hello, %s!", name)
}
```

- [ ] **Step 5: Run the greeter test to verify it passes**

Run: `cd apps/desktop && go test ./internal/greeter/`
Expected: PASS (3 sub-tests).

- [ ] **Step 6: Write the failing App adapter test (with a mock)**

`apps/desktop/app_test.go`:
```go
package main

import "testing"

type mockGreeter struct{ called string }

func (m *mockGreeter) Greet(name string) string {
	m.called = name
	return "mocked:" + name
}

func TestApp_Greet_DelegatesToGreeter(t *testing.T) {
	mg := &mockGreeter{}
	app := &App{greeter: mg}

	got := app.Greet("Gui")

	if got != "mocked:Gui" {
		t.Errorf("Greet() = %q, want %q", got, "mocked:Gui")
	}
	if mg.called != "Gui" {
		t.Errorf("greeter received %q, want %q", mg.called, "Gui")
	}
}
```

- [ ] **Step 7: Run the App test to verify it fails**

Run: `cd apps/desktop && go test .`
Expected: FAIL — undefined: `App`.

- [ ] **Step 8: Implement the thin Wails-bound adapter**

`apps/desktop/app.go`:
```go
package main

import (
	"context"

	"option-tab/internal/greeter"
)

// App is the Wails-bound layer: a thin adapter that holds service
// interfaces and delegates to them. It contains no business logic.
type App struct {
	ctx     context.Context
	greeter greeter.Greeter
}

// NewApp wires production dependencies.
func NewApp() *App {
	return &App{greeter: greeter.New()}
}

// startup captures the Wails runtime context.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Greet is exposed to the frontend through Wails bindings.
func (a *App) Greet(name string) string {
	return a.greeter.Greet(name)
}
```

- [ ] **Step 9: Run the App test to verify it passes**

Run: `cd apps/desktop && go test .`
Expected: PASS.

- [ ] **Step 10: Add the Wails bootstrap and config (not unit-tested; covered by build smoke)**

`apps/desktop/main.go`:
```go
package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "Option Tab",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{Assets: assets},
		OnStartup:   app.startup,
		Bind:        []interface{}{app},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
```

`apps/desktop/wails.json`:
```json
{
  "$schema": "https://wails.io/schemas/config.v2.json",
  "name": "option-tab",
  "outputfilename": "option-tab",
  "frontend:install": "bun install",
  "frontend:build": "bun run build",
  "frontend:dev:watcher": "bun run dev",
  "frontend:dev:serverUrl": "auto",
  "wailsjsdir": "./frontend/src"
}
```

- [ ] **Step 11: Tidy modules and pull Wails dependency**

Run: `cd apps/desktop && go get github.com/wailsapp/wails/v2@latest && go mod tidy`
Expected: `go.mod` + `go.sum` updated with Wails v2.

Note: `go vet ./...` and `go build ./...` will fail until `frontend/dist` exists (Task 4 creates it). That is expected; the embed is exercised by the build smoke in CI after Task 4.

- [ ] **Step 12: Run the full Go test suite**

Run: `cd apps/desktop && go test ./internal/... . -race`
Expected: PASS (App + greeter). The `main` package compiles for tests because tests don't trigger the embed build constraint at vet-time for `go test` of non-embed symbols — if the embed blocks `go test .`, temporarily create `apps/desktop/frontend/dist/.gitkeep` so the embed resolves.

- [ ] **Step 13: Commit**

```bash
git add apps/desktop go.work .golangci.yml
git commit -m "feat(desktop): add wails bootstrap, thin app adapter, and greeter unit"
```

---

## Task 4: Desktop frontend — Vite + React + TS (TDD)

**Files:**
- Create: `apps/desktop/frontend/package.json`, `tsconfig.json`, `vite.config.ts`, `vitest.config.ts`, `index.html`
- Create: `apps/desktop/frontend/src/main.tsx`, `src/App.tsx`
- Create: `apps/desktop/frontend/.gitignore`
- Create: `apps/desktop/frontend/src/lib/name.ts`
- Test: `apps/desktop/frontend/src/lib/name.test.ts`
- Create: `apps/desktop/frontend/src/lib/desktop.ts` (typed Wails runtime-binding wrapper)
- Test: `apps/desktop/frontend/src/lib/desktop.test.ts`
- Create: `apps/desktop/frontend/src/components/Greeting.tsx`
- Test: `apps/desktop/frontend/src/components/Greeting.test.tsx`

**Interfaces:**
- Consumes: the Wails runtime binding `window.go.main.App.Greet` (exposed by Wails at runtime; accessed through a typed wrapper, NOT a static import of generated files, so the frontend builds standalone in CI without `wails dev`/`wails build`).
- Produces: `sanitizeName(raw: string): string`; `greet(name: string): Promise<string>` (typed runtime-binding wrapper); `<Greeting message={string} />` React component.

- [ ] **Step 1: Create the frontend manifest and tooling configs**

`apps/desktop/frontend/package.json`:
```json
{
  "name": "@option-tab/desktop-frontend",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "lint": "biome check .",
    "test": "vitest run"
  },
  "dependencies": { "react": "^19.0.0", "react-dom": "^19.0.0" },
  "devDependencies": {
    "@testing-library/react": "^16.0.0",
    "@testing-library/jest-dom": "^6.4.0",
    "@types/react": "^19.0.0",
    "@types/react-dom": "^19.0.0",
    "@vitejs/plugin-react": "^4.3.0",
    "jsdom": "^25.0.0",
    "typescript": "^5.6.0",
    "vite": "^6.0.0",
    "vitest": "^3.0.0"
  }
}
```

`apps/desktop/frontend/vite.config.ts`:
```ts
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react()],
  build: { outDir: "dist" },
});
```

`apps/desktop/frontend/vitest.config.ts`:
```ts
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./vitest.setup.ts"],
    include: ["src/**/*.test.{ts,tsx}"],
  },
});
```

`apps/desktop/frontend/vitest.setup.ts`:
```ts
import "@testing-library/jest-dom/vitest";
```

`apps/desktop/frontend/tsconfig.json`:
```json
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "jsx": "react-jsx",
    "strict": true,
    "skipLibCheck": true,
    "noEmit": true,
    "types": ["vitest/globals", "@testing-library/jest-dom"]
  },
  "include": ["src"]
}
```

`apps/desktop/frontend/index.html`:
```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Option Tab</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 2: Write the failing pure-logic test**

`apps/desktop/frontend/src/lib/name.test.ts`:
```ts
import { describe, expect, it } from "vitest";
import { sanitizeName } from "./name";

describe("sanitizeName", () => {
  it("trims surrounding whitespace", () => {
    expect(sanitizeName("  Gui  ")).toBe("Gui");
  });
  it("collapses inner whitespace runs", () => {
    expect(sanitizeName("Ana   Maria")).toBe("Ana Maria");
  });
});
```

- [ ] **Step 3: Run it to verify it fails**

Run: `cd apps/desktop/frontend && bunx vitest run src/lib/name.test.ts`
Expected: FAIL — cannot resolve `./name`.

- [ ] **Step 4: Implement the pure logic**

`apps/desktop/frontend/src/lib/name.ts`:
```ts
export function sanitizeName(raw: string): string {
  return raw.trim().replace(/\s+/g, " ");
}
```

- [ ] **Step 5: Run it to verify it passes**

Run: `cd apps/desktop/frontend && bunx vitest run src/lib/name.test.ts`
Expected: PASS.

- [ ] **Step 6a: Write the failing desktop-binding test**

`apps/desktop/frontend/src/lib/desktop.test.ts`:
```ts
import { afterEach, describe, expect, it, vi } from "vitest";
import { greet } from "./desktop";

afterEach(() => {
  // biome-ignore lint/performance/noDelete: test cleanup of the injected global
  delete (globalThis as { go?: unknown }).go;
});

describe("greet", () => {
  it("delegates to the Wails App.Greet binding", async () => {
    const Greet = vi.fn().mockResolvedValue("Hello, Gui!");
    (globalThis as { go?: unknown }).go = { main: { App: { Greet } } };

    await expect(greet("Gui")).resolves.toBe("Hello, Gui!");
    expect(Greet).toHaveBeenCalledWith("Gui");
  });

  it("throws a helpful error when the Wails runtime is absent", async () => {
    await expect(greet("Gui")).rejects.toThrow(/wails/i);
  });
});
```

- [ ] **Step 6b: Run it to verify it fails**

Run: `cd apps/desktop/frontend && bunx vitest run src/lib/desktop.test.ts`
Expected: FAIL — cannot resolve `./desktop`.

- [ ] **Step 6c: Implement the typed runtime-binding wrapper**

`apps/desktop/frontend/src/lib/desktop.ts`:
```ts
// Typed accessor for the Wails-exposed Go bindings. Wails injects the bound
// App methods on the global `go.main.App` object at runtime. We read them
// here (instead of importing the generated `wailsjs/` files) so the frontend
// builds standalone in CI and the binding is trivially mockable in tests.
interface WailsApp {
  Greet(name: string): Promise<string>;
}

function wailsApp(): WailsApp {
  const app = (globalThis as { go?: { main?: { App?: WailsApp } } }).go?.main?.App;
  if (!app) {
    throw new Error("Wails runtime bindings unavailable (run the app via `wails dev`).");
  }
  return app;
}

export function greet(name: string): Promise<string> {
  return wailsApp().Greet(name);
}
```

- [ ] **Step 6d: Run it to verify it passes**

Run: `cd apps/desktop/frontend && bunx vitest run src/lib/desktop.test.ts`
Expected: PASS (2 assertions).

- [ ] **Step 7: Write the failing component test**

`apps/desktop/frontend/src/components/Greeting.test.tsx`:
```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Greeting } from "./Greeting";

describe("Greeting", () => {
  it("renders the provided message", () => {
    render(<Greeting message="Hello, Gui!" />);
    expect(screen.getByRole("status")).toHaveTextContent("Hello, Gui!");
  });
});
```

- [ ] **Step 7: Run it to verify it fails**

Run: `cd apps/desktop/frontend && bunx vitest run src/components/Greeting.test.tsx`
Expected: FAIL — cannot resolve `./Greeting`.

- [ ] **Step 8: Implement the component and app shell**

`apps/desktop/frontend/src/components/Greeting.tsx`:
```tsx
export function Greeting({ message }: { message: string }) {
  return <p role="status">{message}</p>;
}
```

`apps/desktop/frontend/src/App.tsx`:
```tsx
import { useState } from "react";
import { Greeting } from "./components/Greeting";
import { greet } from "./lib/desktop";
import { sanitizeName } from "./lib/name";

export default function App() {
  const [name, setName] = useState("");
  const [message, setMessage] = useState("");

  async function onGreet() {
    setMessage(await greet(sanitizeName(name)));
  }

  return (
    <main>
      <h1>Option Tab</h1>
      <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Your name" />
      <button type="button" onClick={onGreet}>Greet</button>
      <Greeting message={message} />
    </main>
  );
}
```

`apps/desktop/frontend/src/main.tsx`:
```tsx
import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
```

Note: App.tsx reaches the Go layer through `greet()` (the typed runtime wrapper from Step 6c), so it does NOT statically import any generated `wailsjs/` files. The frontend therefore builds standalone in CI. When Wails generates its runtime into `frontend/src/wailsjs/` during `wails dev`, that directory is git-ignored and lint-excluded — add `**/wailsjs/**` to the frontend `.gitignore` is unnecessary because Step 1's root `.gitignore` does not cover it; instead add a local ignore in this step (below).

Also create `apps/desktop/frontend/.gitignore`:
```
dist/
wailsjs/
src/wailsjs/
```

- [ ] **Step 9: Run the full frontend test suite**

Run: `cd apps/desktop/frontend && bunx vitest run`
Expected: PASS (name + desktop + Greeting = 5 assertions across 3 files). No generated bindings required.

- [ ] **Step 10: Produce a standalone build and confirm the Go embed resolves**

Run: `cd apps/desktop/frontend && bun run build`
Expected: `apps/desktop/frontend/dist/` created (standalone Vite build, no Wails CLI needed).

Run: `cd apps/desktop && go build ./...`
Expected: builds (the embed resolves against the freshly built `frontend/dist`).

- [ ] **Step 11: Commit**

```bash
git add apps/desktop/frontend
git commit -m "feat(desktop): add react frontend with sanitizeName logic and Greeting component"
```

---

## Task 5: Landing page — Next.js static export (TDD)

**Files:**
- Create: `apps/web/package.json`, `next.config.ts`, `tsconfig.json`, `vitest.config.ts`, `playwright.config.ts`
- Create: `apps/web/app/layout.tsx`, `apps/web/app/page.tsx`
- Create: `apps/web/components/DownloadButtons.tsx` (static, all-platform list)
- Create: `apps/web/components/PrimaryDownload.tsx` (client component — detects the visitor's OS)
- Create: `apps/web/lib/download.ts`
- Test: `apps/web/lib/download.test.ts`
- Test: `apps/web/e2e/landing.spec.ts`

**Interfaces:**
- Consumes: `@option-tab/shared` (`downloadUrl`, `latestReleaseUrl`, `Platform`).
- Produces: `detectPlatform(userAgent: string): Platform` and `APP_VERSION` (from `lib/download.ts`); a landing page rendering a heading, a platform-detected primary download button, and the full per-OS download list. `detectPlatform` is load-bearing: the primary button uses it (verified via the E2E userAgent-emulation test), so it is not dead code.

- [ ] **Step 1: Create the Next.js manifest and configs**

`apps/web/package.json`:
```json
{
  "name": "@option-tab/web",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "next dev",
    "build": "next build",
    "start": "next start",
    "lint": "biome check .",
    "test": "vitest run",
    "e2e": "playwright test"
  },
  "dependencies": {
    "@option-tab/shared": "workspace:*",
    "next": "^15.0.0",
    "react": "^19.0.0",
    "react-dom": "^19.0.0"
  },
  "devDependencies": {
    "@playwright/test": "^1.48.0",
    "@types/node": "^20.0.0",
    "@types/react": "^19.0.0",
    "@types/react-dom": "^19.0.0",
    "typescript": "^5.6.0",
    "vitest": "^3.0.0"
  }
}
```

`apps/web/next.config.ts`:
```ts
import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "export",
  images: { unoptimized: true },
};

export default nextConfig;
```

`apps/web/tsconfig.json`:
```json
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "jsx": "preserve",
    "strict": true,
    "skipLibCheck": true,
    "noEmit": true,
    "plugins": [{ "name": "next" }]
  },
  "include": ["**/*.ts", "**/*.tsx", ".next/types/**/*.ts"],
  "exclude": ["node_modules", "e2e"]
}
```

`apps/web/vitest.config.ts`:
```ts
import { defineConfig } from "vitest/config";

export default defineConfig({
  test: { environment: "node", include: ["lib/**/*.test.ts"] },
});
```

`apps/web/playwright.config.ts`:
```ts
import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  webServer: {
    command: "bunx serve out -l 3000",
    url: "http://localhost:3000",
    reuseExistingServer: !process.env.CI,
  },
  use: { baseURL: "http://localhost:3000" },
});
```

- [ ] **Step 2: Write the failing download-logic test**

`apps/web/lib/download.test.ts`:
```ts
import { describe, expect, it } from "vitest";
import { detectPlatform } from "./download";

describe("detectPlatform", () => {
  it.each([
    ["Mozilla/5.0 (Macintosh; Intel Mac OS X)", "darwin"],
    ["Mozilla/5.0 (Windows NT 10.0)", "windows"],
    ["Mozilla/5.0 (X11; Linux x86_64)", "linux"],
  ] as const)("%s -> %s", (ua, expected) => {
    expect(detectPlatform(ua)).toBe(expected);
  });
});
```

- [ ] **Step 3: Run it to verify it fails**

Run: `cd apps/web && bunx vitest run`
Expected: FAIL — cannot resolve `./download`.

- [ ] **Step 4: Implement the download logic over the shared contract**

`apps/web/lib/download.ts`:
```ts
import { type Platform, downloadUrl, latestReleaseUrl } from "@option-tab/shared";

// Single source of truth for the version the landing page advertises.
// Bump this in lockstep with a desktop release tag.
export const APP_VERSION = "0.1.0";

export function detectPlatform(userAgent: string): Platform {
  const ua = userAgent.toLowerCase();
  if (ua.includes("mac")) return "darwin";
  if (ua.includes("win")) return "windows";
  return "linux";
}

export { downloadUrl, latestReleaseUrl };
export type { Platform };
```

- [ ] **Step 5: Run it to verify it passes**

Run: `cd apps/web && bunx vitest run`
Expected: PASS.

- [ ] **Step 6: Build the landing page UI**

`apps/web/app/layout.tsx`:
```tsx
import type { ReactNode } from "react";

export const metadata = { title: "Option Tab", description: "The option-tab desktop app." };

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
```

`apps/web/components/DownloadButtons.tsx`:
```tsx
import { downloadUrl } from "@option-tab/shared";
import { APP_VERSION } from "../lib/download";

const TARGETS = [
  { label: "Download for macOS", platform: "darwin", arch: "arm64" },
  { label: "Download for Windows", platform: "windows", arch: "amd64" },
  { label: "Download for Linux", platform: "linux", arch: "amd64" },
] as const;

export function DownloadButtons() {
  return (
    <nav aria-label="Downloads">
      <ul>
        {TARGETS.map((t) => (
          <li key={t.platform}>
            <a data-testid={`download-${t.platform}`} href={downloadUrl(t.platform, t.arch, APP_VERSION)}>
              {t.label}
            </a>
          </li>
        ))}
      </ul>
    </nav>
  );
}
```

`apps/web/components/PrimaryDownload.tsx` (client component — detects the OS after hydration and renders the recommended download):
```tsx
"use client";

import { useEffect, useState } from "react";
import { type Platform, downloadUrl } from "@option-tab/shared";
import { APP_VERSION, detectPlatform } from "../lib/download";

const ARCH: Record<Platform, "amd64" | "arm64"> = {
  darwin: "arm64",
  windows: "amd64",
  linux: "amd64",
};
const OS_LABEL: Record<Platform, string> = {
  darwin: "macOS",
  windows: "Windows",
  linux: "Linux",
};

export function PrimaryDownload() {
  const [platform, setPlatform] = useState<Platform | null>(null);

  useEffect(() => {
    setPlatform(detectPlatform(navigator.userAgent));
  }, []);

  if (!platform) {
    return null;
  }

  return (
    <a
      data-testid="primary-download"
      data-platform={platform}
      href={downloadUrl(platform, ARCH[platform], APP_VERSION)}
    >
      Download for {OS_LABEL[platform]}
    </a>
  );
}
```

`apps/web/app/page.tsx`:
```tsx
import { DownloadButtons } from "../components/DownloadButtons";
import { PrimaryDownload } from "../components/PrimaryDownload";

export default function Home() {
  return (
    <main>
      <h1>Option Tab</h1>
      <PrimaryDownload />
      <p>A fast, open-source desktop app. Free forever.</p>
      <DownloadButtons />
    </main>
  );
}
```

- [ ] **Step 7: Write the Playwright smoke test**

`apps/web/e2e/landing.spec.ts`:
```ts
import { expect, test } from "@playwright/test";

test("landing page renders and exposes per-OS download links", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Option Tab" })).toBeVisible();

  const macLink = page.getByTestId("download-darwin");
  await expect(macLink).toHaveAttribute("href", /\/releases\/download\/v0\.1\.0\/option-tab_0\.1\.0_darwin_arm64\.dmg$/);
});

test.describe("primary download (platform detection)", () => {
  test.use({ userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)" });

  test("recommends the macOS build for a mac user agent", async ({ page }) => {
    await page.goto("/");
    const primary = page.getByTestId("primary-download");
    await expect(primary).toHaveAttribute("data-platform", "darwin");
    await expect(primary).toHaveAttribute("href", /option-tab_0\.1\.0_darwin_arm64\.dmg$/);
  });
});
```

Note: `PrimaryDownload` renders `null` until hydration sets the detected platform, so the E2E relies on JS being enabled (Playwright's default). Playwright's `getByTestId` auto-waits for the element to appear post-hydration.

- [ ] **Step 8: Build, then run the e2e smoke**

Run: `cd apps/web && bun run build`
Expected: static export to `apps/web/out/`.

Run: `cd apps/web && bunx playwright install --with-deps chromium && bunx playwright test`
Expected: PASS (1 test).

- [ ] **Step 9: Commit**

```bash
git add apps/web
git commit -m "feat(web): add static landing page with shared-contract download buttons and e2e smoke"
```

---

## Task 6: Lefthook git hooks

**Files:**
- Create: `lefthook.yml`
- Modify: root `package.json` (add `lefthook` devDependency + `prepare` script)

**Interfaces:**
- Consumes: `task test`, `biome`, `gofumpt`, `golangci-lint` (must be installed).
- Produces: pre-commit, pre-push, commit-msg hooks.

- [ ] **Step 1: Add Lefthook to the root manifest**

Modify root `package.json` — add to `devDependencies`: `"lefthook": "^1.7.0"`, and add to `scripts`: `"prepare": "lefthook install"`.

- [ ] **Step 2: Create the hooks config**

`lefthook.yml`:
```yaml
pre-commit:
  parallel: true
  commands:
    biome:
      glob: "*.{js,jsx,ts,tsx,json,jsonc}"
      run: bunx biome check --write --no-errors-on-unmatched {staged_files}
      stage_fixed: true
    gofumpt:
      glob: "*.go"
      run: gofumpt -w {staged_files}
      stage_fixed: true
    golangci:
      glob: "*.go"
      run: cd apps/desktop && golangci-lint run ./...

pre-push:
  commands:
    test:
      run: task test

commit-msg:
  commands:
    conventional:
      run: |
        head -n1 {1} | grep -qE '^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\(.+\))?!?: .+' \
          || { echo 'Commit message must follow Conventional Commits (e.g. "feat: add X")'; exit 1; }
```

- [ ] **Step 3: Install and verify hooks register**

Run: `bun install && bunx lefthook install`
Expected: prints `lefthook installed` and creates `.git/hooks/pre-commit` etc.

Run: `bunx lefthook run commit-msg --commit-msg-file <(echo "chore: verify hook")`
Expected: passes. Then test a bad message:
Run: `bunx lefthook run commit-msg --commit-msg-file <(echo "bad message")`
Expected: fails with the Conventional Commits error.

- [ ] **Step 4: Commit**

```bash
git add lefthook.yml package.json bun.lock
git commit -m "build: add lefthook pre-commit, pre-push, and commit-msg hooks"
```

---

## Task 7: CI/CD GitHub Actions workflows

**Files:**
- Create: `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `.github/workflows/deploy-web.yml`

**Interfaces:**
- Consumes: `task`/`bun`/`go`/`wails` toolchain; the build outputs from prior tasks.
- Produces: PR checks; tagged desktop release; Pages deploy.

- [ ] **Step 1: Create the CI workflow**

`.github/workflows/ci.yml`:
```yaml
name: CI
on:
  pull_request:
  push:
    branches: [main]

jobs:
  js:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: oven-sh/setup-bun@v2
        with: { bun-version: latest }
      - run: bun install --frozen-lockfile
      - run: bun run lint
      - run: bun run test
      - run: bun run build
      - run: cd apps/web && bunx playwright install --with-deps chromium && bun run e2e

  go:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.23" }
      - name: Install desktop system deps
        run: sudo apt-get update && sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev
      - run: cd apps/desktop && go test ./... -race -cover
      - uses: golangci/golangci-lint-action@v7
        with:
          version: v2.12
          working-directory: apps/desktop
```

- [ ] **Step 2: Create the desktop release workflow**

`.github/workflows/release.yml`:
```yaml
name: Release
on:
  push:
    tags: ["v*"]

permissions:
  contents: write

jobs:
  build-desktop:
    strategy:
      fail-fast: false
      matrix:
        include:
          - os: macos-latest
            platform: darwin
          - os: windows-latest
            platform: windows
          - os: ubuntu-latest
            platform: linux
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: oven-sh/setup-bun@v2
        with: { bun-version: latest }
      - uses: actions/setup-go@v5
        with: { go-version: "1.23" }
      - name: Install Linux system deps
        if: matrix.platform == 'linux'
        run: sudo apt-get update && sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev
      - name: Install Wails CLI
        run: go install github.com/wailsapp/wails/v2/cmd/wails@latest
      - run: bun install --frozen-lockfile
      - name: Build desktop binary
        run: cd apps/desktop && wails build -platform ${{ matrix.platform }}/amd64
      - name: Package artifact
        shell: bash
        run: |
          cd apps/desktop/build/bin
          ls -la
          tar -czf "option-tab_${GITHUB_REF_NAME#v}_${{ matrix.platform }}_amd64.tar.gz" --exclude='*.tar.gz' *
      - uses: softprops/action-gh-release@v2
        with:
          files: apps/desktop/build/bin/*.tar.gz
```

Note: macOS/Windows packaging extensions (`.dmg`/`.zip`) and arm64 builds are documented as follow-ups in `docs/release.md`; this scaffold ships the amd64 `tar.gz` matrix as the working baseline that matches `releaseAssetName` for Linux. Aligning mac/windows asset extensions with `@option-tab/shared` is the first post-scaffold task.

- [ ] **Step 3: Create the Pages deploy workflow**

`.github/workflows/deploy-web.yml`:
```yaml
name: Deploy Web
on:
  push:
    branches: [main]
    paths: ["apps/web/**", "packages/shared/**"]
  workflow_dispatch:

permissions:
  contents: read
  pages: write
  id-token: write

jobs:
  deploy:
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: oven-sh/setup-bun@v2
        with: { bun-version: latest }
      - run: bun install --frozen-lockfile
      - run: cd apps/web && bun run build
      - uses: actions/configure-pages@v5
      - uses: actions/upload-pages-artifact@v3
        with: { path: apps/web/out }
      - id: deployment
        uses: actions/deploy-pages@v4
```

- [ ] **Step 4: Validate workflow YAML**

Run: `bunx --bun yaml-lint .github/workflows/*.yml 2>/dev/null || python3 -c "import yaml,glob;[yaml.safe_load(open(f)) for f in glob.glob('.github/workflows/*.yml')];print('yaml ok')"`
Expected: `yaml ok` (or yaml-lint success).

- [ ] **Step 5: Commit**

```bash
git add .github
git commit -m "ci: add PR checks, cross-platform desktop release, and pages deploy workflows"
```

---

## Task 8: Documentation

**Files:**
- Create: `docs/architecture.md`, `docs/development.md`, `docs/testing.md`, `docs/release.md`, `CONTRIBUTING.md`
- Modify: `README.md`

**Interfaces:**
- Consumes: everything built above.
- Produces: contributor-facing docs. No tests; verified by link/command accuracy.

- [ ] **Step 1: Write the README**

Replace `README.md` with: project one-liner; badges placeholder; "What is option-tab" (desktop app is the product, web is the landing page); Prerequisites (Go 1.23+, Bun 1.1+, Wails CLI v2, Task, golangci-lint+gofumpt); Quickstart (`bun install`, `task dev:desktop`, `task dev:web`); the monorepo map table from this plan's File Structure; "Testing" pointer to `docs/testing.md`; License: Apache-2.0.

- [ ] **Step 2: Write `docs/architecture.md`**

Cover: the apps/packages layout; the desktop hexagonal-lite pattern (`internal/` behind interfaces, `app.go` thin adapter); why web is a separate static site; the `@option-tab/shared` contract as the seam between `release.yml` asset names and landing-page download links; Taskfile + Turbo orchestration split.

- [ ] **Step 3: Write `docs/development.md`**

Cover: installing each tool (links to Wails, Bun, Task, golangci-lint, gofumpt install commands); the `task` verb list; how Wails bindings get generated (`wails dev`); how the Go embed depends on `frontend/dist`.

- [ ] **Step 4: Write `docs/testing.md`**

Cover the testability pattern per layer with the real examples from this repo: Go table-driven + interface mock (`greeter` + `app_test.go`), frontend pure-`lib` + Testing Library (`sanitizeName` + `Greeting`), shared-contract unit tests, Playwright smoke. State the rule: business logic goes behind an interface/pure function so the framework-bound layer stays thin and test-light.

- [ ] **Step 5: Write `docs/release.md`**

Cover: tag `vX.Y.Z` → `release.yml` matrix builds desktop binaries → GitHub Release; how asset names must match `@option-tab/shared`; documented follow-ups (mac `.dmg`/notarization+signing, windows `.zip`, arm64 matrix, auto-update); landing-page deploy via `deploy-web.yml` to GitHub Pages and how download buttons resolve.

- [ ] **Step 6: Write `CONTRIBUTING.md`**

Cover: fork/branch flow; Conventional Commits requirement (enforced by commit-msg hook); that Lefthook auto-formats on commit and runs tests on push; how to run `task lint`/`task test` before opening a PR; Apache-2.0 + DCO/sign-off note.

- [ ] **Step 7: Verify documented commands exist**

Run: `grep -RhoE 'task [a-z:]+' README.md docs/*.md | sort -u`
Expected: every listed task verb exists in `Taskfile.yml` (cross-check against `task --list`).

- [ ] **Step 8: Commit**

```bash
git add README.md docs CONTRIBUTING.md
git commit -m "docs: add architecture, development, testing, release, and contributing guides"
```

---

## Final Verification

- [ ] **From a clean state, run the full toolchain:**

```bash
bun install
task lint
task test
task build
task e2e
```
Expected: all succeed (desktop `wails build` requires the Wails CLI + system deps per `docs/development.md`).

- [ ] **Confirm each workspace has a passing example test:** `packages/shared`, `apps/desktop` (Go), `apps/desktop/frontend`, `apps/web` (unit + Playwright).

- [ ] **Confirm hooks block bad commits:** an unformatted file is auto-fixed on commit; a non-conventional message is rejected.

---

## Self-Review Notes

- **Spec coverage:** topology (Task 1, 3–5), Turbo+Bun (Task 1), Go lint (Task 3), Biome (Task 1), Lefthook (Task 6), full CI/release/deploy (Task 7), testing pyramid (Tasks 2–5), Apache-2.0 (Task 1), docs (Task 8), `packages/shared` contract (Task 2, consumed in Task 5). All spec sections map to a task.
- **Known scaffold limitation (surfaced, not hidden):** `release.yml` ships an amd64 `tar.gz` matrix; mac/windows asset extensions and arm64 are documented follow-ups in `docs/release.md` (Task 7 Step 2 note, Task 8 Step 5). This is the one place the running pipeline intentionally lags the `@option-tab/shared` contract; called out explicitly rather than left as a silent gap.
- **Type consistency:** `Greet`, `greeter.Greeter`, `App`, `sanitizeName`, `detectPlatform`, `releaseAssetName`/`downloadUrl`/`latestReleaseUrl` names are used identically across producing and consuming tasks.

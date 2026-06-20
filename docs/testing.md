# Testing guide

## Philosophy

The core rule: **all decision logic lives in pure functions or behind interfaces; the
framework-bound and OS-bound layers stay thin and test-light.**

- Pure-Go packages (`domain`, `config`, `filter`, `order`, `search`, `mru`, `hotkey`) hold
  the logic and are exhaustively table-tested with zero Wails/CGO involvement.
- `switcher.Controller` — the state machine — is driven end-to-end against an in-memory
  **fake platform** (`internal/platform/fake`), so a full activate → cycle → search →
  confirm sequence is verified without touching the OS.
- The macOS CGO backend (`darwin.go`/`.m`) only translates OS calls and holds no decision
  logic, so it gets a small `//go:build darwin` smoke test (it constructs and enumerates
  windows without erroring).
- `app.go` is a thin Wails adapter; only its settings get/save logic is unit-tested.
- Frontend logic (`lib/keymap.ts`, `lib/layout.ts`, `lib/bridge.ts`) is pure TypeScript,
  tested without a browser; the `Overlay` and `Settings` components are tested with Testing
  Library (rendering, keyboard handling, search, controls).
- Playwright provides a smoke layer over the fully-built landing page.

---

## Running tests

```bash
task test     # All unit/integration tests: Vitest (JS/TS) + go test -race (Go)
task e2e      # Playwright smoke tests (requires task build first)
```

---

## Layer-by-layer examples

### 1. Pure-Go logic — table-driven

Packages like `internal/filter`, `internal/order`, `internal/search`, and `internal/hotkey`
use table-driven tests. Example (`order`): given windows with timestamps, assert the
most-recently-used ordering, then alphabetical and by-space. Run with:

```bash
cd apps/desktop && go test ./... -race -cover
```

### 2. The controller against the fake platform (`internal/switcher`)

`switcher_test.go` builds a `Controller` with a `fake.Fake` platform and a recording
`View`, then asserts behavior of the whole cycle:

```go
c, f, v := newController(t, threeWins(), nil)
c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyRelease})
// the fake records that the previous window was focused, no OS involved
if f.LastFocused != 2 { t.Fatalf("quick switch should focus window 2") }
```

This covers activation, advance/reverse with wrap, type-to-search, per-shortcut scopes,
window controls, and cancel-restores-focus — all without a real platform.

### 3. macOS backend — smoke test (`internal/platform`, `//go:build darwin`)

`darwin_test.go` only asserts the backend constructs and that read-only queries
(`Windows()`, permission checks, `ActiveApp()`) run without panicking — even when no
permissions are granted on a CI host.

### 4. Frontend pure logic (`frontend/src/lib`)

`keymap.ts` (key event → action) and `layout.ts` (grid columns + thumbnail auto-sizing)
are pure functions tested in `*.test.ts` with Vitest — no DOM needed. `bridge.ts` is tested
by injecting/removing the Wails globals and asserting it calls the bound methods (and
no-ops safely when absent).

### 5. Frontend components (`frontend/src/overlay`, `frontend/src/settings`)

`Overlay.test.tsx` renders the switcher and asserts: all three visual styles render, the
selected entry is marked, Tab/Shift+Tab/Escape/Enter route to handlers, typing updates the
search query, hover selects and click confirms, and the hover controls fire close/minimize.
`Settings.test.tsx` asserts the controlled form emits updated settings on each edit.

### 6. Shared contract (`packages/shared`)

`@option-tab/shared` exports `releaseAssetName`, `downloadUrl`, and `latestReleaseUrl` —
pure functions tested with Vitest. They are the contract between the release pipeline and
the landing page, so any asset-naming change must be reflected in the tests.

### 7. Landing page — Playwright smoke (`apps/web/e2e`)

Playwright runs against the built static site and verifies the page renders, the per-OS
download links resolve to valid GitHub Release URLs, the primary button adapts to the
User-Agent, and the feature showcase (the three styles) is present.

---

## Test configuration summary

| Workspace | Runner | Notes |
|-----------|--------|-------|
| `apps/desktop` (Go) | `go test -race -cover` | pure packages + controller-vs-fake + darwin smoke |
| `apps/desktop/frontend` | Vitest + Testing Library | `jsdom` environment |
| `apps/web` | Vitest | `node` environment, `lib/**/*.test.ts` |
| `apps/web` (e2e) | Playwright | serves `out/` |
| `packages/shared` | Vitest | pure functions |

---

## CI enforcement

The `go` job in `ci.yml` runs `go test ./... -race -cover`; the `js` job runs all Vitest
workspaces and Playwright. Both must pass before merging to `main`.

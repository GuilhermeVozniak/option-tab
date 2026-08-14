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
by mocking the generated bindings module (`bindings/option-tab/app.js`) and the
`@wailsio/runtime` event bus, asserting it calls the bound methods (and degrades safely
when the backend is absent).

### 5. Frontend components (`frontend/src/overlay`, `frontend/src/settings`)

`Overlay.test.tsx` renders the switcher and asserts: all three visual styles render, the
selected entry is marked, keys route to handlers — driven through the production native
path (mocked `switcher:key` events) plus one DOM-fallback test for browser dev — typing
updates the
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

### 8. Desktop UI — Playwright e2e (`apps/desktop/frontend/e2e`)

The switcher overlay and the preferences window are driven end-to-end in real Chromium
against the built Vite bundle. The Go side is faked at the Wails v3 transport seam:
`e2e/support/fakeWails.ts` intercepts the bindings' `POST /wails/runtime` calls (routing
on the numeric method ids from the generated bindings) and dispatches Go→UI events through
`window._wails.dispatchWailsEvent`, so the real event-driven overlay is fully interactive
without the native backend (navigation methods re-emit `switcher:update`; window/app
actions are recorded for assertions; `page.keyboard` input is re-emitted as `switcher:key`
native-key payloads, mirroring the event-tap path). Coverage: the three visual styles (via the
built-in `#demo` route), keyboard navigation (Tab/Shift+Tab, arrows, vim), type-to-search,
Escape, click-to-confirm, the hover window/app controls (close/minimize/fullscreen/hide/
quit), the blur toggle, and the full `#settings` preferences surface (tab navigation,
editing controls, shortcuts, blacklist, language, import/export/reset, About). It does not
exercise the real Go backend (window enumeration, AX actions, the global hotkey) — that
native boundary stays smoke-only (§3).

---

## Test configuration summary

| Workspace | Runner | Notes |
|-----------|--------|-------|
| `apps/desktop` (Go) | `go test -race -cover` | pure packages + controller-vs-fake + darwin smoke |
| `apps/desktop/frontend` | Vitest + Testing Library | `jsdom` environment |
| `apps/web` | Vitest | `node` environment, `lib/**/*.test.ts` |
| `apps/web` (e2e) | Playwright | serves `out/` |
| `apps/desktop/frontend` (e2e) | Playwright | builds + serves the Vite bundle; drives overlay + preferences in Chromium with a fake Wails runtime |
| `packages/shared` | Vitest | pure functions |

---

## CI enforcement

The `go` job in `ci.yml` runs `go test ./... -race -cover`; the `js` job runs all Vitest
workspaces plus both Playwright suites (landing page and desktop UI). Both jobs must pass
before merging to `main`. The desktop UI e2e also gates releases: the `e2e` job in
`release.yml` runs before `build-desktop`, so a broken overlay or preferences flow blocks
the build and publish.

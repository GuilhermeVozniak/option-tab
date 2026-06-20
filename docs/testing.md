# Testing guide

## Philosophy

The core rule: **business logic lives behind an interface or pure function; the framework-bound layer stays thin and test-light.**

- Go business units live in `internal/<unit>/` behind interfaces — testable with zero Wails/CGo involvement.
- `app.go` is a thin adapter; it delegates to interfaces and needs only light smoke coverage.
- Frontend business logic (pure TypeScript helpers) lives in `src/lib/` — testable without a browser.
- UI components that only render props (like `Greeting`) are tested with Testing Library snapshots.
- `apps/web` keeps OS-detection and URL-construction logic in `lib/` (pure functions) and tests those separately from the page components.
- Playwright provides a thin smoke layer over the fully-built static site.

---

## Running tests

```bash
task test     # All unit/integration tests: Vitest (JS/TS) + go test (Go)
task e2e      # Playwright smoke tests (requires task build first)
```

---

## Layer-by-layer test examples

### 1. Go business unit — table-driven tests (`apps/desktop/internal/greeter`)

`internal/greeter/greeter_test.go` tests the production `DefaultGreeter` directly via the `Greeter` interface:

```go
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

The table-driven pattern makes it easy to add edge cases without adding boilerplate. Run with:

```bash
cd apps/desktop && go test ./... -race -cover
```

The `-race` flag catches data races; `-cover` reports coverage. CI enforces both.

### 2. Desktop frontend — pure lib function (`apps/desktop/frontend/src/lib/name.ts`)

`sanitizeName` is a pure function tested in `name.test.ts` with Vitest. No DOM, no Wails context needed.

### 3. Desktop frontend — Wails binding accessor (`apps/desktop/frontend/src/lib/desktop.ts`)

`desktop.ts` reads the Wails global at call time. In tests, mock the global before each test and restore it after:

```ts
// desktop.test.ts pattern
afterEach(() => {
  // restore window.go after each test
});

describe("greet", () => {
  it("delegates to the Wails runtime", async () => {
    // set window.go.main.App.Greet to a mock, then assert greet() calls it
  });
});
```

This approach keeps the binding testable without importing the generated `wailsjs/` directory.

### 4. Desktop frontend — React component (`apps/desktop/frontend/src/components/Greeting.tsx`)

`Greeting` is a pure presentational component (`({ message }) => <p>{message}</p>`). It is tested with Testing Library:

```tsx
// Greeting.test.tsx
describe("Greeting", () => {
  it("renders the message", () => {
    render(<Greeting message="Hello, World!" />);
    expect(screen.getByText("Hello, World!")).toBeInTheDocument();
  });
});
```

The component itself holds no logic — all logic is in `lib/` — so a single render assertion is sufficient.

### 5. Shared contract — unit tests (`packages/shared/src/index.test.ts`)

`@option-tab/shared` exports `releaseAssetName`, `downloadUrl`, and `latestReleaseUrl`. These are pure functions tested with Vitest. Because they are the contract between the release pipeline and the landing page, any change to asset naming must be reflected in the tests.

Run:

```bash
cd packages/shared && bun run test
```

### 6. Landing page — pure lib function (`apps/web/lib/download.ts`)

`detectPlatform(userAgent)` maps a User-Agent string to `Platform`. It is a pure function tested in `download.test.ts`:

```ts
describe("detectPlatform", () => {
  it("detects macOS", () => { /* ... */ });
  it("detects Windows", () => { /* ... */ });
  it("defaults to linux", () => { /* ... */ });
});
```

### 7. Landing page — Playwright smoke (`apps/web/e2e/landing.spec.ts`)

Playwright runs against the fully-built static site (`apps/web/out`). The smoke tests verify:

- The landing page renders without error.
- All three platform download links are present and point to valid GitHub Release URLs.
- The primary download button changes based on the platform detected from the User-Agent.

```bash
# Build first (required: Playwright serves the static output)
task build
task e2e
# Or in CI: the e2e job depends on build in turbo.json
```

---

## Test configuration summary

| Workspace | Runner | Config |
|-----------|--------|--------|
| `apps/desktop` (Go) | `go test` | standard Go toolchain |
| `apps/desktop/frontend` | Vitest | `vitest.config.ts` (`jsdom` environment) |
| `apps/web` | Vitest | `vitest.config.ts` (`node` environment, `lib/**/*.test.ts`) |
| `apps/web` (e2e) | Playwright | `playwright.config.ts` (serves `out/` on port 3000) |
| `packages/shared` | Vitest | default Vitest config |

---

## CI enforcement

The `go` job in `ci.yml` runs `go test ./... -race -cover`. The `js` job runs `bun run test` (Turbo → all Vitest workspaces) and `bunx playwright install --with-deps chromium && bun run e2e` for Playwright. Both jobs must pass before merging to `main`.

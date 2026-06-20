# Contributing

Thank you for your interest in contributing to option-tab. This document covers the workflow, tooling conventions, and quality bar expected for contributions.

## Fork and branch flow

1. Fork the repository on GitHub.
2. Clone your fork and create a feature branch from `main`:

   ```bash
   git clone https://github.com/<your-username>/option-tab.git
   cd option-tab
   git checkout -b feat/my-feature
   ```

3. Make your changes, commit, and push to your fork.
4. Open a pull request against `main` in the upstream repository.

Keep branches focused on a single concern. Prefer small, reviewable PRs over large all-in-one changes.

---

## Conventional Commits

All commit messages **must** follow the [Conventional Commits](https://www.conventionalcommits.org/) specification. This is enforced by the `commit-msg` Lefthook hook — non-conforming messages are rejected at commit time.

Format:

```
<type>(<optional scope>): <short description>
```

Allowed types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`.

Examples:

```
feat(greeter): support emoji in names
fix(desktop): handle empty name input correctly
docs: add architecture overview
chore(release): bump version to 0.2.0
```

Breaking changes append `!` after the type/scope:

```
feat(shared)!: rename releaseAssetName parameters
```

---

## Git hooks (Lefthook)

Lefthook is installed automatically when you run `bun install` (via the `prepare` script). It registers three hooks:

| Hook | What runs |
|------|-----------|
| `pre-commit` | Biome formats and lints staged JS/TS/JSON files (auto-fixes staged); gofumpt formats staged Go files (auto-fixes staged); golangci-lint lints staged Go files |
| `pre-push` | `task test` — runs all unit tests before the push lands |
| `commit-msg` | Validates the commit message against the Conventional Commits pattern |

If Biome or gofumpt auto-fix your files, the fixes are staged automatically. You do not need to `git add` again; just commit.

If the `pre-push` test run fails, fix the failing tests before pushing.

---

## Before opening a pull request

Run the full local check suite to catch issues early:

```bash
bun install          # ensure deps are up to date
task lint            # Biome + golangci-lint
task test            # Vitest + go test -race -cover
task build           # verify the full build succeeds
task e2e             # Playwright smoke tests (optional; requires task build)
```

All four must pass. CI runs the same checks on every PR.

---

## Code style

- **JS/TS:** Biome enforces formatting (2-space indent, 100-char line width, double quotes) and recommended lint rules. Run `bunx biome check --write .` to auto-fix.
- **Go:** gofumpt enforces formatting; golangci-lint v2 enforces lint rules configured in `.golangci.yml`. Run `gofumpt -w ./...` and `golangci-lint run ./...` in `apps/desktop/`.
- Do not disable lint rules inline unless you include a comment explaining why.

---

## Adding tests

Follow the layer rules described in [docs/testing.md](docs/testing.md):

- Business logic belongs in `internal/<unit>/` (Go) or `src/lib/` (frontend/web) behind a pure interface or function.
- Test new behavior with table-driven tests (Go) or Vitest `describe`/`it` blocks (TS).
- Framework-bound layers (`app.go`, React components) stay thin; test them lightly via mocks or simple render assertions.

---

## License and sign-off

By contributing, you agree that your contribution will be licensed under the [Apache-2.0 License](LICENSE).

We follow the [Developer Certificate of Origin (DCO)](https://developercertificate.org/). Add a `Signed-off-by` trailer to each commit if required by the project:

```bash
git commit -s -m "feat: add new feature"
# Produces: Signed-off-by: Your Name <you@example.com>
```

---

## Getting help

Open a GitHub Issue for bug reports and feature requests. For questions, start a GitHub Discussion.

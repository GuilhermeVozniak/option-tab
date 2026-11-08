# Switcher reliability implementation plan

> Execute inline with superpowers:executing-plans; use regression tests and a fresh final review.

**Goal:** Fix the six reproduced bugs, then review preview ordering and Option/Command+Tab changes.

**Architecture:** Keep native lifecycle decisions in Go and keyboard/rendering state in the existing React renderers. Scope asynchronous input to its originating session. Preserve current shortcut defaults and ordering policy during the six fixes.

**Tech stack:** Go/Wails, React/TypeScript, Vitest, Playwright.

**Spec:** The six findings in `refactor-notes/review-2026-11-08/review.md`, approved for implementation in this conversation.

## Constraints and review focus

- No new dependencies; preserve user's review notes and unrelated files.
- Test delayed replies, older state echoes, session reopening, and backspacing before acknowledgement.
- Check overflowing app rails and preview galleries without changing keyboard focus.
- Retire active switcher and pending fades before focusing Preferences without lock inversion.
- Automatic update policy changes must not disable explicit manual installation.
- Recorder blur, unmount, rerender, and rapid refocus must not save stale state.
- Initial scope: work on a local fix branch. The user subsequently requested opening a PR and merging into origin/main; no release requested.

## Tasks

- [x] 1. Add permanent regressions from the recorded reproductions. Run and verify expected failures.
- [x] 2. Fix search accumulation/session ownership in the renderers and bridge. Test delayed echoes, reordered requests, and new sessions.
- [x] 3. Scroll selected window/app cards into view; consume effective app style. Test in components and real browser geometry.
- [x] 4. Retire switcher when opening Preferences and recheck update policy before auto-install. Test with fake platform/HTTP; no live update.
- [x] 5. Cancel retired recorder sessions and invoke the latest settings callback. Test cancellation and rerenders.
- [x] 6. Run JS suites, frontend build, Go race tests and focused browser tests. Review changes with a fresh reviewer.
- [x] 7. Inspect default/migrated shortcuts, native matching/release, MRU and preview grouping/order. Reproduce supported findings and report any remaining issues.

## Execution ledger

- Initial baseline: clean tracked tree at `0616f27`; previous review run passed 675 JS tests, all Go packages with race detector, and frontend build. Reproduction tests demonstrated all six defects.
- Ruling: use a local branch in the existing checkout; dependencies and review artifacts are already prepared here. User authorized the concrete fixes, so no second design approval is needed.
- Pre-flight: renderer search behavior and route bridge dispatch share query/session ownership; scrolling and style share renderer files. Execute those edits serially.
- Implemented all six fixes, including session/sequence-aware search and regenerated Wails bindings.
- Fresh review identified two further races within these fixes: overlapping recorder cancellations and delayed search snapshots. Added reproductions, fixed both, and obtained a clean follow-up review for P1/P2 issues. Added real-browser app-gallery scrolling coverage.
- Separate follow-up review reproduced three open issues: initial selection after filtering the current window, alphabetical ordering of windowless apps, and lost Shift direction on native activation/rearming. Evidence is in `refactor-notes/review-2026-11-08/preview-ordering-and-shortcuts.md`; failing review probes remain outside normal test discovery.
- Final validation: 687 JS tests passed (676 desktop, 7 shared, 4 web); all 25 Go packages passed with the race detector; 55 focused Chrome browser tests passed. Frontend TypeScript/Vite build, native desktop build to `/tmp/option-tab-review-fixed`, JS lint, Go vet, and `git diff --check` passed.
- One existing media-provider test timed out during a run concurrent with heavy validation. Its full file passed separately (60 tests), and the subsequent complete JS run passed with no timeout changes.
- Go lint remains unavailable: installed golangci-lint 2.13.1 cannot decode Go 1.27.3 standard-library export data (version 5, supported maximum 4). No toolchain or global tool was changed. Native builds retain existing macOS deprecation/deployment-target warnings; Vite retains its large-chunk warning. No live native switch, update installation, merge, push, or release was performed.
- Integration follow-up: user requested a PR and merge into origin/main. Fetched origin/main and confirmed it matches the branch's base. Rechecked JS suites (matching Turbo cache) and all Go race tests with `-count=1`; both passed. Preserve untracked review notes in the workspace.

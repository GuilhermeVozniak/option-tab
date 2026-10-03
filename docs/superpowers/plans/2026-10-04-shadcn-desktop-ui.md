# Desktop shadcn migration implementation plan

**Goal:** Migrate Settings and Alt-Tab presentation to official shadcn primitives while preserving the approved v0.6.0 UI.
**Architecture:** Registry-owned local components supply interaction and accessibility. Existing application state, native events, geometry, and visual rules remain authoritative.
**Tech stack:** React 19, TypeScript, Tailwind 4, Vite, shadcn New York/Radix, Vitest, Playwright Chromium/WebKit.
**Spec:** `docs/superpowers/specs/2026-10-04-shadcn-desktop-ui.md`

## Constraints and review focus

Preserve dimensions/copy/materials; inactive panels stay mounted; imported or stale Settings disable all controls; Settings themes do not leak into runtime surfaces; native keyboard, gesture, and window identity code stays unchanged. Review disabled sliders and portaled controls, label association, radio arrow keys, theme resolution in WebKit, and hidden drafts.

## 1. Shared foundation (root)

- [x] Audit existing primitive and theme configuration and verify official registry/docs.
- [x] Create isolated `refactor/shadcn-desktop-ui` worktree.
- [x] Run baseline frontend tests: 655 tests pass on untouched main.
- [x] Capture before screenshots with reproducible production-browser scripts.
- [x] Add focused Button composition test; run against old implementation to demonstrate missing `asChild` behavior.
- [x] Add official components with the shadcn CLI; inspect registry/dependency changes.
- [x] Adapt brand styles and preserve explicit unstyled presentation variants for custom runtime surfaces.
- [x] Complete theme tokens and a disabled context used by controls; pass canonical busy state from App into Settings.
- [x] Verify primitives, typecheck, and settings visual styles.

## 2. Settings migration (Settings worker)

Files: `src/settings/**/*.tsx`, `src/widgets/WidgetSettings.tsx`, `src/widgets/WidgetPackages.tsx`, corresponding tests. Visual verification worker owns Settings CSS; root owns shared UI components.

- [x] Replace native selects with NativeSelect + NativeSelectOption and toggle-shaped Checkbox with Switch checked/onCheckedChange.
- [x] Use RadioGroup/RadioGroupItem, Slider number-array callbacks, ToggleGroup, Tabs with forceMount/hidden, and Collapsible where appropriate.
- [x] Migrate remaining buttons, labels, textareas, inputs, and status presentation to shared primitives while retaining classes/copy.
- [x] Preserve all drafts, deep links, storage preference, and controlled settings callbacks.
- [x] Add explicit Settings disabled prop/provider and scoped resolved theme class.
- [x] Run existing Settings/editor tests, updating only interactions that correspond to real accessibility/API changes. Preserve assertions.

## 3. Switcher migration (switcher worker)

Files: window Overlay, AppSwitcher components, WindowControls, ActionNotice, corresponding tests. Root owns global styles and App.

- [x] Compose actionable controls, status, and surfaces from shared primitives with existing visual classes.
- [x] Preserve application-specific image/layout elements, hit regions, native key ownership, guarded window actions, and preview identity.
- [x] Do not add Dialog/Command focus or keyboard ownership.
- [x] Run switcher/action tests and compare all 17 captured switcher scenarios.

## 4. Integration and verification (root + independent reviewer)

- [x] Resolve type/style integration issues; complete source inventory so no bespoke duplicate form controls remain in scope.
- [x] Run monorepo lint, each workspace unit suite, frontend/website production builds, Go race tests, desktop browser suite and Chromium/WebKit Settings tests.
- [x] Add/run meaningful keyboard and canonical-busy regression tests for changed primitives.
- [x] Compare all Settings/theme/editor screenshots and runtime switcher baselines; inspect differences.
- [x] Review official registry provenance, CLI info, theme tokens, and dependency lockfile.
- [x] Independent whole-change review; resolve findings and rerun affected checks.
- [x] Record validation and commit the completed migration on its branch. Publishing is a separate release action.

## Validation record — 2026-10-04

- Desktop unit suite: 662 tests / 49 files passed with `bun run test --maxWorkers=2`.
- Website unit suite: 4 passed; shared package unit suite: 7 passed.
- Desktop Chromium browser suite: 145 passed. Settings WebKit suite: 22 passed. Both used the same final production preview; Chromium includes the Settings tests.
- `bun run lint`: all four tasks passed. Desktop TypeScript/Vite and website production builds passed. `go test ./... -race -cover` passed.
- `bun install --frozen-lockfile` and `shadcn info` passed. Components come from the official New York/Radix registry, installed with pinned shadcn 4.21.1.
- Visual comparison: 120 before/after screenshots; 94 pixel-identical, including every one of the 17 switcher captures. Remaining images differ by 20–92 pixels (at most 0.0271%) in checkbox corners or disclosure triangles. All 62 Settings layout/scroll measurements match, with no horizontal overflow. Twenty-seven semantic tokens were verified in six theme combinations.
- Composition and accessibility regressions were reproduced before fixes, then passed: Button Slot/ref composition, radio state and arrow focus, Collapsible disabled inheritance, canonical busy state, visible labels, slider keyboard changes, and native save payloads. Existing draft and native identity tests remain intact.
- Independent final review found no remaining serious issues. Source audit found no remaining native form-control wrappers in the migrated Settings or switcher renderers.

Private evidence is in the main checkout at `.superpowers/sdd/shadcn-migration/`, including raw before/after images, comparisons, CLI provenance, build logs, test logs, and earlier failed runs. It is excluded from this implementation commit.

### Limits and maintenance notes

Browser tests use the existing fake Wails backend. No new native app was launched and physical gestures were not retested. Native keyboard/gesture routing and backend behavior were not modified.

One test now explicitly removes/restores ResizeObserver to exercise the existing resize fallback, since Radix requires a jsdom observer shim. A concurrent full unit run hit timeouts while browser/build jobs were active; the affected 144 tests and the full 662-test suite subsequently passed with two workers and unchanged timeout limits. Existing React `act(...)` warnings remain.

The existing Vite 500 kB chunk warning remains. Desktop JavaScript grows from 643,616 to 721,078 bytes (about 25 kB additional gzip); CSS grows from 97,429 to 121,418 bytes (about 4 kB additional gzip). This is the measured cost of the shared Radix/shadcn component system and theme utilities; no warning thresholds were relaxed.

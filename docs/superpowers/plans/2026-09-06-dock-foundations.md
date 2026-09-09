# Dock Enhancement Foundations Implementation Plan

> **For agentic workers:** Use superpowers:subagent-driven-development. Steps use checkbox syntax for tracking.

**Goal:** Implement the action, interaction and capture foundations required by the complete narrowed Dock enhancement roadmap.
**Architecture:** Typed explicit-target actions sit above native capabilities. React interaction settings are shared across switcher surfaces. Visible capture has a bounded session lifecycle.
**Tech Stack:** Go 1.26, Wails v3 alpha2.117, Objective-C/AppKit/ScreenCaptureKit, React/TypeScript/Vitest.
**Spec:** `docs/superpowers/specs/2026-09-06-dockdoor-parity-design.md`

## Global Constraints

- Existing features and settings must migrate without losing user shortcuts.
- No tiling, maximize/restore, centering, calendar, weather, file staging, AirDrop or generic script runner.
- New actions target explicit window/app identities and report failures.
- Native event callbacks must not perform blocking window enumeration/capture/persistence.
- Unsupported native features must not pretend to succeed in stub/browser modes.
- Work on `feat/dockdoor-parity`; preserve existing untracked refactor-notes.
- Native features require real native implementations and validation; controls or mocks alone do not prove completion.

### Task 1: explicit-target window and app actions

**Files:** Create `internal/actions/service.go` and tests, `app_actions.go` and tests, `internal/platform/darwin_actions.{go,m,h}`. May modify switcher controller only to expose refresh/selection operations with tests. Do not edit App.tsx, app.go or app_switcher.go. Paths relative to apps/desktop.
**Interfaces:** Produce `App.PerformAction(kind string, windowID uint64, appID int) (actions.Result, error)`. Result JSON fields `succeeded: number`, `failures: [{windowId:number,error:string}]`. Kinds: focus, close, minimize, fullscreen, hide, quit, forceQuit, newWindow, closeAll, minimizeAll. Window actions validate the fresh owner against appID when nonzero; bulk app actions snapshot its windows and report each failure. Optional native capability supports NewWindow and ForceQuitApp; existing Platform need not grow required methods.

- [x] Write failing service tests using a fake window source and recorded native operations. Example: closeAll app 10 sees window 1 owned by 10, 2 owned by 20, 3 owned by 10; force close 3 to fail; result must report succeeded=1, failure windowId=3, window 2 unchanged. A mismatched owner must make zero actions.
- [x] Run `go test ./internal/actions -count=1` and record failure before implementation.
- [x] Implement service plus native operations. New Window must use the app's exposed AX menu command (not global key injection); unsupported apps report an error. Force quit uses NSRunningApplication and rejects self/invalid identity.
- [x] Add App bridge with explicit result/error and controller refresh after operations; no selection mutation to target a click.
- [x] Run service, App and native build tests, review code and report red/green evidence. No commits: coordinator owns shared index.

### Task 2: configurable switcher interactions

**Files:** `internal/config/config.go` and config tests; frontend `src/lib/{types,keymap,layout,demo}.ts`, their tests, `src/overlay`, `src/settings`, `src/styles.css`, `src/lib/i18n.ts`. Do not edit App.tsx, bridge.ts, generated bindings, app.go or app_switcher.go.
**Interfaces:** Add `Appearance.CompactThreshold` JSON compactThreshold (0 disabled, otherwise switch at count >= threshold); `Appearance.LayoutDirection` JSON layoutDirection horizontal/vertical. Add `Behavior.ActionBindings` JSON actionBindings mapping physical KeyboardEvent code to action kind for held-modifier actions (preserve W/M/Q/H/F defaults, distinguish nil default vs explicit empty); `Behavior.MiddleClickAction` JSON middleClickAction (none/close/minimize), swipeUpAction/swipeDownAction (none/close/minimize/fullscreen/hide/quit; default none). No positioning choices. Extend OverlayHandlers with optional `onAction(kind, windowId, appId)` for new actions; existing mandatory callbacks remain usable.

- [x] Write failing keymap tests: remap KeyW to minimize, disable mappings with an explicit empty object, invalid/reserved bindings rejected by validation. Existing navigation and search remain unaffected.
- [x] Write failing layout/interaction tests: threshold at exact boundary selects titles, layout selection preserved, middle-click closes clicked window without confirming, vertical navigation follows chosen arrangement; swipe threshold/momentum cancellation prevents duplicate actions.
- [x] Run focused Vitest/config tests and record red output.
- [x] Implement config validation/migration, settings controls and real Overlay handlers. Wire appBadge rendering. Include new/forceQuit/closeAll/minimizeAll controls invoking onAction. Leave native dismissal lifecycle to coordinator.
- [x] Run focused tests and frontend typecheck. Report new fields needed in App.tsx; do not silently reduce requirements to pure helper tests.

### Task 3: live window capture lifecycle

**Files:** Create `internal/preview` manager/tests and native `internal/platform/darwin_stream.{go,m,h}`. May edit app.go and app_switcher.go for session fields and Show/Update/Hide lifecycle only. Do not edit config, frontend or actions/controller.
**Interfaces:** Optional platform stream capability yielding window-ID keyed frames with cancellation. Manager prioritizes selected window, caps concurrent streams at four, replaces stale targets and stops all streams on hide. Emit existing `switcher:thumbnails` and `switcher:preview` payload shapes.

- [x] Write failing manager tests: show five windows starts at most four, selected window included; selection replaces lowest priority; Hide cancels all; late frames from old generation never emit; close removes cached images; native unsupported case selects snapshot fallback.
- [x] Run `go test ./internal/preview -count=1` and record red output.
- [x] Implement true ScreenCaptureKit SCStream output using native frame callbacks, throttled frame delivery and cancellation; do not call periodically sampled screenshots continuous capture. Native errors/permission loss terminate sessions and preserve honest snapshot fallback.
- [x] Wire lifecycle into App, including bounded background cache and stop on app shutdown. Keep no hidden capture unless existing background preference enabled. Avoid duplicate concurrent snapshot preview jobs.
- [x] Run manager/App tests and Go native build; report OS capture smoke-test evidence or exact remaining native validation.

### Task 4: integrate, review and continue full roadmap

**Files:** App.tsx, bridge.ts, bindings, app_settings.go/app_switcher.go only after owners finish, browser e2e, roadmap and execution ledger.

- [x] Wire PerformAction to all frontend action paths with visible failures and explicit targets, including native keyboard forwarding.
- [x] Add failing browser test for action error display and explicit target preservation across selection changes, then implement and pass it.
- [x] Complete native dismissal delay without hiding a newly reopened overlay; test rapid hide/show invalidation.
- [x] Run task lint, task test, task build and task e2e. Obtain independent task reviews and fix findings.
- [ ] Mark only proven roadmap items complete. Continue with grouped app switching, native Dock observation/panels/gestures, folders/media, automation, distribution and optional replacement through subsequent focused plans. The full roadmap remains the objective; this plan is its first stage.

## Checkpoint outcome

The foundation tasks are implemented and reviewed; the full retained roadmap remains active. See [validation and limitations](../reports/2026-09-06-dock-foundations.md). The next milestone is [app groups and Dock previews](2026-09-06-app-groups-and-dock-previews.md).

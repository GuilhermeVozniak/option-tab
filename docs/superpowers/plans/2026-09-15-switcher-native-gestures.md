# Native switcher gestures implementation plan

> **For agentic workers:** Use subagent-driven development: the frontend reporter/settings task runs alongside the native/core task. Each task follows red/green tests. Root owns commits, packaging and PR integration.

**Goal:** Enable retained B08 up/down actions in both switchers using qualified native precise gesture input and exact presentation/target admission.

**Architecture:** Attach an AppKit local scroll-event monitor to the existing Wails overlay. Reuse the Dock's bounded wheel ownership/acknowledgment transport and pure reducer, publish clipped rendered card regions, and dispatch guarded actions off the native input thread.

**Tech stack:** Go, AppKit Objective-C, Wails, React/TypeScript, Vitest, native property fixtures.

**Spec:** `docs/superpowers/specs/2026-09-06-dockdoor-parity-design.md`, retained B08 in `docs/dockdoor-roadmap.md`, and D12 qualification contract in `2026-09-06-dock-input-and-preview-drag.md`.

## Constraints

- Preserve existing independent app/window settings and all saved actions: none, close, minimize, fullscreen, hide, quit. Minimize/fullscreen keep pointer toggle behavior.
- No DOM wheel/mouse-drag action fallback. Ordinary coarse and phase-less wheels pass through. Precise gesture input cannot establish exact finger count or exclude every precision mouse.
- Reuse exact process/window identity and final native admission after blocking lookup. No action callbacks, AX, Go callbacks or Wails calls inside event classification.
- No user app/window actions, app launch, public release or subagent commits. Disposable owned native property/action fixtures are permitted.
- Widget-localization worker owns both translation catalogs. Root generates bindings after stable signatures and runs final integration/package checks.

## Task 1: Native source and immutable ownership

Files: `internal/platform/darwin_dock_panel.m`, `darwin_switcher_wheel.go`, `switcher_wheel.go`, `testdata/panel-wheel/main.m`, new switcher native fixture/test.

- [x] Write native fixture assertions for overlay-specific host checks, precise/phase admission, immutable regions, hide/resize cancellation, pass-through and pending acknowledgment; observe RED.
- [x] Reuse the existing native wheel classifier/mailbox with an overlay-specific record/monitor adapter. A never-reused token owns the host/content and monitor; only copied events cross to a serial Go worker. Native retirement preserves queued terminal cancellation.
- [x] Add optional `SwitcherWheelHost.CreateSwitcherWheel(unsafe.Pointer)` returning a source with policy/validate/complete/close methods. Attach only to a live exact NSApp window; nil/retired hosts fail without waiting for AppKit. Keep all AppKit work outside App locks.
- [x] Verify native fixture, nil-host test, wheel worker transport, and existing Dock wheel tests.

## Task 2: Guarded pointer actions and app integration

Files: `internal/platform/{switcher_wheel.go,darwin_active_window.go,darwin_active_window.m,darwin_active_window.h}`, `app_switcher_gestures.go`, lifecycle call sites, core/App tests.

- [x] Write failing action-fixture cases for close, minimize/fullscreen toggle, hide, quit and retirement during native preparation. Write App tests for exact target, independent mode maps, settings/session/geometry changes, delayed replies and acknowledgment.
- [x] Add a guarded pointer performer using captured process/window identity and existing native final-admission checks. Never substitute explicit automation minimize/fullscreen semantics or global shortcuts.
- [x] Implement `GetSwitcherGestureCapabilities() {available:bool}` and `SetSwitcherGestureRegions(session,stateRevision,sequence,regions) error`. Regions carry `{windowId,appId,bounds:{x,y,w,h}}`; match current displayed entries and finite positive clipped bounds. A monotonic input authority retires changed policies/geometry/session; ordinary selection changes cannot retarget a captured region.
- [x] Reuse `dock.PanelGestureRecognizer` with bottom-edge away/toward mapped to current mode up/down and horizontal actions none. One pending worker owns its native token, validates before/after native checks, dispatches once, reports scoped `switcher:gestureError {session,revision,message}`, acknowledges on all outcomes and refreshes successful current presentations.
- [x] Invalidate before hide, settings publication, session inactivity, native host resize/close and shutdown. Verify Go race tests and native action fixtures.

## Task 3: Rendered regions, status and settings

Files: frontend `lib/useSwitcherGestureRegions.ts`, `lib/bridge.ts`, `App.tsx`, both switchers/card components, `Settings.tsx`, `ControlsTab.tsx`, focused tests/e2e.

- [x] Write failing geometry/capability tests. Existing DOM drag/wheel rejection tests remain.
- [x] Publish clipped actual card rectangles for both modes/all styles; exclude toolbar, rail, backdrop, delayed apparition and fade frames. Observe scroll/resize and state revision; use monotonic sequence with empty retirement publications.
- [x] Enable saved selectors from real native availability. Explain precise gesture semantics in EN/PT/ES; show current-session native errors through existing action feedback.
- [x] Verify focused Vitest/browser tests. Coordinate generated bindings with root; no handwritten generated artifacts.

## Task 4: Integration evidence

- [x] Review source for cancellation, lock ordering, source identity and ownership gaps; run relevant race/native/frontend checks after final edits.
- [ ] Root runs full integration checks, reviews PR batch and creates the local build without launching it.
- [ ] Record physical trackpad, ordinary mouse wheel and real Wails delivery as separate acceptance evidence. Synthetic fixtures prove extraction/classification/transport/actions, not physical hardware delivery or finger count.

# Dock monitor locking implementation plan

> **For agentic workers:** Use superpowers:subagent-driven-development. Root owns shared contracts, App integration, bindings and git; workers own disjoint native, controller, and settings/UI files.

**Goal:** Deliver retained D15: an opt-in native Dock monitor lock, bypass modifier, disconnect recovery and an explicit placement action.

**Architecture:** A dedicated native owner observes the Dock container and display UUIDs off its event callback. It prevents migration by moving eligible off-target edge events inward only while fresh observations verify the Dock on its target. A cancellable, user-invoked placement operation uses the same owner and verifies the resulting Dock location.

**Tech stack:** Go/Wails, Objective-C Accessibility/CoreGraphics/AppKit, React/TypeScript.

**Spec:** `docs/superpowers/specs/2026-09-06-dockdoor-parity-design.md`, milestone 4. This implements the already approved retained roadmap.

**Current checkpoint:** Manual placement plus native monitor protection is the available product path. Public `DockPlacementAvailable()` is false; the UI hides automatic placement and public placement never queues input. Experimental native movement tests proved one-point cursor conversion/restoration and one outbound Dock relocation, but the return cancelled on unmarked input evidence. Automatic placement and broad physical acceptance remain unfinished. The earlier explicit-placement design below is retained as the target for that follow-up, not as a shipped capability claim.

## Constraints and product behavior

- Independent of hover previews; disabled by default. Pausing Option Tab, session inactivity and shutdown remove protection.
- Select the main display dynamically or a persistent display UUID. Preserve a disconnected UUID; never silently substitute a similarly named screen.
- Default bypass is Option; allow exactly one of Option, Command, Control or Shift. Held bypass passes input unchanged. If the Dock migrated, releasing bypass shows `awaitingPlacement` until it returns or the user selects Move Dock here.
- No persistent Dock preference changes, window actions or shared ownership with icon gestures, preview drag, or passive shake taps.
- No documented native monitor setter exists. Do not label protection active until actual container geometry verifies placement. AX uncertainty, expiry, mirrored/ambiguous geometry, inaccessible target edges and disconnect leave input unchanged.
- Movement filtering affects only exposed off-target Dock edges, with no held mouse button. Clicks, wheel, drag, synthetic and bypass input pass unchanged.
- Move Dock here explicitly explains temporary pointer movement. One bounded attempt; cancel on physical input, bypass, topology/configuration/session change or deadline. Restore the cursor only while still owning it; physical takeover prevents restoration. Verify Dock placement after restoring before reporting success. No automatic pointer relocation.
- Public statuses: `disabled`, `starting`, `protected`, `bypassed`, `awaitingPlacement`, `placing`, `disconnected`, `unreachable`, `unavailable`.

## Shared interfaces

`internal/platform/dock_lock.go` defines copied display/state DTOs, immutable policy, placement request/result and optional `DockMonitorLockSource`. Policy and state carry Session/Revision; native environment changes advance Generation. Placement carries the exact Session/Revision/Generation/RequestID. Display inventory is a separate bounded read without installing a filter. All geometry is global top-left logical points.

`dock.NewMonitorLockController(MonitorLockControllerDeps{Source, Changed})` supplies `Configure(enabled bool, policy platform.DockMonitorLockPolicy)`, `Run(context.Context)`, `Snapshot() platform.DockMonitorLockState`, `PlaceScoped(context.Context, session, revision, generation uint64) (platform.DockPlacementResult,error)`, and `CancelPlacement()`. It serializes native lifetimes and placement admission, copies snapshots, retries failed sources at most once per second, and suppresses obsolete callbacks/results. The convenience `Place(ctx)` snapshots before delegating to the same scoped admission, so racing configuration changes still refuse.

Root exports `GetDockMonitorLockState`, `GetDockMonitorLockDisplays`, `PlaceDockOnSelectedMonitor(session, revision, generation uint64)`, and `CancelDockPlacement`; emits `dock:monitor-lock` with copied state. The UI captures the exact runtime scope on click; App synchronizes published settings before admission. Native placement never runs while App locks are held. The settings UI subscribes before fetching its initial state and rejects older Session/Revision/Sequence snapshots. Cancel is exposed only once runtime status reports admitted placement, avoiding cancellation before admission.

## Task 1: Native contract and monitor owner

Files: `internal/platform/dock_lock.go`, `darwin_dock_lock.{go,m,h}`, `dock_lock_stub.go`, focused native tests and fixture README.

- [ ] Write failing geometry/admission tests for negative coordinates, all three Dock orientations, partially covered edges, wholly covered edges, mirroring, stale snapshots and bypass.
- [ ] Implement bounded read-only display UUID inventory and exact Dock-container AX observation. Do not reuse hovered-app identity as the monitor authority.
- [ ] Implement a separate active movement tap with short-lived immutable environment data, bounded callback logic, pass-through outside eligible edges and joined cleanup. Expired/reconfigured environment invalidates protection synchronously.
- [ ] Write failing placement tests for current generation, one owner, physical takeover, timeout, explicit cancellation and restore ownership. Implement tagged synthetic movement with a fixed deadline and observed placement verification.
- [ ] Exercise callback/worker seams without desktop input. Provide a read-only real Dock/display snapshot smoke and a separately gated multi-display interaction fixture; clearly distinguish simulated and physical evidence.

## Task 2: Controller lifetime and copied state

Files: `internal/dock/monitor_lock.go`, `monitor_lock_test.go`.

- [x] Write failing tests: disabled source never starts; configure invalidates synchronously; previous source joins before replacement; delayed callbacks cannot overwrite current state; repeated failure retries are bounded.
- [x] Implement the contract above, returning `disabled` while inactive. Invoke Changed without controller locks and prevent stale callbacks escaping source retirement.
- [x] Test one pending placement, exact source generation, cancellation on settings/inactivity/shutdown, external blocked operation refusal and copied snapshots. A late result from an old source must not become success in the current session.
- [x] Run focused race tests and release files for root integration.

## Task 3: Persisted settings and localized controls

Files: `internal/config/dock.go`, `config.go`, `dock_test.go`, frontend `lib/types.ts`, `lib/i18n.ts`, settings `DockTab.tsx`/`Settings.tsx`, new `DockMonitorLock.tsx`, bridge and focused unit/browser cases.

- [x] Add `Dock.MonitorLock { enabled, target: main|display, displayUUID, bypassModifier }` with default-first migration, preserved explicit false and validated UUID/modifier/target. Missing old configuration defaults to disabled/main/Option.
- [x] Add independent toggle, target selector preserving disconnected selections, bypass selector, actual runtime status and visible source/action error. Move/cancel controls exist only when the backend advertises proven placement capability; production currently shows manual-placement guidance.
- [x] Explain pointer movement beside a supported explicit action and waiting/manual placement behavior beside status. Do not request screen recording for locking. Accessibility requests happen only on the explicit enable flow.
- [x] Add EN/PT-BR/ES strings; test settings persistence, disconnected selection, stale status, disabled/placing action admission and surfaced failure. Do not edit generated bindings; root regenerates.

## Task 4: App integration and acceptance

Files: `app_dock_lock.go`, `app_dock_lock_test.go`, lifecycle calls in `app.go`/`app_dock.go`; generated bindings; milestone report and roadmap.

- [x] Wire the optional source, configure separately from hover preview suspension, run on the existing application lifetime, cancel on pause/inactivity/shutdown and expose runtime DTOs/actions.
- [x] Test no lock activation by default, preferences/switcher not disabling it, exact settings revision, disabled admission, cancel during placement, and no App locks across native operations. Published target changes and earlier UI scopes are refused before native placement.
- [x] Run focused race/unit/browser tests, lint and production builds. Avoid parallel frontend rebuild and Go embedding. Independent review checks actual ownership, geometry and pass-through contracts.
- [ ] Record real read-only display/Dock evidence and remaining physical multi-monitor gaps. Keep D15 acceptance unchecked until actual prevention, bypass, placement, orientations, auto-hide, disconnect and mixed scaling pass.
- [ ] Commit and push the coherent milestone to draft PR #28; continue folders/media and automation.

## References and implementation boundary

Apple's event-tap contract permits active event filtering: https://developer.apple.com/documentation/coregraphics/cgeventtapoptions. Display UUIDs use https://developer.apple.com/documentation/colorsync/cgdisplaycreateuuidfromdisplayid%28_%3A%29; topology invalidation follows https://developer.apple.com/documentation/coregraphics/cgdisplayreconfigurationcallback.

Read-only research found edge filtering and separately simulated relocation in DockDoor and lockdock. These are feasibility references, not source imports. Implement independently from public native API behavior and our own tests. Actual placement remains a native acceptance question.

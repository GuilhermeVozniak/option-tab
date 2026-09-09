# Local AppleScript automation implementation plan

**Goal:** Implement retained F01–F06 from `docs/dockdoor-roadmap.md`, through the existing Option Tab services and a packaged AppleScript dictionary.

**Spec:** `docs/superpowers/specs/2026-09-06-dockdoor-parity-design.md`, milestone 6.

**Architecture:** A private Apple event suite receives bounded commands on AppKit's main thread, suspends each reply, then dispatches Go work asynchronously. A typed automation service resolves identities and calls existing switching, presentation and window-action services. Replies resume exactly once on the native owner thread. Preserve the Wails application delegate and existing event handlers.

This plan incorporates the coordinator's prior read-only native research. Public API names and packaging behavior must be verified against the local SDK during the native implementation spike. No native feasibility or end-to-end acceptance is claimed by this document.

## Scope and invariants

- Local AppleScript/osascript and macro-tool use only. No new CLI, network listener, socket server, generic script execution, AppIntents product, tiling or saved commands.
- Commands: open switcher; show/hide app previews; focus, close, minimize, hide and fullscreen a window; query apps, windows and active-window details. Query images are explicitly requested existing cache data only.
- App selectors are typed name, bundle ID or PID. Exact matching; ambiguity is an error. Names are not durable identity. Capture resolved PID and process launch identity, and revalidate immediately before mutation.
- Explicit window IDs resolve to an exact app/process identity. The active-window selector uses authoritative AX focused-window evidence, never first inventory entry or frontmost-app guessing.
- Queries do not activate apps, show UI, prompt for permissions or initiate capture. Missing permissions and unavailable active-window evidence produce explicit errors/unknown fields as appropriate.
- Preview coordinates use global top-left logical points and clamp to an actual display. Automation presentation has its own owner/session; it must not invent a Dock icon or mutate native hover identity.
- A late request cannot act after timeout, shutdown, owner retirement or identity replacement. Native handlers do not wait for Go/AX/AppKit work on the main thread.
- Preserve Apple-event/TCC refusal behavior. No bypass of macOS automation authorization. Returned errors and JSON are bounded and contain no unrelated application data beyond the requested query scope.

## Proposed file boundaries

- New `apps/desktop/internal/automation/{types,resolve,service}.go` and corresponding tests: command DTOs, selector resolution, request lifetimes, queries and action dispatch.
- New `apps/desktop/internal/platform/automation.go`, `darwin_automation.{go,m,h}`, portable stub and tests: private suite registration, reply lifecycle, native error conversion.
- New `apps/desktop/internal/platform/active_window.go`, `darwin_active_window.{go,m,h}`, portable stub/tests: authoritative active root and captured process identity, if no equivalent port exists when work begins.
- New `apps/desktop/app_automation.go` and tests: Wails/App lifetime integration and automation-owned presentation.
- Narrow existing controller addition for idempotent switcher `Open`; cache metadata extension in current preview manager/cache owner; presentation/controller integration only where necessary.
- New `apps/desktop/build/darwin/OptionTab.sdef`; narrow `Info.plist` and packaging-resource edits.
- New `apps/desktop/testdata/automation/` packaged smoke harness and disposable fixture; new `docs/automation.md`.

Root owns shared interfaces, App hooks, bindings/packaging integration and checkpoints. Delegate disjoint native/service/UI tasks only after contracts are frozen. Never edit generated bindings concurrently with frontend builds.

## Task 1 — Typed service, selectors and explicit errors (F02/F04 foundation)

1. Define versioned JSON query envelopes and typed selectors. Prefer distinct selector fields over a string that ambiguously means a PID, name or bundle identifier. Define request ID, deadline and cancellation scope.
2. Write failing tests for exact names, duplicate names, duplicate running bundle instances, missing/reused PID, invalid window ID, cancelled requests and excessive query size. No action should run on ambiguous resolution.
3. Implement resolver over copied existing app/window inventories. Preserve windowless apps. Do not use title equality as window identity.
4. Add structured errors such as invalid argument, ambiguous target, unavailable target, permission denied, cancelled, timed out and unsupported operation. AppleScript error numbers map centrally to those errors.
5. Test stable JSON, empty inventories, escaping/unicode, unknown active-window evidence and count/byte limits. Document truncation explicitly rather than returning apparently complete partial results.

**Gate:** Focused service tests and race tests pass; no native event registration or window action is needed.

## Task 2 — Native Apple-event transport and packaged dictionary (F01/F06 foundation)

1. Verify `NSAppleScriptEnabled`, `OSAScriptingDefinition`, `.sdef` resource installation and the public `NSAppleEventManager` suspension/resumption API against the local SDK. Allocate a private suite/event codes without replacing standard Wails handlers or the application delegate.
2. Write isolated lifetime tests: successful reply, malformed descriptor, unknown command, bounded queue refusal, deadline, cancellation, duplicate completion and shutdown. Each suspended event must resume exactly once; stale tokens must not access retired owners.
3. Native handler validates/copies bounded descriptors, suspends the incoming event and queues work. It performs no AX queries and waits for no Go operation. Completion queues reply construction/resumption on the native owner thread; never hold a Go mutex across synchronous main-thread dispatch.
4. Keep native references native-owned and use opaque integer handles for asynchronous identity. On shutdown close admission first, cancel queued work and complete pending replies before removing handlers safely.
5. Package a unique isolated test app and run a real `osascript` query round trip. Confirm the dictionary is discoverable and existing Wails callbacks still function. Record actual TCC/refusal behavior; isolated callback tests alone do not establish transport acceptance.

**Gate:** Actual packaged osascript round trip plus deterministic reply-lifetime tests. No real-user app actions.

## Task 3 — Window queries and guarded actions (F03/F04)

1. Add the narrow authoritative active-window port if needed: frontmost process + exact AX focused root, captured process launch identity, exact window ID, bounded lookup and explicit unavailable result. Do not substitute an arbitrary window when AX evidence is missing.
2. Write failing service tests for process/window reuse between resolution and action, active-window changes, cancellation during final native lookups and unsupported action capability.
3. Route focus/close/minimize/hide/fullscreen through the existing action service. Verify its final native dispatch guard covers the automation request lifetime and captured target identity. Extend a narrow guarded capability only if the existing path cannot enforce that boundary.
4. Specify toggle versus set semantics in the dictionary explicitly. Do not accidentally implement "minimize" as a toggle or add maximize/restore/positioning actions.
5. Exercise actual actions only against a separately launched fixture's announced PID/window IDs. Verify observed postconditions and survival of unrelated fixture windows. Test stale identity refusal without touching other applications.

**Gate:** Meaningful race tests and fixture-only osascript action evidence, including actual native errors.

## Task 4 — Switcher and automation-owned preview presentation (F01/F02)

1. Write a failing regression: opening an already open switcher preserves selection/session rather than advancing. Add an idempotent controller entry point; do not simulate `HotkeyActivate`, whose existing semantics advance selection.
2. Define an automation presentation owner/token separate from native Dock hover. Show resolves the selected app, enumerates eligible windows and publishes the real preview model through the existing nonactivating host/presentation path.
3. Test exact explicit coordinates, absent coordinates, negative positions, invalid/nonfinite coordinates, display disconnect and clamping. Snapshot the actual display topology at presentation time.
4. Hide cancels only the automation-owned preview session it targets; late hide/frames/actions cannot dismiss a newer hover or automation presentation. Settings/session inactivity/shutdown retire the owner and capture subscriptions.
5. Verify actual React content and Wails action delivery in the isolated packaged app. Do not claim frontend rendering from a Go state assertion alone.

**Gate:** Idempotence/session tests, presentation geometry tests and packaged visual/bridge evidence. No fabricated Dock candidate.

## Task 5 — Optional cached image queries (F05)

1. Inspect current cache ownership before exposing it. A map from window ID to data URL is insufficient: add captured PID/process identity, window identity, capture timestamp and availability/retirement metadata to the cache boundary.
2. Write failing tests for stale process/window reuse, positive retirement, expired entries, missing images, concurrent eviction and byte/count limits.
3. Add a read-only copied cache query. `includeImages=false` is default. A true request may include only valid already-cached entries plus explicit age/availability metadata; it never starts or refreshes capture, even when recording permission is granted.
4. Bound encoded response size before constructing a large Apple-event reply. Report omitted image data explicitly. Do not silently fall back to a new screenshot.

**Gate:** No-capture-call assertions, race tests and real osascript JSON containing an existing fixture-only cached image, with missing-cache behavior also verified.

## Task 6 — Documentation and combined acceptance (F06)

1. Document dictionary commands and JSON schema with runnable Terminal `osascript` examples and generic macro-tool integration. Explain exact selector ambiguity, coordinates, permission failures, timeout behavior, optional cached images and unsupported commands.
2. Retain a reproducible isolated packaged harness with unique bundle/single-instance/settings paths. Launch only disposable fixture PIDs; constrain mutation selectors to those identities and clean up only the launched children. Generated apps/binaries remain ignored or temporary.
3. Execute real osascript coverage for app/window/active queries, idempotent switcher open, show/hide previews with coordinates, every retained action, ambiguity/permission/error replies, cancellation and shutdown. Separate any unverified physical/visual cases in the report.
4. Run focused Go race tests, frontend checks if changed, lint, packaging/resource validation and production build. Coordinate frontend embedding and native builds; do not run broad gates concurrently with deliberate compile-red work.
5. Independently review native reference ownership, exactly-once reply resumption, identity guards, response bounds and UI session isolation. Update roadmap acceptance only for actual evidence; then root checkpoints the coherent milestone.

## Acceptance limitations to preserve

- A compiled `.sdef` or simulated handler does not prove real macOS Apple-event routing or authorization.
- A frontmost app can have no authoritative active window; report that honestly.
- Cached preview availability is opportunistic and reflects capture opt-in and prior presentation, not guaranteed fresh image service.
- Window IDs and PIDs alone are insufficient for delayed mutation or cached image attribution.
- No test may perform cross-user-app mutations merely to demonstrate automation coverage.

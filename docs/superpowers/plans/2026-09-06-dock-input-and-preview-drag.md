# Dock Input and Preview Drag Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver retained Dock workflows D09–D14 with explicit targets, bounded gesture recognition, ordinary input pass-through, and preview dragging that changes window position only.

**Architecture:** Extend the existing Dock observer with a separately owned native input source. Pure Go reducers recognize icon click, scroll, modified right-click, and Aero Shake; App executes their explicit intents through the existing action service. Preview swipes stay local to the nonactivating Dock panel, while preview drag hands an exact window identity to a temporary native capture that continues beyond the webview and sets only `kAXPositionAttribute`.

**Tech Stack:** Go, Objective-C/AppKit/Core Graphics/Accessibility, Wails v3, React/TypeScript, Vitest, Playwright.

**Spec:** `docs/superpowers/specs/2026-09-06-dockdoor-parity-design.md` Milestone 4 and `docs/dockdoor-roadmap.md` D09–D14.

## Global constraints

- D09–D14 only. D15 monitor lock is a separate milestone because it observes/repositions the system Dock rather than acting on an input gesture.
- Never persistently modify system Dock preferences.
- Preview drag changes the selected window's position only. It never resizes, snaps, tiles, centers, maximizes, restores, or changes Spaces.
- Disabled features install no active filter. Unqualified events always return the original `CGEventRef` unchanged.
- A qualified gesture may suppress only the event sequence it owns. Cancellation, configuration changes, sleep/session loss, Dock restart, panel hide, and tap timeout release ownership immediately.
- Every intent contains the observed PID/window ID and is revalidated immediately before action. Never infer a target from current selection.
- Preserve dialogs, sheets, popovers, and Option Tab windows during Aero Shake “other window” actions.
- Event-tap callbacks do bounded copying/state updates only. AX lookup, app/window actions, and Wails emission run off the tap callback.
- Accessibility is required for global input filtering and AX window movement. Screen Recording remains optional and affects images only.

## Frozen contracts

Add these domain-neutral platform contracts in `apps/desktop/internal/platform/dock_input.go`:

```go
type DockInputKind string
const (
    DockInputLeftDown DockInputKind = "leftDown"
    DockInputLeftUp DockInputKind = "leftUp"
    DockInputMove DockInputKind = "move"
    DockInputScroll DockInputKind = "scroll"
    DockInputRightDown DockInputKind = "rightDown"
    DockInputRightUp DockInputKind = "rightUp"
    DockInputCancelled DockInputKind = "cancelled"
)

type DockInputEvent struct {
    Sequence, Generation, GestureID uint64
    Timestamp time.Time
    Kind DockInputKind
    Item DockItem
    DockPID int
    PointerX, PointerY float64
    DeltaX, DeltaY float64
    Button int
    Modifiers hotkey.ModSet
    Owned bool // native suppression ownership accepted, then freshly revalidated by worker
    Precise, DirectionInverted bool
    Phase, MomentumPhase string // began, changed, ended, cancelled, none
    Reason string
}

type DockInputPolicy struct {
    ClickToHide, ScrollShowHide, ModifiedRightClick bool
}
type DockInputTarget struct {
    Generation uint64
    DockPID int
    ObservedAt time.Time
    Item *DockItem // nil clears new admission; generation zero retires the source
}

type DockInputSource interface {
    ObserveDockInput(context.Context, DockInputPolicy, <-chan DockInputTarget, func(DockInputEvent)) error
}
type DockInputGestureValidator interface {
    DockInputGestureCurrent(generation, gestureID uint64) bool
}
type DockInputGestureAcknowledger interface {
    CompleteDockInputGesture(generation, gestureID uint64)
}

type PreviewDragRequest struct {
    Session, GestureID uint64
    WindowID domain.WindowID
    AppID domain.AppID
    PointerX, PointerY float64
    GrabX, GrabY float64 // normalized 0...1 position inside the preview
}
type PreviewDragResult struct { FinalX, FinalY float64 }
type PreviewDragger interface {
    DragPreview(context.Context, PreviewDragRequest) (PreviewDragResult, error)
}

type WindowRole struct { WindowID domain.WindowID; AppID domain.AppID; Role, Subrole string }
type ActionWindowRole struct {
    WindowRole
    SelfApplication bool
    RootConfirmed, RelationshipsKnown bool
    Modal, HasAttachedSheet, HasModalChild bool
    ParentWindowID domain.WindowID
    Bounds domain.Bounds
    Reason string
}
type WindowRoleSource interface { ActionWindowRoles() ([]ActionWindowRole, error) }
type OtherWindowPerformer interface {
    PerformOtherWindowAction(kind string, id domain.WindowID, app domain.AppID) error
}
type GuardedOtherWindowPerformer interface {
    PerformOtherWindowActionGuarded(kind string, id domain.WindowID, app domain.AppID, guard func() error) error
}
type WindowDragEvent struct {
    Sequence, Generation, GestureID uint64
    Timestamp time.Time
    Kind string // candidate, moved, up, cancelled
    Window WindowRole
    PointerX, PointerY, WindowX, WindowY float64
}

type PreviewRegion struct {
    WindowID domain.WindowID
    AppID domain.AppID
    Bounds domain.Bounds
}
type DockPanelWheelPolicy struct {
    Session, Revision uint64
    Enabled bool
    Regions []PreviewRegion
}
type DockPanelWheelEvent struct {
    Session, Revision, Sequence, GestureID uint64
    Timestamp time.Time
    WindowID domain.WindowID
    AppID domain.AppID
    PanelX, PanelY, DeltaX, DeltaY float64
    Owned, Precise, DirectionInverted bool
    Phase, MomentumPhase string
    Reason string
}
type DockPanelWheelSource interface {
    SetDockPanelWheelPolicy(DockPanelWheelPolicy, func(DockPanelWheelEvent)) error
}
type DockPanelWheelGestureValidator interface {
    ValidateDockPanelWheelGesture(session, revision, gestureID uint64) bool
}
type DockPanelWheelGestureAcknowledger interface {
    CompleteDockPanelWheelGesture(session, revision, gestureID uint64)
}
```

The Dock observer publishes a copied `DockInputTarget` after its worker has completed AX identity resolution. A bounded latest-value channel continuously supplies these targets to the input source; it is not a one-time policy snapshot. The source worker copies each update into a native cache protected by a short mutex; no Go pointer crosses into retained native state. A nil item clears the cache immediately, including on invalidation; channel closure cancels observation. Reconfiguration cancels and joins the old source before replacing its immutable policy. The tap accepts that cache only while its generation matches and `ObservedAt` is at most 150 ms old. The event-tap callback never calls AX, AppKit, Wails, or Go action code. It performs bounded rectangle/modifier checks, owns suppression synchronously, and copies the complete raw event above to a worker. The worker freshly resolves/revalidates the cached item before producing an action intent; every intent retains `Generation`. Only accepted owned gestures set `Owned=true`. Cancellation retires ownership even when its packet has `Owned=false`. Gesture IDs increase per new click or scroll stream; phase-less scrolling gets a new ID only after more than 250 ms idle, and every momentum event refreshes that timer. A fresh precise `began` may start a new stream immediately. The reducer never revives a cancelled or already-fired ID.

CG scroll phase and momentum phase are public event fields with different numeric encodings; map each enum explicitly. Direction inversion is a public `NSEvent` property, so retain/copy the raw CG event in the bounded native queue and read that property on the worker, never in the event-tap callback. Normalize delta sign once before the Go reducer; the retained inversion flag is diagnostic metadata and must not reverse it a second time.

`ObservedAt` is stamped before the observer begins its AX lookup, never when its result is dequeued. An ordinary pointer exit publishes a nil item with the same generation and Dock PID. This prevents new admission while preserving an already accepted click's original app. Generation zero explicitly invalidates the environment and cancels the source. Source replacement joins the old native lifetime and then resnapshots coalesced settings before starting another source.

Both native input sources retain at most one pending action token until acknowledgment. New gestures pass through while that token is pending; accepted work is never overwritten by a newer gesture. Normal finger/click completion preserves the token for asynchronous lookup and dispatch. The executor acknowledges success, refusal, and cancellation; terminal gestures that produced no intent are also acknowledged. A terminal packet must not acknowledge an action still being executed. Acknowledgment during an owned scroll preserves its momentum tail without another action. Explicit source/policy/session invalidation cancels the token immediately. Guards recheck application admission after external native validation as well as after other lookups. Native source errors and recovery are visible in Dock settings, including recovery after policy changes.

Pure preview recognition preserves retired gesture IDs across ended/cancelled packets, momentum, idle expiry, malformed samples and explicit reset. A delayed higher-sequence packet cannot revive the same gesture. Fresh ownership uses a newer native gesture ID.

Add these config fields under `DockSettings.Input`:

```go
type DockInputSettings struct {
    ClickToHide bool `json:"clickToHide"`
    ScrollShowHide bool `json:"scrollShowHide"`
    ModifiedRightClick bool `json:"modifiedRightClick"`
    SwipeTowardDock PointerAction `json:"swipeTowardDock"`
    SwipeAwayFromDock PointerAction `json:"swipeAwayFromDock"`
    SwipePrevious PointerAction `json:"swipePrevious"`
    SwipeNext PointerAction `json:"swipeNext"`
    PreviewDrag bool `json:"previewDrag"`
    AeroShakeAction string `json:"aeroShakeAction"` // none|minimizeOthers|closeOthers
}
```

Defaults are all disabled/`none`. Reuse the retained `PointerAction` validation set (`none`, `close`, `minimize`, `fullscreen`, `hide`, `quit`) for preview swipes. Aero Shake accepts only its three values.

## Task 1: Config and settings surface

**Files:** modify `apps/desktop/internal/config/dock.go`, `config.go`, config tests, `frontend/src/lib/types.ts`, `settings/tabs/DockTab.tsx`, `Settings.test.tsx`, `lib/i18n.ts`, and `e2e/settings.spec.ts`.

**Produces:** normalized, deep-copied `Dock.Input` and localized controls. Existing schema-3 documents with no `input` object load disabled defaults; explicit false/none values survive save/reload.

- [x] Add failing Go tests for missing input defaults, invalid actions, deep-copy behavior, and explicit disabled round-trip.
- [x] Add `DockInputSettings`, normalization, validation, and schema migration without changing existing Dock defaults.
- [x] Add failing React tests proving independent D09–D14 edits preserve Dock scope/appearance and global/window/app-switcher preferences.
- [x] Render an “Input and gestures” Dock settings card with the three icon toggles, four edge-relative swipe selectors, preview-drag toggle, and Aero Shake selector. Explain that precise trackpad scrolling is used for swipes and hardware finger count is not exposed reliably.
- [x] Run `go test ./internal/config` and focused Settings Vitest/Playwright tests.

## Task 2: Dock icon input source and pure reducer (D09–D11)

**Files:** create `internal/dock/input.go`, `input_test.go`, `internal/platform/dock_input.go`, `darwin_dock_input.{go,m,h}`, `dock_input_stub.go`; modify `internal/dock/types.go`, `controller.go`, `app.go`, and `app_dock.go`.

**Consumes:** `DockInputPolicy`, exact `DockItem`, existing `actions.Service`, `ApplicationActivator`, and Dock generation/session suspension.

**Produces:** `dock.Intent{Generation, GestureID uint64; Kind string; AppID domain.AppID}` with kinds `hide`, `show`, `quit`, `forceQuit`. Generation rejects stale Dock identities; gesture ID rejects work queued after native cancellation.

- [x] Write table tests for enabled/disabled policy, stale generation, self/invalid PID, click down/up ownership, modifier exactness, scroll sign, direction reversal, 250 ms idle rearm, phase cancellation, and one action across momentum.
- [x] Implement `inputReducer.Step(at, DockInputEvent) []Intent`. Accumulate precise deltas to 80 points and coarse wheel deltas to one notch; discard momentum after the first action and refresh the idle timer on every event. Map positive visual scroll toward “show” and negative toward “hide” after applying `isDirectionInvertedFromDevice` natively.
- [x] Have the existing Dock observer worker publish `DockInputTarget{Generation, ObservedAt, Item}` after AX lookup. Implement a dedicated session event tap for left/right down/up, movement, and scroll wheel. In the callback, accept only a point inside the fresh cached item rectangle; enqueue the copied event for a worker that re-runs the existing Dock identity lookup before intent emission. Do not AX hit-test from the event tap and do not reuse the hotkey tap.
- [x] Use listen-only mode when no suppressing feature is enabled; preferably stop the tap entirely when every D09–D11 toggle is false. Return the original event for misses, stale identities, unsupported modifiers, disabled actions, synthetic events, and tap-disabled notifications. Re-enable after `kCGEventTapDisabledByTimeout`/user-input disable and reset gesture ownership.
- [x] Suppress only a qualified D09 left-click sequence or D11 right-click sequence whose down began in a fresh cached app rectangle. Use the shared 6-point click slop. Retain the original suppressed down event while deciding whether the gesture is a click. If movement, modifiers, identity invalidation, or an outside up abandons click ownership, repost that original down with `CGEventTapPostEvent` from the callback before returning the current original drag/up; otherwise forwarding only the later event would break ordinary Dock dragging. Mark replayed events for pass-through and never synthesize an up or a completed click. Teardown during an owned down must similarly release the original down before the source exits, with its replay marker, so a later physical up cannot arrive without its down. Exercise callback ordering and cancellation with a disposable native receiver. D10 suppresses only a qualified scroll gesture over a fresh cached actionable app icon. Never suppress Finder/Desktop, separators, folders, Trash, minimized-window Dock items, expired caches, or ambiguous identities.
- [x] Revalidate cache generation, rectangle, PID, bundle/path identity, and actionability on the worker immediately before emitting. Carry the accepted generation into the intent; the Dock controller drops it after Dock restart or observer generation change.
- [x] Keep synchronous ownership and asynchronous intent recognition distinct in code/tests: Go cannot retroactively suppress a returned event. Native tests assert exactly which original events return `NULL`; reducer tests assert which owned `GestureID` produces an intent and that cancelled IDs cannot act.
- [x] Execute intents outside the tap: D09 calls exact `hide`; D10 show calls `ApplicationActivator.ActivateApp(appID)` and hide calls exact `hide`; D11 calls exact `quit`/`forceQuit`. Surface refusal through `dock:error` without activating Option Tab.
- [ ] Add a disposable native fixture proof for pass-through when disabled, one qualified event per gesture, modifier mapping, timeout recovery, and exact PID. Synthetic timeout/gesture injection proves wiring only; retain a distinct physical-device acceptance run. Do not click or quit real user apps.

## Task 3: Edge-relative preview swipe recognition (D12)

**Files:** create `internal/dock/panel_gesture.go`, `panel_gesture_test.go`; modify `internal/platform/dock_panel.go`, `darwin_dock_panel.{go,m,h}`, `DockPanelView.tsx`, `dock.css`, `App.tsx`, `lib/dock-bridge.ts`, `app_dock.go`, and focused Dock tests/e2e fake.

**Produces:** `PerformDockGesture(session, action, windowID, appID)` using the same session/identity guard as `PerformDockAction`.

- [x] Write pure tests for physical delta to semantic direction on `bottom`, `left`, and `right`: toward Dock, away from Dock, previous, next. Reversing Dock edge must reverse only the perpendicular mapping.
- [x] Write tests for 80-point accumulation, dominant-axis lock, direction reversal reset, phase ended/cancelled, 250 ms legacy-wheel idle rearm, and momentum ignored after one action.
- [x] Extend the native Dock panel host to observe `NSEventTypeScrollWheel` before WKWebView delivery and emit scoped `DockPanelWheelEvent` values with AppKit's precise flag, gesture phase, momentum phase, inversion flag, timestamp, and panel-local point. JavaScript `WheelEvent` is not the source of truth because it does not portably expose phase/momentum/precision.
- [x] When all preview swipe actions are `none`, deliver every scroll event unchanged to WKWebView. When enabled, begin ownership only for a precise `phase=began/changed` stream whose start point maps to a frontend-published `PreviewRegion{Session, WindowID, AppID, Bounds}`. Suppress only that owned stream; coarse wheel and misses pass through for panel scrolling.
- [x] Dispatch the configured action against the immutable preview region captured at gesture begin. A gesture never follows a later hover selection. Worker/App session and exact ownership checks still run before action.
- [x] Cancellation, panel hide, and region revision change invalidate ownership. Normal finger completion preserves a pending action token until acknowledgment; momentum cannot refire it. A native Go worker delivers scoped wheel events directly to the pure Go reducer; the frontend publishes geometry through `SetDockPreviewRegions` and does not recognize a duplicate JavaScript wheel stream.
- [x] Use Go reducer/App tests for every Dock edge, small-delta accumulation, momentum, cancellation, exact target and visible refusal. Add Chromium regressions for clipped DOM region geometry and the generated region RPC. These prove reducer/transport behavior only. Native and physical-device tests separately prove NSEvent classification and disabled/coarse-scroll pass-through.

**Hardware limit:** Public `NSEvent` exposes precise deltas plus gesture and momentum phases, but it does not provide a reliable “exactly two fingers” count for every scroll stream. D12 means a precise trackpad-class swipe begun over a preview; settings/help text must not promise rejection of three/four-finger scroll gestures on every device/OS.

## Task 4: Exact preview drag capture and position-only movement (D13)

**Files:** create `internal/platform/preview_drag.go`, `darwin_preview_drag.{go,m,h}`, `preview_drag_stub.go`; modify `DockPanelView.tsx`, `dock.css`, `lib/dock-bridge.ts`, `App.tsx`, `app_dock.go`; add platform, App, Vitest, and Playwright tests plus disposable native fixture.

**Produces:** `BeginDockPreviewDrag(session, gestureID, windowID, appID, pointerX, pointerY, grabX, grabY) error` and `CancelDockPreviewDrag(session, gestureID)`; one explicitly owned drag process-wide. The frontend allocates a monotonically increasing nonzero gesture ID before the asynchronous begin call; App admits and retires the `(session, gestureID)` pair, and cancellation is idempotent even when it arrives before begin completes. Pass that ID into `PreviewDragRequest` and check context/owner cancellation immediately before native capture installation. `grabX/grabY` are clamped normalized coordinates within the preview, not an offset measured in the scaled thumbnail.

- [x] Add frontend tests for a visible drag affordance, threshold before capture, exact card identity, pointer cancel, Escape, panel hide, and no focus/action click after a drag.
- [x] On threshold, compute `grabX=(pointerX-previewLeft)/previewWidth` and `grabY=(pointerY-previewTop)/previewHeight`, clamp both to `0...1`, and call `BeginDockPreviewDrag`. App verifies the current Dock session and exact entry before transferring ownership to a process-wide drag owner. Buffer pointer-up/cancel while async native start is pending; if release wins, cancel the returned owner immediately so no drag remains stuck.
- [x] Native start revalidates PID ownership and reads the AX window's position and size. For each global cursor point, destination is `x=cursorX-grabX*windowWidth`, `y=cursorY-grabY*windowHeight`. This preserves the visual point grabbed across thumbnail scaling and avoids the source window's potentially distant original cursor offset.
- [x] Immediately before installing capture, verify the originating `GestureID` is still current and `CGEventSourceButtonState(kCGEventSourceStateCombinedSessionState, kCGMouseButtonLeft)` is still down. If pointer-up won the async race, fail closed without installing a tap.
- [x] A temporary active tap owns left-drag/up plus Escape only after start; it sends coalesced destinations to a serial AX worker. Ordinary hover dismissal may hide the preview panel but must not cancel an off-panel owned drag. The explicit owner ends only on mouse-up, Escape, feature disable, session inactivity, app shutdown, target loss, screen reconfiguration, or tap failure.
- [x] Read position, size, and fullscreen state only to validate identity and calculate normalized destination. Set only `kAXPositionAttribute`. Reject non-finite coordinates, refused/non-settable position, identity change, minimized/disappeared window, second concurrent drag, and Option Tab's own windows. Never write size/fullscreen/Space attributes.
- [x] On mouse-up, explicit cancellation, inactivity/shutdown, screen reconfiguration, or tap disable, remove the run-loop source, release the Mach port/AX objects, restore pass-through, and complete once. Panel hide or normal Dock hover-session retirement only detaches the UI and does not end drag ownership. Never synthesize a click.
- [ ] Add pure lifecycle tests and a disposable native fixture proving exact window displacement, unchanged size, cancellation cleanup, off-panel continuation, and ordinary drag pass-through while disabled.

Apple defines `kAXPositionAttribute` as the accessibility object's global top-left screen coordinate. Keep the project's existing global top-left coordinate convention rather than mixing AppKit bottom-left coordinates.

## Task 5: Passive Aero Shake and dialog-preserving bulk action (D14)

**Files:** create `internal/dock/shake.go`, `shake_test.go`, `internal/platform/window_drag.go`, `darwin_window_drag.{go,m,h}`, `window_drag_stub.go`; extend `internal/actions/service.go` and tests; modify `app.go` lifecycle wiring.

**Produces:** passive `WindowDragObservationSource.ObserveWindowDrags(context.Context, func(WindowDragEvent))`; `actions.Service.PerformOthers(kind string, keepWindowID, keepAppID)` returning the existing `actions.Result`.

- [x] Write reducer tests using timestamped global points plus AX window positions. The reducer does not arm until at least two samples prove the same AX root window moved consistently with the pointer. Then require at least four alternating horizontal window-position reversals, each at least 24 points, total window path at least 180 points, within 700 ms. Reset on stationary AX position, inconsistent pointer/window deltas, vertical-dominant movement, pauses over 250 ms, mouse-up, cancellation, identity change, session loss, or disabled settings; fire once per drag.
- [x] Implement a listen-only event tap for left down/drag/up that only copies timestamp and pointer samples to a worker. The callback never AX hit-tests and never returns `NULL`. On the worker, resolve the root AX window for the initial point, then sample that exact window's `kAXPositionAttribute` during drag. Emit `moved` only when AX position actually changes in the same direction and within tolerance of pointer movement. Text selection and content dragging leave window position stationary and can never arm Aero Shake.
- [x] Extend native action-window classification to return role/subrole and attached-sheet/modal relationships. `PerformOthers` snapshots fresh roots, excludes the kept window, all Option Tab windows, `AXDialog`, `AXSheet`, system dialogs/popovers, nonstandard roots, unresolved identities, and any normal parent that owns an attached modal/sheet, then performs `setMinimized` or `close` per eligible window. Unknown role/relationship lookup is an explicit failure and is never treated as safe eligibility. Continue after individual action failures and report them in `actions.Result`.
- [x] Before execution, verify the kept window still belongs to the captured PID, remains a root window, and its latest AX position continues the observed movement. If this check is uncertain/refused, perform no bulk action.
- [x] Add tests proving cross-app normal windows are affected, kept window and dialogs survive, partial failures are reported, stale identity performs nothing, and disabled mode is passive.
- [ ] Add a disposable native fixture with two normal windows and a dialog. Prove content text selection does nothing, pointer-only synthetic drag does nothing, AX-confirmed window movement triggers once, and the dialog survives. Keep a manual hardware smoke command because synthetic CG drags do not validate physical-device feel.

## Follow-up admission details

- `BeginDockPreviewDrag` reserves its cancellable owner before external window lookup. Early release, configuration changes (including filters), inactivity and shutdown can cancel preparation. Ordinary hover dismissal detaches the UI without retiring an accepted native drag. Icon action guards retain the Dock configuration admission epoch and recheck it after external validation.
- `GetDockState.dragGestureFloor` lets a recreated webview allocate a newer gesture in the same logical Dock session. The floor includes accepted gestures and early cancellation tombstones.
- The native drag checks physical button state and display topology after installation, during polling and immediately before position writes. A missed owned mouse-up cancels without synthesizing a release. An already recorded final owned release is preserved.
- Native Aero Shake evidence retains the exact root through initial stationary drag-threshold samples. A discontinuity timestamp retires older action evidence; later correlated motion can restart the same physical drag's recognition without inventing a new gesture.
- The guarded other-window performer calls the Go admission guard after native target/role/button lookup, immediately before mutation. Guarded bulk execution cannot fall back to the unguarded performer. The callback runs without native locks and preserves the exact refusal error.
- A positively identified sheet/nonstandard surface is always preserved. Missing modal metadata on that already protected surface is not reported as a failed action. Standard-root candidates still require all relationship facts.
- Icon and shake sources report runtime permission/tap failure, join their native resources and retry through bounded controller lifetimes. Old-source errors cannot replace a new configuration's status. Icon, swipe, drag and shake failures have independent status slots, so unrelated recovery cannot erase an active failure.

## Task 6: Lifecycle, permissions, and acceptance gate

**Files:** modify App lifecycle/session tests, Dock controller tests, permission copy/i18n, native smoke README and scripts.

- [ ] Start icon input only when Dock is enabled and a D09–D11 feature is enabled. Start passive shake observation only when D14 is enabled. Cancel preview swipes with Dock hide. An owned preview drag survives ordinary hover hide and preferences/switcher opening; cancel it only for feature disable, pause, session inactivity, shutdown, target loss, screen reconfiguration, or explicit user cancellation.
- [ ] Test reconfiguration while every gesture phase is active. The old source must join before replacement; no callback from an old generation may act.
- [ ] Test Dock restart, target app exit, Space/display change, screen lock/wake, Accessibility denial/revocation, and tap timeout. Every failure resets ownership and leaves subsequent input passing through.
- [ ] Run `go test -race ./internal/dock ./internal/actions ./internal/platform`, frontend focused Vitest, focused Chromium tests, and the disposable native input/drag smoke suite.
- [ ] Manually verify one Magic Mouse, one notched wheel mouse, and one Force Touch trackpad. Record delta/phase traces and confirm the fixed thresholds produce one action without blocking ordinary disabled input. Hardware unavailable in CI remains an explicit release-check limitation.

## D15 separation

D15 needs a different source and lifecycle: screen topology plus native Dock window/AX position observation, an opt-in target display identifier, bypass modifier state, and disconnect fallback. It should reuse session and display-generation invalidation but must not share the D09–D14 active event tap or preview-drag capture. Plan and ship it independently after D09–D14 so monitor recovery cannot compromise input pass-through.

## API references

- Apple `CGEventTapCreate`: event masks, listen-only versus active-filter taps, permissions, run-loop ownership, and `NULL` creation failure: https://developer.apple.com/documentation/coregraphics/cgevent/tapcreate(tap:place:options:eventsofinterest:callback:userinfo:)
- Apple `CGEventTapPostEvent`: the local SDK `CoreGraphics/CGEvent.h` documents that a posted event enters immediately before the event returned by the tap callback; downstream taps receive it: https://developer.apple.com/documentation/coregraphics/cgevent/tappostevent(_:)
- Apple tap timeout event and re-enable API: https://developer.apple.com/documentation/coregraphics/cgeventtype/tapdisabledbytimeout and https://developer.apple.com/documentation/coregraphics/cgevent/tapenable(tap:enable:)
- Apple `NSEvent.phase`, precise scrolling deltas, and `momentumPhase`: https://developer.apple.com/documentation/appkit/nsevent/phase-swift.property and https://developer.apple.com/documentation/appkit/nsevent/momentumphase
- Apple `kAXPositionAttribute`: https://developer.apple.com/documentation/applicationservices/kaxpositionattribute

## Self-review

- D09: Task 2 click ownership and exact hide intent.
- D10: Task 2 signed accumulation, inversion, momentum, show/hide.
- D11: Task 2 exact modifiers and quit/force-quit.
- D12: Task 3 four edge-relative directions, precise-stream limitation, exact preview.
- D13: Task 4 off-panel capture, exact identity, position only, cleanup.
- D14: Task 5 passive shake, other-window actions, dialog preservation.
- Pass-through, cancellation, permissions, lifecycle, and hardware gates: Tasks 2–6.
- D15 is explicitly isolated with its required mechanism identified.

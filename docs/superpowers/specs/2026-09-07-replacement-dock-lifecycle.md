# H01/H02/H08 — Replacement Dock lifecycle proposal

Status: design proposal only, 2026-09-07. No production changes, native runtime probe, desktop mutation, new permission request, or physical acceptance was performed. Widget extensibility is a separate required design and is not covered here. These notes do not mark H01/H02/H08 complete.

## Decision and scope

Deliver an independently enabled application-launcher/Dock mode that preserves the native Dock as a recovery path. First implementation must leave the native Dock process, preferences, visibility policy, reveal gesture and system shortcuts untouched. “Replacement” describes the user's primary launcher surface; it must not imply that Option Tab has removed the system Dock.

Three mechanisms were considered:

1. **Own panels with coexistence and yielding (recommended).** Reversible, no system preference restoration transaction, crash cleanup reduces to releasing our windows. Works with either the user's existing visible or auto-hidden native Dock. Requires layout/reveal arbitration and honest UI copy.
2. **Application presentation options.** Not sufficient for this product: the installed AppKit SDK says `NSApplication.presentationOptions` applies while this app is active. Apple's reference describes these options principally for fullscreen/kiosk apps. Keeping Option Tab active to force global suppression would violate nonactivation and app switching behavior. Do not use hideDock, DisableProcessSwitching, or DisableForceQuit.
3. **Persistent native preferences, killing/restarting Dock, private suppression or global event interception.** No supported, independently recoverable mechanism is established in this repository. Do not implement as fallback. A future separately approved capability would need a crash-safe restore design, external-change preservation and positive native proof before exposing it.

Primary evidence: [Apple presentation options](https://developer.apple.com/documentation/appkit/nsapplication/presentationoptions-swift.struct), and local macOS SDK `AppKit.framework/Headers/NSApplication.h` lines 327–329. The header limits effects to the active application; this is not a tested global suppression API.

Existing D15 `DockPlacementAvailable()` remains false and is irrelevant to first H delivery. The failed/partial experimental placement evidence must not become an automatic migration prerequisite.

## Recovery invariant

At every point, either the native Dock's configured access remains unmodified or all Option Tab replacement surfaces are hidden. Disabling replacement, pausing, losing the interactive session, an unclassified environment transition, or terminal shutdown immediately retires interaction admission before scheduling native hide/close. No restore operation may move the pointer, move a user window, activate another app, rewrite native Dock preferences or restart Dock.

A permanent Option Tab menu-bar command “Use native Dock” disables this runtime and persists replacement enabled=false through the normal settings save path. First retire runtime, then persist; a failed save remains visibly disabled for the process lifetime and reports the error. The existing keyboard switcher and system app switching remain available. No recovery shortcut may intercept an existing system Dock shortcut. Crash/relaunch leaves native preferences untouched; default launch waits for a valid interactive session and topology before showing anything.

Suspend active native icon-action interception and D15 movement protection while replacement mode owns a visible/reveal-capable surface. Preserve their saved settings; restart only after the replacement lifetime joins. This prevents an old native input owner from consuming the very gesture intended to reach the system Dock. Existing keyboard switching and explicitly pinned media panels remain independent. Native hover/folder/media previews yield to the replacement presentation while it owns the pointer, without inventing native DockItems.

Do not position an invisible mouse-catching strip across the native Dock's reveal edge. Our hidden state has no input-accepting panel. Use read-only pointer observation to reveal our panel at an inward activation band; reserve the physical edge for macOS. When the native Dock appears, or the pointer enters its protected access corridor, hide our conflicting surface. If Dock visibility/edge/identity is uncertain, use conservative yielding near all potentially affected edges; if safe separation cannot be established, keep the affected panel unavailable. Do not claim a finite inset alone proves recovery against all native Dock sizes.

## Controller and resource ownership

One application-lifetime replacement controller owns configuration, session admission and topology. One presentation owner per resolved logical display owns its native host, profile revision, visibility lease, pointer/reveal state and subordinate preview/widget subscriptions. Persisted display UUID is the selection identity; CG display IDs and coordinates are runtime observations.

State sequence: disabled → preparing → hidden/visible → suspended or disconnected → preparing after fresh validation. Closing is terminal for a host incarnation. Every configure, pause, session, topology or profile selection transition increments a durable controller admission epoch immediately, even if a bounded mailbox coalesces disable→enable. Native callbacks and UI commands carry controller epoch, display UUID, presentation session, revision, host incarnation and visibility epoch; stale callbacks cannot recreate, move, reveal or act on a replacement owner.

Keep the established nonblocking, per-command-kind mailbox pattern. Native creation/show/hide/close remains scheduled on AppKit by the existing dockWindow/liveWindow machinery; no Go owner lock across synchronous AppKit dispatch or user callback. Host close notification marks the exact live resource dead before asynchronous owner retirement. Native context cancellation joins worker/timer/notification resources; no callback escapes a completed observer lifetime. Shutdown has a permanent terminal guard and never waits for workers while holding App.viewMu or blocking AppKit's reply/close drain.

Display disconnect retires and closes only that UUID's owner, preserving its saved binding/profile. Do not silently transplant a UUID-bound Dock onto another screen. Other valid owners continue. A dynamic “main display” binding may resolve anew after topology validation; collision with an explicit binding resolves deterministically, with explicit UUID priority and visible duplicate-binding status. Mirrored physical screens represent one logical desktop surface: deduplicate or explicitly refuse uncertain mirror inventory, never stack duplicate panels. UUID/name lookup failure is unavailable, not disconnected.

On session resume create fresh owners from fresh topology and access evidence; never revive old capture/gesture/profile sessions. Failure in one display host does not churn healthy display owners. Bound retries (e.g. one second) and deduplicate repeated errors while retaining a recovery notification.

## Profiles and geometry

New independent `ReplacementDockSettings`, enabled=false by default, schema versioned. Do not overload current `DockSettings`, whose meaning is native Dock enhancement. Suggested bounded first schema: up to 16 profiles and 16 display bindings, stable profile IDs, one effective profile per logical display, explicit edge/layout/size/alignment/auto-hide fields. Import validation belongs to H17; no implicit profile file execution.

Store edge, floating/fullWidth layout, normalized along-edge alignment, thickness, maximum length fraction, inset and auto-hide timing. Do not persist absolute desktop X/Y or platform IDs. Resolve in global top-left logical points from the display's UUID, full frame, visible frame, scale and safe-area data. Convert to pixels only for raster resources. Never feed measured DOM size into a repeated auto-size calculation that expands its own viewport.

Geometry is a pure function of validated profile + one topology generation + native Dock access regions. Full-width means the usable span on that display, excluding reserved access and safe areas; floating uses bounded content length within that span. Top layouts require menu-bar/notch exclusions, not just a screen rectangle. Use [NSScreen safe-area and auxiliary areas](https://developer.apple.com/documentation/AppKit/NSScreen/auxiliaryTopLeftArea-uglc) when supported, with explicit compatibility handling.

H08 overlap avoidance moves/hides only Option Tab panels. It must not set AX positions/sizes, change application visible work areas, or promise that arbitrary apps reserve space for this Dock. No public desktop-strut API is established here. Prefer yield/auto-hide when native Dock, fullscreen content, system UI or another owned transient surface conflicts. Fullscreen exposure is a deliberate policy; the existing preview host's `CanJoinAllSpaces | FullScreenAuxiliary` flags are not automatically the right replacement policy.

Auto-hide has shown/arming/hiding/hidden states and monotonic deadlines. Defer hiding while an owned click, item drag, menu or child preview is active; cancellation must release that hold. Bound stale holds. Pointer state must originate outside inactive WebKit, following the existing native pointer-to-Go-to-DOM transport lesson; do not depend on inactive-window CSS :hover. Read-only observation does not consume the native edge gesture.

## Minimal proposed platform boundary

Keep platform ports typed and small; names below are proposals, not frozen code:

- `ObserveDockEnvironment(ctx, emit func(DockEnvironment)) error`: read-only, cancellable/joined. Reports generation, timestamp, complete logical display inventory (UUID/ID/name/frame/visible frame/scale/safe regions/mirror group), pointer state, native Dock PID/start identity/container bounds/edge/visibility confidence, and explicit unavailable reason. Share existing native bounded AX/display helpers only after factoring away D15's active movement tap. Never invoke ObserveDockMonitorLock merely to obtain observations.
- Reuse `DockPanelHost` for dedicated Wails content and `DockPanel` Show/Hide/Close. Add a narrow replacement-specific creation policy if collection behavior/level truly differs; do not mutate media/preview hosts globally. Events require the existing exact resource incarnation and visibility lease. No generic arbitrary native-window setter.
- `ReplacementDockHostCapabilities` is read-only and explicit about supported Spaces/fullscreen behavior and pointer observation. `NativeDockSuppressionAvailable=false` until separately proven; no `HideNativeDock` production method in the first port.
- App controller owns the “restore access” operation by terminally retiring/hiding its own presentations and input policies. If native environment is unavailable, report `nativeAccessUnverified`; do not turn successful own-host Close into a claim that the native Dock was visually verified.

The pure controller exposes Configure, Suspend, Run, Snapshot and exact-session UI commands. It depends on an environment source and host factory, not AppKit, config disk I/O, or window-action internals. Profile routing and item/widget actions can attach later through narrowly scoped, guarded ports; they do not own lifecycle admission.

## Validation gates before enabling the mode

Pure/seam tests:

- Defaults/migration leave mode off; invalid profiles/nonfinite sizes/unknown edges/duplicate bindings refuse. Negative coordinates, portrait/scale changes, main reassignment, notch and unusable spans are deterministic.
- Configure false→true while observer join is blocked cannot admit old callbacks or create two owners. Rapid session inactive→active still retires all previous gestures/frames/actions. Shutdown then late resume never reopens.
- Disconnect one of two display UUIDs closes exactly one host; reconnect recreates a fresh session. No fallback to a similarly named display. Mirror and incomplete inventory report honestly.
- Native Dock reveal/unknown-state/corridor entry yields the competing panel; hidden panels intercept no input. Monitor-lock/icon input cancellation joins before recovery access is presented.
- Delayed host-create/show, old host close, stale topology, old profile action and frame completion cannot mutate replacement sessions. Own-host failure leaves native policies untouched; serialization never holds owner locks across View.
- Kill/failed-start/cancel harness proves no native Dock preference write/process restart/event suppression API was called. Tests should assert prohibited side effects, not fake a successful restore.

Native acceptance remains separate and explicitly authorized: disposable host show/hide/close with unchanged foreground; native Dock reveal access while replacement is visible and hidden; process crash and relaunch; real disconnect/reconnect, mirrored arrangements, mixed scale/Spaces/fullscreen/Mission Control; physical auto-hide and recovery path. The existing generic panel proofs help establish host mechanics but do not establish these H lifecycle claims.

## Decisions required before H implementation

Approve coexistence/yield as the initial meaning of optional replacement; keep global suppression unavailable. Freeze display/profile schema and default edge/layout. Decide supported Spaces/fullscreen behavior after native feasibility evidence. Approve the independent widget-capability design separately. No H implementation should begin by silently adopting preference mutation, cursor transport or global input suppression as a substitute for those decisions.

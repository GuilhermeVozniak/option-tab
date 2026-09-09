# H01/H02/H08 first vertical slice

Approved implementation plan after the G checkpoint (`c04206c`); native acceptance remains separate. Inputs are the tracked replacement Dock lifecycle and widget specifications. Root approved coexistence/yielding and declarative capability grants within the retained scope.

## Deliverable and limits

An opt-in, nonactivating primary launcher showing running applications (including windowless apps) on selected logical displays, plus one built-in declarative clock widget. The native Dock remains unchanged and reachable. No preference writes, process restart, suppression, cursor movement, global input interception, app-window repositioning, arbitrary widget code, package downloader or installer. This is partial H01/H02/H08, not completion of every edge/layout/profile/auto-hide scenario. Pinned apps/files, new visual modes and other widget providers remain later slices under the accepted retained scope.

## Frozen candidate configuration

Add an independent `replacementDock` settings field; existing `dock` retains its native-Dock enhancement meaning. No schema-version bump that discards old settings.

```go
type ReplacementDockSettings struct {
    Version int                 // 1
    Enabled bool                // false
    Profiles []LauncherProfile  // 1..8
    Bindings []LauncherBinding  // 0..8; empty => no surfaces
}
type LauncherProfile struct {
    ID, Name string             // bounded stable ASCII ID; escaped display name
    Edge, Layout string         // v1 accepts only "bottom", "floating"
    IconPx, ThicknessPx int      // 24..64, 48..112 logical points
    MaxLengthFraction float64   // finite 0.25..0.9
    InsetPx int                 // 12..64, additional to protected areas
    AutoHide bool               // false by default
    Widgets []WidgetInstance    // 0..4
}
type LauncherBinding struct {
    ID, Target, DisplayUUID, ProfileID string // target main|display; UUID only for display
}
type WidgetInstance struct {
    ID, PackageID, Digest string // v1 compiled-in org.optiontab.clock + fixed digest
    Enabled bool                // false until explicit enable/grant
    Grants []string             // v1 only clock.read; exact package digest + instance
}
```

Default profile: `default`, bottom/floating, IconPx40, ThicknessPx64, MaxLengthFraction0.8, InsetPx16, AutoHide=false; binding `main` references it. Top-level Enabled=false. IDs use `[a-z0-9][a-z0-9._-]{0,63}`; names are1..80 Unicode characters; lists are bounded before allocation. Reject nonfinite values, unknown versions/edges/layouts/capabilities, duplicate IDs, missing profile references and malformed UUIDs. Resolve a main binding at runtime; an explicit UUID binding wins a same-display collision and the other reports conflict. Preserve disconnected bindings. No persisted coordinates, CG IDs, session handles, process IDs, widget data or imported executables. Enabling the launcher does not grant widgets capabilities.

## Ports and pure ownership

New package `internal/launcher` owns configuration validation, profile resolution, pure geometry and lifecycle. It depends on typed snapshots and a View; no AppKit/disk/network. Proposed API: `New(Deps)`, `Configure(settings) error`, `Suspend(bool)`, `Run(ctx) error`, `Snapshot() State`, `Activate(scope Scope, itemID string) error`. `Configure`/`Suspend` retire admission synchronously before posting bounded work. `Scope` = controllerEpoch, displayUUID, session, revision; the App wrapper additionally owns hostIncarnation/visibilityEpoch. Every snapshot/entry is copied.

One controller owns a cancellable environment source; one presentation per resolved display owns its session and View lifecycle. Cancel/join a retired source before replacement; coalescing cannot erase disable→enable/session invalidation. Retry failures at a bounded one-second interval. Complete inventories may disconnect a UUID; failed/incomplete inventories suspend affected presentations and cannot declare disconnect. Host failure affects only that display. Closing/shutdown is terminal. Never hold a controller/App lock across View, native dispatch, action preparation or callbacks.

New platform port, separate from D15 protection and every active input source:

```go
type LauncherEnvironmentSource interface {
    ObserveLauncherEnvironment(context.Context, func(LauncherEnvironment)) error
}
type LauncherEnvironment struct {
    Generation, Sequence uint64
    ObservedAt time.Time
    Complete bool
    Status, Reason, WorkspaceStatus string // fixed enum/code vocabulary; no raw errors
    Displays []LauncherDisplay
    PointerX, PointerY float64
    NativeDock LauncherNativeDock
}
type LauncherDisplay struct {
    UUID, MirrorGroup string
    ID domain.ScreenID
    Main bool
    Frame, UsableFrame domain.Bounds
    Scale float64
}
type LauncherNativeDock struct {
    Process ProcessIdentity
    Edge, Visibility, Confidence string // bottom|left|right|unknown; visible|hidden|unknown; known|unknown
    Bounds domain.Bounds
}
type LauncherPanelHost interface {
    CreateLauncherPanel(unsafe.Pointer) (DockPanel, error)
}
type LauncherAppActivator interface {
    ActivateLauncherApp(context.Context, ProcessIdentity, func() error) error
}
```

Environment uses public read-only NSScreen frame/visibleFrame/UUID helpers, current-pointer observation and existing bounded Dock AX container helpers. Factor only observation helpers away from `darwin_dock_lock.*`; never call `ObserveDockMonitorLock`, install its tap or its placement path. Emit complete display inventories separately from explicit unavailable states. Pointer sampling ≤20Hz while enabled, expensive Dock/topology refresh coalesced ≤2Hz plus notifications. Context cancellation joins timers/observers; callback work only copies values. Mirror groups must be positively deduplicated, otherwise those surfaces are unavailable. Do not invent workspace classification: unknown yields; native feasibility must establish ordinary/fullscreen handling before enablement acceptance.

Reuse existing `ApplicationSource` inventory and `AutomationIdentitySource.ProcessIdentity` for current running apps, with opaque item IDs bound to PID/start/bundle identity. The new narrow activator must recheck exact running-process identity and final Go scope after native preparation, immediately before activation. Do not use PID-only `ApplicationActivator` for queued launcher clicks. No launch by arbitrary path or window action in this slice. Cache only ordinary app icons; zero new capture sessions.

Create dedicated hidden Wails hosts through existing `dockWindow` scheduling/lifetime patterns, with a replacement-specific native policy: nonactivating, no drag gesture, no global wheel hook, and no fullscreen-auxiliary admission. Do not globally change preview/media panel behavior. Native close callbacks retire only the exact host. Renderer feedback cannot revive a hidden incarnation. Existing panel proof supports mechanisms, not replacement Spaces/recovery acceptance.

## Geometry and recovery

`Layout(profile, display, itemCount, protectedAreas) -> {bounds, revealBand, status}` is pure global top-left logical-point arithmetic. Length is derived once from bounded item count and constants; overflow scrolls within that width. Height does not grow from DOM feedback. Clamp into UsableFrame and reject nonfinite/empty geometry. The native Dock's known bounds expanded by12px and a32px physical-edge corridor are protected; additional profile inset applies afterward. The corridor is a conservative implementation policy, not proof that all Dock configurations are safe. If a bottom floating rectangle cannot be separated from protected areas, hide it.

Native Dock reveal, pointer entry into its protected corridor, unknown Dock geometry/identity, fullscreen/system transition or incomplete environment evidence retires interaction and hides affected surfaces. The native reveal edge has no invisible hit-catching window. With AutoHide=true, use read-only pointer presence in an inward8px band of the calculated launcher rectangle: show after250ms, hide after500ms outside; monotonic deadlines reset on scope changes. Hidden windows are ordered out, not transparent input surfaces. No pointer event is consumed to reveal. Actual recovery behavior requires physical acceptance.

“Use native Dock” is a permanent menu action: retire launcher and hide its hosts immediately, then persist Enabled=false. Failed persistence leaves a process-local disabled latch and an actionable error. Suspend native icon actions and D15 protection for the entire enabled launcher lifetime, retaining their settings; join those sources before revealing launcher hosts. Native hover/folder/media presentations yield while pointer is owned by launcher; keyboard switching and independently pinned media remain available. Disabling launcher restores only Option Tab source policy after its owners join; it performs no native Dock restoration mutation.

## Declarative widget boundary in this slice

Implement a small trusted manifest validator and renderer using only row/text nodes, maximum depth4/nodes16, literal text or direct `clock.time` field plus `shortTime` formatter. Built-in manifest is immutable and digest-bound. No HTML/CSS/JS/URLs, expression evaluator, Wails method names, package paths or generic provider map. The clock gets one shared host timer at most1Hz only while at least one visible, enabled instance has `clock.read`; hide/pause/revoke releases it. Grant revocation retires widget epochs synchronously; stale samples cannot reenter. Use the approved broader proposal as the extension boundary, but do not implement archive installation or pretend the other providers are already supported.

## Implementation order and meaningful tests

1. **Pure config/geometry**: add config/replacement.go plus launcher profile/layout tests. RED: disabled migration, duplicate/missing binding, negative display origin, fractional scale, narrow/portrait usable frame, native Dock overlap, count overflow and two bindings colliding. No golden screenshot as geometry proof.
2. **Controller and widget grants**: manual clock/fake source/View. RED: blocked join with disable→enable; old generation after reconnect; one of two monitors fails; immutable snapshots; shutdown plus late callbacks; current process replaced; revoked widget produces zero subscriptions/actions; multiple clocks share one timer. Failures do not revive prior owners.
3. **Read-only environment + exact host/activation ports**: native seams for snapshot serialization/unknown confidence, copied callback lifetime and final activation guard; reuse existing SDK helpers. Do not open user apps or manipulate Dock during tests. Record absent native capability honestly rather than accepting fabricated geometry.
4. **App/UI wiring**: new app_launcher.go and route `#/launcher`; narrow App/main/config integration. Dedicated hidden host factories; exact host-close envelope; recovery menu; settings profile/binding/clock-grant controls with EN/PT/ES. Browser tests assert running/windowless app choices, stale click rejection, escaped widget text, bounded scrolling and a stopped/revoked timer. Ordinary host clicks only; no replacement of global keyboard handlers.
5. **Separate native acceptance after root coordination**: disposable foreground fixture and launcher host on one then two displays; real click targets exact fixture app without first activating Option Tab; visible/auto-hidden native Dock remains reachable; disable/pause/host crash leaves native preferences/process unchanged. Mixed scale, disconnect/reconnect, fullscreen/Spaces and recovery remain explicitly pending until observed. No H completion claim from pure tests or the prior preview host smoke.

Deliver this vertical slice and its gates as the first H checkpoint; continue the richer profiles/widgets and retained H follow-ups afterward.

## Coordinator freeze — authoritative integration decisions

This section supersedes candidate API shapes above. Work in `.worktrees/replacement-dock`, branch `feat/replacement-dock`, starting from `c04206c`. Root owns git, generated bindings and App/main wiring. No release, installation, native Dock preference writes, cursor movement or user-app action during automated verification.

- `internal/platform/launcher.go` is the frozen shared native contract. Space evidence is **per display**, with ordinary/fullscreen/system/unknown and known/unsupported/unavailable/transition status. Main means CG's main display. `PointerKnown=false`, incomplete inventories, unknown Dock confidence and unknown Space type yield.
- Public APIs do not provide the required per-display Space classifier. A narrowly scoped optional **private read-only** SkyLight adapter is permitted, dynamically resolving symbols and validating metadata strictly. A read-only macOS 26.6.2 probe found two UUID-matched records with numeric Current Space id/type, both type 0 and corroborated by their Spaces entries. This establishes the observed ordinary route only. Nil/boolean/unknown values never become ordinary. Shared-Space or mirror arrangements without positive mapping stay unavailable. No new private mutation API is permitted.
- Native launcher panels use a separate policy, including transient Exposé behavior and no FullScreenAuxiliary. Every panel receives a unique native token and a display UUID. Final app activation requires the exact token to still identify a visible launcher host, the same ordinary display Space, exact PID/start/bundle identity and the final bounded Go guard. Public host flags alone are not fullscreen/Mission Control acceptance.
- Pure `launcher.Controller` exposes `New(Deps)`, `Configure(config.ReplacementDockSettings) error`, `Suspend(bool)`, `Run(context.Context) error`, `Snapshot() State`, `Activate(context.Context, Scope, string) error`, and `FailDisplay(Scope, string)`. `Scope` contains epoch, displayUUID, session and revision. `View` exposes `Publish(State)`. State contains a copied display/status inventory and presentations, each with scope, visibility, reason, profileID, fixed bounds, icon size, items and bounded declarative widget data. Items expose opaque IDs, app display names and icons; process identities remain backend-only.
- `Deps` takes Environment, Applications, Identities (only `ProcessIdentity(domain.AppID)` is required), View, Now, and an `Activate(context.Context, Scope, platform.LauncherAppTarget, func() error) error` function. App supplies that function, adding the current native panel token and its settings/host checks to the controller guard. Never hold either controller or App mutex across native activation, environment source joins, inventory work or View callbacks.
- Existing `ApplicationSource.Apps()` cannot be cancelled. Keep one serialized bounded-inventory worker, ignore its results after scope retirement, and join it before replacing that worker; do not claim context cancellation aborts an in-flight native query. No windows query or capture is required. Inventory errors make app items unavailable rather than retaining clickable stale identities.
- Root adds a narrow `RetireSource() <-chan struct{}` receipt to native Dock input and monitor-lock controllers. It immediately retires admission and closes only after the owning source epoch joins, including concurrent startup. Wait off-lock/off-main before allowing launcher visibility. Recovery disables the launcher immediately with a process-local latch, then persists false; a save failure cannot resurrect it.
- Config worker owns `config/replacement.go` and additive config.go hooks, including strict nested decode **before Normalize**, preserving explicitly empty bindings and rejecting invalid/future enabled authority. All snapshots copy nested grants/widgets/profiles. Root owns `app_settings.go` snapshot wiring.
- Renderer RPCs will be `GetLauncherState(session uint64)`, `GetLauncherStatus()`, `ActivateLauncherItem(epoch uint64, displayUUID string, session uint64, revision uint64, itemID string) error`, and `UseNativeDock() error`. Native route is `/#/launcher/<session>`. Root and UI worker freeze final DTOs after the pure state types land; no renderer calls arbitrary methods/providers or receives a native panel token.
- A built-in immutable clock manifest demonstrates digest-bound `clock.read`, a bounded row/text renderer, explicit grant/revoke and a shared timer only while visible and granted. Later providers/install/profile/item work remains retained H scope. No downloaded executable widget code.

The integration map and read-only probe evidence are in the ignored H ledger. Verify ordinary native hosts with disposable fixtures only after root coordination; the pending Dock restoration question does not authorize any new movement or preference action.

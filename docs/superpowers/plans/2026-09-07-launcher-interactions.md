# Launcher interactions: retained H05, H06 and H15

**Scope.** Add runtime item reorder/grouping, spring magnification, and opt-in launcher gestures/navigation. This plan does not add external file drops, arbitrary actions, global hotkeys, activation on hover, cursor warping, or new window-management behavior. All defaults remain off except ordinary clicking already shipped.

## Existing foundations

- `config.LauncherProfile` already owns per-profile geometry and items; item lists are bounded to 16 and groups may contain unique app items only (`apps/desktop/internal/config/replacement.go`, `replacement_items.go`).
- `launcher.Presentation.Scope` is the renderer authority: epoch, display UUID, session and revision. Items expose opaque IDs only (`apps/desktop/internal/launcher/types.go`).
- `LauncherItemStrip` already closes menus when item identity changes and has keyboard-accessible item buttons (`apps/desktop/frontend/src/launcher/LauncherItemStrip.tsx`).
- `platform.LauncherPanelValidator` validates the exact host and original display/Space. Native panels are nonactivating (`apps/desktop/internal/platform/launcher.go`, `darwin_launcher.go`).
- The native panel wheel source already copies precise deltas, phase/momentum, session/revision/gesture/sequence and retains a validation token until acknowledgement (`apps/desktop/internal/platform/dock_panel_wheel.go`, `darwin_dock_panel.m`). Reuse this ownership design, not its window-specific region DTO.
- Haptics already use `platform.HapticFeedback.HapticTick` and `NSHapticFeedbackManager` alignment feedback (`apps/desktop/internal/platform/platform.go`, `darwin.go`, `darwin.m`). Do not invent device flags or private feedback APIs.

## Additive configuration

Keep replacement config version 2 and add optional profile fields with strict validation and deep copies. H06 uses a separate optional `magnification` object:

```go
type LauncherMagnification struct {
    Enabled bool    // default false
    Scale float64   // 1.0..2.0, default 1.35
    Reach int       // 0..4 neighbours, default 2
}
```

Absent fields normalize to off/1.35/2; partial objects default only omitted properties. Scale and reach remain stored while disabled. Version 1 documents reject this later field. H05 adds its own optional runtime-reorder switch when implemented. Gesture, letter-navigation and haptic fields are added only with their runtime implementation. Settings explain that trackpad gesture availability is hardware/OS dependent; do not claim finger count.

## H05: exact runtime item mutations

Dragging starts only from an existing `.ot-launcher-app` drag handle after primary pointer movement crosses 6 CSS px. Decorations cannot be dragged. Internal item IDs use a private MIME type; external URL/file/text drops are ignored. The renderer shows before/after insertion and “add to group” targets. Escape, pointer cancel, host hide, item revision change, profile/display/session change, and unmount cancel the draft. Keyboard commands provide Move before/after, Add to group, Remove from group.

Runtime persistence must not call the Preferences-only `SetLauncherItems` bridge. Add one closed mutation endpoint:

```go
type LauncherItemMutation struct {
    Kind string // moveBefore|moveAfter|addToGroup|removeFromGroup
    ItemID, TargetID string
}
func (App) MutateLauncherItems(epoch uint64, displayUUID string,
    session, presentationRevision uint64, expectedItemsRevision string,
    mutation LauncherItemMutation) error
```

Add `itemsRevision` (SHA-256 of the canonical profile item list) to `Presentation`. App validates the exact live parent scope, current profile, expected item revision and opaque IDs, applies the mutation to a copied list, runs `ValidateReplacementDock`, then persists it under the existing `App.saveMu` through `saveSettingsLocked`. `saveMu` already serializes `SaveSettings`, item/package/profile imports and App read-modify-write operations; do not add another writer or queue. The Preferences-webview promise queue still cannot serialize a mutation from another Wails webview, so persistence also needs a settings-wide compare-and-swap revision. A stale revision returns a scoped visible error and the renderer discards its draft. No renderer-supplied item array, reference, path, icon or group membership becomes authority.

Keep a process-local monotonic `settingsRevision`, initialized nonzero after settings load and incremented with each successful `saveSettingsLocked` publication under `settingsMu` while the writer holds `saveMu`. Add revision-aware Preferences transport while retaining legacy methods only for compatibility:

```go
type SettingsState struct { Revision uint64; JSON string }
func (App) GetSettingsState() SettingsState
func (App) SaveSettingsAtRevision(json string, expectedRevision uint64) (SettingsState, error)
```

`GetSettingsState` snapshots revision and settings coherently under `settingsMu`; it must not wait for `saveMu` because a native menu callback could otherwise block the main thread while a writer needs that thread to apply settings. `SaveSettingsAtRevision` decodes strictly, locks `saveMu`, compares the expected revision, saves once, advances the revision and returns the canonical committed snapshot. Runtime `MutateLauncherItems` reads fresh settings only after acquiring `saveMu`, mutates that snapshot, then saves under the same lock. It therefore cannot overwrite a concurrent Preferences save. The revision need not be persisted across process restarts because no renderer survives that boundary.

`OpenPreferences` emits loading and copied revisioned settings events with a monotonic open generation, even when reusing an existing webview. `useSettingsModel` subscribes before its initial read, retires old queued work when a newer generation begins, and replaces its hidden stale model before enabling edits. A snapshot received before its matching loading event completes that generation; reordered or duplicate loading events cannot disable it again. Pending saves from older revisions are refused by App CAS and reload canonical settings instead of retrying stale full JSON. Recovery blocks edit admission synchronously and every delayed completion rechecks its owner. Successful runtime mutations may additionally emit the same revisioned snapshot, but no background frontend polling or second persistence queue is needed. Closing/reopening Preferences and a runtime mutation must never leave an old controlled form enabled against a newer App revision. A confirmed browser-only session retains local demo edits; failure to read a real backend keeps edits blocked.

Groups retain current config invariants: only app items, one group per member, no empty group, maximum 16 configured records including member records. Moving a group moves it atomically; moving a member within its group changes member order. Cross-container moves use explicit add/remove operations. Adding an app to a top-level app creates a new Group with a bounded collision-free profile-local ID; adding to an existing group transfers the source and deletes its old group if empty. Removing a member restores its existing record beside the group. Decorations may be move targets but cannot be drag sources. `itemsRevision` is a content hash and may return to A after A→B→A. Every effective mutation must still advance the launcher owner/presentation revision (or create a successor session), and action/drag admission checks that exact scope as well as the hash and the monotonic display-admission watermark. Thus a gesture admitted against the first A cannot act on the later A even though their content hashes match.

Tests: pure mutation table for every placement/group invariant; blocked settings write followed by profile/session retirement; runtime mutation followed by reopening a retained Preferences webview loads the new canonical revision before edits; a stale full Preferences save is refused and cannot overwrite runtime pins; concurrent Preferences edit and runtime mutation serialize under `saveMu` with exactly one stale loser; failed disk persistence does not advance `settingsRevision`; A→B→A content plus a delayed first-A drag is rejected by presentation scope; stale renderer mutation cannot affect successor; DOM pointer and keyboard paths emit one exact mutation; external drop emits none.

## H06: spring magnification and host geometry

The pure layout result must reserve the maximum transformed envelope before the host is shown. For icon size `I`, scale `S`, and reach `R`, reserve at least `ceil(I*(S-1)/2)` beyond both primary-axis ends, plus the pure spring layout’s maximum neighbour displacement across `R`, and enough cross-axis thickness for the scaled icon plus labels, border and padding. The resulting native host bounds must still fit the display usable frame with the existing 32 px native-Dock recovery inset. If it cannot fit, reduce scale toward 1.0 for that presentation and publish the resolved scale; never clip configured icons, cover the recovery target, or silently expand beyond the validated display.

Frontend computes target scale from pointer distance to item centers across `R` neighbours. Use a damped spring per item (position/velocity), transform only compositor properties, and keep hit testing on stable layout boxes. One `requestAnimationFrame` loop serves the strip and stops when settled, hidden, tombstoned, `document.visibilityState !== 'visible'`, or reduced motion is requested. `prefers-reduced-motion: reduce` resolves scale immediately to 1. High-refresh means following the browser display callback cadence; do not promise a numeric refresh rate.

Pointer leave returns all scales to 1 without activation. Reorder hit testing uses untransformed boxes so magnification cannot retarget a drop. Tests: deterministic fake-rAF spring settling/no overshoot bound; stop on hide/reduced-motion; all four edges at scale 2/reach 4 fit resolved native bounds and 32 px recovery; 120 Hz-capable fixture may verify no hard-coded 60 Hz step, while physical smoothness remains native acceptance.

## H15: scoped gestures, haptics and letter navigation

Add a launcher-specific native source rather than encoding launcher items as `PreviewRegion` window IDs:

```go
type LauncherGesturePolicy struct {
    Epoch, Session, Revision uint64
    DisplayUUID string
    Enabled bool
    Bounds domain.Bounds
    Edge string
}
type LauncherGestureEvent struct {
    Epoch, Session, Revision uint64
    DisplayUUID string
    Sequence, GestureID uint64
    Timestamp time.Time
    PanelX, PanelY, DeltaX, DeltaY, Magnification float64
    Phase, MomentumPhase, Kind string // scroll|magnify|swipe
    Precise, Owned bool
}
type LauncherGestureSource interface {
    SetLauncherGesturePolicy(LauncherGesturePolicy, func(LauncherGestureEvent)) error
}
```

Follow `DockPanelWheelSource` safety: AppKit callback only copies bounded event data and performs no controller/AX work; one exact host/session/revision token owns a gesture; policy change/hide/Space/topology invalidates it; worker validates before and after reduction; terminal/cancel is acknowledged; pass-through remains enabled when the feature is disabled or the event is unowned. Edge-relative mapping uses the launcher primary axis and toward/away semantics already documented for Dock gestures. Momentum cannot start a second action.

**Unknown native capability:** repository evidence covers `NSEvent` precise scroll phase/momentum extraction, but does not yet prove physical pinch or swipe delivery to a nonactivating `LauncherPanel`. Before exposing those controls as available, add a property-extraction fixture and coordinated physical trackpad acceptance. If delivery is unavailable, keep controls disabled with truthful status; do not infer pinch from wheel deltas.

Haptics fire once when the selected/reorder target changes and only when both profile Haptics and the corresponding interaction are enabled. Rate-limit to one alignment tick per 40 ms; never haptic on passive hover when every interaction toggle is off.

Letter navigation is opt-in and scoped to the visible panel. The implementation uses an explicit keyboard-mode button and a guarded native key-window policy, followed by committed WebKit text; it does not forward raw native key packets. Native hide, close, Escape or key resignation retires permission, and only another explicit request with a fresh admission can reacquire it. Physical nonactivating WebKit input delivery remains unverified, so production letter input stays unavailable until that acceptance passes. Accept unmodified printable Unicode letters, case-fold and cycle prefix matches among current actionable items; repeated same letter advances, 750 ms idle resets. Ignore modifiers, decorations, unavailable items and hidden/tombstoned sessions. Composition-end uses committed event data; paste, drop and cancelled composition cannot become letter commands. Enter used for IME candidate confirmation never activates the selection. Ordinary Enter activation is separately enabled and defaults off. No global event tap or shortcut registration.

Enabled letter navigation reserves 28 logical points on the primary axis before maximum-length and magnification fitting, containing a compact 24-point keyboard control. Geometry must retain one complete icon and the existing padding on all four edges or refuse the host. A single authoritative interaction-state event includes selection, keyboard mode, capabilities and any current error. Rendered and pending future-revision states share the newest admission/sequence watermark; delayed older packets cannot discard newer authority or its error.

Tests: pure edge-relative gesture reducer with reversal, momentum, terminal replay and session A→B→A; disabled policy passes through; haptic transition/rate tests; letter-cycle Unicode/current-item/revision tests; browser focus and reduced-motion tests. Native acceptance separately records actual panel-local wheel, optional pinch/swipe, exact-host cancellation, no activation, no cursor movement and disabled pass-through.

## Bounded implementation owners

1. **Pure/config/core owner:** schema/clone/validation, item mutation reducer/CAS revision, magnification envelope and gesture/letter reducers with race tests. No AppKit or React.
2. **Native/App owner:** launcher gesture/key source, exact owner lifecycle, haptic rate boundary, revision-aware use of the existing `saveMu`, runtime mutation and resolved host bounds. Native callbacks copy only; App workers call core outside native/App locks.
3. **Frontend owner:** settings, drag/group UI, spring renderer, local selection, scoped error/event bridges and Chromium geometry/input tests. Root alone freezes RPC DTOs, regenerates bindings and integrates shared App routing.

Local gates cover pure/unit/race, renderer build and Chromium. Native fixture gates cover extraction, ownership, cancellation and pass-through without user apps. Physical pinch/swipe, high-refresh visual quality, multi-display recovery geometry and haptic feel remain explicitly coordinated acceptance and stay unchecked until observed.

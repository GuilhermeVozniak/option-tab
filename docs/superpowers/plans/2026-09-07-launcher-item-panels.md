# Launcher Item Panels Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans task-by-task; coordinator owns shared contracts, bindings and integration. This document authorizes implementation, not uncoordinated native GUI acceptance.

**Goal:** Complete H09 folder list/grid fan-out, then H11 exact-app show-all, using child panels owned by current launcher parents.
**Architecture:** One independent child owner per launcher parent session, with its own native LauncherPanel, context, revisions and resources. The core supplies pure parent authority/geometry; App joins filesystem/capture work and reuses existing folder/window preparation and cards.
**Tech Stack:** Go, Wails/React, existing platform LauncherPanel/FolderSource/guarded actions, identity-bound preview peers.
**Spec:** `docs/superpowers/plans/2026-09-07-launcher-items.md` Task 3; retained H09/H11 in `docs/dockdoor-roadmap.md`; replacement Dock lifecycle spec.

**Implementation checkpoint:** Folder and window child panels are integrated with automated race/browser/build evidence in [the checkpoint report](../reports/2026-09-07-launcher-item-panels.md). Native acceptance remains separate. Review tightened predecessor joining, bounded Show admission, exact capture-start identities and physical guards; the report describes final behavior.

## Constraints and source-derived decisions

- No fake DockItem, public automation RPC delegation, private Exposé invocation, generic command, app launch on show-all, Dock preference/input changes or new permissions on observation.
- Launcher panel policy remains nonactivating and display/ordinary-Space bound. Parent and child native host incarnation matter independently. Native validation and all AppKit calls run outside App/controller/preview mutexes.
- Clock/widget-only parent revisions do not retire children. Changes to current parent session/epoch/profile, item/ref/process authority, settings admission, original display Space/topology generation, or native host retire them synchronously.
- Each child gets `platform.NewFolderSource(bookmarkPath)`: its `known`, `snapshots` and `revisions` maps are per-instance. The existing private bookmark file can be shared (`folder.BookmarkStore` serializes process writes); never share the platform default FolderSource with Dock or another child.
- Pinned access is retained through `LauncherReferenceSource.WithLauncherFolder`. Folder sources expose no Close; count/join their caller jobs. No chooser starts on list/retry. Missing/moved/revoked references offer explicit relink in Settings, using the existing preferences workflow.
- Existing FolderPanel expects `folderIdentity`, but native FolderRef.Identity is a file URL. Transport an opaque child identity instead; actual canonical FolderRef/path and source-native entry maps stay backend-only.
- `preview.Manager.Hide` and `Close` currently cancel jobs but do not wait for `run` exits. Add peer-only `CloseAndDrain() <-chan struct{}` before window children; do not describe cancellation as joining.
- Existing stream/snapshot channels already enforce a process budget of four live streams and one snapshot. New children share those channels, at most two live priorities each; never allocate independent budgets or evict other owners on hide.

## Frozen core protocol (coordinator-owned `internal/launcher/children.go`)

```go
type ChildParent struct {
    Scope Scope
    ProfileID, ItemID string
    Configured config.LauncherItem
    Reference platform.LauncherReference
    App platform.LauncherAppTarget
    Display platform.LauncherDisplay
    Bounds domain.Bounds
    Admission uint64
}
func (*Controller) CaptureChildParent(Scope, string) (ChildParent, error)
func (*Controller) ValidateChildParent(ChildParent) (Scope, error)
func (*Controller) PlaceChild(ChildParent, float64, float64) (domain.Bounds, error)
func (*Controller) SetChildBounds(ChildParent, uint64, domain.Bounds) error
func (*Controller) ClearChildBounds(ChildParent, uint64)
```

- Capture requires exact initial visible Scope and current eligible app/folder item, including an explicit app-group member. App target is exact PID/start/bundle; folder reference is exact current native live revision, never the private store CAS alone.
- Validate returns latest parent Scope only if captured authority remains current; ignore clock-only Scope.Revision. Include a monotonic per-display Admission retired immediately on accepted Space/topology A→B→A evidence, not merely after reconciliation.
- PlaceChild anchors toward display interior: bottom→above, top→below, left→right, right→left; clamp within current UsableFrame and 32px recovery inset, avoid parent/native Dock protected bounds. Use backend parent bounds, not renderer coordinates. Refuse if no candidate fits.
- Set/Clear match both captured parent admission and exact child session; an old clear cannot release a successor's hold. Pointer within child or bounded connecting corridor extends existing 500ms leave grace. Native Dock access/yield, incomplete environment and policy retirement always override the hold.

## App ownership and lifecycle

- New `app_launcher_item_panels.go` owns `map[parentSession]*launcherItemPanel`; at most eight, matching valid display bindings. A request replaces only that parent's child. Folder primary click opens fan-out; retain explicit “Open folder” action for the existing root-open RPC. App context “Show all windows” opens the window child; no action for stopped app.
- Child captures parent core record, exact `appLauncherHost` + scheduler/native token, current launcher-item manager/source incarnation, immutable item/reference identity, settings admission snapshot, and child-native host lease. One monotonic child session and content revision; no renderer-provided paths/tokens/process IDs.
- Register source lifetime under viewMu (`manager.wg.Add(1)` before releasing lock); retain through every WithLauncherFolder callback, outstanding action and child teardown. Consult manager.ctx.Err directly at final guards as well as child context; release WG only after all source calls return. Manager shutdown cancels children before waiting/closing reference sources.
- On replace/hide/settings/profile/Space/inactivity/pause/preferences/host-close/shutdown: first remove current child, cancel its context, clear atomic frame admission and emit terminal hide; clear only its core child hold. Then schedule native close and join workers/capture outside viewMu. Replacement may show a loading host after exact old host retirement, but cannot start replacement capture/filesystem work before its predecessor drain.
- Keep one serialized query worker and one bounded busy action slot per child; latest pending query/sort/resize wins. Replacement admission permits one active plus one accepted pending child per parent; further Show requests return busy without allocation until its predecessor receipt drains. Cap active children at eight and all retained active/retiring owners at sixteen across parent generations; capacity releases only after actual receipts join. Tag every result with child session, request sequence, owner incarnation and captured parent admission. A late result cannot update a replacement, even if item ID returns A→B→A.
- Native child route `#/launcher-item/<childSession>` uses a dedicated factory `(session, displayUUID, style, onClosed) *dockWindow` wrapping `CreateLauncherPanel`. CSS header/Close is ordinary web content; do not add media drag/wheel policy. Native close callback captures exact scheduler+window; old callbacks cannot retire a successor.
- App guard: copy/check logical child+parent under viewMu, unlock; ValidateChildParent; validate exact parent `LauncherPanelValidator` and current child validator (when shown); resolve/recheck captured native reference or window/process; rerun core/logical/source-context admission after external work. Never call native validators or reference resolution while viewMu is held.

## DTO and RPC freeze (new `app_launcher_item_panel_types.go`)

```go
type LauncherItemPanelState struct {
    Session, Revision, ParentEpoch, ParentSession uint64
    DisplayUUID, ProfileID, ItemID, Kind, Title string // kind folder|windows
    Open bool
    Bounds domain.Bounds
    Folder *LauncherItemFolderState
    Windows *AutomationPreviewViewState // shared data shape only; never automation owner/token
    Error string
}
type LauncherItemFolderState struct {
    FolderIdentity, Status, Reason, View string // opaque identity; view list|grid
    Entries []platform.FolderEntry
    Sort platform.FolderSort
    Partial bool
    Revision uint64
}
```

- All wire tags camelCase. Nested Windows Session/Revision equal child Session/Revision; token/automation runtime is absent. Use existing bounded frame snapshot fields. FolderIdentity is `launcher-child:<session>` and never changes to a file URL.
- `ShowLauncherItemPanel(epoch uint64, displayUUID string, parentSession, parentRevision uint64, itemID string) (LauncherItemPanelState,error)` derives folder/windows from validated item kind, reserves child and returns loading state after host admission; no action on a stopped app.
- `GetLauncherItemPanelState(session uint64) *LauncherItemPanelState`; `CloseLauncherItemPanel(session,revision uint64) error`; `SetLauncherItemPanelSize(session,revision uint64,width,height int) error`. Positive finite logical sizes <=960×720, clamp/refuse through PlaceChild; initial folder 420×360, windows 560×360, small-display fit may shrink to available area, minimum useful120×96 or refuse. No position RPC.
- `SetLauncherFolderSort(session,revision uint64,field,direction string,foldersFirst bool) error`; `SetLauncherFolderView(session,revision uint64,view string) error`; `OpenLauncherFolderEntry(session,revision uint64,entryID string) error`. Sort/view changes are child-local; initial sort reuses current Dock folder sort, initial view configured per pin; Dock folder Enabled does not gate launcher pins.
- `SelectLauncherWindow(session,revision uint64,windowID domain.WindowID) error`; `PerformLauncherWindowAction(session,revision uint64,kind string,windowID domain.WindowID,fullscreen bool) error`. Allowed kinds focus/close/minimize/hide/fullscreen only, exact currently rendered entry. No arbitrary app/window selector.
- Events: `launcher-item:update` full copied DTO; `launcher-item:hide {session,revision}` terminal; `launcher-item:frames {session,revision,sequence,frames}`. Parent clock ticks do not increment child Revision or clear child menus/errors. Any changed child entries/selection/layout/bounds/error increments child Revision; frame sequence is independent and monotonic.

## Task 1 — pure parent authority and peer drain

**Owner/files:** root `internal/launcher/children.go`, tests, narrow controller/reconcile hooks; independent capture owner `internal/preview/manager.go`/tests only.
- [ ] RED core: clock revision remains valid; item/ref/process change, Space/topology A→B→A, old Clear after replacement and recovery-edge pointer retire/refuse correctly. Test all edges/negative origins/narrow usable bounds and no native calls.
- [ ] Implement the frozen core protocol. Child hold contributes pointer ownership only for admitted bounds, not whole display, and never suppresses native recovery yield.
- [ ] RED drain: active stream blocked on cancel, queued semaphore job, bound thumbnail fallback and emit-in-progress. Receipt remains open until each exits; sibling owner survives and overall four/one budget holds.
- [ ] Implement CloseAndDrain: mark peer closed/cancel under Manager.mu; track every launched goroutine including canceled jobs removed from jobs map; close receipt only when they all exit, waiting outside mu. Existing Hide/Close behavior stays compatible. Permit new identity peers from an already-initialized shared budget outside viewMu without changing other live limits; first peer budget setup remains at App construction.

## Task 2 — folder child first

**Owner/files:** App owner new panel types/owner/folder files and tests, narrow launcher/item-manager lifecycle hooks and main factory; frontend owner new launcher child route/bridge/tests, narrow FolderPanel and folder CSS. Root owns bindings/App.tsx shared wiring.
- [ ] RED blocked ListFolder then parent retire/replace; relink during final entry guard; same folder simultaneously in Dock and two children; no source close before child job drain; opaque DTO identity; canceled queued host operation/old host-close callback.
- [ ] For each list/open, acquire WithLauncherFolder using captured reference revision and isolated FolderSource. Keep callback alive through native operation. Final entry guard fresh-resolves reference ID, requires same live revision/kind/ready state, revalidates parent/child physically, then reruns logical admission. Source still checks actual parent/child filesystem identity after that guard.
- [ ] Publish loading/ready/partial/empty/missing/access/error states; preserve existing 500-entry/500ms source limits. Retry sorts/refreshes only; it never calls a chooser. Reference repair opens existing Settings explicitly, retiring the child; FolderPanel accepts optional access label “Relink in Settings” while Dock's grant behavior is unchanged.
- [ ] Add optional `view="list"|"grid"` FolderPanel rendering with existing handlers/opaque IDs. Grid is bounded responsive columns with text/size hidden only visually as appropriate; keyboard opens same entry. Measure complete host content including header/padding, debounce size updates and stop viewport feedback loops.
- [ ] Focused Go race + FolderPanel/route tests: stale errors and GetState after hide, revisioned sort/layout, no direct paths or window APIs, zero capture calls in folder mode. Deliver usable folder child before window composition.

## Task 3 — exact-app window child

**Owner/files:** App owner new `app_launcher_item_windows.go`/tests, narrow private preview helper extraction; capture owner owns peer API; frontend reuses existing window cards and adds show-all context bridge. Do not refactor unrelated automation lifecycle.
- [ ] RED blocked fresh WindowSource/identity lookup then parent hide/ref change; failed/incomplete inventory never silently resolves a partial app; stopped/reused process refuses; close one child leaves automation/Dock/sibling captures intact.
- [ ] Reuse private prepareAutomationPreview with captured process and copied settings/global window filters; rename/extract only a pure preparation helper if necessary. Capture appearance follows existing automation window-preview appearance; folder mode never creates a peer. Child empty state is explicit, no launch or first-inventory fallback.
- [ ] Create independent `NewPeerWithIdentity` sharing the established budget after predecessor drain. Immutable atomic lease contains child session/revision/exact captured identities; callback never takes viewMu or reads manager cache. Retire lease before CloseAndDrain. Cap capture candidates30, frame<=512KiB encoded and catch-up snapshot<=4MiB; only current visible IDs.
- [ ] Guarded actions call `actions.Service.PerformAutomationWindowAction`; repeat parent/child and reference/process admission after external preparation via final guard. Minimize sets true; fullscreen is explicit bool. Refresh only the still-current child after success, publish scoped error after refusal; never retarget a replacement.
- [ ] Frontend frame high-water/tombstone tests and concurrent owner race tests; no public automation RPC calls, no automatic image query/capture when appearance policy disables it.

## Integration gate and pending native acceptance

- [ ] Coordinator freezes signatures once, regenerates bindings, runs focused packages/frontend build and Chromium parent/child clock/resize/list-grid/window identity tests. Persist RED/GREEN reports per task; update H09/H11 implementation evidence separately from native acceptance.
- [ ] Prepare only disposable fixture tests for real child Show/Hide/Close/Space validation, folder access/entry open and exact window actions. Actual GUI, chooser, activation, input, multi-display/fullscreen acceptance requires explicit coordination; this plan runs none. Already-dispatched NSWorkspace/AX actions cannot be recalled; native filesystem calls can outlive cancellation, so receipts report actual joins rather than invented deadlines.

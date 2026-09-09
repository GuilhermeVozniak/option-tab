# Native Switcher Materials Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Root assigns disjoint native, App and frontend ownership; workers do not commit or regenerate bindings independently.

**Goal:** Complete B10 for the existing window/app switcher and native-Dock window previews: real desktop translucency where supported, with an explicit solid fallback.

**Architecture:** Reuse the launcher's native effect-container implementation without changing launcher or preview input policy. Dock preview material follows its exact panel bounds. The screen-sized switcher uses one trusted, session-scoped interior rectangle, never a full-screen effect. App owns asynchronous material admission and frontend receives truthful installation status.

**Tech Stack:** Go/Wails v3, AppKit NSVisualEffectView/CALayer, existing React renderers, injected native fixtures and Playwright.

**Spec:** `docs/superpowers/specs/2026-09-06-dockdoor-parity-design.md` (Preview/UI behavior); `docs/dockdoor-roadmap.md` B10. Existing native mechanism: `darwin_dock_panel.m:ot_launcher_panel_style`.

## Global Constraints

- Existing keyboard/focus/capture/Space behavior remains unchanged. No new input taps, permissions, native Dock preferences, app activation, capture or network activity.
- Scope: existing window-switcher, app-switcher and Dock `ContentKind == windows` previews. Replacement launcher styling stays unchanged. Folder/media cards and independent pinned-media/automation hosts are not implicitly restyled.
- Use existing mode-specific `config.Appearance.Blur`, `Theme`, corner radius and opacity. No new persisted appearance setting. Blur false requests solid; true requests system material. Theme is system/light/dark; CSS retains tint, opacity, border and text.
- An unavailable/missing native styler or missing/invalid rectangle means solid CSS fallback. Never infer support from CSS backdrop-filter or expose an invisible fallback.
- Never hold App.viewMu, dockWindow.mu, liveWindow.mu or a platform Go mutex across AppKit dispatch. Native host closure must retire tokens before Wails destroys content.
- Cancellation/replacement invalidates admission synchronously; queued work must check exact owner/session/revision before mutating native views. No generic window-pointer RPC.
- Use only local SDK/Wails source and existing native registry code. Preserve exact `//go:build darwin` tags. Physical acceptance is separate and must be coordinated with root.

## Frozen ports and transport

Create `internal/platform/material.go` (root freezes definitions before parallel implementation):

```go
type MaterialScope struct { Session, Revision uint64 }
type MaterialStyle struct {
    Scope MaterialScope
    Enabled bool
    Theme string
    CornerRadiusPx int
    Rect domain.Bounds // host-local top-left logical points
}
type MaterialStatus struct {
    Session uint64 `json:"session"`
    Revision uint64 `json:"revision"`
    State string `json:"state"` // system|solid|unavailable
    Reason string `json:"reason,omitempty"`
}
type MaterialSurface interface {
    Apply(context.Context, MaterialStyle) (MaterialStatus, error)
    Retire(MaterialScope, int) error // fadeMs: only 0 or 180; terminal for this session
    Close() error // terminal for this surface, idempotent; restore owned views
}
type PreviewMaterialHost interface {
    CreatePreviewMaterial(DockPanel) (MaterialSurface, error)
}
type OverlayMaterialHost interface {
    CreateOverlayMaterial(unsafe.Pointer) (MaterialSurface, error)
}
```

- A surface binds one exact native host incarnation; create methods are internal trusted adapters, not renderer methods. Preview factory verifies its DockPanel token belongs to the standard preview policy, not a launcher/media panel. Overlay factory validates its live Wails NSWindow on main and registers destruction cleanup.
- MaterialScope.Revision is an App-owned material mutation serial, distinct from high-frequency state/content revisions. Sessions are existing switcher/Dock presentation IDs. New surface incarnation starts its own owner; a retired session cannot Apply again. App serializes desired-state delivery and skips superseded scopes before native dispatch; native also enforces revision/session high-water marks. Different sessions must advance monotonically within a surface, so retirement needs bounded counters rather than an unbounded tombstone set.
- Apply requires nonzero session/revision. Enabled requires exactly one finite Rect, positive W/H, contained entirely within current content bounds, each coordinate/dimension magnitude <=65536 points. Radius is integer 0..64, further clamped to half the shorter edge. Theme outside system/light/dark refuses. Missing/retired native evidence refuses without touching a successor.
- Preview Rect is generated from current backend panel size, `{0,0,W,H}`. Overlay Rect comes only from the renderer's actual panel, validated again against current native content bounds. No DPR multiplication: browser CSS pixels correspond to host-local logical points; native converts top-left to AppKit bottom-left exactly once.
- `Retire` first tombstones the session, then removes/fades only its existing effect. Fade 180ms mirrors existing switcher dismissal; new Apply cancels old visual completion and starts opaque. Dock hide uses 0. A late retired-session completion cannot remove a new effect. Close/destruction restores content and cancels animation immediately.
- App private owner holds latest desired mutation and one queued UI operation, following `dockWindow.scheduleLocked/reconcile`. Surface operations run from that worker without owner/App locks. Status is admitted only for exact current surface incarnation and desired scope after the call returns.

Root RPC and event names:

```go
SetSwitcherMaterialRect(session, stateRevision, sequence uint64,
    x, y, width, height float64) error
GetSwitcherMaterialStatus(session uint64) platform.MaterialStatus
GetDockMaterialStatus(session uint64) platform.MaterialStatus
```

- Rectangle admission requires visibleSwitcherSession and controller.PresentationSession match; stateRevision must equal the last emitted switcher revision; sequence is positive and strictly increasing per session. Sequence prevents reordered ResizeObserver reports from restoring old geometry. Geometry/state changes allocate a new internal material revision.
- Event `switcher:material` / `dock:material` carries MaterialStatus. Getters return copied current status, with unavailable/missingReporter for a live switcher lacking a valid report. No creation, show or capture is triggered by a getter.
- MaterialStatus.Revision uses the material serial, so clients compare it only within the material channel. They accept only their current presentation Session and reject lower/equal revisions. Hide immediately retires frontend admission; status from an old surface never affects the next route.

## Task 1: Native material ownership and restoration

**Owner:** native worker. **Files:** new `platform/material.go` definitions root-owned; create `darwin_material.go/m/h`, `material_stub.go`, `material_test.go`, native fixture under `testdata/material`; minimally extract effect-container helper from `darwin_dock_panel.m` and adapt existing launcher call without changing its policy.

- [ ] Write RED injected native tests for one effect behind retained content, rectangular top-left conversion on negative-screen-origin hosts, radius/containment/NaN refusal, solid removal, and exact content frame/autoresizing restoration.
- [ ] Add host token/revision tests: retire then late Apply, queued A→B→A, host destruction/recreation, stale fade completion and non-preview policy refusal; all must leave successor content untouched.
- [ ] Extract only visual attachment/removal/theme/clip logic. Keep launcher-specific record fields/policy authorization as an adapter; do not apply native materials to every panel globally.
- [ ] Implement preview material token lookup in the existing registry. Implement an independently registered overlay effect below Wails content without reparenting WKWebView or changing key/collection/level behavior. Preserve original view/frame/mask/appearance properties that are actually modified.
- [ ] Use NSVisualEffectMaterialHUDWindow, BehindWindow and Active, matching the existing launcher. Solid removes the effect. Unsupported/failure returns unavailable and leaves a valid solid host; never destroys a working preview because material failed.
- [ ] Bind native WillClose cleanup while the host/content are alive; double Close is harmless. Context admission is checked before and after preparation and immediately before applying on main, using private context/owner registry only.
- [ ] Run focused native/pure race tests and platform lint; compile !darwin stubs. Fixtures substitute native hosts and do not open user windows.

## Task 2: App ownership, scheduler and truthful fallback

**Owner:** root/App worker. **Files:** new `app_material.go/test.go`; narrow `app_switcher.go`, `app_dock.go`, `app_dock_window.go/test.go`, `app_window.go`, `app.go`, `main.go`. Root alone updates generated bindings.

- [ ] Write RED fake-surface tests: missing reporter stays solid; unsupported/error material leaves the preview usable; stale rectangle or completed native Apply cannot publish after hide, mode replacement, topology resize or host recreation.
- [ ] Extend dockWindow desired state with optional material request and current material surface. Reconcile calls Show(bounds) then applies style to that exact current panel in the same dispatched operation, outside locks; a new native panel begins at 1×1, so its final bounds must exist before strict rectangle validation. Failure only updates fallback status. Dispose closes material before existing panel/content restoration. No style changes for launcher/media factories.
- [ ] `showDock` captures window-content appearance with Session and actual bounds; changing to folder/media retires the prior window material. Dock admission epoch and native host incarnation gate returned status; `dismissDockLocked` invalidates it synchronously before scheduling hide.
- [ ] Switcher Show starts missingReporter fallback for the new session, and snapshots appearance/host ownership. Ordinary selection/content revisions retain the last admitted same-session rect until the reporter updates it; identical rect/style reports avoid another native Apply. New session, host-size or material-affecting layout/style changes invalidate the rect and use solid fallback pending a current report. Capture frames alone never retire material. FitOverlayToScreen remains unchanged.
- [ ] Implement rectangle RPC validation and monotonic sequence. Enqueue one latest request; apply only after releasing viewMu and recheck exact surface/session/material serial before emitting status. Never call CreateOverlayMaterial or Apply inline under Show/Update's existing viewMu lock.
- [ ] HideSession/finishHideLocked use existing session and viewGeneration checks: stop admission immediately, enqueue native retire with matching 180ms/0 fade, then existing native overlay hide. New Show invalidates old hide/animation. Shutdown/host loss closes surfaces outside App locks before their native content is destroyed.
- [ ] Add fake blocked-Apply tests for settings blur true→false→true and session A→B, cancellation after native preparation, and host closure callback reentry; expect no deadlock, no late status and no successor style mutation.
- [ ] Run focused App race tests and lint, then freeze RPCs/events and generate bindings once.

## Task 3: Exact panel rectangle and native-status rendering

**Owner:** frontend worker. **Files:** new `frontend/src/lib/material.ts` and tests; `App.tsx`/window-switcher panel renderer, `app-switcher/AppSwitcher.tsx`, `app-switcher.css`, `styles.css`, `dock/DockPanelView.tsx`, `dock.css`, bridge wrappers/tests, dedicated browser test.

- [ ] Write RED tests for missing/native-refused status falling back, session/revision/sequence admission, and long/dynamic content changing the reported panel rectangle without changing native host size.
- [ ] Attach ResizeObserver to the actual `.ot-panel` / `.ot-app-panel`, not the full-screen overlay or preview thumbnail. Report its getBoundingClientRect after each accepted switcher state revision and layout change. Coalesce with requestAnimationFrame, cap at 30 reports/sec, suppress unchanged geometry within the same state revision, and disconnect/cancel on hide/unmount.
- [ ] Keep initial solid fallback until an admitted system status arrives. If blur=false or native status solid/unavailable, use existing solid/tinted background and no CSS desktop-backdrop claim. With system status, preserve theme/tint/opacity/border/contrast while letting the native effect show behind content. Existing appearance controls remain independently scoped by mode.
- [ ] Consume current status RPC at mount and ordered material events thereafter. Apply only matching presentation sessions; do not infer native success from reporter RPC success. Dock uses backend bounds and must not add a DOM resize feedback loop.
- [ ] Browser tests assert interior report bounds (including narrow viewport), state-revision re-report, missing reporter fallback, mode/theme/blur changes, stale replies/events after replacement and unchanged keyboard/preview selection behavior.
- [ ] Run focused unit tests, Biome, build and dedicated Chromium tests. Native acceptance claims require Task 4.

## Task 4: Bounded integration and acceptance

**Owner:** root coordinates workers and any real native fixture. **Files:** tracked report `docs/superpowers/reports/2026-09-07-native-switcher-materials.md`; native source/logs in ignored SDD ledger.

- [ ] Independently inspect lock/host-content ownership and fallback paths, then run combined focused race/frontend gates; do not expand into unrelated input/layout redesign.
- [ ] With explicit coordination, use only a disposable nonactivating host to verify actual native effect installation/removal, clipping and content restoration; record exact host/content/effect bounds and unchanged foreground. No user app actions, pointer movement or native Dock changes.
- [ ] Record actual switcher/preview visual acceptance separately: real desktop blur rather than CSS blur, no screen-wide material outside the interior rect, light/dark/solid fallback, resize, fade/hide/reopen and host destruction. If physical acceptance is unavailable, leave it explicitly pending instead of marking B10 fully accepted.
- [ ] Root updates roadmap evidence and owns Git/checkpoint integration. This plan authorizes no unrelated H features.

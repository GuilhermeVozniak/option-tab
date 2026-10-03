# Folder Pop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Implement E01–E03: hover an exact Dock folder to list and sort its contents, request access only from an explicit user action, and open an exact listed file or folder.

**Architecture:** Extend the existing Dock observer and panel lifecycle with a typed folder item and discriminated panel content. A bounded folder service owns enumeration, security-scoped bookmarks, and exact-item opening; the browser receives opaque item IDs and never supplies a path to open.

**Tech Stack:** Go, Objective-C/AppKit Accessibility and security-scoped bookmarks, Wails, React/TypeScript, Vitest and Playwright.

**Spec:** `docs/superpowers/specs/2026-09-06-dockdoor-parity-design.md`, milestone 5; retained items E01–E03 in `docs/dockdoor-roadmap.md`.

## Global Constraints

- Folder Pop is read/open only. Do not add file staging, copy/move, pinned shelves, AirDrop, scripts, or DragZone behavior.
- `Dock.FolderPop.Enabled` defaults off and is independent of `Dock.Enabled` (window previews). The Dock observer runs when either feature is enabled; per-item admission shows apps only for window previews and folders only for Folder Pop, so either or both may be enabled.
- The exact Dock `kAXURLAttribute` file URL is the folder identity. Never infer a path from the localized Dock title.
- Hover may inspect existing access but must never open `NSOpenPanel` automatically.
- D09–D14 app gestures, wheel ownership, preview dragging, and Aero Shake must never arm for folder items.
- Enumeration is capped at 500 entries and 500 ms. Return an explicit partial state when the cap is reached; cancellation and deadlines stop work.
- Every asynchronous listing, grant, and open result is admitted by current session, revision, folder identity, and service generation.
- Security-scoped bookmarks live in an application-support store separate from settings JSON. Start and stop resource access in the same bounded operation.

---

### Task 1: Typed Dock folder identity

**Files:**
- Modify: `apps/desktop/internal/platform/dock.go`
- Modify: `apps/desktop/internal/platform/darwin_dock.go`
- Modify: `apps/desktop/internal/platform/darwin_dock.m`
- Modify: `apps/desktop/internal/platform/darwin_dock_test.go`
- Modify: `apps/desktop/internal/dock/hover.go`
- Modify: `apps/desktop/internal/dock/controller.go`
- Test: `apps/desktop/internal/dock/controller_test.go`
- Test: `apps/desktop/internal/platform/darwin_dock_test.go`

**Interfaces:**
- Produces `platform.DockItem.Kind string`, with only `"app"` and `"folder"` admitted.
- Produces `dock.Item{Kind:"folder", AppID:0, BundleID:"", Path:<canonical folder path>}`.
- Folder identity is `(Kind, Path)`; app identity remains `(Kind, AppID, Path)`.

- [x] Write mapping tests for exact folder subrole + file URL, malformed/non-file URL, non-directory path, stale Dock PID, invalid bounds, and title/path ambiguity.
- [x] Run `go test ./apps/desktop/internal/platform -run 'TestDock.*Folder'` and verify the new cases fail because folder subroles are rejected.
- [x] Add `Kind` to `platform.DockItem`; classify application and folder subroles separately inside the existing bounded AX parent walk. Canonicalize the URL exactly as the app path is canonicalized, reject a positively observed non-directory during worker-side classification, while retaining exact folder URL authority when access restrictions prevent classification, and keep `AppID == 0` for folders.
- [x] Update app mapping to set `Kind:"app"`; update conversion/copy helpers without changing app behavior.
- [x] Add controller tests proving a folder can enter hover delay/panel placement while app composition is not called and folder-to-app transitions create a new presentation.
- [x] Add regressions proving `DockInputTarget`, panel wheel policy, app icon actions, and preview regions remain empty/disabled for `Kind:"folder"`.
- [x] Run `go test -race ./apps/desktop/internal/platform ./apps/desktop/internal/dock`.

### Task 2: Bounded folder service and bookmark store

**Files:**
- Create: `apps/desktop/internal/platform/folder.go`
- Create: `apps/desktop/internal/platform/darwin_folder.go`
- Create: `apps/desktop/internal/platform/darwin_folder.m`
- Create: `apps/desktop/internal/platform/darwin_folder.h`
- Create: `apps/desktop/internal/platform/folder_stub.go`
- Create: `apps/desktop/internal/platform/folder_test.go`
- Create: `apps/desktop/internal/platform/darwin_folder_test.go`
- Create: `apps/desktop/internal/folder/bookmarks.go`
- Create: `apps/desktop/internal/folder/bookmarks_test.go`

**Interfaces:**

```go
type FolderRef struct { Identity, Path string }
type FolderSort struct { Field, Direction string; FoldersFirst bool }
type FolderEntry struct {
    ID, Name, Kind string
    Size, ModifiedAtMs int64
    Hidden bool
}
type FolderListing struct {
    FolderIdentity string
    Entries []FolderEntry
    Status, Reason string // ready|permissionRequired|missing|revoked|partial|unavailable
}
type FolderGrant struct { FolderIdentity, Status, Reason string }
type FolderSource interface {
    ListFolder(context.Context, FolderRef, FolderSort) (FolderListing, error)
    RequestFolderAccess(context.Context, FolderRef) (FolderGrant, error)
    OpenFolderEntryGuarded(context.Context, FolderRef, string, func() error) error
}
```

- [x] Write pure tests for name/modified/size/kind sorting, ascending/descending order, folders-first, deterministic normalized-name + opaque-ID ties, 500-entry truncation, deadline cancellation, missing folder, and revoked bookmark.
- [x] Run `go test ./apps/desktop/internal/folder ./apps/desktop/internal/platform -run Folder` and verify missing implementations fail.
- [x] Implement opaque IDs as a random service-generation token plus per-listing index; keep the canonical URL, file identity `(device,inode)`, and parent identity only in a backend listing snapshot. Never derive or accept a path from an opaque ID.
- [x] Implement a bookmark store under application support using atomic `0600` replacement. Key by canonical folder identity, store bookmark bytes only, copy returned bytes, and remove stale/revoked records after a failed refresh.
- [x] Implement `ListFolder`: resolve existing bookmark without UI, start security-scoped access, verify the resolved folder has the same file identity, enumerate resource keys without following descendants, sort/cap, then stop access with `defer`.
- [x] Implement `RequestFolderAccess` with `NSOpenPanel` configured for one directory. Preselect the requested folder when possible and require its canonical URL to equal the exact captured Dock URL. If a previously captured file identity exists, require it to match; when access restrictions prevented the initial stat, a matching explicit chooser selection establishes and stores the first identity. Create a security-scoped bookmark, persist it, and stop access. Cancellation returns `permissionRequired` without changing the store.
- [x] Implement `OpenFolderEntryGuarded`: resolve the current bookmark, start access, look up the opaque ID in the current listing snapshot, re-stat parent and child identities, require the child still belongs directly to the same parent, invoke `guard()` immediately before `NSWorkspace openURL:`, refuse on any guard error, then dispatch and stop access.
- [x] Run `go test -race ./apps/desktop/internal/folder ./apps/desktop/internal/platform -run Folder`.

### Task 3: Folder controller and session admission

**Files:**
- Create: `apps/desktop/internal/dock/folder_controller.go`
- Create: `apps/desktop/internal/dock/folder_controller_test.go`
- Modify: `apps/desktop/internal/dock/types.go`
- Modify: `apps/desktop/internal/dock/controller.go`

**Interfaces:**

```go
type FolderState struct {
    Status, Reason, FolderIdentity string
    Entries []platform.FolderEntry
    Sort platform.FolderSort
    Partial bool
}
type State struct {
    // existing fields remain
    ContentKind string // windows|folder
    Folder *FolderState
}
```

- [x] Write fake-source tests for permission required, ready listing, missing/revoked access, stable sorting, cap/partial result, delayed old-session result, same-session old revision, hover retirement, and folder-to-app transition.
- [x] Run `go test ./apps/desktop/internal/dock -run FolderController` and verify failures.
- [x] Implement one owner goroutine per folder presentation. Copy entries/sort on every boundary; cancel listing when identity/session/admission changes; never hold controller locks across source calls.
- [x] Treat an accepted access request as a separate exact-folder grant owner. Ordinary hover dismissal hides the panel but does not cancel the chooser. A completed grant may refresh only the original still-current folder session; if that presentation retired, persist the valid grant but emit no panel update. Settings disable, pause, session inactivity, shutdown, and explicit cancellation stop and join the chooser owner.
- [x] Emit the existing `View.Show/Update/Hide` lifecycle with `ContentKind:"folder"`, the same panel bounds/edge/corridor behavior, and no window capture IDs.
- [x] Add `SetFolderSort(session,revision,sort)` and `RefreshFolder(session,revision)` commands. Reject stale scope without starting I/O.
- [x] Run `go test -race ./apps/desktop/internal/dock -run 'Folder|Controller'`.

### Task 4: App service, exact open, and access flow

**Files:**
- Create: `apps/desktop/app_folder.go`
- Create: `apps/desktop/app_folder_test.go`
- Modify: `apps/desktop/app.go`
- Modify: `apps/desktop/app_dock.go`
- Regenerate: `apps/desktop/frontend/bindings/option-tab/**`

**Interfaces:**

```go
func (a *App) SetDockFolderSort(session, revision uint64, field, direction string, foldersFirst bool) error
func (a *App) RequestDockFolderAccess(session, revision uint64) error
func (a *App) CancelDockFolderAccess(session, revision uint64)
func (a *App) OpenDockFolderEntry(session, revision uint64, itemID string) error
```

- [x] Write App tests proving each RPC requires current folder content, session, revision, and exact folder identity; app panels and retired folder panels are refused.
- [x] Add blocked-source tests where ordinary hover hide occurs during access and prove the accepted chooser continues. Add folder replacement/reopen tests proving a late valid grant never updates the new panel. Pause, Folder Pop disable, inactivity, shutdown, and explicit cancellation must stop the chooser. For open, block immediately before dispatch and prove a session/revision/folder/item change makes the final guard refuse with zero workspace calls.
- [x] Run `go test ./apps/desktop -run DockFolder` and verify failures.
- [x] Add folder fields to the copied Dock DTO and revision every folder state/error transition. Keep `Entries` empty and capture manager stopped for folder content.
- [x] Wire the optional `FolderSource`, expose the four scoped RPCs, and emit visible current-revision errors. `CancelDockFolderAccess` cancels only the matching accepted grant owner; stale cancellation cannot affect a newer request. The request-access RPC is the only path allowed to show `NSOpenPanel`.
- [x] Pass a final App scope validator into `OpenFolderEntryGuarded`; it must verify current folder session, revision, canonical folder identity, and opaque item ID immediately before the adapter calls `NSWorkspace`. Do not hold App/view locks while the adapter runs.
- [x] Regenerate bindings and run `go test -race ./apps/desktop -run DockFolder`.

### Task 5: Reused panel UI and localized interactions

**Files:**
- Create: `apps/desktop/frontend/src/dock/FolderPanel.tsx`
- Create: `apps/desktop/frontend/src/dock/FolderPanel.test.tsx`
- Modify: `apps/desktop/frontend/src/dock/DockPanelView.tsx`
- Modify: `apps/desktop/frontend/src/dock/dock.css`
- Modify: `apps/desktop/frontend/src/App.tsx`
- Modify: `apps/desktop/frontend/src/lib/dock-bridge.ts`
- Modify: `apps/desktop/frontend/src/lib/types.ts`
- Modify: `apps/desktop/frontend/src/lib/i18n.ts`
- Modify: `apps/desktop/internal/config/dock.go`
- Modify: `apps/desktop/internal/config/config.go`
- Test: `apps/desktop/internal/config/dock_test.go`
- Modify: `apps/desktop/frontend/e2e/support/fakeWails.ts`
- Create: `apps/desktop/frontend/e2e/folder-pop.spec.ts`

- [x] Write config tests proving `Dock.FolderPop.Enabled` defaults false, survives save/load, normalizes old missing configuration to false, and remains independent from `Dock.Enabled`. Write component tests for ready/permission-required/missing/revoked/partial states, sort controls, folders-first, long-name truncation, keyboard focus, and visible action errors.
- [x] Run `bun run test -- src/dock/FolderPanel.test.tsx` and verify failures.
- [x] Render `FolderPanel` inside the existing Dock shell when `contentKind === "folder"`. Reuse theme, opacity, sizing, pointer admission, native placement, dismissal corridor, and session/revision tombstones; do not render window controls, app actions, thumbnails, or fake window cards.
- [x] Add localized EN/PT-BR/ES labels for access, retry, missing/revoked/partial states, sort field/direction, folders-first, and open failures.
- [x] Add `Dock.FolderPop.Enabled bool` and an independent localized “Enable Folder Pop” control. Configure native Dock observation when `Dock.Enabled || Dock.FolderPop.Enabled`; app observations remain gated by `Dock.Enabled`, folder observations by the Folder Pop toggle.
- [x] Wire scoped access/sort/open handlers. Disable repeated access while pending; opening a folder uses the same exact-item API as opening a file.
- [x] Add Chromium coverage that delivers a real folder event, confirms no automatic chooser RPC on hover, requests access by explicit click, changes sorting, opens one opaque ID with session/revision, rejects a reordered stale update, and displays missing/revoked errors.
- [x] Run `bun run test -- src/dock/FolderPanel.test.tsx src/App.test.tsx`, `bun run build`, and `bunx playwright test e2e/folder-pop.spec.ts --project=chromium` from `apps/desktop/frontend`.

### Task 6: Native disposable-folder acceptance and milestone gate

**Files:**
- Create: `apps/desktop/internal/platform/testdata/folder-pop/README.md`
- Create: `apps/desktop/internal/platform/darwin_folder_smoke_test.go`
- Modify: `docs/dockdoor-roadmap.md`
- Create: `.superpowers/sdd/2026-09-06-folder-pop/acceptance-report.md`

- [ ] Build a disposable directory containing files, a subfolder, hidden content, equal-name sort cases, and a symlink outside the directory. Do not open user files.
- [ ] Verify read-only enumeration, every sort mode, cap/partial behavior, direct-child identity checks, and symlink refusal in the disposable fixture.
- [ ] Exercise grant denial/cancellation, successful bookmark reuse after service restart, revoked/stale bookmark, and deleted-folder states. Mark chooser UI evidence separately from injected bookmark tests.
- [ ] Open only a disposable fixture file/folder and verify `NSWorkspace` receives the exact revalidated URL. Record that this proves dispatch, not user-default-app behavior.
- [ ] Run focused Go race tests, frontend tests/build, Chromium Folder Pop tests, repository lint, and existing Dock regression suites.
- [ ] Mark E01–E03 complete only when actual Dock folder AX classification, an explicit native access grant, persisted bookmark reuse, revoked/missing behavior, and exact disposable-item opening pass. Record any unsigned/sandbox limitation instead of converting simulated evidence into acceptance.


## Integration decisions and evidence

- Implemented Tasks 1–5 with independent controller/UI/App reviews. Native acceptance remains separate in Task 6; see the checkpoint report.
- Ruling: accepted grants retain their original folder session, admission epoch and internal folder revision after initial exact UI-revision validation. Geometry-only outer revisions do not cancel a valid refresh. Cost if wrong: a grant may refresh a resized panel; changed contents, identity or session still refuse it. Open dispatch retains exact outer-revision checks.
- Ruling: one serialized cancellable folder read serves current presentations, with only the newest pending request. No periodic folder relisting; explicit retry reuses the current sort command. This preserves opaque IDs. Cost if wrong: external directory changes appear on explicit refresh/rehover instead of immediately.
- Filesystem deadlines are cooperative between calls; individual OS filesystem calls are not forcibly interruptible. Partial UI copy describes incomplete results without claiming 500 entries or globally first results.
- Accepted chooser owners remain reserved until native cleanup returns. Ordinary hover hide does not cancel them; disable, pause, inactive session, explicit cancellation and shutdown initiate cancellation. The request RPC returns after the source finishes cleanup, without holding App locks across source calls.

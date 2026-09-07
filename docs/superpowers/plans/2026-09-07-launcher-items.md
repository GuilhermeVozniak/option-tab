# Launcher Items Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans task-by-task. Coordinator owns integration and commits; native acceptance requires a separately coordinated disposable fixture.

**Goal:** Deliver retained H04/H05/H09/H11 items, grouping, folder fan-out and exact app context actions on the existing replacement launcher.
**Architecture:** Keep portable profile structure separate from private selected-resource authority. Extend the current launcher owner and guarded native dispatch; compose existing folder/window services in parent-owned child presentations.
**Tech Stack:** Go, React/Wails, AppKit NSOpenPanel/NSWorkspace, Foundation bookmarks, POSIX bounded reads.
**Spec:** `docs/dockdoor-roadmap.md` H04/H05/H09/H11; `docs/superpowers/specs/2026-09-06-dockdoor-parity-design.md`; replacement-dock lifecycle/widget specs dated 2026-09-07.

## Global constraints and existing evidence

- Default configuration adds no pins and opens nothing. Import, startup, observation, reorder and icon changes never launch an application, open a URL or show a chooser.
- Preserve native Dock recovery, per-display ordinary-Space gating, pause/session retirement, self/blacklist exclusions, existing widget grants and all media/preview ownership.
- No shell, commands, arguments, environment injection, generic URL schemes, forced quit, tiling, Dock preferences or input taps.
- Existing `platform.LauncherAppActivator` only activates an exact running PID/start/bundle with native token/Space checks. Do not widen it to launch arbitrary paths.
- Existing `FolderSource` takes a canonical captured `FolderRef` and current opaque entry IDs; it checks parent/child identity again after its Go guard. Its bookmark resolver rejects a moved path. Reuse those semantics for listings, not as a persistent pin identity algorithm.
- SDK `NSURL.h` explicitly says file/volume resource identifiers are not persistent across restarts. Persist native bookmark authority, not serialized inode/resource-ID equality as permanent identity.
- SDK `NSRunningApplication.terminate` is asynchronous and can refuse. NSWorkspace open configuration supports exact application URL and `allowsRunningApplicationSubstitution=false`; no terminate-return-as-exit assumption.

## Frozen model and boundaries

- Add `LauncherProfile.Items []LauncherItem` in config, keep version 2 with optional Items; older v2 initializes no items, and legacy v1 explicitly rejects an Items field. Existing bounded decoder permits at most 16 elements per array: retain max 16 item records per profile, including group members and separators; no unbounded decoder relaxation.
- `LauncherItem{ID,Kind,Label,ReferenceID,URL,IconID string; Members []string; FolderView string}`; kinds `app|folder|file|link|group|spacer|separator`; folder view `list|grid`. IDs follow existing launcher ID rules; labels <=80 runes/no NUL; unused fields must be empty.
- Groups contain app item IDs only, one level, unique membership and no cycles; members are removed from top-level visual slots. Array order is top-level/member order; a group never activates all members. Empty groups are refused. Spacers/separators have no action or keyboard focus.
- Link input is parsed once: absolute http/https, host required, <=2048 UTF-8 bytes, no credentials, controls, file/custom schemes or network fetch. Renderer never opens it directly; native uses the stored exact canonical URL on explicit action.
- Private store keyed by opaque random ReferenceID holds bookmark bytes, resource kind, selected canonical location, app bundle identity and schema version; mode 0700 directory/0600 atomic files, bounded total 4 MiB. No bookmark/path/authority in renderer DTOs or diagnostics.
- A new-machine imported ReferenceID has no authority: `needsSelection`. H17 may export structural IDs, labels, links and bundle hints only, excluding bookmarks, resolved paths, private icons and grants. H17 must not treat those hints as permission.
- Reference states: `ready|moved|missing|accessRequired|changed|unsupported|unavailable`. Resolve without UI or mounting. A bookmark locating the same object at another path reports moved and requires explicit relink confirmation before dispatch; never silently open a new object at the old path.
- Revalidate a resolved reference before and after native preparation/final Go guard; runtime handle binds revision, resolved URL and freshly captured identity. Symlink substitution or changed app bundle refuses; signed app updates require explicit reselection when stored identity no longer validates. Document unavoidable last-check-to-NSWorkspace pathname race.
- Custom icon selection is PNG only, <=2 MiB encoded, dimensions <=1024 each and <=1M pixels; decode, strip metadata and normalize to <=256px PNG before storing by digest, <=512 KiB/result, total private icon store <=8 MiB. No remote icons, SVG or executable formats. Reuse bounded image decoding patterns; no raw selected bytes in config.

## Native ports (new `platform/launcher_items.go`)

```go
type LauncherReference struct { ID, Kind, Label, BundleID, State, Reason string; Revision uint64 }
type LauncherItemAction struct {
    ReferenceID string; ReferenceRevision uint64; Kind string // open|relaunch
    Process ProcessIdentity; BundleID, DisplayUUID string; PanelToken uint64
}
type LauncherReferenceSource interface {
    ChooseLauncherReference(context.Context, string) (LauncherReference, error) // app|folder|file
    ResolveLauncherReference(context.Context, string) (LauncherReference, error)
    RelinkLauncherReference(context.Context, string) (LauncherReference, error)
    PerformLauncherItem(context.Context, LauncherItemAction, func() error) error
    WithLauncherFolder(context.Context, string, uint64, func(FolderRef) error) error
    OpenLauncherLink(context.Context, string, string, uint64, func() error) error // URL, display, token
}
```

- Source constructor takes a private store path; its IDs never expose that path. WithLauncherFolder retains native access through the callback, resolves the current canonical FolderRef and revalidates reference revision; callback uses existing FolderSource. No Go lock across callback/AppKit.
- Choose/relink require explicit preferences actions; one joined chooser globally for this source. Standard context is checked synchronously before queued presentation and final acceptance, not merely by polling. Cancellation keeps busy until main closure/read/security scope joins.
- Add separate `LauncherIconSource.ChooseLauncherIcon(ctx) (iconID string, err error)` plus read-only copied normalized icon lookup; icon source owns its private asset store and same bounded chooser discipline.
- Perform/open-link must validate exact native launcher token, physical visibility and original ordinary display Space before and after the bounded Go guard. Go guard rechecks current profile/item revision, current policy and exact host incarnation; caller holds no App/controller lock across native calls.
- App launch uses the selected resolved `.app` URL, exact bundle verification, NSWorkspace completion, substitution=false, newInstance=false, promptsUserIfNeeded=false. A selected running instance uses existing exact activation; multiple matching installations/instances refuse ambiguity. Returned native acceptance is not proof that a window appeared.
- Relaunch accepts an exact running PID/start/bundle plus selected reference, never PID-only. Request graceful terminate once, await that captured instance's actual exit for <=10s, then resolve/recheck reference and all guards again before one launch. Refusal, save dialog, timeout, replacement process, parent retirement or cancellation stops the sequence; no force escalation or deferred background launch. Already-delivered termination cannot be undone.

## Task 1 — configuration and private reference ownership

**Owner/files:** pure owner: new `internal/config/replacement_items.go`, tests; narrow version/clone/decoder edits in `replacement.go`; new `internal/launcheritems/store.go` and tests. Native owner: new `platform/launcher_items.go`, Darwin/stub/reference/icon files and fixture directory; shared port frozen before consumers.
- [ ] RED config cases: duplicate member across groups, cycle, unknown field, 17 records, file URI, embedded credentials, imported missing authority. RED store tests: canceled write, failed save leaves old bytes, copied returns, private data absent from config JSON.
- [ ] Implement strict optional-v2 decoding, legacy-v1 refusal and cloning and bounded atomic private store; explicit reference removal saves profile first, then releases unreferenced private assets. Failed asset cleanup reports failure without undoing saved removal.
- [ ] RED native seams: cancel after main enqueue before presentation => zero panels; bookmark moved/replaced/revoked; restart resolution; file substitution during guard; app bundle mismatch; malformed/oversized PNG. Implement source with NSOpenPanel/Foundation/POSIX; no visible chooser tests.
- [ ] Verify focused `go test -race ./internal/config ./internal/launcheritems ./internal/platform -run 'Launcher(Item|Reference|Icon)|ReplacementItem'`; lint touched packages and portable stub compile. Keep RED/GREEN logs.

## Task 2 — item composition, reorder and App admission

**Owner/files:** pure: new `internal/launcher/items.go`/tests; narrow `types.go`, `reconcile.go`, `layout.go`, controller edits. App owner: new `app_launcher_items.go`/tests and narrow `app_launcher.go` hooks. Frontend owner: launcher route/components, replacement settings and bridge tests.
- [ ] RED owner test: blocked resolve A, reorder/remove/re-add A => old result/action refused; profile switch A→B→A cannot reuse session. RED App test: native guard callback publishes disabled settings/Space retirement => zero dispatch.
- [ ] Publish copied item DTOs with kind/status and opaque IDs. Merge pins with eligible running apps without duplicate exact references; unpinned running apps remain visible. Pinned apps retain stable item IDs across process exit; bind current process afresh per action.
- [ ] Add scoped RPCs `ActivateLauncherItem(scope,itemID)`, `SetLauncherItems(profileID,expectedConfigRevision,items)` and explicit choose/relink/icon preferences RPCs. Save successful reorder before publication; reject stale config revision. Never accept a renderer path or native token.
- [ ] Implement internal drag reorder/grouping only, keyboard move equivalents and accessible labels; no external dropped-file authority. Show missing/access/moved states with explicit relink. Bounds/layout account for groups, separators and widgets together without violating native recovery corridor.
- [ ] Verify focused owner/App race tests and browser tests for reorder persistence, stale revisions, grouping, icon fallback and disabled items; no implicit native action during rendering/import.

## Task 3 — folder and app-window child presentations

**Owner/files:** new `internal/launcheritems/presentation.go`/tests, `app_launcher_item_panels.go`/tests; narrow reusable `FolderPanel.tsx`/window-card components and launcher route. Do not synthesize a native DockItem or invoke public automation RPCs as an internal shortcut.
- [ ] RED parent retirement while folder list/window enumeration blocks => no child publish/open; child from display A cannot dispatch through replacement host B. Sorting/reopening old result cannot overwrite newer listing revision.
- [ ] One child per launcher parent session, folder list/grid or app show-all. Capture parent scope+item/reference revision; close/cancel/join on parent hide, profile change, disconnect, inactive/pause/shutdown. Child bounds use current usable display frame; joining never holds viewMu.
- [ ] Reuse FolderSource sorting, opaque entry IDs, incomplete/access states and FolderPanel. The pinned root itself opens via PerformLauncherItem; entries use OpenFolderEntryGuarded through WithLauncherFolder. Exact parent validator surrounds entry guard and rechecks logical admission afterward.
- [ ] Show-all means current filtered real windows of the exact app in an Option Tab preview, not a private Exposé gesture. Reuse automation preview's capture/identity/card mechanics via a parent-owned helper, keeping automation and launcher sessions separate; respect appearance/capture policy and shared stream budgets.
- [ ] Window actions reuse `actions.Service.PerformAutomationWindowAction` with captured process/window identity and explicit fullscreen/minimize semantics. Show empty/error states honestly; no first-window fallback. Relaunch menu calls the dedicated native operation above, with status for refusal/timeout.
- [ ] Verify fake-native blocked list/lookup/capture completion, parent-hide independence from automation/media, exact child identity and no capture when disabled; browser list/grid, errors, context actions and long-name layout.

## Task 4 — integration and bounded acceptance

- [ ] Coordinator regenerates bindings once ports/RPCs freeze; run focused race/lint, production frontend build and targeted Chromium launcher-item cases. Update roadmap/report only for demonstrated implementation; preserve separate native acceptance checklist.
- [ ] Prepare disposable bundled app + temporary folder/file/PNG fixtures; injected NSWorkspace tests assert exact URL/configuration and zero dispatch on canceled/replaced preparation, relaunch exit/refusal/timeout and no action on import. No shell launch implementation.
- [ ] Separately coordinate real chooser select/cancel, bookmark reuse after service restart, only owned file/folder/browser test URL open, owned app graceful relaunch/save-dialog refusal, two-display child lifetime and native nonactivation. No user-app relaunch, persistent Dock changes or unannounced input. Record actual accepted dispatch versus completed launch/open distinctly.

## Handoff

Private references solve persistence without converting portable config into authority. Existing folder/window services retain exact-target behavior; new native launch/relaunch ports cover only the missing explicit operations. Commit/integration sequencing belongs to the coordinator; this plan does not authorize native GUI acceptance by itself.

## Coordinator port freeze

Root owns platform/launcher_items.go. Both sources include Close() error; icon lookup is LauncherIcon(context.Context,string)([]byte,error). Constructors NewLauncherReferenceSource(path) and NewLauncherIconSource(path) return (source,error). Reference DTO includes BundleID only as selected app identity, never a path or bookmark. Keep config version 2, as the additive widget fields do; the in-progress unreleased schema remains strictly bounded. Native source uses the pure private store; launcheritems package must not import platform (to avoid a cycle), so later child-presentation integration lives in App or another package.

Both source interfaces expose RemoveLauncherReference(ctx,id)/RemoveLauncherIcon(ctx,id) for private unreferenced-record/asset cleanup only. App persists removal first and never deletes the selected original resource. Native NSWorkspace completion is joined after dispatch; cancellation prevents new dispatch and stale completion but cannot recall an open already handed to macOS.

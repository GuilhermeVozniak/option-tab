# App Groups and Native Dock Previews Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task by task. Steps use checkbox syntax for tracking. The user has authorized implementation; no further scope approval is needed for this milestone.

**Goal:** Deliver B01–B03 app-grouped/windowless switching and D01–D08 native Dock app previews as working macOS features.

**Architecture:** Preserve the existing window controller and add an explicit app mode backed by real running-app inventory. A separate Dock controller consumes bounded native pointer/AX observations and uses the same filters, exact-target actions, and one admitted visible capture session. A native non-activating panel hosts a dedicated Dock webview route; the keyboard switcher and Dock panel are mutually exclusive.

**Tech Stack:** Go 1.26, Wails v3 alpha2.117, Objective-C/AppKit/Accessibility/ScreenCaptureKit, React/TypeScript/Vitest, Playwright.

**Spec:** `docs/superpowers/specs/2026-09-06-dockdoor-parity-design.md`; authoritative IDs in `docs/dockdoor-roadmap.md`; prerequisite checkpoint in `docs/superpowers/plans/2026-09-06-dock-foundations.md`.

## Global Constraints

- “Keep Option Tab's existing keyboard switcher and settings.”
- “Use versioned settings with migrations preserving existing shortcuts.”
- “New Dock behavior defaults off until enabled.”
- “Native event callbacks enqueue lightweight events; window enumeration, AX queries, capture, and persistence stay off event-tap callbacks.”
- “No hidden recording unless the user explicitly enables background capture.”
- “Windows/Linux remain explicit unsupported/demo backends until separately implemented.”
- “Do not add sibling-app dependencies or automatic integrations.”
- “Native panel styling, localization and release compatibility remain requirements for Option Tab.”
- Scope is B01–B03 and D01–D08 plus the minimal shared preview/control work needed for them. D09–D15 gestures, folders/media, automation, distribution, and the separately tracked Dock replacement are not completed by this plan.
- No tiling, maximize/restore engine, centering, calendar/weather, file staging, AirDrop, saved-command execution, or system Dock preference changes.
- Work on `feat/dockdoor-parity`. The coordinator owns the git index, bindings generation, shared wiring files, and milestone commits. Implementers do not commit shared files.
- Finish the foundations review fixes and integrated check before starting production changes from this plan. A pure helper or mocked control does not satisfy native acceptance.

## Current architecture and important seams

Paths below are relative to `apps/desktop` unless explicitly rooted at `docs/`.

- `internal/domain/domain.go` already defines `App`, `Window`, `Screen`, and top-left global point `Bounds`; reuse these rather than inventing a second coordinate system.
- `internal/switcher/switcher.go` stores a window list and selection. `State.Entries` currently means windows; `StyleAppIcons` is only a visual style and does not group applications. Treat mode and style as separate settings.
- `internal/config/config.go` schema is version 2. `Behavior` mixes global lifecycle settings with switcher controls. Preserve global fields and existing JSON keys; an app-mode preference object contains only switcher behavior.
- `internal/filter/filter.go` already implements window visibility, app blacklists, and scope. Its `HideWhenNoWindow` blacklist value is intentionally dormant for window lists; app mode must implement it against actual raw window presence.
- `App.PerformAction` is the exact-target action bridge. Native New Window and Force Quit are optional capabilities. Keep visible error/partial-result handling on both surfaces.
- `internal/preview.Manager` has a per-instance four-stream semaphore. Creating independent managers for both surfaces would double that bound. Keep one admitted visible manager and route its output to the active surface; finish the old session before admitting another.
- `internal/dock/hover.go` already provides `Item`, `Hover.Step`, and `PlacePanel`. It does not yet observe the Dock, reposition a shown item whose bounds move, or host a panel.
- `main.go` currently creates a full-screen transparent switcher NSWindow and a settings window. The local Wails `MacWindow` API exposes levels/collection behavior but no NSPanel class. The existing window can activate Option Tab on click. A small true non-activating panel must be proven before D03 is marked complete.

## Runtime contracts

Task 1 owns these shared declarations and lands them before the consuming tasks start. Optional capabilities live in new platform files, not the required `Platform` aggregate.

```go
// internal/platform/apps.go
// Apps returns regular user-facing running applications, including those with
// no windows. It excludes terminated/background/accessory utilities and self.
type ApplicationSource interface { Apps() ([]domain.App, error) }
type ApplicationActivator interface { ActivateApp(domain.AppID) error }

// internal/config/modes.go
// Mode is independent of Appearance.Style.
type SwitcherMode string
const (ModeWindows SwitcherMode = "windows"; ModeApps SwitcherMode = "apps")
type SwitcherBehavior struct {
    HoldToCycle, VimKeys, ArrowKeys, MouseHoverSelect bool
    CursorFollowFocus, HapticFeedback bool
    ActionBindings map[string]ActionKind
    MiddleClickAction, SwipeUpAction, SwipeDownAction PointerAction
}
type ModePreferences struct {
    Appearance Appearance `json:"appearance"`
    Behavior SwitcherBehavior `json:"behavior"`
    Order OrderMode `json:"order"`
    Placement Placement `json:"placement"`
}
// Every SwitcherBehavior member receives its corresponding existing camelCase
// JSON tag. Add Shortcut.Mode `json:"mode"` and Settings.AppSwitcher
// `json:"appSwitcher"`. Existing window preference keys remain authoritative.
func (s Settings) Preferences(mode SwitcherMode) ModePreferences

// internal/config/dock.go
// JSON tags use the camelCase names below. Scope inherits global filters when
// its fields are empty, exactly like an existing shortcut scope.
type DockSettings struct {
    Enabled bool `json:"enabled"`
    HoverDelayMs int `json:"hoverDelayMs"`
    DismissDelayMs int `json:"dismissDelayMs"`
    HoverSlopPx int `json:"hoverSlopPx"`
    BridgePaddingPx int `json:"bridgePaddingPx"`
    Scope ShortcutScope `json:"scope"`
    Appearance Appearance `json:"appearance"`
}
// Add Settings.Dock `json:"dock"`.

// internal/platform/dock.go
// All native coordinates are top-left global points, not backing pixels.
// Sequence increases for every observation; Generation changes when native
// Dock identity/Space/display geometry/accessibility validity changes.
type DockObservation struct {
    Sequence, Generation uint64
    PointerX, PointerY float64
    Item *DockItem
    Status string // "ready", "permissionDenied", "dockUnavailable"
}
type DockItem struct {
    AppID domain.AppID
    BundleID, Path, Title string
    Bounds domain.Bounds
    ScreenID domain.ScreenID
    Edge string // "left", "bottom", "right"
}
type DockObservationSource interface {
    ObserveDock(context.Context, func(DockObservation)) error
}
```

Use `platform.DockItem` for the platform port and explicitly convert it to existing `dock.Item{Kind:"app", ...}` in `internal/dock`. This avoids a `platform -> dock -> platform` import cycle. No native AX references or NSRunningApplication objects cross the Go boundary.

App-mode `switcher.State` retains current fields and adds:

```go
Mode config.SwitcherMode `json:"mode"`
Apps []AppEntry `json:"apps"`
SelectedWindowID domain.WindowID `json:"selectedWindowId"`

// AppEntry represents exactly one running process; duplicate display names
// or bundle identifiers do not merge separately launched app instances.
type AppEntry struct {
    AppID domain.AppID `json:"appId"`
    AppName string `json:"appName"`
    BundleID string `json:"bundleId"`
    Hidden bool `json:"hidden"`
    WindowCount int `json:"windowCount"`
    Icon string `json:"icon,omitempty"`
}
```

In window mode, `Selected` indexes `Entries` as today. In app mode, `Selected` indexes `Apps`, and `Entries` contains only the selected app's eligible windows. `SelectedWindowID` identifies its selected preview, or zero for a truly windowless app. Do not put invented zero-ID windows into `Entries` or capture them.

## Settings defaults and migration decisions

| Setting | New install | Existing version 1/2 document |
|---|---|---|
| First default `command+tab` shortcut mode | `apps` | Missing mode becomes `windows`; preserve IDs/chords/scopes/overrides/release behavior |
| Second default `option+tab` shortcut mode | `windows` | Missing mode becomes `windows` |
| App appearance | App icons, selected preview on, other appearance defaults copied | Independent deep copy of current window appearance, then app-icon style and selected preview on |
| App behavior/order/placement | Independent copy of current switcher defaults | Independent copy of current switcher settings, including bindings and explicit empty maps |
| `dock.enabled` | false | false |
| Hover/dismiss delay | 300 ms / 250 ms | Same |
| Hover movement tolerance | 8 points | Same |
| Icon-to-panel corridor padding | 12 points | Same |
| Dock scope | Inherit global scope; app scope always all | Same |
| Dock appearance | Thumbnails; 240 px; max 2 rows / 5 columns; selected preview off; window controls on; fade off initially | Same, with current theme/accent copied once |

Bump schema to 3. Missing optional objects must initialize from the loaded legacy values, not an early `Default()` overlay that forgets the user's custom settings. Preserve explicit false, zero-delay, and empty binding maps. Validate delays in 0…2000 ms, slop in 0…32 points, corridor padding in 0…48 points. Normalize imported values to those bounds; direct save rejects invalid values consistently with existing validation. Global login, pause, onboarding, language, update/crash preferences, background capture and menubar state are never copied into per-mode controls.

## Ownership and execution order

| Task | Exclusive production ownership | Depends on |
|---|---|---|
| 1. Contracts/settings/pure app composition | `internal/config/{config.go,modes.go,dock.go}` and config tests; `internal/platform/{apps.go,dock.go}`; `internal/appgroup/*`; `internal/filter/apps.go` and tests; frontend `lib/types.ts` | Foundations checkpoint |
| 2. Native app inventory/activation | `internal/platform/darwin_apps.{go,m,h}`, `apps_stub.go`, associated platform tests | Task 1 declarations |
| 3. App-mode controller | `internal/switcher/{switcher.go,apps.go,apps_test.go}` and existing switcher tests | Tasks 1–2 contracts; fake native source sufficient during coding |
| 4. Native Dock observation | `internal/platform/darwin_dock.{go,m,h}`, `dock_stub.go`, associated tests | Task 1 declarations |
| 5. Pure Dock controller | `internal/dock/{hover.go,hover_test.go,controller.go,controller_test.go,types.go}` | Task 1 declarations |
| 6. Native Dock panel host | `internal/platform/darwin_dock_panel.{go,m,h}`, `dock_panel.go`, associated tests | Wails native feasibility; can parallel Tasks 2/4/5 |
| 7. React app/Dock surfaces and settings | frontend `app-switcher/*`, `dock/*`, `settings/*`, `lib/i18n.ts`, new `app-switcher.css`/`dock.css`; `overlay/EntryItem.tsx` only if extracting non-activating shared controls | Task 1 types; Tasks 3/5 event contracts |
| 8. Runtime/bindings/acceptance | `app.go`, `app_switcher.go`, `app_settings.go`, `app_prefs.go`, `app_dock.go`, `app_dock_window.go`, `app_apps.go`, `main.go`, frontend `App.tsx`, `lib/{bridge.ts,dock-bridge.ts}`, generated bindings, App/browser tests, docs | All earlier task reviews |

Task 1 completes shared declarations before Tasks 2–7 modify consumers. Run at most three independent implementation agents with the coordinator doing useful integration work. No agent writes another task's files. Task 8 owns all shared App/Main/bridge changes; other implementers report required wiring explicitly.

Initial allocation after the foundations checkpoint: coordinator completes/reviews Task 1, then assigns native app inventory (Task 2), native Dock observation (Task 4), and native panel proof (Task 6) to the three available workers while building the pure Dock controller (Task 5) locally. Prioritize the Task 6 native proof before investing in its UI. As workers finish, assign the app controller (Task 3) and React surfaces (Task 7); integrate only reviewed contracts in Task 8. This schedule keeps the unresolved native host constraint early and does not expand into later roadmap milestones.

## Task 1: migrate settings and compose real app groups

**Deliverable:** independently testable schema-3 migration and pure grouping/filter behavior, with the above contract types available to workers.

**Files:** ownership table row 1; new tests `internal/config/modes_test.go`, `internal/appgroup/groups_test.go`, `internal/filter/apps_test.go`.

**Interfaces:** `appgroup.Compose(apps []domain.App, raw, eligible []domain.Window) []appgroup.Group`, where `Group{App domain.App; Windows []domain.Window}`. Preserve order of already-filtered/ordered `eligible` windows; groups appear at their first eligible window. Append genuinely windowless apps in case-insensitive name order, PID tie-break. `filter.AppAllowed(app domain.App, rawWindowCount int, f config.Filters, scope config.ShortcutScope, ctx filter.Context) bool` applies self/blacklist/active-app/hidden rules without inventing window geometry.

- [ ] Add a migration fixture with schema 2, custom `KeyX` binding, explicit false controls and a custom Command+Tab chord record. Assert mode remains windows, values survive, app settings are independent, and Dock is disabled.

```go
func TestLegacyModeSettingsPreserveBindingsWithoutAliasing(t *testing.T) {
    s, err := Load(strings.NewReader(`{"version":2,"behavior":{"actionBindings":{"KeyX":"close"}},"shortcuts":[{"id":1,"chord":"command+tab","enabled":true}]}`))
    if err != nil { t.Fatal(err) }
    if s.Shortcuts[0].Mode != ModeWindows || s.Dock.Enabled { t.Fatal("legacy behavior changed") }
    s.AppSwitcher.Behavior.ActionBindings["KeyX"] = ActionMinimize
    if s.Behavior.ActionBindings["KeyX"] != ActionClose { t.Fatal("mode maps alias") }
}
```

- [ ] Run `cd apps/desktop && go test ./internal/config -run TestLegacyMode -count=1`; record the red result.
- [ ] Implement modes/defaults/migration and deep-copy helpers. `Settings.Preferences` returns a snapshot, including a cloned binding map. Add an explicit per-shortcut mode selector type to TS; missing incoming state mode falls back to windows for compatibility.
- [ ] Add a grouping regression with apps 10, 20, 30; raw windows for 10 and 20; eligible windows only for 10. Assert 10 and truly windowless 30 appear, while filtered-out 20 is not relabeled windowless. Add duplicate-name/PID and `HideWhenNoWindow` blacklist cases.

```go
func TestFilteredWindowsDoNotReappearAsWindowlessApps(t *testing.T) {
    apps := []domain.App{{ID:10,Name:"A"},{ID:20,Name:"B"},{ID:30,Name:"C"}}
    raw := []domain.Window{{ID:1,AppID:10},{ID:2,AppID:20}}
    got := Compose(apps, raw, raw[:1])
    if len(got)!=2 || got[0].App.ID!=10 || got[1].App.ID!=30 || len(got[1].Windows)!=0 { t.Fatalf("%+v",got) }
}
```

- [ ] Implement composition using raw per-PID counts, never title matching. Extend `filter` through the new app helper, preserving existing window-filter behavior. Test all app hiding values and active-app scope.
- [ ] Run focused config/appgroup/filter tests. Report red/green evidence and exact new field names. Coordinator reviews and commits this contract checkpoint before consumers start.

## Task 2: enumerate and activate windowless native apps

**Deliverable:** real `ApplicationSource` and `ApplicationActivator` capabilities with honest stub behavior.

**Files:** ownership table row 2. Test seams are package-local; no new required `Platform` methods.

- [ ] Add tests for filtering native inventory records: regular non-terminated bundled apps survive, self/prohibited/accessory processes do not; a regular app with zero windows survives. Use records supplied to a pure Go mapping helper in `darwin_apps.go`.
- [ ] Run `go test ./internal/platform -run 'TestApplicationInventory|TestApplicationActivation' -count=1` and record red.
- [ ] Implement inventory from `NSWorkspace.sharedWorkspace.runningApplications`, carrying PID/name/bundle ID/hidden. Execute AppKit-dependent snapshot creation on the main thread with a main-thread fast path; return owned JSON/string data to Go. Never synchronously dispatch to main from main.
- [ ] Implement `ActivateApp(pid)` using a fresh `NSRunningApplication` identity check and the OS's application activation request; reject invalid/self/terminated/non-regular identities and propagate refused activation. No global key injection and no auto-New-Window on selection. Native acceptance means OS request accepted, consistent with foundations actions.

```go
// Go-side validation before entering native code.
if id <= 0 || int64(id) > math.MaxInt32 || int(id) == os.Getpid() {
    return errors.New("invalid or self application identity")
}
```

- [ ] Provide `!darwin` optional methods returning explicit unsupported errors. Do not present synthetic windows as a real running-app inventory.
- [ ] Run focused mapping/identity/refusal tests and native build. Perform a safe disposable-fixture smoke: a small test app stays running after its only window closes; inventory retains its PID; ActivateApp returns acceptance and macOS reports that exact foreground PID. Terminate only that fixture after the check.

## Task 3: app-mode controller with stable explicit targets

**Deliverable:** keyboard selection cycles applications once each, exposes the selected application's eligible windows, and commits a real window or windowless app correctly.

**Files:** ownership table row 3.

**Interfaces:** extend `Deps` with optional `Apps platform.ApplicationSource` and `AppActivator platform.ApplicationActivator`. Add `SelectApp(appID domain.AppID)`, `SelectAppWindow(windowID domain.WindowID)`, `ConfirmApp(appID domain.AppID) error`. Make `Confirm() error` return action failure; statement calls may ignore the return, but `HandleHotkey` forwards it to optional `interface{ ActionFailed(string) }` on the view. App-mode commit invokes the selected exact window when available; windowless commit invokes only `ActivateApp`. The coordinator implements error event forwarding. `State` semantics are defined above.

- [ ] Add semantic tests with two windows of app 10, one of 20, and windowless app 30. Command+Tab app mode yields three app entries; Advance/Reverse visit apps rather than windows; window mode remains three independent windows. Create fixture settings with explicit modes so `Default()` changing on new install does not silently change legacy window tests.

```go
// After configuring an app-mode shortcut and activating it:
st := c.State()
if st.Mode != config.ModeApps || len(st.Apps) != 3 { t.Fatalf("%+v", st) }
c.SelectApp(30)
if len(c.State().Entries) != 0 || c.State().SelectedWindowID != 0 { t.Fatal("invented a window") }
if err := c.ConfirmApp(30); err != nil { t.Fatal(err) }
if !reflect.DeepEqual(activated, []domain.AppID{30}) { t.Fatalf("%v", activated) }
```

- [ ] Run focused app-controller tests red, then implement a separate `apps.go` composition branch. Keep existing `list/baseList` behavior for windows. Store selection by app PID and selected preview by real window ID, never app display name.
- [ ] Apply existing window filters/order before grouping; apply app blacklist/hidden/active-app rules to actual inventory. For app mode, preserve group selection on refresh when its PID survives and clamp safely after quit/close. Fuzzy search matches app names and eligible window titles and retains exactly one app entry.
- [ ] Use `Settings.Preferences(mode)` for all controller behavior and appearance reads, including HoldToCycle, placement/order, pointer/action controls, haptics/cursor following. `Shortcut.StyleOverride` applies only to its own active mode and never changes grouping. Add tests showing different settings in each mode.
- [ ] Make commit fresh-target checks reject a vanished or changed owner. Failure leaves the session available to show a visible error; success touches window MRU when there is a window and dismisses. A windowless app never receives window action ID zero. `ConfirmApp` resolves the supplied app inside the current list under the controller lock, rather than relying on a separate Select call.
- [ ] Add interleaving tests for close/quit while selected, delayed action completion and a reopened session. Capture the opening session generation for commits; an old completion must not dismiss a newly opened switcher. Preserve window-mode tests and run `go test -race ./internal/switcher -count=1` once after focused tests pass.

## Task 4: cancellable native Dock AX observer

**Deliverable:** actual app-icon identity, bounds, Dock edge and monitor under the real pointer, with restart/Space/display recovery and bounded callbacks.

**Files:** ownership table row 4. Do not modify the existing global hotkey event tap.

- [ ] Add a native-observation mapping seam and tests for duplicate app names, non-app Dock subroles, ambiguous bundle URL matches, dead Dock generation, negative monitor origins and mixed display scales. Reuse `domain.Bounds` points.
- [ ] Run focused observer tests red. Implement `ObserveDock(ctx, emit)` with one owned serial worker/timer and a one-slot coalescing Go delivery channel. The callback only delivers immutable observations; it never waits on enumeration/UI/capture.
- [ ] Locate `com.apple.dock` through running-app bundle identity, create its AX application element and retain its PID as the observer generation's owner. On a 50 ms tick, read pointer position; perform AX hit testing only near a display's possible left/right/bottom Dock edge or the current known Dock bounds. A 160-point edge gate accommodates magnified icons. Outside those regions emit cheap pointer-only updates without AX lookup; the Go controller decides whether a panel needs them. Do not query every desktop app.
- [ ] Hit-test the system-wide AX object with `AXUIElementCopyElementAtPosition` and require the returned element's owning PID to equal the current Dock PID. Walk at most six parents looking for `kAXDockItemRole` plus `kAXApplicationDockItemSubrole`; reject folders, files, trash, separators and minimized-window items for this milestone. Set AX messaging timeout to 0.05 s and bound the whole classification walk to 0.15 s. An exhausted/failed query returns an empty item, not a guessed target.
- [ ] Read typed `AXURL`, position, size and title attributes. Normalize a file URL's standardized/symlink-resolved app bundle URL and match against live `NSRunningApplication.bundleURL`; use bundle ID only when it uniquely identifies one live regular app. Never resolve by display title. If multiple processes still match, emit no actionable item and a diagnostic; do not choose the first PID. A pinned app without a live process may produce AppID zero for a non-actionable “App is not running” state.
- [ ] Convert native AX/CG points directly into the domain's top-left coordinate system. Pick the display containing the icon center, falling back to largest icon/display intersection. Infer left/right/bottom from icon geometry and visible Dock container geometry on that display. Read the Dock orientation preference only as a tie-breaker; never write it. Do not assume primary display, integer scale, or a bottom-only Dock.
- [ ] Register NSWorkspace launch/termination and active-Space notifications on the workspace notification center; display-parameter and wake notifications on their proper centers. Their callbacks only mark state dirty. Attach AX destroyed/moved/resized/children notifications when supported; notification refusal does not disable timer fallback. Recreate all retained Dock AX references after PID change, invalid-element failure, or wake; increment generation, emit invalidation, and clear geometry before rediscovery. Retry unavailable Dock/permission at most once per second without prompting from the worker.
- [ ] Cancellation removes notification registrations, run-loop sources/timer, retained AX objects, pending callbacks and registry entry. Every later observation carries generation/sequence and is rejected by a cancelled Go session. Test cancellation during classification through a seam; `!darwin` returns unsupported.
- [ ] Native smoke records only role/subrole/PID/bundle/bounds/edge for a hovered disposable app icon. Verify all Dock positions, auto-hide visibility transitions, Dock migration to a second monitor, and Dock restart using an explicit isolated QA step. Restore any QA-only Dock position/autohide changes before finishing. Native input is never swallowed by this observer.

## Task 5: Dock controller, filters and pointer continuity

**Deliverable:** deterministic hover timing, live window refresh, exact target state, and stale-generation suppression independent of Wails.

**Files:** ownership table row 5.

**Interfaces:** `dock.NewController(deps Deps, settings config.Settings) *Controller`, `Run(context.Context)`, `Configure(config.Settings)`, `Suspend(bool)`, `SetPanelBounds(session uint64, bounds domain.Bounds)`, `SelectWindow(session uint64, id domain.WindowID)`, `Refresh(session uint64)`, `Dismiss(session uint64)`. Calls enqueue messages to one owner loop. `Deps{Observations platform.DockObservationSource; Windows platform.WindowSource; Env platform.Environment; View View; SelfBundleID string}`. `View` receives `Show(State)`, `Update(State)`, `Hide(session uint64)`. `State{Session uint64; Item Item; Windows []domain.Window; SelectedWindowID domain.WindowID; Bounds domain.Bounds; Appearance config.Appearance; EmptyReason string}`; serialized view DTO is produced by the App adapter. `EmptyReason` is `""`, `"noWindows"`, `"filtered"`, or `"notRunning"`.

- [ ] Add tests using timestamped observations: 300 ms hover threshold, leave before threshold, 250 ms dismissal, icon→corridor→panel crossing, reversing back into icon, another icon's independent delay, and native generation changes while a query is pending. Extend `Hover.Step` to emit an explicit `Move *Item` transition when the shown icon's bounds/screen/edge changes.

```go
func TestShownIconMovementRepositionsWithoutNewHoverDelay(t *testing.T) {
    h := NewHover(0, 250*time.Millisecond)
    at := time.Unix(1,0)
    item := Item{Kind:"app",AppID:10,Bounds:domain.Bounds{X:100,Y:700,W:50,H:50},Edge:"bottom"}
    if h.Step(at,&item,false).Show == nil { t.Fatal("not shown") }
    item.Bounds.X = 180
    change := h.Step(at.Add(time.Millisecond),&item,false)
    if change.Move == nil || change.Show != nil { t.Fatal("must reposition existing panel") }
}
```

- [ ] Run `go test ./internal/dock -count=1` red, implement pointer continuity using the icon bounds, actual panel bounds, and a narrow corridor joining the nearest icon/panel edges, inflated by BridgePaddingPx. Do not use the entire bounding-box union as a hover region. HoverSlopPx only tolerates pointer jitter during the opening delay; it must not keep a panel open after pointer escape.
- [ ] Observe and enumerate on separate workers. Opening an icon schedules window enumeration; a 500 ms refresh runs only while shown. Results carry native generation and controller session; drop them if stale, suspended, disabled, or pointed at another app. Coalesce refresh demand so only one window query is in flight. The input owner loop never waits on AX or window enumeration.
- [ ] Restrict raw windows to the exact hovered AppID, then apply global exclusions/visibility and Dock scope with current context. Reuse `filter.Apply` and `order.Sort/SendToBack`; use `AppAllowed` for empty states and blacklist `whenNoWindow`. An app hidden by exclusions has no panel; an allowed app whose windows fail scope gets a “No windows match these filters” state. No capture target is generated for an empty state.
- [ ] Compute bounded panel size from card count, Dock-specific appearance, selected display usable bounds and `PlacePanel`. Use overflow scrolling rather than oversized windows. Reset immediately on Dock-generation/Space/display invalidation and after suspension; require a fresh hover before reopening after preferences/switcher closes.
- [ ] Add tests for stale enumeration after Dock restart, display removal while shown, filter updates, permission loss, zero-delay values, same-icon movement, empty/minimized apps and shutdown. Run the focused controller/geometry tests and then package race test once.

## Task 6: genuine non-activating native Dock panel host

**Deliverable:** a bounded NSPanel with working Wails controls that neither steals foreground app activation nor intercepts clicks outside its bounds.

**Files:** ownership table row 6; coordinator later supplies the hidden Wails host and lifecycle wiring.

**Interfaces:** new `platform.DockPanelHost` capability:

```go
type DockPanel interface {
    Show(domain.Bounds) error
    Hide() error
    Close() error
}
type DockPanelHost interface {
    CreateDockPanel(host unsafe.Pointer) (DockPanel, error)
}
```

`host` is the retained native NSWindow of a dedicated hidden Wails `/#/dock` webview. The native adapter owns its panel, not Wails' host window. Calls are safe from any Go goroutine and marshal AppKit mutation to main with a main-thread fast path. An invalid/destroyed host returns an error. Close is terminal and idempotent.

- [ ] First add lifecycle tests against an injected native-host seam: Show after Close fails; repeated Hide/Close do not send messages to retired objects; a stale close callback cannot retire a new host. Run those tests red.
- [ ] Implement an `NSPanel` subclass initialized with borderless and nonactivating-panel style at creation, `canBecomeMainWindow = NO`, `canBecomeKeyWindow = NO`, floating level, no activation on display, non-movable background, and auxiliary/all-Spaces collection behavior suitable for the active Space. Show with `orderFrontRegardless`, never `makeKeyAndOrderFront` or application activation.
- [ ] Transfer the dedicated Wails host's retained content view to the native panel while keeping the original Wails host/delegate and WKWebView alive for bindings and event dispatch. The original host stays hidden throughout. Resize the hosted content view to panel bounds so its actual web viewport tracks the panel. On Close, detach/restore the retained content to a still-live host before releasing native panel ownership. Do not swizzle Wails' NSWindow class or mutate the module cache.
- [ ] Run the native proof before broad UI integration: render one Dock button, click it while a disposable text app is foreground, receive an exact-target Go bridge call, and assert foreground PID and focused text window did not change. Assert a click immediately outside panel bounds reaches the underlying fixture and viewport geometry matches CSS pixels on Retina and a second display. Repeated show/hide/resize/close must not crash Wails or leak a visible host.
- [ ] Treat failure of that native proof as implementation work remaining. It cannot be downgraded to “hide Option Tab again after activation,” because that loses D03 behavior. Resolve the host integration within these owned files before Task 8 marks D03 complete; record any necessary dependency-source change for coordinator review instead of silently editing Wails.
- [ ] Implement screenshot-free native smoke output for panel frame, host frame/visibility, foreground PID before/after and runtime callback count. Report lifecycle tests and native evidence. Native material styling remains the separate B10/C04 follow-up; this panel can use the existing CSS solid/transparent appearance in this milestone.

## Task 7: app switcher, Dock preview route and settings controls

**Deliverable:** independently tested rendered surfaces with explicit app/window targeting and separate settings.

**Files:** ownership table row 7. Coordinator owns `App.tsx`, bridges and generated bindings.

**Interfaces:** `AppSwitcher({state, handlers})`, where `state` uses the extended `SwitcherState`; handlers add `onSelectApp(appId)`, `onSelectAppWindow(windowId)`, `onConfirmApp(appId)` to existing explicit actions. `DockPanelView({state, handlers})` receives `DockViewState{session,item,entries,selectedWindowId,appearance,emptyReason}`; handlers expose `onSelectWindow(session,id)`, `onFocusWindow(session,id,appId)`, `onAction(session,kind,id,appId)`, and `onSize(session,width,height)`. Session is checked by the backend; local closures alone are insufficient.

- [ ] Add failing rendered tests with duplicate app names/different PIDs: exactly one icon per app; selecting an app shows only its windows; clicking a preview sends its exact window ID/PID without an earlier Select dependency; windowless apps have an empty state and app-level actions only.

```tsx
expect(screen.getAllByRole("option", { name: /Example/ })).toHaveLength(2);
await user.click(screen.getByRole("button", { name: "Focus document B" }));
expect(handlers.onFocusWindow).toHaveBeenCalledWith(7, 102, 10);
expect(handlers.onAction).not.toHaveBeenCalled();
```

- [ ] Run focused Vitest red, then implement an app-icon strip plus selected-app window gallery. The strip uses stable PID keys; window cards use window IDs. Tab/Shift+Tab and the configured primary-axis keys cycle apps. Orthogonal-arrow movement selects windows within the selected app, and Enter/modifier release commits the chosen window or windowless app. Native key forwarding and browser fallback consume the same handlers.
- [ ] Disable window-only controls and window-bound mappings when SelectedWindowID is zero. Keep Hide/Quit/Force Quit/New Window with a valid running AppID; New Window errors remain visible. App mode supports separate theme/layout/size/preview/control behavior and existing action bindings.
- [ ] Build the Dock route as a bounded panel with real window cards, exact focus/action buttons, visible error/partial-result notices and localized empty states. No search input or keyboard interception is required for this hover-only milestone. Do not attach the switcher's full-screen backdrop or global key handler. Use ResizeObserver to report content size with the current session; stop reporting on unmount.
- [ ] Add settings mode selection on each shortcut, a “Window switcher / App switcher” selector for Appearance and switcher controls, and a Dock tab with enable/delay/tolerance/scope/size/control settings. Global behavior controls stay in General. The explicit Dock enable path invokes existing permission guidance; background observers do not prompt. Preserve explicit disabled bindings and independent unsaved edit snapshots.
- [ ] Add English/Brazilian Portuguese/Spanish strings and accessibility labels for new UI. Add tests proving a mode edit does not mutate the other mode and Dock enable does not alter Command+Tab. Run focused UI/settings/keymap tests and typecheck against the declared contract; report coordinator-required imports and wiring.

## Task 8: runtime integration, shared capture and acceptance

**Deliverable:** combined native app that exercises the tested controls, not only browser fixtures.

**Files:** ownership table row 8. Create `app_dock_test.go`, `app_apps_test.go`, and browser specs under the existing e2e test directory.

- [ ] Add App integration tests red for: switcher Show dismisses/suspends Dock; Dock never reopens while switcher/preferences is active or app is paused; settings disable cancels observation; shutdown is terminal; stale Dock session action/size callbacks are rejected without native dispatch.
- [ ] Wire optional native app dependencies and `App.ConfirmApp/SelectApp/SelectAppWindow`. Forward hotkey/app activation errors through `switcher:error` to the existing visible error notice. Enrich app icons independently of window entries, including windowless app PIDs. Update selected capture-ID extraction for app-mode State.
- [ ] Create the hidden Wails Dock host in a factory, route `/#/dock`, and a retained runtime wrapper in `app_dock_window.go`. The wrapper owns `DockPanel` and `liveWindow`; it tracks native close/host close and recreates both coherently. Do not call Wails Show on the host. Native panel geometry is authoritative.
- [ ] Add `App` Dock bound methods with session validation: `SelectDockWindow(session,id)`, `FocusDockWindow(session,id,appId) (actions.Result,error)`, `PerformDockAction(session,kind,id,appId) (actions.Result,error)`, and `SetDockPanelSize(session,width,height)`. Require the target to belong to the current panel's displayed app/windows, then use the existing action service. Focus closes only the same successfully acted-on panel session; non-focus actions refresh that session and preserve native save dialogs. A delayed result cannot hide a replacement panel.
- [ ] Keep one visible `preview.Manager`. Under the App lifecycle mutex, hide/cancel the old session before changing its routing owner, selected ID and requested windows. Switcher wins arbitration; Dock is suspended before switcher admission. The manager's unchanged four-slot semaphore prevents newly admitted jobs from exceeding the budget while old cancellation finishes. Background snapshots skip both visible surfaces and preferences/pause. Do not hold the App mutex from frame emission callbacks.
- [ ] Emit Dock-specific `dock:show`, `dock:update`, `dock:hide` and `dock:frames` events. Every Dock payload includes the session ID; frames contain `{session, frames: map[string]string}`. Frontend rejects old-session state/frames/errors. Preserve existing switcher thumbnail/preview payloads. On switching surface, invalidate the prior manager jobs before changing event routing to avoid labeling old captures as new-surface output.
- [ ] Subscribe App runtime to Dock state on startup only when enabled and supported. Reconfigure on settings changes; suspend/dismiss on preferences, pause, screen lock/session resignation, and keyboard switcher opening. Return to observation after resume with a fresh hover delay. Terminal shutdown closes panel/controller/observer/capture and makes queued UI callbacks harmless.
- [ ] Regenerate bindings with `cd apps/desktop && wails3 generate bindings`, then update browser fakeWails method mappings and explicit-result fixtures. Add browser tests for app grouping, windowless controls, independent settings, Dock empty states, clicked target preservation and visible failures.
- [ ] Run the combined required `task lint`, `task test`, `task build`, and `task e2e` once after focused fixes. Obtain independent reviews for controller/migration, native observer/panel lifecycle, and frontend/bridge integration; fix concrete findings with focused regression checks.
- [ ] Execute the native acceptance matrix below and record evidence/limitations in the milestone report. Coordinator marks individual roadmap IDs only when their native criteria pass, then commits/pushes the branch checkpoint. No version bump or feature release is part of this task.

## Native acceptance matrix

Use a disposable app fixture with two distinct titled windows plus a second fixture that remains running without windows. Log fixture PIDs/IDs and capture only those fixtures. Do not close or force quit unrelated user applications.

| ID | Required actual behavior | Evidence |
|---|---|---|
| B01 | Configured Command+Tab app mode displays each running app once; chosen app shows its windows; cycling then release focuses the exact selected fixture window | Native key path, app/window IDs, short recording |
| B02 | Close a fixture's last window while leaving it running; it remains in app mode, window controls disappear, selection activates that exact PID | Inventory before/after and foreground PID |
| B03 | Change app layout/binding/hold behavior, switch back to windows, and verify its previous controls/appearance remain; schema-2 import preserves original shortcuts | Native settings round trip plus migration tests |
| D01 | Hover a real Dock app icon and display only its filtered windows after configured delay | Actual AX icon identity and panel recording |
| D02 | Click second preview after selection has changed; focus exact window on same and another Space | Window ID and foreground/focused-window observation |
| D03 | Close/minimize from hover panel without first activating the fixture or Option Tab; native refusal/save dialog remains truthful | Foreground PID before/after; action result; panel button bridge |
| D04/D05 | Change delays; cross icon→gap→panel and back; leave both; move to another icon; panel behavior matches configured timing | Timestamped observation/transition trace and visual check |
| D06 | Left/bottom/right Dock, auto-hide, negative-origin second display, mixed scale; panel stays on icon's display and doesn't intercept outside clicks | Frames in global points plus native pointer checks |
| D07 | Global app blacklist, no-window blacklist, title/minimized/hidden/fullscreen/Space/screen filters apply to Dock | Native fixture combinations and state payloads |
| D08 | Restart Dock while panel is pending/open; change Space; connect/disconnect display; regain permission after denial; stale panel/captures disappear and fresh hover reconnects | Generation/PID transitions; no stale callbacks; recording indicator returns idle |
| Shared capture | Rapidly alternate Dock and keyboard surfaces; at most four native streams; no hidden capture when preference disabled; no sustained RSS/stream growth after 30 cycles | Native stream counts/registry cleanup, CPU/RSS baseline and settled sample |

Dock orientation/auto-hide/restart tests are explicit QA operations for the user's authorized native feature work. Record and restore any temporary system preference values used by the tester; the product never changes them. If the hardware lacks a second display, record D06/D08 multi-display acceptance as incomplete rather than claiming success from geometry tests.

## Primary API references and implementation cautions

- AX hit testing uses top-left screen coordinates and can report invalid/not-implemented elements; use typed error handling and PID validation. [Apple: AXUIElementCopyElementAtPosition](https://developer.apple.com/documentation/applicationservices/1462077-axuielementcopyelementatposition)
- AX notifications are not guaranteed supported, and the system-wide accessibility element does not support observer notifications; observe the Dock application when possible and keep bounded polling fallback. [Apple: AXObserverAddNotification](https://developer.apple.com/documentation/applicationservices/1462089-axobserveraddnotification)
- Active-Space notifications must use the NSWorkspace notification center. [Apple: activeSpaceDidChangeNotification](https://developer.apple.com/documentation/appkit/nsworkspace/activespacedidchangenotification)
- NSPanel provides the auxiliary panel behavior needed for this native surface. The specific Wails content-host integration is this plan's implementation design and requires the native proof in Task 6. [Apple: NSPanel](https://developer.apple.com/documentation/appkit/nspanel)
- Local SDK `AXRoleConstants.h` declares `kAXApplicationDockItemSubrole`; `AXAttributeConstants.h` declares AXURL/position/size. Exact attributes exposed by the running Dock are OS-dependent and must be probed in the native smoke. This plan does not assume a Dock hierarchy depth or localized icon title is a stable identity contract.

## Plan self-review

- B01–B03 map to Tasks 1–3/7/8; D01–D08 map to Tasks 4–8 and the native matrix. Later roadmap work remains outside this milestone.
- File ownership separates contracts, native inventory, native observation, native panel, pure controllers, React UI and root wiring. No worker owns shared App/Main/bindings except the coordinator.
- Every identity action uses a PID/window ID and fresh native validation; no app-name dispatch or fake zero-ID window exists.
- Observer generation, panel session and terminal shutdown are distinct concepts and each has explicit rejection checks.
- NSPanel feasibility and real hardware acceptance are gates, not mocked-success substitutes. Legacy settings, global behavior and four-stream admission remain protected.

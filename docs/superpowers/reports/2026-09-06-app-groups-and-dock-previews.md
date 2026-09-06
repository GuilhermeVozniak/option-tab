# App groups and native Dock previews

Branch: `feat/dockdoor-parity`. Baseline: foundations checkpoint `0acace5`.
Status: implementation checkpoint; required automated checks pass, native acceptance is partial. This is not a release and does not complete the remaining roadmap.

## Behavior

- Shortcuts can open either windows mode or apps mode. New settings use app grouping for Command+Tab; imported shortcuts with no mode retain windows mode. App mode groups by running PID and shows the selected app's eligible windows. Running apps without usable windows remain explicit app targets.
- App and window modes have independent appearance, interaction, ordering, and placement preferences. Global lifecycle preferences remain shared.
- Dock previews are opt-in. A native Accessibility observer resolves the hovered app icon, its identity, bounds, display, and edge. The owner loop applies delays, movement tolerance, filters, and the icon-to-panel corridor.
- A dedicated hidden Wails host supplies content to a nonactivating NSPanel. Its native lifetime is independent of the keyboard overlay. Cards retain explicit window/PID targets for focus and management actions.
- Both surfaces share the bounded preview manager. Changing owner first drains prior emissions. Frames and UI events retain presentation identity; stale delivery cannot restore an old panel or image. Native keys are tagged before queueing and checked again before frontend handling.
- Window lookup and native dispatch are separate boundaries. Actions recheck presentation admission after lookup, and bulk actions repeat this check for each target. Work already dispatched to another application cannot be recalled.

## Corrections found by integration and review

- Native screen JSON boxed a C comparison as `0/1`, which failed Go boolean decoding and caused display ID 0 fallback. Explicit native booleans restore real display IDs.
- A coalesced suspend/resume could erase Dock invalidation. A synchronous admission epoch now survives mailbox coalescing and guards pending queries, views, and actions.
- An old switcher dismissal could hide a reopened presentation. Dismissal is now token-aware through both controller and native App layers.
- Wails dispatches events asynchronously. Monotonic revisions and session tombstones protect show/update/hide delivery; image packets and native keys carry their original session.
- Background snapshots completed after a presentation change could replace the idle cache. Publication now checks the view, Dock, and desktop-session epoch.
- Shutdown previously left controller action admission open. Terminal controller stop cannot be undone by a late session-resume callback.
- WebKit suppresses normal hover updates in a panel that cannot become key. The Dock observer now publishes scoped panel-local pointer coordinates; the frontend resolves the actual card under that point without making the panel active. Pointer sequence numbers are independent of presentation revisions, and late webviews can recover the latest pointer from their state snapshot. The combined native test verified hover selection and actual pointer down/up/click on the second card’s Close button with the other fixture still foreground at action dispatch.
- App-mode appearance now applies title-list, icon, thumbnail, selected-preview, layout, sizing, controls, swipe and middle-click preferences. Dock appearance uses its own complete editor and bounded row viewport. Title-list mode retains window names when thumbnail-title labels are disabled.

## Native evidence reached

- Application inventory retains a disposable running app after its last window closes and activates its exact PID.
- The standalone panel fixture verifies a real Wails button-to-Go callback without changing the foreground app or its focused window, continued typing into the foreground fixture, outside clicks, bounded viewport sizing, and repeated host/panel teardown and recreation.
- The Dock observer resolves a disposable app's actual Dock icon, PID, bundle URL, and bounds across repeated start/cancel lifetimes, with no callbacks after cancellation returns.
- The combined harness exercised real app grouping and gallery rendering, AX Dock hover through the real controller/App/frontend, and exact second-preview focus. Native pointer coordinates selected the second card and revealed its controls even though WebKit’s `:hover` list remained empty. An actual native button click dispatched Close to that exact window/PID while a different disposable app was foreground at dispatch. Fixture close notifications and AX removal confirm Close succeeded; the post-close foreground assertion occurs after the failed stale-inventory check and was not reached.
- An early harness probe imported a second Wails runtime and replaced its event dispatcher, preventing React from receiving native events. The probe now uses its isolated reporting endpoint without replacing the production runtime. Those earlier hover failures do not establish a production transport defect.

The combined harness restricts inventory and actions to its own fixtures. Its exact two-document filter also excludes auxiliary CG/AX surfaces, including observed 66×20 AX dialogs whose origin has not been established. That filter is a harness safety boundary, not proof that production auxiliary-window classification is complete.

Repeatable native runners: [app inventory](../../../apps/desktop/internal/platform/testdata/run_app_inventory_smoke.sh), [panel lifecycle](../../../apps/desktop/internal/platform/testdata/run_dock_panel_smoke.sh), [combined App/Dock flow](../../../apps/desktop/testdata/app_dock_smoke/README.md).

The combined runner does not produce an overall PASS: its stale-window inventory assertion remains red in both the deliberately retained and owner-release fixture variants. Dropping the owner array reference did not establish actual `NSWindow` deallocation. The last-document-close/windowless activation stage was therefore not reached in this combined runner; the separate inventory fixture evidence remains distinct.

## Limits and outstanding acceptance

- Both available displays report scale 1. Mixed-scale/Retina placement, physical display disconnect, and negative-origin combinations require additional hardware validation.
- Current native evidence covers the bottom Dock and observed auto-hide behavior. Pure geometry tests cover all edges; physical left/right Dock changes and an actual Dock-process restart have not yet been established by the combined harness.
- Screen/session observation uses public workspace signals and best-effort native lock hints. Actual lock/wake, permission revocation/regrant, and startup while locked remain distinct acceptance checks.
- AX-empty plus successful CG inventory with no owned surfaces establishes windowlessness. Ambiguous CG-only surfaces or failed AX lookup remain “Windows unavailable”; they do not satisfy a no-window blacklist rule.
- Complete bulk discovery across all apps/Spaces remains a foundations limitation.
- A fixture that deliberately retains its closed `NSWindow` confirms a separate C06 limitation: Close succeeds and the window leaves `AXWindows`, but its titled CG layer-0 surface remains with `OnScreen=false`. The current CG inventory can keep that stale entry. A generic offscreen filter would also remove legitimate other-Space windows, so this case is not hidden by filtering. Existing positive AX-destruction handling stops capture and clears frames but does not yet exclude the identity from window enumeration.
- In that retained-window native run, live capture peaked at 2 streams, delivered 136 frames, and returned to 0 active streams after shutdown. This proves bounded positive capture/cleanup for that run, not the separate 30-cycle/resource-growth acceptance matrix.
- Dock arrangement, sizing and appearance are independent. Inter-card spacing still uses a fixed value; C04 remains incomplete until it has its own setting.
Browser tests use mocked Wails events and do not establish native nonactivation or physical input behavior.

## Required checks

- `task lint`: passed, 0 issues.
- `task test`: passed, 234 desktop frontend tests, 5 shared tests, 3 website tests, and all 17 Go packages with race detection.
- `task build`: passed, frontend production bundles, generated Wails bindings, and embedded macOS Go binary.
- `task e2e`: passed, 50 desktop Chromium tests and 4 website Chromium tests.
- Existing SDK deployment-target/Carbon and formatter-configuration warnings remain; this milestone does not establish the separate macOS distribution compatibility requirements in G01–G03.

The whole-tree Go gate also included the isolated next-milestone pure Aero Shake reducer. It has no runtime integration and is excluded from this milestone commit.

The authoritative retained scope remains [the roadmap](../../dockdoor-roadmap.md). Dock input/drag, folders/media, automation, distribution, and optional Dock replacement remain later work; sibling-product exclusions continue to apply.

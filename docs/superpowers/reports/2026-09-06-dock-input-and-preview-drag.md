# Dock input, preview drag and Aero Shake checkpoint

This checkpoint adds the retained D09–D14 workflows and the remaining Dock card-spacing control from C04. Input features are opt-in and default to disabled. It follows the app/Dock checkpoint and the positive closed-window retirement fix `53009b3`.

## Implemented behavior

- Dock icon left-click can hide the exact app. Vertical scrolling shows/hides it, with wheel/precise-scroll thresholds and momentum handling. Command-right-click quits; Command-Option-right-click force quits. Native ownership requires a fresh, explicit actionable icon identity and excludes Finder, Option Tab, folders and ambiguous items.
- Preview swipes map to four configurable directions relative to the Dock edge. Native `NSEvent` precision/phase information drives recognition. The frontend publishes clipped card geometry, and each gesture retains the original window/PID rather than following later selection.
- Preview dragging uses an explicit pending owner before lookup, a normalized grab point and a position-only native setter. Early release, configuration changes, inactivity and shutdown cancel preparation. An accepted drag survives ordinary hover dismissal. A recreated webview receives the current gesture floor; terminal pointer sequences cannot restart a drag or focus the window accidentally.
- Aero Shake requires correlated pointer and actual AX window movement, four qualifying reversals and bounded time/path thresholds. Initial stationary drag-threshold samples can restart evidence without losing the exact root. A discontinuity invalidates older action evidence.
- Minimize/close-other-window actions preserve the kept root, Option Tab instances, dialogs, sheets, nonstandard surfaces and modal/sheet parents. Unknown standard-root relationships are reported and never treated as eligible. Minimize uses a setter, so it cannot restore an already minimized window. Partial failures retain successful counts.
- Dock card spacing is independent of switcher appearance, accepts zero and ranges from 0–24 points. Input controls and drag guidance extend English, Brazilian Portuguese and Spanish settings.

## Admission, ownership and errors

Native callbacks perform bounded copying and suppression decisions. AX queries, actions and UI emission happen on workers. Source replacement cancels and joins the previous native lifetime, then resnapshots coalesced settings. Local source epochs and native generations reject delayed callbacks.

Icon and panel input retain at most one pending action token until acknowledgment. New gestures pass through while that action is pending. A normal finger/click end does not cancel an accepted asynchronous action, and terminal no-intent gestures release admission. Momentum tails cannot repeat the action. Pure preview recognition remembers retired gesture IDs across cancellation, termination, idle expiry and malformed samples.

Action guards recheck admission after external validation. Settings changes invalidate pending icon actions and cancel preview-drag ownership, including filter-only changes after eligibility lookup. For Aero Shake bulk actions, the native performer invokes the guard after its target/role/button lookups, immediately before AX mutation. It preserves the exact guard refusal and stops the remaining bulk work. An AX operation already sent to another application cannot be recalled.

Runtime permission loss, invalid taps and failed re-enabling terminate and join observation, report an error and use bounded retry. Preview drag also checks button state and screen topology during polling and before writes; a missed mouse-up cancels instead of leaving capture active. Icon, swipe, drag and shake failures have independent status slots, so one source recovering does not erase another source's failure.

## Automated verification

- Frontend/shared/site unit suites: 242 desktop, 5 shared and 3 site tests pass.
- All 17 Go packages pass with race detection and coverage enabled.
- Biome and golangci-lint report no issues.
- Frontend/site production builds and the embedded desktop Go build pass.
- Chromium suites: 55 desktop and 4 site tests pass. The new browser cases cover clipped region RPC geometry, disabled ordinary focus, drag threshold/identity/global and normalized coordinates, gesture-floor seeding, cancellation, Escape, trailing click suppression and no hover rearming after completion.
- Native callback/lifecycle seams cover event extraction, pass-through and owned streams, pending-action backpressure, replay of an abandoned original down, momentum, overflow, cancellation, source replacement, permission/tap failure and final guarded refusal. These seams do not post events or establish physical-device behavior.

## Native fixture evidence

The [window-drag fixture](../../../apps/desktop/internal/platform/testdata/window-drag/README.md) exercises actual remote AX roots in disposable bundled apps. Fresh cases prove exact eligible-window minimize and close, a final native guard callback refusing with zero mutation, and preservation of the kept parent and visible attached sheet. The foreground app remains unchanged; every fixture process is joined and cleaned up.

Positive child traversal identifies the sheet as `AXSheet`, with its own exact window ID and parent ID, without calling it an AX root. Its unsupported modal attribute remains explicitly unknown. The confirmed sheet is preserved regardless; standard-root candidates still need complete relationship evidence.

The [preview-position fixture](../../../apps/desktop/internal/platform/testdata/preview-drag/README.md) exercises the production AX preparation, position setter and cancellation against a remote disposable window. It confirms an exact +80/+60-point movement, unchanged 320×272-point window size, an unchanged second window, refusal of later writes after explicit cancellation/button release, and unchanged foreground through cleanup. Only the standalone test translation unit substitutes button state; it forbids tap creation and event posting. It never invokes `DragPreview`/native capture start, so this is setter evidence rather than end-to-end drag acceptance.

The earlier native icon receiver smoke verifies real tap installation and synthetic pass-through to a disposable receiver. It does not establish qualified physical icon gestures.

## Remaining native acceptance

Physical qualified Dock clicks, wheel/trackpad feel and momentum, real WK off-panel drag handoff, AX-sampled physical shaking/text-selection rejection, and end-to-end permission/lock/display transitions still need acceptance. Precise `NSEvent` scroll phases do not reliably expose an exact finger count. Off-Space AX roots or unsupported fullscreen/position attributes may be refused explicitly. Mixed-scale geometry and broader third-party AX compatibility remain release checks.

These limits keep D09–D14 acceptance open on the [retained roadmap](../../dockdoor-roadmap.md). D15, folders/media, automation, distribution and the optional replacement Dock remain part of the ongoing work. Features assigned to sibling products stay excluded.

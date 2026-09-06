# Dock monitor protection checkpoint

This checkpoint implements opt-in monitor protection for retained D15. The available product path uses manual Dock placement. Automatic placement remains unavailable while its native return behavior is investigated; the UI does not offer an unusable Move control.

## Available behavior

- Enable protection independently of Dock window previews. Select a persistent display UUID or follow the main display, with Option, Command, Control or Shift as the bypass modifier. Defaults remain disabled/main/Option.
- The native owner identifies the exact Dock AX container and the current display inventory. It infers orientation from verified geometry when macOS has no stored orientation preference.
- After observing the Dock on its target, a dedicated movement filter prevents migration through exposed edges on other displays. It passes through uncertain/expired observations, bypass input, synthetic input, held-button movement and non-movement events. Covered edges and ambiguous/mirrored configurations do not become eligible.
- Bypass, disconnected targets, unavailable edges, source failure and waiting for manual placement have distinct runtime states. Disconnected UUID selections persist. Reconnection rechecks actual placement; it does not silently move the pointer.
- Settings changes retire old source scopes synchronously. Replacement joins the old native lifetime; pause, inactivity and shutdown cancel protection. Settings and the switcher can remain open while protection runs.
- Localized English, Brazilian Portuguese and Spanish controls preserve explicit main-display UUID choices separately from following the main display. Runtime inventory refreshes the selector. Late initial-query failures cannot overwrite recovered state or clear an unrelated placement failure.

No persistent Dock preferences are modified. No user-window position, size or state action is used for monitor protection.

## Automatic placement boundary

The native public capability is false, and public placement refuses without queueing or posting movement. Experimental placement is reachable only through an unexported test helper. Future enabled placement requests are already scoped to the UI's exact session, settings revision and native generation; an older click cannot adopt a replacement monitor target.

Experimental transport sends inert tagged events and converts them to movement only at delivery after checking ownership. Isolated seams cover stale delivery, physical-input evidence, cancellation, topology, approach/outward pressure and post-restoration environment validation. This is not a claim that automatic placement is ready to enable.

## Verification

- 246 desktop, 5 shared and 3 site unit tests pass.
- All 17 Go packages pass with race detection and coverage enabled.
- Biome and golangci-lint report no issues; production frontend/site and embedded desktop builds pass.
- Chromium: 57 desktop and 4 site tests pass. Cases include saved physical-display selection, disconnected admission, exact generated RPC arguments, visible failures, and the default manual-only capability.
- Portable Linux platform test compilation passes with CGO disabled.
- A real read-only probe identifies the hidden bottom Dock and two connected displays, with exact UUIDs, localized names and positive container geometry. It does not establish physical edge prevention.
- An inert native transport attempt missed its acknowledgment; a diagnostic retry passed with unchanged cursor and foreground. A subsequent standalone native test moved the actual cursor exactly one point and restored it exactly, with unchanged foreground and joined cleanup.
- A bounded native Dock test successfully moved it to the second display, independently confirmed its AX location, and restored the cursor exactly. The return attempt observed an unmarked source-PID-zero event and cancelled without restoring over lost ownership. This proves neither that a human supplied that event nor that automatic round trips are reliable. Both native owners joined; automatic placement stays disabled.

## Remaining acceptance

Actual physical edge protection and bypass, all orientations, disconnect/reconnect, mixed Retina scaling and full session/permission transitions still need native acceptance. Automatic placement requires a reliable return path before enabling its capability. D15 remains unchecked on the [roadmap](../../dockdoor-roadmap.md).

Reproducible checked-in commands are documented in the [native fixture README](../../../apps/desktop/internal/platform/testdata/dock-monitor-lock/README.md). The implementation plan is [here](../plans/2026-09-06-dock-monitor-lock.md). Folder Pop, media, automation, distribution and the optional replacement Dock remain ongoing retained work.

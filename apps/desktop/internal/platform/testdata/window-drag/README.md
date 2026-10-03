# Passive window-drag source checks

Run from the repository root on macOS:

```sh
go test -race ./apps/desktop/internal/platform -run TestWindowDrag -count=1 -v
```

These tests install no CGEvent tap, show no window and generate no desktop input. The native callback probe uses a test owner with the production callback, passes a locally created CGEvent directly to it without posting it, and tests copied queue/gesture state. The position-baseline probe uses the production evidence updater with supplied AX positions; it does not claim those positions were sampled from a real remote window.

Covered: original-event pass-through; pointer-only refusal; mouse-up retirement; bounded queue overflow; correlated horizontal movement; stationary/vertical evidence reset; retaining the exact root through the initial drag threshold; retiring an old intent after discontinuity; cancellation joining the native lifetime; failed second observer leaving first evidence untouched; permission denial before installation and simulated permission revocation joining the source with an explicit error.

The source emits `moved` for sampled drag events even when AX position is stationary. This lets the pure recognizer restart its baseline and later arm in the same physical drag after the OS drag threshold. Such a sample cannot itself contribute movement evidence. A discontinuity timestamp prevents an older pending intent from being validated after evidence restarts.

Remaining native acceptance must use disposable bundled fixtures only: normal windows plus an attached sheet/modal parent; exact AX membership/role classification; refusal to close/minimize protected parent/dialog; direct eligible-window action; and real AX sampling through text selection and window movement. A hardware run must confirm physical dragging/shake feel. The isolated tests do not establish these acceptance results or physical finger/button behavior.

## Disposable native role/action acceptance

```sh
OPTION_TAB_WINDOW_DRAG_FIXTURE_SMOKE=1 go test -race ./apps/desktop/internal/platform -run '^TestWindowDragNativeFixture$' -count=1 -v
```

The retained test compiles `fixture.m` into a temporary uniquely named `.app` with bundle ID `com.optiontab.windowdragfixture`. Each case launches its own child with two standard windows and an attached sheet. It announces exact PID/window IDs in an atomically written state file. The test checks the PID against the launched process and permits mutations only on its explicitly classified eligible window. Separate fresh fixtures exercise `setMinimized` and `close`; the kept standard parent and its visible attached sheet must survive. The final native guarded variant must invoke its Go guard and propagate a refusal unchanged without mutation before the plain eligible action runs. Foreground PID must remain unchanged, including after fixture shutdown. No cursor movement, event posting, or tap installation occurs.

AX startup can briefly report an unresolved root while the sheet appears. The harness waits up to four seconds for positive read-only fixture classification before admitting any action; action calls themselves are not retried. Each child is stopped through its own command file and joined; a timeout kills only that launched child and fails the test. Go removes all temporary bundles/binaries/state files.

For bounded read-only AX diagnostics on the same announced fixture PID:

```sh
OPTION_TAB_WINDOW_DRAG_FIXTURE_SMOKE=1 OPTION_TAB_WINDOW_DRAG_ROLE_DIAGNOSTICS=1 go test ./apps/desktop/internal/platform -run '^TestWindowDragNativeFixture$' -count=1 -v
```

`roles.m` refuses any PID whose bundle is not the disposable fixture. It showed the sheet as an exact `AXSheet` child with its own CG ID and an explicit parent ID, absent from the app AXWindows root list. Production now exposes that positive protected-child identity without making it an actionable root. The fixture's sheet does not expose a supported AXModal attribute; its remaining relationship metadata stays explicitly unknown. Its parent is positively classified with `HasAttachedSheet=true`.

A preliminary sequence that minimized then attempted to close the same minimized window was refused. The retained acceptance uses fresh fixtures to verify each requested operation independently and makes no claim about close-button availability while minimized. Real remote-window drag sampling and hardware shake feel remain separate gates; role/action acceptance does not establish them.

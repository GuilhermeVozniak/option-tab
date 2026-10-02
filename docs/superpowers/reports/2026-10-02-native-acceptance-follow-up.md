# Native acceptance follow-up

This follow-up adds test coverage and a disposable bulk-action fixture. It does not enable a capability or establish new packaged/native acceptance. D15, A07 and A08 remain unchecked. The current product build remains `option-tab-local-2cd66bf.app`; it was not launched for these checks.

## Dock placement ownership

The existing in-memory delivery probe now exercises the real callback with an unmarked key event. A nonzero source PID preserves cursor ownership; PID zero retires it. After retirement, a queued carrier remains inert and new movement/restoration is refused. The isolated owner is retired, stopped and destroyed before the bounded test returns.

The focused race run passed ten top-level tests. In an isolated source copy, removing only the callback's PID-zero ownership increment makes the new assertions fail. This establishes regression sensitivity, not a newly fixed product defect. No event was posted, event tap installed, pointer moved or Dock relocated. Production classification and availability are unchanged. The source of the historical unmarked event and a reliable placement/return round trip remain unverified.

## Bulk-action fixture and native results

The opt-in fixture binds owned child handles, executable, process start time, private nonce, bundle, role and window IDs before dispatch. It permits only close/minimize actions against its own roots, observes a separate bystander, refuses foreground changes and document writes, and joins the children it created. A native alarm bounds the fixture's lifetime independently of Go cleanup. Ordinary test runs skip the live probe.

The planned A08 check observes three minimized windows, including an initially minimized window, then repeats the operation. The planned A07 check uses an edited in-memory `NSDocument` and AppKit's normal save sheet, with all write entry points refused. Neither scenario is counted as accepted merely because AX accepted a request.

Two bounded runs on macOS 27.0 (26A428), arm64, did not establish native acceptance:

- The first run reported one accepted minimize operation and two identity-guard refusals. The final target snapshot still had only its initially minimized window minimized. It did not establish a settled A08 result; A07 was never reached. Both children were joined.
- The read-only diagnostic run stopped during target startup after that disposable app became foreground. Its sticky activation and foreground-change flags were set. It never launched the bystander, produced an identity diagnostic sample, invoked the action service or reached the document scenario. The owned child was joined. Cleanup does not record its exit reason or final foreground state.

Read-only process inspection subsequently found none of the three owned processes remaining. No further live probes were attempted after the foreground failure. No delivered product app, real user document or media player was launched by these probes.

The original identity refusal remains unexplained. Production bulk enumeration can use a remote-token fallback that the fixture's additional automation-current check does not use; that difference is a hypothesis, not an established cause. No guard was relaxed, and neither failed run establishes a product defect or save-dialog preservation.

## Intel and universal compilation

A clean isolated checkout of `2cd66bf` compiled with CGO for x86_64. Combining that executable with the same checkout's arm64 executable produced both slices with a macOS 14 minimum. Both build records identify the exact commit with `vcs.modified=false`.

This verifies compilation and architecture/deployment metadata only. The executables were not launched or published. Intel runtime, a universal app bundle, release signing/notarization, installation, updating and Homebrew lifecycle remain unverified. This compilation predates the test-helper changes in this follow-up.

## Remaining delivery gates

Automatic native Dock placement, replacement-Dock pinch/swipe/letter input and Spotify seeking remain disabled. Real Music contents/playback synchronization, physical input and keyboard/IME focus, save dialogs, broader permission/display/Space/restart recovery and distribution acceptance remain open. Physical trackpad testing is deferred at the user's request.

The final frozen-source Go run passed all 25 packages with race detection and coverage: 1,099 top-level tests passed and 15 live tests were skipped, including the new bulk probe. Go lint passed with zero issues after a formatting-only correction. All 584 Go/native inputs matched before and after verification. The previous 607 JavaScript unit tests and 157 browser cases retain their passing evidence with all 223 frontend/shared/website inputs unchanged; they were not rerun for this test-only follow-up.

Local commands, source hashes, mutation control, failed native receipts, process cleanup observation and compilation metadata are retained under `.superpowers/sdd/fix-issues-2026-10-02/native-acceptance/`. The reproducible bulk fixture is under `apps/desktop/internal/actions/testdata/native-bulk/`.

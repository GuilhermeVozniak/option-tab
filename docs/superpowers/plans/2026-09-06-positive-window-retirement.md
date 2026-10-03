# Positive closed-window retirement

**Goal:** Fix the confirmed C06 retained-closed-window regression without treating offscreen status, AX absence, or generic capture failure as closure.

**Evidence:** Combined native fixture `r26m5obk` positively registered `kAXUIElementDestroyedNotification` for captured PID10689/window6615 and received that exact callback after Close. The destroyed window remained in CG layer-0 enumeration. Foreground10690 remained unchanged after the callback. Current `darwin_stream.m` already receives that notification but only retires capture/cache; `Windows()` still includes the stale surface.

**Scope:** Positive retirement for actually observed window identities. Complete discovery of never-observed closed surfaces, AX-hostile apps, and roots inaccessible on other Spaces remains separate. No generic offscreen/AX-absence filter; no window actions or process termination in this implementation beyond disposable test fixtures.

## Ownership and implementation

Worker `live_previews` owns platform retirement helper/tests, existing `darwin_stream.{go,m}` plus any narrow native identity helper/header, `darwin.go` Windows filtering, and the existing combined testdata harness. Root owns App/controller/config/frontend/bindings/index. Coordinate before changing another platform file. No commits by worker.

- [ ] Add a small pure retirement registry with tests first. Key positive evidence by exact window ID, PID and process launch identity; reject zero/unknown identities. A delayed callback must never hide a different process after PID reuse or a different window owner. Keep captured identity from registration; never query an invalid destroyed AX element in the callback.
- [ ] Add a distinct positive destruction path from the existing native stream observer. Generic owner loss, permission failure, ordinary stream stop/cancellation and snapshot failure must not mark a window closed. Notification delivery queues only lightweight identity data; no AX enumeration, capture or App callback under its registry mutex.
- [ ] Capture a stable native process identity while the stream target is valid. Validate it again against current inventory before excluding a CG candidate. Do not reduce identity to title or PID alone. A process exit/relaunch or a disappeared CG identity retires its registry record; prune using complete successful inventory snapshots, never a failed query.
- [ ] Filter only candidates with matching positive destruction evidence. Support reopening/reusing the same retained window: clear retirement on positive fresh root/reappearance evidence, without assuming a timeout establishes liveness. Query positive reappearance outside registry locks with bounded native messaging; guard concurrent newer destruction with a revision so an old validation cannot clear a fresh retirement.
- [ ] Add Go regressions for observed-close exclusion, off-Space/minimized/hidden windows without destruction remaining, non-destruction errors, PID/window reuse, stale callbacks, reappearance, failed inventory, and pruning. Run focused platform race tests. The registry may remain scoped to streamed/observed roots; document that coverage rather than claiming complete C06 acceptance.
- [ ] Re-run the existing retained-NSWindow native harness with its stale-list assertion intact. Prove exact native close, matching AX destruction, inventory eviction, next-frame/cache retirement, and post-close foreground preservation. Reach the last-document/windowless activation stage if the independent presence classification allows it; otherwise record the exact remaining reason rather than inventing empty windows.
- [ ] Record evidence and limits, run relevant Go/App regression checks, and hand changes to root. Root integrates them into a separately reviewable follow-up commit after checks. No release or C06 complete mark is implied by this bounded fix.

## Native API constraints

Local macOS SDK `HIServices/AXNotificationConstants.h` defines the destruction notification as an invalidated AX element. Capture numeric target/process identity at registration. Existing `ot_window_pid` only establishes current CG ownership; it does not establish process launch identity or window liveness.

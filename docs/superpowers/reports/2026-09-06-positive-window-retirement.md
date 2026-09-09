# Positively observed closed windows

Follow-up to the app/Dock checkpoint `273c159`. The retained-closed-NSWindow regression now passes the original native inventory assertion.

The platform records exact AX destruction separately from generic capture errors. Each observation carries window ID, PID, process-start seconds/microseconds and a generation. Window enumeration excludes only matching positive retirement evidence. Failed inventory or unknown process identity leaves candidates available; offscreen status and missing AX roots never establish closure. Fresh positive root evidence can clear retirement, guarded by revision so a delayed validation cannot overwrite newer destruction. Complete successful CG inventory prunes disappeared or replaced identities.

Native queries run outside the registry lock. Reappearance lookup is bounded per candidate (0.1-second app timeout, 0.03-second root timeout, 64-root/0.3-second traversal bound); there is not yet a global time budget across all retained candidates.

## Verification

- Test-first Go regressions cover exact positive evidence, invalid identity, stale observations, PID/process reuse, finished-state release, failed inventory, pruning and concurrent revision changes during reappearance validation.
- Focused race checks pass for platform, preview and App integration. Independent review found no correctness blocker.
- `SMOKE_RETAIN_CLOSED=1 python3 apps/desktop/testdata/app_dock_smoke/run.py` passes using the actual app, frontend, AX Dock observer, nonactivating panel, generated bindings and disposable fixtures.
- Actual native pointer Close targets the second preview. An independent observer receives exact AX destruction; the other fixture remains foreground after Close. The closed CG surface disappears from Option Tab's inventory despite its retained NSWindow.
- Closed-target capture stops at 40 frames, its cache entry is absent, and no further frame arrives during the 250-ms check. Total live capture peaks at 2 streams, delivers 82 frames and returns to zero active streams.
- After the last document closes, app mode retains both running fixture apps with zero window entries and activates the chosen PID. Window presence remains explicitly `unknown`, because auxiliary native surfaces are not proven absent.

## Remaining C06 coverage

This fixes roots whose stream successfully registered exact AX destruction. Never-observed roots, denied/unsupported notifications, missing process identity and inaccessible roots on other Spaces remain outside this mechanism. Native reopening of the same retained NSWindow and forced process-ID reuse were not exercised; deterministic tests cover the identity/revision rules. Complete C06 acceptance and a feature release are not claimed.

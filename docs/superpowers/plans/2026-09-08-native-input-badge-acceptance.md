# Native input and badge acceptance tools

The retained H15/H16 implementation is present, but synthetic native fixtures and Chromium cannot prove physical input delivery or live Dock badge association. Prepare two test-only tools before the coordinated native session. Building them is separate from running them. Production capability gates stay unchanged.

## Controlled badge application

Use a disposable application with its own bundle identity. It accepts only a small fixed command set to change its own badge to a known count, an indicator and empty, then quit. Bound its lifetime and command sizes. Do not operate other applications, read Notification Center, post input, request permissions or change Dock preferences.

An explicitly opted-in driver starts that exact fixture and derives its canonical bundle URL and process incarnation. Observe only that target through the production badge source. Match fresh values for each commanded phase; an unreadable/unsupported value cannot prove a count. The removal phase must establish that no count or indicator remains, while preserving the distinction between known absence and unavailable data. Join observation and the owned process before cleanup. Report typed outcomes and aggregate status, never raw AX labels or unrelated app data.

Ordinary tests must skip before launching any application. In this preparation stage, compile the fixture and driver, inspect their refusal/cleanup paths and verify the default opt-out. Actual known-badge changes remain unperformed until the coordinated session.

## Physical input panel

Use a standalone Wails hidden host with the production LauncherPanel, gesture and keyboard ports. Require an explicit native-run option and a real display identifier before creating any native application or window. Use current ordinary display/Space admission, safe interior placement, bounded lifetime and joined cleanup. Retire when the admitted native context changes.

The person testing supplies all pointer and keyboard input. The tool must not warp the cursor, post synthetic events, activate another app, execute window actions or modify saved Option Tab settings. Keyboard permission is acquired only from an explicit local control with the native final guard. Collect coarse gesture-kind and committed-input counts, plus permission/focus observations; do not retain typed text or unrelated app identities.

This probe establishes delivery through actual Wails/native ports. It does not replace later checks of the complete production route, edge layouts, actions or device combinations. Compile it now and document its deliberate invocation and cleanup; do not launch it during preparation.

## Completion evidence

Source and build success prove tool preparation only. Record actual build, display/Space, device, commanded phase or physical action, observed typed result and cleanup during the native session. Keep failed, unavailable and not-run outcomes distinct. Enable a production input capability only after the corresponding physical evidence exists. No release is part of this work.

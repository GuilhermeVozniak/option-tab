# Dock foundations implementation checkpoint

Branch: `feat/dockdoor-parity`. App version remains 0.4.8; no feature release is published by this checkpoint.

The switcher now supports explicit window/app actions, configurable action keys and pointer gestures, compact-list thresholds and layout direction. Live ScreenCaptureKit previews use at most four native streams and cancel when the switcher closes. Dismissal animation, app badges and visible failure feedback are connected to the app runtime.

## Delivered behavior

- New Window uses an enabled application Accessibility menu command. Force Quit targets the explicit PID. Close/minimize-all process verified window roots and return accepted counts plus per-window or incomplete-discovery failures.
- Clicking a control sends its window/app identity directly; changing selection cannot retarget the operation. Existing single-window minimize remains a toggle, while bulk minimize only sets the minimized state.
- Physical A–Z action bindings can be added, edited, remapped or disabled. Middle click defaults to close; swipe actions are opt-in. Wheel deltas accumulate and each continuous gesture fires once.
- Horizontal/vertical layouts and automatic compact titles preserve navigation. Controls respect their visibility setting; action labels identify the selected app.
- Live previews retain their image element between frames, cancel stale sessions, and clear both image paths when a window is reported closed. Background screenshots remain opt-in.
- Native dismissal waits 180 ms for the frontend fade; reopening, preferences and shutdown invalidate pending hides. Preferences closure now cancels Wails destruction through a synchronous hook.

## Verification

Passed repository checks: `task lint` (0 issues), `task test` (201 desktop frontend, 5 shared and 3 web tests, plus all Go race tests), `task build`, and `task e2e` (35 desktop and 4 web browser tests). Focused regression checks were observed failing before fixes for identity routing, map isolation, gesture behavior, native bulk uncertainty, hidden/shutdown capture admission, closed-image invalidation and dismissal timing.

Repeatable macOS fixtures are committed as source only:

- Actions: `python3 apps/desktop/internal/actions/testdata/native_smoke.py`. Final reviewed fixture PID 69244: New Window, exact key-window focus, single minimize toggle, all three real windows minimized/closed, close veto preservation and exact-PID Force Quit passed. Both bulk results reported three accepted operations plus one honest incomplete-discovery warning for eight unresolved native surfaces.
- Capture: `cd apps/desktop && OPTIONTAB_STREAM_SMOKE=1 go test ./internal/platform -run TestNativeStreamAnimationSmoke -v -count=1`. Five cycles each produced three distinct PNG frames. Cancellation took 6–13 ms; the registry emptied after each. Verified minimized/hidden windows remained subscribed. Retained close stopped in 10 ms and destroyed-window close in 5 ms.
- Chromium checks cover the real rendered UI, explicit RPC arguments, visible failures, ordinary binding edits, accumulated wheel momentum, swipe-click suppression, compact navigation and image-node retention. Visual inspection covered the toolbar, badges and interaction settings.
- Independent native and frontend reviews found issues that were corrected. The last native integration finding, clearing a previously selected window's cached large preview, has a regression through the App event transport.

## Remaining limits and next milestone

An accepted AX request does not guarantee an application completes an asynchronous close or bypasses a save dialog. New Window currently recognizes the explicit English “New Window” and “New Finder Window” menu titles; other menu variants return unsupported.

Native AX lookup is bounded. Unknown candidates produce an explicit incomplete bulk result; they are never silently counted as processed. Cross-Space behavior and third-party menu variants still need broader native validation.

Exact AX destruction notifications detect retained closed windows when supported. If Accessibility or that notification is unavailable, ownership checks detect exited/reused identities but may not detect a retained closed window until the capture session ends. One shared idle observation thread lives for the process lifetime. Long-term GPU allocation, permission revocation, and older macOS were not manually profiled in this checkpoint.

The next plan covers app-grouped/windowless switching and native Dock observation/panels. Folder/media panels, automation, distribution work and the optional Dock replacement remain outstanding in the narrowed roadmap. Sibling-product exclusions remain unchanged.

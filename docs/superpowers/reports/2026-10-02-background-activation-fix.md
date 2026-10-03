# Background startup activation

An owned native launcher fixture reproduced unexpected application activation when its nonactivating panel first appeared. The pinned Wails version requested application activation unconditionally during startup, including for accessory applications such as Option Tab.

The app now uses Wails `v3.0.0-beta.5` and the matching `@wailsio/runtime` package. This is the first published version containing the [upstream startup activation fix](https://github.com/wailsapp/wails/pull/5897): only regular applications receive the forced activation request. The previous Go alpha2.117 / JavaScript alpha.97 pair was the historical supported pairing; its different version numbers were not themselves a defect. Generated frontend bindings are unchanged.

## Causal native test

The test used an isolated entry point, the actual launcher panel adapter and production frontend, and three disposable owned applications. Capture, global hotkeys, application actions, updates, login changes and user settings were unavailable. No product build was launched.

Bootstrap alone, hidden-host creation and attachment of an unshown native panel each preserved the original foreground for 30 seconds. Directly showing the panel with the original framework activated the harness approximately 2.161 milliseconds later. Copying the exact pinned framework and changing only its startup activation guard made the identical 30-second Show probe pass: all 119 samples retained the original foreground, with no active harness or key window. Navigation, runtime readiness, scoped input/host/environment cleanup and all four normal child exits completed.

These A/B results identify the unconditional startup request as the cause of that reproduction. The control and candidate bundles, source hashes and traces are preserved in `.superpowers/sdd/h15-production-acceptance-2026-10-02/`. They do not establish every product startup, display or Space configuration.

The published beta.5 package was then tested without a local module replacement, using the updated runtime and explicit borderless host option. Its normal launcher presentation passed a separate 30-second run: all 120 samples preserved the foreground, with no active harness or key window, followed by complete scoped cleanup and four normal exits. A preceding direct-Show run also remained inactive, but observed unrelated foreground changes and is not counted as uninterrupted foreground preservation.

## Frameless host compatibility

Beta.5 also changes the default macOS frameless window to use a native title frame and rounded corners. The overlay and hidden panel hosts now explicitly use the square corner option, retaining a true borderless host. Their CSS, material effects and owned native panel layers continue to control visible rounding. This intentionally avoids introducing another clipping layer; it is not a claim that every old framework layer property is identical.

## Remaining native input acceptance

The patched framework also reached the normal three-item launcher without startup activation. Opening the already running Alpha fixture through Finder admitted an exact foreground baseline. Clicking the keyboard-mode button then published keyboard mode and produced an application-active observation, so the strict harness stopped before any letters were sent. That observation has not yet separated computer-use targeting from native keyboard admission behavior.

Native committed text, IME and physical gesture delivery are therefore still unaccepted. Production launcher pinch, swipe and letter-input capability gates remain unchanged. The startup correction must not be described as completing those features or the entire native acceptance matrix.

## Automated verification

All 25 Go packages passed with race detection and coverage. Fresh frontend checks passed 547 desktop, seven shared and four website unit tests; 125 desktop Chromium, 23 desktop WebKit and four website Chromium cases; TypeScript, workspace lint and Go lint. Ten focused native-harness race tests also passed. Regenerating bindings produced no changes. The first browser attempt could not launch because the required Playwright cache was absent; installing the matching test browsers and rerunning the unchanged source passed without retries. Compiler deprecation and frontend build-size warnings remain recorded.

Verification receipts and source manifests are under `.superpowers/sdd/fix-issues-2026-10-02/wails-beta5/`. These checks verify the dependency correction and its supported integration paths; they do not replace the outstanding hands-on acceptance.

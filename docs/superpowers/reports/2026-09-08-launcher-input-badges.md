# Dock input and badge integration

H15 adds per-profile gesture, haptic and letter-navigation settings. All are opt-in. Precise scrolling selects an item or opens/closes its existing preview panel according to edge-relative mappings. Native pinch and swipe packets use their own sources and thresholds. Selection starts empty and never activates an app merely from hovering or cycling. Haptics use the existing platform alignment-feedback port and a 40 ms transition limiter.

Keyboard mode requires an explicit click and exact native key-window permission. Only committed, unmodified Unicode letters cycle current actionable items; an app inside a group reveals that group. Paste, drop, cancelled composition and IME candidate-confirmation Enter do not dispatch actions. Ordinary Enter activation is a separate opt-in. Native Escape, key resignation, hide and close retire keyboard permission. A 28-point layout reservation keeps the compact control and one complete icon inside the host on all four edges, including magnification and protected placement.

Each input owner binds the exact host, native token, display, Space, profile, items and private references. Clock-only content revisions preserve selection and keyboard mode. Native callbacks copy into bounded mailboxes; serial App workers reduce gestures, validate authority and perform any requested action. Replacement owners wait for prior native work to actually finish. Initialization waits for physical panel readiness within two seconds, retires failed owners and limits future retries. There is one authoritative state/error event, and delayed events cannot discard a newer state waiting for its parent presentation.

H16 adds optional badge display from the native Dock's advertised Accessibility status-label attribute. This is a compatibility source, not a universal notification API. Exact canonical application identity and process incarnation prevent name-only matching. Numeric labels become bounded counts, other nonempty labels become a generic indicator, and unavailable reads remain unknown. The interface displays counts above 99 as `99+` and preserves the full count in the accessible description. No raw label, application path or process identity enters the renderer.

One observer serves the union of enabled, visible Dock hosts, with at most 256 private targets. Reads are bounded and start no more than once per second; unchanged values are coalesced. Every published sample revalidates private identity and native/logical host admission. Generations retire immediately, while cancellation receipts wait for reads and callbacks to return. Failed preparation and source exits retry serially after a one-second delay. Partially available targets recover without restarting a healthy complete observation. Frontend ownership rejects old sessions, retired owners and reordered values while retaining clock-only updates.

## Review fixes

Focused regressions exposed and fixed same-sequence error loss, newer pending-state displacement, IME Enter activation, compact keyboard geometry, and the asynchronous native-panel readiness race. Badge review addressed physical-loss cancellation and transient preparation/source recovery. Native gesture tests cover policy replacement, terminal acknowledgement, exact host retirement and a queued setter completing after hide/show.

The final retained-scope audit found no additional missing feature implementation in A–H. It did identify English-only custom chooser labels under G06. Folder, lyrics, widget-package and diagnostics choosers now use macOS-localized defaults. Diagnostics presents its new-file-only instruction in the translated app review before Save. Independent reviews distinguish these code changes from physical and release acceptance still required by the roadmap.

## Verification

- All 25 Go packages pass race/coverage tests; golangci-lint reports zero issues.
- 420 desktop, 7 shared and 4 website unit tests pass. Workspace Biome, TypeScript and production builds pass.
- All 100 desktop Chromium checks pass, covering badge placement on all four edges, compact and large-icon keyboard layouts, selection, reordering and Settings reconciliation.
- Pinned Wails generation reports 132 methods, 14 enums and 75 models. The platform test package also cross-compiles for Linux with CGO disabled.
- Real CGO arm64 and x86_64 builds pass; binary inspection records minimum macOS 14.0 for both. These binaries were neither launched nor distributed.

The initial combined frontend gate exposed an outdated capabilities mock, ambiguous text selectors and a browser test ending before Settings reconciliation. Those fixtures now model the new read-only API, identify the intended explanatory text and await the actual post-action read. The final full suites pass without increased timeouts or skipped tests. No verification hook was bypassed.

## Acceptance boundaries

Production pinch, swipe and letter input remain unavailable until actual trackpad and nonactivating WebKit keyboard delivery are verified. Precise-scroll and haptic availability reflect the implemented native ports; physical delivery and sensation remain separate acceptance checks. Synthetic native extraction fixtures and Chromium do not establish those outcomes.

A read-only Dock probe on the development Mac found advertised status-label attributes and readable string values without retaining their contents. It establishes source feasibility on that machine, not universal badge support or correct live count changes. Actual badge updates/removal, physical input, mixed-scale display behavior, signed-app permission flows and the broader native acceptance matrix remain open. This checkpoint does not create a release.

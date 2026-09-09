# Fixes from the 2496558 native retest

This follow-up fixes blacklist creation, standalone preview admission, native focus restoration, localization and import feedback from [the September 8 native retest](2026-09-08-native-2496558-retest.md). The suspected folder-child defect was resolved by an independent native check: the existing installed build opens a separate visible child. No folder behavior or guard was changed.

## Changes

- **Blacklist creation:** New entries remain local drafts until a nonempty matcher is saved. Previously, Add immediately persisted an empty matcher, which canonical normalization correctly removed before the user could type. Existing matcher edits commit on blur or Enter; blank edits retain the stored matcher. Canonical refresh, import and failed-save recovery discard obsolete drafts.
- **Automation previews:** App filters now run before capturing window identities. A hidden, untitled or excluded minimized auxiliary CG window no longer blocks eligible document previews merely because it lacks an authoritative AX window. Selected identities and the exact process are revalidated after host preparation; stale targets still cannot acquire controls or receive actions.
- **Automation focus:** The admitted `AXRaise` call receives the remaining time from the original 1.5 second action deadline. It previously inherited the 100 ms inventory lookup timeout, rejecting a restore that subsequently completed. Identity, permission, cancellation and deadline guards remain in place; late acknowledgements still fail.
- **Localization:** PT-BR and Spanish now include the background-capture label and explanatory text, plus the new blacklist and import confirmation labels.
- **Import feedback:** Import clears prior export feedback and announces success only after persistence returns with current settings authority. A superseded import no longer silently resolves as successful. Cancelling the chooser remains neutral.

The browser blacklist test now follows the explicit Save app flow and checks canonical persistence, blur edits, refresh and removal.

## Native evidence

An owned remote AppKit fixture reproduced the focus failure: `AXRaise` returned `kAXErrorCannotComplete` after approximately 105 ms while restoration subsequently finished. The corrected production function received native acknowledgement after approximately 535 ms and returned success. The application's `AXFocusedWindow` equalled the exact target window immediately and after settling; AppKit reported key/main and unminimized. Both fixture processes exited successfully.

The minimal Wails probe created real hidden Wails hosts and production launcher panels on both monitors. A second probe exercised production App child preparation and the native folder source, listing exactly its owned `Owned.txt`. Both phases passed on the CU34G2XP and LG ULTRAGEAR displays. All parent and child validators passed, all close receipts and manager drains joined, and no owned visible windows remained. These disposable fixtures used no user settings, global input hooks or actions against user windows.

For the decisive installed-app check, root selected the owned `Native UI test folder`, saved it in a temporary profile and left only the LG test launcher enabled. Before the click, independent CG and AX reads showed only parent window `1790`, frame `(3921, 2231, 478, 64)`. After the click, both APIs also showed child `1795`, frame `(3950, 1994, 420, 229)`, onscreen, alpha 1 and not minimized. A second capture 1.18 seconds later confirmed both windows. The reader performed no activation or UI action. This establishes working child presentation in the unchanged installed `2496558` build and corrects the inference from screenshots scoped to its parent.

The child remained visible in a later read, but its AX group exposed no descendants. The names `Nested`, `One.txt` and `Two.txt`, sorting, opening and closing from this packaged child were therefore not verified. The earlier September 8 window snapshot remains a historical observation; it does not establish a reproducible folder defect after this controlled check. A headless diagnostic also confirmed generation changes retire child admissions; without evidence of a live defect, those guards remain intact.

## Verification and limits

Regression tests cover canonical blacklist save/reopen and recovery, stale import responses, filtered preview candidates, selected-window retirement during host preparation, refused stale actions, native focus timeout and late acknowledgement. Independent reviews found no actionable issue in the preview or focus changes.

Fresh checks passed with the final source changes: 488 desktop, seven shared and four website unit tests; all 25 Go packages with race detection and coverage and no cached results; full frontend lint and zero Go lint issues; 110 Chromium tests and ten WebKit layout tests with two workers and no retries. TypeScript and scoped regression checks also passed. The first Chromium run found the obsolete blacklist test expectation; the corrected full rerun passed. Existing toolchain deprecation and React test warnings remain in the logs.

The focus correction has owned native evidence; blacklist/import/localization and filtered preview corrections have automated coverage. Their final packaged-app UI flows have not been rerun here. Physical input, media playback and lyrics, Space/lock/restart transitions, permission transitions, supported-OS coverage and signed universal distribution retain their earlier acceptance limits. Intentionally unavailable gesture delivery, automatic native Dock relocation and Spotify seek are unchanged. The separately assembled local bundle records its exact source commit and checks in its `.build.json` sidecar.

Root restored the original configuration through native General import and removed the unused test reference through the UI. Final independent comparison verified the original 6,467 bytes and SHA-256 `0a75abba2be980f06a0cf939164dc2d820767d2fe235216d0c73f83fc953a60b`, with no private references or packages. The installed app remained PID `58044`; it was not rebuilt, restarted or replaced. User-owned `refactor-notes/` was untouched.

Local evidence: `.superpowers/sdd/native-fixes-2026-09-09/` contains `frontend/`, `automation-preview/`, `focus/`, `folder/`, `folder-wails/`, `verification/` and `build/` logs and results. Native screenshots were inspected in the conversation; the independent CG/AX metadata is retained as JSON.

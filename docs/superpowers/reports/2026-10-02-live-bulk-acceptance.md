# Live bulk-action follow-up

The running `option-tab-local-2cd66bf` build was exercised through its visible controls after Accessibility and Screen Recording showed Granted. This session used only three disposable TextEdit documents (`Untitled`, `Untitled 2`, `Untitled 3`) in process 2480. The first contained a disposable test sentence. These results extend the [earlier guarded fixture attempts](2026-10-02-native-acceptance-follow-up.md); they do not change the build or complete the broader A07/A08 acceptance matrix.

## Minimize all

The first invocation reported three accepted requests, nine unresolved Core Graphics candidates and one minimize-setter failure. A later invocation reported three accepted requests and the same enumeration warning, without the setter failure. Read-only native snapshots confirmed that the exact three document windows—5638, 5650 and 5652—were minimized and off-screen. After another invocation, all three remained minimized: the bulk operation did not restore them. Process identity remained unchanged across those snapshots.

The nine unresolved layer-zero Core Graphics surfaces were absent from the returned Accessibility window roots. That observation does not establish that they are safe to omit; the incomplete-enumeration warning remains appropriate. The initial result accounts for four root entries, but the failed window ID was not retained in the toast and remains unidentified. Later snapshots contained three AX roots. No classifier or action code was changed to suppress these uncertainties.

## Close all

The first close-all attempt displayed TextEdit's actual save sheet for `Untitled 2`. Cancel preserved that document. The Window menu then showed `Untitled` and `Untitled 2`; the clean `Untitled 3` had closed. A second attempt displayed the actual save sheet for the edited first document. Cancel returned to its intact test sentence. No save or discard confirmation was selected.

These observations establish bounded native minimize/idempotency behavior, clean-document closure and preservation of TextEdit's save prompts and cancelled edits. Action feedback still reported close uncertainty/failures; accepted requests are not proof that every window closed. Bystander isolation, other applications, Spaces, broader lifecycle cases and release validation remain unverified. A07/A08 remain unchecked.

## Evidence and scope

The read-only metadata records are retained under `.superpowers/sdd/fix-issues-2026-10-02/native-session/`: `textedit-window-metadata.json`, `textedit-after-repeat.json`, `textedit-after-idempotent.json` and their execution receipts. They establish exact process/window identity and minimize state. Save-sheet, Cancel, Window-menu and text-preservation observations came from the computer-use UI session, not those metadata files.

Physical gesture testing was separately waived by the user and was not performed or counted as passed here. These observations do not validate the later gesture-availability change in the older running build.

## Dock settings and remaining checks

Monitor lock reported Protected for the main display, Awaiting placement when targeting the second display, then Protected after returning to the main target. The original disabled setting was restored. This checks position admission and status transitions, not physical migration blocking or modifier bypass.

The awaiting-placement guidance referenced an unavailable Move Dock here button. The frontend now shows that optional-action hint only when automatic placement is available; the existing translated manual-placement instruction remains visible. All 596 desktop frontend tests passed, including the existing monitor-lock tests; TypeScript and Biome checks passed.

Spotify connected through the product using existing consent. A separate read-only timing diagnostic stopped at macOS Automation preflight with consent required, before reading any properties. Spotify seeking remains unavailable pending runtime unit verification. Keyboard/IME focus acceptance also remains open; an additional bounded fixture run stopped before input because an owned foreground baseline had not been established. Its four owned processes exited normally and native input resources drained.

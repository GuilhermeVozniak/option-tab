# Conditional provider lyrics

E05 now has a Music source alongside the existing local LRC source. When media controls and Music are enabled, a visible media presentation can read the current track's lyric text through an existing Automation grant. Text becomes synchronized lyrics only when the existing bounded LRC parser finds usable timestamps. Plain text, missing lyrics and failed reads have explicit localized states; no timestamps are invented.

This does not establish access to Music's catalog synchronized-lyrics UI. The installed Music 1.7 scripting dictionary exposes the `pLyr` text property under `pTrk`, without a separate timing contract. Actual track contents and native playback synchronization remain unverified.

## Source and ownership

The optional provider capability uses the existing per-player gate and captured provider, PID, process incarnation, observation generation, track epoch and track ID. Native preparation checks the exact process and persistent track ID, then calls the App's final admission guard immediately before the typed property getter. Identity and admission are checked again before publication.

The getter uses no-prompt Automation preflight, noninteractive PID-addressed AppleEvents and a shared two-second event deadline. It accepts text descriptors only and caps decoded UTF-8 at 1 MiB. Native descriptor/string allocation precedes that cap. Cancellation cannot interrupt an AppleEvent already in flight; its completion is discarded after the bounded native return. Read-and-recheck is not atomic against an unobserved A→B→A track change.

A chosen local association takes priority. Invalid or unreadable local data stays a local error and never silently falls back. Import admission retires pending reads while preserving already displayed lyrics; matching reads resume only after the chooser drains. Remove/offset operations similarly serialize with same-track reload/import. Provider data has no local document ID or saved offset, and local-only mutations are refused by the backend as well as hidden in the panel.

Provider cues use the existing pause/seek/progress timeline. Position updates do not reread lyrics; explicit Reload can retry. The panel shows Music attribution, retains local Import, and resets manual lyric scrolling when the source or document changes. Pending imports disable conflicting local controls while retaining Cancel. Provider text is memory-only; the feature adds no network lyric service, credentials, persistent lyric cache or silent permission request. The data-handling disclosure has been updated.

## Progress update correction

Expanded WebKit media checks reproduced a second issue: the Dock media panel could consume a progress sequence inside a React state updater, then discard the accepted update when Strict Mode replayed that updater. Sequence admission now occurs once in the event callback; the updater is pure and still checks the rendered session/revision. Regressions cover replay, reordered/duplicate progress, stale/future/foreign scopes, revision changes and batched show/update/hide events.

## Verification boundary

Platform tests cover gate serialization, exact scope, cancellation, final guards, bounded replies and coarse errors. A controlled native descriptor seam checks property/container/opcode, exact targeting, deadline, denied access, wrong types, empty/oversized text and process/track replacement. It does not deliver AppleEvents to Music.

App tests exercise the real controller and timeline with the external lyric read replaced: local precedence, import/mutation races, malformed replies, no repeat lookup on progress, explicit retry and late completion after track change, disablement or final panel closure. Frontend tests cover source controls, translation, pending import and same-track replacement.

No delivered app or real player was launched, no Music library was accessed and no consent was requested for this implementation. Broader E04–E06 packaged/native acceptance remains open; the roadmap checkboxes are unchanged.

Final automated verification passed on the frozen implementation: 596 desktop unit tests, 126 desktop Chromium cases, 27 WebKit cases and all 25 Go packages with race detection and coverage. TypeScript, frontend lint/build and Go lint passed. The unchanged shared/website inputs were compared by hash with the prior passing evidence: seven shared tests, four website tests, four website Chromium cases and the website build. Combined validated totals are 607 unit tests and 157 browser cases; desktop checks were rerun after the progress correction, with no browser retries.

Local commands, source hashes, initial failures and final results are retained under `.superpowers/sdd/fix-issues-2026-10-02/e05/`. Backend verification includes 1,098 top-level passing Go tests; 14 opt-in native/live tests remain skipped. Build/CI receipts are separate from these source-level checks.

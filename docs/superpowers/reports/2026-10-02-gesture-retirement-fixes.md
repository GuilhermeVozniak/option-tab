# Gesture and keyboard retirement fixes

This follow-up to the [native switcher gesture checkpoint](2026-10-02-native-switcher-gestures-and-widget-readings.md) corrects reproduced input lifetime failures found in the subsequent audit.

## Cancelled swipes cannot acquire another target

The shared native Dock/switcher wheel classifier previously allowed a precise `phaseChanged` event to begin ownership. When a card layout or policy changed during a swipe, the old gesture was cancelled, but its next continuation could acquire the replacement card. The original finger movement could therefore perform an action against a different window.

New ownership now requires `NSEventPhaseBegan`. Changed events still continue an already owned gesture. Policy replacement, disable/re-enable and hide/reopen each require a fresh beginning; stationary, MayBegin and momentum events cannot acquire a new target. Existing direction normalization, immutable target, acknowledgement and bounded queue behavior are unchanged. This deliberately tightens the historical D12 allowance for beginning ownership from a changed event.

The regression fixture compiles and invokes the actual Objective-C classifier without posting OS input. All three retirement scenarios failed against the previous implementation, then passed after the correction. It also checks normal changed/end/acknowledgement and recovery from a fresh beginning.

## Keyboard sessions discard unfinished composition

Launcher text composition and modifier refs previously belonged to the persistent launcher view. If keyboard mode ended during composition, or the native keyboard admission was replaced, those refs could survive into the next input session. Ordinary Enter then remained blocked, and the same input element could retain partial composition across admissions.

The input now owns those refs in a component keyed to its exact launcher and keyboard admission. Exiting or replacing the session discards unfinished input and its handlers. Ordinary presentation updates within the same admission retain the input. Regression tests reproduce both retirement cases, check that Enter works in the new session, and reject a late composition completion from the detached input. Existing IME confirmation, modifier, paste/drop and opt-in activation checks remain covered.

## Reopened switcher and Dock preview use the saved language

The persistent switcher and Dock preview loaded their translators only when the webviews mounted. Saving a different language in Preferences therefore left the next session showing the old labels and gesture errors. Each translator now reloads once when a visible session opens. Selection updates do not trigger further settings reads, and a reply from a hidden or replaced session cannot change the current translator.

## Evidence and remaining acceptance

Local evidence is retained under `.superpowers/sdd/fix-issues-2026-10-02/`. The native review receipt records the failing and passing classifier runs and focused race checks. The keyboard folder records its two failing regressions and corrected run.

Final checks passed: 547 desktop frontend, seven shared and four website unit tests; all 25 Go packages with race detection and coverage; TypeScript, full workspace lint and Go lint. Browser checks cover 125 desktop Chromium and 23 WebKit cases (ten layout, twelve switcher and one launcher keyboard lifecycle). Four unchanged website browser cases retain their previously verified evidence. The final Spanish error-dismiss label was checked in Chromium and WebKit; the remaining Chromium evidence predates only that accessible-label translation and its assertions.

The final frontend unit suite used two workers and unchanged timeouts. An earlier run concurrent with browser checks exceeded a settings test's five-second limit, then its unfinished async body affected the following test. The isolated 15-test settings file and the complete bounded-worker run both pass; the failed attempt is retained in the receipt. A temporary WebKit configuration also initially used the wrong server working directory; the corrected 23-case run passes without retries. Existing native toolchain and React test warnings remain in the logs.

These tests establish the identified classifier and renderer fixes. They do not establish physical device delivery or native WKWebView IME behavior. Production launcher pinch, swipe and letter-input capability gates remain unchanged pending their native acceptance. The separate acceptance harness and any observations from it must be reported with their own source and input provenance.

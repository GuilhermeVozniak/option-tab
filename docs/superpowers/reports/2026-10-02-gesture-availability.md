# Pinch and swipe availability

The user explicitly chose to skip physical gesture testing and assume the implemented gestures work. This changes the physical-test requirement; it does not supply evidence that those tests passed.

Replacement-Dock pinch and swipe are now available when the platform supplies a launcher panel host. Both remain opt-in through the existing per-profile settings. Their native handlers, gesture reduction, target validation, retirement guards and action routing were already implemented. This change exposes those existing paths by reporting their capabilities.

Keyboard/letter navigation remains unavailable pending its separate focus and input checks. Gesture availability does not grant keyboard ownership. Default settings, haptics, saved preferences, native input guards and the informational `deliveryUnverified` reason are unchanged. The settings UI handles each capability independently and needs no translation or layout change.

Two regressions exercise the production capability path instead of the test override: host/no-host availability and installation of an opted-in magnify/swipe policy with precise scroll disabled. The latter also checks actual selection change from a delivered controlled swipe and continued keyboard refusal. Both failed against the old capability declaration and passed after the two capability fields were enabled. The focused race run passed 23 top-level App, reducer and platform tests. Native extraction checks use an unattached controlled seam; they are not physical input evidence.

The user has mouse and keyboard only. Physical gesture testing is waived for this session. Keyboard/focus, other native behavior and release checks remain separate. No roadmap checkbox is marked as a physical test pass.

Final verification passed all 25 Go packages with race detection and coverage: 1,101 top-level tests passed, 15 live tests skipped, and Go lint reported zero issues. All 584 Go/native inputs matched before and after the run. No live test opt-ins were inherited and no physical input was supplied by these checks.

Local red/green logs, frozen source hashes and final verification receipts are retained under `.superpowers/sdd/fix-issues-2026-10-02/native-acceptance/gesture-waiver/`.

# Typed media AppleEvent fixtures

Run from the repository root:

```sh
go test -race ./apps/desktop/internal/platform -run '^TestMedia' -count=1
OPTION_TAB_MEDIA_DELIVERY_SMOKE=1 go test -race ./apps/desktop/internal/platform -run '^TestMediaActualDeliveryOptIn$' -count=1 -v
```

The default native descriptor fixture includes the production Objective-C adapter with compile-time-only process, permission and event-sender substitutions. It validates fixed suites/properties, PID addresses, finite deadlines, wrong types, oversized strings, changed track, process disappearance/relaunch after reads begin, provider-isolated denial, a two-second in-flight sender join, denied/not-running state, final admission refusal, exact pause and Spotify seek refusal. The Go source test separately cancels the observation while command preparation is held and verifies the final guard prevents mutation when it joins. The fixture data is original. This is not a real-player or TCC acceptance test.

The optional delivery executable creates no windows and uses prohibited activation policy. It registers its own fixed AppleEvent handlers, performs no-prompt preflight against its own PID, then sends the adapter's actual OS AppleEvents to that PID. It exits 3 (test skipped with explicit reason) if preflight/delivery is unavailable; it never asks for consent. It does not target Spotify, Music, another fixture process, or user apps. It tests self-process OS delivery, not cross-process Automation entitlement/signing behavior. The test-only process substitution is absent in production builds; the production provider bundle allowlist remains fixed.

Accepted proof on 2026-09-06: optional race run receiver PID40072, 10 metadata reads, one pause mutation, zero prompts; executable exited normally. No cursor/Dock interaction, audio playback, player launch, or file-open action occurs. Build outputs are temporary test files; manual diagnostic binaries/logs are under the worktree's ignored `.cache/media/`.

Real Music/Spotify metadata, capabilities, duration units, signed cross-process TCC grant/denial/revocation, artwork compatibility, and actual UI buttons remain separate coordinated acceptance. Spotify duration/seek is deliberately disabled.

## Local lyric file fixture

`go test -race ./apps/desktop/internal/platform -run '^TestMediaLyrics' -count=1 -v`

`lyrics.m` includes the production file reader with a substituted NSOpenPanel class. It creates only original disposable .lrc files in its test directory, verifies real bookmark round-trip and exact inode refusal, and counts balanced scope begin/end. It tests queued/active chooser cancellation without displaying a window. No user paths, file opening, media players, or permissions dialogs are involved. Actual visible file chooser acceptance is not established by this seam.

## Native media pin geometry/ownership seam

`go test -race ./apps/desktop/internal/platform -run 'TestMediaPanel|TestDarwinPanelWheelNativeSeam' -count=1 -v`

`panel.m` tests native clamp/display selection and header event admission with fake window/event objects. It does not create a visible NSPanel, post input, or prove physical dragging/Retina transitions. The fake drag hides its own token to verify queued ownership retirement; real performWindowDragWithEvent cancellation remains a separate acceptance case.

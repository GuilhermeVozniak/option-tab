# Native acceptance of caf9166 — 2026-09-08

The user installed and launched `/Applications/option-tab-local-caf9166.app`. This pass exercised that running version 0.4.8 bundle on the user's second monitor. Its executable matches the previously delivered clean `caf916602e27157232dda70eac94eee9e8977544` build (SHA-256 `fb00b659e8edbe1ff968e6a11d4e08a34179f50a094a85bdc152a29b1f098a6b`). Accessibility and Screen Recording both showed Granted. The app remained running; it was not rebuilt, replaced, or restarted during native testing.

## Native results

| Flow | Observed result |
|---|---|
| Settings export | Native Save and neutral Cancel worked. Saving a fresh file showed success and produced valid canonical JSON with mode 0600. |
| Profile duplication, naming, export | The owned duplicate retained its new name in the profile export. Native Save completed. The first accessibility click after editing committed blur; a second opened Save, so single-click blur-to-export behavior is not claimed. |
| Export collision | Reusing the owned filename and confirming Replace returned the translated filename-exists error. The existing file hash remained unchanged and controls remained usable. |
| Portable profile import | The actual WebKit chooser reviewed one item, one widget, and three sanitization notices. Import created an unassigned profile, disabled its widget, and cleared grants while preserving the existing display assignment and disabled global Dock state. |
| Imported folder repair | Selecting the owned folder and saving persisted a new private reference without the previous `cleanupFailed` error. The structural placeholder disappeared. |
| Display choices | LG ULTRAGEAR remained available after settings saves while replacement Dock was disabled. |
| Timezone | `Europe/` showed validation feedback and was not persisted. `Europe/Rome` committed successfully on Enter. This pass does not establish incomplete-draft retention across a Preferences reopen. |
| Widget package lifecycle | Chooser Cancel was neutral. Review displayed the owned package's metadata and required clock access. Installation granted nothing. Adding its instance left it disabled; explicit grant and enable worked. Removal disabled the instance and cleared its grants in canonical settings. |
| Replacement Dock | Assigning the owned profile to LG ULTRAGEAR and closing Preferences displayed the live Dock with its folder pin, running applications, and first clock. Preferences correctly suspended the Dock while open. |
| Second clock | Both clock texts existed in native accessibility, but only the first was visible. The second lay outside the widget container despite remaining inside the native window. This is a failed acceptance check. |
| Folder popover | Accessibility and coordinate activation of the owned folder did not produce an observable child panel. Native window metadata subsequently showed only the parent Dock on screen. The exact failed startup stage is unknown. |
| Media controls | Spotify Connect was disabled with master media off and became available with master and provider enabled. Connecting did not reach Connected during this pass. The button remained enabled while the first request was pending; another click showed `media provider command is busy`. No playback action was performed. |
| AppleScript | Two bounded, read-only `query applications` calls reached the app's handler and returned `permissionDenied: Only local current-user Apple events are accepted` (-1743), each in about 0.1 seconds. This app-generated refusal differs from the earlier build's timeout; it does not prove a macOS consent denial or a successful query. |
| Settings import and cleanup | Importing the actual settings export restored the main-display binding and disabled Dock/media settings. The owned duplicate, folder reference, and package were removed through Preferences. A final read-only comparison found no differences from the captured baseline after removing only the owned duplicate. |

General media panel/lyrics behavior, physical gestures, full Space/display transitions, permission recovery for another build identity, and release signing/distribution remain outside this native pass. No full-roadmap completion is claimed.

## Diagnosis and source corrections

The native widget group was 119px wide. Each clock text was approximately 44px wide, but the second slot began 166px after the first. The same clipping reproduced in standalone WebKit using the real components and CSS; Chromium's intrinsic flex sizing had concealed it. An explicit preferred flex basis now reserves the slot count's extent, while retaining shrinking, scrolling, and the existing four-slot viewport cap. The failing WebKit regression passed after the correction. Twenty cross-engine cases cover all four edges, constrained hosts, and overflow beyond four slots.

A windowless own-process diagnostic captured actual read-only Apple event metadata. macOS supplied sender PID and EUID as `typeUInt32` (`magn`), although the SDK comments describe `typeSInt32`. The transport previously rejected those representations. It now accepts only exact four-byte signed nonnegative or unsigned integers, a positive PID within signed process-ID range, and the current effective user. Local/same-process source restrictions remain unchanged; remote and direct-call sources still fail. Regression coverage includes mixed representations, mismatched users, malformed sizes, missing/wrong types, negative identities, and invalid PID bounds. This diagnostic used source `kAEDirectCall`, which remains refused; it establishes the encoding mismatch, not an authenticated cross-process round trip.

Media permission work already retained an exclusive provider job until its native call drained, including after cancellation. The source correction exposes that pending work through the existing status snapshot/events and disables Connect with localized Connecting feedback. The frontend also retains its original bridge request and rejects stale snapshots/results. This corrects duplicate submission and misleading idle feedback; it does not diagnose or interrupt a pending synchronous macOS consent call.

Folder-child startup waited for a native token, then retired the child after one failed physical visibility check. The established interaction-host path already waits for physical readiness. Child startup now follows that pattern within its existing deadline, preserving parent authority, original tokens, and cancellation. Tests delay visibility, refuse folder access before readiness, and exercise timeout and retirement. This fixes a demonstrated source race; the observed native popover failure could also have occurred at an earlier parent/Space validation stage, so the native result remains unresolved.

Review also caught and corrected two edge cases: an initial permission-snapshot failure must leave Connect retryable, and successful child validation arriving after the absolute startup deadline must not admit folder access. The latter retains the original deadline even when validation blocks. The focused regressions failed before their respective corrections and passed afterward.

## Combined verification

- `task test`: 480 desktop, 7 shared, and 4 website tests passed; all 25 Go packages passed with race detection and coverage.
- Desktop Playwright: all 110 Chromium tests passed, including the ten new layout cases. The same ten layout cases passed in WebKit with the dedicated configuration. CI now runs the WebKit cases as well.
- TypeScript, workspace Biome, full golangci-lint, and diff checks passed. Existing macOS deprecation/linker and lint-configuration warnings remain in the logs.
- Independent review found no remaining blocker in sender identity handling, child readiness/retirement, or pending media ownership after the corrections above.

Browser tests use a fake Wails backend. The isolated frontend bundle was rebuilt for browser testing; no replacement macOS app bundle was produced or launched in this pass.

## Evidence and restoration

Local evidence is in the ignored `.superpowers/sdd/native-ui-final-caf9166-2026-09-08-195035/` directory. It includes the owned fixture/archive, actual exports, baseline provenance, final settings comparison, and automation identity diagnostic and red/green logs.

The baseline was captured after creating the owned `profile-2` duplicate, not before testing. The final comparison removed only that duplicate from the expected baseline. Final canonical settings were 6,467 bytes with SHA-256 `0a75abba2be980f06a0cf939164dc2d820767d2fe235216d0c73f83fc953a60b`; only the original `default` profile remained. The app's private reference and widget directories were empty. Native Dock placement was not changed.

These source corrections require a subsequent packaged native retest. The results above describe the tested caf9166 binary and distinguish observed passes from unresolved behavior.

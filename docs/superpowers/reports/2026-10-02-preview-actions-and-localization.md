# Preview actions and localized controls

The A09/G06 follow-up corrects two implementation gaps found while reviewing the retained roadmap: preview controls used explicit automation setters instead of pointer toggles, and several new surfaces still exposed English labels or backend error codes after changing the app language.

## Preview actions

The automation preview and replacement-Dock window child now dispatch close, minimize/restore, fullscreen, hide and quit through the existing guarded native pointer-action interface. A minimized window can therefore restore, and fullscreen reads the current native state instead of applying a potentially stale rendered value. Focus continues through its existing guarded route. Explicit AppleScript setters and the public RPC signatures are unchanged.

Both preview surfaces expose Hide app and Quit app for the exact selected window's application. The controls are inside the measured panel content and disappear when there is no selected rendered target. Dispatch requires the same live owner, presentation revision, window identity and process identity at admission and at the final native guard. Missing guarded native support produces an unavailable result.

## Language and feedback

Window/app switchers, selected previews, status indicators, window controls, the launcher landmark and retained Dock/Folder Pop settings now use the current English, Portuguese or Spanish translation. The persistent window switcher receives the saved language when it reopens. Bulk actions have readable accessible names, and minimized cards announce Restore window. Interpolation preserves application and window titles literally, including `$&` characters.

Widget package failures map known backend categories to readable messages, with a generic fallback for unknown or empty failures. Pending failures are translated when rendered, so a language change during an operation takes effect. Backend details and paths are not copied into the UI. Cancellation remains neutral, and delayed cancellation failures cannot replace feedback from a newer review or a retired package-management session.

## Verification and remaining acceptance

Focused regressions cover each reproduced failure, exact action identity, stale-owner refusal, unchanged explicit AppleScript behavior, translated accessible names and language changes during pending operations. Evidence is under `.superpowers/sdd/fix-issues-2026-10-02/` in the A09, G06 and combined verification directories.

Combined checks passed 587 desktop, seven shared and four website unit tests; all 25 Go packages with race detection and coverage; and 125 desktop Chromium, 23 desktop WebKit and four website Chromium cases. TypeScript, workspace lint, Go lint and both frontend builds passed. Initial verification caught outdated error/button-name expectations and test-only TypeScript errors; those were corrected and the affected checks rerun. No browser retries were used.

These checks exercise UI-to-RPC and RPC-to-native-port boundaries separately; they do not establish physical input or packaged native action acceptance. A09 and G06 remain unchecked until their broader acceptance requirements pass. The user has deferred hands-on testing; no delivered product build is launched by this follow-up. Existing compiler deprecation and frontend build-size warnings remain.

# Native Settings and media fixes — 2026-09-08

This checkpoint follows `a1dcfc6` on `feat/dockdoor-parity`. Actual Preferences testing and focused review exposed the fixes below. Combined automated verification passed. This is not a release or completion of the native acceptance matrix.

The user subsequently installed and ran the final local build. [The caf9166 native acceptance report](2026-09-08-native-caf9166-acceptance.md) records those results, cleanup, and follow-up defects; the observations below describe the earlier checkpoint.

## Changes

| Area | Resulting behavior |
|---|---|
| Profile/settings export | Both exports use a backend-owned native Save dialog instead of a Blob download unsupported by the pinned Wails macOS download bridge. Profiles retain backend sanitization; settings export the canonical configuration. Fixed suggested filenames, bounded JSON, exclusive atomic writes, neutral Cancel, visible success and coarse translated errors reuse the existing diagnostics writer. Pending settings edits finish before either export starts; failed saves and superseding Preferences snapshots retire queued exports. |
| Disabled replacement Dock | Status updates refresh an empty runtime display list through the same static topology reader as the initial Settings query. Saving a profile no longer removes the connected second display from binding choices. The bounded query runs outside the view lock and rejects retired runtime, revision, Preferences and session owners. |
| Widget settings/packages | Native package chooser cancellation is neutral. Review/install/remove controls prevent overlapping operations and closing a review during installation. Package installation/removal reloads canonical settings, including cleared grants, before later edits. Timezone editing retains incomplete drafts and commits a valid complete zone on blur/Enter; Escape restores the saved value and IME composition does not commit. |
| Media | Missing/invalid lyric cues serialize as arrays; the frontend also tolerates null cues. Cancelling lyric selection or an explicit import preserves the existing document without an error. Successful import/reload/changes clear a previous error. Ready providers show Connected, Connect is unavailable while media is disabled, and supported status/error text is translated into Portuguese and Spanish, including rejected Connect calls. |
| Imported selections | H17 `selection-` placeholders are structural repair markers, not private bookmark records. Repair/removal no longer reports cleanup failure for those absent records; actual private-resource cleanup failures remain visible. Item repair states use readable translated messages. |
| AppleScript documentation | [Command examples](../../automation.md) use dictionary commands directly. The previous vertical-bar escapes compiled as variable references, so that compilation was not command validation. Corrected examples compile with the intended private event codes. |

Primary implementation and regression coverage is in `app_json_export.go`, `app_launcher_status_test.go`, `app_launcher_items_test.go`, `app_media_assets_test.go`, `App.test.tsx`, `App.settings.test.tsx`, `LauncherProfileTransfer.test.tsx`, `Settings.export.test.tsx`, and the widget/media component tests under `apps/desktop`.

## Observed native evidence

- H17 used the actual WebKit file picker with an owned portable JSON fixture. Review showed one item, one widget and all three sanitization notices. Cancel did not persist a profile. Confirmed import created a new unassigned profile with its widget disabled and grants cleared, preserving existing display bindings and global replacement-Dock state.
- Selecting the owned folder again and explicitly saving persisted the repaired pin. The older running build then reported `cleanupFailed` for the discarded import placeholder. That observation motivated the placeholder cleanup regression and source fix above; the revised native flow still needs the final build.
- Clock grant and widget enable controls were exercised in Preferences; profile assignment and the global replacement-Dock toggle were also exercised. This does not prove visible rendering or interaction in a live nonactivating Dock host.
- Export on the older build produced no dialog, success/error or expected output file. Checks were limited to the expected export filename in Downloads, Desktop, app working directory and the owned evidence directory. Source inspection confirmed the Blob-only path and missing native download destination handling. The replacement native exports await visible Save/Cancel/collision acceptance in the final build.
- Corrected `query applications` targeting the installed app returned a generic Apple event timeout after the eight-second caller deadline. There is no proof that the application handler was entered, so neither its cause nor a successful automation round trip is established.

The installed `9f9e04d` app had its Accessibility and Screen Recording grants. The separate clean test build was closed by the user. Computer Use app selection reopens Preferences, preventing a reliable observation of the nonactivating host after dismissing Preferences. These limitations do not establish a host-rendering failure or permission recovery for another build identity.

All owned temporary profiles and the unused test folder reference were removed through Preferences. A final read-only configuration check confirmed one original `default`/`Profile`, no pins, the original main-display binding, replacement Dock disabled, and the original clock disabled with no grants. Native Dock placement was not changed.

## Verification and remaining acceptance

Focused export/H02 Go tests passed with the race detector. Export, Settings and App suites passed 65 frontend tests after the save-queue correction; all six deferred-export regressions were also rerun independently. TypeScript and scoped Biome checks passed. Native writer fixtures exercised the three fixed names, cancellation, atomic new-file writes, collision/symlink refusal and context retirement without constructing real UI. Widget, media and imported-placeholder regressions have separate focused results; they are not a substitute for the final combined gate.

Local evidence is retained in the ignored `.superpowers/sdd/native-ui-2026-09-08-165934/` directory, including the owned H17 fixture, export queue red/green logs, `json-export-final-frontend.log`, `json-export-platform-test.log`, `json-export-launcher-status-race.log` and pinned bindings generation output. These paths describe local evidence, not published release artifacts.

**Final combined verification:** 472 desktop, 7 shared and 4 website unit tests passed; all 100 desktop Chromium tests passed. All 25 Go packages passed with race detection and coverage. Workspace Biome passed. Full golangci-lint passed after formatting one new test fixture. Pinned bindings were regenerated before the frontend checks. The browser checks use a fake Wails backend; they do not prove native bridge behavior.

**Final local build:** `apps/desktop/build/bin/option-tab-local-caf9166.app` in the primary checkout, built from clean source commit `caf916602e27157232dda70eac94eee9e8977544`. TypeScript and the production frontend/native builds passed. The packaged binary reports arm64, minimum macOS 14.0 and `vcs.modified=false`; bundle metadata and its scripting dictionary were checked. This local version 0.4.8 bundle has no release signature or notarization and was left unlaunched. Executable SHA-256: `fb00b659e8edbe1ff968e6a11d4e08a34179f50a094a85bdc152a29b1f098a6b`. Local provenance is stored beside the bundle as `option-tab-local-caf9166.build.json`.

Visible native export acceptance, live replacement-Dock host/widget proof, successful packaged AppleScript replies, physical input/display/Space coverage and release signing/distribution remain open. No roadmap checkbox is closed by this checkpoint.

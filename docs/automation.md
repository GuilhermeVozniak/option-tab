# Local AppleScript automation

Option Tab exposes a typed AppleScript dictionary for its switcher, app previews and individual window actions. These commands are part of the feature branch; cross-process authorization and complete packaged UI acceptance are still being verified. Use the bundled application, whose Resources directory contains `OptionTab.sdef`.

The caller must be a local process belonging to the current user. macOS can require Automation permission for the calling application; window operations also need Option Tab's Accessibility permission. A query does not request permission or activate another application. Denial is reported as an Apple event error. Option Tab does not evaluate scripts, run shell commands or expose a network listener.

## Commands

| Command | Inputs | Result |
|---|---|---|
| `open switcher` | Optional `mode windows mode` or `mode apps mode` | Current switcher presentation; repeated opens preserve selection |
| `show app previews` | Exactly one app selector; optional paired `x position` and `y position` | Independent preview owner token and accepted bounds |
| `hide app previews` | `presentation token` returned by show | Retires that automation preview owner |
| `perform window action` | `operation`, one window selector, optional app constraint | Result of the requested action |
| `query applications` | None | Running applications, including windowless apps |
| `query windows` | Optional app selector; optional `include images true` | Window details and optional existing cached previews |
| `query active window` | Optional `include images true` | Authoritative Accessibility focused window |

App selectors are `app name "Name"`, `bundle identifier "com.example.app"`, or `process id 12345`. Matching is exact and case-sensitive. Duplicate names or running bundle instances are ambiguous; query applications and use the intended PID. Option Tab captures the process launch identity after resolution and checks it again before an action. It does not launch a target app to resolve it.

Window selectors are `window id "12345"` or `active window true`, never both. Window IDs and presentation tokens are decimal text because AppleScript integers cannot represent every backend identifier. An active-window command captures one focused window; a later focus change does not redirect the action to a different target.

Supported operations are `focus window`, `close window`, `minimize window`, `hide application`, and `fullscreen window`. Minimize sets minimized=true. Fullscreen requires `fullscreen state true` or `false`; it sets that state. Hide affects the selected window's entire application. Close preserves the target application's normal save dialogs. There are no tiling, bulk-close, quit, force-quit or arbitrary-command operations in this suite.

## Terminal examples

Use the dictionary terms directly, without vertical-bar identifier escaping. If several local builds share the bundle identifier, replace `application id "com.optiontab.app"` with `application "/absolute/path/to/Option Tab.app"` to select the intended installed bundle.

Read applications and windows:

```sh
osascript -e 'tell application id "com.optiontab.app" to query applications'
osascript -e 'tell application id "com.optiontab.app" to query windows bundle identifier "com.apple.finder"'
osascript -e 'tell application id "com.optiontab.app" to query active window'
```

Open a switcher without advancing an existing selection:

```sh
osascript -e 'tell application id "com.optiontab.app" to open switcher mode apps mode'
osascript -e 'tell application id "com.optiontab.app" to open switcher mode windows mode'
```

The selected mode uses its configured appearance and filtering. Disabled keyboard shortcuts do not disable these explicit commands. Paused or inactive sessions refuse new presentations and actions.

Show a Finder preview at explicit coordinates:

```sh
osascript -e 'tell application id "com.optiontab.app" to show app previews bundle identifier "com.apple.finder" x position 160.0 y position 120.0'
```

The reply's `presentation.token` belongs to this preview. Pass that exact text to hide it; replace `TOKEN_FROM_REPLY` below with the returned value:

```sh
osascript -e 'tell application id "com.optiontab.app" to hide app previews presentation token "TOKEN_FROM_REPLY"'
```

Coordinates are global top-left logical points, so secondary displays can have negative coordinates. The preview clamps to a real display's usable frame. Omitted coordinates use the current display with a main-display fallback. A later show replaces only the previous automation preview; a late hide cannot dismiss the replacement or the Dock's hover panel. `accepted` means presentation work was admitted, not that native rendering has already been observed.

Explicit window actions, using a window ID from a query:

```applescript
tell application id "com.optiontab.app"
    perform window action operation focus window window id "12345"
    perform window action operation minimize window window id "12345"
    perform window action operation fullscreen window window id "12345" fullscreen state true
end tell
```

These examples perform actions when executed; substitute the intended live window ID. The `active window true` alternative applies only when Accessibility supplies an authoritative focused root. Missing or changing focus is an error, not a guess based on a window list.

## JSON replies and cached images

Successful replies are JSON text with `schemaVersion: 1`, `status`, `truncated`, and `omitted`. Query payloads use `apps`, `windows`, or `active`; presentation commands use `presentation`. Window fields include `windowID`, `appID`, `appName`, `bundleID`, `title`, `bounds`, visibility states and captured `process` identity. Bounds have `X`, `Y`, `W`, `H` in logical points. A zero screen/Space ID means unknown. Window IDs, Space IDs and process start seconds are decimal strings.

Inventories are capped at 500 entries and replies at 4 MiB. `truncated` and omission counts distinguish a partial bounded response from a complete inventory. An inventory failure is an error; it is not returned as a successful empty list.

To explicitly request already-cached image data:

```sh
osascript -e 'tell application id "com.optiontab.app" to query windows bundle identifier "com.apple.finder" include images true'
```

This can return visible window content to the authorized caller. It never starts capture, refreshes an image or prompts for Screen Recording. Availability depends on prior preview presentation and the user's capture settings. Images must match the current captured process/window identity and be no older than 30 seconds. Responses include per-window `imageStatus`, optional PNG data URL `image`, and `capturedAt`; at most eight images and 512 KiB per encoded image are included. Extra image data is omitted explicitly. Local folder bookmarks, lyric files and settings secrets are not included in these replies.

## Errors, deadlines and macro tools

Operational failures are Apple event errors with a stable code prefix: `invalidArgument`, `ambiguous`, `notFound`, `unavailable`, `permissionDenied`, `unsupported`, `staleIdentity`, `retired`, `cancelled`, `timeout`, `busy` or `internal`.

The native transport accepts at most 16 pending requests, bounds typed input to 16 KiB and normally allows five seconds from receipt. A smaller caller timeout can shorten that budget. Cancellation, shutdown and target replacement retire pending work before dispatch. An action already delivered to the target cannot be recalled; a timeout is not evidence of rollback. Retrying a close or other mutation should follow a fresh query.

Macro applications that can run AppleScript can use the same commands directly. Select their AppleScript action, paste the relevant `tell` block, and parse the returned JSON if the workflow needs a token or window ID. Keep the returned preview token with that invocation. macOS permission belongs to the actual calling application, so a successful Terminal test does not establish that a different macro application is authorized.

## Current verification boundary

Typed decoding, guarded service dispatch, suspended-reply lifetimes and shutdown have deterministic tests. Earlier windowless fixtures reached macOS authorization but required consent, and older packaged calls timed out or were refused by sender validation. Those historical results do not describe the latest running build.

The [2496558 packaged retest on 2026-09-08](superpowers/reports/2026-09-08-native-2496558-retest.md) completed successful `query applications`, `query windows`, and `query active window` round trips against an owned TextEdit document. Both switcher modes were accepted and visibly rendered. Minimizing that document returned success and its native minimized state was verified. A subsequent focus command returned `unavailable` after partially restoring the window; exact focus was not verified. `show app previews` for TextEdit returned `staleIdentity` before presentation. Standalone preview controls, other automation callers' permission flows, and the full native action matrix remain acceptance work.

The [September 9 follow-up](superpowers/reports/2026-09-09-native-retest-fixes.md) fixes the focus restore timeout and moves preview identity admission after window filtering. A native owned-window fixture verifies that focus receives macOS acknowledgement and selects the exact target. Service-to-App regressions verify that excluded auxiliary windows no longer block previews while selected-window and process identity guards remain enforced. Final packaged preview controls and other callers' permission flows remain outside that evidence.

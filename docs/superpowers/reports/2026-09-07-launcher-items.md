# Replacement Dock item checkpoint

The feature branch adds persistent application, folder, file and HTTP(S) link pins, app groups, spacers, separators, custom PNG icons and item management. This is an implementation checkpoint in draft PR 28, not a release or completion of the replacement-Dock roadmap. Folder fan-out and show-all child previews remain the next part of the item plan.

## Ownership and behavior

Items belong to a launcher profile. The optional `items` field keeps the unreleased configuration at version 2 and retains the 16-record bound, including group members. Groups contain individual app pins; opening a group never launches all its members. Configured order precedes unpinned running apps. A pin merges with a running app only when the selected installation resolves to that exact process identity and bundle. A stopped app remains an actionable pin. Missing, moved, excluded, ambiguous or changed references stay disabled.

Preferences select local applications, folders, files and PNGs through explicit native choosers. Selection creates only an inert private reference or icon; saving the profile admits it to the launcher. Reference and icon cleanup follows successful configuration persistence, checks usage across profiles and deletes only private records. The selected original resource is never deleted. Item saves compare the current item-list digest to reject stale edits.

Portable configuration contains opaque reference IDs and normalized icon digests, never bookmark bytes or selected paths. Private records use atomic files and restricted permissions. Foundation bookmarks provide persisted selection; transient identity and an independent runtime revision protect current operations. A moved or stale bookmark requires explicit relink. PNG selection is bounded, decoded and normalized to at most 256 pixels per dimension with metadata removed. Links accept only bounded HTTP(S) URLs without credentials or control characters.

The controller resolves items outside its mutex in one cancelable, joined inventory worker. Presentation revisions change when selected resource or process authority changes. Copies include independent group members. Profile/session retirement prevents old results and old actions from reaching a successor. Native operations also recheck current settings, the exact host token, physical visibility, ordinary display Space and selected resource after preparation.

App activation uses an exact running process when present. New launches use the selected application URL with running-application substitution disabled. Ambiguous or newly appeared instances are refused. Relaunch requests graceful termination once, waits up to ten seconds for the captured instance to exit, then rechecks admission and launches once. Refusal, timeout, replacement process or retirement stops the sequence. No forced quit, shell command, saved-command runner or Dock preference mutation is added.

## Verification

Focused tests establish configuration/clone bounds, private-store persistence and rollback, immutable reference authority, canceled chooser cleanup, icon normalization, exact process deduplication, stopped-app admission and saved-reference dispatch. App regressions cover parent retirement, manager replacement and settings publication before controller reconciliation.

The injected native fixture covers canceled queued selection, bookmark replacement, exact installation matching, ambiguous/reused processes, canceled preparation, graceful relaunch refusal and timeout. It uses disposable filesystem fixtures and injected workspace/process operations; it does not open a real chooser or manipulate a user application.

- All 25 Go packages passed with race detection and coverage. The subsequent action-lifetime and native-name changes passed focused App/controller/platform race checks. Global and changed-package Go lint reported zero issues.
- JavaScript unit tests passed: 349 desktop, seven shared and four site tests. Workspace lint and production builds passed.
- All 78 desktop Chromium scenarios passed across the full run and a four-case rerun. Four settings pages timed out during an overlapping production build; those exact cases passed against an isolated server without code changes. The new pin/group scenario verifies retained choices across content ticks, disabled missing references, exact open/relaunch RPC arguments and bounded cross-axis geometry.
- Wails generation completed without warnings: 110 methods and 63 models. Apple Silicon and Intel CGO builds passed, both with macOS 14.0 minimum deployment targets. These binaries were not launched or distributed.

Review found and fixed stopped-app exclusion, ambiguous process fallback, application-name blacklist mismatch, unbounded private-store reads, stale item-save publication, group loss on clock refresh, and relaunch canceling itself after graceful termination. The relaunch fix keeps initial UI admission exact, then allows benign presentation ticks and only the expected captured-process-to-stopped transition during the admitted operation. Reference, profile, native host and replacement-process changes still retire it.

## Remaining acceptance and item work

Folder pins currently open their selected root through the native workspace operation. H09 list/grid fan-out, H11 show-all child previews and complete child lifetime/capture integration remain in the item plan. Real chooser selection/cancellation, bookmark reuse across an OS restart, owned-resource opening, owned-app relaunch, two-display behavior and Wails presentation acceptance remain pending. H04/H05/H09/H11 therefore remain unchecked in the roadmap.

NSWorkspace completion cannot be recalled after dispatch. Cancellation rejects further dispatch and stale completion but may still wait for macOS to finish an already accepted operation. Filesystem and process validation is repeated immediately before dispatch; those checks are not atomic OS transactions. No native GUI, cursor, Dock placement or user-resource action was performed for this checkpoint.

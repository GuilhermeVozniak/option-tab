# Folder Pop checkpoint

This checkpoint implements retained E01–E03: hover an exact Dock folder, browse and sort its contents, request access explicitly, and open the exact listed file or folder. Folder Pop defaults off and can run independently of Dock window previews.

## Behavior

- Native AX classification accepts an exact folder Dock subrole and canonical local file URL. Localized titles never become file paths. Positive non-directory evidence refuses classification; restricted metadata access can still lead to an explicit access request.
- Folder panels reuse the existing nonactivating Dock shell, placement and hover corridor. They expose no window actions, capture streams, app gesture targets or preview-drag regions.
- Read-only enumeration supports name, modification time, size and kind sorting, direction and folders-first. Reads stop between filesystem calls after a 500 ms deadline or 500 entries; partial state is explicit. Individual OS calls can outlive that deadline.
- A presentation shows loading immediately. One serialized reader cancels obsolete work and rejects old sessions, content revisions and source generations. A folder stays stable until explicit sort/retry/rehover; its opaque IDs do not churn on a periodic timer.
- Access requests are explicit. Ordinary hover dismissal allows an already accepted chooser to finish for its original folder. Disable, pause, session inactivity, shutdown and exact explicit cancellation cancel that owner. A second request cannot overlap its cleanup.
- Successful access can refresh the original presentation after a geometry-only resize. Replacement contents, folder identity, admission epoch or session cannot receive that completion.
- Bookmarks use a separate atomic 0600 store. Resource access is balanced per operation; persisted grants are resolved without UI. Invalid or revoked bookmarks, missing folders and incomplete reads have explicit states.
- Opening accepts only backend-issued IDs from the current listing. It checks the parent and child device/inode, direct parent relationship, symlink refusal and the App's exact session/revision/item guard immediately before native dispatch. The native adapter repeats file identity checks after the external App guard.
- The frontend preserves error-event revisions so retry remains usable, scopes async errors to current content, keeps accepted grant cancellation tied to its original request, and localizes controls in English, Brazilian Portuguese and Spanish. Ordinary Retry relists without opening a chooser.

## Review fixes

Independent review and native fixtures caught and fixed a canonical bookmarked-path mismatch, replacement during the final guard, stale frontend revisions after errors, grant refresh lost after panel resize, errors crossing folder presentations, misleading partial-result copy, and Retry opening a chooser. Folder failures survive geometry updates and cannot be cleared by unrelated app-input recovery.

App input now also rejects non-app item kinds and any captured input-policy change before final dispatch, including the interval after new settings are published and before native reconfiguration arrives. Tests keep window previews enabled while disabling or changing input controls.

## Verification

- 254 desktop, 5 shared and 3 site unit tests pass on the isolated Folder Pop checkpoint.
- All 18 Go packages pass with race detection and coverage. Focused Folder/App/input regressions pass after the final policy fix.
- Chromium: 57 desktop and 4 site tests pass, including no chooser on hover, explicit access, exact opaque-item RPC arguments, stale-update rejection, missing/revoked states and the independent feature switch.
- Biome and golangci-lint pass. Production frontend/site and embedded desktop builds pass. Portable platform compilation passes with CGO disabled on Linux.
- Actual read-only Dock AX traversal recognizes the Downloads folder subrole and file URL without changing the Dock or reading Downloads contents.
- A disposable AppKit runloop fixture uses production native code with injected chooser responses and intercepted workspace dispatch. Cancellation, wrong-folder refusal, bookmark reuse and exactly two fixture URL dispatches pass, with unchanged foreground. These are injected UI/dispatch tests.
- A separate packaged, ad-hoc signed host invoked the real chooser code and cancelled through the production context path. The native request returned cancellation and wrote no bookmark; the host exited successfully. The computer-control service failed to start on two attempts, so visible chooser approval and real file/folder opening were not exercised.

The fixture commands and evidence boundaries are documented in the [native README](../../../apps/desktop/internal/platform/testdata/folder-pop/README.md). The implementation plan is [here](../plans/2026-09-06-folder-pop.md).

## Remaining native acceptance

Visible approval in the real chooser, access reuse after that approval, exact real recipient-window opening/cleanup, physical folder scrolling and distributed signing/TCC behavior remain pending. Path-based NSWorkspace dispatch has a remaining non-atomic interval between final filesystem identity checks and URL resolution. The implementation does not claim atomic file-descriptor opening.

E01–E03 remain unchecked on the [retained roadmap](../../dockdoor-roadmap.md). No staging, copy/move, pinned file shelf, AirDrop or script execution was added. This checkpoint is development work, not a release.

# Launcher profile transfer

Implementation checkpoint for retained H17. Preferences can export the selected replacement-Dock profile and import a reviewed document as a new, unassigned profile. Import preserves the existing Dock enablement, profiles, display bindings, focused-app rules and unrelated settings. This checkpoint does not release the app.

The portable JSON preserves layout, appearance, item order, groups, links, folder presentation, widget package references, typed widget settings and stacks. Both export and import independently remove custom icon IDs, widget grants and enabled widget execution. App, folder and file references become structural placeholders that cannot resolve through the private native reference store. Each import receives a fresh profile ID and fresh selection placeholders. Existing Choose/Relink controls restore local selections explicitly; widgets require local installation/access and enablement.

The exact UTF-8 document is limited to 256 KiB. Parsing rejects duplicate or unknown fields, unsupported versions, trailing JSON and invalid nested configuration. Review returns the document digest and current Dock configuration revision. Apply reparses the exact document, checks both revisions and current Preferences ownership, then appends to the latest settings under the existing serialized writer. The eight-profile limit and the exact persisted document size are checked before saving. Parse, review and import create no native reference or icon artifacts and invoke no app, folder, package or provider action.

The Preferences flow previews the name, counts and repair guidance before explicit import. It serializes with pending settings saves and uses the returned validated canonical settings if the normal post-save reload fails. Changing profile or leaving the screen retires pending UI operations; a late file read cannot start review and a late export cannot start a download. Copy and accessible controls include English, Portuguese and Spanish.

## Verification

- All 25 Go packages pass race/coverage tests; golangci-lint reports zero issues.
- 372 desktop, 7 shared and 4 website unit tests pass. Workspace Biome and production TypeScript/Vite builds pass.
- The full desktop Chromium suite passes 82 checks, including the file-input/download round trip and exact preview/apply RPC arguments.
- Pinned Wails generation reports 122 methods and 67 models.
- Real CGO arm64 and x86_64 binaries compile with minimum macOS 14.0, verified with `vtool`. Neither binary was launched or distributed.
- Regressions cover strict malformed/oversized input, independent sanitization, deep-copy isolation, stale document/settings revisions, fresh imported identities, capacity, failed persistence, Preferences retirement, queued saves, canonical reload recovery and unmount during deferred export/file reads.

An independent backend review found no concrete blocker. A persistence-size regression caught the difference between compact RPC JSON and the larger pretty-printed on-disk representation; apply now validates the actual persisted representation before saving.

## Remaining acceptance

Actual Wails WebKit file selection/download behavior remains native acceptance, as do the previously recorded replacement-Dock, source, gesture and distribution gates. Automated browser tests do not establish those outcomes. H05 runtime dragging/grouping, H06 magnification, H15 gestures/navigation and H16 supported badge sources remain retained work; H06 is in progress in its separate branch.

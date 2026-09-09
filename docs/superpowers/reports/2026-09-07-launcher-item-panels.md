# Launcher folder and app-window panels

Implementation checkpoint for retained H09 and the show-all portion of H11. The feature branch now opens a pinned folder in a list/grid child panel and opens a running app's filtered windows from its context menu. Relaunch remains the separate previously implemented exact-app action. Native desktop acceptance remains open; no release is part of this checkpoint.

## User behavior

- Folder primary click opens contents beside its parent Dock. The context menu retains explicit Open folder for the existing root-open action. List/grid, sorting, exact entry opening and Settings relink guidance reuse Folder Pop components and source checks.
- A running app or app-group member offers Show all windows. The child reuses existing window cards and appearance, with selection, focus, close, minimize, hide and explicit fullscreen actions. Stopped apps are not launched by show-all. Empty or unavailable inventories are explicit.
- Parent clock/widget content ticks preserve child context. Parent hide, profile/settings retirement, item/reference/process change, Space/topology evidence, native-host replacement and shutdown retire children. Pointer inside the child or its short connecting span preserves auto-hide grace; native Dock recovery always wins.
- Product controls and known error guidance include English, Portuguese and Spanish. A child cannot restore after terminal hide, including transport replacement or late snapshots/frames.

## Ownership and resource limits

Each parent owns one current child with an independent native LauncherPanel, session, context, isolated FolderSource and optional capture peer. Initial commands require the exact rendered scope/item; continuing guards tolerate content revisions while rechecking current parent, profile, bounds, reference/process and native host tokens. Physical panel validation runs outside App locks and repeats after external identity preparation.

Accepted display changes retire monotonic child authority before event coalescing. A Go-only publication watermark ensures the App closes children even when a Space A→B→A returns to identical parent pixels. Removed-display records are pruned without reusing old authority. Closed child sessions retain a high-water mark and cannot restore pointer ownership.

Native resources and source jobs issue completion receipts only after actual exit. Replacement chains join ancestors even when an intermediate child is cancelled. Repeated Show requests refuse while an accepted replacement waits for its predecessor; at most16 active/retiring child records are retained across all parents. Native calls that already entered can outlive cancellation; receipts do not invent a deadline for OS completion.

Folder references/bookmarks and canonical paths stay backend-only. A fresh private FolderSource per child prevents another panel's listing from replacing opaque entry IDs. Listing completion and final entry dispatch revalidate the selected native reference; imported/moved/revoked references require explicit selection/relink.

Window children share the existing process budget of four live captures and one snapshot, with two live priorities per peer. Additive `preview.Manager.UpdateBound` stores copied expected identities before capture starts; queued numeric-ID reuse cannot retarget capture to another process, and unsupported bound capture never falls back to a numeric-ID-only API. Capture retirement joins actual jobs and admitted callbacks without closing sibling owners.

Frames are limited to30 candidate windows,512KiB per encoded image and4MiB catch-up data. Immutable frame admission avoids App locks/native lookups in callbacks. Failed refresh clears old entries, identities and capture authority. Current snapshots carry static frames across content revisions; bounded frontend buffering preserves a static frame delivered before its matching state update. Size measurement is deduplicated independently of frame cadence.

## Verification

- All25 Go packages pass race/coverage tests; full golangci-lint reports0issues.
- 365 desktop,7 shared and4 website JavaScript tests pass. Workspace build and Biome lint pass.
- 80 Chromium checks passed before final UI review;11 focused launcher/child Chromium checks passed after the menu, error and frame-order fixes. These exercise actual hash routes and literal generated RPC arguments, including list/grid/narrow sizing, root-folder open across a clock tick, window focus and hide.
- Pinned Wails generation reports119 methods and65 models. Direct generated calls are used for all nine child RPCs.
- Real CGO arm64 and x86_64 binaries compile. `vtool` reports minimum macOS14.0 for both. These binaries were not launched or distributed.
- Deterministic regressions cover blocked folder reads and relinks, exact native-host replacement, actual core→App child retirement, three-generation drain ancestry, request flooding/capacity, cancelled capture/emit joins, queued/fallback capture identity reuse, failed window refresh, static snapshot/frame ordering, final physical retirement during identity lookup, tombstones and scoped errors.

Initial full frontend tests exposed two App mock modules missing the new exports; those mocks were updated and the full suite reran successfully. The preceding H04 CI test synchronization and Linux helper-coverage issues were fixed separately; commit52e0cca is green in GitHub CI. No test hook was bypassed.

## Remaining acceptance and retained work

Actual Wails child Show/Hide/Close, ordinary/fullscreen Space transitions, physical multi-display layout, security-scoped folder behavior, real window actions/capture and mixed-scale/high-refresh rendering remain native acceptance gates. Automated/fake-source results do not establish those outcomes. No real Dock placement, pointer movement, app/player action, folder chooser or audio-output change ran for this checkpoint.

H05 runtime Dock dragging/grouping, H06 magnification, H15 gestures/navigation, H16 supported badge sources and H17 profile transfer continue as retained work. H17 is being implemented in its own branch. Existing native acceptance and distribution follow-ups remain listed in the roadmap.

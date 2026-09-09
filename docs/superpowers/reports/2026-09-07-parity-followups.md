# Parity follow-ups checkpoint

Status: implementation checkpoint. Combined automated gates and real universal compilation pass. This is not a release. G05 documentation is complete; native acceptance remains separate from implementation.

## Implemented scope

- Media-provider Dock hovers can expose backend-owned **Windows** and **Media** choices when both are enabled. A real content change creates a fresh outer Dock session; stale media/window state, frames and failures cannot replace the new owner. Selecting Windows performs the normal bounded window query, while returning to Media does not query windows.
- The release contract for G01/G03 now defines future universal macOS assets, exact updater selection, a macOS 14 floor, pinned binding generation, mandatory architecture/minimum-OS inspection and fail-closed signing/notarization steps. Existing published v0.4.8 links remain ARM64 until a verified universal asset is published.
- G02 has an in-repository Homebrew cask for the existing v0.4.8 ARM64 artifact, pinned to its SHA-256 and macOS 14/Apple silicon. Installation and maintenance documentation uses the explicit repository tap and preserves user data on normal uninstall.
- G04 adds an explicit, native-only diagnostics workflow. Recording is off by default and bounded in memory. The user reviews exact immutable allowlisted JSON before saving; export has no browser/Blob fallback or automatic upload. Existing destinations are refused.
- G05 documents settings and grant stores, previews and automation images, media/provider network boundaries, default update checks, crash-report navigation, diagnostics retention/export and user controls. The README links the data-handling, distribution and Homebrew guides.
- G06 adds explicit Brazilian Portuguese and Spanish strings for the new Folder, Media, automation preview, Dock input, monitor-lock, content-selector and diagnostics controls. Dictionary-completeness checks inspect actual locale entries rather than accepting English fallback.

## Current automated evidence

The coordinator's combined checks pass:

- Desktop frontend unit tests: **283 passed**.
- Shared package unit tests: **7 passed**.
- Website unit tests: **4 passed**.
- Desktop Chromium tests: **68 passed**.
- Biome: clean across the current batch.
- Go race/coverage: **21 packages passed**, including App, diagnostics and the native fixture package. The diagnostics App cancellation assertion also passed after tightening its error check.
- Full golangci-lint: **0 issues**.
- Real CGO builds: arm64 and amd64 compiled; `lipo` confirmed the assembled binary contains **arm64 and x86_64**. `xcrun vtool` reports **minos 14.0** for both slices (SDK 26.5). These compile-only files are local, unsigned artifacts; they were not installed or run as a release.

Focused work also covered exact content-selector session/revision admission, immutable diagnostics tokens, neutral chooser cancellation, localized accessible controls, exact release-asset matching, packaging refusal paths and the pinned v0.4.8 Cask metadata.

Independent diagnostics review reproduced and fixed cancellation before queued Save dialog presentation and replacement of the temporary source before commit. Native fixtures now use the actual Go context guard before presentation/commit, retain the source descriptor, refuse observed source substitution, and preserve foreign resources during cleanup. Private staging is not a security boundary against arbitrary same-user filesystem modification.

The previous Linux CI failure was traced to one malformed Darwin build constraint in `darwin_window_drag.m`. Correcting it excludes that Objective-C file from Linux package metadata; the platform test binary cross-compiles for Linux. The new GitHub CI run remains the check for the complete Linux environment.

## Pending coordinator evidence

- **Release acceptance:** pending Developer ID signing, notarization, stapling, Gatekeeper/install/update execution and runtime checks on Apple silicon and Intel hardware.
- **Homebrew acceptance:** pending isolated install, upgrade and uninstall using the merged tap on supported hardware. Current checks establish cask syntax/metadata and the published artifact checksum only.
- **Diagnostics native acceptance:** pending a real NSSavePanel approval/cancel flow and disposable-path write behavior in a signed/distributed app. Automated native seams do not open a chooser.
- **Media selector native acceptance:** pending an actual same-hover switch between Media and Windows with a disposable provider/window fixture, including polling retirement/restart and proof that the selector itself performs no focus/action.

These pending checks keep G01–G04 and the native portions of the media work unchecked. G05 documentation is complete. Localization is implemented, with the final rendered native-dialog language matrix still pending under G06.

## Detailed references

- [Distribution contract](../plans/2026-09-07-distribution-contract.md)
- [Homebrew route](../plans/2026-09-07-homebrew-route.md)
- [Diagnostics and data handling](../plans/2026-09-07-diagnostics.md)
- [Media content selector](../plans/2026-09-07-media-content-selector.md)
- [Data-handling guide](../../data-handling.md)
- [Homebrew guide](../../homebrew.md)

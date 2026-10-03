# macOS distribution contract

Option Tab requires macOS 14 or later. Starting with v0.5.0, macOS releases provide
`option-tab_VERSION_darwin_universal.dmg` containing both arm64 and x86_64 slices.
The bundle script checks the actual Mach-O architectures and each slice's
`LC_BUILD_VERSION` minimum OS, in addition to setting the compiler deployment target.

[v0.5.0](https://github.com/GuilhermeVozniak/option-tab/releases/tag/v0.5.0) was
published on October 3, 2026 after the [release workflow](https://github.com/GuilhermeVozniak/option-tab/actions/runs/37142247653)
succeeded at commit `470b0e7`. The downloaded DMG's SHA-256 matched the GitHub asset
digest: `6ef2d41f3e1593bafb9d79c570e8e5e7c2eef1e5b05457584ab2f9d1b1e4e231`.
Inspection confirmed `Option Tab.app` version 0.5.0, arm64 and x86_64 slices with
a macOS 14.0 deployment floor, Developer ID team `CT22R575UG`, a valid stapled
notarization ticket, and Gatekeeper acceptance. The app was not launched during
this artifact inspection; Intel execution and install/upgrade testing remain open.

Website and Homebrew metadata now target this published universal asset. For later
releases, update `APP_VERSION` and `PUBLISHED_ARCH` in `apps/web/lib/download.ts`
only after the corresponding asset has been published and verified.

The updater selects a unique exact filename for the running architecture and the
release's tag. If that filename is absent, macOS may use a unique exact universal
filename. Duplicate candidates refuse selection. Checksums, prefixed copies, assets
from another version, and architecture substrings are not download candidates.

Versions through 0.4.8 only recognize `darwin_arm64` when updating Apple Silicon
installations. v0.5.0 also publishes the identical signed universal DMG as
`option-tab_0.5.0_darwin_arm64.dmg`; both asset digests match. Retain this compatibility
filename in later releases while supporting direct upgrades from those versions,
because users may skip intervening releases.

## Build locally

Run `BUNDLE_MODE=unsigned ./scripts/bundle.sh`, or `task bundle:unsigned`.
Add `UNIVERSAL=1` to build both slices locally. These outputs have an
`_UNVERIFIED.dmg` suffix and print an unsigned/not-notarized notice; they are not
release artifacts. This path never uses signing credentials, even if inherited.

Bindings generation runs before the desktop frontend build, using the exact Wails
module version resolved from `apps/desktop/go.mod`. Generation failure stops the
build; a globally installed CLI or stale committed bindings cannot silently replace it.
`task build` uses the same order. The script requires the usual Xcode command-line
build tools, Go, Bun, and the existing icon/DMG tooling.

## Release

A release runs `BUNDLE_MODE=release UNIVERSAL=1 VERSION=X.Y.Z ./scripts/bundle.sh`.
It requires `CODESIGN_IDENTITY`, `APPLE_ID`, `APPLE_TEAM_ID`, and
`APPLE_APP_PASSWORD` before any build begins. CI also requires the certificate and
password used to import the Developer ID identity. Do not print or commit credentials.

The app is signed with the existing hardened-runtime entitlements, verified, and
packaged. The DMG is signed, submitted to Apple's notarization service, stapled, and
validated. Any failed command stops packaging. Tagged CI uploads only the exact
expected asset filename to a draft release; wildcard matches cannot upload a stale
or local unverified DMG. Publish after all platform jobs and downloaded-artifact
checks pass, with the compatibility alias described above. No unsigned fallback is
allowed for a tagged macOS release.

`python3 scripts/test_bundle.py` runs on macOS in a disposable miniature checkout
with fake compiler, packager, and signing tools. It proves command ordering, naming,
required-credential refusal, architecture/floor rejection, and explicit local mode.
It does **not** prove codesigning, notarization, Gatekeeper acceptance, Intel execution,
or installation. Before a real release, inspect an actual universal build with
`lipo -archs` and `xcrun vtool -arch arm64/-arch x86_64 -show-build`; then validate
real signed/notarized artifacts on supported Intel and Apple silicon machines.

The v0.5.0 artifact checks establish signing, notarization and package contents.
They do not establish runtime acceptance on every supported macOS version or
architecture, Homebrew lifecycle behavior, or completion of the native feature
checks in the [retained-feature roadmap](dockdoor-roadmap.md).

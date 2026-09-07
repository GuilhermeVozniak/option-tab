# macOS distribution contract

Option Tab requires macOS 14 or later. Future macOS releases publish one
`option-tab_VERSION_darwin_universal.dmg` containing both arm64 and x86_64 slices.
The bundle script checks the actual Mach-O architectures and each slice's
`LC_BUILD_VERSION` minimum OS, in addition to setting the compiler deployment target.

The website still advertises the existing v0.4.8 Apple silicon DMG. Do not change
`APP_VERSION` or `PUBLISHED_ARCH` in `apps/web/lib/download.ts` until the corresponding
asset has actually been published. The upcoming universal filename is not evidence
that an Intel-compatible release already exists.

The updater selects a unique exact filename for the running architecture and the
release's tag. If that filename is absent, macOS may use a unique exact universal
filename. Duplicate candidates refuse selection. Checksums, prefixed copies, assets
from another version, and architecture substrings are not download candidates.

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
validated. Any failed command stops packaging. Tagged CI publishes only the exact
expected asset filename; wildcard matches cannot publish a stale or local unverified
DMG. No unsigned fallback is allowed for a tagged macOS release.

`python3 scripts/test_bundle.py` runs on macOS in a disposable miniature checkout
with fake compiler, packager, and signing tools. It proves command ordering, naming,
required-credential refusal, architecture/floor rejection, and explicit local mode.
It does **not** prove codesigning, notarization, Gatekeeper acceptance, Intel execution,
or installation. Before a real release, inspect an actual universal build with
`lipo -archs` and `xcrun vtool -arch arm64/-arch x86_64 -show-build`; then validate
real signed/notarized artifacts on supported Intel and Apple silicon machines.

No signing, notarization submission, publication, version bump, or application
installation was performed while implementing this contract.

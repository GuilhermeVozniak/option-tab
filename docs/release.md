# Release process

## Overview

Pushing a `v*` tag triggers `release.yml`, which builds the desktop binary for each supported platform and uploads the artifacts to a draft GitHub Release. Publish the draft after all jobs and artifact checks pass. The landing page is deployed separately via `deploy-web.yml` on every push to `main` that touches `apps/web/` or `packages/shared/`.

---

## Desktop release (`release.yml`)

### Trigger

```bash
git tag v1.2.3
git push origin v1.2.3
```

Any tag matching `v*` triggers the release workflow.

### What the workflow does

1. Gates on the desktop-UI e2e suite (a broken overlay or preferences flow blocks the release).
2. Runs a matrix build (`macos-latest`, `windows-latest`, `ubuntu-latest`) in parallel.
   Each runner:
   - Installs Bun and Go 1.26.
   - On Linux, installs WebKit system deps (`libgtk-4-dev`, `libwebkitgtk-6.0-dev`).
   - Runs `bun install --frozen-lockfile` and builds the frontend (embedded into the Go
     binary via `//go:embed`).
   - **macOS**: `scripts/bundle.sh` generates pinned bindings, builds both arm64 and
     x86_64 slices, assembles `option-tab.app` (Info.plist + icon), and requires
     signing, notarization and staple validation. Missing credentials stop the release.
     There is no `wails build`
     step — Wails v3 serves the embedded frontend from the plain Go binary.
   - **Windows/Linux** (stub-platform demo builds): plain `go build`, packaged as
     `.zip` / `.tar.gz`.
3. Uploads exact `.dmg` / `.zip` / `.tar.gz` filenames to a draft GitHub Release via
   `softprops/action-gh-release`. All matrix jobs keep the release in draft.
4. After the workflow succeeds, download the macOS asset and verify its digest,
   bundle version, architectures, minimum OS, signature and notarization. Upload
   the compatibility filename below, attach release notes, then publish the draft
   with `gh release edit vX.Y.Z --draft=false --latest`. Do not reuse a published
   tag: draft upload settings do not unpublish an existing release.

To assemble a local unsigned dmg: `task bundle:unsigned` (or
`BUNDLE_MODE=unsigned UNIVERSAL=1 ./scripts/bundle.sh`). Never publish its
`_UNVERIFIED.dmg` output.

### Asset naming

The pipeline produces files matching the `@option-tab/shared` contract:

```
option-tab_<version>_darwin_universal.dmg
option-tab_<version>_windows_amd64.zip
option-tab_<version>_linux_amd64.tar.gz
```

where `<version>` is the tag name with the leading `v` stripped (e.g., tag `v1.2.3` → version `1.2.3`).

For the first universal release, also upload an identical copy of the verified,
signed universal DMG as `option-tab_<version>_darwin_arm64.dmg`. Versions through
0.4.8 only recognize that name when updating Apple Silicon installations. Verify
that both asset digests match; the compatibility download contains both slices.

---

## How the landing page resolves download links

`@option-tab/shared` (`packages/shared/src/index.ts`) defines `releaseAssetName(platform, arch, version)` and `downloadUrl(platform, arch, version)`. The landing page imports these functions and passes `APP_VERSION` from `apps/web/lib/download.ts` to construct download URLs.

After the desktop assets are published and verified, update `APP_VERSION` and
`PUBLISHED_ARCH` in `apps/web/lib/download.ts`, its download tests, and the Homebrew
cask's version, filename and actual SHA-256. Merge this follow-up only after the
download exists. The `apps/web/` change triggers `deploy-web.yml` on `main`.

---

## Known limitations

| Item | Description |
|------|-------------|
| Windows/Linux are stub builds | No native window-switching backend on those platforms yet (synthetic demo data only) |
| Windows code signing | Optional but reduces SmartScreen warnings |
| linux/arm64 | No Linux ARM64 asset is published; macOS uses one universal asset for Apple Silicon and Intel |
| Native acceptance | See the changelog and retained-feature roadmap for disabled features and hardware checks still pending |
| Auto-update | macOS self-updates in place (downloads the dmg, swaps the `.app`, relaunches); Windows/Linux stub builds update manually from the releases page |

---

## Landing page deploy (`deploy-web.yml`)

### Trigger

- Push to `main` that touches `apps/web/**` or `packages/shared/**`.
- Manual dispatch via the GitHub Actions UI.

### What the workflow does

1. Installs Bun and runs `bun install --frozen-lockfile`.
2. Runs `cd apps/web && bun run build`, which produces a static export in `apps/web/out/`.
3. Uploads `apps/web/out/` as a GitHub Pages artifact and deploys it.

The deployed site is available at [option-tab.vozniak.dev](https://option-tab.vozniak.dev) (a custom domain on GitHub Pages). No server-side rendering is involved; the site is a fully static export.

---

## Versioning convention

- Use [Semantic Versioning](https://semver.org): `vMAJOR.MINOR.PATCH`.
- Tag on `main` after merging the release PR.
- The release PR bumps `appVersion` in `apps/desktop/app_update.go`, both bundle
  versions in `apps/desktop/build/darwin/Info.plist`, and the changelog. Website
  and Homebrew metadata follow verified publication as described above.
- Commit message for the bump: `chore(release): bump version to X.Y.Z`.

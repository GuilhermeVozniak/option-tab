# Release process

## Overview

Pushing a `v*` tag triggers `release.yml`, which builds the desktop binary for each supported platform and uploads the artifacts to a GitHub Release. The landing page is deployed separately via `deploy-web.yml` on every push to `main` that touches `apps/web/` or `packages/shared/`.

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
   - **macOS**: `scripts/bundle.sh` builds the binary, assembles `option-tab.app`
     (Info.plist + icon), signs when the Apple secrets are present, and creates the dmg
     via `appdmg` (then notarizes + staples when credentialed). There is no `wails build`
     step — Wails v3 serves the embedded frontend from the plain Go binary.
   - **Windows/Linux** (stub-platform demo builds): plain `go build`, packaged as
     `.zip` / `.tar.gz`.
3. Uploads `*.dmg` / `*.zip` / `*.tar.gz` artifacts to the GitHub Release via
   `softprops/action-gh-release`.

To assemble a local unsigned dmg: `task bundle` (or `UNIVERSAL=1 ./scripts/bundle.sh`).

### Asset naming

The pipeline produces files matching the `@option-tab/shared` contract:

```
option-tab_<version>_darwin_arm64.dmg
option-tab_<version>_windows_amd64.zip
option-tab_<version>_linux_amd64.tar.gz
```

where `<version>` is the tag name with the leading `v` stripped (e.g., tag `v1.2.3` → version `1.2.3`).

---

## How the landing page resolves download links

`@option-tab/shared` (`packages/shared/src/index.ts`) defines `releaseAssetName(platform, arch, version)` and `downloadUrl(platform, arch, version)`. The landing page imports these functions and passes `APP_VERSION` from `apps/web/lib/download.ts` to construct download URLs.

When a new desktop release is made, update `APP_VERSION` in `apps/web/lib/download.ts` to match the release tag version. This change to `apps/web/` will automatically trigger `deploy-web.yml` on push to `main`, updating the landing page's download links.

---

## Known limitations

| Item | Description |
|------|-------------|
| Windows/Linux are stub builds | No native window-switching backend on those platforms yet (synthetic demo data only) |
| Windows code signing | Optional but reduces SmartScreen warnings |
| linux/arm64, darwin/amd64 assets | Only `darwin/arm64` is published for macOS; `UNIVERSAL=1 scripts/bundle.sh` can produce a universal binary locally |
| Auto-update | The app checks GitHub Releases and downloads the dmg; there is no in-app updater |

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
- The release PR should bump `APP_VERSION` in `apps/web/lib/download.ts` and update the changelog.
- Commit message for the bump: `chore(release): bump version to X.Y.Z`.

# Install with Homebrew

The recommended cask is published in [the personal Homebrew tap](https://github.com/GuilhermeVozniak/homebrew-tap):

```sh
brew tap GuilhermeVozniak/tap
brew install --cask GuilhermeVozniak/tap/option-tab
```

The cask targets the published [v0.6.4 universal release](https://github.com/GuilhermeVozniak/option-tab/releases/tag/v0.6.4), containing Apple Silicon and Intel binaries and requiring macOS 14 or later. It installs `Option Tab.app` from the DMG and verifies the pinned SHA-256, `79786e6c67c3db7dfce32a3931fa483ef3e0563f94f78ebd89d077a16927ea9e`. It does not change Accessibility or Screen Recording consent.

The downloaded v0.6.4 package's version, architectures, deployment floor, Developer
ID signature, notarization and Gatekeeper acceptance were checked without launching
the app. Homebrew lifecycle checks were not repeated for v0.6.4. The earlier
[Homebrew lifecycle test](superpowers/reports/2026-10-03-homebrew-lifecycle.md) used
v0.5.0 and passed fresh installation, upgrade from a v0.4.8 fixture and removal in
a temporary application directory on Apple Silicon, including installation through
the published tap. Default Applications-directory behavior, Intel lifecycle/runtime
and other supported macOS versions remain unverified; see the
[distribution evidence](distribution.md).

To use Homebrew for an update, quit Option Tab and run:

```sh
brew update
brew upgrade --cask --greedy GuilhermeVozniak/tap/option-tab
```

Option Tab also has its own updater. Avoid running both installations at once. If a manually installed copy already occupies the destination, resolve Homebrew's reported conflict before installing; the cask does not force replacement.

Quit Option Tab before uninstalling:

```sh
brew uninstall --cask GuilhermeVozniak/tap/option-tab
```

Normal uninstall preserves your settings and local grant/lyrics records. The cask has no `zap` or process-killing scripts.

## Existing installations from the repository tap

The original `GuilhermeVozniak/option-tab` tap remains available. If you installed
through it, continue upgrading with its existing qualified cask name:

```sh
brew upgrade --cask --greedy GuilhermeVozniak/option-tab/option-tab
```

The personal tap is the recommended installation route for new users. Both casks
point to the same signed release; this change does not require reinstalling the app.

## Maintaining the cask

After publishing a release, download the exact macOS asset and compute `shasum -a 256 <asset>`. Compare it with the GitHub release asset digest and inspect its application version, architectures and signing/notarization evidence. Update `Casks/option-tab.rb` in both this repository and `GuilhermeVozniak/homebrew-tap` through reviewed PRs with the real version, filename and checksum. Keep the old cask until the new artifact is available.

The current cask uses `darwin_universal.dmg` and has no ARM64-only restriction. Keep its filename and the website's published architecture metadata aligned with verified release assets in the same release-maintenance PR. Never use `:no_check` or point the cask at an unsigned development build. The macOS 14 support floor is intentional; the older v0.4.8 artifact declares a lower minimum and has not established the capture fallback on macOS 13.

Validate with Homebrew's cask checks, then test installation, upgrade and removal in a disposable environment on each supported architecture. Local metadata/checksum verification alone does not prove those postconditions. See the [Homebrew tap guide](https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap) and [Cask Cookbook](https://docs.brew.sh/Cask-Cookbook) for the maintained syntax and explicit tap URL form.

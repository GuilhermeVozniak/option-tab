# Install with Homebrew

The cask is maintained in this repository and is available through an explicit tap:

```sh
brew tap GuilhermeVozniak/option-tab https://github.com/GuilhermeVozniak/option-tab
brew install --cask GuilhermeVozniak/option-tab/option-tab
```

The cask targets the published [v0.5.0 universal release](https://github.com/GuilhermeVozniak/option-tab/releases/tag/v0.5.0), containing Apple Silicon and Intel binaries and requiring macOS 14 or later. It installs `Option Tab.app` from the DMG and verifies the pinned SHA-256, `6ef2d41f3e1593bafb9d79c570e8e5e7c2eef1e5b05457584ab2f9d1b1e4e231`. It does not change Accessibility or Screen Recording consent.

The downloaded package's version, architectures, deployment floor, Developer ID signature, notarization and Gatekeeper acceptance were checked. These checks did not launch or install the app. Homebrew installation, upgrade and removal on supported architectures, and Intel runtime behavior, remain unverified; see the [distribution evidence](distribution.md).

To use Homebrew for an update, quit Option Tab and run:

```sh
brew update
brew upgrade --cask --greedy GuilhermeVozniak/option-tab/option-tab
```

Option Tab also has its own updater. Avoid running both installations at once. If a manually installed copy already occupies the destination, resolve Homebrew's reported conflict before installing; the cask does not force replacement.

Quit Option Tab before uninstalling:

```sh
brew uninstall --cask GuilhermeVozniak/option-tab/option-tab
brew untap GuilhermeVozniak/option-tab
```

Normal uninstall preserves your settings and local grant/lyrics records. The cask has no `zap` or process-killing scripts.

## Maintaining the cask

After publishing a release, download the exact macOS asset and compute `shasum -a 256 <asset>`. Compare it with the GitHub release asset digest and inspect its application version, architectures and signing/notarization evidence. Update `Casks/option-tab.rb` in a reviewed PR with the real version, filename and checksum. Keep the old cask until the new artifact is available.

The v0.5.0 cask uses `darwin_universal.dmg` and has no ARM64-only restriction. Keep its filename and the website's published architecture metadata aligned with verified release assets in the same release-maintenance PR. Never use `:no_check` or point the cask at an unsigned development build. The macOS 14 support floor is intentional; the older v0.4.8 artifact declares a lower minimum and has not established the capture fallback on macOS 13.

Validate with Homebrew's cask checks, then test installation, upgrade and removal in a disposable environment on each supported architecture. Local metadata/checksum verification alone does not prove those postconditions. See the [Homebrew tap guide](https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap) and [Cask Cookbook](https://docs.brew.sh/Cask-Cookbook) for the maintained syntax and explicit tap URL form.

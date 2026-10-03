# Install with Homebrew

The cask is maintained in this repository. These commands become available after the feature branch is merged into the default branch:

```sh
brew tap GuilhermeVozniak/option-tab https://github.com/GuilhermeVozniak/option-tab
brew install --cask GuilhermeVozniak/option-tab/option-tab
```

The current cask installs the published **v0.4.8 Apple Silicon** build and requires macOS 14 or later. It does not install the unreleased feature branch. Intel support will be added after a universal release has been published and verified. The cask verifies the downloaded DMG against its pinned SHA-256 and lets Homebrew install the application; it does not change Accessibility or Screen Recording consent.

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

For the first verified universal release, change the suffix to `darwin_universal.dmg` and remove `depends_on arch: :arm64`. Update the website's published architecture metadata in the same release-maintenance PR. Never use `:no_check` or point the cask at an unsigned development build. The macOS 14 support floor is intentional; the older v0.4.8 artifact declares a lower minimum and has not established the capture fallback on macOS 13.

Validate with Homebrew's cask checks, then test installation, upgrade and removal in a disposable environment on each supported architecture. Local metadata/checksum verification alone does not prove those postconditions. See the [Homebrew tap guide](https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap) and [Cask Cookbook](https://docs.brew.sh/Cask-Cookbook) for the maintained syntax and explicit tap URL form.

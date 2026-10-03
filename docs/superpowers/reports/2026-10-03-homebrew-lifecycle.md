# Homebrew lifecycle verification — October 3, 2026

The published v0.5.0 cask passed fresh installation, upgrade from a v0.4.8 fixture,
and removal on an Apple Silicon host running macOS 27.0.1 and Homebrew
`7.0.7-92-g6873e0a`. Neither application version was launched.

## Method and results

The cask from commit `14333a2` was copied unchanged into a unique disposable local
tap. Application, download-cache, log, temporary and trust-configuration paths
were redirected to a temporary directory. Homebrew's own cask receipt remained
under its normal Caskroom and was removed during cleanup. Automatic Homebrew
updates, cleanup, autoremove, analytics and app quitting were disabled for the test.

- Fresh installation placed `Option Tab.app` version 0.5.0 in the requested app
  directory. Homebrew recorded version 0.5.0 and the explicit application path;
  the installed signature verified.
- Removal deleted the temporary application and its Homebrew receipt.
- The real published v0.4.8 ARM64 DMG was verified against SHA-256
  `5ae22160923c74767d6a381bf8aac56d7749162eca435fc4457cbc8036ed1186` and installed as
  the upgrade fixture. Its historical cask needed one fixture correction:
  `app "option-tab.app"` became `app "Option Tab.app"`, matching the actual DMG.
  This does not establish that the unmodified historical cask worked.
- Replacing only the disposable tap's cask with the current definition and running
  `brew upgrade --cask --greedy --no-quit` upgraded 0.4.8 to 0.5.0, retained the
  requested application path, and purged the old version. The installed app had
  both arm64 and x86_64 slices and a valid signature.
- A further fresh installation by fully qualified cask name succeeded after
  removing the test cask's trust entry. Homebrew granted trust for that item
  automatically; the documented install command does not need a separate
  `brew trust` command.
- A separate run used the documented public tap URL and fully qualified cask name.
  Homebrew cloned `guilhermevozniak/option-tab`, whose cask matched the verified
  source, and installed version 0.5.0 with a valid signature into another temporary
  appdir. Uninstall, untrust and untap succeeded, and baseline checks passed again.

All downloads used the published release URLs and pinned checksums. The v0.5.0
universal DMG digest was
`6ef2d41f3e1593bafb9d79c570e8e5e7c2eef1e5b05457584ab2f9d1b1e4e231`.

## Cleanup and scope

Final uninstall, cask untrust and untap removed the temporary application, receipt,
tap and trust entry; the scratch directory was then removed. The original tap
list and installed cask versions matched their baseline. Hashes of the existing
application binaries and Option Tab settings were unchanged, and the existing
test-app process remained running. Its Preferences still reported Accessibility
and Screen Recording as granted.

This verifies the cask's filesystem and receipt lifecycle in an explicit appdir
on this host, including installation through the published tap. It does not verify
installation into the default Applications directory, Intel execution or Intel Homebrew
lifecycle, other supported macOS versions, app launch, permission recovery for
v0.5.0, or the in-app updater. G01–G03 and broader native acceptance remain open.

References: [v0.5.0 release](https://github.com/GuilhermeVozniak/option-tab/releases/tag/v0.5.0),
[Homebrew tap trust](https://docs.brew.sh/Tap-Trust), and
[distribution evidence](../../distribution.md).

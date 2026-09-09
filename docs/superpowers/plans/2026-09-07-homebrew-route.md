# Homebrew installation route

Retained G02 uses a `Casks` directory in this repository and an explicit tap URL. No separate maintained tap was found among the owner's GitHub repositories. This avoids creating another repository while keeping the cask versioned with release maintenance.

1. Pin the existing published v0.4.8 ARM64 DMG and its verified SHA-256; declare Apple Silicon hardware and the supported macOS 14 floor. Do not advertise the unshipped universal artifact.
2. Provide install/update/uninstall instructions using the explicit repository tap, noting that the route becomes available after this branch is merged. Normal uninstall preserves user settings. Do not add broad deletion or app-killing scripts.
3. Add maintainer steps for matching a later published universal asset, updating its checksum, removing the ARM64 restriction only after architecture verification, and updating published website metadata together.
4. Verify syntax, Homebrew metadata resolution and the real asset checksum without installing/replacing the user's app. Actual isolated install/upgrade/uninstall and signed universal release acceptance remain separately recorded.

Primary references: [tap repositories](https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap), [Cask language](https://docs.brew.sh/Cask-Cookbook). No release or new remote repository is created by this plan.

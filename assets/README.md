# Option Tab icon

`option-tab.svg` is the shared app and website mark. It reuses the Settings sidebar’s white Option symbol on a `#202020` tile, with the same 9:33 corner-radius proportion. The symbol is outlined so rendering does not depend on installed fonts. Settings keeps its existing theme-aware mark.

After editing the vector, run from the repository root:

```sh
bun install --frozen-lockfile
bun scripts/generate-icons.mjs
```

The generator uses the Sharp renderer included in the locked Next.js dependencies and updates:

- `apps/desktop/build/appicon.png`: 1024 × 1024, with transparent macOS icon padding.
- `apps/web/app/icon.png`: 512 × 512 favicon using the same mark.
- `apps/web/public/option-tab.svg`: unpadded vector for website navigation and branding.

`scripts/bundle.sh` converts the desktop PNG into all required `.icns` resolutions and installs `iconfile.icns` in the app bundle. The same asset is used for Intel, Apple silicon, and universal builds.

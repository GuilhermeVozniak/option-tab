# Window grid defaults, feature guide, and brand release

User direction: make the original flat window thumbnail grid the default, release it, document every app feature and how it works on the landing site, match the app theme, and reuse the existing in-app icon for main branding.

## Implementation

- Change both built-in shortcut defaults to Window switcher / All windows. Retain thumbnails with the separate selected preview disabled. Preserve explicitly saved preferences and optional App switcher behavior; no schema migration.
- Keep Go and frontend defaults aligned, update existing regression expectations, and give mode-specific tests explicit fixtures.
- Publish one structured feature inventory with exact settings paths, enablement steps, and capability limits, plus a repository guide.
- Reuse Settings neutral System/Light/Dark themes on the landing page and public /docs guide, with responsive navigation and accessible links/controls.
- Reuse the in-app Option mark as canonical brand artwork; export matching web and macOS icon assets without redesigning the desktop UI.

## Verification and publication

- [x] Default regression tests fail before the fix and pass afterward; saved explicit preferences remain preserved.
- [x] Full Go/frontend suites, lint, builds, package checks, and relevant browser tests pass.
- [x] Verify desktop default settings; visually inspect website and docs in light/dark/mobile, and icon exports.
- [x] Independently review default behavior, full feature accuracy, and website accessibility.
- [x] Merge reviewed release PR; tag v0.6.2; verify signed/notarized universal artifacts and compatibility alias before publishing.
- [x] Prepare website version, both Homebrew casks, and distribution docs with the verified checksum for the metadata PRs.

After metadata merges, verify main CI and the deployed website; retain the results with the release evidence.

The current installed primary shortcut already uses the desired window grid. This implementation changes new-install/reset defaults without overwriting the user's other saved preferences.

Verification evidence: all Go packages passed with race detection and coverage; 662 desktop frontend tests and all other workspace unit suites passed; 145 desktop browser tests, 10 website browser tests, and eight packaging contract tests passed. Production builds, Biome, and golangci-lint passed. Independent review verified saved preference preservation and corrected the guide to explain shared window filters. No new native runtime or physical gesture acceptance is claimed.

Release v0.6.2 was published after workflow 37164795192 and downloaded-artifact verification, including icon pixel comparison. PR #36 fixes GitHub Pages directory routing after live checks identified `/docs/` returning 404. Its export regression and all 11 website browser tests passed. Distribution metadata follows the published artifact; final deployment checks are recorded with the release evidence.

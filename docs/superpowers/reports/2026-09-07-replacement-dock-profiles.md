# Replacement Dock profiles and appearance checkpoint

Native CGO builds passed for arm64 and x86_64; Mach-O metadata confirms a macOS 14.0 minimum for both. These local unsigned binaries were not installed or distributed.

Replacement Dock profiles now support every screen edge, floating or full-width placement, alignment, automatic hiding and independent appearance. Settings provide New/Duplicate/Rename/Delete with explicit deletion reassignment, plus removable main-display and stable UUID assignments. Disconnected assignments are preserved. Native recovery remains available.

Version 1 settings migrate explicitly to version 2 without resetting profiles, bindings, enabled state or clock grants. Invalid legacy values, unknown/new fields disguised as legacy, and future versions are rejected. Migration is assigned after top-level JSON decoding so that decoding cannot overwrite it.

Geometry uses safe logical display bounds, protected native Dock access regions and fixed host dimensions. Native observation intersects visibleFrame with public safeAreaInsets. Four edges, alignment, full-width layouts, spacing and inward overlap displacement are pure functions; impossible geometry yields. No app window or native Dock preference is moved or changed.

Typed appearance includes theme, solid/system material, tint, opacity, border, corner radius, spacing and labels. Dedicated launcher hosts apply native styling before Show: system material uses an NSVisualEffectView behind Wails content, and close restores the original content and frames. Profile edits retire the old host/session. Native style failures close the failed host. Preview/media host behavior is unchanged.

The renderer follows system light/dark appearance, keeps text legible against tint, and scrolls only on the primary axis. Browser checks exercise all four edges at the minimum 72-point cross-axis with full-size 64-point icons, labels, clock and twelve items. Review also corrected stale deletion reassignment and overlong duplicated names.

Verification: full Go race/coverage across 22 packages, golangci-lint and repository Biome passed. The full JavaScript run passed 295 desktop, 7 shared and 4 site tests; final focused UI regressions passed 11 tests. The full desktop browser run passed 71 tests before the final geometry expansion; the final six-test launcher suite passed afterward. Generated bindings contain 84 methods/42 models, with unchanged RPC IDs, and the regenerated-binding production frontend build passed. App integration verifies style-before-Show, style failure cleanup and fresh-session profile changes. Native view seams verify launcher-only styling, original-content restoration and safe-area conversion.

Actual macOS blur/appearance, notch hardware, native placement, clicks, multiple displays/Spaces and recovery remain separate physical acceptance work. This remains an unreleased draft implementation checkpoint; the complete retained roadmap continues with focus rules, pinned items/groups, magnification, folder fan-out, app menus, audio/widgets, community installation, gestures, supported badges and import/export. B10 native material for the existing switcher/preview surfaces is also still outstanding; this slice styles the replacement launcher only.

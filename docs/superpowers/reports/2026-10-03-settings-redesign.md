# Settings UI and navigation redesign

The Settings window now uses a monochrome sidebar layout with independent System,
Light, and Dark themes. The theme is a local window preference and does not change
the window switcher, app switcher, launcher, or Dock preview appearance settings.

The user approved this direction and requested clearer placement, grouping, and
explanations as part of the redesign. Navigation now names tasks directly:
General, Shortcuts, Appearance, Window rules, Excluded apps, Dock, and About.
Existing native deep links retain their original keys.

## Organization and behavior

- General separates app behavior, window capture, permissions, updates, crash
  reporting, and backup/reset.
- Shortcuts have labelled fields and expandable behavior/override settings.
  Keyboard navigation, mouse behavior, and window actions have distinct groups.
- Appearance separates style/theme, sizing, surface effects, window information,
  and motion. Dock reuse explains Dock previews rather than the switcher.
- Window rules explains which settings are shared and which are mode-specific.
- Excluded apps has a clear empty state and structured draft/entry rows.
- Dock has Launcher, Window previews, Media, and Monitor sections. Launcher
  settings separate profiles, items, widgets, layout, interactions, display
  assignments, and focus rules.
- Item, widget, package, profile import, and diagnostic editors share aligned
  fields, action rows, and readable review/report surfaces.
- Navigation keeps editors mounted to preserve drafts. Existing action handlers,
  permission requests, disabled states, and capability gates are retained.
- New copy is translated into Portuguese and Spanish. Sidebar keyboard navigation
  and its accessibility orientation follow the current layout.

## Verification

- Production frontend build and full frontend Biome check passed.
- All **655 tests in 47 frontend files** passed. Existing unrelated React `act`
  warnings remain in the older App tests; there were no test failures.
- All **42 Settings browser cases** passed against the production bundle in
  Chromium and WebKit, using `playwright.settings.config.ts`. Coverage includes
  theme persistence/isolation, system appearance changes, keyboard navigation,
  draft editing, updates, and layout at 390, 720, 900, and 1280 px widths.
- Six additional Dock/profile/monitor settings browser flows passed.
- Five reproducible window/app switcher screenshots were byte-identical before
  and after: thumbnail, app-icon, and title modes, plus dark horizontal and light
  vertical app switchers with preview content.
- Main Settings screenshots cover all pages and Dock sections in both themes.
  Populated child editors, review dialogs, and diagnostics were inspected at
  900×640 and 720×640 without horizontal overflow.
- Independent code review found no remaining blockers. `git diff --check` passed.

Only Settings and its tests/supporting translations changed. The running native
app was not restarted or replaced during verification. Browser checks use the
real frontend with fallback or fake native services; they do not claim a new
round of native permission, hardware, or window-management acceptance.

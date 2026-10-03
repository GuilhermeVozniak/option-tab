# Changelog

## 0.6.0 — 2026-10-03

- Redesign Settings with a responsive sidebar, compact aligned controls, and independent System, Light, and Dark themes.
- Organize preferences around General, Shortcuts, Appearance, Window rules, Excluded apps, Dock, and About, with clearer labels and feature explanations.
- Split Dock settings into Launcher, Window previews, Media, and Monitor, with focused Launcher sections for profiles, items, widgets, layout, interactions, displays, and focus rules.
- Improve profile import/export, widget configuration, diagnostics, keyboard navigation, and Portuguese/Spanish Settings translations.
- Preserve existing settings, deep links, and the switcher and preview appearance.

The native feature limitations and hardware checks documented for 0.5.0 remain unchanged. Windows and Linux downloads remain demonstration builds without native window switching.

## 0.5.0 — 2026-10-03

- Add app-grouped switching, live window previews, per-mode preferences, and guarded individual and bulk window actions.
- Add native Dock previews and controls, preview dragging, manual monitor protection, Folder Pop, and Music/Spotify panels with optional lyrics and pinned media views.
- Add an optional replacement Dock with display/app profiles, layouts, materials, magnification, pins, groups, reordering, folder/window panels, supported notification badges, and profile import/export.
- Add Clock, Battery, Network, Audio, and media widgets with explicit capabilities and reviewed widget packages.
- Add AppleScript window automation, user-reviewed diagnostics, and Portuguese/Spanish feedback for the new controls.
- Fix startup activation, stale interaction targets, preview minimize/restore behavior, child-panel failures, widget overflow, and saved-language refresh.
- Publish a signed and notarized universal macOS build for Apple Silicon and Intel, requiring macOS 14 or later.

Replacement-Dock keyboard mode, automatic native Dock placement, and Spotify seeking remain unavailable pending native validation. Music playback/lyrics, broader display/Space recovery, real audio-output changes, battery readings, and Intel runtime still need additional device testing. Physical trackpad checks were waived, not passed. Automated tests do not establish complete native acceptance.

Windows and Linux downloads remain demonstration builds without native window switching.

## 0.4.8 — 2026-09-06

- Pass updater relaunch paths as literal arguments, including paths with spaces and shell characters.
- Focus clicked windows atomically so selection changes cannot focus the wrong window.
- Serialize settings saves, report persistence and login errors, validate imports before display, and recover safely from malformed settings.
- Correct native window-ID capture, shortcut modifier matching, application bypass rules, and hold-to-cycle behavior.

Windows and Linux downloads remain demonstration builds without native window switching.

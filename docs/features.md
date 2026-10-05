# Option Tab feature guide

<!-- Keep this guide in sync with apps/web/lib/feature-guide.ts, the landing-page documentation content. -->

Option Tab is a free, open-source macOS window switcher with optional Dock previews, a configurable launcher, widgets, and local automation. The supported desktop release requires macOS 14 or later and runs on Apple Silicon and Intel.

On a fresh setup, Command+Tab and Option+Tab open the window switcher: individual windows in a thumbnail grid, with no separate enlarged preview. Existing saved shortcut and appearance choices are preserved when you update.

Open Settings from Option Tab’s menu-bar icon. Settings paths below use the English labels. Where an Editing selector appears, choose Window switcher or App switcher before changing that mode’s appearance, behavior, or ordering; changing Editing does not change what a shortcut opens.

## Contents

- [Getting started](#getting-started)
- [Switching and window actions](#switching)
- [Appearance and previews](#appearance)
- [Window rules and exclusions](#filtering)
- [Native Dock enhancements](#dock-previews)
- [Music and media panels](#media)
- [Replacement Dock and profiles](#replacement-dock)
- [Widgets and local packages](#widgets)
- [Local AppleScript automation](#automation)
- [Preferences, updates, and support](#maintenance)

<a id="getting-started"></a>

## Getting started

Install the app, grant the two core permissions, and restore the window grid on an existing setup.

### macOS installation

Install the signed universal app with Homebrew or download its DMG. Option Tab lives in the menu bar, so it does not need a permanent app icon in the Dock.

1. Run: brew install --cask GuilhermeVozniak/tap/option-tab
2. Alternatively, download the macOS DMG, move Option Tab to Applications, and open it.
3. Use the menu-bar icon to open Settings, show the switcher, pause or resume switching, or quit.

**Note:** The feature guide describes the macOS product. Other-platform build targets do not provide the same native window and Dock integrations.

### Permission onboarding

The first-run setup shows live permission status. Accessibility enables global shortcuts and window actions; Screen Recording supplies live thumbnails.

1. Follow the onboarding prompts, or open Settings → General → Permissions.
2. Grant Accessibility and Screen Recording in the corresponding macOS System Settings panes, then return to the app.
3. If macOS asks you to quit and reopen the app after granting access, do so.

**Note:** Without Screen Recording, previews fall back to app icons. Music control, selected folders, and widgets have separate, explicit access choices.

### The default window grid

Both default shortcuts open individual windows from all apps as thumbnails. The separate enlarged selected-window preview is off for the window switcher.

1. For an existing setup, open Settings → Shortcuts. For the shortcut you use, set Opens to Window switcher, Include to All windows, and Visual style to Thumbnails or Default style.
2. Open Settings → Appearance, set Editing to Window switcher, choose Thumbnails, and turn off Preview the selected window.
3. Use Settings → General → Backup and reset → Reset to defaults only if you want to reset all feature preferences, not just the preview layout.

**Note:** Updates keep saved preferences. All windows here means all applications; Window rules can still limit Spaces, screens, and window states.

<a id="switching"></a>

## Switching and window actions

Move between individual windows or applications, search, and act on the selected window.

### Window-level switching

Each eligible window has its own entry, including separate windows belonging to the same application. This makes the exact document or browser window directly selectable.

1. Hold Command or Option and press Tab to open the default window switcher. Keep holding the modifier and press Tab again to advance; Shift+Tab goes backward.
2. Release the modifier to focus the selection. Enter or clicking a window also confirms; Escape or clicking outside cancels.

### Optional app switcher

The app switcher presents an application strip with the selected app’s windows underneath, including running apps without an open window. It remains available as a separate mode.

1. Open Settings → Shortcuts and set a shortcut’s Opens field to App switcher.
2. Use the application strip to choose an app, then select a window or use the available Open app, New window, Hide app, and Quit app actions.
3. Choose Editing: App switcher in Appearance or Shortcuts to adjust that mode independently.

**Note:** The app strip and window grid are different modes. The Editing selector chooses which preferences you edit; the shortcut’s Opens field chooses the mode you actually launch.

### Up to nine independent shortcuts

Each shortcut can open a different mode, include all apps or only the active app, and override style, Space, screen, ordering, and release behavior.

1. In Settings → Shortcuts, click an existing shortcut field and press the desired key combination, or choose + Add shortcut.
2. Choose Opens, Include, and Visual style. Expand Behavior and overrides for On release, Spaces, Screens, and Order.
3. Use the switch beside a shortcut to disable it without deleting it.

### Search and keyboard navigation

Type to filter windows by title or app name with fuzzy matching. Tab navigation is available alongside optional arrow keys and Vim keys.

1. Open the switcher and type part of a title or app name to search.
2. In Settings → Shortcuts → Keyboard navigation, enable Navigate with arrow keys or Navigate with vim keys (h / j / k / l).
3. Adjust Hold modifier to cycle and each shortcut’s On release behavior if you prefer an explicit confirmation.

**Note:** When Vim navigation is enabled, unmodified h, j, k, and l are navigation keys rather than search text.

### Window and application controls

Focus, close, minimize or restore, enter or leave fullscreen, hide or show an app, and quit it from the switcher. Additional actions include New window, Force quit, Close all windows, and Minimize all windows.

1. Use the controls on each window preview or the toolbar icons. Hover a toolbar or window-control icon for 2.5 seconds to see what it does. Window controls stay visible while you cycle with Command+Tab or Option+Tab; enable them in Settings → Appearance → Window information if they are hidden.
2. With the activation modifier held, the default action keys are W for close, M for minimize/restore, F for fullscreen, H for hide/show, and Q for quit.
3. Open Settings → Shortcuts → Window actions to change physical-key bindings, add a binding, or disable action keys.

**Note:** Availability depends on the target app and its macOS Accessibility controls. Unsupported or refused actions are reported. Close keeps the app’s normal save prompts; Force quit can discard unsaved work.

### Mouse, gestures, and haptics

Select by hovering, move the cursor to a focused window, assign middle-click and swipe actions, and request a haptic tick when selection changes.

1. Open Settings → Shortcuts and select the mode to edit.
2. Use Mouse behavior for hover selection and Cursor follows focus; use Keyboard navigation for Trackpad haptic feedback.
3. Under Window actions, assign Middle click, Swipe up, and Swipe down where available.

**Note:** Gesture and haptic support depends on the native input source and your hardware. Precise trackpad input is used; macOS does not reliably expose a finger count. A mouse-only setup does not provide trackpad gestures.

<a id="appearance"></a>

## Appearance and previews

Keep the thumbnail grid or tailor each switcher’s layout, theme, information, and timing.

### Thumbnails, icons, and titles

Choose thumbnail cards, app icons, or a compact title presentation. Horizontal and vertical layouts and a window-count threshold for title mode support different working sets.

1. Open Settings → Appearance and choose the switcher in Editing.
2. Choose the visual style and Layout direction. Set Use titles at window count to automatically switch to titles above that threshold; zero disables it.
3. A shortcut’s Visual style override takes precedence over the mode’s default style.

### Preset and fine-grained sizing

Small, Medium, and Large presets set a starting size. Auto-sizing fits thumbnails to the window count, while explicit controls let you tune density.

1. Use Settings → Appearance → Style and theme for the size preset.
2. Under Layout and sizing, adjust maximum rows and columns, thumbnail size, icon size, title width, font size, and Auto-size thumbnails.

### Independent themes and surface styling

Switcher themes can follow the system or use light or dark mode, with an accent color, background opacity, blur, and corner radius. Settings has its own neutral System / Light / Dark preference.

1. Open Settings → Appearance → Style and theme for the selected switcher’s theme and accent.
2. Use Surface and effects for opacity, corner radius, and blur.
3. Use the theme choices at the bottom of the Settings sidebar to change Settings alone.

**Note:** The Settings theme is a local presentation preference and is separate from exported switcher and Dock settings.

### Optional enlarged preview and display placement

An optional larger preview follows the selected window. The switcher can appear on the cursor’s display, the active screen, or the focused window’s screen.

1. In Settings → Appearance → Style and theme, turn Preview the selected window on or off for the selected switcher.
2. Choose the display using Show on. Leave the enlarged preview off for the default compact window grid.

### Titles, app badges, and window state

Show window titles, app icon badges, window controls, minimized/hidden/fullscreen status, and Space labels. Long titles can truncate at the start, middle, or end.

1. Open Settings → Appearance → Window information and enable the details you want on each card.

**Note:** Space information depends on what macOS exposes. Labels describe the detected Spaces, not user-defined desktop names.

### Capture freshness and motion

Visible previews capture window content as needed. Optional background capture keeps thumbnails fresh between activations. Appearance controls adjust the show delay, exit fade, and selected-preview fade.

1. Enable Settings → General → Window capture → Capture windows in the background if you want previews prepared before the switcher opens.
2. Use Settings → Appearance → Motion and timing to change Apparition delay, Fade out animation, and Fade in the selected-window preview.

**Note:** Background capture is off by default. It requires Screen Recording and can keep macOS’s recording indicator visible while the switcher is hidden.

<a id="filtering"></a>

## Window rules and exclusions

Window filters are shared by both switchers; each mode has its own ordering. Individual shortcuts can override Space, screen, and ordering rules.

### Window ordering

Order entries by recently focused, recently created, alphabetically, or by Space.

1. Open Settings → Window rules, choose the switcher in Editing, and set Display order.
2. For a single shortcut, use Settings → Shortcuts → Behavior and overrides → Order.

### Spaces, screens, and visibility filters

Include every Space or only the active one; every screen, the active screen, or the screen under the cursor. Minimized, hidden, and fullscreen windows can be shown, hidden, or placed at the end.

1. Open Settings → Window rules → Which windows to show and choose the scope and visibility rules.
2. Use Show windows without a title to include or omit untitled windows.
3. Set a shortcut’s Include to Active app only when you want to cycle just that app’s windows.

**Note:** The Which windows to show controls affect both switchers, regardless of the Editing selection. A shortcut’s Space and screen overrides take precedence when that shortcut is used.

### App exclusions and shortcut exceptions

Match an app by its name or bundle identifier. Hide it always or when it has no open window, and optionally ignore Option Tab shortcuts while it is frontmost.

1. Open Settings → Excluded apps and choose + Add app.
2. Enter the app name or bundle ID, select the hide rule, and enable Ignore shortcuts when needed.

**Note:** These exclusions apply to both switchers. An exact bundle identifier is useful when app names are similar.

<a id="dock-previews"></a>

## Native Dock enhancements

Add app-window previews and folder browsing to the macOS Dock without replacing it.

### Window previews on hover

Rest the pointer on an app’s native Dock icon to see its windows and available actions. Dock previews have their own appearance and timing settings.

1. Open Settings → Dock → Window previews and turn on Enable Dock previews.
2. Grant Accessibility and Screen Recording, then hover over a running app’s Dock icon.
3. Choose Applications, visual style, sizing, information, and effects in the same section.

**Note:** This optional feature starts off. Preview availability depends on the app’s windows and the Dock identity exposed by macOS.

### Dock clicks, scrolling, and swipes

Optional controls let you click a Dock icon to hide an app, scroll to show or hide it, Command-right-click to quit, and assign middle-click or directional swipe actions.

1. Open Settings → Dock → Window previews → Input and gestures.
2. Enable the click or scroll options you want and choose actions for middle-click and the four swipe directions.
3. Under Timing and movement, tune hover delay, dismiss delay, movement tolerance, corridor padding, and card spacing.

**Note:** Precise trackpad scrolling powers swipes. Actions can be unavailable when macOS or the target app does not expose the required control.

### Move windows from previews and Aero Shake

Drag a preview to move its window. An optional shake action during preview dragging can minimize or close other windows.

1. Enable Settings → Dock → Window previews → Input and gestures → Drag previews to move windows.
2. Choose an Aero Shake action, or leave it set to None.

**Note:** Window movement depends on Accessibility support and the window’s current state. Choose Close other windows only when you intend to close them; target apps retain their normal save behavior.

### Folder Pop

Browse a native Dock folder’s contents without first opening a Finder window. Sort by name, modification date, size, or kind, reverse the order, and put folders first.

1. Enable Settings → Dock → Window previews → Enable Folder Pop, then hover over a folder in the native Dock.
2. If requested, use the folder access chooser to grant access to that folder.
3. Use the panel’s sorting controls and select an entry to open it with its associated app.

**Note:** Access is requested explicitly. Moving or replacing a selected folder can require granting access again.

### Keep the native Dock on a monitor

Monitor protection watches the native Dock’s verified location and helps keep it on the selected display, without changing persistent Dock preferences.

1. Open Settings → Dock → Monitor, enable Lock Dock to a monitor, and choose the main display or a connected monitor.
2. Move the Dock to that display manually and wait for the status to show Protected.
3. Hold the selected Bypass modifier to temporarily bypass protection.

**Note:** Automatic Move Dock here placement is currently disabled. Protection only starts after location verification; disconnected displays, inaccessible edges, or missing Accessibility permission can prevent it.

<a id="media"></a>

## Music and media panels

Optional Apple Music and Spotify controls, artwork, and synchronized local lyrics.

### Apple Music and Spotify playback

Show track metadata and supported play/pause, previous, and next controls for an enabled running player. A player’s Dock hover can switch between its Windows and Media views.

1. Open Settings → Dock → Media. Enable media controls and the player you use, then click Connect and grant the requested macOS Automation permission.
2. Open the player and hover its Dock icon. Choose Media if a Windows / Media selector appears.
3. Use the controls shown as available for the current player and track.

**Note:** Players are opt-in and are not launched just to poll metadata. Apple Music seeking is available when the player supplies a usable duration. Spotify seeking and duration remain disabled pending native verification.

### Pinned media panels

Keep a media panel available after leaving its Dock icon, with its own movable presentation.

1. Use the Pin control in an available media panel.
2. Move it by its header and use its close control when finished.

**Note:** Media pins last for the current app session; they are not permanent desktop widgets.

### Synchronized lyrics

Follow timestamped lyrics supplied by Apple Music when available, or associate a local LRC file with a track. Local files support a timing offset and take priority over provider text.

1. Open the Media panel’s Synchronized lyrics area and import a local .lrc file for the current track.
2. Adjust the timing offset if needed; use the panel controls to return to the current line, replace the file, or remove its association.

**Note:** There is no lyrics search or download service. Plain text without timestamps cannot be synchronized. Removing an association does not delete the original file, and Music’s catalog lyrics are not guaranteed to be exposed to other apps.

### Optional artwork

Apple Music artwork can come from the local player. Spotify’s remote artwork is a separate opt-in; playback metadata and controls remain available without it.

1. Open Settings → Dock → Media and enable Allow remote artwork only if you want the app to fetch player-supplied cover images.

**Note:** Remote artwork contacts the image host over HTTPS. It is off by default.

<a id="replacement-dock"></a>

## Replacement Dock and profiles

An optional launcher with persistent pins, groups, per-display layouts, and app-based profile rules.

### Optional floating launcher

The replacement Dock provides a configurable launcher alongside the native macOS Dock, which remains available.

1. Open Settings → Dock → Launcher and turn on Enable replacement Dock.
2. Choose the profile to edit and use the Profile, Items, Widgets, Layout, Interactions, Displays, and Focus rules sections.
3. Use Use native Dock to turn the replacement launcher off and return to the system Dock.

### Multiple launcher profiles

Keep up to eight profiles with separate items, widgets, appearance, and interaction choices. Create, rename, duplicate, and delete profiles without rebuilding every layout.

1. Open Settings → Dock → Launcher → Profile. Use New profile or Duplicate profile, then give it a name.
2. Before deleting a profile, choose its replacement so display assignments and focus rules can be reassigned.

### Pins, links, groups, and custom icons

Pin applications, folders, files, and web links; organize applications into groups; insert spacers and separators; and choose custom PNG icons. Running apps are matched to existing pins to avoid duplicates.

1. Open Settings → Dock → Launcher → Items and use the item editor to select local apps, files, or folders, add a link, and arrange groups and decorative items.
2. Choose a PNG icon where offered, then save the item changes.
3. If an item moves or loses access, use its relink or select-again control in Settings.

### Reorder items directly on the Dock

Optional item handles let you rearrange persistent pins and create application groups from the running launcher.

1. Enable Settings → Dock → Launcher → Items → Reorder items on the Dock.
2. Use the visible handles to move pins or use the available grouping controls. The Settings item editor also supports ordering and grouping.

### Folder and application panels

Pinned folders open list or grid panels; application controls provide available launch, window, and relaunch actions. Show-all window panels let you choose an exact app window.

1. Add a folder or app under Settings → Dock → Launcher → Items, then use that item in the launcher.
2. Click a folder item to browse its contents using the panel’s list/grid controls. Right-click the folder item and choose Open folder to open the root folder itself.
3. Right-click an app item and choose Show all windows or Relaunch when available.

**Note:** Relaunch closes and reopens the selected app, so its normal quit/save behavior still applies. The launcher is not an arbitrary shell-command runner.

### Layout, materials, and magnification

Place a floating or full-width launcher on any of the four screen edges. Tune alignment, icon size, maximum length, thickness, inset, spacing, labels, auto-hide, theme, material, tint, opacity, border, and radius.

1. Open Settings → Dock → Launcher → Layout and adjust Position and size and Appearance for the selected profile.
2. Enable magnification and choose its scale and reach to enlarge nearby items as the pointer moves over them.

**Note:** Animation respects reduced-motion settings. Layout includes Dock-overlap avoidance; it does not tile application windows.

### Display assignments and focus rules

Assign base profiles to displays and override them when a chosen application becomes frontmost. The first matching enabled app rule wins.

1. Open Settings → Dock → Launcher → Displays to assign profiles to displays.
2. Open Focus rules, add a rule, select a running app or enter its exact bundle identifier, and choose the destination profile and display scope.
3. Reorder rules to set priority; the display assignment remains the fallback when no rule matches.

### Available native Dock badges

Mirror supported count badges or generic indicators from the native Dock onto matching launcher app items.

1. Enable Settings → Dock → Launcher → Items → Show Dock badges for each profile where you want them.

**Note:** Badges require Accessibility and a supported source app/macOS version. Missing values stay hidden. This reads Dock status, not Notification Center or notification message contents.

### Launcher gestures and haptics

Optional precise scrolling, pinch, and swipe gestures can navigate items or show and hide a selected preview. Haptics provide feedback where the device supports them.

1. Open Settings → Dock → Launcher → Interactions and enable launcher interactions.
2. Enable the available gesture sources, then choose Primary, Toward, and Pinch actions and optional haptic feedback.

**Note:** Hardware and native capability determine which switches are available. Launcher letter/keyboard navigation is currently unavailable; do not rely on that disabled setting. Physical gesture behavior varies by device.

### Portable profile import and export

Share a profile’s structural layout, items, and widget settings. Import reviews the document and creates a new, unassigned profile.

1. Open Settings → Dock → Launcher → Profile and use its export or import controls.
2. Review the incoming profile, import it, restore requested local item selections and widget access, then assign it under Displays when ready.

**Note:** Private file/folder references and access grants are not a portable permission backup. Imported profiles need local repair or consent before those features become available.

<a id="widgets"></a>

## Widgets and local packages

Add useful local information and typed actions to launcher profiles, with access controlled per widget.

### Clock, battery, network, and audio

Built-in widgets show local time, battery charge and power state, network status with optional usage rates, and audio output status. The clock supports time format and timezone choices.

1. Open Settings → Dock → Launcher → Widgets, choose a Widget package, and click Add widget.
2. Review Required and Optional access, grant the capabilities you want, enable the widget, and adjust its settings.
3. Reorder or remove instances within the profile using the widget controls.

**Note:** A widget without required access remains unavailable. Optional data can remain unavailable while its other permitted content still works.

### Choose an audio output

The audio widget can offer an explicit output-device selector in addition to displaying current output, volume, and mute state.

1. Add the Audio widget in Settings → Dock → Launcher → Widgets.
2. Grant its optional Choose audio output capability, then use the widget’s output action to select an available device.

**Note:** This changes the selected system output device; it is not a per-application audio mixer.

### Widget stacks

Group two to four widgets into one launcher slot and choose which one is shown. Each profile supports up to four visible widget slots; a stack occupies one slot.

1. In Settings → Dock → Launcher → Widgets → Stacks, name a stack and select its members.
2. Choose the Shown widget, reorder members where offered, or dissolve the stack to return them to separate slots.
3. On the running Dock, click a stack’s member buttons to change the shown widget directly.

### Review and install local widget packages

Install declarative widget ZIPs containing layouts, declared assets, fixed data bindings, and supported actions. Custom packages can also display granted Apple Music or Spotify metadata and offer supported playback controls. Packages cannot include arbitrary executable scripts.

1. Open Settings → Dock → Launcher → Widgets → Widget packages and choose Review local package….
2. Review its version, declared data access, actions, and unverified-publisher notice, then choose Install reviewed package.
3. Add an instance from the widget catalog and grant its capabilities separately. Remove installed packages from the same section when no longer needed.

**Note:** Installing a package grants no data access or actions. Community publisher identity is not independently verified. This is a local package workflow, not an online widget marketplace.

**Note:** Media widgets require a custom package and the appropriate data/control grants; the four built-in widgets remain Clock, Battery, Network, and Audio. Player permission and capability limits still apply, including unavailable Spotify seeking.

<a id="automation"></a>

## Local AppleScript automation

Use a bundled scripting dictionary to query windows, show a switcher, or request supported actions.

### Query applications and windows

Local AppleScript callers can query running applications, windows, and the active window. Replies are bounded JSON with explicit status and truncation information.

1. Run from Terminal: osascript -e 'tell application id "com.optiontab.app" to query applications'
2. Use query windows or query active window in the same tell block for window metadata. Use the returned IDs when targeting a particular window.

**Note:** The calling application can need macOS Automation permission. Explicit image queries return only eligible, already-cached previews; they do not initiate capture or prompt for Screen Recording.

### Open switchers and app previews

Automation can open the window or app switcher and show an independently owned preview panel for an exactly identified running app.

1. Run: osascript -e 'tell application id "com.optiontab.app" to open switcher mode windows mode'
2. Use apps mode for the app switcher. The show app previews command accepts an exact app selector and optional screen coordinates; retain its returned presentation token to hide that same preview.

**Note:** Paused or inactive sessions refuse new presentations. Automation admission is not a guarantee that every app window can be presented; stale or unsupported identities return errors.

### Typed window actions for scripts and macros

AppleScript-capable macro tools can request focus, close, minimize, hide application, and an explicit fullscreen state for a selected window.

1. Query a live window ID, then use perform window action with the desired operation and that exact ID, or use the active-window selector.
2. Use the repository’s Local AppleScript automation guide for complete syntax, permissions, limits, and error handling.

**Note:** Automation is local and same-user; it is not a network server or arbitrary script executor. This suite does not offer bulk close, quit, force quit, or tiling. Native app support and the caller’s permissions still apply.

<a id="maintenance"></a>

## Preferences, updates, and support

Manage startup, languages, backups, updates, and reviewed diagnostic reports.

### Startup, menu-bar style, and languages

Launch at login, choose the menu-bar icon style or hide it, and use English, Brazilian Portuguese, Spanish, or the system language.

1. Open Settings → General → App behavior and adjust Start at login, Menubar icon, and Language.
2. Use the menu-bar Pause/Resume command when you temporarily want Option Tab to stop intercepting activation shortcuts.

**Note:** Missing translations fall back to English.

### Settings backup, import, and reset

Export preferences as JSON, import a saved configuration, or restore feature defaults.

1. Open Settings → General → Backup and reset and choose Export… or Import….
2. Use Reset to defaults to restore the standard feature setup, including the default window grid.

**Note:** Review exported preferences before sharing. A settings file is not a backup of macOS permissions, local folder grants, lyric files, or all private launcher resources. Use profile transfer for a portable launcher layout.

### Update checking and installation

The default policy periodically checks GitHub releases and announces updates. You can choose automatic installation or turn scheduled checks off; manual checking remains available.

1. Open Settings → General → Updates to choose Check for updates periodically, Auto-install updates, or Don’t check for updates.
2. Use Check for updates now… for a manual check. Auto-install can download, install, and restart the app after a scheduled check.
3. For Homebrew-managed updates, quit Option Tab and run brew update followed by brew upgrade --cask --greedy GuilhermeVozniak/tap/option-tab.

**Note:** A manual update check alone does not auto-install. Scheduled checks contact GitHub and do not attach window data or settings.

### Reviewed local diagnostics

Record a bounded support session, review its JSON report, and save the reviewed snapshot locally. Reports include version, coarse status, and diagnostic events rather than window or media content.

1. Open Settings → About → Diagnostics and choose Review diagnostics.
2. Start recording, reproduce the issue, stop recording, and refresh the preview. Recording stops automatically after ten minutes.
3. Review the contents, then choose Save report… with a new filename. Clear diagnostics discards the in-memory recording.

**Note:** Reports are never uploaded automatically. They exclude window titles, screenshots, track metadata, lyrics, file paths, and raw crash logs. Saved reports remain your files until you delete them.

### Crash reporting

The app can detect a locally captured crash from its previous session and offer a report or dismissal flow.

1. Choose a policy in Settings → General → Crash reports. Use Dismiss if you do not want to share a detected crash.
2. Use Report crash… only when you intend to send its text to GitHub in a prefilled issue URL.

**Note:** Clicking Report transmits crash text in the URL before an issue is submitted; logs may contain paths or other sensitive details. The current Always send label still uses the report/dismiss flow rather than automatic upload.

### Version, help, and local data

About shows the app version and links to the project, feedback, and support. Preferences and app-managed references stay on your Mac; there is no app analytics or automatic diagnostic-upload service.

1. Open Settings → About for version and support links.
2. Use the menu’s Debug tools command to reveal the app data directory when troubleshooting. Its normal location is ~/Library/Application Support/option-tab/.
3. Read the repository’s Data handling guide before sharing settings, raw crash text, or local data files.

**Note:** Update checks, optional remote artwork, and links opened in your browser make network requests. Local automation can return explicitly requested metadata or cached images to its authorized caller.

## Detailed reference

- [Local AppleScript automation](automation.md)
- [Data handling](data-handling.md)
- [Homebrew installation](homebrew.md)
- [Distribution and local builds](distribution.md)
- [Widget package format and runtime](../apps/desktop/internal/widgets/README.md)

This guide describes implemented product behavior and marks current native limits explicitly. It is not a claim that every third-party application, display arrangement, or physical gesture device has been validated.

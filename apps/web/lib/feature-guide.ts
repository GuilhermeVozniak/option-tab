/** Public feature inventory. Keep docs/features.md in sync when behavior changes. */
export interface FeatureGuideEntry {
  id: string;
  title: string;
  description: string;
  howTo: string[];
  notes?: string[];
}

export interface FeatureGuideSection {
  id: string;
  title: string;
  description: string;
  features: FeatureGuideEntry[];
}

export const featureGuideIntro = [
  "Option Tab is a free, open-source macOS window switcher with optional Dock previews, a configurable launcher, widgets, and local automation. The supported desktop release requires macOS 14 or later and runs on Apple Silicon and Intel.",
  "On a fresh setup, Command+Tab and Option+Tab open the window switcher: individual windows in a thumbnail grid, with no separate enlarged preview. Existing saved shortcut and appearance choices are preserved when you update.",
  "Open Settings from Option Tab’s menu-bar icon. Settings paths below use the English labels. Where an Editing selector appears, choose Window switcher or App switcher before changing that mode’s appearance, behavior, or ordering; changing Editing does not change what a shortcut opens.",
];

export const featureSections: FeatureGuideSection[] = [
  {
    id: "getting-started",
    title: "Getting started",
    description:
      "Install the app, grant the two core permissions, and restore the window grid on an existing setup.",
    features: [
      {
        id: "installation",
        title: "macOS installation",
        description:
          "Install the signed universal app with Homebrew or download its DMG. Option Tab lives in the menu bar, so it does not need a permanent app icon in the Dock.",
        howTo: [
          "Run: brew install --cask GuilhermeVozniak/tap/option-tab",
          "Alternatively, download the macOS DMG, move Option Tab to Applications, and open it.",
          "Use the menu-bar icon to open Settings, show the switcher, pause or resume switching, or quit.",
        ],
        notes: [
          "The feature guide describes the macOS product. Other-platform build targets do not provide the same native window and Dock integrations.",
        ],
      },
      {
        id: "permissions",
        title: "Permission onboarding",
        description:
          "The first-run setup shows live permission status. Accessibility enables global shortcuts and window actions; Screen Recording supplies live thumbnails.",
        howTo: [
          "Follow the onboarding prompts, or open Settings → General → Permissions.",
          "Grant Accessibility and Screen Recording in the corresponding macOS System Settings panes, then return to the app.",
          "If macOS asks you to quit and reopen the app after granting access, do so.",
        ],
        notes: [
          "Without Screen Recording, previews fall back to app icons. Music control, selected folders, and widgets have separate, explicit access choices.",
        ],
      },
      {
        id: "window-grid-default",
        title: "The default window grid",
        description:
          "Both default shortcuts open individual windows from all apps as thumbnails. The separate enlarged selected-window preview is off for the window switcher.",
        howTo: [
          "For an existing setup, open Settings → Shortcuts. For the shortcut you use, set Opens to Window switcher, Include to All windows, and Visual style to Thumbnails or Default style.",
          "Open Settings → Appearance, set Editing to Window switcher, choose Thumbnails, and turn off Preview the selected window.",
          "Use Settings → General → Backup and reset → Reset to defaults only if you want to reset all feature preferences, not just the preview layout.",
        ],
        notes: [
          "Updates keep saved preferences. All windows here means all applications; Window rules can still limit Spaces, screens, and window states.",
        ],
      },
    ],
  },
  {
    id: "switching",
    title: "Switching and window actions",
    description:
      "Move between individual windows or applications, search, and act on the selected window.",
    features: [
      {
        id: "window-switcher",
        title: "Window-level switching",
        description:
          "Each eligible window has its own entry, including separate windows belonging to the same application. This makes the exact document or browser window directly selectable.",
        howTo: [
          "Hold Command or Option and press Tab to open the default window switcher. Keep holding the modifier and press Tab again to advance; Shift+Tab goes backward.",
          "Release the modifier to focus the selection. Enter or clicking a window also confirms; Escape or clicking outside cancels.",
        ],
      },
      {
        id: "app-switcher",
        title: "Optional app switcher",
        description:
          "The app switcher presents an application strip with the selected app’s windows underneath, including running apps without an open window. It remains available as a separate mode.",
        howTo: [
          "Open Settings → Shortcuts and set a shortcut’s Opens field to App switcher.",
          "Use the application strip to choose an app, then select a window or use the available Open app, New window, Hide app, and Quit app actions.",
          "Choose Editing: App switcher in Appearance or Shortcuts to adjust that mode independently.",
        ],
        notes: [
          "The app strip and window grid are different modes. The Editing selector chooses which preferences you edit; the shortcut’s Opens field chooses the mode you actually launch.",
        ],
      },
      {
        id: "custom-shortcuts",
        title: "Up to nine independent shortcuts",
        description:
          "Each shortcut can open a different mode, include all apps or only the active app, and override style, Space, screen, ordering, and release behavior.",
        howTo: [
          "In Settings → Shortcuts, click an existing shortcut field and press the desired key combination, or choose + Add shortcut.",
          "Choose Opens, Include, and Visual style. Expand Behavior and overrides for On release, Spaces, Screens, and Order.",
          "Use the switch beside a shortcut to disable it without deleting it.",
        ],
      },
      {
        id: "search-navigation",
        title: "Search and keyboard navigation",
        description:
          "Type to filter windows by title or app name with fuzzy matching. Tab navigation is available alongside optional arrow keys and Vim keys.",
        howTo: [
          "Open the switcher and type part of a title or app name to search.",
          "In Settings → Shortcuts → Keyboard navigation, enable Navigate with arrow keys or Navigate with vim keys (h / j / k / l).",
          "Adjust Hold modifier to cycle and each shortcut’s On release behavior if you prefer an explicit confirmation.",
        ],
        notes: [
          "When Vim navigation is enabled, unmodified h, j, k, and l are navigation keys rather than search text.",
        ],
      },
      {
        id: "window-actions",
        title: "Window and application controls",
        description:
          "Focus, close, minimize or restore, enter or leave fullscreen, hide or show an app, and quit it from the switcher. Additional actions include New window, Force quit, Close all windows, and Minimize all windows.",
        howTo: [
          "Use the controls on each window preview or the toolbar icons. Hover an icon to see its action label. Window controls stay visible while you cycle with Command+Tab or Option+Tab; enable them in Settings → Appearance → Window information if they are hidden.",
          "With the activation modifier held, the default action keys are W for close, M for minimize/restore, F for fullscreen, H for hide/show, and Q for quit.",
          "Open Settings → Shortcuts → Window actions to change physical-key bindings, add a binding, or disable action keys.",
        ],
        notes: [
          "Availability depends on the target app and its macOS Accessibility controls. Unsupported or refused actions are reported. Close keeps the app’s normal save prompts; Force quit can discard unsaved work.",
        ],
      },
      {
        id: "pointer-gestures",
        title: "Mouse, gestures, and haptics",
        description:
          "Select by hovering, move the cursor to a focused window, assign middle-click and swipe actions, and request a haptic tick when selection changes.",
        howTo: [
          "Open Settings → Shortcuts and select the mode to edit.",
          "Use Mouse behavior for hover selection and Cursor follows focus; use Keyboard navigation for Trackpad haptic feedback.",
          "Under Window actions, assign Middle click, Swipe up, and Swipe down where available.",
        ],
        notes: [
          "Gesture and haptic support depends on the native input source and your hardware. Precise trackpad input is used; macOS does not reliably expose a finger count. A mouse-only setup does not provide trackpad gestures.",
        ],
      },
    ],
  },
  {
    id: "appearance",
    title: "Appearance and previews",
    description:
      "Keep the thumbnail grid or tailor each switcher’s layout, theme, information, and timing.",
    features: [
      {
        id: "styles-layout",
        title: "Thumbnails, icons, and titles",
        description:
          "Choose thumbnail cards, app icons, or a compact title presentation. Horizontal and vertical layouts and a window-count threshold for title mode support different working sets.",
        howTo: [
          "Open Settings → Appearance and choose the switcher in Editing.",
          "Choose the visual style and Layout direction. Set Use titles at window count to automatically switch to titles above that threshold; zero disables it.",
          "A shortcut’s Visual style override takes precedence over the mode’s default style.",
        ],
      },
      {
        id: "sizing",
        title: "Preset and fine-grained sizing",
        description:
          "Small, Medium, and Large presets set a starting size. Auto-sizing fits thumbnails to the window count, while explicit controls let you tune density.",
        howTo: [
          "Use Settings → Appearance → Style and theme for the size preset.",
          "Under Layout and sizing, adjust maximum rows and columns, thumbnail size, icon size, title width, font size, and Auto-size thumbnails.",
        ],
      },
      {
        id: "themes-materials",
        title: "Independent themes and surface styling",
        description:
          "Switcher themes can follow the system or use light or dark mode, with an accent color, background opacity, blur, and corner radius. Settings has its own neutral System / Light / Dark preference.",
        howTo: [
          "Open Settings → Appearance → Style and theme for the selected switcher’s theme and accent.",
          "Use Surface and effects for opacity, corner radius, and blur.",
          "Use the theme choices at the bottom of the Settings sidebar to change Settings alone.",
        ],
        notes: [
          "The Settings theme is a local presentation preference and is separate from exported switcher and Dock settings.",
        ],
      },
      {
        id: "preview-placement",
        title: "Optional enlarged preview and display placement",
        description:
          "An optional larger preview follows the selected window. The switcher can appear on the cursor’s display, the active screen, or the focused window’s screen.",
        howTo: [
          "In Settings → Appearance → Style and theme, turn Preview the selected window on or off for the selected switcher.",
          "Choose the display using Show on. Leave the enlarged preview off for the default compact window grid.",
        ],
      },
      {
        id: "window-information",
        title: "Titles, app badges, and window state",
        description:
          "Show window titles, app icon badges, window controls, minimized/hidden/fullscreen status, and Space labels. Long titles can truncate at the start, middle, or end.",
        howTo: [
          "Open Settings → Appearance → Window information and enable the details you want on each card.",
        ],
        notes: [
          "Space information depends on what macOS exposes. Labels describe the detected Spaces, not user-defined desktop names.",
        ],
      },
      {
        id: "capture-motion",
        title: "Capture freshness and motion",
        description:
          "Visible previews capture window content as needed. Optional background capture keeps thumbnails fresh between activations. Appearance controls adjust the show delay, exit fade, and selected-preview fade.",
        howTo: [
          "Enable Settings → General → Window capture → Capture windows in the background if you want previews prepared before the switcher opens.",
          "Use Settings → Appearance → Motion and timing to change Apparition delay, Fade out animation, and Fade in the selected-window preview.",
        ],
        notes: [
          "Background capture is off by default. It requires Screen Recording and can keep macOS’s recording indicator visible while the switcher is hidden.",
        ],
      },
    ],
  },
  {
    id: "filtering",
    title: "Window rules and exclusions",
    description:
      "Window filters are shared by both switchers; each mode has its own ordering. Individual shortcuts can override Space, screen, and ordering rules.",
    features: [
      {
        id: "ordering",
        title: "Window ordering",
        description:
          "Order entries by recently focused, recently created, alphabetically, or by Space.",
        howTo: [
          "Open Settings → Window rules, choose the switcher in Editing, and set Display order.",
          "For a single shortcut, use Settings → Shortcuts → Behavior and overrides → Order.",
        ],
      },
      {
        id: "scope-filters",
        title: "Spaces, screens, and visibility filters",
        description:
          "Include every Space or only the active one; every screen, the active screen, or the screen under the cursor. Minimized, hidden, and fullscreen windows can be shown, hidden, or placed at the end.",
        howTo: [
          "Open Settings → Window rules → Which windows to show and choose the scope and visibility rules.",
          "Use Show windows without a title to include or omit untitled windows.",
          "Set a shortcut’s Include to Active app only when you want to cycle just that app’s windows.",
        ],
        notes: [
          "The Which windows to show controls affect both switchers, regardless of the Editing selection. A shortcut’s Space and screen overrides take precedence when that shortcut is used.",
        ],
      },
      {
        id: "excluded-apps",
        title: "App exclusions and shortcut exceptions",
        description:
          "Match an app by its name or bundle identifier. Hide it always or when it has no open window, and optionally ignore Option Tab shortcuts while it is frontmost.",
        howTo: [
          "Open Settings → Excluded apps and choose + Add app.",
          "Enter the app name or bundle ID, select the hide rule, and enable Ignore shortcuts when needed.",
        ],
        notes: [
          "These exclusions apply to both switchers. An exact bundle identifier is useful when app names are similar.",
        ],
      },
    ],
  },
  {
    id: "dock-previews",
    title: "Native Dock enhancements",
    description:
      "Add app-window previews and folder browsing to the macOS Dock without replacing it.",
    features: [
      {
        id: "dock-hover",
        title: "Window previews on hover",
        description:
          "Rest the pointer on an app’s native Dock icon to see its windows and available actions. Dock previews have their own appearance and timing settings.",
        howTo: [
          "Open Settings → Dock → Window previews and turn on Enable Dock previews.",
          "Grant Accessibility and Screen Recording, then hover over a running app’s Dock icon.",
          "Choose Applications, visual style, sizing, information, and effects in the same section.",
        ],
        notes: [
          "This optional feature starts off. Preview availability depends on the app’s windows and the Dock identity exposed by macOS.",
        ],
      },
      {
        id: "dock-input",
        title: "Dock clicks, scrolling, and swipes",
        description:
          "Optional controls let you click a Dock icon to hide an app, scroll to show or hide it, Command-right-click to quit, and assign middle-click or directional swipe actions.",
        howTo: [
          "Open Settings → Dock → Window previews → Input and gestures.",
          "Enable the click or scroll options you want and choose actions for middle-click and the four swipe directions.",
          "Under Timing and movement, tune hover delay, dismiss delay, movement tolerance, corridor padding, and card spacing.",
        ],
        notes: [
          "Precise trackpad scrolling powers swipes. Actions can be unavailable when macOS or the target app does not expose the required control.",
        ],
      },
      {
        id: "preview-drag",
        title: "Move windows from previews and Aero Shake",
        description:
          "Drag a preview to move its window. An optional shake action during preview dragging can minimize or close other windows.",
        howTo: [
          "Enable Settings → Dock → Window previews → Input and gestures → Drag previews to move windows.",
          "Choose an Aero Shake action, or leave it set to None.",
        ],
        notes: [
          "Window movement depends on Accessibility support and the window’s current state. Choose Close other windows only when you intend to close them; target apps retain their normal save behavior.",
        ],
      },
      {
        id: "folder-pop",
        title: "Folder Pop",
        description:
          "Browse a native Dock folder’s contents without first opening a Finder window. Sort by name, modification date, size, or kind, reverse the order, and put folders first.",
        howTo: [
          "Enable Settings → Dock → Window previews → Enable Folder Pop, then hover over a folder in the native Dock.",
          "If requested, use the folder access chooser to grant access to that folder.",
          "Use the panel’s sorting controls and select an entry to open it with its associated app.",
        ],
        notes: [
          "Access is requested explicitly. Moving or replacing a selected folder can require granting access again.",
        ],
      },
      {
        id: "monitor-protection",
        title: "Keep the native Dock on a monitor",
        description:
          "Monitor protection watches the native Dock’s verified location and helps keep it on the selected display, without changing persistent Dock preferences.",
        howTo: [
          "Open Settings → Dock → Monitor, enable Lock Dock to a monitor, and choose the main display or a connected monitor.",
          "Move the Dock to that display manually and wait for the status to show Protected.",
          "Hold the selected Bypass modifier to temporarily bypass protection.",
        ],
        notes: [
          "Automatic Move Dock here placement is currently disabled. Protection only starts after location verification; disconnected displays, inaccessible edges, or missing Accessibility permission can prevent it.",
        ],
      },
    ],
  },
  {
    id: "media",
    title: "Music and media panels",
    description:
      "Optional Apple Music and Spotify controls, artwork, and synchronized local lyrics.",
    features: [
      {
        id: "media-players",
        title: "Apple Music and Spotify playback",
        description:
          "Show track metadata and supported play/pause, previous, and next controls for an enabled running player. A player’s Dock hover can switch between its Windows and Media views.",
        howTo: [
          "Open Settings → Dock → Media. Enable media controls and the player you use, then click Connect and grant the requested macOS Automation permission.",
          "Open the player and hover its Dock icon. Choose Media if a Windows / Media selector appears.",
          "Use the controls shown as available for the current player and track.",
        ],
        notes: [
          "Players are opt-in and are not launched just to poll metadata. Apple Music seeking is available when the player supplies a usable duration. Spotify seeking and duration remain disabled pending native verification.",
        ],
      },
      {
        id: "media-pins",
        title: "Pinned media panels",
        description:
          "Keep a media panel available after leaving its Dock icon, with its own movable presentation.",
        howTo: [
          "Use the Pin control in an available media panel.",
          "Move it by its header and use its close control when finished.",
        ],
        notes: [
          "Media pins last for the current app session; they are not permanent desktop widgets.",
        ],
      },
      {
        id: "lyrics",
        title: "Synchronized lyrics",
        description:
          "Follow timestamped lyrics supplied by Apple Music when available, or associate a local LRC file with a track. Local files support a timing offset and take priority over provider text.",
        howTo: [
          "Open the Media panel’s Synchronized lyrics area and import a local .lrc file for the current track.",
          "Adjust the timing offset if needed; use the panel controls to return to the current line, replace the file, or remove its association.",
        ],
        notes: [
          "There is no lyrics search or download service. Plain text without timestamps cannot be synchronized. Removing an association does not delete the original file, and Music’s catalog lyrics are not guaranteed to be exposed to other apps.",
        ],
      },
      {
        id: "artwork",
        title: "Optional artwork",
        description:
          "Apple Music artwork can come from the local player. Spotify’s remote artwork is a separate opt-in; playback metadata and controls remain available without it.",
        howTo: [
          "Open Settings → Dock → Media and enable Allow remote artwork only if you want the app to fetch player-supplied cover images.",
        ],
        notes: ["Remote artwork contacts the image host over HTTPS. It is off by default."],
      },
    ],
  },
  {
    id: "replacement-dock",
    title: "Replacement Dock and profiles",
    description:
      "An optional launcher with persistent pins, groups, per-display layouts, and app-based profile rules.",
    features: [
      {
        id: "launcher-enable",
        title: "Optional floating launcher",
        description:
          "The replacement Dock provides a configurable launcher alongside the native macOS Dock, which remains available.",
        howTo: [
          "Open Settings → Dock → Launcher and turn on Enable replacement Dock.",
          "Choose the profile to edit and use the Profile, Items, Widgets, Layout, Interactions, Displays, and Focus rules sections.",
          "Use Use native Dock to turn the replacement launcher off and return to the system Dock.",
        ],
      },
      {
        id: "launcher-profiles",
        title: "Multiple launcher profiles",
        description:
          "Keep up to eight profiles with separate items, widgets, appearance, and interaction choices. Create, rename, duplicate, and delete profiles without rebuilding every layout.",
        howTo: [
          "Open Settings → Dock → Launcher → Profile. Use New profile or Duplicate profile, then give it a name.",
          "Before deleting a profile, choose its replacement so display assignments and focus rules can be reassigned.",
        ],
      },
      {
        id: "launcher-items",
        title: "Pins, links, groups, and custom icons",
        description:
          "Pin applications, folders, files, and web links; organize applications into groups; insert spacers and separators; and choose custom PNG icons. Running apps are matched to existing pins to avoid duplicates.",
        howTo: [
          "Open Settings → Dock → Launcher → Items and use the item editor to select local apps, files, or folders, add a link, and arrange groups and decorative items.",
          "Choose a PNG icon where offered, then save the item changes.",
          "If an item moves or loses access, use its relink or select-again control in Settings.",
        ],
      },
      {
        id: "launcher-reorder",
        title: "Reorder items directly on the Dock",
        description:
          "Optional item handles let you rearrange persistent pins and create application groups from the running launcher.",
        howTo: [
          "Enable Settings → Dock → Launcher → Items → Reorder items on the Dock.",
          "Use the visible handles to move pins or use the available grouping controls. The Settings item editor also supports ordering and grouping.",
        ],
      },
      {
        id: "launcher-panels",
        title: "Folder and application panels",
        description:
          "Pinned folders open list or grid panels; application controls provide available launch, window, and relaunch actions. Show-all window panels let you choose an exact app window.",
        howTo: [
          "Add a folder or app under Settings → Dock → Launcher → Items, then use that item in the launcher.",
          "Click a folder item to browse its contents using the panel’s list/grid controls. Right-click the folder item and choose Open folder to open the root folder itself.",
          "Right-click an app item and choose Show all windows or Relaunch when available.",
        ],
        notes: [
          "Relaunch closes and reopens the selected app, so its normal quit/save behavior still applies. The launcher is not an arbitrary shell-command runner.",
        ],
      },
      {
        id: "launcher-layout",
        title: "Layout, materials, and magnification",
        description:
          "Place a floating or full-width launcher on any of the four screen edges. Tune alignment, icon size, maximum length, thickness, inset, spacing, labels, auto-hide, theme, material, tint, opacity, border, and radius.",
        howTo: [
          "Open Settings → Dock → Launcher → Layout and adjust Position and size and Appearance for the selected profile.",
          "Enable magnification and choose its scale and reach to enlarge nearby items as the pointer moves over them.",
        ],
        notes: [
          "Animation respects reduced-motion settings. Layout includes Dock-overlap avoidance; it does not tile application windows.",
        ],
      },
      {
        id: "launcher-displays-focus",
        title: "Display assignments and focus rules",
        description:
          "Assign base profiles to displays and override them when a chosen application becomes frontmost. The first matching enabled app rule wins.",
        howTo: [
          "Open Settings → Dock → Launcher → Displays to assign profiles to displays.",
          "Open Focus rules, add a rule, select a running app or enter its exact bundle identifier, and choose the destination profile and display scope.",
          "Reorder rules to set priority; the display assignment remains the fallback when no rule matches.",
        ],
      },
      {
        id: "launcher-badges",
        title: "Available native Dock badges",
        description:
          "Mirror supported count badges or generic indicators from the native Dock onto matching launcher app items.",
        howTo: [
          "Enable Settings → Dock → Launcher → Items → Show Dock badges for each profile where you want them.",
        ],
        notes: [
          "Badges require Accessibility and a supported source app/macOS version. Missing values stay hidden. This reads Dock status, not Notification Center or notification message contents.",
        ],
      },
      {
        id: "launcher-interactions",
        title: "Launcher gestures and haptics",
        description:
          "Optional precise scrolling, pinch, and swipe gestures can navigate items or show and hide a selected preview. Haptics provide feedback where the device supports them.",
        howTo: [
          "Open Settings → Dock → Launcher → Interactions and enable launcher interactions.",
          "Enable the available gesture sources, then choose Primary, Toward, and Pinch actions and optional haptic feedback.",
        ],
        notes: [
          "Hardware and native capability determine which switches are available. Launcher letter/keyboard navigation is currently unavailable; do not rely on that disabled setting. Physical gesture behavior varies by device.",
        ],
      },
      {
        id: "launcher-profile-transfer",
        title: "Portable profile import and export",
        description:
          "Share a profile’s structural layout, items, and widget settings. Import reviews the document and creates a new, unassigned profile.",
        howTo: [
          "Open Settings → Dock → Launcher → Profile and use its export or import controls.",
          "Review the incoming profile, import it, restore requested local item selections and widget access, then assign it under Displays when ready.",
        ],
        notes: [
          "Private file/folder references and access grants are not a portable permission backup. Imported profiles need local repair or consent before those features become available.",
        ],
      },
    ],
  },
  {
    id: "widgets",
    title: "Widgets and local packages",
    description:
      "Add useful local information and typed actions to launcher profiles, with access controlled per widget.",
    features: [
      {
        id: "built-in-widgets",
        title: "Clock, battery, network, and audio",
        description:
          "Built-in widgets show local time, battery charge and power state, network status with optional usage rates, and audio output status. The clock supports time format and timezone choices.",
        howTo: [
          "Open Settings → Dock → Launcher → Widgets, choose a Widget package, and click Add widget.",
          "Review Required and Optional access, grant the capabilities you want, enable the widget, and adjust its settings.",
          "Reorder or remove instances within the profile using the widget controls.",
        ],
        notes: [
          "A widget without required access remains unavailable. Optional data can remain unavailable while its other permitted content still works.",
        ],
      },
      {
        id: "audio-output",
        title: "Choose an audio output",
        description:
          "The audio widget can offer an explicit output-device selector in addition to displaying current output, volume, and mute state.",
        howTo: [
          "Add the Audio widget in Settings → Dock → Launcher → Widgets.",
          "Grant its optional Choose audio output capability, then use the widget’s output action to select an available device.",
        ],
        notes: [
          "This changes the selected system output device; it is not a per-application audio mixer.",
        ],
      },
      {
        id: "widget-stacks",
        title: "Widget stacks",
        description:
          "Group two to four widgets into one launcher slot and choose which one is shown. Each profile supports up to four visible widget slots; a stack occupies one slot.",
        howTo: [
          "In Settings → Dock → Launcher → Widgets → Stacks, name a stack and select its members.",
          "Choose the Shown widget, reorder members where offered, or dissolve the stack to return them to separate slots.",
          "On the running Dock, click a stack’s member buttons to change the shown widget directly.",
        ],
      },
      {
        id: "widget-packages",
        title: "Review and install local widget packages",
        description:
          "Install declarative widget ZIPs containing layouts, declared assets, fixed data bindings, and supported actions. Custom packages can also display granted Apple Music or Spotify metadata and offer supported playback controls. Packages cannot include arbitrary executable scripts.",
        howTo: [
          "Open Settings → Dock → Launcher → Widgets → Widget packages and choose Review local package….",
          "Review its version, declared data access, actions, and unverified-publisher notice, then choose Install reviewed package.",
          "Add an instance from the widget catalog and grant its capabilities separately. Remove installed packages from the same section when no longer needed.",
        ],
        notes: [
          "Installing a package grants no data access or actions. Community publisher identity is not independently verified. This is a local package workflow, not an online widget marketplace.",
          "Media widgets require a custom package and the appropriate data/control grants; the four built-in widgets remain Clock, Battery, Network, and Audio. Player permission and capability limits still apply, including unavailable Spotify seeking.",
        ],
      },
    ],
  },
  {
    id: "automation",
    title: "Local AppleScript automation",
    description:
      "Use a bundled scripting dictionary to query windows, show a switcher, or request supported actions.",
    features: [
      {
        id: "automation-queries",
        title: "Query applications and windows",
        description:
          "Local AppleScript callers can query running applications, windows, and the active window. Replies are bounded JSON with explicit status and truncation information.",
        howTo: [
          "Run from Terminal: osascript -e 'tell application id \"com.optiontab.app\" to query applications'",
          "Use query windows or query active window in the same tell block for window metadata. Use the returned IDs when targeting a particular window.",
        ],
        notes: [
          "The calling application can need macOS Automation permission. Explicit image queries return only eligible, already-cached previews; they do not initiate capture or prompt for Screen Recording.",
        ],
      },
      {
        id: "automation-presentations",
        title: "Open switchers and app previews",
        description:
          "Automation can open the window or app switcher and show an independently owned preview panel for an exactly identified running app.",
        howTo: [
          "Run: osascript -e 'tell application id \"com.optiontab.app\" to open switcher mode windows mode'",
          "Use apps mode for the app switcher. The show app previews command accepts an exact app selector and optional screen coordinates; retain its returned presentation token to hide that same preview.",
        ],
        notes: [
          "Paused or inactive sessions refuse new presentations. Automation admission is not a guarantee that every app window can be presented; stale or unsupported identities return errors.",
        ],
      },
      {
        id: "automation-actions",
        title: "Typed window actions for scripts and macros",
        description:
          "AppleScript-capable macro tools can request focus, close, minimize, hide application, and an explicit fullscreen state for a selected window.",
        howTo: [
          "Query a live window ID, then use perform window action with the desired operation and that exact ID, or use the active-window selector.",
          "Use the repository’s Local AppleScript automation guide for complete syntax, permissions, limits, and error handling.",
        ],
        notes: [
          "Automation is local and same-user; it is not a network server or arbitrary script executor. This suite does not offer bulk close, quit, force quit, or tiling. Native app support and the caller’s permissions still apply.",
        ],
      },
    ],
  },
  {
    id: "maintenance",
    title: "Preferences, updates, and support",
    description: "Manage startup, languages, backups, updates, and reviewed diagnostic reports.",
    features: [
      {
        id: "startup-language",
        title: "Startup, menu-bar style, and languages",
        description:
          "Launch at login, choose the menu-bar icon style or hide it, and use English, Brazilian Portuguese, Spanish, or the system language.",
        howTo: [
          "Open Settings → General → App behavior and adjust Start at login, Menubar icon, and Language.",
          "Use the menu-bar Pause/Resume command when you temporarily want Option Tab to stop intercepting activation shortcuts.",
        ],
        notes: ["Missing translations fall back to English."],
      },
      {
        id: "settings-backup",
        title: "Settings backup, import, and reset",
        description:
          "Export preferences as JSON, import a saved configuration, or restore feature defaults.",
        howTo: [
          "Open Settings → General → Backup and reset and choose Export… or Import….",
          "Use Reset to defaults to restore the standard feature setup, including the default window grid.",
        ],
        notes: [
          "Review exported preferences before sharing. A settings file is not a backup of macOS permissions, local folder grants, lyric files, or all private launcher resources. Use profile transfer for a portable launcher layout.",
        ],
      },
      {
        id: "updates",
        title: "Update checking and installation",
        description:
          "The default policy periodically checks GitHub releases and announces updates. You can choose automatic installation or turn scheduled checks off; manual checking remains available.",
        howTo: [
          "Open Settings → General → Updates to choose Check for updates periodically, Auto-install updates, or Don’t check for updates.",
          "Use Check for updates now… for a manual check. Auto-install can download, install, and restart the app after a scheduled check.",
          "For Homebrew-managed updates, quit Option Tab and run brew update followed by brew upgrade --cask --greedy GuilhermeVozniak/tap/option-tab.",
        ],
        notes: [
          "A manual update check alone does not auto-install. Scheduled checks contact GitHub and do not attach window data or settings.",
        ],
      },
      {
        id: "diagnostics",
        title: "Reviewed local diagnostics",
        description:
          "Record a bounded support session, review its JSON report, and save the reviewed snapshot locally. Reports include version, coarse status, and diagnostic events rather than window or media content.",
        howTo: [
          "Open Settings → About → Diagnostics and choose Review diagnostics.",
          "Start recording, reproduce the issue, stop recording, and refresh the preview. Recording stops automatically after ten minutes.",
          "Review the contents, then choose Save report… with a new filename. Clear diagnostics discards the in-memory recording.",
        ],
        notes: [
          "Reports are never uploaded automatically. They exclude window titles, screenshots, track metadata, lyrics, file paths, and raw crash logs. Saved reports remain your files until you delete them.",
        ],
      },
      {
        id: "crash-reports",
        title: "Crash reporting",
        description:
          "The app can detect a locally captured crash from its previous session and offer a report or dismissal flow.",
        howTo: [
          "Choose a policy in Settings → General → Crash reports. Use Dismiss if you do not want to share a detected crash.",
          "Use Report crash… only when you intend to send its text to GitHub in a prefilled issue URL.",
        ],
        notes: [
          "Clicking Report transmits crash text in the URL before an issue is submitted; logs may contain paths or other sensitive details. The current Always send label still uses the report/dismiss flow rather than automatic upload.",
        ],
      },
      {
        id: "about-data",
        title: "Version, help, and local data",
        description:
          "About shows the app version and links to the project, feedback, and support. Preferences and app-managed references stay on your Mac; there is no app analytics or automatic diagnostic-upload service.",
        howTo: [
          "Open Settings → About for version and support links.",
          "Use the menu’s Debug tools command to reveal the app data directory when troubleshooting. Its normal location is ~/Library/Application Support/option-tab/.",
          "Read the repository’s Data handling guide before sharing settings, raw crash text, or local data files.",
        ],
        notes: [
          "Update checks, optional remote artwork, and links opened in your browser make network requests. Local automation can return explicitly requested metadata or cached images to its authorized caller.",
        ],
      },
    ],
  },
];

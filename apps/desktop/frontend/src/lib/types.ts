// TypeScript mirror of the Go switcher State (internal/switcher/switcher.go).
// The Go layer serializes these as JSON over Wails events.

export type VisualStyle = "thumbnails" | "appIcons" | "titles";
export type Theme = "system" | "light" | "dark";
export type SizePreset = "small" | "medium" | "large";
export type Placement = "activeScreen" | "cursorScreen" | "focusedWindowScreen";
export type TruncationMode = "end" | "middle" | "start";
export type LayoutDirection = "horizontal" | "vertical";
export type WindowAction =
  | "close"
  | "minimize"
  | "fullscreen"
  | "hide"
  | "quit"
  | "newWindow"
  | "forceQuit"
  | "closeAll"
  | "minimizeAll";
export type PointerAction = "none" | "close" | "minimize" | "fullscreen" | "hide" | "quit";
export type SwitcherMode = "windows" | "apps";

// PermState mirrors platform.PermState; "unknown" covers the not-yet-determined
// state. PermKey names the permissions the switcher needs.
export type PermState = "granted" | "denied" | "unknown";
export type PermKey = "accessibility" | "screenRecording";

export interface Permissions {
  accessibility: PermState;
  screenRecording: PermState;
}

export interface Appearance {
  style: VisualStyle;
  theme: Theme;
  sizePreset: SizePreset;
  maxRows: number;
  maxColumns: number;
  thumbnailMaxPx: number;
  iconSizePx: number;
  titleMaxWidthPx: number;
  fontSizePx: number;
  accentColor: string;
  backgroundOpacity: number;
  blur: boolean;
  cornerRadiusPx: number;
  showAppBadge: boolean;
  showTitle: boolean;
  showWindowControls: boolean;
  autoSize: boolean;
  apparitionDelayMs: number;
  fadeOutAnimation: boolean;
  showStatusIcons: boolean;
  showSpaceNumbers: boolean;
  titleTruncation: TruncationMode;
  previewSelected: boolean;
  previewFade: boolean;
  compactThreshold: number;
  layoutDirection: LayoutDirection;
}

export interface Entry {
  windowId: number;
  appId: number;
  title: string;
  appName: string;
  bundleId: string;
  spaceId?: number;
  minimized: boolean;
  hidden: boolean;
  fullscreen: boolean;
  icon?: string;
  thumbnail?: string;
  // preview is a higher-resolution capture streamed for the selected entry
  // when "preview selected window" is enabled.
  preview?: string;
}

export interface SwitcherState {
  session?: number;
  revision?: number;
  open: boolean;
  style: VisualStyle;
  appearance: Appearance;
  placement: Placement;
  entries: Entry[];
  selected: number;
  search: string;
  shortcutId: number;
  vimKeys: boolean;
  arrowKeys: boolean;
  mouseHover: boolean;
  activeSpaceId: number;
  actionBindings: Record<string, WindowAction>;
  middleClickAction: PointerAction;
  swipeUpAction: PointerAction;
  swipeDownAction: PointerAction;
  mode?: SwitcherMode;
  apps?: AppEntry[];
  selectedWindowId?: number;
}

export interface AppEntry {
  appId: number;
  appName: string;
  bundleId: string;
  hidden: boolean;
  windowCount: number;
  windowPresence?: "present" | "none" | "unknown";
  icon?: string;
}

export interface Bounds {
  x: number;
  y: number;
  w: number;
  h: number;
}
export interface DockItem {
  appId: number;
  bundleId: string;
  path: string;
  title: string;
  bounds: Bounds;
  screenId: number;
  edge: string;
  kind: string;
}
export interface DockViewState {
  session: number;
  revision?: number;
  open?: boolean;
  item: DockItem;
  entries: Entry[];
  selectedWindowId: number;
  appearance: Appearance;
  emptyReason: string;
  error?: string;
  pointer?: DockPointer;
}
export interface DockPointer {
  session: number;
  sequence: number;
  x: number;
  y: number;
  inside: boolean;
}

export const emptyState: SwitcherState = {
  open: false,
  style: "thumbnails",
  appearance: {
    style: "thumbnails",
    theme: "system",
    sizePreset: "medium",
    maxRows: 4,
    maxColumns: 6,
    thumbnailMaxPx: 280,
    iconSizePx: 32,
    titleMaxWidthPx: 240,
    fontSizePx: 13,
    accentColor: "#3b82f6",
    backgroundOpacity: 0.85,
    blur: true,
    cornerRadiusPx: 12,
    showAppBadge: true,
    showTitle: true,
    showWindowControls: true,
    autoSize: true,
    apparitionDelayMs: 0,
    fadeOutAnimation: true,
    showStatusIcons: true,
    showSpaceNumbers: true,
    titleTruncation: "end",
    previewSelected: false,
    previewFade: true,
    compactThreshold: 0,
    layoutDirection: "horizontal",
  },
  placement: "cursorScreen",
  entries: [],
  selected: 0,
  search: "",
  shortcutId: 0,
  vimKeys: false,
  arrowKeys: true,
  mouseHover: true,
  activeSpaceId: 0,
  actionBindings: {
    KeyW: "close",
    KeyM: "minimize",
    KeyQ: "quit",
    KeyH: "hide",
    KeyF: "fullscreen",
  },
  middleClickAction: "close",
  swipeUpAction: "none",
  swipeDownAction: "none",
  mode: "windows",
  apps: [],
  selectedWindowId: 0,
};

// ---- Settings (mirror of internal/config.Settings) ----

export type OrderMode = "recent" | "recentlyCreated" | "alphabetical" | "space";
export type SpaceScope = "active" | "all";
export type ScreenScope = "active" | "all" | "cursor";
export type AppScopeMode = "all" | "activeApp";
export type WindowVisibility = "show" | "hide" | "showAtEnd";
export type ReleaseAction = "focusSelected" | "doNothing";
export type MenubarIconStyle = "default" | "outline" | "dot";
export type UpdatePolicy = "off" | "check" | "auto";
export type CrashPolicy = "never" | "ask" | "always";
export type BlacklistHide = "always" | "whenNoWindow";

export interface BlacklistEntry {
  match: string;
  hide: BlacklistHide;
  ignoreShortcuts: boolean;
}

export interface ShortcutScope {
  appScope: AppScopeMode;
  spaces?: SpaceScope;
  screens?: ScreenScope;
  order?: OrderMode;
}

export interface Shortcut {
  id: number;
  chord: string;
  enabled: boolean;
  scope: ShortcutScope;
  styleOverride?: VisualStyle;
  whenReleased?: ReleaseAction;
  mode?: SwitcherMode;
}

export interface Filters {
  spaces: SpaceScope;
  screens: ScreenScope;
  showMinimized: WindowVisibility;
  showHiddenApps: WindowVisibility;
  showFullscreen: WindowVisibility;
  showWindowsWithoutTitle: boolean;
  appBlacklist: BlacklistEntry[];
}

export interface Behavior {
  holdToCycle: boolean;
  startAtLogin: boolean;
  paused: boolean;
  showMenubarIcon: boolean;
  vimKeys: boolean;
  arrowKeys: boolean;
  menubarIconStyle: MenubarIconStyle;
  language: string;
  updatePolicy: UpdatePolicy;
  crashReports: CrashPolicy;
  mouseHoverSelect: boolean;
  cursorFollowFocus: boolean;
  hapticFeedback: boolean;
  captureInBackground: boolean;
  onboarded: boolean;
  actionBindings: Record<string, WindowAction>;
  middleClickAction: PointerAction;
  swipeUpAction: PointerAction;
  swipeDownAction: PointerAction;
}

export interface SwitcherBehavior {
  holdToCycle: boolean;
  vimKeys: boolean;
  arrowKeys: boolean;
  mouseHoverSelect: boolean;
  cursorFollowFocus: boolean;
  hapticFeedback: boolean;
  actionBindings: Record<string, WindowAction>;
  middleClickAction: PointerAction;
  swipeUpAction: PointerAction;
  swipeDownAction: PointerAction;
}

export interface ModePreferences {
  appearance: Appearance;
  behavior: SwitcherBehavior;
  order: OrderMode;
  placement: Placement;
}

export interface DockSettings {
  enabled: boolean;
  hoverDelayMs: number;
  dismissDelayMs: number;
  hoverSlopPx: number;
  bridgePaddingPx: number;
  scope: ShortcutScope;
  appearance: Appearance;
}

export interface Settings {
  version: number;
  shortcuts: Shortcut[];
  appearance: Appearance;
  filters: Filters;
  order: OrderMode;
  placement: Placement;
  behavior: Behavior;
  appSwitcher: ModePreferences;
  dock: DockSettings;
}

const DEFAULT_ACTION_BINDINGS: Record<string, WindowAction> = {
  KeyW: "close",
  KeyM: "minimize",
  KeyQ: "quit",
  KeyH: "hide",
  KeyF: "fullscreen",
};

export const defaultSettings: Settings = {
  version: 3,
  shortcuts: [
    { id: 1, chord: "command+tab", enabled: true, scope: { appScope: "all" }, mode: "apps" },
    {
      id: 2,
      chord: "option+tab",
      enabled: true,
      scope: { appScope: "activeApp" },
      mode: "windows",
    },
  ],
  appearance: { ...emptyState.appearance },
  filters: {
    spaces: "all",
    screens: "all",
    showMinimized: "show",
    showHiddenApps: "show",
    showFullscreen: "show",
    showWindowsWithoutTitle: false,
    appBlacklist: [],
  },
  order: "recent",
  placement: "cursorScreen",
  behavior: {
    holdToCycle: true,
    startAtLogin: false,
    paused: false,
    showMenubarIcon: true,
    vimKeys: false,
    arrowKeys: true,
    menubarIconStyle: "default",
    language: "",
    updatePolicy: "check",
    crashReports: "ask",
    mouseHoverSelect: true,
    cursorFollowFocus: false,
    hapticFeedback: true,
    captureInBackground: false,
    onboarded: false,
    actionBindings: { ...DEFAULT_ACTION_BINDINGS },
    middleClickAction: "close",
    swipeUpAction: "none",
    swipeDownAction: "none",
  },
  appSwitcher: {
    appearance: { ...emptyState.appearance, style: "appIcons", previewSelected: true },
    behavior: {
      holdToCycle: true,
      vimKeys: false,
      arrowKeys: true,
      mouseHoverSelect: true,
      cursorFollowFocus: false,
      hapticFeedback: true,
      actionBindings: { ...DEFAULT_ACTION_BINDINGS },
      middleClickAction: "close",
      swipeUpAction: "none",
      swipeDownAction: "none",
    },
    order: "recent",
    placement: "cursorScreen",
  },
  dock: {
    enabled: false,
    hoverDelayMs: 300,
    dismissDelayMs: 250,
    hoverSlopPx: 8,
    bridgePaddingPx: 12,
    scope: { appScope: "all" },
    appearance: {
      ...emptyState.appearance,
      style: "thumbnails",
      thumbnailMaxPx: 240,
      maxRows: 2,
      maxColumns: 5,
      previewSelected: false,
      showWindowControls: true,
      fadeOutAnimation: false,
    },
  },
};

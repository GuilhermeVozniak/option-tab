// TypeScript mirror of the Go switcher State (internal/switcher/switcher.go).
// The Go layer serializes these as JSON over Wails events.

export type VisualStyle = "thumbnails" | "appIcons" | "titles";
export type Theme = "system" | "light" | "dark";
export type SizePreset = "small" | "medium" | "large";
export type Placement = "activeScreen" | "cursorScreen" | "focusedWindowScreen";
export type TruncationMode = "end" | "middle" | "start";

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
}

export interface Settings {
  version: number;
  shortcuts: Shortcut[];
  appearance: Appearance;
  filters: Filters;
  order: OrderMode;
  placement: Placement;
  behavior: Behavior;
}

export const defaultSettings: Settings = {
  version: 2,
  shortcuts: [
    { id: 1, chord: "command+tab", enabled: true, scope: { appScope: "all" } },
    { id: 2, chord: "option+tab", enabled: true, scope: { appScope: "activeApp" } },
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
  },
};

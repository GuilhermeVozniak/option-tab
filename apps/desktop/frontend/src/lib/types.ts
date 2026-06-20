// TypeScript mirror of the Go switcher State (internal/switcher/switcher.go).
// The Go layer serializes these as JSON over Wails events.

export type VisualStyle = "thumbnails" | "appIcons" | "titles";
export type Theme = "system" | "light" | "dark";
export type Placement = "activeScreen" | "cursorScreen" | "focusedWindowScreen";

export interface Appearance {
  style: VisualStyle;
  theme: Theme;
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
}

export interface Entry {
  windowId: number;
  appId: number;
  title: string;
  appName: string;
  bundleId: string;
  minimized: boolean;
  hidden: boolean;
  fullscreen: boolean;
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
}

export const emptyState: SwitcherState = {
  open: false,
  style: "thumbnails",
  appearance: {
    style: "thumbnails",
    theme: "system",
    maxRows: 4,
    maxColumns: 6,
    thumbnailMaxPx: 256,
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
  },
  placement: "cursorScreen",
  entries: [],
  selected: 0,
  search: "",
  shortcutId: 0,
};

// ---- Settings (mirror of internal/config.Settings) ----

export type OrderMode = "recent" | "alphabetical" | "space";
export type SpaceScope = "active" | "all";
export type ScreenScope = "active" | "all" | "cursor";
export type AppScopeMode = "all" | "activeApp";

export interface ShortcutScope {
  appScope: AppScopeMode;
  spaces?: SpaceScope;
  screens?: ScreenScope;
}

export interface Shortcut {
  id: number;
  chord: string;
  enabled: boolean;
  scope: ShortcutScope;
  styleOverride?: VisualStyle;
}

export interface Filters {
  spaces: SpaceScope;
  screens: ScreenScope;
  showMinimized: boolean;
  showHiddenApps: boolean;
  showFullscreen: boolean;
  showWindowsWithoutTitle: boolean;
  appBlacklist: string[];
}

export interface Behavior {
  holdToCycle: boolean;
  startAtLogin: boolean;
  paused: boolean;
  showMenubarIcon: boolean;
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
  version: 1,
  shortcuts: [
    { id: 1, chord: "option+tab", enabled: true, scope: { appScope: "all" } },
    { id: 2, chord: "option+grave", enabled: true, scope: { appScope: "activeApp" } },
  ],
  appearance: { ...emptyState.appearance },
  filters: {
    spaces: "all",
    screens: "all",
    showMinimized: true,
    showHiddenApps: true,
    showFullscreen: true,
    showWindowsWithoutTitle: false,
    appBlacklist: [],
  },
  order: "recent",
  placement: "cursorScreen",
  behavior: { holdToCycle: true, startAtLogin: false, paused: false, showMenubarIcon: true },
};

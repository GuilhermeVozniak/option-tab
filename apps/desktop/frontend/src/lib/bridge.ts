// Bridge to the Wails runtime: typed accessors for the bound Go App methods and
// a subscription to the switcher events the Go controller emits. Both degrade
// gracefully when Wails is absent (e.g. running the UI in a plain browser during
// development or in tests), so the frontend builds and runs standalone.

import type { Permissions, PermKey, Settings, SwitcherState } from "./types";

interface SwitcherApp {
  Advance(): Promise<void>;
  Reverse(): Promise<void>;
  Confirm(): Promise<void>;
  Cancel(): Promise<void>;
  Select(index: number): Promise<void>;
  SetSearch(query: string): Promise<void>;
  CloseSelected(): Promise<void>;
  MinimizeSelected(): Promise<void>;
  FullscreenSelected(): Promise<void>;
  QuitSelectedApp(): Promise<void>;
  HideSelectedApp(): Promise<void>;
}

interface SettingsApp {
  GetSettings(): Promise<string>;
  SaveSettings(json: string): Promise<void>;
}

interface SystemApp {
  TogglePause(): Promise<void>;
  SetPaused(paused: boolean): Promise<void>;
  OpenPreferences(): Promise<void>;
  ClosePreferences(): Promise<void>;
  GetPermissions(): Promise<string>;
  RequestAccessibility(): Promise<void>;
  RequestScreenRecording(): Promise<void>;
  OpenPermissionSettings(kind: string): Promise<void>;
  GetVersion(): Promise<string>;
  OpenURL(url: string): Promise<void>;
  CheckForUpdates(): Promise<void>;
  GetCrashReport(): Promise<string>;
  DismissCrashReport(): Promise<void>;
  ReportCrash(): Promise<void>;
  CaptureShortcut(): Promise<string>;
  CancelShortcutCapture(): Promise<void>;
}

interface WailsRuntime {
  EventsOn(event: string, cb: (data: unknown) => void): () => void;
}

function boundApp<T>(): Partial<T> {
  const g = globalThis as { go?: { main?: { App?: Partial<T> } } };
  return g.go?.main?.App ?? {};
}

function runtime(): WailsRuntime | undefined {
  return (globalThis as { runtime?: WailsRuntime }).runtime;
}

async function call(
  fn: ((...a: never[]) => Promise<void>) | undefined,
  ...args: never[]
): Promise<void> {
  if (typeof fn === "function") {
    await fn(...args);
  }
}

// switcher exposes the controller actions the overlay invokes. Each method is a
// no-op when its binding is unavailable.
export const switcher = {
  advance: () => call(boundApp<SwitcherApp>().Advance),
  reverse: () => call(boundApp<SwitcherApp>().Reverse),
  confirm: () => call(boundApp<SwitcherApp>().Confirm),
  cancel: () => call(boundApp<SwitcherApp>().Cancel),
  select: (index: number) => call(boundApp<SwitcherApp>().Select as never, index as never),
  setSearch: (query: string) => call(boundApp<SwitcherApp>().SetSearch as never, query as never),
  closeSelected: () => call(boundApp<SwitcherApp>().CloseSelected),
  minimizeSelected: () => call(boundApp<SwitcherApp>().MinimizeSelected),
  fullscreenSelected: () => call(boundApp<SwitcherApp>().FullscreenSelected),
  quitSelectedApp: () => call(boundApp<SwitcherApp>().QuitSelectedApp),
  hideSelectedApp: () => call(boundApp<SwitcherApp>().HideSelectedApp),
};

// system exposes app-level (non-switcher) actions: the menubar pause toggle and
// opening/closing the preferences panel. Each is a no-op without Wails.
export const system = {
  togglePause: () => call(boundApp<SystemApp>().TogglePause),
  setPaused: (paused: boolean) => call(boundApp<SystemApp>().SetPaused as never, paused as never),
  openPreferences: () => call(boundApp<SystemApp>().OpenPreferences),
  closePreferences: () => call(boundApp<SystemApp>().ClosePreferences),
  checkForUpdates: () => call(boundApp<SystemApp>().CheckForUpdates),
  // captureShortcut arms native chord recording (sees Command+Tab and the
  // switcher's own chord, which never reach the DOM). Resolves with the chord,
  // "" on cancel/timeout, or null when the binding is unavailable (browser/
  // dev) so callers can fall back to DOM key events.
  captureShortcut: async (): Promise<string | null> => {
    const fn = boundApp<SystemApp>().CaptureShortcut;
    if (typeof fn !== "function") return null;
    try {
      return await fn();
    } catch {
      return "";
    }
  },
  cancelShortcutCapture: () => call(boundApp<SystemApp>().CancelShortcutCapture),
  // openURL routes through Go so links open in the system browser; without
  // Wails it falls back to window.open (browser/dev).
  openURL: (url: string): Promise<void> => {
    const fn = boundApp<SystemApp>().OpenURL;
    if (typeof fn === "function") return fn(url);
    globalThis.open?.(url, "_blank", "noopener");
    return Promise.resolve();
  },
};

// loadVersion reads the app version for the About tab, or null without Wails.
export async function loadVersion(): Promise<string | null> {
  const fn = boundApp<SystemApp>().GetVersion;
  if (typeof fn !== "function") return null;
  try {
    return await fn();
  } catch {
    return null;
  }
}

export interface SwitcherEventHandlers {
  onShow: (state: SwitcherState) => void;
  onUpdate: (state: SwitcherState) => void;
  onHide: () => void;
  onThumbnails?: (thumbs: Record<string, string>) => void;
  onPreview?: (previews: Record<string, string>) => void;
  onPrefsOpen?: () => void;
  onPrefsClose?: () => void;
  /** Menubar deep-link: open preferences on a specific tab (e.g. "About"). */
  onPrefsTab?: (tab: string) => void;
}

// onSwitcherEvent subscribes to the Go controller's events and returns an
// unsubscribe function. It is a no-op (returning a no-op unsubscribe) when the
// Wails runtime is not present.
export function onSwitcherEvent(handlers: SwitcherEventHandlers): () => void {
  const rt = runtime();
  if (!rt) return () => {};
  const offShow = rt.EventsOn("switcher:show", (data) => handlers.onShow(data as SwitcherState));
  const offUpdate = rt.EventsOn("switcher:update", (data) =>
    handlers.onUpdate(data as SwitcherState),
  );
  const offHide = rt.EventsOn("switcher:hide", () => handlers.onHide());
  const offThumbs = rt.EventsOn("switcher:thumbnails", (data) =>
    handlers.onThumbnails?.(data as Record<string, string>),
  );
  const offPreview = rt.EventsOn("switcher:preview", (data) =>
    handlers.onPreview?.(data as Record<string, string>),
  );
  const offPrefsOpen = rt.EventsOn("prefs:open", () => handlers.onPrefsOpen?.());
  const offPrefsClose = rt.EventsOn("prefs:close", () => handlers.onPrefsClose?.());
  const offPrefsTab = rt.EventsOn("prefs:tab", (data) => handlers.onPrefsTab?.(data as string));
  return () => {
    offShow?.();
    offUpdate?.();
    offHide?.();
    offThumbs?.();
    offPreview?.();
    offPrefsOpen?.();
    offPrefsClose?.();
    offPrefsTab?.();
  };
}

// UpdateInfo describes a newer release found by the background checker.
export interface UpdateInfo {
  version: string;
  url: string;
}

// onUpdateAvailable subscribes to the Go update checker's event. No-op
// unsubscribe without Wails.
export function onUpdateAvailable(cb: (u: UpdateInfo) => void): () => void {
  const rt = runtime();
  if (!rt) return () => {};
  const off = rt.EventsOn("update:available", (data) => cb(data as UpdateInfo));
  return () => off?.();
}

// crashReports exposes the crash-report flow: open a prefilled GitHub issue
// with the pending report, or discard it. Both no-ops without Wails.
export const crashReports = {
  report: () => call(boundApp<SystemApp>().ReportCrash),
  dismiss: () => call(boundApp<SystemApp>().DismissCrashReport),
};

// loadCrashReport returns the previous run's crash log, or null when there is
// none (or Wails is absent).
export async function loadCrashReport(): Promise<string | null> {
  const fn = boundApp<SystemApp>().GetCrashReport;
  if (typeof fn !== "function") return null;
  try {
    const log = await fn();
    return log || null;
  } catch {
    return null;
  }
}

// permissions exposes OS-permission requests. Requesting triggers the system
// prompt; openSettings opens the relevant System Settings pane (for when a prior
// denial means the prompt no longer appears). Each is a no-op without Wails.
export const permissions = {
  requestAccessibility: () => call(boundApp<SystemApp>().RequestAccessibility),
  requestScreenRecording: () => call(boundApp<SystemApp>().RequestScreenRecording),
  request: (kind: PermKey) =>
    kind === "accessibility"
      ? call(boundApp<SystemApp>().RequestAccessibility)
      : call(boundApp<SystemApp>().RequestScreenRecording),
  openSettings: (kind: PermKey) =>
    call(boundApp<SystemApp>().OpenPermissionSettings as never, kind as never),
};

// loadPermissions reads the current OS-permission grant state from Go, or null
// when the binding is unavailable (e.g. browser/dev/tests).
export async function loadPermissions(): Promise<Permissions | null> {
  const fn = boundApp<SystemApp>().GetPermissions;
  if (typeof fn !== "function") return null;
  try {
    return JSON.parse(await fn()) as Permissions;
  } catch {
    return null;
  }
}

// loadSettings reads persisted settings from Go, or null when unavailable.
export async function loadSettings(): Promise<Settings | null> {
  const fn = boundApp<SettingsApp>().GetSettings;
  if (typeof fn !== "function") return null;
  try {
    const s = JSON.parse(await fn()) as Settings;
    // Go marshals a nil slice as null; the settings UI maps over the list.
    if (s?.filters) s.filters.appBlacklist = s.filters.appBlacklist ?? [];
    return s;
  } catch {
    return null;
  }
}

// saveSettings persists settings through Go; a no-op when unavailable.
export async function saveSettings(s: Settings): Promise<void> {
  const fn = boundApp<SettingsApp>().SaveSettings;
  if (typeof fn === "function") await fn(JSON.stringify(s));
}

// Bridge to the Wails runtime: typed accessors for the bound Go App service
// (generated bindings under ../bindings) and a subscription to the switcher
// events the Go controller emits (@wailsio/runtime Events). Everything degrades
// gracefully when no Wails backend answers (e.g. running the UI in a plain
// browser during development or in tests), so the frontend builds and runs
// standalone.

import { Events } from "@wailsio/runtime";
import * as AppService from "../../bindings/option-tab/app.js";
import type { Permissions, PermKey, Settings, SwitcherState } from "./types";

// hasBackend resolves to true when a real Wails backend answers a cheap call.
// Cached: the probe runs once and every consumer shares it. In a plain browser
// the bindings' HTTP transport fails — or, under vite dev/preview, hits the SPA
// fallback and "succeeds" with index.html — so the answer is validated as a
// semver-ish version string. While false, the UI keeps its browser fallbacks
// (DOM key events, window.open, no-op saves).
let backendProbe: Promise<boolean> | null = null;

export function hasBackend(): Promise<boolean> {
  if (!backendProbe) {
    backendProbe = Promise.resolve()
      .then(() => AppService.GetVersion())
      .then(
        (v) => typeof v === "string" && /^\d+\.\d+\.\d+/.test(v),
        () => false,
      );
  }
  return backendProbe;
}

// resetBackendProbeForTests clears the cached probe (vitest: module state
// persists across tests in a file).
export function resetBackendProbeForTests(): void {
  backendProbe = null;
}

// call invokes a bound method, swallowing rejections so a missing backend
// turns every action into a no-op instead of an unhandled promise rejection.
async function call(fn: Promise<unknown>): Promise<void> {
  try {
    await fn;
  } catch {
    // no backend (browser/dev/tests): no-op
  }
}

// switcher exposes the controller actions the overlay invokes.
export const switcher = {
  advance: () => call(AppService.Advance()),
  reverse: () => call(AppService.Reverse()),
  confirm: () => call(AppService.Confirm()),
  cancel: () => call(AppService.Cancel()),
  select: (index: number) => call(AppService.Select(index)),
  setSearch: (query: string) => call(AppService.SetSearch(query)),
  closeSelected: () => call(AppService.CloseSelected()),
  minimizeSelected: () => call(AppService.MinimizeSelected()),
  fullscreenSelected: () => call(AppService.FullscreenSelected()),
  quitSelectedApp: () => call(AppService.QuitSelectedApp()),
  hideSelectedApp: () => call(AppService.HideSelectedApp()),
};

// system exposes app-level (non-switcher) actions: the menubar pause toggle and
// opening/closing the preferences window.
export const system = {
  togglePause: () => call(AppService.TogglePause()),
  setPaused: (paused: boolean) => call(AppService.SetPaused(paused)),
  openPreferences: () => call(AppService.OpenPreferences()),
  closePreferences: () => call(AppService.ClosePreferences()),
  checkForUpdates: () => call(AppService.CheckForUpdates()),
  // captureShortcut arms native chord recording (sees Command+Tab and the
  // switcher's own chord, which never reach the DOM). Resolves with the chord,
  // "" on cancel/timeout, or null when there is no backend (browser/dev) so
  // callers can fall back to DOM key events.
  captureShortcut: async (): Promise<string | null> => {
    if (!(await hasBackend())) return null;
    try {
      return await AppService.CaptureShortcut();
    } catch {
      return "";
    }
  },
  cancelShortcutCapture: () => call(AppService.CancelShortcutCapture()),
  // openURL routes through Go so links open in the system browser; without a
  // backend it falls back to window.open (browser/dev).
  openURL: async (url: string): Promise<void> => {
    if (!(await hasBackend())) {
      globalThis.open?.(url, "_blank", "noopener");
      return;
    }
    await call(AppService.OpenURL(url));
  },
};

// loadVersion reads the app version for the About tab, or null without Wails
// (or when a dev-server SPA fallback answers with HTML instead of a version).
export async function loadVersion(): Promise<string | null> {
  try {
    const v = await AppService.GetVersion();
    return typeof v === "string" && /^\d+\.\d+\.\d+/.test(v) ? v : null;
  } catch {
    return null;
  }
}

// KeyPayload mirrors platform.KeyEvent: a raw key press forwarded by the native
// event tap while the overlay is open (the overlay never becomes key, so this
// is its only keyboard source). Fields match the DOM KeyboardEvent shape the
// keymap consumes.
export interface KeyPayload {
  key: string;
  code: string;
  shift: boolean;
  ctrl: boolean;
  alt: boolean;
  meta: boolean;
}

export interface SwitcherEventHandlers {
  onShow: (state: SwitcherState) => void;
  onUpdate: (state: SwitcherState) => void;
  onHide: () => void;
  onThumbnails?: (thumbs: Record<string, string>) => void;
  onPreview?: (previews: Record<string, string>) => void;
}

// onSwitcherEvent subscribes to the Go controller's events and returns an
// unsubscribe function. Without a backend the subscriptions simply never fire.
export function onSwitcherEvent(handlers: SwitcherEventHandlers): () => void {
  const offShow = Events.On("switcher:show", (ev) => handlers.onShow(ev.data as SwitcherState));
  const offUpdate = Events.On("switcher:update", (ev) =>
    handlers.onUpdate(ev.data as SwitcherState),
  );
  const offHide = Events.On("switcher:hide", () => handlers.onHide());
  const offThumbs = Events.On("switcher:thumbnails", (ev) =>
    handlers.onThumbnails?.(ev.data as Record<string, string>),
  );
  const offPreview = Events.On("switcher:preview", (ev) =>
    handlers.onPreview?.(ev.data as Record<string, string>),
  );
  return () => {
    offShow();
    offUpdate();
    offHide();
    offThumbs();
    offPreview();
  };
}

// onSwitcherKey subscribes to the raw key presses the native event tap captures
// while the overlay is open. Without a backend it never fires (browser dev uses
// the DOM keydown fallback instead).
export function onSwitcherKey(cb: (key: KeyPayload) => void): () => void {
  return Events.On("switcher:key", (ev) => cb(ev.data as KeyPayload));
}

// onPrefsTab subscribes to the menubar deep-link that opens preferences on a
// specific tab (e.g. "About"). Only the preferences window acts on it.
export function onPrefsTab(cb: (tab: string) => void): () => void {
  return Events.On("prefs:tab", (ev) => cb(ev.data as string));
}

// UpdateInfo describes a newer release found by the background checker.
export interface UpdateInfo {
  version: string;
  url: string;
}

// onUpdateAvailable subscribes to the Go update checker's event.
export function onUpdateAvailable(cb: (u: UpdateInfo) => void): () => void {
  return Events.On("update:available", (ev) => cb(ev.data as UpdateInfo));
}

// crashReports exposes the crash-report flow: open a prefilled GitHub issue
// with the pending report, or discard it.
export const crashReports = {
  report: () => call(AppService.ReportCrash()),
  dismiss: () => call(AppService.DismissCrashReport()),
};

// loadCrashReport returns the previous run's crash log, or null when there is
// none (or Wails is absent).
export async function loadCrashReport(): Promise<string | null> {
  try {
    const log = await AppService.GetCrashReport();
    return log || null;
  } catch {
    return null;
  }
}

// permissions exposes OS-permission requests. Requesting triggers the system
// prompt; openSettings opens the relevant System Settings pane (for when a prior
// denial means the prompt no longer appears).
export const permissions = {
  requestAccessibility: () => call(AppService.RequestAccessibility()),
  requestScreenRecording: () => call(AppService.RequestScreenRecording()),
  request: (kind: PermKey) =>
    kind === "accessibility"
      ? call(AppService.RequestAccessibility())
      : call(AppService.RequestScreenRecording()),
  openSettings: (kind: PermKey) => call(AppService.OpenPermissionSettings(kind)),
};

// loadPermissions reads the current OS-permission grant state from Go, or null
// when the backend is unavailable (e.g. browser/dev/tests).
export async function loadPermissions(): Promise<Permissions | null> {
  try {
    return JSON.parse(await AppService.GetPermissions()) as Permissions;
  } catch {
    return null;
  }
}

// loadSettings reads persisted settings from Go, or null when unavailable.
export async function loadSettings(): Promise<Settings | null> {
  try {
    const s = JSON.parse(await AppService.GetSettings()) as Settings;
    // Go marshals a nil slice as null; the settings UI maps over the list.
    if (s?.filters) s.filters.appBlacklist = s.filters.appBlacklist ?? [];
    return s;
  } catch {
    return null;
  }
}

// saveSettings persists settings through Go; a no-op when unavailable.
export async function saveSettings(s: Settings): Promise<void> {
  await call(AppService.SaveSettings(JSON.stringify(s)));
}

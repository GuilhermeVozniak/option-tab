// Bridge to the Wails runtime: typed accessors for the bound Go App methods and
// a subscription to the switcher events the Go controller emits. Both degrade
// gracefully when Wails is absent (e.g. running the UI in a plain browser during
// development or in tests), so the frontend builds and runs standalone.

import type { Settings, SwitcherState } from "./types";

interface SwitcherApp {
  Advance(): Promise<void>;
  Reverse(): Promise<void>;
  Confirm(): Promise<void>;
  Cancel(): Promise<void>;
  Select(index: number): Promise<void>;
  SetSearch(query: string): Promise<void>;
  CloseSelected(): Promise<void>;
  MinimizeSelected(): Promise<void>;
  QuitSelectedApp(): Promise<void>;
  HideSelectedApp(): Promise<void>;
}

interface SettingsApp {
  GetSettings(): Promise<string>;
  SaveSettings(json: string): Promise<void>;
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
  quitSelectedApp: () => call(boundApp<SwitcherApp>().QuitSelectedApp),
  hideSelectedApp: () => call(boundApp<SwitcherApp>().HideSelectedApp),
};

export interface SwitcherEventHandlers {
  onShow: (state: SwitcherState) => void;
  onUpdate: (state: SwitcherState) => void;
  onHide: () => void;
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
  return () => {
    offShow?.();
    offUpdate?.();
    offHide?.();
  };
}

// loadSettings reads persisted settings from Go, or null when unavailable.
export async function loadSettings(): Promise<Settings | null> {
  const fn = boundApp<SettingsApp>().GetSettings;
  if (typeof fn !== "function") return null;
  try {
    return JSON.parse(await fn()) as Settings;
  } catch {
    return null;
  }
}

// saveSettings persists settings through Go; a no-op when unavailable.
export async function saveSettings(s: Settings): Promise<void> {
  const fn = boundApp<SettingsApp>().SaveSettings;
  if (typeof fn === "function") await fn(JSON.stringify(s));
}

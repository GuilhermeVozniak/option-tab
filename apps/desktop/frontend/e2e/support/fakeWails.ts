import type { Page } from "@playwright/test";

// A complete Appearance matching emptyState defaults, tuned for deterministic
// e2e: dark theme, no apparition delay, animations off so nothing races.
const APPEARANCE = {
  style: "thumbnails",
  theme: "dark",
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
  fadeOutAnimation: false,
  showStatusIcons: true,
  showSpaceNumbers: true,
  titleTruncation: "end",
  previewSelected: false,
  previewFade: false,
};

// Four synthetic windows: two on Space 1, one minimized, one on Space 2 (so the
// status icons, Space badges and other-Space marker all have something to show).
const ENTRIES = [
  {
    windowId: 1,
    appId: 1,
    appName: "Editor",
    bundleId: "com.ex.editor",
    title: "main.go",
    spaceId: 1,
    minimized: false,
    hidden: false,
    fullscreen: false,
  },
  {
    windowId: 2,
    appId: 2,
    appName: "Browser",
    bundleId: "com.ex.browser",
    title: "GitHub — option-tab",
    spaceId: 1,
    minimized: false,
    hidden: false,
    fullscreen: false,
  },
  {
    windowId: 3,
    appId: 3,
    appName: "Terminal",
    bundleId: "com.ex.term",
    title: "zsh",
    spaceId: 1,
    minimized: true,
    hidden: false,
    fullscreen: false,
  },
  {
    windowId: 4,
    appId: 4,
    appName: "Notes",
    bundleId: "com.ex.notes",
    title: "Parity",
    spaceId: 2,
    minimized: false,
    hidden: false,
    fullscreen: false,
  },
];

export type ShowState = Record<string, unknown>;

// showState builds a full SwitcherState payload for a switcher:show event.
export function showState(overrides: ShowState = {}): ShowState {
  return {
    open: true,
    style: "thumbnails",
    appearance: { ...APPEARANCE },
    placement: "cursorScreen",
    entries: ENTRIES.map((e) => ({ ...e })),
    selected: 0,
    search: "",
    shortcutId: 1,
    vimKeys: false,
    arrowKeys: true,
    mouseHover: true,
    activeSpaceId: 1,
    ...overrides,
  };
}

// installFakeWails injects a fake Wails runtime + bound App before the app
// script runs, so the real event-driven OverlayRoute is fully interactive in a
// plain browser. Navigation methods mutate a shared state and re-emit
// switcher:update (a NEW object each time so React re-renders); window/app
// action methods are recorded on window.__calls for assertions.
export async function installFakeWails(page: Page): Promise<void> {
  await page.addInitScript(() => {
    const w = window as any;
    w.__handlers = {};
    w.__calls = [];
    w.__state = null;
    w.runtime = {
      EventsOn: (event: string, cb: (d: unknown) => void) => {
        (w.__handlers[event] = w.__handlers[event] || []).push(cb);
        return () => {
          w.__handlers[event] = (w.__handlers[event] || []).filter((h: unknown) => h !== cb);
        };
      },
      EventsOff: () => {},
      EventsEmit: () => {},
    };
    w.__emit = (event: string, data: unknown) => {
      for (const h of w.__handlers[event] || []) h(data);
    };
    const clamp = (i: number, n: number) => ((i % n) + n) % n;
    const patch = (p: Record<string, unknown>) => {
      w.__state = { ...w.__state, ...p };
      w.__emit("switcher:update", w.__state);
    };
    const rec =
      (name: string) =>
      (...args: unknown[]) => {
        w.__calls.push([name, ...args]);
        return Promise.resolve();
      };
    w.go = {
      main: {
        App: {
          Advance: () => {
            patch({ selected: clamp(w.__state.selected + 1, w.__state.entries.length) });
            return Promise.resolve();
          },
          Reverse: () => {
            patch({ selected: clamp(w.__state.selected - 1, w.__state.entries.length) });
            return Promise.resolve();
          },
          Select: (i: number) => {
            patch({ selected: i });
            return Promise.resolve();
          },
          SetSearch: (q: string) => {
            patch({ search: q });
            return Promise.resolve();
          },
          Confirm: () => {
            w.__calls.push(["Confirm"]);
            w.__emit("switcher:hide", null);
            return Promise.resolve();
          },
          Cancel: () => {
            w.__calls.push(["Cancel"]);
            w.__emit("switcher:hide", null);
            return Promise.resolve();
          },
          CloseSelected: rec("CloseSelected"),
          MinimizeSelected: rec("MinimizeSelected"),
          FullscreenSelected: rec("FullscreenSelected"),
          QuitSelectedApp: rec("QuitSelectedApp"),
          HideSelectedApp: rec("HideSelectedApp"),
        },
      },
    };
  });
}

// emitShow waits until the app has subscribed to switcher events, then pushes a
// switcher:show with the given state (also seeding the controller-sim state).
export async function emitShow(page: Page, state: ShowState): Promise<void> {
  await page.waitForFunction(() => {
    const w = window as any;
    return (w.__handlers?.["switcher:show"]?.length ?? 0) > 0;
  });
  await page.evaluate((s) => {
    const w = window as any;
    w.__state = s;
    w.__emit("switcher:show", s);
  }, state);
}

// getCalls returns the recorded bound-method names, in order.
export async function getCalls(page: Page): Promise<string[]> {
  return page.evaluate(() => ((window as any).__calls || []).map((c: unknown[]) => c[0] as string));
}

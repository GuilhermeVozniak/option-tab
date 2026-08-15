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

// Bound-method ids from the generated bindings (bindings/option-tab/app.js).
// They are deterministic hashes of the Go service method names, so they only
// change when a method is renamed — regenerate mentally via that file.
const METHOD = {
  Advance: 3974603045,
  Reverse: 2687580445,
  Select: 149583269,
  SetSearch: 1454005219,
  Confirm: 3228319335,
  Cancel: 2191755235,
  CloseSelected: 2868361114,
  MinimizeSelected: 1240045732,
  FullscreenSelected: 529416953,
  QuitSelectedApp: 3876391122,
  HideSelectedApp: 3942268823,
  GetVersion: 1049863377,
  InstallUpdate: 2443992793,
} as const;

const METHOD_NAME = new Map<number, string>(Object.entries(METHOD).map(([name, id]) => [id, name]));

// installFakeWails emulates the Wails v3 backend in a plain browser:
//
//   - RPC: the generated bindings POST {object, method, args} to
//     /wails/runtime; a route handler answers navigation/action calls, driving
//     a shared fake controller state and re-emitting switcher:update.
//   - Events: page-side dispatch goes through window._wails.dispatchWailsEvent,
//     which the real @wailsio/runtime installs on import.
//   - Keyboard: the production overlay takes keys from native-tap "switcher:key"
//     events, so a DOM keydown listener re-emits them in that exact shape (the
//     tap only forwards while the switcher is open, hence the __state.open gate).
//
// Window/app action calls are recorded on window.__calls for assertions.
export async function installFakeWails(page: Page): Promise<void> {
  await page.addInitScript(() => {
    const w = window as unknown as {
      __calls: unknown[][];
      __state: Record<string, unknown> | null;
      _wails?: { dispatchWailsEvent?: (ev: { name: string; data: unknown }) => void };
    };
    w.__calls = [];
    w.__state = null;
    window.addEventListener("keydown", (e) => {
      if (!w.__state?.open) return; // the native tap only forwards while open
      w._wails?.dispatchWailsEvent?.({
        name: "switcher:key",
        data: {
          key: e.key,
          code: e.code,
          shift: e.shiftKey,
          ctrl: e.ctrlKey,
          alt: e.altKey,
          meta: e.metaKey,
        },
      });
    });
  });

  await page.route("**/wails/runtime", async (route) => {
    // Binding calls arrive as {object: CallBinding, method: 0, args: {"call-id",
    // methodID, args: [...]}} — the numeric methodID selects the Go method.
    const body = route.request().postDataJSON() as {
      args?: { methodID?: number; args?: unknown[] };
    };
    const name = METHOD_NAME.get(body.args?.methodID ?? 0);
    const args = body.args?.args ?? [];
    const json = (result: unknown) =>
      route.fulfill({ contentType: "application/json", body: JSON.stringify(result) });

    const evaluate = (fn: (arg: unknown) => void, arg?: unknown) => page.evaluate(fn, arg);

    switch (name) {
      case "GetVersion":
        return json("0.0.0-e2e");
      case "InstallUpdate":
        // No real install in e2e: acknowledge and stay put.
        await evaluate((n) => {
          (window as any).__calls.push([n]);
        }, name);
        return json(null);
      case "Advance":
      case "Reverse":
      case "Select":
      case "SetSearch": {
        await evaluate(
          ([n, a]) => {
            const w = window as any;
            const st = w.__state;
            if (!st) return;
            const clamp = (i: number, len: number) => ((i % len) + len) % len;
            if (n === "Advance") st.selected = clamp(st.selected + 1, st.entries.length);
            else if (n === "Reverse") st.selected = clamp(st.selected - 1, st.entries.length);
            else if (n === "Select") st.selected = a;
            else st.search = a;
            w.__state = { ...st };
            w._wails?.dispatchWailsEvent?.({ name: "switcher:update", data: w.__state });
          },
          [name, args[0]],
        );
        return json(null);
      }
      case "Confirm":
      case "Cancel": {
        await evaluate((n) => {
          const w = window as any;
          w.__calls.push([n]);
          w.__state = { ...w.__state, open: false };
          w._wails?.dispatchWailsEvent?.({ name: "switcher:hide", data: null });
        }, name);
        return json(null);
      }
      case "CloseSelected":
      case "MinimizeSelected":
      case "FullscreenSelected":
      case "QuitSelectedApp":
      case "HideSelectedApp": {
        await evaluate((n) => {
          (window as any).__calls.push([n]);
        }, name);
        return json(null);
      }
      default:
        // Anything unmapped behaves as "no backend": the bridge degrades to its
        // browser defaults (empty settings, no permissions UI, ...).
        return route.fulfill({ status: 500, body: "unbound method" });
    }
  });
}

// emitShow pushes a switcher:show with the given state (also seeding the fake
// controller state). The runtime's listener registry is module-private, so a
// lost race shows up as the overlay never appearing — retry a few times.
export async function emitShow(page: Page, state: ShowState): Promise<void> {
  await page.waitForFunction(() => {
    const w = window as any;
    return typeof w._wails?.dispatchWailsEvent === "function";
  });
  for (let attempt = 0; attempt < 10; attempt++) {
    await page.evaluate((s) => {
      const w = window as any;
      w.__state = s;
      w._wails.dispatchWailsEvent({ name: "switcher:show", data: s });
    }, state);
    try {
      await page.locator(".ot-overlay").waitFor({ state: "visible", timeout: 500 });
      return;
    } catch {
      // subscription not ready yet; dispatch again
    }
  }
  throw new Error("switcher:show never rendered the overlay");
}

// getCalls returns the recorded bound-method names, in order.
export async function getCalls(page: Page): Promise<string[]> {
  return page.evaluate(() => ((window as any).__calls || []).map((c: unknown[]) => c[0] as string));
}

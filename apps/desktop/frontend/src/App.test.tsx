import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

// Mock the Wails v3 seams: the generated App service bindings and the
// @wailsio/runtime event bus, with a handler registry so tests can fire Go-side
// events (switcher:show, prefs:tab, ...) exactly like the runtime does.
const eventHandlers = new Map<string, (ev: { data: unknown }) => void>();

vi.mock("@wailsio/runtime", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@wailsio/runtime")>();
  return {
    ...actual,
    Events: {
      On: vi.fn((name: string, cb: (ev: { data: unknown }) => void) => {
        eventHandlers.set(name, cb);
        return () => eventHandlers.delete(name);
      }),
    },
  };
});

vi.mock("../bindings/option-tab/app.js", () => ({
  PerformAction: vi.fn().mockResolvedValue({ succeeded: 1, failures: [] }),
  Advance: vi.fn().mockResolvedValue(undefined),
  Reverse: vi.fn().mockResolvedValue(undefined),
  Confirm: vi.fn().mockResolvedValue(undefined),
  ConfirmWindow: vi.fn().mockResolvedValue(undefined),
  SelectApp: vi.fn().mockResolvedValue(undefined),
  SelectAppWindow: vi.fn().mockResolvedValue(undefined),
  ConfirmApp: vi.fn().mockResolvedValue(undefined),
  GetDockState: vi.fn().mockResolvedValue(null),
  SelectDockWindow: vi.fn().mockResolvedValue(undefined),
  FocusDockWindow: vi.fn().mockResolvedValue({ succeeded: 1, failures: [] }),
  PerformDockAction: vi.fn().mockResolvedValue({ succeeded: 1, failures: [] }),
  SetDockPanelSize: vi.fn().mockResolvedValue(undefined),
  SetDockPreviewRegions: vi.fn().mockResolvedValue(undefined),
  BeginDockPreviewDrag: vi.fn().mockResolvedValue(undefined),
  CancelDockPreviewDrag: vi.fn().mockResolvedValue(undefined),
  SetDockFolderSort: vi.fn().mockResolvedValue(undefined),
  RequestDockFolderAccess: vi.fn().mockResolvedValue(undefined),
  CancelDockFolderAccess: vi.fn().mockResolvedValue(undefined),
  OpenDockFolderEntry: vi.fn().mockResolvedValue(undefined),
  Cancel: vi.fn().mockResolvedValue(undefined),
  Select: vi.fn().mockResolvedValue(undefined),
  SetSearch: vi.fn().mockResolvedValue(undefined),
  CloseSelected: vi.fn().mockResolvedValue(undefined),
  MinimizeSelected: vi.fn().mockResolvedValue(undefined),
  FullscreenSelected: vi.fn().mockResolvedValue(undefined),
  QuitSelectedApp: vi.fn().mockResolvedValue(undefined),
  HideSelectedApp: vi.fn().mockResolvedValue(undefined),
  GetSettings: vi.fn().mockResolvedValue("{}"),
  GetPermissions: vi.fn().mockResolvedValue("{}"),
  GetVersion: vi.fn().mockResolvedValue("1.2.3"),
  InstallUpdate: vi.fn().mockResolvedValue(undefined),
  GetCrashReport: vi.fn().mockResolvedValue(""),
}));

import * as AppService from "../bindings/option-tab/app.js";
import App from "./App";
import { resetBackendProbeForTests } from "./lib/bridge";
import type { Entry, SwitcherState } from "./lib/types";
import { emptyState } from "./lib/types";

const mocked = vi.mocked(AppService);

function appEntry(windowId: number, title: string): Entry {
  return {
    windowId,
    appId: windowId,
    title,
    appName: title,
    bundleId: "",
    minimized: false,
    hidden: false,
    fullscreen: false,
  };
}

function openSwitcherState(overrides: Partial<SwitcherState> = {}): SwitcherState {
  return { ...emptyState, open: true, selected: 0, ...overrides };
}

beforeEach(() => {
  eventHandlers.clear();
  resetBackendProbeForTests();
  window.location.hash = "";
});

describe("App", () => {
  it("renders the overlay route (closed) by default", () => {
    const { container } = render(<App />);
    // Overlay renders nothing while closed.
    expect(container.firstChild).toBeNull();
  });

  it("renders the settings route at #settings", () => {
    window.location.hash = "#settings";
    render(<App />);
    expect(screen.getByText(/Preferences/)).toBeInTheDocument();
  });

  it("renders app mode and confirms an explicit app/window target", async () => {
    render(<App />);
    act(() =>
      eventHandlers.get("switcher:show")?.({
        data: openSwitcherState({
          mode: "apps",
          apps: [
            {
              appId: 10,
              appName: "Editor",
              bundleId: "a",
              hidden: false,
              windowCount: 1,
              windowPresence: "present",
            },
          ],
          entries: [{ ...appEntry(101, "Document"), appId: 10, bundleId: "a" }],
          selectedWindowId: 101,
        }),
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Focus Document" }));
    await waitFor(() => expect((AppService as any).SelectAppWindow).toHaveBeenCalledWith(101));
    expect(mocked.ConfirmWindow).toHaveBeenCalledWith(101);
  });

  it("admits only monotonic switcher state, current frames, and rejects closed-session resurrection", () => {
    render(<App />);
    const state = (session: number, revision: number, title: string) =>
      openSwitcherState({
        session,
        revision,
        entries: [appEntry(101, title)],
      });
    act(() => {
      eventHandlers.get("switcher:update")?.({ data: state(7, 3, "Newest") });
      eventHandlers.get("switcher:show")?.({ data: state(7, 2, "Old show") });
      eventHandlers.get("switcher:update")?.({ data: state(6, 20, "Old session") });
      eventHandlers.get("switcher:thumbnails")?.({
        data: { session: 6, frames: { "101": "data:old" } },
      });
      eventHandlers.get("switcher:thumbnails")?.({
        data: { session: 7, frames: { "101": "data:current" } },
      });
    });
    expect(screen.getByText("Newest")).toBeInTheDocument();
    expect(document.querySelector(".ot-thumb-img")).toHaveAttribute("src", "data:current");
    act(() => {
      eventHandlers.get("switcher:hide")?.({ data: { session: 6, revision: 21 } });
    });
    expect(screen.getByText("Newest")).toBeInTheDocument();
    act(() => {
      eventHandlers.get("switcher:hide")?.({ data: { session: 7, revision: 4 } });
      eventHandlers.get("switcher:update")?.({ data: state(7, 5, "Late update") });
      eventHandlers.get("switcher:show")?.({ data: state(7, 6, "Late show") });
      eventHandlers.get("switcher:preview")?.({
        data: { session: 7, frames: { "101": "data:late" } },
      });
    });
    expect(screen.queryByText(/Late/)).toBeNull();
    expect(screen.getByRole("dialog")).toHaveClass("ot-closing");
  });

  it("rejects unscoped legacy frames and hides after a real switcher session", () => {
    render(<App />);
    act(() =>
      eventHandlers.get("switcher:show")?.({
        data: openSwitcherState({ session: 8, revision: 1, entries: [appEntry(102, "Scoped")] }),
      }),
    );
    act(() => {
      eventHandlers.get("switcher:thumbnails")?.({ data: { "102": "data:legacy" } });
      eventHandlers.get("switcher:hide")?.({ data: null });
    });
    expect(screen.getByText("Scoped")).toBeInTheDocument();
    expect(screen.queryByRole("img")).toBeNull();
  });

  it("treats a real hide as scoped before any show arrives", () => {
    render(<App />);
    act(() => {
      eventHandlers.get("switcher:hide")?.({ data: { session: 11, revision: 2 } });
      eventHandlers.get("switcher:show")?.({
        data: openSwitcherState({ entries: [appEntry(1, "Legacy resurrection")] }),
      });
      eventHandlers.get("switcher:thumbnails")?.({ data: { "1": "data:legacy" } });
    });
    expect(screen.queryByText("Legacy resurrection")).toBeNull();
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("does not let an old native key control a newer app presentation", () => {
    mocked.Advance.mockClear();
    render(<App />);
    act(() =>
      eventHandlers.get("switcher:show")?.({
        data: openSwitcherState({
          session: 15,
          revision: 1,
          mode: "apps",
          apps: [{ appId: 10, appName: "App", bundleId: "a", hidden: false, windowCount: 0 }],
          entries: [],
          selectedWindowId: 0,
        }),
      }),
    );
    act(() => {
      eventHandlers.get("switcher:key")?.({
        data: {
          session: 14,
          key: "Tab",
          code: "Tab",
          shift: false,
          ctrl: false,
          alt: false,
          meta: false,
        },
      });
      eventHandlers.get("switcher:key")?.({
        data: {
          session: 15,
          key: "Tab",
          code: "Tab",
          shift: false,
          ctrl: false,
          alt: false,
          meta: false,
        },
      });
    });
    expect(mocked.Advance).toHaveBeenCalledTimes(1);
  });

  it("rejects stale Dock snapshots, frames, hides, and errors", async () => {
    let resolveSnapshot: (value: never) => void = () => {};
    (AppService as any).GetDockState.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveSnapshot = resolve;
      }) as never,
    );
    window.location.hash = "#dock";
    const appearance = { ...emptyState.appearance, showWindowControls: true };
    render(<App />);
    const dockState = (session: number, title: string, revision = 0) => ({
      session,
      revision,
      item: {
        appId: 10,
        bundleId: "a",
        path: "/A.app",
        title: "A",
        bounds: { x: 0, y: 0, w: 40, h: 40 },
        screenId: 1,
        edge: "bottom",
        kind: "app",
      },
      entries: [{ ...appEntry(102, title), appId: 10 }],
      selectedWindowId: 102,
      appearance,
      emptyReason: "",
    });
    act(() => eventHandlers.get("dock:show")?.({ data: dockState(5, "Current") }));
    expect(screen.getByRole("button", { name: "Focus Current" })).toBeInTheDocument();
    await act(async () => resolveSnapshot(dockState(3, "Old") as never));
    act(() => {
      eventHandlers.get("dock:frames")?.({ data: { session: 3, frames: { "102": "old-frame" } } });
      eventHandlers.get("dock:error")?.({ data: { session: 3, message: "old error" } });
      eventHandlers.get("dock:hide")?.({ data: { session: 3 } });
    });
    expect(screen.getByRole("button", { name: "Focus Current" })).toBeInTheDocument();
    expect(screen.queryByText("old error")).toBeNull();
    act(() => {
      eventHandlers.get("dock:hide")?.({ data: { session: 5 } });
      eventHandlers.get("dock:update")?.({ data: dockState(5, "Retired") });
    });
    expect(screen.queryByRole("button", { name: "Focus Retired" })).toBeNull();
    act(() => {
      eventHandlers.get("dock:show")?.({ data: dockState(5, "Late show", 4) });
      eventHandlers.get("dock:update")?.({ data: dockState(5, "Late update", 4) });
    });
    expect(screen.queryByText(/Late/)).toBeNull();
  });

  it("rejects out-of-order revisions within the active Dock session", () => {
    window.location.hash = "#dock";
    render(<App />);
    const base = {
      session: 8,
      item: {
        appId: 10,
        bundleId: "a",
        path: "/A.app",
        title: "New",
        bounds: { x: 0, y: 0, w: 40, h: 40 },
        screenId: 1,
        edge: "bottom",
        kind: "app",
      },
      entries: [],
      selectedWindowId: 0,
      appearance: emptyState.appearance,
      emptyReason: "unavailable",
    };
    act(() => {
      eventHandlers.get("dock:show")?.({ data: { ...base, revision: 2 } });
      eventHandlers.get("dock:update")?.({
        data: { ...base, revision: 1, item: { ...base.item, title: "Old" } },
      });
    });
    expect(screen.getByText("New")).toBeInTheDocument();
    expect(screen.queryByText("Old")).toBeNull();
  });

  it("renders folder content with outer scope and no window-region traffic", async () => {
    mocked.SetDockPreviewRegions.mockClear();
    window.location.hash = "#dock";
    render(<App />);
    act(() =>
      eventHandlers.get("dock:show")?.({
        data: {
          session: 21,
          revision: 8,
          open: true,
          contentKind: "folder",
          item: {
            appId: 0,
            bundleId: "",
            path: "/tmp/Folder",
            title: "Folder",
            bounds: { x: 0, y: 0, w: 48, h: 48 },
            screenId: 1,
            edge: "bottom",
            kind: "folder",
          },
          entries: [],
          selectedWindowId: 0,
          appearance: emptyState.appearance,
          emptyReason: "",
          folder: {
            status: "ready",
            reason: "",
            folderIdentity: "file:///tmp/Folder",
            entries: [
              {
                id: "opaque-7",
                name: "Notes.txt",
                kind: "file",
                size: 12,
                modifiedAtMs: 1,
                hidden: false,
              },
            ],
            sort: { field: "name", direction: "asc", foldersFirst: true },
            partial: false,
            revision: 99,
          },
        },
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Open Notes.txt" }));
    fireEvent.change(screen.getByLabelText("Sort folder contents by"), {
      target: { value: "size" },
    });
    await waitFor(() =>
      expect((AppService as any).OpenDockFolderEntry).toHaveBeenCalledWith(21, 8, "opaque-7"),
    );
    expect((AppService as any).SetDockFolderSort).toHaveBeenCalledWith(21, 8, "size", "asc", true);
    expect(mocked.SetDockPreviewRegions).not.toHaveBeenCalled();
    expect(screen.queryByLabelText("Close window")).toBeNull();
  });

  it("uses an admitted Dock error revision for the next folder retry", async () => {
    window.location.hash = "#dock";
    render(<App />);
    act(() =>
      eventHandlers.get("dock:show")?.({
        data: {
          session: 22,
          revision: 1,
          open: true,
          contentKind: "folder",
          item: {
            appId: 0,
            bundleId: "",
            path: "/tmp/Folder",
            title: "Folder",
            bounds: { x: 0, y: 0, w: 48, h: 48 },
            screenId: 1,
            edge: "bottom",
            kind: "folder",
          },
          entries: [],
          selectedWindowId: 0,
          appearance: emptyState.appearance,
          emptyReason: "",
          folder: {
            status: "unavailable",
            reason: "",
            folderIdentity: "file:///tmp/Folder",
            entries: [],
            sort: { field: "name", direction: "asc", foldersFirst: true },
            partial: false,
            revision: 1,
          },
        },
      }),
    );
    act(() =>
      eventHandlers.get("dock:error")?.({
        data: { session: 22, revision: 2, message: "Temporary failure" },
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() =>
      expect((AppService as any).SetDockFolderSort).toHaveBeenCalledWith(
        22,
        2,
        "name",
        "asc",
        true,
      ),
    );
  });

  it("admits only current-session monotonic Dock pointer packets and clears outside", () => {
    window.location.hash = "#dock";
    render(<App />);
    const base = {
      session: 18,
      revision: 1,
      item: {
        appId: 10,
        bundleId: "a",
        path: "/A.app",
        title: "A",
        bounds: { x: 0, y: 0, w: 40, h: 40 },
        screenId: 1,
        edge: "bottom",
        kind: "app",
      },
      entries: [
        { ...appEntry(102, "First"), appId: 10 },
        { ...appEntry(103, "Second"), appId: 10 },
      ],
      selectedWindowId: 102,
      appearance: emptyState.appearance,
      emptyReason: "",
    };
    let hit: Element | null = null;
    Object.defineProperty(document, "elementFromPoint", {
      configurable: true,
      value: () => hit ?? document.querySelector("article[data-window-id='103']"),
    });
    act(() => {
      eventHandlers.get("dock:pointer")?.({
        data: { session: 17, sequence: 99, x: 1, y: 1, inside: true },
      });
      eventHandlers.get("dock:pointer")?.({
        data: { session: 18, sequence: 2, x: 10, y: 10, inside: true },
      });
    });
    act(() => eventHandlers.get("dock:show")?.({ data: base }));
    const first = document.querySelector("article[data-window-id='102']")!;
    const second = document.querySelector("article[data-window-id='103']")!;
    expect(second).toHaveClass("is-hovered");
    expect(AppService.SelectDockWindow).toHaveBeenCalledWith(18, 103);
    hit = first;
    act(() => {
      eventHandlers.get("dock:pointer")?.({
        data: { session: 18, sequence: 1, x: 1, y: 1, inside: true },
      });
      eventHandlers.get("dock:pointer")?.({
        data: { session: 17, sequence: 3, x: 1, y: 1, inside: true },
      });
      eventHandlers.get("dock:update")?.({
        data: {
          ...base,
          revision: 2,
          pointer: { session: 18, sequence: 1, x: 1, y: 1, inside: true },
        },
      });
    });
    expect(second).toHaveClass("is-hovered");
    expect(first).not.toHaveClass("is-hovered");
    hit = null;
    act(() =>
      eventHandlers.get("dock:pointer")?.({
        data: { session: 18, sequence: 3, x: -1, y: -1, inside: false },
      }),
    );
    expect(second).not.toHaveClass("is-hovered");
    expect(AppService.FocusDockWindow).not.toHaveBeenCalled();
    expect(AppService.PerformDockAction).not.toHaveBeenCalled();
    Reflect.deleteProperty(document, "elementFromPoint");
  });

  it("publishes visible Dock preview regions through the current session bridge", async () => {
    window.location.hash = "#dock";
    const rect = {
      x: 10,
      y: 10,
      left: 10,
      top: 10,
      right: 110,
      bottom: 70,
      width: 100,
      height: 60,
      toJSON: () => ({}),
    } as DOMRect;
    const bounds = vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue(rect);
    (AppService as any).SetDockPreviewRegions.mockClear();
    render(<App />);
    act(() =>
      eventHandlers.get("dock:show")?.({
        data: {
          session: 21,
          revision: 1,
          open: true,
          item: {
            appId: 10,
            bundleId: "a",
            path: "/A.app",
            title: "A",
            bounds: { x: 0, y: 0, w: 40, h: 40 },
            screenId: 1,
            edge: "bottom",
            kind: "app",
          },
          entries: [{ ...appEntry(101, "Region target"), appId: 10 }],
          selectedWindowId: 101,
          appearance: emptyState.appearance,
          emptyReason: "",
        },
      }),
    );
    await waitFor(() =>
      expect((AppService as any).SetDockPreviewRegions).toHaveBeenCalledWith(21, 1, [
        expect.objectContaining({
          windowId: 101,
          appId: 10,
          bounds: { x: 10, y: 10, w: 100, h: 60 },
        }),
      ]),
    );
    bounds.mockRestore();
  });

  it("uses a Dock error revision as a state high-water mark", () => {
    window.location.hash = "#dock";
    render(<App />);
    const base = {
      session: 9,
      revision: 2,
      item: {
        appId: 10,
        bundleId: "a",
        path: "/A.app",
        title: "Current",
        bounds: { x: 0, y: 0, w: 40, h: 40 },
        screenId: 1,
        edge: "bottom",
        kind: "app",
      },
      entries: [],
      selectedWindowId: 0,
      appearance: emptyState.appearance,
      emptyReason: "unavailable",
    };
    act(() => {
      eventHandlers.get("dock:show")?.({ data: base });
      eventHandlers.get("dock:error")?.({
        data: { session: 9, revision: 4, message: "Current refusal" },
      });
      eventHandlers.get("dock:update")?.({
        data: { ...base, revision: 3, item: { ...base.item, title: "Stale" } },
      });
    });
    expect(screen.getByRole("alert")).toHaveTextContent("Current refusal");
    expect(screen.queryByText("Stale")).toBeNull();
  });

  it("uses a hidden catch-up snapshot as a same-session tombstone", async () => {
    let resolveSnapshot: (value: never) => void = () => {};
    (AppService as any).GetDockState.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveSnapshot = resolve;
      }),
    );
    window.location.hash = "#dock";
    render(<App />);
    const state = {
      session: 12,
      revision: 2,
      open: true,
      item: {
        appId: 10,
        bundleId: "a",
        path: "/A.app",
        title: "Shown",
        bounds: { x: 0, y: 0, w: 40, h: 40 },
        screenId: 1,
        edge: "bottom",
        kind: "app",
      },
      entries: [],
      selectedWindowId: 0,
      appearance: emptyState.appearance,
      emptyReason: "unavailable",
    };
    act(() => eventHandlers.get("dock:show")?.({ data: state }));
    await act(async () => resolveSnapshot({ ...state, revision: 3, open: false } as never));
    act(() => eventHandlers.get("dock:show")?.({ data: state }));
    expect(screen.queryByText("Shown")).toBeNull();
  });

  it("acts on the clicked window without changing selection", async () => {
    render(<App />);
    act(() => {
      eventHandlers.get("switcher:show")?.({
        data: openSwitcherState({
          entries: [appEntry(11, "Editor"), appEntry(22, "Browser"), appEntry(33, "Terminal")],
        }),
      });
    });
    fireEvent.click(screen.getAllByLabelText("Close window")[2]);
    act(() => {
      eventHandlers.get("switcher:update")?.({
        data: openSwitcherState({
          selected: 1,
          entries: [appEntry(11, "Editor"), appEntry(22, "Browser"), appEntry(33, "Terminal")],
        }),
      });
    });
    await waitFor(() => expect(mocked.PerformAction).toHaveBeenCalledWith("close", 33, 33));
    expect(mocked.Select).not.toHaveBeenCalled();
    expect(mocked.CloseSelected).not.toHaveBeenCalled();
  });

  it("shows a failed native action instead of silently discarding it", async () => {
    mocked.PerformAction.mockRejectedValueOnce(new Error("Window no longer exists"));
    render(<App />);
    act(() => {
      eventHandlers.get("switcher:show")?.({
        data: openSwitcherState({
          entries: [appEntry(11, "Editor")],
        }),
      });
    });
    fireEvent.click(screen.getByLabelText("Close window"));
    expect(await screen.findByRole("alert")).toHaveTextContent("Window no longer exists");
  });

  it("shows partial bulk-action failures", async () => {
    mocked.PerformAction.mockResolvedValueOnce({
      succeeded: 2,
      failures: [{ windowId: 11, error: "Window refused to close" }],
    } as never);
    render(<App />);
    act(() => {
      eventHandlers.get("switcher:show")?.({
        data: openSwitcherState({ entries: [appEntry(11, "Editor")] }),
      });
    });
    fireEvent.click(screen.getByLabelText("Close window"));
    expect(await screen.findByRole("alert")).toHaveTextContent("Window refused to close");
    expect(screen.getByRole("alert")).toHaveTextContent("2 action requests accepted");
  });

  it("does not show a late action failure in a reopened switcher", async () => {
    let rejectAction: (error: Error) => void = () => {};
    mocked.PerformAction.mockReturnValueOnce(
      new Promise((_, reject) => {
        rejectAction = reject;
      }) as never,
    );
    render(<App />);
    const state = openSwitcherState({ entries: [appEntry(11, "Editor")] });
    act(() => {
      eventHandlers.get("switcher:show")?.({ data: state });
    });
    fireEvent.click(screen.getByLabelText("Close window"));
    act(() => {
      eventHandlers.get("switcher:hide")?.({ data: null });
      eventHandlers.get("switcher:show")?.({ data: state });
    });
    await act(async () => {
      rejectAction(new Error("Old session failure"));
    });
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("confirms a clicked entry atomically by window id", async () => {
    render(<App />);
    await waitFor(() => expect(mocked.GetVersion).toHaveBeenCalled());

    act(() => {
      eventHandlers.get("switcher:show")?.({
        data: openSwitcherState({
          mouseHover: false,
          entries: [appEntry(11, "Editor"), appEntry(22, "Browser")],
        }),
      });
    });

    fireEvent.click(screen.getByText("Browser").closest('[role="option"]') as HTMLElement);
    expect(mocked.ConfirmWindow).toHaveBeenCalledWith(22);
    expect(mocked.Confirm).not.toHaveBeenCalled();
  });

  it("merges streamed thumbnails and previews by windowId and resets them on show", () => {
    const { container } = render(<App />);

    const previewState = () =>
      openSwitcherState({
        entries: [appEntry(1, "Editor")],
        appearance: { ...emptyState.appearance, previewSelected: true },
      });

    act(() => {
      eventHandlers.get("switcher:show")?.({ data: previewState() });
    });
    act(() => {
      eventHandlers.get("switcher:thumbnails")?.({ data: { "1": "data:thumb" } });
    });
    act(() => {
      eventHandlers.get("switcher:preview")?.({ data: { "1": "data:prev" } });
    });

    // The high-resolution preview wins for the selected-window preview...
    const previewImg = screen
      .getByLabelText("Selected window preview")
      .querySelector("img") as HTMLImageElement;
    expect(previewImg.src).toContain("data:prev");
    // ...while the entry cell keeps the merged thumbnail.
    const thumbImg = container.querySelector(".ot-thumb-img") as HTMLImageElement;
    expect(thumbImg).not.toBeNull();
    expect(thumbImg.src).toContain("data:thumb");

    // A new session drops the previous captures.
    act(() => {
      eventHandlers.get("switcher:show")?.({ data: previewState() });
    });
    expect(screen.queryByLabelText("Selected window preview")).toBeNull();
    expect(container.querySelector(".ot-thumb-img")).toBeNull();
  });

  it("renders the demo route with a style override and thumbnails fallback", () => {
    window.location.hash = "#demo:appIcons";
    const first = render(<App />);
    expect(first.container.querySelector(".ot-demo-backdrop")).not.toBeNull();
    expect(first.container.querySelector('[data-style="appIcons"]')).not.toBeNull();
    first.unmount();

    window.location.hash = "#demo";
    const second = render(<App />);
    expect(second.container.querySelector(".ot-demo-backdrop")).not.toBeNull();
    expect(second.container.querySelector('[data-style="thumbnails"]')).not.toBeNull();
  });

  it("deep-links a preferences tab via prefs:tab in the settings window", () => {
    window.location.hash = "#settings";
    render(<App />);

    const tabs = screen.getByRole("tablist");
    expect(within(tabs).getByRole("tab", { name: "General" })).toHaveAttribute(
      "aria-selected",
      "true",
    );

    act(() => {
      eventHandlers.get("prefs:tab")?.({ data: "About" });
    });
    expect(within(tabs).getByRole("tab", { name: "About" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });
});

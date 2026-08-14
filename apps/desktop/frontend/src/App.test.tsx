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
  Advance: vi.fn().mockResolvedValue(undefined),
  Reverse: vi.fn().mockResolvedValue(undefined),
  Confirm: vi.fn().mockResolvedValue(undefined),
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

  it("selects the clicked entry's index before closing its window", async () => {
    const calls: string[] = [];
    let resolveSelect: () => void = () => {};
    mocked.Select.mockImplementation((i: number) => {
      calls.push(`Select(${i})`);
      return new Promise<void>((resolve) => {
        resolveSelect = resolve;
      }) as never;
    });
    mocked.CloseSelected.mockImplementation(() => {
      calls.push("CloseSelected");
      return Promise.resolve() as never;
    });
    render(<App />);

    act(() => {
      eventHandlers.get("switcher:show")?.({
        data: openSwitcherState({
          entries: [appEntry(11, "Editor"), appEntry(22, "Browser"), appEntry(33, "Terminal")],
        }),
      });
    });

    // Close the NON-selected third entry (windowId 33 -> index 2).
    fireEvent.click(screen.getAllByLabelText("Close window")[2]);
    expect(calls).toEqual(["Select(2)"]);

    // CloseSelected must wait for Select to resolve (select-then-act).
    await Promise.resolve();
    expect(calls).toEqual(["Select(2)"]);
    resolveSelect();
    await waitFor(() => expect(calls).toEqual(["Select(2)", "CloseSelected"]));
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

import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import App from "./App";
import type { Entry, SwitcherState } from "./lib/types";
import { emptyState } from "./lib/types";

// stubWailsRuntime installs a fake Wails runtime and returns the handler map
// so tests can fire Go-side events (switcher:show, prefs:open, ...) directly.
function stubWailsRuntime(): Record<string, (data: unknown) => void> {
  const handlers: Record<string, (data: unknown) => void> = {};
  (globalThis as any).runtime = {
    EventsOn: (name: string, cb: (data: unknown) => void) => {
      handlers[name] = cb;
      return () => delete handlers[name];
    },
  };
  return handlers;
}

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

afterEach(() => {
  window.location.hash = "";
  (globalThis as any).runtime = undefined;
  (globalThis as any).go = undefined;
  vi.restoreAllMocks();
});

describe("App", () => {
  it("renders the overlay route (closed) by default", () => {
    window.location.hash = "";
    const { container } = render(<App />);
    // Overlay renders nothing while closed.
    expect(container.firstChild).toBeNull();
  });

  it("renders the settings route at #settings", () => {
    window.location.hash = "#settings";
    render(<App />);
    expect(screen.getByText(/Preferences/)).toBeInTheDocument();
  });

  it("opens the preferences panel on the prefs:open event", () => {
    const handlers: Record<string, (data: unknown) => void> = {};
    (globalThis as any).runtime = {
      EventsOn: (name: string, cb: (data: unknown) => void) => {
        handlers[name] = cb;
        return () => delete handlers[name];
      },
    };
    window.location.hash = "";
    render(<App />);
    // No preferences dialog until the menubar fires the event.
    expect(screen.queryByRole("dialog", { name: "Preferences" })).toBeNull();

    act(() => {
      handlers["prefs:open"]?.(null);
    });
    expect(screen.getByRole("dialog", { name: "Preferences" })).toBeInTheDocument();
  });

  it("selects the clicked entry's index before closing its window", async () => {
    const calls: string[] = [];
    let resolveSelect: () => void = () => {};
    (globalThis as any).go = {
      main: {
        App: {
          Select: vi.fn((i: number) => {
            calls.push(`Select(${i})`);
            return new Promise<void>((resolve) => {
              resolveSelect = resolve;
            });
          }),
          CloseSelected: vi.fn(() => {
            calls.push("CloseSelected");
            return Promise.resolve();
          }),
        },
      },
    };
    const handlers = stubWailsRuntime();
    window.location.hash = "";
    render(<App />);

    act(() => {
      handlers["switcher:show"]?.(
        openSwitcherState({
          entries: [appEntry(11, "Editor"), appEntry(22, "Browser"), appEntry(33, "Terminal")],
        }),
      );
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
    const handlers = stubWailsRuntime();
    window.location.hash = "";
    const { container } = render(<App />);

    const previewState = () =>
      openSwitcherState({
        entries: [appEntry(1, "Editor")],
        appearance: { ...emptyState.appearance, previewSelected: true },
      });

    act(() => {
      handlers["switcher:show"]?.(previewState());
    });
    act(() => {
      handlers["switcher:thumbnails"]?.({ "1": "data:thumb" });
    });
    act(() => {
      handlers["switcher:preview"]?.({ "1": "data:prev" });
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
      handlers["switcher:show"]?.(previewState());
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

  it("deep-links a preferences tab via prefs:tab and unmounts on prefs:close", () => {
    const handlers = stubWailsRuntime();
    window.location.hash = "";
    render(<App />);

    act(() => {
      handlers["prefs:open"]?.(null);
    });
    const dialog = screen.getByRole("dialog", { name: "Preferences" });
    expect(within(dialog).getByRole("tab", { name: "General" })).toHaveAttribute(
      "aria-selected",
      "true",
    );

    act(() => {
      handlers["prefs:tab"]?.("About");
    });
    expect(within(dialog).getByRole("tab", { name: "About" })).toHaveAttribute(
      "aria-selected",
      "true",
    );

    act(() => {
      handlers["prefs:close"]?.(null);
    });
    expect(screen.queryByRole("dialog", { name: "Preferences" })).toBeNull();
  });
});

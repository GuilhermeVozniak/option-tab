import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { emptyState } from "../lib/types";
import { AppSwitcher } from "./AppSwitcher";

const handlers = () => ({
  onAdvance: vi.fn(),
  onReverse: vi.fn(),
  onConfirm: vi.fn(),
  onConfirmWindow: vi.fn(),
  onCancel: vi.fn(),
  onSelect: vi.fn(),
  onSearchChange: vi.fn(),
  onClose: vi.fn(),
  onMinimize: vi.fn(),
  onFullscreen: vi.fn(),
  onQuit: vi.fn(),
  onHide: vi.fn(),
  onSelectApp: vi.fn(),
  onSelectAppWindow: vi.fn(),
  onConfirmApp: vi.fn(),
  onAction: vi.fn(),
});

describe("AppSwitcher", () => {
  it("keeps duplicate names as distinct PIDs and commits explicit preview targets", () => {
    const h = handlers();
    render(
      <AppSwitcher
        nativeKeys={false}
        handlers={h}
        state={{
          ...emptyState,
          open: true,
          mode: "apps",
          apps: [
            {
              appId: 10,
              appName: "Example",
              bundleId: "a",
              hidden: false,
              windowCount: 2,
              windowPresence: "present",
            },
            {
              appId: 20,
              appName: "Example",
              bundleId: "b",
              hidden: false,
              windowCount: 0,
              windowPresence: "none",
            },
          ],
          entries: [
            {
              windowId: 101,
              appId: 10,
              appName: "Example",
              bundleId: "a",
              title: "Document A",
              minimized: false,
              hidden: false,
              fullscreen: false,
            },
            {
              windowId: 102,
              appId: 10,
              appName: "Example",
              bundleId: "a",
              title: "Document B",
              minimized: false,
              hidden: false,
              fullscreen: false,
            },
          ],
          selected: 0,
          selectedWindowId: 101,
        }}
      />,
    );
    expect(screen.getAllByRole("option", { name: /Example/ })).toHaveLength(2);
    fireEvent.click(screen.getByRole("button", { name: "Focus Document B" }));
    expect(h.onSelectAppWindow).toHaveBeenCalledWith(102);
    expect(h.onConfirmWindow).toHaveBeenCalledWith(102);
  });

  it("shows unknown windows without inventing controls and activates exact app", () => {
    const h = handlers();
    render(
      <AppSwitcher
        nativeKeys={false}
        handlers={h}
        state={{
          ...emptyState,
          open: true,
          mode: "apps",
          apps: [
            {
              appId: 30,
              appName: "Notes",
              bundleId: "c",
              hidden: false,
              windowCount: 0,
              windowPresence: "unknown",
            },
          ],
          entries: [],
          selected: 0,
          selectedWindowId: 0,
        }}
      />,
    );
    expect(screen.getByText("Windows unavailable")).toBeInTheDocument();
    expect(screen.queryByLabelText("Close window")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Open Notes" }));
    expect(h.onConfirmApp).toHaveBeenCalledWith(30);
  });

  it("uses primary arrows for apps and orthogonal arrows for previews", () => {
    const h = handlers();
    render(
      <AppSwitcher
        nativeKeys={false}
        handlers={h}
        state={{
          ...emptyState,
          open: true,
          mode: "apps",
          apps: [{ appId: 10, appName: "A", bundleId: "a", hidden: false, windowCount: 1 }],
          entries: [
            {
              windowId: 101,
              appId: 10,
              appName: "A",
              bundleId: "a",
              title: "One",
              minimized: false,
              hidden: false,
              fullscreen: false,
            },
          ],
          selected: 0,
          selectedWindowId: 101,
        }}
      />,
    );
    fireEvent.keyDown(window, { key: "ArrowRight" });
    fireEvent.keyDown(window, { key: "ArrowDown" });
    expect(h.onAdvance).toHaveBeenCalled();
    expect(h.onSelectAppWindow).toHaveBeenCalledWith(101);
  });

  it("keeps bare binding letters for search and applies bindings with a modifier", () => {
    const h = handlers();
    render(
      <AppSwitcher
        nativeKeys={false}
        handlers={h}
        state={{
          ...emptyState,
          open: true,
          mode: "apps",
          apps: [{ appId: 10, appName: "A", bundleId: "a", hidden: false, windowCount: 0 }],
          actionBindings: { KeyW: "closeAll" },
        }}
      />,
    );
    fireEvent.keyDown(window, { key: "w", code: "KeyW" });
    fireEvent.keyDown(window, { key: "w", code: "KeyW", metaKey: true });
    expect(h.onSearchChange).toHaveBeenCalledWith("w");
    expect(h.onAction).toHaveBeenCalledWith("closeAll", 0, 10);
  });

  it("honors arrow, Vim, controls, layout, capacity, and selected-preview preferences", () => {
    const h = handlers();
    const state = {
      ...emptyState,
      open: true,
      mode: "apps" as const,
      arrowKeys: false,
      vimKeys: true,
      appearance: {
        ...emptyState.appearance,
        layoutDirection: "vertical" as const,
        maxRows: 2,
        maxColumns: 3,
        showWindowControls: false,
        previewSelected: true,
      },
      apps: [{ appId: 10, appName: "A", bundleId: "a", hidden: false, windowCount: 1 }],
      entries: [
        {
          windowId: 101,
          appId: 10,
          appName: "A",
          bundleId: "a",
          title: "One",
          minimized: false,
          hidden: false,
          fullscreen: false,
          preview: "data:image/png;base64,preview",
        },
      ],
      selectedWindowId: 101,
    };
    const { container } = render(<AppSwitcher nativeKeys={false} handlers={h} state={state} />);
    fireEvent.keyDown(window, { key: "ArrowDown", code: "ArrowDown" });
    fireEvent.keyDown(window, { key: "j", code: "KeyJ" });
    expect(h.onAdvance).toHaveBeenCalledTimes(1);
    expect(screen.queryByLabelText("Close window")).not.toBeInTheDocument();
    expect(screen.getByLabelText("Selected window preview")).toBeInTheDocument();
    expect(container.querySelector(".ot-app-switcher")).toHaveClass("is-vertical");
    expect(container.querySelector(".ot-app-switcher")).toHaveStyle({
      "--ot-app-columns": "1",
      "--ot-app-rows": "1",
    });
  });

  it("honors gallery hover, middle click, and swipe actions without confirming a swipe", () => {
    const h = handlers();
    render(
      <AppSwitcher
        nativeKeys={false}
        handlers={h}
        state={{
          ...emptyState,
          open: true,
          mode: "apps",
          mouseHover: false,
          middleClickAction: "minimize",
          swipeUpAction: "close",
          apps: [{ appId: 10, appName: "A", bundleId: "a", hidden: false, windowCount: 1 }],
          entries: [
            {
              windowId: 101,
              appId: 10,
              appName: "A",
              bundleId: "a",
              title: "One",
              minimized: false,
              hidden: false,
              fullscreen: false,
            },
          ],
          selectedWindowId: 101,
        }}
      />,
    );
    const preview = screen.getByRole("button", { name: "Focus One" });
    fireEvent.mouseEnter(preview);
    expect(h.onSelectAppWindow).not.toHaveBeenCalled();
    fireEvent(preview, new MouseEvent("auxclick", { bubbles: true, button: 1 }));
    expect(h.onMinimize).toHaveBeenCalledWith(101);
    const pointer = (type: string, clientY: number) => {
      const event = new Event(type, { bubbles: true });
      Object.defineProperties(event, {
        button: { value: 0 },
        isPrimary: { value: true },
        pointerId: { value: 7 },
        clientY: { value: clientY },
      });
      return event;
    };
    fireEvent(preview, pointer("pointerdown", 100));
    fireEvent(preview, pointer("pointermove", 40));
    fireEvent(preview, pointer("pointerup", 40));
    expect(h.onClose).toHaveBeenCalledWith(101);
    expect(h.onConfirmWindow).not.toHaveBeenCalled();
  });

  it("keeps streamed frames out of app-icon and title styles and reduces wheel momentum once", () => {
    vi.useFakeTimers();
    const h = handlers();
    const state = {
      ...emptyState,
      open: true,
      mode: "apps" as const,
      swipeUpAction: "close" as const,
      apps: [{ appId: 10, appName: "A", bundleId: "a", hidden: false, windowCount: 1 }],
      entries: [
        {
          windowId: 101,
          appId: 10,
          appName: "A",
          bundleId: "a",
          title: "One",
          minimized: false,
          hidden: false,
          fullscreen: false,
          icon: "data:image/png;base64,icon",
          thumbnail: "data:image/png;base64,frame",
        },
      ],
      selectedWindowId: 101,
      appearance: { ...emptyState.appearance, style: "appIcons" as const, compactThreshold: 99 },
    };
    const { container, rerender } = render(
      <AppSwitcher nativeKeys={false} handlers={h} state={state} />,
    );
    expect(container.querySelector(".ot-app-card-fallback > img")).toHaveAttribute(
      "src",
      "data:image/png;base64,icon",
    );
    expect(
      container.querySelector('img[src="data:image/png;base64,frame"]'),
    ).not.toBeInTheDocument();
    rerender(
      <AppSwitcher
        nativeKeys={false}
        handlers={h}
        state={{ ...state, appearance: { ...state.appearance, style: "titles" } }}
      />,
    );
    expect(screen.getByText("One")).toBeInTheDocument();
    expect(container.querySelector(".ot-app-card-media")).not.toBeInTheDocument();
    const card = screen.getByRole("button", { name: "Focus One" });
    for (let index = 0; index < 8; index++) {
      fireEvent.wheel(card, { deltaY: -10 });
      act(() => vi.advanceTimersByTime(60));
    }
    expect(h.onClose).toHaveBeenCalledTimes(1);
    expect(h.onConfirmWindow).not.toHaveBeenCalled();
    vi.useRealTimers();
  });

  it("renders compact title rows after a streamed thumbnail and exposes appearance metadata", () => {
    const state = {
      ...emptyState,
      open: true,
      mode: "apps" as const,
      apps: [
        {
          appId: 10,
          appName: "Writer",
          bundleId: "a",
          hidden: false,
          windowCount: 1,
          icon: "data:image/png;base64,icon",
        },
      ],
      entries: [
        {
          windowId: 101,
          appId: 10,
          appName: "Writer",
          bundleId: "a",
          title: "A very long document title that should retain its useful ending.pdf",
          minimized: true,
          hidden: false,
          fullscreen: false,
          spaceId: 42,
        },
      ],
      selectedWindowId: 101,
      appearance: {
        ...emptyState.appearance,
        style: "thumbnails" as const,
        compactThreshold: 1,
        showTitle: true,
        showAppBadge: true,
        showStatusIcons: true,
        showSpaceNumbers: true,
        titleTruncation: "middle" as const,
        titleMaxWidthPx: 120,
      },
    };
    const { container, rerender } = render(
      <AppSwitcher nativeKeys={false} handlers={handlers()} state={state} />,
    );
    expect(container.querySelector(".ot-app-switcher")).toHaveClass("style-titles");
    expect(screen.getByText(/A very long document/)).toBeInTheDocument();
    rerender(
      <AppSwitcher
        nativeKeys={false}
        handlers={handlers()}
        state={{
          ...state,
          entries: [{ ...state.entries[0], thumbnail: "data:image/png;base64,frame" }],
        }}
      />,
    );
    expect(screen.getByText(/A very long document/)).toBeInTheDocument();
    expect(container.querySelector(".ot-app-preview img")).not.toBeInTheDocument();
    expect(screen.getByLabelText("Space 1")).toBeInTheDocument();
    expect(screen.getByLabelText("Minimized")).toBeInTheDocument();
  });

  it("applies panel, layout, preview, badge, and native window-control preferences live", () => {
    const state = {
      ...emptyState,
      open: true,
      mode: "apps" as const,
      apps: [
        {
          appId: 10,
          appName: "Writer",
          bundleId: "a",
          hidden: false,
          windowCount: 1,
          icon: "data:image/png;base64,icon",
        },
      ],
      entries: [
        {
          windowId: 101,
          appId: 10,
          appName: "Writer",
          bundleId: "a",
          title: "Draft",
          minimized: false,
          hidden: false,
          fullscreen: false,
          thumbnail: "data:image/png;base64,frame",
        },
      ],
      selectedWindowId: 101,
      appearance: {
        ...emptyState.appearance,
        theme: "light" as const,
        blur: false,
        backgroundOpacity: 0.55,
        maxColumns: 1,
        maxRows: 1,
        thumbnailMaxPx: 220,
        autoSize: false,
        showTitle: true,
        showAppBadge: true,
        showWindowControls: true,
        previewSelected: true,
      },
    };
    const { container, rerender } = render(
      <AppSwitcher nativeKeys={false} handlers={handlers()} state={state} />,
    );
    const root = container.querySelector(".ot-app-switcher")!;
    expect(root).toHaveClass("ot-theme-light", "ot-no-blur");
    expect(root).toHaveStyle({
      "--ot-bg-opacity": "0.55",
      "--ot-app-columns": "1",
      "--ot-app-rows": "1",
      "--ot-app-thumb": "220px",
    });
    expect(container.querySelector(".ot-app-badge")).toBeInTheDocument();
    expect(container.querySelector(".ot-traffic-close")).toBeInTheDocument();
    expect(screen.getByLabelText("Selected window preview")).toHaveStyle({
      "--ot-preview-width": "352px",
    });
    rerender(
      <AppSwitcher
        nativeKeys={false}
        handlers={handlers()}
        state={{
          ...state,
          appearance: {
            ...state.appearance,
            thumbnailMaxPx: 300,
            showTitle: false,
            showAppBadge: false,
            showWindowControls: false,
          },
        }}
      />,
    );
    expect(screen.getByLabelText("Selected window preview")).toHaveStyle({
      "--ot-preview-width": "480px",
    });
    expect(screen.queryByText("Draft")).not.toBeInTheDocument();
    expect(container.querySelector(".ot-app-badge")).not.toBeInTheDocument();
    expect(container.querySelector(".ot-traffic-close")).not.toBeInTheDocument();
  });

  it("delays apparition and retains a closing frame only when fade-out is enabled", () => {
    vi.useFakeTimers();
    const state = {
      ...emptyState,
      open: true,
      mode: "apps" as const,
      appearance: { ...emptyState.appearance, apparitionDelayMs: 80, fadeOutAnimation: true },
    };
    const { rerender } = render(
      <AppSwitcher nativeKeys={false} handlers={handlers()} state={state} />,
    );
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    act(() => vi.advanceTimersByTime(80));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    rerender(
      <AppSwitcher nativeKeys={false} handlers={handlers()} state={{ ...state, open: false }} />,
    );
    expect(screen.getByRole("dialog")).toHaveClass("ot-closing");
    act(() => vi.advanceTimersByTime(180));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    vi.useRealTimers();
  });
});

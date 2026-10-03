import { act, fireEvent, render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AppSwitcher } from "../app-switcher/AppSwitcher";
import { Overlay } from "../overlay/Overlay";
import type { OverlayHandlers } from "../overlay/types";
import { emptyState, type SwitcherState } from "./types";

function rect(x: number, y: number, width: number, height: number): DOMRect {
  return {
    x,
    y,
    width,
    height,
    left: x,
    top: y,
    right: x + width,
    bottom: y + height,
    toJSON() {},
  };
}
const handlers: OverlayHandlers = {
  onAdvance() {},
  onReverse() {},
  onConfirm() {},
  onConfirmWindow() {},
  onCancel() {},
  onSelect() {},
  onSearchChange() {},
  onClose() {},
  onMinimize() {},
  onFullscreen() {},
  onQuit() {},
  onHide() {},
};
const entries = [1, 2, 3].map((windowId) => ({
  windowId,
  appId: 31,
  appName: "Editor",
  title: `Window ${windowId}`,
  bundleId: "test.editor",
  minimized: false,
  hidden: false,
  fullscreen: false,
}));
function state(mode: "apps" | "windows", style: SwitcherState["style"]): SwitcherState {
  return {
    ...emptyState,
    mode,
    session: 7,
    revision: 1,
    open: true,
    style,
    entries,
    appearance: { ...emptyState.appearance, style, apparitionDelayMs: 0 },
    apps: [
      { appId: 31, appName: "Editor", bundleId: "test.editor", windowCount: 3, hidden: false },
    ],
  };
}
let cardOffset = 0;
beforeEach(() => {
  cardOffset = 0;
  Object.defineProperty(HTMLElement.prototype, "getAnimations", {
    configurable: true,
    value: () => [],
  });
  vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockImplementation(function (
    this: HTMLElement,
  ) {
    return this.getBoundingClientRect().width;
  });
  vi.spyOn(HTMLElement.prototype, "clientHeight", "get").mockImplementation(function (
    this: HTMLElement,
  ) {
    return this.getBoundingClientRect().height;
  });
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function (
    this: HTMLElement,
  ) {
    if (this.matches(".ot-panel, .ot-app-panel")) return rect(10, 10, 600, 400);
    if (this.matches(".ot-list, .ot-app-gallery")) return rect(20, 80, 360, 200);
    if (this.matches(".ot-entry, .ot-app-gallery article")) {
      const index = Array.from(this.parentElement?.children ?? []).indexOf(this);
      return rect(30 + index * 180 - cardOffset, 100, 160, 220);
    }
    return rect(0, 0, 0, 0);
  });
});
afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

describe.each(["windows", "apps"] as const)("%s native gesture geometry", (mode) => {
  const Switcher = mode === "apps" ? AppSwitcher : Overlay;
  it("keeps the actual embedded control hit area outside native ownership", () => {
    const report = vi.fn();
    const { container } = render(
      <Switcher
        state={state(mode, "thumbnails")}
        handlers={{ ...handlers, onGestureRegions: report }}
      />,
    );
    const control = container.querySelector<HTMLElement>(".ot-controls, .ot-app-window-actions")!;
    vi.spyOn(control, "getBoundingClientRect").mockReturnValue(rect(60, 140, 80, 30));
    fireEvent.scroll(container.querySelector(".ot-list, .ot-app-gallery")!);
    const parts = report.mock.lastCall?.[2].filter(
      (region: { windowId: number }) => region.windowId === 1,
    );
    expect(parts).toHaveLength(4);
    for (const { bounds: b } of parts) {
      expect(b.x >= 140 || b.x + b.w <= 60 || b.y >= 170 || b.y + b.h <= 140).toBe(true);
    }
  });

  it("leaves moving or fading presentation frames unowned until the entrance animation ends", () => {
    let animating = true;
    vi.spyOn(HTMLElement.prototype, "getAnimations").mockImplementation(() =>
      animating ? [{ playState: "running" } as Animation] : [],
    );
    const report = vi.fn();
    const { container } = render(
      <Switcher
        state={state(mode, "thumbnails")}
        handlers={{ ...handlers, onGestureRegions: report }}
      />,
    );
    expect(report).toHaveBeenLastCalledWith(7, 1, []);
    animating = false;
    fireEvent.animationEnd(container.querySelector(".ot-panel, .ot-app-panel")!);
    expect(report.mock.lastCall?.[2].length).toBeGreaterThan(0);
  });

  it("does not claim scrollbars or borders as visible card area", () => {
    const report = vi.fn();
    const { container } = render(
      <Switcher
        state={state(mode, "thumbnails")}
        handlers={{ ...handlers, onGestureRegions: report }}
      />,
    );
    const viewport = container.querySelector<HTMLElement>(".ot-list, .ot-app-gallery")!;
    viewport.style.overflow = "scroll";
    Object.defineProperties(viewport, {
      clientLeft: { configurable: true, value: 2 },
      clientTop: { configurable: true, value: 2 },
      clientWidth: { configurable: true, value: 328 },
      clientHeight: { configurable: true, value: 168 },
    });
    fireEvent.scroll(viewport);
    expect(report).toHaveBeenLastCalledWith(7, 1, [
      { windowId: 1, appId: 31, bounds: { x: 30, y: 100, w: 160, h: 150 } },
      { windowId: 2, appId: 31, bounds: { x: 210, y: 100, w: 140, h: 150 } },
    ]);
  });

  it("drops obsolete observer callbacks after a changed gallery or unmount", () => {
    const callbacks: Array<() => void> = [];
    vi.stubGlobal(
      "ResizeObserver",
      class {
        constructor(callback: () => void) {
          callbacks.push(callback);
        }
        observe() {}
        disconnect() {}
      },
    );
    const report = vi.fn();
    const original = state(mode, "titles");
    const { rerender, unmount } = render(
      <Switcher state={original} handlers={{ ...handlers, onGestureRegions: report }} />,
    );
    const oldCallbacks = [...callbacks];
    rerender(
      <Switcher
        state={{ ...original, revision: 2, entries: original.entries.slice(1) }}
        handlers={{ ...handlers, onGestureRegions: report }}
      />,
    );
    report.mockClear();
    act(() => {
      for (const callback of oldCallbacks) callback();
    });
    expect(report).not.toHaveBeenCalled();
    unmount();
    expect(report).toHaveBeenLastCalledWith(7, 2, []);
    report.mockClear();
    act(() => {
      for (const callback of callbacks) callback();
    });
    expect(report).not.toHaveBeenCalled();
  });

  it.each([
    "thumbnails",
    "appIcons",
    "titles",
  ] as const)("reports only clipped %s window cards", (style) => {
    const report = vi.fn();
    const { container } = render(
      <Switcher state={state(mode, style)} handlers={{ ...handlers, onGestureRegions: report }} />,
    );
    const viewport = container.querySelector<HTMLElement>(".ot-list, .ot-app-gallery")!;
    viewport.style.overflow = "auto";
    fireEvent.scroll(viewport);
    expect(report).toHaveBeenLastCalledWith(7, 1, [
      { windowId: 1, appId: 31, bounds: { x: 30, y: 100, w: 160, h: 180 } },
      { windowId: 2, appId: 31, bounds: { x: 210, y: 100, w: 160, h: 180 } },
    ]);
    cardOffset = 180;
    fireEvent.scroll(viewport);
    expect(report).toHaveBeenLastCalledWith(7, 1, [
      { windowId: 2, appId: 31, bounds: { x: 30, y: 100, w: 160, h: 180 } },
      { windowId: 3, appId: 31, bounds: { x: 210, y: 100, w: 160, h: 180 } },
    ]);
  });

  it("republishes unchanged rectangles at the new state revision and retires hidden/fading cards", () => {
    const report = vi.fn();
    const original = state(mode, "thumbnails");
    const { rerender, unmount } = render(
      <Switcher state={original} handlers={{ ...handlers, onGestureRegions: report }} />,
    );
    expect(report).toHaveBeenLastCalledWith(7, 1, expect.any(Array));
    rerender(
      <Switcher
        state={{ ...original, revision: 2 }}
        handlers={{ ...handlers, onGestureRegions: report }}
      />,
    );
    expect(report).toHaveBeenLastCalledWith(7, 2, expect.any(Array));
    rerender(
      <Switcher
        state={{ ...original, revision: 3, open: false }}
        handlers={{ ...handlers, onGestureRegions: report }}
      />,
    );
    expect(report).toHaveBeenLastCalledWith(7, 3, []);
    unmount();
    expect(report.mock.calls.at(-1)?.[2]).toEqual([]);
  });

  it("keeps delayed apparition unowned, including a new session immediately after hiding", () => {
    vi.useFakeTimers();
    const report = vi.fn();
    const original = state(mode, "thumbnails");
    original.appearance.apparitionDelayMs = 150;
    const { rerender } = render(
      <Switcher state={original} handlers={{ ...handlers, onGestureRegions: report }} />,
    );
    expect(report.mock.calls.every((call) => call[2].length === 0)).toBe(true);
    act(() => vi.advanceTimersByTime(150));
    expect(report.mock.calls.at(-1)?.[2].length).toBeGreaterThan(0);
    rerender(
      <Switcher
        state={{ ...original, open: false, revision: 2 }}
        handlers={{ ...handlers, onGestureRegions: report }}
      />,
    );
    report.mockClear();
    rerender(
      <Switcher
        state={{ ...original, session: 8 }}
        handlers={{ ...handlers, onGestureRegions: report }}
      />,
    );
    expect(report.mock.calls.every((call) => call[2].length === 0)).toBe(true);
    act(() => vi.advanceTimersByTime(150));
    expect(report).toHaveBeenLastCalledWith(
      8,
      1,
      expect.arrayContaining([expect.objectContaining({ windowId: 1 })]),
    );
  });
});

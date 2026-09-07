import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { makeT } from "../lib/i18n";
import { type DockViewState, emptyState } from "../lib/types";
import { DockPanelView } from "./DockPanelView";

const state: DockViewState = {
  session: 7,
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
    {
      windowId: 102,
      appId: 10,
      appName: "A",
      bundleId: "a",
      title: "Document B",
      minimized: false,
      hidden: false,
      fullscreen: false,
    },
  ],
  selectedWindowId: 102,
  appearance: emptyState.appearance,
  emptyReason: "",
  error: "refused",
};
it("sends session and explicit target and displays native errors", () => {
  const h = { onSelectWindow: vi.fn(), onFocusWindow: vi.fn(), onAction: vi.fn(), onSize: vi.fn() };
  render(<DockPanelView state={state} handlers={h} />);
  expect(screen.getByRole("alert")).toHaveTextContent("refused");
  fireEvent.click(screen.getByRole("button", { name: "Focus Document B" }));
  expect(h.onFocusWindow).toHaveBeenCalledWith(7, 102, 10);
  fireEvent.click(screen.getByLabelText("Close window"));
  expect(h.onAction).toHaveBeenCalledWith(7, "close", 102, 10);
});

it("selects an available media or windows view without invoking window actions", () => {
  const h = {
    onSelectWindow: vi.fn(),
    onFocusWindow: vi.fn(),
    onAction: vi.fn(),
    onSize: vi.fn(),
    onSelectContent: vi.fn(),
  };
  render(
    <DockPanelView
      state={{
        ...state,
        revision: 9,
        contentKind: "windows",
        contentOptions: ["windows", "media"],
      }}
      handlers={h}
    />,
  );
  expect(screen.getByRole("button", { name: "Windows" })).toHaveAttribute("aria-pressed", "true");
  fireEvent.click(screen.getByRole("button", { name: "Media" }));
  expect(h.onSelectContent).toHaveBeenCalledWith(7, 9, "media");
  expect(h.onFocusWindow).not.toHaveBeenCalled();
  expect(h.onAction).not.toHaveBeenCalled();
  expect(screen.getByRole("button", { name: "Windows" })).toHaveAttribute("aria-pressed", "true");
});

it("keeps the selector available while the selected window inventory is loading", () => {
  const h = { onSelectWindow: vi.fn(), onFocusWindow: vi.fn(), onAction: vi.fn(), onSize: vi.fn() };
  render(
    <DockPanelView
      state={{
        ...state,
        revision: 10,
        contentKind: "windows",
        contentOptions: ["windows", "media"],
        entries: [],
        emptyReason: "loading",
      }}
      handlers={h}
    />,
  );
  expect(screen.getByText("Loading windows…")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Media" })).toBeEnabled();
});

it("localizes the generic native preview close control", () => {
  const h = { onSelectWindow: vi.fn(), onFocusWindow: vi.fn(), onAction: vi.fn(), onSize: vi.fn() };
  render(
    <DockPanelView state={state} handlers={h} nativeHeader onClose={vi.fn()} t={makeT("es")} />,
  );
  expect(screen.getByRole("button", { name: "Cerrar vista previa" })).toBeInTheDocument();
});

it("starts an exact normalized preview drag after threshold and suppresses focus", () => {
  vi.stubGlobal("PointerEvent", MouseEvent);
  const h = {
    onSelectWindow: vi.fn(),
    onFocusWindow: vi.fn(),
    onAction: vi.fn(),
    onSize: vi.fn(),
    onBeginDrag: vi.fn(),
    onCancelDrag: vi.fn(),
  };
  render(
    <DockPanelView
      state={{ ...state, previewDragEnabled: true, dragGestureFloor: 44 }}
      handlers={h}
    />,
  );
  const preview = screen.getByRole("button", { name: "Focus Document B" });
  vi.spyOn(preview, "getBoundingClientRect").mockReturnValue({
    left: 100,
    top: 200,
    width: 200,
    height: 100,
    right: 300,
    bottom: 300,
    x: 100,
    y: 200,
    toJSON: () => ({}),
  });
  fireEvent.pointerDown(preview, {
    pointerId: 4,
    button: 0,
    isPrimary: true,
    clientX: 150,
    clientY: 225,
  });
  fireEvent.pointerMove(preview, {
    pointerId: 4,
    clientX: 152,
    clientY: 227,
    screenX: 502,
    screenY: 302,
  });
  expect(h.onBeginDrag).not.toHaveBeenCalled();
  fireEvent.pointerMove(preview, {
    pointerId: 4,
    clientX: 170,
    clientY: 245,
    screenX: 520,
    screenY: 320,
  });
  expect(h.onBeginDrag).toHaveBeenCalledWith(
    expect.objectContaining({
      session: 7,
      windowId: 102,
      appId: 10,
      pointerX: 520,
      pointerY: 320,
      grabX: 0.25,
      grabY: 0.25,
    }),
  );
  const gesture = h.onBeginDrag.mock.calls[0][0].gesture;
  expect(gesture).toBe(45);
  fireEvent.pointerUp(preview, { pointerId: 4 });
  expect(h.onCancelDrag).toHaveBeenCalledWith(7, gesture);
  fireEvent.click(preview);
  expect(h.onFocusWindow).not.toHaveBeenCalled();
});

it("never rearms a terminal physical down and native handoff cancel does not stop its owner", () => {
  vi.stubGlobal("PointerEvent", MouseEvent);
  let finish!: () => void;
  const pending = new Promise<void>((resolve) => {
    finish = resolve;
  });
  const h = {
    onSelectWindow: vi.fn(),
    onFocusWindow: vi.fn(),
    onAction: vi.fn(),
    onSize: vi.fn(),
    onBeginDrag: vi.fn(() => pending),
    onCancelDrag: vi.fn(),
  };
  render(<DockPanelView state={{ ...state, previewDragEnabled: true }} handlers={h} />);
  const preview = screen.getByRole("button", { name: "Focus Document B" });
  vi.spyOn(preview, "getBoundingClientRect").mockReturnValue({
    left: 0,
    top: 0,
    width: 100,
    height: 100,
    right: 100,
    bottom: 100,
    x: 0,
    y: 0,
    toJSON: () => ({}),
  });
  fireEvent.pointerDown(preview, {
    pointerId: 8,
    button: 0,
    isPrimary: true,
    clientX: 20,
    clientY: 20,
  });
  fireEvent.pointerMove(preview, {
    pointerId: 8,
    clientX: 40,
    clientY: 40,
    screenX: 40,
    screenY: 40,
  });
  fireEvent.pointerCancel(preview, { pointerId: 8 });
  fireEvent.pointerMove(preview, {
    pointerId: 8,
    clientX: 70,
    clientY: 70,
    screenX: 70,
    screenY: 70,
  });
  expect(h.onBeginDrag).toHaveBeenCalledTimes(1);
  expect(h.onCancelDrag).not.toHaveBeenCalled();
  fireEvent.click(preview);
  expect(h.onFocusWindow).not.toHaveBeenCalled();
  finish();
});

it("maps native panel points to exact visible cards and clears hover without actions", () => {
  const h = {
    onSelectWindow: vi.fn(),
    onFocusWindow: vi.fn(),
    onAction: vi.fn(),
    onSize: vi.fn(),
  };
  let hit: Element | null = null;
  vi.stubGlobal("ResizeObserver", undefined);
  Object.defineProperty(document, "elementFromPoint", {
    configurable: true,
    value: vi.fn(() => hit),
  });
  const { container, rerender } = render(
    <DockPanelView
      state={state}
      handlers={h}
      nativePointer={{ session: 7, sequence: 1, x: 12, y: 18, inside: true }}
    />,
  );
  const article = container.querySelector("article[data-window-id='102']")!;
  hit = article.querySelector("button");
  rerender(
    <DockPanelView
      state={state}
      handlers={h}
      nativePointer={{ session: 7, sequence: 2, x: 14, y: 20, inside: true }}
    />,
  );
  expect(article).toHaveClass("is-hovered");
  expect(h.onSelectWindow).toHaveBeenCalledWith(7, 102);
  const viewport = container.querySelector(".ot-dock-list-viewport")!;
  fireEvent.scroll(viewport);
  expect(h.onSelectWindow).toHaveBeenCalledTimes(1);
  hit = null;
  fireEvent.scroll(viewport);
  expect(article).not.toHaveClass("is-hovered");
  hit = article;
  rerender(
    <DockPanelView
      state={{ ...state, appearance: { ...state.appearance, layoutDirection: "vertical" } }}
      handlers={h}
      nativePointer={{ session: 7, sequence: 2, x: 14, y: 20, inside: true }}
    />,
  );
  expect(article).toHaveClass("is-hovered");
  expect(h.onSelectWindow).toHaveBeenCalledTimes(2);
  expect(h.onFocusWindow).not.toHaveBeenCalled();
  expect(h.onAction).not.toHaveBeenCalled();
  rerender(
    <DockPanelView
      state={state}
      handlers={h}
      nativePointer={{ session: 7, sequence: 3, x: -1, y: -1, inside: false }}
    />,
  );
  expect(article).not.toHaveClass("is-hovered");
  vi.restoreAllMocks();
  Reflect.deleteProperty(document, "elementFromPoint");
  vi.unstubAllGlobals();
});

it("publishes deduplicated visible clipped preview regions and updates them on scroll", () => {
  const onRegions = vi.fn();
  const h = {
    onSelectWindow: vi.fn(),
    onFocusWindow: vi.fn(),
    onAction: vi.fn(),
    onSize: vi.fn(),
    onRegions,
  };
  let articleX = 0;
  const rect = vi
    .spyOn(HTMLElement.prototype, "getBoundingClientRect")
    .mockImplementation(function (this: HTMLElement) {
      if (this.classList.contains("ot-dock-panel"))
        return DOMRect.fromRect({ x: 0, y: 0, width: 300, height: 200 });
      if (this.classList.contains("ot-dock-list-viewport"))
        return DOMRect.fromRect({ x: 10, y: 20, width: 200, height: 100 });
      if (this.matches("article[data-window-id]"))
        return DOMRect.fromRect({ x: articleX, y: 10, width: 100, height: 80 });
      return DOMRect.fromRect({ x: 0, y: 0, width: 280, height: 160 });
    });
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      disconnect() {}
    },
  );
  try {
    const { container, rerender } = render(<DockPanelView state={state} handlers={h} />);
    expect(onRegions).toHaveBeenLastCalledWith(7, 1, [
      { windowId: 102, appId: 10, bounds: { x: 10, y: 20, w: 90, h: 70 } },
    ]);
    const count = onRegions.mock.calls.length;
    rerender(<DockPanelView state={{ ...state }} handlers={h} />);
    expect(onRegions).toHaveBeenCalledTimes(count);
    articleX = 30;
    fireEvent.scroll(container.querySelector(".ot-dock-list-viewport")!);
    expect(onRegions).toHaveBeenLastCalledWith(7, 2, [
      { windowId: 102, appId: 10, bounds: { x: 30, y: 20, w: 100, h: 70 } },
    ]);
    rerender(<DockPanelView state={{ ...state, session: 8 }} handlers={h} />);
    expect(onRegions).toHaveBeenCalledWith(7, 3, []);
    expect(onRegions).toHaveBeenLastCalledWith(8, 1, [
      { windowId: 102, appId: 10, bounds: { x: 30, y: 20, w: 100, h: 70 } },
    ]);
  } finally {
    rect.mockRestore();
    vi.unstubAllGlobals();
  }
});

it("publishes an empty region policy for a mounted empty Dock panel", () => {
  const onRegions = vi.fn();
  const h = {
    onSelectWindow: vi.fn(),
    onFocusWindow: vi.fn(),
    onAction: vi.fn(),
    onSize: vi.fn(),
    onRegions,
  };
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      disconnect() {}
    },
  );
  try {
    render(<DockPanelView state={{ ...state, entries: [] }} handlers={h} />);
    expect(onRegions).toHaveBeenCalledWith(7, 1, []);
  } finally {
    vi.unstubAllGlobals();
  }
});

it("always names windows in title-list mode even when optional thumbnail titles are hidden", () => {
  const h = {
    onSelectWindow: vi.fn(),
    onFocusWindow: vi.fn(),
    onAction: vi.fn(),
    onSize: vi.fn(),
  };
  render(
    <DockPanelView
      state={{
        ...state,
        appearance: {
          ...state.appearance,
          style: "titles",
          compactThreshold: 0,
          showTitle: false,
        },
      }}
      handlers={h}
    />,
  );
  expect(screen.getByRole("button", { name: "Focus Document B" })).toHaveTextContent("Document B");
});

it("honors selected preview and controls independently, using the exact selected image", () => {
  const h = { onSelectWindow: vi.fn(), onFocusWindow: vi.fn(), onAction: vi.fn(), onSize: vi.fn() };
  const s = {
    ...state,
    entries: [
      {
        ...state.entries[0],
        thumbnail: "data:image/png;base64,thumb",
        preview: "data:image/png;base64,large",
      },
    ],
    appearance: { ...state.appearance, previewSelected: true, showWindowControls: false },
  };
  const { rerender } = render(<DockPanelView state={s} handlers={h} />);
  expect(screen.queryByLabelText("Close window")).not.toBeInTheDocument();
  expect(screen.getByLabelText("Selected window preview").querySelector("img")).toHaveAttribute(
    "src",
    "data:image/png;base64,large",
  );
  rerender(
    <DockPanelView
      state={{ ...s, appearance: { ...s.appearance, previewSelected: false } }}
      handlers={h}
    />,
  );
  expect(screen.queryByLabelText("Selected window preview")).not.toBeInTheDocument();
});
it("renders titles at the compact threshold and icons for the icon style", () => {
  const h = { onSelectWindow: vi.fn(), onFocusWindow: vi.fn(), onAction: vi.fn(), onSize: vi.fn() };
  const s = {
    ...state,
    entries: [
      {
        ...state.entries[0],
        thumbnail: "data:image/png;base64,thumb",
        icon: "data:image/png;base64,icon",
      },
    ],
    appearance: { ...state.appearance, style: "appIcons" as const, compactThreshold: 0 },
  };
  const { rerender } = render(<DockPanelView state={s} handlers={h} />);
  expect(
    screen.getByRole("button", { name: "Focus Document B" }).querySelector("img"),
  ).toHaveAttribute("src", "data:image/png;base64,icon");
  rerender(
    <DockPanelView
      state={{ ...s, appearance: { ...s.appearance, compactThreshold: 1 } }}
      handlers={h}
    />,
  );
  expect(screen.getByRole("button", { name: "Focus Document B" })).toHaveTextContent("Document B");
  expect(screen.getByRole("button", { name: "Focus Document B" }).querySelector("img")).toBeNull();
});
it("hides title text without removing an accessible focus target", () => {
  const h = { onSelectWindow: vi.fn(), onFocusWindow: vi.fn(), onAction: vi.fn(), onSize: vi.fn() };
  render(
    <DockPanelView
      state={{ ...state, appearance: { ...state.appearance, showTitle: false } }}
      handlers={h}
    />,
  );
  expect(screen.getByRole("button", { name: "Focus Document B" })).not.toHaveTextContent(
    "Document B",
  );
});

it("reports bounded content size, deduplicates observer notifications, and remeasures content changes", () => {
  const h = { onSelectWindow: vi.fn(), onFocusWindow: vi.fn(), onAction: vi.fn(), onSize: vi.fn() };
  let height = 160;
  const rect = vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(() => ({
    x: 0,
    y: 0,
    width: 320,
    height,
    top: 0,
    left: 0,
    right: 320,
    bottom: height,
    toJSON() {},
  }));
  const scroll = vi.spyOn(HTMLElement.prototype, "scrollHeight", "get").mockReturnValue(9999);
  let resize = () => {};
  vi.stubGlobal(
    "ResizeObserver",
    class {
      constructor(callback: () => void) {
        resize = callback;
      }
      observe() {}
      disconnect() {}
    },
  );
  try {
    const { rerender } = render(<DockPanelView state={state} handlers={h} />);
    expect(h.onSize).toHaveBeenLastCalledWith(7, 342, 182);
    resize();
    resize();
    expect(h.onSize).toHaveBeenCalledTimes(1);
    height = 180;
    rerender(<DockPanelView state={{ ...state, error: "Longer error" }} handlers={h} />);
    expect(h.onSize).toHaveBeenLastCalledWith(7, 342, 202);
    rerender(<DockPanelView state={{ ...state, session: 8 }} handlers={h} />);
    expect(h.onSize).toHaveBeenLastCalledWith(8, 342, 202);
  } finally {
    rect.mockRestore();
    scroll.mockRestore();
    vi.unstubAllGlobals();
  }
});

it("resizes the selected preview with configured and auto-sized thumbnails", () => {
  const h = { onSelectWindow: vi.fn(), onFocusWindow: vi.fn(), onAction: vi.fn(), onSize: vi.fn() };
  const entries = Array.from({ length: 8 }, (_, i) => ({
    ...state.entries[0],
    windowId: i + 1,
    preview: "data:image/png;base64,frame",
  }));
  const appearance = {
    ...state.appearance,
    previewSelected: true,
    thumbnailMaxPx: 240,
    maxColumns: 2,
    maxRows: 2,
    autoSize: false,
  };
  const s = { ...state, entries, selectedWindowId: 1, appearance };
  const { container, rerender } = render(<DockPanelView state={s} handlers={h} />);
  const height = () =>
    (container.querySelector(".ot-dock-panel") as HTMLElement).style.getPropertyValue(
      "--ot-dock-preview-height",
    );
  expect(height()).toBe("300px");
  rerender(
    <DockPanelView
      state={{ ...s, appearance: { ...appearance, thumbnailMaxPx: 120 } }}
      handlers={h}
    />,
  );
  expect(height()).toBe("150px");
  rerender(
    <DockPanelView state={{ ...s, appearance: { ...appearance, autoSize: true } }} handlers={h} />,
  );
  expect(height()).toBe("150px");
  rerender(
    <DockPanelView
      state={{ ...s, appearance: { ...appearance, thumbnailMaxPx: 1024 } }}
      handlers={h}
    />,
  );
  expect(height()).toBe("600px");
  rerender(
    <DockPanelView
      state={{ ...s, appearance: { ...appearance, thumbnailMaxPx: 64 } }}
      handlers={h}
    />,
  );
  expect(height()).toBe("120px");
  expect(screen.getByLabelText("Selected window preview").querySelector("img")).toHaveAttribute(
    "src",
    "data:image/png;base64,frame",
  );
});

it("uses zero and maximum card spacing in bounded-row geometry", () => {
  const h = {
    onSelectWindow: vi.fn(),
    onFocusWindow: vi.fn(),
    onAction: vi.fn(),
    onSize: vi.fn(),
  };
  const entries = Array.from({ length: 5 }, (_, index) => ({
    ...state.entries[0],
    windowId: index + 1,
  }));
  const appearance = {
    ...state.appearance,
    thumbnailMaxPx: 150,
    maxColumns: 2,
    maxRows: 2,
    autoSize: false,
    showTitle: false,
  };
  const { container, rerender } = render(
    <DockPanelView state={{ ...state, entries, appearance, cardSpacingPx: 0 }} handlers={h} />,
  );
  const panel = container.querySelector(".ot-dock-panel") as HTMLElement;
  const content = container.querySelector(".ot-dock-content") as HTMLElement;
  expect(panel.style.getPropertyValue("--ot-dock-spacing")).toBe("0px");
  expect(panel.style.getPropertyValue("--ot-dock-list-height")).toBe("212px");
  expect(content).toHaveStyle({ width: "324px" });
  rerender(
    <DockPanelView state={{ ...state, entries, appearance, cardSpacingPx: 24 }} handlers={h} />,
  );
  expect(panel.style.getPropertyValue("--ot-dock-spacing")).toBe("24px");
  expect(panel.style.getPropertyValue("--ot-dock-list-height")).toBe("236px");
  expect(content).toHaveStyle({ width: "348px" });
});

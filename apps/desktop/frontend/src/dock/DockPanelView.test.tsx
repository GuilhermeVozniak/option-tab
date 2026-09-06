import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
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

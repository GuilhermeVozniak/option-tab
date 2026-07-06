import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { SwitcherState } from "../lib/types";
import { emptyState } from "../lib/types";
import { Overlay } from "./Overlay";

function stateWith(overrides: Partial<SwitcherState>): SwitcherState {
  return {
    ...emptyState,
    open: true,
    entries: [
      {
        windowId: 1,
        appId: 1,
        title: "main.go",
        appName: "Editor",
        bundleId: "",
        minimized: false,
        hidden: false,
        fullscreen: false,
      },
      {
        windowId: 2,
        appId: 2,
        title: "GitHub",
        appName: "Browser",
        bundleId: "",
        minimized: false,
        hidden: false,
        fullscreen: false,
      },
    ],
    selected: 0,
    ...overrides,
  };
}

const noopHandlers = () => ({
  onAdvance: vi.fn(),
  onReverse: vi.fn(),
  onConfirm: vi.fn(),
  onCancel: vi.fn(),
  onSelect: vi.fn(),
  onSearchChange: vi.fn(),
  onClose: vi.fn(),
  onMinimize: vi.fn(),
  onFullscreen: vi.fn(),
  onQuit: vi.fn(),
  onHide: vi.fn(),
});

describe("Overlay", () => {
  it("renders nothing when closed", () => {
    const { container } = render(
      <Overlay state={{ ...emptyState, open: false }} handlers={noopHandlers()} />,
    );
    expect(container.firstChild).toBeNull();
  });

  it("renders each entry's window title in thumbnail mode", () => {
    render(<Overlay state={stateWith({})} handlers={noopHandlers()} />);
    expect(screen.getByText("main.go")).toBeInTheDocument();
    expect(screen.getByText("GitHub")).toBeInTheDocument();
  });

  it("renders app names in app-icon mode", () => {
    render(<Overlay state={stateWith({ style: "appIcons" })} handlers={noopHandlers()} />);
    expect(screen.getByText("Editor")).toBeInTheDocument();
    expect(screen.getByText("Browser")).toBeInTheDocument();
  });

  it("marks the selected entry", () => {
    render(<Overlay state={stateWith({ selected: 1 })} handlers={noopHandlers()} />);
    const selected = screen.getByRole("option", { selected: true });
    expect(selected).toHaveTextContent("GitHub");
  });

  it("exposes the active visual style", () => {
    const { container } = render(
      <Overlay state={stateWith({ style: "titles" })} handlers={noopHandlers()} />,
    );
    expect(container.querySelector('[data-style="titles"]')).not.toBeNull();
  });

  it("routes Tab/Shift+Tab/Escape/Enter to handlers", () => {
    const h = noopHandlers();
    render(<Overlay state={stateWith({})} handlers={h} />);
    fireEvent.keyDown(window, { key: "Tab" });
    expect(h.onAdvance).toHaveBeenCalled();
    fireEvent.keyDown(window, { key: "Tab", shiftKey: true });
    expect(h.onReverse).toHaveBeenCalled();
    fireEvent.keyDown(window, { key: "Escape" });
    expect(h.onCancel).toHaveBeenCalled();
    fireEvent.keyDown(window, { key: "Enter" });
    expect(h.onConfirm).toHaveBeenCalled();
  });

  it("routes held-modifier action keys to the selected entry", () => {
    const h = noopHandlers();
    render(<Overlay state={stateWith({ selected: 1 })} handlers={h} />);
    // Option+W closes the selected window (id 2 == second entry).
    fireEvent.keyDown(window, { key: "∑", code: "KeyW", altKey: true });
    expect(h.onClose).toHaveBeenCalledWith(2);
    fireEvent.keyDown(window, { key: "f", code: "KeyF", altKey: true });
    expect(h.onFullscreen).toHaveBeenCalledWith(2);
  });

  it("navigates with vim keys when enabled", () => {
    const h = noopHandlers();
    render(<Overlay state={stateWith({ vimKeys: true })} handlers={h} />);
    fireEvent.keyDown(window, { key: "j", code: "KeyJ" });
    expect(h.onAdvance).toHaveBeenCalled();
    fireEvent.keyDown(window, { key: "k", code: "KeyK" });
    expect(h.onReverse).toHaveBeenCalled();
  });

  it("types into the search query", () => {
    const h = noopHandlers();
    render(<Overlay state={stateWith({ search: "ab" })} handlers={h} />);
    fireEvent.keyDown(window, { key: "c" });
    expect(h.onSearchChange).toHaveBeenCalledWith("abc");
    fireEvent.keyDown(window, { key: "Backspace" });
    expect(h.onSearchChange).toHaveBeenCalledWith("a");
  });

  it("shows the current search text", () => {
    render(<Overlay state={stateWith({ search: "term" })} handlers={noopHandlers()} />);
    expect(screen.getByText(/term/)).toBeInTheDocument();
  });

  it("selects on hover and confirms on click", () => {
    const h = noopHandlers();
    render(<Overlay state={stateWith({})} handlers={h} />);
    const second = screen.getByText("GitHub").closest('[role="option"]') as HTMLElement;
    fireEvent.mouseEnter(second);
    expect(h.onSelect).toHaveBeenCalledWith(1);
    fireEvent.click(second);
    expect(h.onConfirm).toHaveBeenCalled();
  });

  it("fires window controls without confirming", () => {
    const h = noopHandlers();
    render(<Overlay state={stateWith({ selected: 0 })} handlers={h} />);
    fireEvent.click(screen.getAllByLabelText("Close window")[0]);
    expect(h.onClose).toHaveBeenCalledWith(1);
    expect(h.onConfirm).not.toHaveBeenCalled();
    fireEvent.click(screen.getAllByLabelText("Minimize window")[0]);
    expect(h.onMinimize).toHaveBeenCalledWith(1);
    fireEvent.click(screen.getAllByLabelText("Fullscreen window")[0]);
    expect(h.onFullscreen).toHaveBeenCalledWith(1);
    fireEvent.click(screen.getAllByLabelText("Hide app")[0]);
    expect(h.onHide).toHaveBeenCalledWith(1);
    fireEvent.click(screen.getAllByLabelText("Quit app")[0]);
    expect(h.onQuit).toHaveBeenCalledWith(1);
    expect(h.onConfirm).not.toHaveBeenCalled();
  });

  it("drops the frosted-glass blur class when appearance.blur is off", () => {
    const on = stateWith({});
    on.appearance = { ...on.appearance, blur: true };
    const { container, rerender } = render(<Overlay state={on} handlers={noopHandlers()} />);
    expect(container.querySelector(".ot-overlay")?.classList.contains("ot-no-blur")).toBe(false);

    const off = stateWith({});
    off.appearance = { ...off.appearance, blur: false };
    rerender(<Overlay state={off} handlers={noopHandlers()} />);
    expect(container.querySelector(".ot-overlay")?.classList.contains("ot-no-blur")).toBe(true);
  });

  it("hides window controls when disabled in appearance", () => {
    const s = stateWith({});
    s.appearance = { ...s.appearance, showWindowControls: false };
    render(<Overlay state={s} handlers={noopHandlers()} />);
    expect(screen.queryByLabelText("Close window")).toBeNull();
  });

  it("renders status icons for minimized and other-Space windows", () => {
    const s = stateWith({ activeSpaceId: 1 });
    s.entries = [
      { ...s.entries[0], minimized: true },
      { ...s.entries[1], spaceId: 2 },
    ];
    render(<Overlay state={s} handlers={noopHandlers()} />);
    expect(screen.getByLabelText("Minimized")).toBeInTheDocument();
    expect(screen.getByLabelText("On another Space")).toBeInTheDocument();
  });

  it("hides status icons when disabled in appearance", () => {
    const s = stateWith({});
    s.entries = [{ ...s.entries[0], minimized: true }, s.entries[1]];
    s.appearance = { ...s.appearance, showStatusIcons: false };
    render(<Overlay state={s} handlers={noopHandlers()} />);
    expect(screen.queryByLabelText("Minimized")).toBeNull();
  });

  it("does not select on hover when mouseHover is off", () => {
    const h = noopHandlers();
    render(<Overlay state={stateWith({ mouseHover: false })} handlers={h} />);
    const second = screen.getByText("GitHub").closest('[role="option"]') as HTMLElement;
    fireEvent.mouseEnter(second);
    expect(h.onSelect).not.toHaveBeenCalled();
  });

  it("shows Space number badges when windows span multiple Spaces", () => {
    const s = stateWith({ activeSpaceId: 1 });
    s.entries = [
      { ...s.entries[0], spaceId: 1 },
      { ...s.entries[1], spaceId: 5 },
    ];
    render(<Overlay state={s} handlers={noopHandlers()} />);
    // Ordinals are 1..N over the sorted distinct space ids (1->1, 5->2).
    expect(screen.getByLabelText("Space 1")).toBeInTheDocument();
    expect(screen.getByLabelText("Space 2")).toBeInTheDocument();
  });

  it("hides Space number badges when disabled", () => {
    const s = stateWith({ activeSpaceId: 1 });
    s.entries = [
      { ...s.entries[0], spaceId: 1 },
      { ...s.entries[1], spaceId: 5 },
    ];
    s.appearance = { ...s.appearance, showSpaceNumbers: false };
    render(<Overlay state={s} handlers={noopHandlers()} />);
    expect(screen.queryByLabelText("Space 1")).toBeNull();
  });

  it("renders a preview of the selected window when enabled", () => {
    const s = stateWith({ selected: 0 });
    s.entries = [{ ...s.entries[0], thumbnail: "data:image/png;base64,x" }, s.entries[1]];
    s.appearance = { ...s.appearance, previewSelected: true };
    render(<Overlay state={s} handlers={noopHandlers()} />);
    expect(screen.getByLabelText("Selected window preview")).toBeInTheDocument();
  });

  it("prefers the high-resolution preview capture over the thumbnail", () => {
    const s = stateWith({ selected: 0 });
    s.entries = [
      {
        ...s.entries[0],
        thumbnail: "data:image/png;base64,small",
        preview: "data:image/png;base64,big",
      },
      s.entries[1],
    ];
    s.appearance = { ...s.appearance, previewSelected: true };
    render(<Overlay state={s} handlers={noopHandlers()} />);
    const img = screen
      .getByLabelText("Selected window preview")
      .querySelector("img") as HTMLImageElement;
    expect(img.src).toContain("big");
  });

  it("truncates long titles in the middle when configured", () => {
    const long = `left-${"x".repeat(80)}-right`;
    const s = stateWith({});
    s.entries = [{ ...s.entries[0], title: long }, s.entries[1]];
    s.appearance = { ...s.appearance, titleTruncation: "middle" };
    render(<Overlay state={s} handlers={noopHandlers()} />);
    const el = screen.getByText(/left-.*….*-right/);
    expect(el.textContent?.length).toBeLessThan(long.length);
  });

  it("delays apparition when apparitionDelayMs is set", () => {
    vi.useFakeTimers();
    try {
      const s = stateWith({});
      s.appearance = { ...s.appearance, apparitionDelayMs: 300 };
      const { container } = render(<Overlay state={s} handlers={noopHandlers()} />);
      expect(container.querySelector(".ot-overlay")).toBeNull();
      act(() => {
        vi.advanceTimersByTime(310);
      });
      expect(container.querySelector(".ot-overlay")).not.toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });

  it("keeps a closing frame mounted for the fade-out animation", () => {
    vi.useFakeTimers();
    try {
      const s = stateWith({});
      const { container, rerender } = render(<Overlay state={s} handlers={noopHandlers()} />);
      expect(container.querySelector(".ot-overlay")).not.toBeNull();
      rerender(<Overlay state={{ ...s, open: false }} handlers={noopHandlers()} />);
      expect(container.querySelector(".ot-closing")).not.toBeNull();
      act(() => {
        vi.advanceTimersByTime(200);
      });
      expect(container.querySelector(".ot-overlay")).toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });

  it("navigates with arrow keys only when arrowKeys is enabled", () => {
    const h = noopHandlers();
    const { rerender } = render(<Overlay state={stateWith({ arrowKeys: true })} handlers={h} />);
    fireEvent.keyDown(window, { key: "ArrowRight" });
    expect(h.onAdvance).toHaveBeenCalledTimes(1);
    fireEvent.keyDown(window, { key: "ArrowLeft" });
    expect(h.onReverse).toHaveBeenCalledTimes(1);

    rerender(<Overlay state={stateWith({ arrowKeys: false })} handlers={h} />);
    fireEvent.keyDown(window, { key: "ArrowRight" });
    fireEvent.keyDown(window, { key: "ArrowLeft" });
    expect(h.onAdvance).toHaveBeenCalledTimes(1);
    expect(h.onReverse).toHaveBeenCalledTimes(1);
    // Tab keeps working with arrows disabled.
    fireEvent.keyDown(window, { key: "Tab" });
    expect(h.onAdvance).toHaveBeenCalledTimes(2);
  });

  it("cancels on backdrop click but not on panel click", () => {
    const h = noopHandlers();
    const { container } = render(<Overlay state={stateWith({})} handlers={h} />);
    fireEvent.click(container.querySelector(".ot-overlay") as HTMLElement);
    expect(h.onCancel).toHaveBeenCalledTimes(1);
    // A click inside the panel bubbles up but target !== currentTarget.
    fireEvent.click(container.querySelector(".ot-panel") as HTMLElement);
    expect(h.onCancel).toHaveBeenCalledTimes(1);
  });

  it("routes modifier M/Q/H to minimize, quit, and hide for the selected entry", () => {
    const h = noopHandlers();
    render(<Overlay state={stateWith({ selected: 1 })} handlers={h} />);
    // Option mangles the character, so routing matches on e.code.
    fireEvent.keyDown(window, { key: "µ", code: "KeyM", altKey: true });
    expect(h.onMinimize).toHaveBeenCalledWith(2);
    fireEvent.keyDown(window, { key: "œ", code: "KeyQ", altKey: true });
    expect(h.onQuit).toHaveBeenCalledWith(2);
    fireEvent.keyDown(window, { key: "˙", code: "KeyH", altKey: true });
    expect(h.onHide).toHaveBeenCalledWith(2);
    expect(h.onConfirm).not.toHaveBeenCalled();
  });

  it("omits the preview pane when the selected entry has no image source", () => {
    const s = stateWith({ selected: 0 });
    s.appearance = { ...s.appearance, previewSelected: true };
    render(<Overlay state={s} handlers={noopHandlers()} />);
    expect(screen.queryByLabelText("Selected window preview")).toBeNull();
  });

  it("renders compact rows with app name and title in titles mode", () => {
    const { container } = render(
      <Overlay state={stateWith({ style: "titles" })} handlers={noopHandlers()} />,
    );
    const rows = container.querySelectorAll(".ot-entry-titles");
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent("Editor");
    expect(rows[0]).toHaveTextContent("main.go");
    expect(rows[1]).toHaveTextContent("Browser");
    expect(rows[1]).toHaveTextContent("GitHub");
    expect(container.querySelector(".ot-thumb")).toBeNull();
  });

  it("renders the icon image when set and an initial glyph otherwise", () => {
    const s = stateWith({ style: "appIcons" });
    s.entries = [
      { ...s.entries[0], icon: "data:image/png;base64,ICON" },
      { ...s.entries[1], appName: "editor", icon: "" },
    ];
    const { container } = render(<Overlay state={s} handlers={noopHandlers()} />);
    const img = container.querySelector("img.ot-icon-img") as HTMLImageElement;
    expect(img).not.toBeNull();
    expect(img.src).toContain("data:image/png;base64,ICON");
    const glyphs = container.querySelectorAll(".ot-icon");
    expect(glyphs[1]).toHaveTextContent("E");
  });

  it("applies the theme class to the overlay root", () => {
    const dark = stateWith({});
    dark.appearance = { ...dark.appearance, theme: "dark" };
    const { container, rerender } = render(<Overlay state={dark} handlers={noopHandlers()} />);
    expect(container.querySelector(".ot-overlay")?.classList.contains("ot-theme-dark")).toBe(true);
    const light = stateWith({});
    light.appearance = { ...light.appearance, theme: "light" };
    rerender(<Overlay state={light} handlers={noopHandlers()} />);
    expect(container.querySelector(".ot-overlay")?.classList.contains("ot-theme-light")).toBe(true);
  });

  it("wires appearance knobs to CSS variables on the root", () => {
    const s = stateWith({});
    s.appearance = {
      ...s.appearance,
      accentColor: "#ff0000",
      backgroundOpacity: 0.5,
      cornerRadiusPx: 20,
      fontSizePx: 15,
    };
    const { container } = render(<Overlay state={s} handlers={noopHandlers()} />);
    const root = container.querySelector(".ot-overlay") as HTMLElement;
    expect(root.style.getPropertyValue("--ot-accent")).toBe("#ff0000");
    expect(root.style.getPropertyValue("--ot-bg-opacity")).toBe("0.5");
    expect(root.style.getPropertyValue("--ot-radius")).toBe("20px");
    expect(root.style.getPropertyValue("--ot-font")).toBe("15px");
  });

  it("hides titles in every style when showTitle is off", () => {
    const s = stateWith({});
    s.appearance = { ...s.appearance, showTitle: false };
    const { container, rerender } = render(<Overlay state={s} handlers={noopHandlers()} />);
    expect(container.querySelector(".ot-titlebar")).toBeNull();
    expect(screen.queryByText("main.go")).toBeNull();
    const icons = stateWith({ style: "appIcons" });
    icons.appearance = { ...icons.appearance, showTitle: false };
    rerender(<Overlay state={icons} handlers={noopHandlers()} />);
    expect(container.querySelector(".ot-meta")).toBeNull();
  });

  it("renders status icons for hidden and fullscreen windows", () => {
    const s = stateWith({});
    s.entries = [
      { ...s.entries[0], hidden: true },
      { ...s.entries[1], fullscreen: true },
    ];
    render(<Overlay state={s} handlers={noopHandlers()} />);
    expect(screen.getByLabelText("Hidden app")).toBeInTheDocument();
    expect(screen.getByLabelText("Fullscreen")).toBeInTheDocument();
  });

  it("unmounts immediately without a closing frame when fadeOutAnimation is off", () => {
    const s = stateWith({});
    s.appearance = { ...s.appearance, fadeOutAnimation: false };
    const { container, rerender } = render(<Overlay state={s} handlers={noopHandlers()} />);
    expect(container.querySelector(".ot-overlay")).not.toBeNull();
    rerender(<Overlay state={{ ...s, open: false }} handlers={noopHandlers()} />);
    expect(container.querySelector(".ot-overlay")).toBeNull();
    expect(container.querySelector(".ot-closing")).toBeNull();
  });

  it("relayouts thumbnails via the window resize fallback when the viewport shrinks", () => {
    // jsdom has no ResizeObserver, so the Overlay must fall back to the
    // window "resize" listener — the path the first-show relayout relies on.
    expect(typeof ResizeObserver).toBe("undefined");
    const originalWidth = window.innerWidth;
    const dozen = Array.from({ length: 12 }, (_, i) => ({
      windowId: i + 1,
      appId: i + 1,
      title: `win-${i}`,
      appName: `App ${i}`,
      bundleId: "",
      minimized: false,
      hidden: false,
      fullscreen: false,
    }));
    try {
      const { container } = render(
        <Overlay state={stateWith({ entries: dozen })} handlers={noopHandlers()} />,
      );
      const thumbWidth = () =>
        parseInt((container.querySelector(".ot-thumb") as HTMLElement).style.width, 10);
      const before = thumbWidth();
      Object.defineProperty(window, "innerWidth", {
        configurable: true,
        writable: true,
        value: Math.round(originalWidth / 3),
      });
      fireEvent(window, new Event("resize"));
      expect(thumbWidth()).toBeLessThan(before);
    } finally {
      Object.defineProperty(window, "innerWidth", {
        configurable: true,
        writable: true,
        value: originalWidth,
      });
    }
  });
});

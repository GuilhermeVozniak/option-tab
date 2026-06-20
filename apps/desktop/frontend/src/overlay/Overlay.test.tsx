import { fireEvent, render, screen } from "@testing-library/react";
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

  it("renders all entries with titles and app names", () => {
    render(<Overlay state={stateWith({})} handlers={noopHandlers()} />);
    expect(screen.getByText("main.go")).toBeInTheDocument();
    expect(screen.getByText("GitHub")).toBeInTheDocument();
    expect(screen.getByText("Editor")).toBeInTheDocument();
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
  });

  it("hides window controls when disabled in appearance", () => {
    const s = stateWith({});
    s.appearance = { ...s.appearance, showWindowControls: false };
    render(<Overlay state={s} handlers={noopHandlers()} />);
    expect(screen.queryByLabelText("Close window")).toBeNull();
  });
});

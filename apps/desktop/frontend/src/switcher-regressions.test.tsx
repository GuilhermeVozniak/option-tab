import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { AppSwitcher } from "./app-switcher/AppSwitcher";
import { emptyState } from "./lib/types";
import { Overlay } from "./overlay/Overlay";

function handlers() {
  return {
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
  };
}

describe("switcher input and style regressions", () => {
  it("honors the backend's per-shortcut app style override", () => {
    render(
      <AppSwitcher
        nativeKeys={false}
        handlers={handlers()}
        state={{
          ...emptyState,
          mode: "apps",
          open: true,
          style: "titles",
          appearance: { ...emptyState.appearance, style: "thumbnails" },
        }}
      />,
    );
    expect(screen.getByRole("dialog")).toHaveClass("style-titles");
  });

  for (const [name, Component] of [
    ["windows", Overlay],
    ["apps", AppSwitcher],
  ] as const) {
    it(`${name}: preserves characters typed before a backend state reply`, () => {
      const h = handlers();
      render(<Component nativeKeys={false} handlers={h} state={{ ...emptyState, open: true }} />);
      fireEvent.keyDown(window, { key: "a", code: "KeyA" });
      fireEvent.keyDown(window, { key: "b", code: "KeyB" });
      expect(h.onSearchChange.mock.calls.map(([query]) => query)).toEqual(["a", "ab"]);
    });
    it(`${name}: keeps pending edits through older echoes and resets on a new session`, () => {
      const h = handlers();
      const state = { ...emptyState, open: true, session: 4 };
      const { rerender } = render(<Component nativeKeys={false} handlers={h} state={state} />);
      fireEvent.keyDown(window, { key: "a", code: "KeyA" });
      fireEvent.keyDown(window, { key: "b", code: "KeyB" });
      rerender(
        <Component
          nativeKeys={false}
          handlers={h}
          state={{ ...state, search: "a", searchSequence: 1 }}
        />,
      );
      fireEvent.keyDown(window, { key: "c", code: "KeyC" });
      expect(h.onSearchChange).toHaveBeenLastCalledWith("abc", 4, 3);
      fireEvent.keyDown(window, { key: "Backspace" });
      fireEvent.keyDown(window, { key: "Backspace" });
      expect(h.onSearchChange).toHaveBeenLastCalledWith("a", 4, 5);
      rerender(<Component nativeKeys={false} handlers={h} state={{ ...state, session: 5 }} />);
      fireEvent.keyDown(window, { key: "x", code: "KeyX" });
      expect(h.onSearchChange).toHaveBeenLastCalledWith("x", 5, 1);
    });
  }
});

import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { defaultSettings } from "../lib/types";
import { Settings } from "./Settings";

describe("Settings navigation and presentation", () => {
  beforeEach(() => localStorage.clear());

  it("changes and remembers the Settings theme without changing switcher settings", () => {
    const onChange = vi.fn();
    const { container, unmount } = render(
      <Settings settings={defaultSettings} onChange={onChange} />,
    );
    expect(container.querySelector(".ot-settings")).toHaveAttribute("data-theme", "system");
    fireEvent.click(screen.getByRole("button", { name: "Settings theme light" }));
    expect(container.querySelector(".ot-settings")).toHaveAttribute("data-theme", "light");
    expect(onChange).not.toHaveBeenCalled();
    unmount();
    const reopened = render(<Settings settings={defaultSettings} onChange={onChange} />);
    expect(reopened.container.querySelector(".ot-settings")).toHaveAttribute("data-theme", "light");
    fireEvent.click(screen.getByRole("button", { name: "Settings theme dark" }));
    expect(reopened.container.querySelector(".ot-settings")).toHaveAttribute("data-theme", "dark");
    expect(onChange).not.toHaveBeenCalled();
  });

  it("ignores invalid saved themes", () => {
    localStorage.setItem("option-tab.settings-theme", "invalid");
    const { container } = render(<Settings settings={defaultSettings} onChange={vi.fn()} />);
    expect(container.querySelector(".ot-settings")).toHaveAttribute("data-theme", "system");
  });

  it("connects sidebar tabs to one exposed panel and supports keyboard navigation", () => {
    render(<Settings settings={defaultSettings} onChange={vi.fn()} />);
    const general = screen.getByRole("tab", { name: "General" });
    const shortcuts = screen.getByRole("tab", { name: "Shortcuts" });
    general.focus();
    fireEvent.keyDown(general, { key: "ArrowDown" });
    expect(shortcuts).toHaveFocus();
    expect(shortcuts).toHaveAttribute("aria-selected", "true");
    expect(screen.getAllByRole("tabpanel")).toHaveLength(1);
    expect(screen.getByRole("tabpanel")).toHaveAttribute(
      "id",
      shortcuts.getAttribute("aria-controls"),
    );
    fireEvent.keyDown(shortcuts, { key: "End" });
    expect(screen.getByRole("tab", { name: "About" })).toHaveFocus();
    fireEvent.keyDown(document.activeElement!, { key: "Home" });
    expect(general).toHaveFocus();
  });

  it("preserves an unsaved excluded-app draft when navigating away", () => {
    render(<Settings settings={defaultSettings} onChange={vi.fn()} />);
    fireEvent.click(screen.getByRole("tab", { name: "Excluded apps" }));
    fireEvent.click(screen.getByRole("button", { name: "+ Add app" }));
    fireEvent.change(screen.getByLabelText("Blacklist entry 1", { exact: true }), {
      target: { value: "com.example.draft" },
    });
    fireEvent.click(screen.getByRole("tab", { name: "General" }));
    fireEvent.click(screen.getByRole("tab", { name: "Excluded apps" }));
    expect(screen.getByLabelText("Blacklist entry 1", { exact: true })).toHaveValue(
      "com.example.draft",
    );
  });

  it("retains existing deep links with the new navigation labels", () => {
    render(<Settings settings={defaultSettings} onChange={vi.fn()} requestedTab="Controls" />);
    expect(screen.getByRole("tab", { name: "Shortcuts" })).toHaveAttribute("aria-selected", "true");
    expect(
      within(screen.getByRole("tabpanel")).getByRole("heading", { name: "Shortcuts" }),
    ).toBeVisible();
  });
});

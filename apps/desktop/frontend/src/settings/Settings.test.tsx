import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { defaultSettings } from "../lib/types";
import { Settings } from "./Settings";

describe("Settings", () => {
  it("renders current values", () => {
    render(<Settings settings={defaultSettings} onChange={vi.fn()} />);
    expect((screen.getByLabelText("Visual style") as HTMLSelectElement).value).toBe("thumbnails");
    expect((screen.getByLabelText("Hold modifier to cycle") as HTMLInputElement).checked).toBe(
      true,
    );
  });

  it("emits changes for the visual style", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    fireEvent.change(screen.getByLabelText("Visual style"), { target: { value: "titles" } });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ appearance: expect.objectContaining({ style: "titles" }) }),
    );
  });

  it("toggles behavior flags", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    fireEvent.click(screen.getByLabelText("Start at login"));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ behavior: expect.objectContaining({ startAtLogin: true }) }),
    );
  });

  it("edits a shortcut chord", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    const chord = screen.getByLabelText("Shortcut 1 chord");
    fireEvent.change(chord, { target: { value: "control+tab" } });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        shortcuts: expect.arrayContaining([
          expect.objectContaining({ id: 1, chord: "control+tab" }),
        ]),
      }),
    );
  });

  it("changes the display order", () => {
    const onChange = vi.fn();
    render(<Settings settings={defaultSettings} onChange={onChange} />);
    fireEvent.change(screen.getByLabelText("Display order"), { target: { value: "alphabetical" } });
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ order: "alphabetical" }));
  });
});

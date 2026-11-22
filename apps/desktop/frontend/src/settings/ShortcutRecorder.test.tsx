import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { system } from "../lib/bridge";
import { ShortcutRecorder } from "./ShortcutRecorder";

vi.mock("../lib/bridge", () => ({
  system: {
    captureShortcut: vi.fn(),
    cancelShortcutCapture: vi.fn(),
  },
}));

const nativeCapture = vi.mocked(system);

function renderRecorder() {
  const onChordChange = vi.fn();
  render(
    <ShortcutRecorder
      aria-label="Recorder chord"
      value=""
      placeholder="Record shortcut"
      onChordChange={onChordChange}
    />,
  );
  return { input: screen.getByLabelText("Recorder chord"), onChordChange };
}

describe("ShortcutRecorder", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("records the chord delivered by native capture on focus", async () => {
    nativeCapture.captureShortcut.mockResolvedValue("command+tab");
    const { input, onChordChange } = renderRecorder();
    await act(async () => {
      fireEvent.focus(input);
    });
    expect(nativeCapture.captureShortcut).toHaveBeenCalledTimes(1);
    expect(onChordChange).toHaveBeenCalledWith("command+tab");
  });

  it("falls back to DOM key events when native capture is unavailable", async () => {
    nativeCapture.captureShortcut.mockResolvedValue(null);
    const { input, onChordChange } = renderRecorder();
    await act(async () => {
      fireEvent.focus(input);
    });
    // The null resolution records nothing; the keydown fallback does.
    expect(onChordChange).not.toHaveBeenCalled();
    fireEvent.keyDown(input, { code: "Tab", altKey: true });
    expect(onChordChange).toHaveBeenCalledWith("option+tab");
  });

  it("cancels a still-pending native capture on blur", async () => {
    nativeCapture.captureShortcut.mockReturnValue(new Promise(() => {}));
    const { input, onChordChange } = renderRecorder();
    await act(async () => {
      fireEvent.focus(input);
    });
    fireEvent.blur(input);
    expect(nativeCapture.cancelShortcutCapture).toHaveBeenCalledTimes(1);
    expect(onChordChange).not.toHaveBeenCalled();
  });

  // Native capture gives up after 10 seconds (and ends after each recorded
  // chord). The field keeps focus, so no new focus event will ever re-arm it:
  // clicking the field again must.
  it("re-arms native capture when the still-focused field is clicked", async () => {
    nativeCapture.captureShortcut.mockResolvedValueOnce("").mockResolvedValueOnce("command+tab");
    const { input, onChordChange } = renderRecorder();
    await act(async () => {
      fireEvent.focus(input);
    });
    expect(onChordChange).not.toHaveBeenCalled();
    await act(async () => {
      fireEvent.click(input);
    });
    expect(nativeCapture.captureShortcut).toHaveBeenCalledTimes(2);
    expect(onChordChange).toHaveBeenCalledWith("command+tab");
  });

  it("does not arm a second capture while one is pending", async () => {
    nativeCapture.captureShortcut.mockReturnValue(new Promise(() => {}));
    const { input } = renderRecorder();
    await act(async () => {
      fireEvent.focus(input);
      fireEvent.click(input);
    });
    expect(nativeCapture.captureShortcut).toHaveBeenCalledTimes(1);
  });
});

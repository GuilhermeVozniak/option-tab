import { act, fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { FormDisabledProvider } from "./form-disabled";
import { NumberInput } from "./number-input";

// A real keystroke is an InputEvent carrying an inputType; the native stepper
// arrows dispatch a plain input event without one.
function typeText(input: HTMLElement, value: string) {
  fireEvent.input(input, { target: { value }, inputType: "insertText" });
}

// Mirrors the backend: every committed value is clamped and echoed back.
function Echo({
  initial,
  min,
  max,
  step,
  onCommit,
}: {
  initial: number;
  min: number;
  max: number;
  step?: number;
  onCommit: (value: number) => void;
}) {
  const [value, setValue] = useState(initial);
  return (
    <NumberInput
      aria-label="Size"
      min={min}
      max={max}
      step={step}
      value={value}
      onCommit={(next) => {
        onCommit(next);
        setValue(Math.min(max, Math.max(min, next)));
      }}
    />
  );
}

describe("NumberInput", () => {
  it("renders a native number input and forwards its label and bounds", () => {
    render(
      <NumberInput
        aria-label="Size"
        min={64}
        max={1024}
        step={2}
        value={280}
        onCommit={() => {}}
      />,
    );
    const input = screen.getByRole("spinbutton", { name: "Size" });
    expect(input).toHaveAttribute("type", "number");
    expect(input).toHaveAttribute("min", "64");
    expect(input).toHaveAttribute("max", "1024");
    expect(input).toHaveAttribute("step", "2");
    expect(input).toHaveAttribute("data-slot", "input");
    expect(input).toHaveValue(280);
  });

  it("holds typed digits locally and commits once on blur", () => {
    const onCommit = vi.fn();
    render(<Echo initial={280} min={64} max={1024} onCommit={onCommit} />);
    const input = screen.getByRole("spinbutton", { name: "Size" });
    typeText(input, "3");
    expect(input).toHaveValue(3);
    typeText(input, "30");
    typeText(input, "300");
    expect(onCommit).not.toHaveBeenCalled();
    fireEvent.blur(input);
    expect(onCommit).toHaveBeenCalledTimes(1);
    expect(onCommit).toHaveBeenCalledWith(300);
    expect(input).toHaveValue(300);
  });

  it("commits on Enter and restores the saved value on Escape", () => {
    const onCommit = vi.fn();
    render(<Echo initial={2} min={1} max={20} onCommit={onCommit} />);
    const input = screen.getByRole("spinbutton", { name: "Size" });
    typeText(input, "7");
    fireEvent.keyDown(input, { key: "Enter" });
    expect(onCommit).toHaveBeenCalledTimes(1);
    expect(onCommit).toHaveBeenCalledWith(7);
    typeText(input, "15");
    fireEvent.keyDown(input, { key: "Escape" });
    expect(input).toHaveValue(7);
    fireEvent.blur(input);
    expect(onCommit).toHaveBeenCalledTimes(1);
  });

  it("clamps a committed draft to its own bounds", () => {
    const onCommit = vi.fn();
    render(<Echo initial={280} min={64} max={1024} onCommit={onCommit} />);
    const input = screen.getByRole("spinbutton", { name: "Size" });
    typeText(input, "5000");
    fireEvent.blur(input);
    expect(onCommit).toHaveBeenLastCalledWith(1024);
    expect(input).toHaveValue(1024);
    typeText(input, "3");
    fireEvent.blur(input);
    expect(onCommit).toHaveBeenLastCalledWith(64);
    expect(input).toHaveValue(64);
    // Already at the bound: nothing new to save, and the draft is dropped.
    typeText(input, "1");
    fireEvent.blur(input);
    expect(onCommit).toHaveBeenCalledTimes(2);
    expect(input).toHaveValue(64);
  });

  it("reverts an empty or non-numeric draft instead of committing 0 or NaN", () => {
    const onCommit = vi.fn();
    render(<Echo initial={12} min={1} max={20} onCommit={onCommit} />);
    const input = screen.getByRole("spinbutton", { name: "Size" });
    fireEvent.input(input, { target: { value: "" }, inputType: "deleteContentBackward" });
    expect(input).toHaveValue(null);
    fireEvent.blur(input);
    expect(onCommit).not.toHaveBeenCalled();
    expect(input).toHaveValue(12);
    fireEvent.input(input, { target: { value: "" }, inputType: "deleteContentBackward" });
    fireEvent.keyDown(input, { key: "Enter" });
    expect(onCommit).not.toHaveBeenCalled();
    expect(input).toHaveValue(12);
  });

  it("rounds whole-number fields and keeps fractions for fractional steps", () => {
    const whole = vi.fn();
    const { unmount } = render(<Echo initial={40} min={24} max={64} onCommit={whole} />);
    typeText(screen.getByRole("spinbutton", { name: "Size" }), "40.6");
    fireEvent.blur(screen.getByRole("spinbutton", { name: "Size" }));
    expect(whole).toHaveBeenCalledWith(41);
    unmount();
    const fraction = vi.fn();
    render(<Echo initial={0.8} min={0.25} max={0.9} step={0.05} onCommit={fraction} />);
    typeText(screen.getByRole("spinbutton", { name: "Size" }), "0.62");
    fireEvent.blur(screen.getByRole("spinbutton", { name: "Size" }));
    expect(fraction).toHaveBeenCalledWith(0.62);
  });

  it("commits native stepper changes immediately", () => {
    const onCommit = vi.fn();
    render(<Echo initial={40} min={24} max={64} onCommit={onCommit} />);
    const input = screen.getByRole("spinbutton", { name: "Size" });
    // The stepper arrows and Arrow keys dispatch input events with no inputType.
    fireEvent.input(input, { target: { value: "41" } });
    expect(onCommit).toHaveBeenLastCalledWith(41);
    fireEvent.keyDown(input, { key: "ArrowUp" });
    fireEvent.input(input, { target: { value: "42" } });
    fireEvent.keyUp(input, { key: "ArrowUp" });
    expect(onCommit).toHaveBeenLastCalledWith(42);
    expect(input).toHaveValue(42);
  });

  it("never commits per keystroke even if the engine omits inputType while a key is down", () => {
    const onCommit = vi.fn();
    render(<Echo initial={280} min={64} max={1024} onCommit={onCommit} />);
    const input = screen.getByRole("spinbutton", { name: "Size" });
    fireEvent.keyDown(input, { key: "3" });
    fireEvent.input(input, { target: { value: "3" } });
    fireEvent.keyUp(input, { key: "3" });
    expect(onCommit).not.toHaveBeenCalled();
    expect(input).toHaveValue(3);
  });

  it("follows the saved value when it changes and honours disabled forms", () => {
    const { rerender } = render(
      <FormDisabledProvider disabled>
        <NumberInput aria-label="Size" value={10} onCommit={() => {}} />
      </FormDisabledProvider>,
    );
    expect(screen.getByRole("spinbutton", { name: "Size" })).toBeDisabled();
    rerender(
      <FormDisabledProvider disabled={false}>
        <NumberInput aria-label="Size" value={14} onCommit={() => {}} />
      </FormDisabledProvider>,
    );
    expect(screen.getByRole("spinbutton", { name: "Size" })).toBeEnabled();
    expect(screen.getByRole("spinbutton", { name: "Size" })).toHaveValue(14);
  });
  // The switcher hotkey never takes focus from Preferences, so a typed value
  // has to reach the saved settings without a blur.
  it("saves a typed value after a short pause without disturbing the text", () => {
    vi.useFakeTimers();
    try {
      const onCommit = vi.fn();
      render(<Echo initial={280} min={64} max={1024} onCommit={onCommit} />);
      const input = screen.getByRole("spinbutton", { name: "Size" });
      typeText(input, "3");
      act(() => void vi.advanceTimersByTime(300));
      typeText(input, "30");
      expect(onCommit).not.toHaveBeenCalled();
      act(() => void vi.advanceTimersByTime(700));
      // 30 is below the minimum: the clamped value is saved, the text stays.
      expect(onCommit).toHaveBeenCalledExactlyOnceWith(64);
      expect(input).toHaveValue(30);
      typeText(input, "300");
      act(() => void vi.advanceTimersByTime(700));
      expect(onCommit).toHaveBeenLastCalledWith(300);
      expect(input).toHaveValue(300);
      fireEvent.blur(input);
      expect(onCommit).toHaveBeenCalledTimes(2);
    } finally {
      vi.useRealTimers();
    }
  });

  it("does not save a pending draft after Escape or after the field is gone", () => {
    vi.useFakeTimers();
    try {
      const onCommit = vi.fn();
      const { unmount } = render(<Echo initial={280} min={64} max={1024} onCommit={onCommit} />);
      const input = screen.getByRole("spinbutton", { name: "Size" });
      typeText(input, "300");
      fireEvent.keyDown(input, { key: "Escape" });
      act(() => void vi.advanceTimersByTime(2000));
      typeText(input, "400");
      unmount();
      act(() => void vi.advanceTimersByTime(2000));
      expect(onCommit).not.toHaveBeenCalled();
    } finally {
      vi.useRealTimers();
    }
  });

  // Hiding or deactivating the window does not blur the field in every engine;
  // the draft must not be left on screen looking like a saved value.
  it("commits the draft when the window loses focus", () => {
    const onCommit = vi.fn();
    render(<Echo initial={280} min={64} max={1024} onCommit={onCommit} />);
    const input = screen.getByRole("spinbutton", { name: "Size" });
    typeText(input, "5000");
    fireEvent.blur(window);
    expect(onCommit).toHaveBeenCalledExactlyOnceWith(1024);
    expect(input).toHaveValue(1024);
  });
});

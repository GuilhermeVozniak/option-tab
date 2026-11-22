import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { expect, it, vi } from "vitest";
import { makeT } from "../../lib/i18n";
import { type Appearance, defaultSettings } from "../../lib/types";
import type { TabContext } from "../shared";
import { AppearanceTab } from "./AppearanceTab";

const clamp = (value: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, value));

// Mirrors Go's Normalize: every saved appearance is clamped and echoed back.
function canonical(a: Appearance): Appearance {
  return {
    ...a,
    thumbnailMaxPx: clamp(a.thumbnailMaxPx, 64, 1024),
    iconSizePx: clamp(a.iconSizePx, 16, 256),
    fontSizePx: clamp(a.fontSizePx, 8, 48),
    maxRows: clamp(a.maxRows, 1, 20),
    maxColumns: clamp(a.maxColumns, 1, 20),
    titleMaxWidthPx: clamp(a.titleMaxWidthPx, 60, 1000),
    cornerRadiusPx: clamp(a.cornerRadiusPx, 0, 64),
    compactThreshold: clamp(a.compactThreshold, 0, 1000),
  };
}

function Echo({
  initial,
  variant,
  saves,
}: {
  initial: Partial<Appearance>;
  variant?: "switcher" | "dock";
  saves: (patch: Partial<Appearance>) => void;
}) {
  const [appearance, setAppearance] = useState<Appearance>({
    ...defaultSettings.appearance,
    ...initial,
  });
  const ctx: TabContext = {
    settings: defaultSettings,
    t: makeT("en"),
    onChange: vi.fn(),
    patch: vi.fn(),
    patchAppearance: vi.fn(),
    patchBehavior: vi.fn(),
    patchFilters: vi.fn(),
    patchShortcut: vi.fn(),
    mode: "windows",
    modeAppearance: appearance,
    modeBehavior: defaultSettings.behavior,
    modePlacement: defaultSettings.placement,
    patchModeAppearance: (patch) => {
      saves(patch);
      setAppearance((current) => canonical({ ...current, ...patch }));
    },
    patchModeBehavior: vi.fn(),
    patchModePreferences: vi.fn(),
  };
  return <AppearanceTab ctx={ctx} variant={variant} />;
}

// One keystroke with the field's text selected replaces it; later keystrokes
// append to whatever the field currently shows, exactly as in the webview.
function typeOver(input: HTMLInputElement, text: string) {
  const [first, ...rest] = Array.from(text);
  fireEvent.input(input, { target: { value: first }, inputType: "insertText" });
  for (const character of rest) {
    fireEvent.input(input, { target: { value: input.value + character }, inputType: "insertText" });
  }
}

it("types a thumbnail size without the saved echo rewriting it between keystrokes", () => {
  const saves = vi.fn();
  render(<Echo initial={{ thumbnailMaxPx: 280 }} saves={saves} />);
  const input = screen.getByLabelText("Thumbnail size") as HTMLInputElement;
  typeOver(input, "300");
  expect(saves).not.toHaveBeenCalled();
  fireEvent.blur(input);
  expect(saves).toHaveBeenCalledTimes(1);
  expect(saves).toHaveBeenCalledWith({ thumbnailMaxPx: 300 });
  expect(input).toHaveValue(300);
});

it("clears max rows and types a new value without sending 0", () => {
  const saves = vi.fn();
  render(<Echo initial={{ maxRows: 4 }} saves={saves} />);
  const input = screen.getByLabelText("Max rows") as HTMLInputElement;
  fireEvent.input(input, { target: { value: "" }, inputType: "deleteContentBackward" });
  fireEvent.input(input, { target: { value: `${input.value}2` }, inputType: "insertText" });
  fireEvent.keyDown(input, { key: "Enter" });
  expect(saves).toHaveBeenCalledTimes(1);
  expect(saves).toHaveBeenCalledWith({ maxRows: 2 });
  expect(input).toHaveValue(2);
});

it("reverts an emptied appearance field to the saved value on blur", () => {
  const saves = vi.fn();
  render(<Echo initial={{ fontSizePx: 13 }} saves={saves} />);
  const input = screen.getByLabelText("Font size") as HTMLInputElement;
  fireEvent.input(input, { target: { value: "" }, inputType: "deleteContentBackward" });
  fireEvent.blur(input);
  expect(saves).not.toHaveBeenCalled();
  expect(input).toHaveValue(13);
});

it.each([
  ["Compact threshold", "compactThreshold", "8", 8, "5000", 1000],
  ["Max columns", "maxColumns", "7", 7, "99", 20],
  ["Max rows", "maxRows", "3", 3, "0", 1],
  ["Thumbnail size", "thumbnailMaxPx", "512", 512, "9999", 1024],
  ["Icon size", "iconSizePx", "48", 48, "1", 16],
  ["Title max width", "titleMaxWidthPx", "320", 320, "5", 60],
  ["Font size", "fontSizePx", "16", 16, "99", 48],
  ["Corner radius", "cornerRadiusPx", "24", 24, "200", 64],
] as const)("commits %s once on blur and clamps it to the field bounds", (label, key, typed, value, outside, bound) => {
  const saves = vi.fn();
  render(<Echo initial={{}} saves={saves} />);
  const input = screen.getByLabelText(label) as HTMLInputElement;
  typeOver(input, typed);
  expect(saves).not.toHaveBeenCalled();
  fireEvent.blur(input);
  expect(saves).toHaveBeenCalledTimes(1);
  expect(saves).toHaveBeenLastCalledWith({ [key]: value });
  expect(input).toHaveValue(value);
  typeOver(input, outside);
  fireEvent.blur(input);
  expect(saves).toHaveBeenLastCalledWith({ [key]: bound });
  expect(input).toHaveValue(bound);
});

it("uses the same draft control for the Dock preview appearance", () => {
  const saves = vi.fn();
  render(<Echo initial={{ thumbnailMaxPx: 280 }} variant="dock" saves={saves} />);
  const input = screen.getByLabelText("Dock thumbnail size") as HTMLInputElement;
  typeOver(input, "320");
  expect(saves).not.toHaveBeenCalled();
  fireEvent.blur(input);
  expect(saves).toHaveBeenCalledTimes(1);
  expect(saves).toHaveBeenCalledWith({ thumbnailMaxPx: 320 });
  expect(input).toHaveValue(320);
});

it("still saves native stepper changes immediately", () => {
  const saves = vi.fn();
  render(<Echo initial={{ maxColumns: 6 }} saves={saves} />);
  const input = screen.getByLabelText("Max columns") as HTMLInputElement;
  fireEvent.change(input, { target: { value: "7" } });
  expect(saves).toHaveBeenCalledWith({ maxColumns: 7 });
  expect(input).toHaveValue(7);
});

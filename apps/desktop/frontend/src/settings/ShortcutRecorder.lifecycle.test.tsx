import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { system } from "../lib/bridge";
import { defaultSettings } from "../lib/types";
import { ShortcutRecorder } from "./ShortcutRecorder";

vi.mock("../lib/bridge", () => ({
  system: { captureShortcut: vi.fn(), cancelShortcutCapture: vi.fn() },
}));
beforeEach(() => vi.clearAllMocks());

for (const retire of ["blur", "unmount"] as const) {
  it(`ignores a late native shortcut after ${retire}`, async () => {
    let resolve!: (chord: string) => void;
    vi.mocked(system.captureShortcut).mockReturnValue(
      new Promise((r) => {
        resolve = r;
      }),
    );
    const onChordChange = vi.fn();
    const { unmount } = render(
      <ShortcutRecorder
        aria-label="Capture"
        value="option+tab"
        placeholder="Capture"
        onChordChange={onChordChange}
      />,
    );
    fireEvent.focus(screen.getByLabelText("Capture"));
    if (retire === "blur") fireEvent.blur(screen.getByLabelText("Capture"));
    else unmount();
    await act(async () => resolve("command+x"));
    expect(onChordChange).not.toHaveBeenCalled();
  });
}

it("disarms native capture when the field is removed", () => {
  vi.mocked(system.captureShortcut).mockReturnValue(new Promise(() => {}));
  const { unmount } = render(
    <ShortcutRecorder
      aria-label="Capture"
      value="option+tab"
      placeholder="Capture"
      onChordChange={vi.fn()}
    />,
  );
  fireEvent.focus(screen.getByLabelText("Capture"));
  unmount();
  expect(system.cancelShortcutCapture).toHaveBeenCalledOnce();
});

it("preserves newer settings when a capture reply arrives after a rerender", async () => {
  let resolve!: (chord: string) => void;
  vi.mocked(system.captureShortcut).mockReturnValue(
    new Promise((r) => {
      resolve = r;
    }),
  );
  const save = vi.fn();
  const props = { "aria-label": "Capture", value: "option+tab", placeholder: "Capture" };
  // Match Settings.patch/patchShortcut: each render closes over its full snapshot.
  const callbackFor = (settings: typeof defaultSettings) => (chord: string) =>
    save({
      ...settings,
      shortcuts: settings.shortcuts.map((s, i) => (i === 0 ? { ...s, chord } : s)),
    });
  const { rerender } = render(
    <ShortcutRecorder {...props} onChordChange={callbackFor(defaultSettings)} />,
  );
  fireEvent.focus(screen.getByLabelText("Capture"));
  const newer = {
    ...defaultSettings,
    behavior: { ...defaultSettings.behavior, startAtLogin: !defaultSettings.behavior.startAtLogin },
  };
  rerender(<ShortcutRecorder {...props} onChordChange={callbackFor(newer)} />);
  await act(async () => resolve("command+x"));
  expect(save.mock.calls[0][0].behavior.startAtLogin).toBe(newer.behavior.startAtLogin);
});

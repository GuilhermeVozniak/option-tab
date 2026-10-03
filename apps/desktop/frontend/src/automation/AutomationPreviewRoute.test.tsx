import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { automationPreview, onAutomationPreviewEvents } from "../lib/automation-preview-bridge";
import { type AutomationPreviewState, emptyState } from "../lib/types";
import { AutomationPreviewRoute } from "./AutomationPreviewRoute";

vi.mock("../lib/automation-preview-bridge", () => ({
  automationPreview: {
    state: vi.fn(),
    select: vi.fn().mockResolvedValue(undefined),
    action: vi.fn().mockResolvedValue(undefined),
    size: vi.fn().mockResolvedValue(undefined),
    close: vi.fn().mockResolvedValue(undefined),
  },
  onAutomationPreviewEvents: vi.fn(() => vi.fn()),
}));

const snapshot = (revision = 3): AutomationPreviewState => ({
  open: true,
  session: 8,
  revision,
  title: "Editor windows",
  entries: [
    {
      windowId: 44,
      appId: 9,
      appName: "Editor",
      bundleId: "com.example.editor",
      title: "Plan",
      minimized: true,
      hidden: false,
      fullscreen: true,
    },
  ],
  selectedWindowId: 44,
  appearance: {
    ...emptyState.appearance,
    showWindowControls: true,
    blur: false,
    apparitionDelayMs: 0,
  },
  cardSpacingPx: 7,
  emptyReason: "",
  error: "",
});

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(automationPreview.state).mockResolvedValue(snapshot());
});

it("routes window and app buttons through the exact rendered target", async () => {
  render(<AutomationPreviewRoute session={8} />);
  fireEvent.click(await screen.findByRole("button", { name: "Hide app" }));
  fireEvent.click(screen.getByRole("button", { name: "Quit app" }));
  fireEvent.click(screen.getByRole("button", { name: /^(Minimize|Restore) window$/ }));
  fireEvent.click(screen.getByRole("button", { name: "Fullscreen window" }));
  fireEvent.click(screen.getByRole("button", { name: "Close window" }));
  fireEvent.click(screen.getByRole("button", { name: "Focus Plan" }));
  for (const kind of ["hide", "quit", "minimize", "fullscreen", "close", "focus"])
    expect(automationPreview.action).toHaveBeenCalledWith(8, 3, kind, 44, false);
  expect(screen.queryByRole("button", { name: "New window" })).toBeNull();
  expect(
    document
      .querySelector(".ot-dock-panel")
      ?.contains(screen.getByRole("button", { name: "Quit app" })),
  ).toBe(true);
});

it("does not expose app actions without an exact selected window", async () => {
  vi.mocked(automationPreview.state).mockResolvedValue({ ...snapshot(), selectedWindowId: 999 });
  render(<AutomationPreviewRoute session={8} />);
  await screen.findByText("Editor windows");
  expect(screen.queryByRole("button", { name: "Hide app" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Quit app" })).toBeNull();
});

it("discards an app action failure after a newer revision", async () => {
  let reject!: (error: Error) => void;
  vi.mocked(automationPreview.action).mockReturnValueOnce(
    new Promise<void>((_, fail) => {
      reject = fail;
    }) as ReturnType<typeof automationPreview.action>,
  );
  render(<AutomationPreviewRoute session={8} />);
  fireEvent.click(await screen.findByRole("button", { name: "Quit app" }));
  await act(async () => vi.mocked(onAutomationPreviewEvents).mock.calls[0][0].update(snapshot(4)));
  await act(async () => reject(new Error("obsolete refusal")));
  expect(screen.queryByText("obsolete refusal")).toBeNull();
});

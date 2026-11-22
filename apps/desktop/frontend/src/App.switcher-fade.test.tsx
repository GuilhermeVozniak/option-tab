import { act, render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const events = new Map<string, (event: { data: unknown }) => void>();
vi.mock("@wailsio/runtime", async (original) => ({
  ...(await original<typeof import("@wailsio/runtime")>()),
  Events: {
    On: (name: string, handler: (event: { data: unknown }) => void) => {
      events.set(name, handler);
      return () => events.delete(name);
    },
  },
}));
vi.mock("../bindings/option-tab/app.js", async (original) => ({
  ...(await original<typeof import("../bindings/option-tab/app.js")>()),
  GetVersion: vi.fn().mockResolvedValue("0.4.8"),
  GetSettingsState: vi.fn().mockResolvedValue({ revision: 1, json: "{}" }),
  GetSettings: vi.fn().mockResolvedValue("{}"),
  GetSwitcherMaterialStatus: vi
    .fn()
    .mockResolvedValue({ session: 0, revision: 0, state: "unavailable" }),
  SetSwitcherMaterialRect: vi.fn().mockResolvedValue(undefined),
  GetSwitcherGestureCapabilities: vi.fn().mockResolvedValue({ available: false }),
  SetSwitcherGestureRegions: vi.fn().mockResolvedValue(undefined),
}));

import App from "./App";
import { emptyState } from "./lib/types";

const THUMB = "data:image/png;base64,THUMB";
const NEXT = "data:image/png;base64,NEXT";

function emit(name: string, data: unknown) {
  act(() => events.get(name)?.({ data }));
}
const state = (session: number) => ({
  ...emptyState,
  open: true,
  session,
  revision: 1,
  style: "thumbnails" as const,
  appearance: {
    ...emptyState.appearance,
    style: "thumbnails" as const,
    apparitionDelayMs: 0,
    fadeOutAnimation: true,
    compactThreshold: 0,
  },
  entries: [
    {
      windowId: 10,
      appId: 20,
      title: "Test",
      appName: "Editor",
      bundleId: "test.editor",
      minimized: false,
      hidden: false,
      fullscreen: false,
    },
  ],
});
const thumbnail = (container: HTMLElement, src: string) =>
  container.querySelector(`img[src="${src}"]`);

describe("switcher fade-out", () => {
  beforeEach(() => {
    window.location.hash = "";
    events.clear();
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  // Hiding starts a ~180ms fade. Dropping the streamed thumbnails at the same
  // moment made the cards flip to icon placeholders for the whole fade.
  it("keeps streamed thumbnails on screen while the overlay fades out", () => {
    const { container } = render(<App />);
    emit("switcher:show", state(7));
    emit("switcher:thumbnails", { session: 7, frames: { "10": THUMB } });
    expect(thumbnail(container, THUMB)).not.toBeNull();

    emit("switcher:hide", { session: 7, revision: 2 });
    expect(container.querySelector(".ot-closing")).not.toBeNull();
    expect(thumbnail(container, THUMB)).not.toBeNull();

    act(() => vi.advanceTimersByTime(1000));
    expect(thumbnail(container, THUMB)).toBeNull();
  });

  it("never shows a previous session's thumbnails in the next one", () => {
    const { container } = render(<App />);
    emit("switcher:show", state(7));
    emit("switcher:thumbnails", { session: 7, frames: { "10": THUMB } });
    emit("switcher:hide", { session: 7, revision: 2 });
    emit("switcher:show", state(8));
    expect(thumbnail(container, THUMB)).toBeNull();
    emit("switcher:thumbnails", { session: 8, frames: { "10": NEXT } });
    // The cleanup scheduled by the earlier hide must not wipe the new session.
    act(() => vi.advanceTimersByTime(1000));
    expect(thumbnail(container, NEXT)).not.toBeNull();
  });
});

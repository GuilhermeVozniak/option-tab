import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { LauncherInteractionState, LauncherPresentation } from "../lib/types";
import {
  type LauncherInteractionTransport,
  useLauncherInteractions,
} from "./useLauncherInteractions";

const presentation = (revision = 5): LauncherPresentation =>
  ({ epoch: 2, displayUUID: "display", session: 3, revision }) as LauncherPresentation;
const interaction = (
  overrides: Partial<LauncherInteractionState> = {},
): LauncherInteractionState => ({
  epoch: 2,
  displayUUID: "display",
  session: 3,
  presentationRevision: 5,
  admission: 4,
  sequence: 1,
  visible: true,
  selectedItemID: "mail",
  keyboardMode: false,
  gestureAvailable: true,
  pinchAvailable: false,
  swipeAvailable: false,
  letterInputAvailable: true,
  hapticsAvailable: true,
  configured: {
    enabled: true,
    preciseScroll: true,
    pinch: false,
    swipe: false,
    primaryAction: "next",
    towardAction: "showPreview",
    pinchAction: "showPreview",
    haptics: true,
    letterNavigation: true,
    enterActivates: false,
  },
  reason: "",
  ...overrides,
});

function fixture() {
  let stateHandler = (_: LauncherInteractionState) => {};
  let errorHandler = (_: LauncherInteractionState) => {};
  const transport: LauncherInteractionTransport = {
    getState: vi.fn().mockResolvedValue(interaction()),
    subscribe: vi.fn((handler) => {
      stateHandler = handler;
      return () => {};
    }),
    subscribeError: vi.fn((handler) => {
      errorHandler = handler;
      return () => {};
    }),
    keyboard: vi.fn().mockResolvedValue(undefined),
    letter: vi.fn().mockResolvedValue(undefined),
    activate: vi.fn().mockResolvedValue(undefined),
  };
  return {
    transport,
    state: (value: LauncherInteractionState) => stateHandler(value),
    error: (value: LauncherInteractionState) => errorHandler(value),
  };
}

describe("useLauncherInteractions", () => {
  it("admits monotonic exact-scope state and queues a future parent revision", async () => {
    const f = fixture();
    const view = renderHook(
      ({ revision }) => useLauncherInteractions(presentation(revision), f.transport),
      {
        initialProps: { revision: 5 },
      },
    );
    await act(async () => {});
    expect(view.result.current.state?.selectedItemID).toBe("mail");
    act(() =>
      f.state(interaction({ presentationRevision: 6, sequence: 2, selectedItemID: "notes" })),
    );
    expect(view.result.current.state?.selectedItemID).toBe("mail");
    view.rerender({ revision: 6 });
    expect(view.result.current.state?.selectedItemID).toBe("notes");
    act(() =>
      f.state(interaction({ presentationRevision: 5, sequence: 1, selectedItemID: "stale" })),
    );
    expect(view.result.current.state?.selectedItemID).toBe("notes");
  });

  it("keeps newer future authority over late old-owner state and promotes its error", async () => {
    const f = fixture();
    const view = renderHook(
      ({ revision }) => useLauncherInteractions(presentation(revision), f.transport),
      {
        initialProps: { revision: 5 },
      },
    );
    await act(async () => {});
    act(() =>
      f.state(
        interaction({
          admission: 5,
          sequence: 1,
          presentationRevision: 6,
          selectedItemID: "future",
          reason: "failed",
        }),
      ),
    );
    act(() => f.state(interaction({ admission: 4, sequence: 99, selectedItemID: "late-old" })));
    expect(view.result.current.state?.selectedItemID).toBe("mail");
    view.rerender({ revision: 6 });
    expect(view.result.current.state?.selectedItemID).toBe("future");
    expect(view.result.current.error).toBe("failed");
  });

  it("tombstones one admission but accepts a newer successor", async () => {
    const f = fixture();
    const view = renderHook(() => useLauncherInteractions(presentation(), f.transport));
    await act(async () => {});
    act(() => f.state(interaction({ sequence: 2, visible: false })));
    expect(view.result.current.state).toBeNull();
    act(() => f.state(interaction({ admission: 4, sequence: 3, selectedItemID: "stale" })));
    expect(view.result.current.state).toBeNull();
    act(() => f.state(interaction({ admission: 5, sequence: 1, selectedItemID: "successor" })));
    expect(view.result.current.state?.selectedItemID).toBe("successor");
  });

  it("admits an authoritative error state and clears it with a newer successful state", async () => {
    const f = fixture();
    const view = renderHook(() => useLauncherInteractions(presentation(), f.transport));
    await act(async () => {});
    act(() => f.state(interaction({ sequence: 3, reason: "failed" })));
    expect(view.result.current.error).toBe("failed");
    act(() => f.state(interaction({ sequence: 2, reason: "" })));
    expect(view.result.current.error).toBe("failed");
    act(() => f.state(interaction({ sequence: 4, reason: "" })));
    expect(view.result.current.error).toBe("");
  });

  it("sends committed input and activation with current parent scope and fresh sequences", async () => {
    const f = fixture();
    const view = renderHook(() => useLauncherInteractions(presentation(7), f.transport));
    await act(async () => {});
    act(() => {
      view.result.current.commitLetter("é");
      view.result.current.activateSelection();
    });
    expect(f.transport.letter).toHaveBeenCalledWith(2, "display", 3, 7, 4, 2, "é", 0, false);
    expect(f.transport.activate).toHaveBeenCalledWith(2, "display", 3, 7, 4, 3);
  });
});

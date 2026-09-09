import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { LauncherBadgeState, LauncherBadgeTransport } from "../lib/launcher-badge-types";
import type { LauncherPresentation } from "../lib/types";
import { useLauncherBadges } from "./useLauncherBadges";

const parent: LauncherPresentation = {
  epoch: 1,
  displayUUID: "display",
  session: 2,
  revision: 3,
  profileID: "default",
  visible: true,
  reason: "",
  edge: "bottom",
  layout: "floating",
  iconPx: 40,
  bounds: { x: 0, y: 0, w: 200, h: 64 },
  appearance: {
    theme: "system",
    material: "solid",
    tint: "#172033",
    opacity: 0.76,
    borderOpacity: 0.16,
    cornerRadiusPx: 18,
    itemSpacingPx: 6,
    showLabels: true,
  },
  items: [
    { id: "pin:app", name: "App", icon: "", kind: "app", status: "ready", referenceRevision: 1 },
  ],
  widgets: [],
};
const packet = (owner = 1, sequence = 1, count = 2): LauncherBadgeState => ({
  epoch: 1,
  displayUUID: "display",
  session: 2,
  presentationRevision: 3,
  owner,
  sequence,
  visible: true,
  status: "ready",
  entries: [{ itemID: "pin:app", state: "known", kind: "count", count }],
});
function source() {
  let send = (_value: LauncherBadgeState) => {};
  let finish = (_value: LauncherBadgeState) => {};
  const off = vi.fn();
  const transport: LauncherBadgeTransport = {
    get: () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
    subscribe: (handler) => {
      send = handler;
      return off;
    },
  };
  return {
    transport,
    off,
    send: (value: LauncherBadgeState) => act(() => send(value)),
    finish: (value: LauncherBadgeState) => act(async () => finish(value)),
  };
}

describe("launcher badge ownership", () => {
  it("keeps newer events over a delayed getter and clears unavailable values", async () => {
    const s = source();
    const { result } = renderHook(() => useLauncherBadges(parent, s.transport));
    s.send(packet(2, 4, 8));
    await s.finish(packet(1, 90, 2));
    expect(result.current.get("pin:app")?.count).toBe(8);
    s.send({ ...packet(2, 5), entries: [{ itemID: "pin:app", state: "unavailable" }] });
    expect(result.current.size).toBe(0);
    s.send(packet(2, 4, 9));
    expect(result.current.size).toBe(0);
  });

  it("preserves clock continuity but refuses a prior selected reference", () => {
    const s = source();
    const { result, rerender } = renderHook(({ p }) => useLauncherBadges(p, s.transport), {
      initialProps: { p: parent },
    });
    s.send(packet());
    rerender({ p: { ...parent, revision: 4 } });
    expect(result.current.get("pin:app")?.count).toBe(2);
    rerender({
      p: { ...parent, revision: 5, items: [{ ...parent.items[0], referenceRevision: 2 }] },
    });
    expect(result.current.size).toBe(0);
    s.send(packet(3, 1, 99));
    expect(result.current.size).toBe(0);
    s.send({ ...packet(3, 2, 4), presentationRevision: 5 });
    expect(result.current.get("pin:app")?.count).toBe(4);
  });

  it("does not render future pixels, unknown items, malformed counts or retired owners", () => {
    const s = source();
    const { result, rerender } = renderHook(({ p }) => useLauncherBadges(p, s.transport), {
      initialProps: { p: parent },
    });
    s.send({ ...packet(), presentationRevision: 4 });
    expect(result.current.size).toBe(0);
    rerender({ p: { ...parent, revision: 4 } });
    expect(result.current.size).toBe(1);
    s.send({
      ...packet(2, 1),
      presentationRevision: 4,
      entries: [
        { itemID: "other", state: "known", kind: "count", count: 3 },
        { itemID: "pin:app", state: "known", kind: "count", count: -1 },
      ],
    });
    expect(result.current.size).toBe(0);
    s.send({ ...packet(2, 2), visible: false, entries: [] });
    s.send(packet(2, 3));
    expect(result.current.size).toBe(0);
    s.send(packet(1, 90));
    expect(result.current.size).toBe(0);
    rerender({ p: { ...parent, visible: false } });
    s.send(packet(3, 1));
    expect(result.current.size).toBe(0);
  });

  it("unsubscribes and ignores a getter that finishes after unmount", async () => {
    const s = source();
    const { unmount } = renderHook(() => useLauncherBadges(parent, s.transport));
    unmount();
    await s.finish(packet());
    expect(s.off).toHaveBeenCalledTimes(1);
  });
});

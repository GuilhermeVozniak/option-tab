import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { LauncherPresentation } from "../lib/types";
import { LauncherRoute, type LauncherTransport } from "./LauncherRoute";

const state = (session: number, revision: number, item = "Current"): LauncherPresentation => ({
  epoch: 5,
  displayUUID: "display-a",
  session,
  revision,
  visible: true,
  reason: "",
  profileID: "default",
  bounds: { x: 0, y: 0, w: 200, h: 64 },
  iconPx: 40,
  items: [{ id: `id-${item}`, name: item, icon: "" }],
  widgets: [],
});

function transport(initial: LauncherPresentation) {
  let update: (next: LauncherPresentation) => void = () => {};
  const api: LauncherTransport = {
    getState: vi.fn(async () => initial),
    activate: vi.fn(async () => {}),
    subscribe: (handler) => {
      update = handler;
      return () => {};
    },
  };
  return {
    api,
    update: (next: LauncherPresentation) => act(() => update(next)),
    hide: (s: number, r: number) =>
      act(() =>
        update({ ...initial, session: s, revision: r, visible: false, items: [], widgets: [] }),
      ),
  };
}

describe("LauncherRoute", () => {
  it("admits only its URL session and monotonic revisions", async () => {
    const t = transport(state(8, 2));
    render(<LauncherRoute session={8} transport={t.api} />);
    expect(await screen.findByRole("button", { name: "Current" })).toBeVisible();
    t.update(state(7, 99, "Old session"));
    t.update(state(8, 1, "Old revision"));
    expect(screen.queryByText(/Old/)).toBeNull();
    t.update(state(8, 3, "Fresh"));
    expect(screen.getByRole("button", { name: "Fresh" })).toBeVisible();
  });

  it("tombstones a hidden session against delayed updates", async () => {
    const t = transport(state(8, 2));
    render(<LauncherRoute session={8} transport={t.api} />);
    await screen.findByRole("button", { name: "Current" });
    t.hide(8, 3);
    expect(screen.queryByRole("button", { name: "Current" })).toBeNull();
    t.update(state(8, 4, "Revived"));
    expect(screen.queryByText("Revived")).toBeNull();
  });

  it("activates with the exact rendered scope", async () => {
    const t = transport(state(8, 2));
    render(<LauncherRoute session={8} transport={t.api} />);
    fireEvent.click(await screen.findByRole("button", { name: "Current" }));
    expect(t.api.activate).toHaveBeenCalledWith(5, "display-a", 8, 2, "id-Current");
  });
});

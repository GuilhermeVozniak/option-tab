import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { LauncherPresentation } from "../lib/types";
import type { LauncherWidgetState } from "../lib/widget-types";
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
  edge: "bottom",
  layout: "floating",
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

  it("admits exact-parent widget revisions, selects stacks, and tombstones retirement", async () => {
    const t = transport(state(8, 2));
    let updateWidgets: (next: LauncherWidgetState) => void = () => {};
    const widgetState: LauncherWidgetState = {
      epoch: 5,
      displayUUID: "display-a",
      session: 8,
      profileID: "default",
      revision: 4,
      visible: true,
      slots: [
        {
          id: "stack:status",
          stackID: "status",
          name: { en: "Status" },
          members: [
            { id: "audio", name: { en: "Audio" } },
            { id: "network", name: { en: "Network" } },
          ],
          selectedID: "audio",
          status: "ready",
          state: {
            lease: {
              controllerEpoch: 5,
              displayUUID: "display-a",
              session: 8,
              profileID: "default",
              instanceID: "audio",
              digest: "owned",
              admissionEpoch: 7,
              revision: 4,
            },
            status: "ready",
            root: { key: "name", kind: "text", status: "ready", text: "Speakers" },
          },
        },
      ],
    };
    t.api.widgets = {
      get: vi.fn(async () => widgetState),
      subscribe: (handler) => {
        updateWidgets = handler;
        return () => {};
      },
      options: vi.fn(async () => ({ options: [] })),
      perform: vi.fn(async () => {}),
      asset: vi.fn(async () => ""),
      select: vi.fn(async () => {}),
    };
    render(<LauncherRoute session={8} transport={t.api} />);
    expect(await screen.findByText("Speakers")).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Network" }));
    expect(t.api.widgets.select).toHaveBeenCalledWith(
      5,
      "display-a",
      8,
      "default",
      "status",
      "network",
    );
    act(() => updateWidgets({ ...widgetState, session: 9, revision: 99 }));
    act(() => updateWidgets({ ...widgetState, revision: 3 }));
    expect(screen.getByText("Speakers")).toBeVisible();
    act(() =>
      updateWidgets({
        ...widgetState,
        epoch: 0,
        displayUUID: "",
        profileID: "",
        revision: 5,
        visible: false,
        slots: [],
      }),
    );
    act(() => updateWidgets({ ...widgetState, revision: 6 }));
    expect(screen.queryByText("Speakers")).toBeNull();
  });
});

it("reports action refusal only for its current presentation", async () => {
  const t = transport(state(8, 2));
  let refuse: (error: Error) => void = () => {};
  t.api.activate = vi.fn(
    () =>
      new Promise<void>((_, reject) => {
        refuse = reject;
      }),
  );
  render(<LauncherRoute session={8} transport={t.api} />);
  fireEvent.click(await screen.findByRole("button", { name: "Current" }));
  await act(async () => refuse(new Error("Open refused")));
  expect(screen.getByRole("alert")).toHaveTextContent("Open refused");
  fireEvent.click(screen.getByRole("button", { name: "Current" }));
  t.update(state(8, 3, "New owner"));
  await act(async () => refuse(new Error("Old refusal")));
  expect(screen.queryByRole("alert")).toBeNull();
});

it("keeps a hidden session retired when a parent replaces the transport object", async () => {
  const t = transport(state(8, 2));
  const { rerender } = render(<LauncherRoute session={8} transport={t.api} />);
  await screen.findByRole("button", { name: "Current" });
  t.hide(8, 3);
  const replacement = transport(state(8, 4, "Obsolete"));
  await act(async () => rerender(<LauncherRoute session={8} transport={replacement.api} />));
  expect(screen.queryByRole("button", { name: "Obsolete" })).toBeNull();
});

it("routes exact runtime mutation and shows only current friendly refusals", async () => {
  const initial = {
    ...state(8, 2),
    runtimeReorder: true,
    itemsRevision: "hash",
    items: [
      { id: "pin:a", name: "A", icon: "", kind: "app" },
      { id: "pin:b", name: "B", icon: "", kind: "app" },
    ],
  };
  const t = transport(initial);
  let reject!: (e: Error) => void;
  t.api.mutate = vi.fn(
    () =>
      new Promise<void>((_resolve, fail) => {
        reject = fail;
      }),
  );
  render(<LauncherRoute session={8} transport={t.api} />);
  fireEvent.click(await screen.findByRole("button", { name: "Reorder A" }));
  fireEvent.click(screen.getByRole("button", { name: "Move later" }));
  expect(t.api.mutate).toHaveBeenCalledWith(5, "display-a", 8, 2, "hash", {
    kind: "moveAfter",
    itemID: "pin:a",
    targetID: "pin:b",
  });
  await act(async () => reject(new Error("stale revision")));
  expect(await screen.findByRole("alert")).toHaveTextContent("The launcher changed. Try again.");
  fireEvent.click(screen.getByRole("button", { name: "Reorder A" }));
  fireEvent.click(screen.getByRole("button", { name: "Move later" }));
  t.update({ ...initial, revision: 3 });
  await act(async () => reject(new Error("late refusal")));
  expect(screen.queryByRole("alert")).toBeNull();
});

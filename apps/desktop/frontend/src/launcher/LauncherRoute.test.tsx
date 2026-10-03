import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { makeT } from "../lib/i18n";
import type {
  LauncherItemPanelState,
  LauncherItemPanelTransport,
} from "../lib/launcher-item-panel-bridge";
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
  let panels: Parameters<LauncherItemPanelTransport["subscribe"]>[0] | undefined;
  const offPanels = vi.fn(() => {
    panels = undefined;
  });
  const api: LauncherTransport = {
    getState: vi.fn(async () => initial),
    activate: vi.fn(async () => {}),
    subscribe: (handler) => {
      update = handler;
      return () => {};
    },
    itemPanels: {
      subscribe: (handlers) => {
        panels = handlers;
        return offPanels;
      },
    },
  };
  return {
    api,
    offPanels,
    update: (next: LauncherPresentation) => act(() => update(next)),
    panel: (next: LauncherItemPanelState) => act(() => panels?.update(next)),
    closePanel: (session: number, revision: number) =>
      act(() =>
        panels?.hide({
          session,
          revision,
          parentEpoch: initial.epoch,
          parentSession: initial.session,
          displayUUID: initial.displayUUID,
          profileID: initial.profileID,
        }),
      ),
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
  t.update(state(8, 3));
  await act(async () => refuse(new Error("Open refused")));
  expect(screen.getByRole("alert")).toHaveTextContent(
    "The launcher action could not be completed. Try again.",
  );
  fireEvent.click(screen.getByRole("button", { name: "Current" }));
  t.update(state(8, 4, "New owner"));
  await act(async () => refuse(new Error("Old refusal")));
  expect(screen.queryByRole("alert")).toBeNull();
});

const child = (session: number, parentRevision = 2): LauncherItemPanelState => ({
  session,
  revision: 1,
  parentEpoch: 5,
  parentSession: 8,
  parentRevision,
  displayUUID: "display-a",
  profileID: "default",
  itemID: "id-Current",
  kind: "windows",
  title: "Current",
  open: true,
  bounds: {},
});

it("shows localized child startup failure before RPC completion and across benign revisions", async () => {
  const initial = {
    ...state(8, 2),
    items: [{ id: "id-Current", name: "Current", icon: "", kind: "app", running: true }],
  };
  const api = transport(initial);
  let finish!: () => void;
  api.api.showPanel = vi.fn(
    () =>
      new Promise<void>((resolve) => {
        finish = resolve;
      }),
  );
  const { rerender } = render(<LauncherRoute session={8} transport={api.api} t={makeT("es")} />);
  fireEvent.contextMenu(await screen.findByRole("button", { name: "Current" }));
  fireEvent.click(screen.getByRole("button", { name: "Mostrar todas las ventanas" }));
  api.panel(child(10));
  api.update({ ...initial, revision: 3 });
  api.panel({ ...child(10), revision: 2, error: "previewUnavailable" });
  api.closePanel(10, 3);
  expect(screen.getByRole("alert")).toHaveTextContent("No se pudo abrir el elemento.");
  await act(async () => finish());
  api.update({ ...initial, revision: 4 });
  expect(screen.getByRole("alert")).toHaveTextContent("No se pudo abrir el elemento.");
  rerender(<LauncherRoute session={8} transport={api.api} t={makeT("pt-BR")} />);
  expect(screen.getByRole("alert")).toHaveTextContent("Não foi possível abrir o item.");
});

it("tombstones child close and ignores errors from a replaced child or hidden parent", async () => {
  const api = transport(state(8, 2));
  render(<LauncherRoute session={8} transport={api.api} />);
  await screen.findByRole("button", { name: "Current" });
  api.closePanel(9, 3);
  api.panel(child(9));
  api.panel({ ...child(9), revision: 2, error: "previewUnavailable" });
  expect(screen.queryByRole("alert")).toBeNull();
  api.panel(child(10));
  api.closePanel(10, 2);
  api.panel({ ...child(10), revision: 3, error: "previewUnavailable" });
  expect(screen.queryByRole("alert")).toBeNull();
  api.panel(child(11));
  api.panel({ ...child(10), revision: 99, error: "previewUnavailable" });
  expect(screen.queryByRole("alert")).toBeNull();
  api.panel({ ...child(11), revision: 2, error: "previewUnavailable" });
  expect(screen.getByRole("alert")).toHaveTextContent("The item could not be opened.");
  api.hide(8, 3);
  api.panel({ ...child(12), revision: 2, error: "previewUnavailable" });
  expect(screen.queryByRole("alert")).toBeNull();
});

it("rejects child failures from another parent scope and releases its subscriptions", async () => {
  const api = transport(state(8, 2));
  const { unmount } = render(<LauncherRoute session={8} transport={api.api} />);
  await screen.findByRole("button", { name: "Current" });
  for (const changed of [
    { parentEpoch: 6 },
    { parentSession: 9 },
    { displayUUID: "display-b" },
    { profileID: "other" },
  ]) {
    api.panel({ ...child(10), ...changed, error: "previewUnavailable" });
    expect(screen.queryByRole("alert")).toBeNull();
  }
  unmount();
  expect(api.offPanels).toHaveBeenCalledTimes(1);
});

it("rejects delayed admission and failure after the same item reference is replaced or retired", async () => {
  const initial = {
    ...state(8, 2),
    items: [{ id: "id-Current", name: "Current", icon: "", referenceRevision: 1 }],
  };
  const api = transport(initial);
  render(<LauncherRoute session={8} transport={api.api} />);
  await screen.findByRole("button", { name: "Current" });
  api.update({ ...initial, revision: 3, items: [{ ...initial.items[0], referenceRevision: 2 }] });
  api.panel(child(10));
  api.panel({ ...child(10), revision: 2, error: "previewUnavailable" });
  expect(screen.queryByRole("alert")).toBeNull();
  api.update({ ...initial, revision: 4, items: [] });
  api.update({ ...initial, revision: 5 });
  api.panel({ ...child(11), revision: 2, error: "previewUnavailable" });
  expect(screen.queryByRole("alert")).toBeNull();
  api.panel(child(12, 5));
  api.panel({ ...child(12, 5), revision: 2, error: "previewUnavailable" });
  expect(screen.getByRole("alert")).toHaveTextContent("The item could not be opened.");
});

it("normalizes raw action failures and translates an existing error on language changes", async () => {
  const api = transport(state(8, 2));
  api.api.activate = vi.fn(async () => {
    throw new Error("launcher: action busy");
  });
  const { rerender } = render(<LauncherRoute session={8} transport={api.api} t={makeT("es")} />);
  fireEvent.click(await screen.findByRole("button", { name: "Current" }));
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "El Dock está ocupado. Inténtalo de nuevo.",
  );
  expect(screen.getByRole("alert")).not.toHaveTextContent("launcher:");
  rerender(<LauncherRoute session={8} transport={api.api} t={makeT("pt-BR")} />);
  expect(screen.getByRole("alert")).toHaveTextContent("O Dock está ocupado. Tente novamente.");
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

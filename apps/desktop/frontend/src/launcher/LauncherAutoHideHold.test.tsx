import { act, fireEvent, render } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { LauncherPresentation } from "../lib/types";
import { LauncherView } from "./LauncherView";

const presentation: LauncherPresentation = {
  epoch: 1,
  session: 2,
  revision: 3,
  displayUUID: "display",
  profileID: "default",
  visible: true,
  reason: "",
  bounds: { x: 0, y: 0, w: 400, h: 64 },
  edge: "bottom",
  layout: "floating",
  iconPx: 40,
  appearance: {
    theme: "light",
    material: "solid",
    tint: "#000000",
    opacity: 1,
    borderOpacity: 0,
    cornerRadiusPx: 8,
    itemSpacingPx: 8,
    showLabels: true,
  },
  runtimeReorder: true,
  itemsRevision: "items",
  items: [
    {
      id: "pin:a",
      kind: "app",
      name: "Owned app",
      icon: "",
      status: "ready",
      running: true,
      referenceRevision: 1,
    },
  ],
  widgets: [],
};
afterEach(() => vi.unstubAllGlobals());

it("holds the real app menu and releases it on Escape and menu close", async () => {
  const hold = vi.fn().mockResolvedValue(undefined);
  const view = render(
    <LauncherView
      presentation={presentation}
      onActivate={() => {}}
      onRelaunch={() => {}}
      onAutoHideHold={hold}
    />,
  );
  fireEvent.contextMenu(view.getByRole("button", { name: "Owned app" }));
  expect(view.getByRole("button", { name: "Relaunch" })).toBeVisible();
  expect(hold).toHaveBeenLastCalledWith(1, "display", 2, 3, 1, "begin");
  fireEvent.keyDown(window, { key: "Escape" });
  await act(async () => {});
  expect(view.queryByRole("button", { name: "Relaunch" })).toBeNull();
  expect(hold.mock.calls.at(-1)?.at(-1)).toBe("end");
  fireEvent.contextMenu(view.getByRole("button", { name: "Owned app" }));
  fireEvent.click(view.getByRole("button", { name: "Close application actions" }));
  await act(async () => {});
  expect(hold.mock.calls.at(-1)?.at(-1)).toBe("end");
});

it("holds reorder pointer capture outside the item and releases on lost capture", async () => {
  vi.stubGlobal("PointerEvent", MouseEvent);
  const hold = vi.fn().mockResolvedValue(undefined);
  const mutate = vi.fn();
  const view = render(
    <LauncherView
      presentation={presentation}
      onActivate={() => {}}
      onMutate={mutate}
      onAutoHideHold={hold}
    />,
  );
  const handle = view.getByRole("button", { name: "Reorder Owned app" });
  fireEvent.pointerDown(handle, { button: 0, clientX: 5, clientY: 5 });
  fireEvent.pointerMove(window, { clientX: -500, clientY: -500 });
  expect(hold.mock.calls.at(-1)?.at(-1)).toBe("begin");
  fireEvent.lostPointerCapture(handle);
  await act(async () => {});
  expect(hold.mock.calls.at(-1)?.at(-1)).toBe("end");
  expect(mutate).not.toHaveBeenCalled();
});

import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import type { LauncherPresentation } from "../lib/types";
import { LauncherView } from "./LauncherView";

const presentation: LauncherPresentation = {
  epoch: 9,
  displayUUID: "display-main",
  session: 12,
  revision: 4,
  visible: true,
  reason: "",
  profileID: "default",
  bounds: { x: 20, y: 700, w: 260, h: 64 },
  iconPx: 40,
  edge: "bottom",
  layout: "floating",
  appearance: {
    theme: "light",
    material: "system",
    tint: "#224466",
    opacity: 0.8,
    borderOpacity: 0.2,
    cornerRadiusPx: 22,
    itemSpacingPx: 9,
    showLabels: false,
  },
  items: [
    { id: "item-17", name: "Owned app", icon: "", kind: "app", status: "ready", running: true },
  ],
  widgets: [],
};

it("offers explicit relaunch for an unpinned running app using only its rendered item scope", () => {
  const relaunch = vi.fn();
  render(<LauncherView presentation={presentation} onActivate={() => {}} onRelaunch={relaunch} />);
  expect(relaunch).not.toHaveBeenCalled();
  fireEvent.contextMenu(screen.getByRole("button", { name: "Owned app" }));
  expect(relaunch).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "Relaunch" }));
  expect(relaunch).toHaveBeenCalledExactlyOnceWith(9, "display-main", 12, 4, "item-17");
});

it.each(["stopped", "unavailable"])("does not offer relaunch for a %s app", (state) => {
  const item = {
    ...presentation.items[0],
    running: state !== "stopped",
    status: state === "unavailable" ? "missing" : "ready",
  };
  const relaunch = vi.fn();
  render(
    <LauncherView
      presentation={{ ...presentation, items: [item] }}
      onActivate={() => {}}
      onRelaunch={relaunch}
    />,
  );
  fireEvent.contextMenu(screen.getByRole("button", { name: "Owned app" }));
  expect(screen.queryByRole("button", { name: "Relaunch" })).toBeNull();
  expect(relaunch).not.toHaveBeenCalled();
});

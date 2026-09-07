import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
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
    { id: "opaque-1", name: "Notes & Tasks", icon: "data:image/png;base64,AA==" },
    { id: "opaque-2", name: "Windowless Helper", icon: "" },
  ],
  widgets: [
    {
      id: "clock",
      packageID: "org.optiontab.clock",
      digest: "builtin-clock-v1",
      status: "ready",
      root: { kind: "row", text: "", children: [{ kind: "text", text: "09:41" }] },
    },
  ],
};

describe("LauncherView", () => {
  it("renders bounded app choices and the trusted clock tree", () => {
    render(<LauncherView presentation={presentation} onActivate={() => {}} />);
    expect(screen.getByRole("button", { name: "Notes & Tasks" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Windowless Helper" })).toBeVisible();
    expect(screen.getByText("09:41")).toBeVisible();
    expect(screen.getByRole("list")).toHaveClass("ot-launcher-strip");
    expect(screen.getByLabelText("Option Tab launcher")).toHaveClass("theme-light", "edge-bottom");
    expect(screen.getByText("Notes & Tasks")).toHaveClass("ot-launcher-label-hidden");
  });

  it("dispatches the exact backend-owned scope and opaque item id", () => {
    const activate = vi.fn();
    render(<LauncherView presentation={presentation} onActivate={activate} />);
    fireEvent.click(screen.getByRole("button", { name: "Windowless Helper" }));
    expect(activate).toHaveBeenCalledWith(9, "display-main", 12, 4, "opaque-2");
  });

  it("does not interpret widget text as markup", () => {
    const hostile = structuredClone(presentation) as any;
    hostile.widgets[0].root.children[0].text = "<img src=x onerror=alert(1)>";
    const { container } = render(<LauncherView presentation={hostile} onActivate={() => {}} />);
    expect(screen.getByText("<img src=x onerror=alert(1)>")).toBeVisible();
    expect(container.querySelector("img[src='x']")).toBeNull();
  });
});

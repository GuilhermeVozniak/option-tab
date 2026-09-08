import { expect, type Page, test } from "@playwright/test";
import type { LauncherPresentation } from "../src/lib/types";
import type { LauncherWidgetState } from "../src/lib/widget-types";
import { installFakeWails } from "./support/fakeWails";

type Edge = "bottom" | "top" | "left" | "right";

async function showClocks(page: Page, edge: Edge, length: number, count = 2) {
  const vertical = edge === "left" || edge === "right";
  const viewport = vertical ? { width: 64, height: length } : { width: length, height: 64 };
  await page.setViewportSize(viewport);
  await installFakeWails(page);
  const presentation: LauncherPresentation = {
    epoch: 7,
    displayUUID: "display-fixture",
    session: 41,
    revision: 2,
    visible: true,
    reason: "",
    profileID: "default",
    bounds: { x: 0, y: 0, w: viewport.width, h: viewport.height },
    iconPx: 40,
    edge,
    layout: "floating",
    appearance: {
      theme: "dark",
      material: "solid",
      tint: "#172033",
      opacity: 0.76,
      borderOpacity: 0.16,
      cornerRadiusPx: 18,
      itemSpacingPx: 6,
      showLabels: false,
    },
    items: Array.from({ length: 8 }, (_, i) => ({
      id: `item-${i}`,
      name: `Application ${i}`,
      icon: "",
      kind: "app",
    })),
    widgets: [],
  };
  const widgets: LauncherWidgetState = {
    epoch: presentation.epoch,
    displayUUID: presentation.displayUUID,
    session: presentation.session,
    profileID: presentation.profileID,
    revision: 1,
    visible: true,
    slots: Array.from({ length: count }, (_, i) => ({
      id: `clock-${i}`,
      name: { en: `Clock ${i}` },
      members: [{ id: `clock-${i}`, name: { en: `Clock ${i}` } }],
      selectedID: `clock-${i}`,
      status: "ready",
      state: {
        lease: {
          controllerEpoch: presentation.epoch,
          displayUUID: presentation.displayUUID,
          session: presentation.session,
          profileID: presentation.profileID,
          instanceID: `clock-${i}`,
          digest: `clock-${i}`,
          admissionEpoch: 1,
          revision: 1,
        },
        status: "ready",
        root: {
          key: "root",
          kind: "row",
          status: "ready",
          children: [{ key: "time", kind: "text", status: "ready", text: `${20 + i}:08` }],
        },
      },
    })),
  };
  await page.addInitScript(
    ([state, widgetState]) => {
      (window as any).__launcherState = state;
      (window as any).__launcherWidgets = widgetState;
    },
    [presentation, widgets],
  );
  await page.goto("/#/launcher/41");
  await expect(page.locator(".ot-widget-text")).toHaveCount(count);
}

async function geometry(page: Page) {
  return page.getByLabel("Launcher widgets", { exact: true }).evaluate((container) => {
    const box = container.getBoundingClientRect();
    const host = container.closest("main")!;
    const shell = host.getBoundingClientRect();
    const strip = host.querySelector(".ot-launcher-strip")!;
    const inside = (child: DOMRect, parent: DOMRect) =>
      child.left >= parent.left - 0.5 &&
      child.right <= parent.right + 0.5 &&
      child.top >= parent.top - 0.5 &&
      child.bottom <= parent.bottom + 0.5;
    return {
      containerInsideHost: inside(box, shell),
      hostInsideViewport:
        shell.left >= 0 &&
        shell.top >= 0 &&
        shell.right <= window.innerWidth &&
        shell.bottom <= window.innerHeight,
      horizontalOverflow: container.scrollWidth > container.clientWidth,
      verticalOverflow: container.scrollHeight > container.clientHeight,
      applicationsInside: [...strip.querySelectorAll(".ot-launcher-app")].map((application) =>
        inside(application.getBoundingClientRect(), strip.getBoundingClientRect()),
      ),
      clocksInside: [...container.querySelectorAll(".ot-widget-text")].map((clock) =>
        inside(clock.getBoundingClientRect(), box),
      ),
    };
  });
}

for (const edge of ["bottom", "top", "left", "right"] as const) {
  test(`two ${edge} clock slots remain visible beside eight applications`, async ({ page }) => {
    await showClocks(page, edge, 718);
    const boxes = await geometry(page);
    expect(boxes.containerInsideHost).toBe(true);
    expect(boxes.hostInsideViewport).toBe(true);
    expect(boxes.applicationsInside).toEqual(Array(8).fill(true));
    expect(boxes.clocksInside).toEqual([true, true]);
    expect(boxes.horizontalOverflow).toBe(false);
    expect(boxes.verticalOverflow).toBe(false);
  });

  test(`narrow ${edge} launcher keeps overflowing clocks reachable by scrolling`, async ({
    page,
  }) => {
    await showClocks(page, edge, 260);
    const vertical = edge === "left" || edge === "right";
    const before = await geometry(page);
    expect(before.containerInsideHost).toBe(true);
    expect(before.hostInsideViewport).toBe(true);
    expect(vertical ? before.verticalOverflow : before.horizontalOverflow).toBe(true);
    await page.getByText("21:08", { exact: true }).evaluate((clock) => {
      clock.scrollIntoView({ block: "nearest", inline: "nearest" });
    });
    await expect.poll(async () => (await geometry(page)).clocksInside[1]).toBe(true);
  });
}

for (const edge of ["bottom", "left"] as const) {
  test(`five ${edge} clock slots retain the four-slot viewport cap`, async ({ page }) => {
    await showClocks(page, edge, 1400, 5);
    const vertical = edge === "left";
    const before = await geometry(page);
    expect(before.clocksInside).toEqual([true, true, true, true, false]);
    expect(vertical ? before.verticalOverflow : before.horizontalOverflow).toBe(true);
    await page.getByLabel("Launcher widgets", { exact: true }).evaluate((container, isVertical) => {
      if (isVertical) container.scrollTop = container.scrollHeight;
      else container.scrollLeft = container.scrollWidth;
    }, vertical);
    await expect.poll(async () => (await geometry(page)).clocksInside[4]).toBe(true);
  });
}

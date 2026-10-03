import { expect, test } from "@playwright/test";
import { getCallRecords, installFakeWails } from "./support/fakeWails";

for (const edge of ["bottom", "top", "left", "right"]) {
  test(`Dock badges remain inside ${edge} icons and retire stale values`, async ({ page }) => {
    const vertical = edge === "left" || edge === "right";
    const size = vertical ? { width: 88, height: 500 } : { width: 500, height: 88 };
    await page.setViewportSize(size);
    await installFakeWails(page);
    await page.addInitScript(
      ({ edge, size }) => {
        const w = window as any;
        w.__launcherState = {
          epoch: 7,
          displayUUID: "main",
          session: 41,
          revision: 1,
          visible: true,
          reason: "",
          profileID: "default",
          bounds: { x: 0, y: 0, w: size.width, h: size.height },
          iconPx: 40,
          edge,
          layout: "floating",
          appearance: {
            theme: "dark",
            material: "solid",
            tint: "#172033",
            opacity: 0.8,
            borderOpacity: 0.2,
            cornerRadiusPx: 12,
            itemSpacingPx: 6,
            showLabels: true,
          },
          magnification: { enabled: true, scale: 1.5, reach: 1, primaryInset: 42, crossInset: 24 },
          items: [
            { id: "a", name: "Alpha", icon: "", kind: "app", status: "ready" },
            {
              id: "group",
              name: "Group",
              icon: "",
              kind: "group",
              members: [{ id: "b", name: "Beta", icon: "", kind: "app", status: "ready" }],
            },
          ],
          widgets: [],
        };
        w.__launcherBadges = {
          epoch: 7,
          displayUUID: "main",
          session: 41,
          presentationRevision: 1,
          owner: 1,
          sequence: 1,
          visible: true,
          status: "ready",
          entries: [
            { itemID: "a", state: "known", kind: "count", count: 137 },
            { itemID: "b", state: "known", kind: "indicator" },
          ],
        };
      },
      { edge, size },
    );
    await page.goto("/#/launcher/41");
    await expect(page.locator(".ot-launcher-badge")).toHaveText("99+");
    await expect(page.getByRole("button", { name: "Alpha", exact: true })).toHaveAttribute(
      "aria-description",
      /Dock badge count: 137/,
    );
    await page.getByRole("button", { name: "Group", exact: true }).click();
    await expect(page.locator(".ot-launcher-badge")).toHaveCount(2);
    const inside = await page.locator(".ot-launcher-badge").evaluateAll((nodes) =>
      nodes.every((node) => {
        const b = node.getBoundingClientRect();
        const icon = node.closest(".ot-launcher-visual")!.getBoundingClientRect();
        return (
          b.left >= icon.left - 0.1 &&
          b.right <= icon.right + 0.1 &&
          b.top >= icon.top - 0.1 &&
          b.bottom <= icon.bottom + 0.1
        );
      }),
    );
    expect(inside).toBe(true);
    await page.evaluate(() => {
      const w = window as any;
      w._wails.dispatchWailsEvent({
        name: "launcher:badges",
        data: {
          ...w.__launcherBadges,
          sequence: 2,
          entries: [
            { itemID: "a", state: "unavailable" },
            { itemID: "b", state: "unsupported" },
          ],
        },
      });
      w._wails.dispatchWailsEvent({ name: "launcher:badges", data: w.__launcherBadges });
    });
    await expect(page.locator(".ot-launcher-badge")).toHaveCount(0);
    const calls = await getCallRecords(page);
    expect(calls.some((call) => call[0] === "GetLauncherBadges")).toBe(true);
    expect(calls.filter((call) => call[0] === "ActivateLauncherItem")).toHaveLength(0);
  });
}

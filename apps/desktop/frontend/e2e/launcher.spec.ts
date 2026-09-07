import { expect, test } from "@playwright/test";
import { getCallRecords, installFakeWails } from "./support/fakeWails";

const presentation = (revision: number, name: string, visible = true) => ({
  epoch: 7,
  displayUUID: "display-main",
  session: 41,
  revision,
  visible,
  reason: "",
  profileID: "default",
  bounds: { x: 20, y: 700, w: 220, h: 64 },
  iconPx: 40,
  edge: "bottom",
  layout: "floating",
  appearance: {
    theme: "dark",
    material: "solid",
    tint: "#172033",
    opacity: 0.76,
    borderOpacity: 0.16,
    cornerRadiusPx: 18,
    itemSpacingPx: 6,
    showLabels: true,
  },
  items: visible ? [{ id: `opaque-${name}`, name, icon: "" }] : [],
  widgets: [],
});

for (const edge of ["bottom", "top", "left", "right"] as const) {
  test(`launcher fits 64px icons and clock at minimum ${edge} cross-axis`, async ({ page }) => {
    const vertical = edge === "left" || edge === "right";
    await page.setViewportSize(vertical ? { width: 72, height: 220 } : { width: 220, height: 72 });
    await page.emulateMedia({ colorScheme: "light" });
    await installFakeWails(page);
    const many = {
      ...presentation(2, "One"),
      edge,
      iconPx: 64,
      appearance: { ...presentation(2, "One").appearance, theme: "system" },
      items: Array.from({ length: 12 }, (_, index) => ({
        id: `item-${index}`,
        name: `Application ${index}`,
        icon: "",
      })),
      widgets: [
        {
          id: "clock",
          packageID: "org.optiontab.clock",
          digest: "clock",
          status: "ready",
          root: { kind: "row", children: [{ kind: "text", text: "09:41" }] },
        },
      ],
    };
    await page.addInitScript((state) => {
      (window as any).__launcherState = state;
    }, many);
    await page.goto("/#/launcher/41");
    const geometry = await page.getByLabel("Option Tab launcher").evaluate((node, isVertical) => {
      const shell = node.getBoundingClientRect();
      const strip = node.querySelector("ul") as HTMLElement;
      const childrenInside = [...node.querySelectorAll("button, aside")].every((child) => {
        const rect = child.getBoundingClientRect();
        return isVertical
          ? rect.left >= shell.left && rect.right <= shell.right
          : rect.top >= shell.top && rect.bottom <= shell.bottom;
      });
      return {
        cross: isVertical ? shell.width : shell.height,
        documentCross: isVertical
          ? document.documentElement.scrollWidth
          : document.documentElement.scrollHeight,
        primaryOverflow: isVertical
          ? strip.scrollHeight > strip.clientHeight
          : strip.scrollWidth > strip.clientWidth,
        childrenInside,
        color: getComputedStyle(node).color,
      };
    }, vertical);
    expect(geometry.cross).toBeLessThanOrEqual(72);
    expect(geometry.documentCross).toBeLessThanOrEqual(72);
    expect(geometry.childrenInside).toBe(true);
    expect(geometry.primaryOverflow).toBe(true);
    expect(geometry.color).toBe("rgb(22, 33, 60)");
  });
}

test("launcher activates the exact rendered scope and tombstones retirement", async ({ page }) => {
  await installFakeWails(page);
  await page.addInitScript(
    (state) => {
      (window as any).__launcherState = state;
    },
    presentation(2, "Windowless Helper"),
  );
  await page.goto("/#/launcher/41");
  await page.getByRole("button", { name: "Windowless Helper" }).click();
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual(["ActivateLauncherItem", 7, "display-main", 41, 2, "opaque-Windowless Helper"]);
  await page.evaluate(
    (state) => (window as any)._wails.dispatchWailsEvent({ name: "launcher:state", data: state }),
    presentation(3, "", false),
  );
  await expect(page.getByRole("button")).toHaveCount(0);
  await page.evaluate(
    (state) => (window as any)._wails.dispatchWailsEvent({ name: "launcher:state", data: state }),
    presentation(4, "Revived"),
  );
  await expect(page.getByText("Revived")).toHaveCount(0);
});

test("settings keeps launcher opt-in separate and offers native Dock recovery", async ({
  page,
}) => {
  await installFakeWails(page);
  await page.goto("/#/settings");
  await page.getByRole("tab", { name: "Dock" }).click();
  await expect(page.getByRole("checkbox", { name: "Enable replacement Dock" })).not.toBeChecked();
  await expect(page.getByRole("checkbox", { name: "Show clock" })).not.toBeChecked();
  await page.getByRole("button", { name: "Use native Dock" }).click();
  await expect.poll(() => getCallRecords(page)).toContainEqual(["UseNativeDock"]);
});

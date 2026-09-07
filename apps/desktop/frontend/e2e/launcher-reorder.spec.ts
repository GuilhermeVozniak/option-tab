import { expect, test } from "@playwright/test";
import { getCallRecords, installFakeWails } from "./support/fakeWails";

const state = {
  epoch: 7,
  displayUUID: "main",
  session: 41,
  revision: 1,
  visible: true,
  reason: "",
  profileID: "default",
  bounds: { x: 0, y: 0, w: 500, h: 88 },
  iconPx: 40,
  edge: "bottom",
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
  runtimeReorder: true,
  itemsRevision: "hash1",
  items: [
    { id: "pin:a", name: "Alpha", icon: "", kind: "app" },
    { id: "pin:b", name: "Beta", icon: "", kind: "app" },
    { id: "pin:c", name: "Missing", icon: "", kind: "app", status: "missing" },
  ],
  widgets: [],
};
async function start(page: import("@playwright/test").Page) {
  await installFakeWails(page);
  await page.addInitScript((value) => {
    (window as any).__launcherState = value;
  }, state);
  await page.goto("/#/launcher/41");
}
test("internal pointer drag uses stable magnified boxes and suppresses activation", async ({
  page,
}) => {
  await start(page);
  const target = await page.getByRole("button", { name: "Beta", exact: true }).boundingBox();
  const handle = await page
    .getByRole("button", { name: "Reorder Alpha", exact: true })
    .boundingBox();
  await page.mouse.move(handle!.x + 7, handle!.y + 8);
  await page.mouse.down();
  await page.mouse.move(target!.x + target!.width - 3, target!.y + 20, { steps: 8 });
  await page.mouse.up();
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual([
      "MutateLauncherItems",
      7,
      "main",
      41,
      1,
      "hash1",
      { kind: "moveAfter", itemID: "pin:a", targetID: "pin:b" },
    ]);
  expect((await getCallRecords(page)).filter((c) => c[0] === "ActivateLauncherItem")).toHaveLength(
    0,
  );
});
test("missing pin keyboard menu groups explicitly and external drops do nothing", async ({
  page,
}) => {
  await start(page);
  const handle = page.getByRole("button", { name: "Reorder Missing", exact: true });
  await handle.focus();
  await page.keyboard.press("Enter");
  await page.getByRole("button", { name: "Group with Alpha", exact: true }).click();
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual([
      "MutateLauncherItems",
      7,
      "main",
      41,
      1,
      "hash1",
      { kind: "addToGroup", itemID: "pin:c", targetID: "pin:a" },
    ]);
  const before = (await getCallRecords(page)).filter((c) => c[0] === "MutateLauncherItems").length;
  await page.locator(".ot-launcher-strip").evaluate((node) => {
    const data = new DataTransfer();
    data.setData("text/plain", "pin:a");
    node.dispatchEvent(new DragEvent("drop", { bubbles: true, dataTransfer: data }));
  });
  expect((await getCallRecords(page)).filter((c) => c[0] === "MutateLauncherItems")).toHaveLength(
    before,
  );
});

test("vertical expanded groups keep distinct handles on their own icons", async ({ page }) => {
  await installFakeWails(page);
  await page.addInitScript(
    (value) => {
      (window as any).__launcherState = value;
    },
    {
      ...state,
      edge: "left",
      bounds: { x: 0, y: 0, w: 100, h: 650 },
      magnification: { enabled: false, scale: 1, reach: 0, primaryInset: 0, crossInset: 0 },
      items: [
        {
          id: "pin:g",
          kind: "group",
          name: "Group",
          icon: "",
          members: [state.items[0], state.items[1]],
        },
        state.items[2],
      ],
    },
  );
  await page.goto("/#/launcher/41");
  await page.getByRole("button", { name: "Group", exact: true }).click();
  await page.getByRole("button", { name: "Reorder Group", exact: true }).focus();
  await page.keyboard.press("Enter");
  const boxes: number[] = [];
  for (const name of ["Group", "Alpha", "Beta"]) {
    const icon = await page.getByRole("button", { name, exact: true }).boundingBox();
    const handle = await page
      .getByRole("button", { name: `Reorder ${name}`, exact: true })
      .boundingBox();
    expect(handle!.x).toBeGreaterThanOrEqual(icon!.x);
    expect(handle!.y).toBeGreaterThanOrEqual(icon!.y);
    expect(handle!.x + handle!.width).toBeLessThanOrEqual(icon!.x + icon!.width);
    expect(handle!.y + handle!.height).toBeLessThanOrEqual(icon!.y + icon!.height);
    boxes.push(handle!.y);
  }
  expect(new Set(boxes).size).toBe(3);
});

test("decorative move targets show the accepted insertion side", async ({ page }) => {
  await installFakeWails(page);
  await page.addInitScript(
    (value) => {
      (window as any).__launcherState = value;
    },
    {
      ...state,
      items: [
        state.items[0],
        { id: "pin:s", name: "", icon: "", kind: "separator" },
        state.items[1],
      ],
    },
  );
  await page.goto("/#/launcher/41");
  const handle = await page
    .getByRole("button", { name: "Reorder Alpha", exact: true })
    .boundingBox();
  const target = page.locator('[data-reorder-target="pin:s"]');
  const box = await target.boundingBox();
  await page.mouse.move(handle!.x + 7, handle!.y + 8);
  await page.mouse.down();
  await page.mouse.move(box!.x + 2, box!.y + box!.height / 2, { steps: 6 });
  await expect(target).toHaveAttribute("data-reorder-zone", "moveBefore");
  expect(await target.evaluate((node) => getComputedStyle(node).boxShadow)).not.toBe("none");
  await page.mouse.move(box!.x + box!.width - 2, box!.y + box!.height / 2);
  await expect(target).toHaveAttribute("data-reorder-zone", "moveAfter");
  await page.mouse.up();
});

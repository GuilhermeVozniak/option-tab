import { expect, type Page, test } from "@playwright/test";
import { getCallRecords, installFakeWails, showState } from "./support/fakeWails";

async function showDock(page: Page, overrides: Record<string, unknown> = {}) {
  const base = showState() as any;
  const state = {
    session: 91,
    revision: 1,
    open: true,
    item: {
      appId: 10,
      bundleId: "qa.app",
      path: "/QA.app",
      title: "QA",
      bounds: { x: 0, y: 0, w: 40, h: 40 },
      screenId: 1,
      edge: "bottom",
      kind: "app",
    },
    entries: [
      { ...base.entries[0], windowId: 101, appId: 10, title: "First" },
      { ...base.entries[1], windowId: 102, appId: 10, title: "Exact target" },
    ],
    selectedWindowId: 101,
    appearance: { ...base.appearance, showWindowControls: false },
    emptyReason: "",
    ...overrides,
  };
  await page.waitForFunction(
    () => typeof (window as any)._wails?.dispatchWailsEvent === "function",
  );
  await page.evaluate(
    (s) => (window as any)._wails.dispatchWailsEvent({ name: "dock:show", data: s }),
    state,
  );
  await expect(page.locator("article[data-window-id='102']")).toBeVisible();
  return state;
}

test.beforeEach(async ({ page }) => {
  await installFakeWails(page);
  await page.goto("/#dock");
});

test("disabled preview dragging remains an ordinary exact focus click", async ({ page }) => {
  await showDock(page, { previewDragEnabled: false });
  await page.getByRole("button", { name: "Focus Exact target" }).click();
  const calls = await getCallRecords(page);
  expect(calls).toContainEqual(["FocusDockWindow", 91, 102, 10]);
  expect(calls.some(([name]) => name === "BeginDockPreviewDrag")).toBe(false);
});

test("threshold drag sends exact target, global point and normalized anchor, then suppresses click", async ({
  page,
}) => {
  await showDock(page, { previewDragEnabled: true, dragGestureFloor: 44 });
  const preview = page.getByRole("button", { name: "Focus Exact target" });
  const box = (await preview.boundingBox())!;
  await preview.dispatchEvent("pointerdown", {
    pointerId: 7,
    button: 0,
    isPrimary: true,
    clientX: box.x + box.width * 0.25,
    clientY: box.y + box.height * 0.4,
    screenX: 700,
    screenY: 500,
  });
  await preview.dispatchEvent("pointermove", {
    pointerId: 7,
    button: 0,
    isPrimary: true,
    clientX: box.x + box.width * 0.27,
    clientY: box.y + box.height * 0.41,
    screenX: 702,
    screenY: 501,
  });
  expect((await getCallRecords(page)).some(([name]) => name === "BeginDockPreviewDrag")).toBe(
    false,
  );
  await preview.dispatchEvent("pointermove", {
    pointerId: 7,
    button: 0,
    isPrimary: true,
    clientX: box.x + box.width * 0.5,
    clientY: box.y + box.height * 0.6,
    screenX: 750,
    screenY: 550,
  });
  await expect
    .poll(
      async () =>
        (await getCallRecords(page)).filter(([name]) => name === "BeginDockPreviewDrag").length,
    )
    .toBe(1);
  const begin = (await getCallRecords(page)).find(([name]) => name === "BeginDockPreviewDrag")!;
  expect(begin.slice(1, 5)).toEqual([91, 45, 102, 10]);
  expect(begin.slice(5, 7)).toEqual([750, 550]);
  expect(begin[7] as number).toBeCloseTo(0.25, 2);
  expect(begin[8] as number).toBeCloseTo(0.4, 2);
  await preview.dispatchEvent("pointerup", { pointerId: 7, button: 0, isPrimary: true });
  // This is the click synthesized from the completed physical sequence. A
  // Playwright `click()` would create a new pointerdown and correctly begin a
  // separate user gesture.
  await preview.dispatchEvent("click");
  const calls = await getCallRecords(page);
  expect(calls).toContainEqual(["CancelDockPreviewDrag", 91, 45]);
  expect(calls.some(([name]) => name === "FocusDockWindow")).toBe(false);
});

test("completed or escaped drag cannot rearm from trailing hover movement", async ({ page }) => {
  await showDock(page, { previewDragEnabled: true, dragGestureFloor: 90 });
  const preview = page.getByRole("button", { name: "Focus Exact target" });
  const box = (await preview.boundingBox())!;
  await preview.dispatchEvent("pointerdown", {
    pointerId: 9,
    button: 0,
    isPrimary: true,
    clientX: box.x + 10,
    clientY: box.y + 10,
  });
  await preview.dispatchEvent("pointermove", {
    pointerId: 9,
    clientX: box.x + 30,
    clientY: box.y + 30,
    screenX: 300,
    screenY: 300,
  });
  await expect
    .poll(
      async () =>
        (await getCallRecords(page)).filter(([name]) => name === "BeginDockPreviewDrag").length,
    )
    .toBe(1);
  await preview.dispatchEvent("pointermove", {
    pointerId: 9,
    clientX: box.x + 60,
    clientY: box.y + 60,
    screenX: 330,
    screenY: 330,
  });
  await page.keyboard.press("Escape");
  await preview.dispatchEvent("click");
  const calls = await getCallRecords(page);
  expect(calls.filter(([name]) => name === "BeginDockPreviewDrag")).toHaveLength(1);
  expect(calls).toContainEqual(["CancelDockPreviewDrag", 91, 91]);
  expect(calls.some(([name]) => name === "FocusDockWindow")).toBe(false);
});

test("publishes real clipped DOM card regions through the D12 RPC", async ({ page }) => {
  await showDock(page);
  await expect
    .poll(
      async () =>
        (await getCallRecords(page)).filter(([name]) => name === "SetDockPreviewRegions").length,
    )
    .toBeGreaterThan(0);
  const record = (await getCallRecords(page))
    .filter(([name]) => name === "SetDockPreviewRegions")
    .at(-1)!;
  expect(record.slice(1, 3)).toEqual([91, expect.any(Number)]);
  const regions = record[3] as Array<{
    windowId: number;
    appId: number;
    bounds: { x: number; y: number; w: number; h: number };
  }>;
  expect(regions.map((r) => [r.windowId, r.appId])).toEqual([
    [101, 10],
    [102, 10],
  ]);
  const viewport = (await page.locator(".ot-dock-list-viewport").boundingBox())!;
  for (const { bounds } of regions) {
    expect(bounds.x).toBeGreaterThanOrEqual(viewport.x);
    expect(bounds.y).toBeGreaterThanOrEqual(viewport.y);
    expect(bounds.x + bounds.w).toBeLessThanOrEqual(viewport.x + viewport.width + 0.02);
    expect(bounds.y + bounds.h).toBeLessThanOrEqual(viewport.y + viewport.height + 0.02);
  }
});

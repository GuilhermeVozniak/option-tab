import { expect, test } from "@playwright/test";
import { getCallRecords, installFakeWails, showState } from "./support/fakeWails";

test("Dock route keeps session targets and displays current native failures", async ({ page }) => {
  await installFakeWails(page);
  await page.goto("/#dock");
  const base = showState() as any;
  const state = {
    open: true,
    revision: 1,
    session: 7,
    item: {
      appId: 10,
      bundleId: "a",
      path: "/A.app",
      title: "Example",
      bounds: { x: 0, y: 0, w: 40, h: 40 },
      screenId: 1,
      edge: "bottom",
      kind: "app",
    },
    entries: [{ ...base.entries[0], windowId: 102, appId: 10, title: "Document B" }],
    selectedWindowId: 102,
    appearance: { ...base.appearance, showWindowControls: true },
    emptyReason: "",
  };
  await page.evaluate((s) => {
    const w = window as any;
    w.__dockState = s;
    w._wails.dispatchWailsEvent({ name: "dock:show", data: s });
  }, state);
  await expect(page.getByRole("button", { name: "Focus Document B" })).toBeVisible();
  await page.locator(".ot-dock-list article").hover();
  await page.getByLabel("Close window").click();
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual(["PerformDockAction", 7, "close", 102, 10]);
  await page.evaluate(() => {
    const w = window as any;
    w._wails.dispatchWailsEvent({
      name: "dock:error",
      data: { session: 7, revision: 2, message: "Accessibility denied" },
    });
  });
  await expect(page.getByRole("alert")).toContainText("Accessibility denied");
  await page.evaluate(() => {
    const w = window as any;
    w._wails.dispatchWailsEvent({ name: "dock:hide", data: { session: 6 } });
  });
  await expect(page.locator(".ot-dock-panel")).toBeVisible();
});

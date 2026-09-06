import { expect, test } from "@playwright/test";
import { getCallRecords, installFakeWails } from "./support/fakeWails";

test("native placement defaults unavailable by default and gives manual guidance", async ({
  page,
}) => {
  await installFakeWails(page);
  await page.goto("/#settings");
  await page.getByRole("tab", { name: "Dock" }).click();
  await expect(page.getByText(/Move the Dock to the selected display manually/)).toBeVisible();
  await expect(page.getByRole("button", { name: "Move Dock here" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Cancel placement" })).toHaveCount(0);
});

test("monitor lock preserves explicit main UUID and scopes placement to admitted runtime state", async ({
  page,
}) => {
  await installFakeWails(page);
  await page.addInitScript(() => {
    const display = {
      uuid: "11111111-1111-1111-1111-111111111111",
      id: 1,
      name: "Built-in Display",
      bounds: { x: 0, y: 0, w: 1440, h: 900 },
      scale: 2,
      main: true,
      mirrored: false,
    };
    (window as any).__dockLockDisplays = [display];
    (window as any).__dockLockState = {
      session: 7,
      revision: 8,
      generation: 9,
      sequence: 1,
      observedAtMs: 1,
      status: "disconnected",
      reason: "target disconnected",
      targetUUID: display.uuid,
      actualUUID: "",
      edge: "bottom",
      displays: [display],
      placementAvailable: true,
    };
    (window as any).__dockPlacementResult = {
      requestId: 2,
      status: "unreachable",
      reason: "edge unavailable",
      actualUUID: "",
      verified: false,
      cursorRestored: true,
    };
  });
  await page.goto("/#settings");
  await page.getByRole("tab", { name: "Dock" }).click();
  await page.getByLabel("Lock Dock to a monitor").check();
  const target = page.getByLabel("Target monitor");
  await expect(target.getByRole("option", { name: "Built-in Display" })).toHaveAttribute(
    "value",
    "11111111-1111-1111-1111-111111111111",
  );
  await target.selectOption("11111111-1111-1111-1111-111111111111");
  await expect(page.getByRole("button", { name: "Move Dock here" })).toBeDisabled();
  await expect
    .poll(async () =>
      (await getCallRecords(page)).some(
        ([name, json]) =>
          name === "SaveSettings" &&
          String(json).includes('"displayUUID":"11111111-1111-1111-1111-111111111111"'),
      ),
    )
    .toBe(true);

  await page.evaluate(() => {
    const w = window as any,
      old = w.__dockLockState;
    w._wails.dispatchWailsEvent({
      name: "dock:monitor-lock",
      data: { ...old, sequence: 2, status: "protected", reason: "" },
    });
  });
  await expect(page.getByRole("button", { name: "Move Dock here" })).toBeEnabled();
  await page.getByRole("button", { name: "Move Dock here" }).click();
  await expect
    .poll(async () =>
      (await getCallRecords(page)).some(
        (call) => JSON.stringify(call) === JSON.stringify(["PlaceDockOnSelectedMonitor", 7, 8, 9]),
      ),
    )
    .toBe(true);
  await expect(page.getByRole("alert")).toHaveText("edge unavailable");
});

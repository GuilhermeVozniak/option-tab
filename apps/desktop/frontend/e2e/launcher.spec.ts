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
  items: visible ? [{ id: `opaque-${name}`, name, icon: "" }] : [],
  widgets: [],
});

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

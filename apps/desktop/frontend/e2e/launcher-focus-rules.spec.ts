import { expect, test } from "@playwright/test";
import { getCallRecords, installFakeWails } from "./support/fakeWails";

test("builds an ordered exact-app focus rule from the settings inventory", async ({ page }) => {
  await installFakeWails(page);
  await page.goto("/#settings");
  await page.getByRole("tab", { name: "Dock" }).click();

  await page.getByRole("button", { name: "Add focus rule" }).click();
  const app = page.getByLabel("Running app");
  await expect(app.locator("option")).toHaveCount(3);
  await app.selectOption("org.example.editor");
  await expect(page.getByLabel("Exact bundle identifier org.example.editor")).toHaveValue(
    "org.example.editor",
  );
  await expect(page.getByText("Priority 1")).toBeVisible();

  await expect
    .poll(async () => {
      const calls = await getCallRecords(page);
      const saves = calls.filter(([name]) => name === "SaveSettingsAtRevision");
      const raw = saves.at(-1)?.[1];
      if (typeof raw !== "string") return "";
      return JSON.parse(raw).replacementDock.rules[0].bundleID;
    })
    .toBe("org.example.editor");
});

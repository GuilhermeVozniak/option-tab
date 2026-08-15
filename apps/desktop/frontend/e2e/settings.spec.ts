import { expect, test } from "@playwright/test";

// The #settings route renders the full preferences form with no Wails backend
// (settings start from defaults, permissions/crash are absent), so the whole
// tabbed surface is drivable directly in a browser.
test.describe("preferences (#settings route)", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/#settings");
  });

  const TABS = ["General", "Controls", "Appearance", "Filtering", "Blacklists", "About"];

  test("renders the tabbed preferences with every section", async ({ page }) => {
    await expect(page.getByRole("heading", { name: "Option Tab — Preferences" })).toBeVisible();
    for (const name of TABS) {
      await expect(page.getByRole("tab", { name })).toBeVisible();
    }
  });

  test("omits the Permissions section without Wails", async ({ page }) => {
    await expect(page.getByText("Option Tab needs these macOS permissions to work.")).toHaveCount(
      0,
    );
  });

  test("navigates between tabs", async ({ page }) => {
    const appearance = page.getByRole("tab", { name: "Appearance" });
    await appearance.click();
    await expect(appearance).toHaveAttribute("aria-selected", "true");
    await expect(page.getByLabel("Background blur")).toBeVisible();
  });

  test("toggles an appearance control", async ({ page }) => {
    await page.getByRole("tab", { name: "Appearance" }).click();
    const blur = page.getByLabel("Background blur");
    const before = await blur.isChecked();
    await blur.click();
    expect(await blur.isChecked()).toBe(!before);
  });

  test("adds a keyboard shortcut (lowest free id)", async ({ page }) => {
    await page.getByRole("tab", { name: "Controls" }).click();
    await expect(page.getByLabel("Remove shortcut 3")).toHaveCount(0);
    await page.getByRole("button", { name: "+ Add shortcut" }).click();
    await expect(page.getByLabel("Remove shortcut 3")).toBeVisible();
  });

  test("adds, edits and removes a blacklist entry", async ({ page }) => {
    await page.getByRole("tab", { name: "Blacklists" }).click();
    await page.getByRole("button", { name: "+ Add app" }).click();
    // exact: otherwise "Blacklist entry 1" also matches "Remove blacklist entry 1".
    const entry = page.getByLabel("Blacklist entry 1", { exact: true });
    await entry.fill("com.example.App");
    await expect(entry).toHaveValue("com.example.App");
    await page.getByLabel("Remove blacklist entry 1").click();
    await expect(page.getByLabel("Blacklist entry 1", { exact: true })).toHaveCount(0);
  });

  test("switches the interface language", async ({ page }) => {
    await page.getByRole("tab", { name: "General" }).click();
    const lang = page.getByLabel("Language");
    await lang.selectOption("pt-BR");
    await expect(lang).toHaveValue("pt-BR");
  });

  test("exposes export / import / reset actions", async ({ page }) => {
    await page.getByRole("tab", { name: "General" }).click();
    await expect(page.getByLabel("Export settings")).toBeVisible();
    await expect(page.getByLabel("Import settings")).toBeVisible();
    await expect(page.getByLabel("Reset to defaults")).toBeVisible();
  });

  test("keeps the update banner global and jumps to the update settings", async ({ page }) => {
    await page.waitForFunction(
      () =>
        typeof (window as unknown as { _wails?: { dispatchWailsEvent?: unknown } })._wails
          ?.dispatchWailsEvent === "function",
    );
    const banner = page.getByText("Version v9.9.9 is available.");
    // The runtime's listener registry is module-private, so a lost race shows
    // up as the banner never appearing — dispatch until it lands.
    await expect(async () => {
      await page.evaluate(() => {
        (
          window as unknown as {
            _wails: { dispatchWailsEvent: (e: { name: string; data: unknown }) => void };
          }
        )._wails.dispatchWailsEvent({
          name: "update:available",
          data: { version: "v9.9.9", url: "https://example.com/rel" },
        });
      });
      await expect(banner).toBeVisible({ timeout: 500 });
    }).toPass();

    // App-level chrome: one banner, still there on any other tab.
    await expect(banner).toHaveCount(1);
    await page.getByRole("tab", { name: "Appearance" }).click();
    await expect(banner).toBeVisible();

    // Clicking it reveals the Updates section of the General tab.
    await page.getByLabel("Show update settings").click();
    await expect(page.getByRole("tab", { name: "General" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    await expect(page.getByRole("heading", { name: "Updates" })).toBeVisible();
    await expect(page.getByLabel("Check for updates now")).toBeVisible();
  });

  test("shows the About tab with the dev version fallback", async ({ page }) => {
    await page.getByRole("tab", { name: "About" }).click();
    await expect(page.getByText("Version dev")).toBeVisible();
    await expect(page.getByLabel("Support this project")).toBeVisible();
  });
});

import { expect, test } from "@playwright/test";
import { getCallRecords, installFakeWails } from "./support/fakeWails";

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
    await expect(page.getByLabel("Background blur", { exact: true })).toBeVisible();
  });

  test("toggles an appearance control", async ({ page }) => {
    await page.getByRole("tab", { name: "Appearance" }).click();
    const blur = page.getByLabel("Background blur", { exact: true });
    const before = await blur.isChecked();
    await blur.click();
    expect(await blur.isChecked()).toBe(!before);
  });

  test("switches appearance and controls to app-specific settings", async ({ page }) => {
    await page.getByRole("tab", { name: "Appearance" }).click();
    const mode = page.getByLabel("Switcher settings mode");
    await mode.selectOption("apps");
    await expect(mode).toHaveValue("apps");
    await expect(page.getByLabel("Layout direction", { exact: true })).toBeVisible();
    await page.getByRole("tab", { name: "Controls" }).click();
    await expect(page.getByLabel("Arrow keys")).toBeVisible();
  });

  test("exposes Dock enablement, timing, scope, and appearance settings", async ({ page }) => {
    await page.getByRole("tab", { name: "Dock" }).click();
    await expect(page.getByLabel("Enable Dock previews")).toBeVisible();
    await expect(page.getByLabel("Dock hover delay")).toBeVisible();
    await expect(page.getByText("Dock window list")).toBeVisible();
    await expect(page.getByLabel("Dock app scope")).toBeVisible();
    await expect(page.getByLabel("Dock max columns")).toBeVisible();
    await expect(page.getByText("Input and gestures")).toBeVisible();
    await page.getByLabel("Click Dock icon to hide app").click();
    await expect(page.getByLabel("Click Dock icon to hide app")).toBeChecked();
    await page.getByLabel("Swipe toward Dock").selectOption("fullscreen");
    await expect(page.getByLabel("Swipe toward Dock")).toHaveValue("fullscreen");
    await page.getByLabel("Aero Shake action").selectOption("closeOthers");
    await expect(page.getByLabel("Aero Shake action")).toHaveValue("closeOthers");
    await expect(
      page.getByText(
        "Precise trackpad scrolling powers preview swipes. macOS does not reliably expose the number of fingers.",
      ),
    ).toBeVisible();
  });

  test("adds a keyboard shortcut (lowest free id)", async ({ page }) => {
    await page.getByRole("tab", { name: "Controls" }).click();
    await expect(page.getByLabel("Remove shortcut 3")).toHaveCount(0);
    await page.getByRole("button", { name: "+ Add shortcut" }).click();
    await expect(page.getByLabel("Remove shortcut 3")).toBeVisible();
  });

  test("shows readable switcher action names", async ({ page }) => {
    await page.getByRole("tab", { name: "Controls" }).click();
    const action = page.getByLabel("Action for KeyW");
    await expect(action.locator('option[value="close"]')).toHaveText("Close");
    await expect(action.locator('option[value="fullscreen"]')).toHaveText("Fullscreen");
    await expect(action.locator('option[value="newWindow"]')).toHaveText("New window");
    await expect(action.locator('option[value="forceQuit"]')).toHaveText("Force quit");
    await expect(action.locator('option[value="closeAll"]')).toHaveText("Close all windows");
    await expect(action.locator('option[value="minimizeAll"]')).toHaveText("Minimize all windows");
    await expect(page.getByLabel("Middle click action").locator('option[value="none"]')).toHaveText(
      "None",
    );
  });

  test("replaces a physical action binding through ordinary typing", async ({ page }) => {
    await page.getByRole("tab", { name: "Controls" }).click();
    const input = page.getByLabel("Physical key KeyW");
    await input.focus();
    await input.press("ControlOrMeta+A");
    await input.pressSequentially("KeyX");
    await input.blur();
    await expect(page.getByLabel("Physical key KeyX")).toHaveValue("KeyX");
    await expect(page.getByLabel("Physical key KeyW")).toHaveCount(0);
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

test("adds, edits and removes a blacklist entry", async ({ page }) => {
  await installFakeWails(page);
  await page.goto("/#settings");
  const saves = async () =>
    (await getCallRecords(page)).filter(([name]) => name === "SaveSettingsAtRevision");
  const persistedBlacklist = () =>
    page.evaluate(() => {
      const w = window as unknown as { __settingsJSON: string };
      return JSON.parse(w.__settingsJSON).filters.appBlacklist;
    });

  await page.getByRole("tab", { name: "Blacklists" }).click();
  await page.getByRole("button", { name: "+ Add app" }).click();
  // exact: otherwise "Blacklist entry 1" also matches "Remove blacklist entry 1".
  const entry = page.getByLabel("Blacklist entry 1", { exact: true });
  const save = page.getByRole("button", { name: "Save app", exact: true });
  await expect(entry).toBeVisible();
  await expect(entry).toHaveValue("");
  await expect(save).toBeDisabled();
  await expect(page.getByRole("button", { name: "Cancel app", exact: true })).toBeVisible();
  await expect(page.getByLabel("Remove blacklist entry 1")).toHaveCount(0);
  await entry.fill("com.example.App");
  await page.getByLabel("Blacklist hide 1").selectOption("whenNoWindow");
  await page.getByLabel("Blacklist ignore shortcuts 1").check();
  await expect(entry).toHaveValue("com.example.App");
  await expect(save).toBeEnabled();
  expect(await saves()).toEqual([]);
  expect(await persistedBlacklist()).toEqual([]);

  await save.click();
  const persistedEntry = {
    match: "com.example.App",
    hide: "whenNoWindow",
    ignoreShortcuts: true,
  };
  await expect.poll(persistedBlacklist).toEqual([persistedEntry]);
  await expect(page.getByLabel("Remove blacklist entry 1")).toBeVisible();
  await expect(save).toHaveCount(0);

  await entry.fill("com.example.Edited");
  expect(await saves()).toHaveLength(1);
  expect(await persistedBlacklist()).toEqual([persistedEntry]);
  await entry.blur();
  await expect
    .poll(persistedBlacklist)
    .toEqual([{ ...persistedEntry, match: "com.example.Edited" }]);
  await expect.poll(saves).toHaveLength(2);

  // Native preferences refresh replaces an uncommitted edit with the saved snapshot.
  await entry.fill("Uncommitted edit");
  await page.evaluate(() => {
    const w = window as unknown as {
      __settingsJSON: string;
      __settingsRevision: number;
      _wails: { dispatchWailsEvent: (event: { name: string; data: unknown }) => void };
    };
    w._wails.dispatchWailsEvent({
      name: "prefs:settings",
      data: { json: w.__settingsJSON, revision: w.__settingsRevision, generation: 1 },
    });
  });
  await expect(entry).toHaveValue("com.example.Edited");
  expect(await saves()).toHaveLength(2);

  await page.getByLabel("Remove blacklist entry 1").click();
  await expect.poll(persistedBlacklist).toEqual([]);
  await expect.poll(saves).toHaveLength(3);
  await expect(entry).toHaveCount(0);
});

test("Dock appearance edits stay separate from window and app switcher preferences", async ({
  page,
}) => {
  await page.goto("/#settings");
  await page.getByRole("tab", { name: "Appearance" }).click();
  const windowSize = await page.getByLabel("Thumbnail size", { exact: true }).inputValue();
  await page.getByLabel("Switcher settings mode").selectOption("apps");
  const appSize = await page.getByLabel("Thumbnail size", { exact: true }).inputValue();
  await page.getByRole("tab", { name: "Dock" }).click();
  const dock = page.getByRole("region", { name: "Dock", exact: true });
  await dock.getByLabel("Dock thumbnail size", { exact: true }).fill("320");
  await dock.getByLabel("Dock layout direction", { exact: true }).selectOption("vertical");
  await dock.getByLabel("Dock theme light", { exact: true }).click();
  await dock.getByLabel("Dock auto-size thumbnails", { exact: true }).uncheck();
  await dock.getByLabel("Dock selected preview", { exact: true }).check();
  await expect(dock.getByLabel("Dock background opacity")).toBeVisible();
  await expect(dock.getByLabel("Dock background blur")).toBeVisible();
  await expect(
    dock.getByLabel(/Overlay placement|Fade out animation|Apparition delay/i),
  ).toHaveCount(0);
  await page.getByRole("tab", { name: "Appearance" }).click();
  await expect(page.getByLabel("Thumbnail size", { exact: true })).toHaveValue(appSize);
  await page.getByLabel("Switcher settings mode").selectOption("windows");
  await expect(page.getByLabel("Thumbnail size", { exact: true })).toHaveValue(windowSize);
  await page.getByRole("tab", { name: "Dock" }).click();
  await expect(dock.getByLabel("Dock thumbnail size")).toHaveValue("320");
  await expect(dock.getByLabel("Dock layout direction")).toHaveValue("vertical");
  await expect(dock.getByLabel("Dock theme light")).toHaveAttribute("aria-pressed", "true");
  await expect(dock.getByLabel("Dock auto-size thumbnails")).not.toBeChecked();
});

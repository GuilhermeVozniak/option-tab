import { expect, test } from "@playwright/test";
import { getCallRecords, installFakeWails } from "./support/fakeWails";

test("Settings theme follows the system, persists an override, and never changes feature settings", async ({
  page,
}) => {
  await installFakeWails(page);
  await page.emulateMedia({ colorScheme: "light" });
  await page.goto("/#settings");
  const settings = page.locator(".ot-settings");
  await expect(settings).toHaveCSS("color-scheme", "light");
  await page.emulateMedia({ colorScheme: "dark" });
  await expect(settings).toHaveCSS("color-scheme", "dark");
  await page.getByRole("radio", { name: "Settings theme light" }).click();
  await expect(settings).toHaveCSS("color-scheme", "light");
  await page.reload();
  await expect(settings).toHaveAttribute("data-theme", "light");
  await expect(settings).toHaveCSS("color-scheme", "light");
  await page.getByRole("radio", { name: "Settings theme dark" }).click();
  await expect(settings).toHaveCSS("color-scheme", "dark");
  expect(
    (await getCallRecords(page)).filter(([name]) => name === "SaveSettingsAtRevision"),
  ).toEqual([]);
});

test("Settings theme and appearance slider support keyboard changes with the correct persistence", async ({
  page,
}) => {
  await installFakeWails(page);
  await page.goto("/#settings");
  const system = page.getByRole("radio", { name: "Settings theme system" });
  await expect(system).toBeEnabled();
  await system.focus();
  await page.keyboard.press("ArrowRight");
  const light = page.getByRole("radio", { name: "Settings theme light" });
  await expect(light).toBeFocused();
  await page.keyboard.press("Space");
  await expect(light).toBeChecked();
  await expect(page.locator(".ot-settings")).toHaveAttribute("data-theme", "light");
  expect(
    (await getCallRecords(page)).filter(([name]) => name === "SaveSettingsAtRevision"),
  ).toEqual([]);

  await page.getByRole("tab", { name: "Appearance", exact: true }).click();
  const opacity = page.getByRole("slider", { name: "Background opacity", exact: true });
  const nextOpacity = Number(
    (Number(await opacity.getAttribute("aria-valuenow")) + 0.05).toFixed(2),
  );
  await opacity.focus();
  await page.keyboard.press("ArrowRight");
  await expect(opacity).toHaveAttribute("aria-valuenow", String(nextOpacity));
  await expect
    .poll(async () =>
      (await getCallRecords(page))
        .filter(([name]) => name === "SaveSettingsAtRevision")
        .map(([, payload]) => JSON.parse(payload as string).appearance.backgroundOpacity),
    )
    .toEqual([nextOpacity]);
});

test("sidebar supports keyboard navigation and keeps the selected panel accessible", async ({
  page,
  browserName,
}) => {
  await page.goto("/#settings");
  const general = page.getByRole("tab", { name: "General", exact: true });
  // Settings intentionally disables its form while the canonical snapshot loads.
  // focus() does not wait for enabled state the way click() does.
  await expect(general).toBeEnabled();
  await general.focus();
  await page.keyboard.press("ArrowDown");
  const shortcuts = page.getByRole("tab", { name: "Shortcuts", exact: true });
  await expect(shortcuts).toBeFocused();
  await expect(shortcuts).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("tabpanel")).toHaveCount(1);
  await page.keyboard.press("End");
  await expect(page.getByRole("tab", { name: "About", exact: true })).toBeFocused();
  await page.keyboard.press("Home");
  await expect(page.getByRole("tab", { name: "General", exact: true })).toBeFocused();
  // macOS WebKit uses Option+Tab to include buttons in native focus traversal.
  await page.keyboard.press(
    browserName === "webkit" && process.platform === "darwin" ? "Alt+Tab" : "Tab",
  );
  await expect(page.getByRole("radio", { name: "Settings theme system" })).toBeFocused();
});

for (const width of [390, 720, 900, 1280]) {
  test(`Settings pages and Dock sections fit a ${width}px window in both themes`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 720 });
    await page.goto("/#settings");
    await expect(page.getByRole("tablist", { name: "Settings sections" })).toHaveAttribute(
      "aria-orientation",
      width <= 560 ? "horizontal" : "vertical",
    );
    for (const theme of ["light", "dark"]) {
      await page.getByRole("radio", { name: `Settings theme ${theme}` }).click();
      for (const label of [
        "General",
        "Shortcuts",
        "Appearance",
        "Window rules",
        "Excluded apps",
        "Dock",
        "About",
      ]) {
        await page.getByRole("tab", { name: label, exact: true }).click();
        await expect(page.getByRole("heading", { name: label, level: 1 })).toBeVisible();
        expect(
          await page.locator(".ot-settings-main").evaluate((el) => el.scrollWidth - el.clientWidth),
          `${label} / ${theme}`,
        ).toBeLessThanOrEqual(1);
      }
      await page.getByRole("tab", { name: "Dock", exact: true }).click();
      const sections = page.getByRole("navigation", { name: "Dock sections", exact: true });
      for (const label of ["Launcher", "Window previews", "Media", "Monitor"]) {
        await sections.getByRole("button", { name: label, exact: true }).click();
        expect(
          await page.locator(".ot-settings-main").evaluate((el) => el.scrollWidth - el.clientWidth),
          `${label} / ${theme}`,
        ).toBeLessThanOrEqual(1);
      }
    }
  });
}

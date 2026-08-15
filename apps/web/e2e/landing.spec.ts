import { expect, test } from "@playwright/test";
import { APP_VERSION } from "../lib/download";

// Expectations derive from APP_VERSION so a release bump cannot break them.
const v = APP_VERSION.replaceAll(".", "\\.");
const macDmg = new RegExp(`option-tab_${v}_darwin_arm64\\.dmg$`);

test("landing page renders and exposes per-OS download links", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Option Tab" })).toBeVisible();

  const macLink = page.getByTestId("download-darwin");
  await expect(macLink).toHaveAttribute(
    "href",
    new RegExp(`/releases/download/v${v}/option-tab_${v}_darwin_arm64\\.dmg$`),
  );
});

test.describe("primary download (platform detection)", () => {
  test.use({ userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)" });

  test("recommends the macOS build for a mac user agent", async ({ page }) => {
    await page.goto("/");
    const primary = page.getByTestId("primary-download");
    await expect(primary).toHaveAttribute("data-platform", "darwin");
    await expect(primary).toHaveAttribute("href", macDmg);
  });
});

test("shows the feature showcase including the three visual styles", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Three ways to switch" })).toBeVisible();
  for (const style of ["Thumbnails", "App Icons", "Titles"]) {
    await expect(page.getByRole("heading", { name: style })).toBeVisible();
  }
  await expect(page.getByRole("heading", { name: /including the paid features/i })).toBeVisible();
});

test("links to the GitHub source", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("link", { name: "Source on GitHub" })).toHaveAttribute(
    "href",
    "https://github.com/GuilhermeVozniak/option-tab",
  );
});

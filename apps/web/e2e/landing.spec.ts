import { expect, test } from "@playwright/test";

test("landing page renders and exposes per-OS download links", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Option Tab" })).toBeVisible();

  const macLink = page.getByTestId("download-darwin");
  await expect(macLink).toHaveAttribute("href", /\/releases\/download\/v0\.1\.0\/option-tab_0\.1\.0_darwin_arm64\.dmg$/);
});

test.describe("primary download (platform detection)", () => {
  test.use({ userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)" });

  test("recommends the macOS build for a mac user agent", async ({ page }) => {
    await page.goto("/");
    const primary = page.getByTestId("primary-download");
    await expect(primary).toHaveAttribute("data-platform", "darwin");
    await expect(primary).toHaveAttribute("href", /option-tab_0\.1\.0_darwin_arm64\.dmg$/);
  });
});

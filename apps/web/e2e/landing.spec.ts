import { expect, test } from "@playwright/test";
import { APP_VERSION } from "../lib/download";
import { featureSections } from "../lib/feature-guide";

const v = APP_VERSION.replaceAll(".", "\\.");
const macDmg = new RegExp(`option-tab_${v}_darwin_universal\\.dmg$`);

test("landing page exposes the published macOS download and labels demo builds", async ({
  page,
}) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Option Tab", exact: true })).toBeVisible();
  const macLink = page.getByTestId("download-darwin");
  await expect(macLink).toContainText("Apple silicon");
  await expect(macLink).toContainText("Intel");
  await expect(page.getByText(/macOS 14\+/)).toBeVisible();
  await expect(macLink).toHaveAttribute(
    "href",
    new RegExp(`/releases/download/v${v}/option-tab_${v}_darwin_universal\\.dmg$`),
  );
  await page.getByText("Other platforms", { exact: true }).click();
  await expect(
    page.getByText("Windows and Linux demo builds; native window switching is macOS-only."),
  ).toBeVisible();
  await expect(page.getByTestId("download-windows")).toContainText("demo");
  await expect(page.getByTestId("download-linux")).toContainText("demo");
});

test.describe("primary download (platform detection)", () => {
  test.use({ userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)" });
  test("recommends the macOS build for a mac user agent", async ({ page }) => {
    await page.emulateMedia({ colorScheme: "light" });
    await page.goto("/");
    const primary = page.getByTestId("primary-download");
    await expect(primary).toHaveAttribute("data-platform", "darwin");
    await expect(primary).toHaveAttribute("href", macDmg);
    await expect(primary).toHaveCSS("color", "rgb(255, 255, 255)");
    await expect(primary).toHaveCSS("background-color", "rgb(32, 32, 32)");
    await page.getByRole("combobox", { name: "Website theme" }).selectOption("dark");
    await expect(primary).toHaveCSS("color", "rgb(22, 22, 22)");
    await expect(primary).toHaveCSS("background-color", "rgb(239, 239, 239)");
  });
});

test("explains the default and links every feature category to its guide", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Your windows, by default." })).toBeVisible();
  await expect(page.getByText(/no separate enlarged preview/)).toBeVisible();
  for (const style of ["Thumbnails", "App icons", "Titles"]) {
    await expect(page.getByRole("heading", { name: style, exact: true })).toBeVisible();
  }
  for (const section of featureSections) {
    await expect(
      page
        .locator(".feature-category")
        .filter({ has: page.getByRole("heading", { name: section.title, exact: true }) }),
    ).toHaveAttribute("href", `/docs#${section.id}`);
  }
  await expect(
    page.getByText(/every Pro feature|including the paid features|every AltTab Pro/),
  ).toHaveCount(0);
});

test("uses the app brand and links the source and Homebrew install", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("link", { name: "Option Tab home" }).locator("img")).toHaveAttribute(
    "src",
    "/option-tab.svg",
  );
  await expect(page.getByRole("link", { name: "Source on GitHub" })).toHaveAttribute(
    "href",
    "https://github.com/GuilhermeVozniak/option-tab",
  );
  await expect(
    page.getByText("brew install --cask GuilhermeVozniak/tap/option-tab", { exact: true }),
  ).toBeVisible();
});

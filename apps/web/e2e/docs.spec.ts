import { expect, test } from "@playwright/test";
import { featureSections } from "../lib/feature-guide";

test("the public guide renders every feature and its instructions", async ({ page }) => {
  await page.goto("/docs");
  await expect(page.getByRole("heading", { name: "User guide", exact: true })).toBeVisible();
  for (const section of featureSections) {
    const content = page.locator(`section#${section.id}`);
    await expect(content.getByRole("heading", { name: section.title, exact: true })).toBeVisible();
    for (const feature of section.features) {
      const article = content.locator(`article#${feature.id}`);
      await expect(
        article.getByRole("heading", { name: feature.title, exact: true }),
      ).toBeVisible();
      await expect(article.getByRole("listitem")).toHaveCount(feature.howTo.length);
    }
  }
});

test("guide navigation and links from the landing page reach the right section", async ({
  page,
}) => {
  await page.goto("/");
  await page.locator('.feature-category[href="/docs#dock-previews"]').click();
  await expect(page).toHaveURL(/\/docs#dock-previews$/);
  await expect(page.locator("section#dock-previews")).toBeInViewport();
  await page
    .locator(".guide-sidebar")
    .getByRole("link", { name: featureSections.at(-1)!.title, exact: true })
    .click();
  await expect(page.locator(`section#${featureSections.at(-1)!.id}`)).toBeInViewport();
});

test("search finds settings instructions and recovers from no matches", async ({ page }) => {
  await page.goto("/docs");
  const search = page.getByRole("searchbox", { name: "Find a feature or setting" });
  await search.fill("Dock previews");
  await expect(page.getByRole("status")).toContainText("matching");
  await expect(page.locator("article.guide-feature").first()).toBeVisible();
  await search.fill("no-such-feature-123");
  await expect(page.getByRole("heading", { name: "No matching topics" })).toBeVisible();
  await expect(page.getByRole("status")).toHaveText("0 matching topics");
  await page.getByRole("button", { name: "Clear", exact: true }).click();
  await expect(page.locator("article.guide-feature")).toHaveCount(
    featureSections.reduce((total, section) => total + section.features.length, 0),
  );
});

test("website theme persists across pages and follows System changes", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "light" });
  await page.goto("/");
  const theme = page.getByRole("combobox", { name: "Website theme" });
  await expect(page.locator("body")).toHaveCSS("background-color", "rgb(255, 255, 255)");
  await theme.selectOption("dark");
  await expect(page.locator("body")).toHaveCSS("background-color", "rgb(23, 23, 23)");
  await page
    .getByRole("navigation", { name: "Main navigation" })
    .getByRole("link", { name: "Guide", exact: true })
    .click();
  await expect(theme).toHaveValue("dark");
  await page.reload();
  await expect(theme).toHaveValue("dark");
  await expect(page.locator("body")).toHaveCSS("background-color", "rgb(23, 23, 23)");
  await theme.selectOption("system");
  await expect(page.locator("body")).toHaveCSS("background-color", "rgb(255, 255, 255)");
  await page.emulateMedia({ colorScheme: "dark" });
  await expect(page.locator("body")).toHaveCSS("background-color", "rgb(23, 23, 23)");
  await theme.selectOption("light");
  await expect(page.locator("body")).toHaveCSS("background-color", "rgb(255, 255, 255)");
});

test("theme selection remains usable without browser storage", async ({ page }) => {
  await page.addInitScript(() => {
    Storage.prototype.getItem = () => {
      throw new Error("Storage unavailable");
    };
    Storage.prototype.setItem = () => {
      throw new Error("Storage unavailable");
    };
  });
  await page.goto("/docs");
  await page.getByRole("combobox", { name: "Website theme" }).selectOption("dark");
  await expect(page.locator("body")).toHaveCSS("background-color", "rgb(23, 23, 23)");
});

test.describe("mobile guide", () => {
  test.use({ viewport: { width: 390, height: 844 }, isMobile: true });
  test("keeps navigation and feature instructions within the viewport", async ({ page }) => {
    await page.goto("/docs");
    await page.getByText("Browse guide sections", { exact: true }).click();
    await page
      .locator(".guide-mobile-nav")
      .getByRole("link", {
        name: featureSections.find((section) => section.id === "widgets")!.title,
        exact: true,
      })
      .click();
    await expect(page.locator("section#widgets")).toBeInViewport();
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
    ).toBe(true);
    await page.goto("/");
    await expect(page.getByRole("heading", { name: "Option Tab", exact: true })).toBeVisible();
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
    ).toBe(true);
  });
});

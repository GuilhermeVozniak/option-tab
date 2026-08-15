import { expect, test } from "@playwright/test";
import { emitShow, getCalls, installFakeWails, showState } from "./support/fakeWails";

// ---- Static rendering via the built-in #demo route (no backend needed) ----
test.describe("overlay — visual styles (demo route)", () => {
  for (const [hash, style] of [
    ["#demo", "thumbnails"],
    ["#demo:appIcons", "appIcons"],
    ["#demo:titles", "titles"],
  ] as const) {
    test(`renders the ${style} style with every demo window`, async ({ page }) => {
      await page.goto(`/${hash}`);
      const overlay = page.locator(".ot-overlay");
      await expect(overlay).toBeVisible();
      await expect(overlay).toHaveAttribute("data-style", style);
      await expect(page.getByRole("option")).toHaveCount(5);
    });
  }

  test("shows the selected-window preview under the grid", async ({ page }) => {
    await page.goto("/#demo");
    const preview = page.locator(".ot-preview-img");
    await expect(preview).toBeVisible();
    // The preview belongs to the selected entry, inside the panel.
    await expect(page.locator(".ot-panel .ot-preview")).toHaveCount(1);
  });

  test("marks the selected entry and renders status icons", async ({ page }) => {
    await page.goto("/#demo");
    await expect(page.locator(".ot-entry.ot-selected")).toHaveCount(1);
    // demo includes a minimized window and one on another Space
    await expect(page.locator(".ot-status").first()).toBeVisible();
  });

  test("reveals close/minimize/fullscreen/hide/quit controls on hover", async ({ page }) => {
    await page.goto("/#demo:titles");
    await page.getByRole("option").first().hover();
    await expect(page.getByLabel("Close window").first()).toBeVisible();
    await expect(page.getByLabel("Minimize window").first()).toBeVisible();
    await expect(page.getByLabel("Fullscreen window").first()).toBeVisible();
    await expect(page.getByLabel("Hide app").first()).toBeVisible();
    await expect(page.getByLabel("Quit app").first()).toBeVisible();
  });
});

// ---- Interactive behavior via an injected fake Wails runtime ----
test.describe("overlay — interactive", () => {
  test.beforeEach(async ({ page }) => {
    await installFakeWails(page);
    await page.goto("/");
  });

  test("Tab advances and Shift+Tab reverses the selection", async ({ page }) => {
    await emitShow(page, showState({ selected: 0 }));
    const options = page.getByRole("option");
    await expect(options.nth(0)).toHaveAttribute("aria-selected", "true");
    await page.keyboard.press("Tab");
    await expect(options.nth(1)).toHaveAttribute("aria-selected", "true");
    await page.keyboard.press("Shift+Tab");
    await expect(options.nth(0)).toHaveAttribute("aria-selected", "true");
  });

  test("arrow keys navigate", async ({ page }) => {
    await emitShow(page, showState({ selected: 0 }));
    const options = page.getByRole("option");
    await expect(options.nth(0)).toHaveAttribute("aria-selected", "true");
    await page.keyboard.press("ArrowRight");
    await expect(options.nth(1)).toHaveAttribute("aria-selected", "true");
    await page.keyboard.press("ArrowLeft");
    await expect(options.nth(0)).toHaveAttribute("aria-selected", "true");
  });

  test("vim keys navigate only when enabled", async ({ page }) => {
    await emitShow(page, showState({ selected: 0, vimKeys: true }));
    const options = page.getByRole("option");
    await expect(options.nth(0)).toHaveAttribute("aria-selected", "true");
    await page.keyboard.press("j");
    await expect(options.nth(1)).toHaveAttribute("aria-selected", "true");
    await page.keyboard.press("k");
    await expect(options.nth(0)).toHaveAttribute("aria-selected", "true");
  });

  test("type-to-search updates the query line", async ({ page }) => {
    await emitShow(page, showState({}));
    await expect(page.locator(".ot-overlay")).toBeVisible();
    await page.keyboard.press("g");
    await expect(page.locator(".ot-search")).toContainText("g");
  });

  test("Escape cancels and dismisses the switcher", async ({ page }) => {
    await emitShow(page, showState({}));
    await expect(page.locator(".ot-overlay")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page.locator(".ot-overlay")).toHaveCount(0);
    expect(await getCalls(page)).toContain("Cancel");
  });

  test("clicking an entry confirms the selection", async ({ page }) => {
    await emitShow(page, showState({}));
    await page.getByRole("option").nth(1).click();
    expect(await getCalls(page)).toContain("Confirm");
  });

  test("hover controls call the matching window/app actions", async ({ page }) => {
    await emitShow(page, showState({}));
    const first = page.getByRole("option").first();
    const actions: [string, string][] = [
      ["Close window", "CloseSelected"],
      ["Minimize window", "MinimizeSelected"],
      ["Fullscreen window", "FullscreenSelected"],
      ["Hide app", "HideSelectedApp"],
      ["Quit app", "QuitSelectedApp"],
    ];
    for (const [label] of actions) {
      await first.hover();
      await page.getByLabel(label).first().click();
    }
    await page.waitForFunction(() => {
      const calls = ((window as any).__calls || []).map((c: unknown[]) => c[0]);
      return [
        "CloseSelected",
        "MinimizeSelected",
        "FullscreenSelected",
        "HideSelectedApp",
        "QuitSelectedApp",
      ].every((n) => calls.includes(n));
    });
  });

  test("blur knob toggles the frosted-glass class", async ({ page }) => {
    await emitShow(page, showState({ appearance: { ...showState().appearance, blur: false } }));
    await expect(page.locator(".ot-overlay.ot-no-blur")).toHaveCount(1);
  });
});

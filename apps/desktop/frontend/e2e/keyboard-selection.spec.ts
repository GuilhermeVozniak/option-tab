import { expect, test } from "@playwright/test";
import { emptyState } from "../src/lib/types";
import { emitShow, installFakeWails, showState } from "./support/fakeWails";

for (const mode of ["apps", "windows"] as const) {
  test(`${mode}: keyboard selection stays visible in overflowing lists`, async ({ page }) => {
    await page.setViewportSize({ width: 900, height: 600 });
    await installFakeWails(page);
    await page.goto("/");
    const entries = Array.from({ length: 30 }, (_, i) => ({
      windowId: i + 1,
      appId: i + 100,
      appName: `App ${i}`,
      bundleId: `app.${i}`,
      title: `Window ${i}`,
      minimized: false,
      hidden: false,
      fullscreen: false,
    }));
    await emitShow(
      page,
      showState({
        ...emptyState,
        open: true,
        mode,
        entries: mode === "apps" ? [] : entries,
        apps: entries.map((e) => ({ ...e, windowCount: 0, windowPresence: "empty" })),
        mouseHover: false,
        style: "titles",
        appearance: { ...emptyState.appearance, style: "titles", fadeOutAnimation: false },
      }),
    );
    for (let i = 1; i < entries.length; i++) {
      // The native overlay never takes keyboard focus: DOM Tab must not scroll it.
      await page.evaluate(() =>
        (window as any)._wails.dispatchWailsEvent({
          name: "switcher:key",
          data: {
            session: 0,
            key: "Tab",
            code: "Tab",
            shift: false,
            ctrl: false,
            alt: true,
            meta: false,
          },
        }),
      );
      await page.waitForFunction((expected) => (window as any).__state.selected === expected, i);
    }
    const selected = page.locator('[aria-selected="true"]');
    await expect(selected).toContainText("29");
    await expect
      .poll(() =>
        selected.evaluate((element) => {
          const scroller = element.closest(".ot-app-rail, .ot-panel")!;
          const item = element.getBoundingClientRect();
          const clip = scroller.getBoundingClientRect();
          return (
            item.left >= clip.left &&
            item.right <= clip.right &&
            item.top >= clip.top &&
            item.bottom <= clip.bottom
          );
        }),
      )
      .toBe(true);
  });
}

test("app preview gallery scrolls its selected window without taking focus", async ({ page }) => {
  await page.setViewportSize({ width: 900, height: 600 });
  await installFakeWails(page);
  await page.goto("/");
  const entries = Array.from({ length: 30 }, (_, i) => ({
    windowId: i + 1,
    appId: 100,
    appName: "Editor",
    bundleId: "editor",
    title: `Window ${i}`,
    minimized: false,
    hidden: false,
    fullscreen: false,
  }));
  await emitShow(
    page,
    showState({
      ...emptyState,
      open: true,
      mode: "apps",
      entries,
      selectedWindowId: 1,
      apps: [{ appId: 100, appName: "Editor", bundleId: "editor", windowCount: 30 }],
      mouseHover: false,
      style: "titles",
      appearance: {
        ...emptyState.appearance,
        style: "titles",
        maxColumns: 1,
        maxRows: 3,
        fadeOutAnimation: false,
      },
    }),
  );
  await page.evaluate(() => {
    const w = window as any;
    w.__state = { ...w.__state, selectedWindowId: 30 };
    w._wails.dispatchWailsEvent({ name: "switcher:update", data: w.__state });
  });
  const selected = page.locator(".ot-app-gallery > .is-selected");
  await expect(selected).toContainText("Window 29");
  await expect
    .poll(() =>
      selected.evaluate((element) => {
        const item = element.getBoundingClientRect();
        const clip = element.parentElement!.getBoundingClientRect();
        return item.top >= clip.top && item.bottom <= clip.bottom;
      }),
    )
    .toBe(true);
  await expect(page.getByRole("button", { name: "Focus Window 29" })).not.toBeFocused();
});

for (const mode of ["windows", "apps"] as const) {
  test(`${mode}: a native key burst survives asynchronous search replies`, async ({ page }) => {
    await installFakeWails(page);
    await page.goto("/");
    await emitShow(page, showState({ ...emptyState, open: true, mode, session: 41, revision: 1 }));
    await page.evaluate(() => {
      const w = window as any;
      for (const key of ["a", "b", "c"])
        w._wails.dispatchWailsEvent({
          name: "switcher:key",
          data: {
            session: 41,
            key,
            code: `Key${key.toUpperCase()}`,
            shift: false,
            ctrl: false,
            alt: false,
            meta: false,
          },
        });
    });
    await expect(page.locator(mode === "apps" ? ".ot-app-search" : ".ot-search")).toContainText(
      "abc",
    );
    await page.waitForFunction(() => (window as any).__state.searchSequence === 3);
  });
}

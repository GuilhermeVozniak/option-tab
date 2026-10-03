import { expect, test } from "@playwright/test";
import { emitShow, getCallRecords, installFakeWails, showState } from "./support/fakeWails";

test.beforeEach(async ({ page }) => {
  await installFakeWails(page);
});

test("reports the exact interior panel and admits only scoped native status", async ({ page }) => {
  await page.setViewportSize({ width: 420, height: 360 });
  await page.goto("/");
  const base = showState();
  await emitShow(
    page,
    showState({
      session: 81,
      revision: 4,
      appearance: { ...(base.appearance as object), blur: true },
    }),
  );

  const panel = page.locator(".ot-panel");
  await expect(panel).toHaveClass(/ot-solid-material/);
  await expect
    .poll(
      async () =>
        (await getCallRecords(page)).filter((call) => call[0] === "SetSwitcherMaterialRect").length,
    )
    .toBeGreaterThan(0);
  await expect
    .poll(async () => {
      const bounds = await panel.boundingBox();
      const reports = (await getCallRecords(page)).filter(
        (call) => call[0] === "SetSwitcherMaterialRect",
      );
      const latest = reports.at(-1);
      if (!bounds || !latest) return false;
      return (
        latest[1] === 81 &&
        latest[2] === 4 &&
        Math.abs(Number(latest[4]) - bounds.x) < 0.5 &&
        Math.abs(Number(latest[5]) - bounds.y) < 0.5 &&
        Math.abs(Number(latest[6]) - bounds.width) < 0.5 &&
        Math.abs(Number(latest[7]) - bounds.height) < 0.5
      );
    })
    .toBe(true);

  await page.evaluate(() =>
    (window as any)._wails.dispatchWailsEvent({
      name: "switcher:material",
      data: { session: 81, revision: 2, state: "system" },
    }),
  );
  await expect(panel).toHaveClass(/ot-native-material/);
  await page.evaluate(() => {
    (window as any)._wails.dispatchWailsEvent({
      name: "switcher:material",
      data: { session: 81, revision: 1, state: "solid" },
    });
    const state = { ...(window as any).__state, revision: 5 };
    (window as any).__state = state;
    (window as any)._wails.dispatchWailsEvent({ name: "switcher:update", data: state });
  });
  await expect(panel).toHaveClass(/ot-native-material/);
  await expect
    .poll(async () =>
      (await getCallRecords(page)).some(
        (call) => call[0] === "SetSwitcherMaterialRect" && call[1] === 81 && call[2] === 5,
      ),
    )
    .toBe(true);

  await emitShow(page, showState({ session: 82, revision: 1 }));
  await expect(panel).toHaveClass(/ot-solid-material/);
  await page.evaluate(() =>
    (window as any)._wails.dispatchWailsEvent({
      name: "switcher:material",
      data: { session: 81, revision: 99, state: "system" },
    }),
  );
  await expect(panel).toHaveClass(/ot-solid-material/);
});

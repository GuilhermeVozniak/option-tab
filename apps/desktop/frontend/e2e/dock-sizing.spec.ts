import { expect, type Page, test } from "@playwright/test";
import { installFakeWails, showState } from "./support/fakeWails";

const emit = (page: Page, name: string, data: unknown) =>
  page.evaluate(
    ({ name, data }) =>
      (
        window as unknown as { _wails: { dispatchWailsEvent(event: unknown): void } }
      )._wails.dispatchWailsEvent({ name, data }),
    { name, data },
  );

test("Dock selected preview follows live sizing and Auto-size without viewport feedback", async ({
  page,
}) => {
  await installFakeWails(page);
  await page.setViewportSize({ width: 320, height: 420 });
  await page.goto("/#dock");
  const base = showState();
  const entries = Array.from({ length: 8 }, (_, i) => ({
    ...base.entries[0],
    windowId: i + 1,
    appId: 10,
    title: `Document ${i + 1}`,
  }));
  const appearance = {
    ...base.appearance,
    previewSelected: true,
    thumbnailMaxPx: 240,
    maxColumns: 2,
    maxRows: 2,
    autoSize: false,
  };
  const state = {
    session: 77,
    revision: 1,
    open: true,
    item: {
      appId: 10,
      bundleId: "sizing.app",
      path: "/Sizing.app",
      title: "Sizing fixture",
      bounds: { x: 20, y: 20, w: 40, h: 40 },
      screenId: 1,
      edge: "bottom",
      kind: "app",
    },
    entries,
    selectedWindowId: 1,
    appearance,
    emptyReason: "",
  };
  await emit(page, "dock:show", state);
  await expect(page.getByRole("button", { name: "Focus Document 1" })).toBeVisible();
  const frame = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="320" height="180"><rect width="320" height="180" fill="#2563eb"/></svg>')}`;
  await emit(page, "dock:frames", { session: 77, frames: { "1": frame } });
  const preview = page.getByLabel("Selected window preview");
  const height = () => preview.evaluate((el) => el.getBoundingClientRect().height);
  await expect.poll(height).toBe(300);
  await expect(preview.locator("img")).toHaveCSS("object-fit", "contain");
  await emit(page, "dock:update", {
    ...state,
    revision: 2,
    appearance: { ...appearance, thumbnailMaxPx: 120 },
  });
  await expect.poll(height).toBe(150);
  await emit(page, "dock:update", {
    ...state,
    revision: 3,
    appearance: { ...appearance, autoSize: true },
  });
  await expect.poll(height).toBe(150);
  await emit(page, "dock:update", {
    ...state,
    revision: 4,
    appearance: { ...appearance, thumbnailMaxPx: 1024 },
  });
  await expect.poll(height).toBe(600);
  const panel = page.locator(".ot-dock-panel");
  const bounds = await panel.boundingBox();
  expect(bounds?.width).toBeLessThanOrEqual(320);
  expect(bounds?.height).toBeLessThanOrEqual(420);
  await page.setViewportSize({ width: 260, height: 360 });
  await expect.poll(height).toBe(600);
  expect((await panel.boundingBox())?.width).toBeLessThanOrEqual(260);
  expect((await panel.boundingBox())?.height).toBeLessThanOrEqual(360);
  await emit(page, "dock:update", {
    ...state,
    revision: 5,
    appearance: { ...appearance, thumbnailMaxPx: 64 },
  });
  await expect.poll(height).toBe(120);
  await expect(preview.locator("img")).toHaveAttribute("src", frame);
});

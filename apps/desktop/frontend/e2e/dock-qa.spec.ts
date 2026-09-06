import { expect, test } from "@playwright/test";
import { installFakeWails, showState } from "./support/fakeWails";

const entry = (id: number) => ({
  ...(showState() as any).entries[0],
  windowId: id,
  appId: 10,
  title: `Document ${id}`,
});
const state = (session: number, revision: number, overrides: Record<string, unknown> = {}) => {
  const base = showState() as any;
  return {
    session,
    revision,
    open: true,
    item: {
      appId: 10,
      bundleId: "qa.app",
      path: "/QA.app",
      title: "QA App",
      bounds: { x: 20, y: 20, w: 40, h: 40 },
      screenId: 1,
      edge: "bottom",
      kind: "app",
    },
    entries: [entry(1)],
    selectedWindowId: 1,
    appearance: { ...base.appearance },
    emptyReason: "",
    ...overrides,
  };
};
const emit = (page: any, name: string, data: unknown) =>
  page.evaluate(
    ([event, payload]: [string, unknown]) =>
      (window as any)._wails.dispatchWailsEvent({ name: event, data: payload }),
    [name, data],
  );

test.beforeEach(async ({ page }) => {
  await installFakeWails(page);
  await page.goto("/#dock");
});

test("renders streamed frames in horizontal and vertical constrained layouts", async ({ page }) => {
  await page.setViewportSize({ width: 720, height: 520 });
  const entries = Array.from({ length: 8 }, (_, i) => entry(i + 1));
  await emit(
    page,
    "dock:show",
    state(20, 1, {
      entries,
      appearance: {
        ...(showState() as any).appearance,
        maxColumns: 3,
        maxRows: 2,
        thumbnailMaxPx: 170,
      },
    }),
  );
  await emit(page, "dock:frames", {
    session: 20,
    frames: Object.fromEntries(
      entries.map((item) => [
        String(item.windowId),
        `data:image/svg+xml,${encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" width="300" height="180"><rect width="100%" height="100%" fill="#2563eb"/></svg>`)}`,
      ]),
    ),
  });
  await expect(page.locator(".ot-dock-image img")).toHaveCount(8);
  await page.screenshot({
    path: "../../../.superpowers/sdd/2026-09-06-app-groups-and-dock-previews/qa-dock-horizontal.png",
  });
  await emit(
    page,
    "dock:update",
    state(20, 2, {
      entries,
      appearance: {
        ...(showState() as any).appearance,
        theme: "light",
        layoutDirection: "vertical",
        maxColumns: 3,
        maxRows: 2,
        thumbnailMaxPx: 150,
      },
    }),
  );
  await expect(page.locator(".ot-dock-vertical")).toBeVisible();
  await page.screenshot({
    path: "../../../.superpowers/sdd/2026-09-06-app-groups-and-dock-previews/qa-dock-vertical-light.png",
  });
});

test("compact titles remain readable and revision tombstones prevent resurrection", async ({
  page,
}) => {
  await emit(
    page,
    "dock:show",
    state(30, 3, {
      entries: [entry(1), entry(2), entry(3)],
      appearance: { ...(showState() as any).appearance, compactThreshold: 2 },
    }),
  );
  await expect(page.locator(".ot-dock-style-titles")).toBeVisible();
  await expect(page.getByRole("button", { name: "Focus Document 1" })).toContainText("Document 1");
  await page.screenshot({
    path: "../../../.superpowers/sdd/2026-09-06-app-groups-and-dock-previews/qa-dock-compact.png",
  });
  await emit(page, "dock:error", { session: 30, revision: 5, message: "Current refusal" });
  await emit(
    page,
    "dock:update",
    state(30, 4, { item: { ...state(30, 4).item, title: "Stale update" } }),
  );
  await expect(page.getByRole("alert")).toHaveText("Current refusal");
  await expect(page.getByText("Stale update")).toHaveCount(0);
  await emit(page, "dock:hide", { session: 30, revision: 6 });
  await emit(page, "dock:show", state(30, 7));
  await expect(page.locator(".ot-dock-panel")).toHaveCount(0);
});

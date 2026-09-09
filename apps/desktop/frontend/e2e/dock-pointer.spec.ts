import { expect, test } from "@playwright/test";
import { getCallRecords, installFakeWails, showState } from "./support/fakeWails";

test("native Dock pointer selects and exposes one exact card with scoped monotonic admission", async ({
  page,
}) => {
  await installFakeWails(page);
  await page.goto("/#dock");
  const base = showState() as any;
  const entries = [1, 2].map((windowId) => ({
    ...base.entries[0],
    windowId,
    appId: 10,
    title: `Document ${windowId}`,
  }));
  const state = {
    session: 81,
    revision: 1,
    open: true,
    item: {
      appId: 10,
      bundleId: "qa.app",
      path: "/QA.app",
      title: "QA",
      bounds: { x: 0, y: 0, w: 40, h: 40 },
      screenId: 1,
      edge: "bottom",
      kind: "app",
    },
    entries,
    selectedWindowId: 1,
    appearance: { ...base.appearance, showWindowControls: true },
    emptyReason: "",
  };
  const emit = (name: string, data: unknown) =>
    page.evaluate(
      ([event, payload]) =>
        (window as any)._wails.dispatchWailsEvent({ name: event, data: payload }),
      [name, data] as const,
    );
  await emit("dock:show", state);
  const second = page.locator("article[data-window-id='2']");
  const box = await second.boundingBox();
  expect(box).not.toBeNull();
  await emit("dock:pointer", {
    session: 81,
    sequence: 4,
    x: box!.x + box!.width / 2,
    y: box!.y + box!.height / 2,
    inside: true,
  });
  await expect(second).toHaveClass(/is-hovered/);
  await expect(second.locator(".ot-dock-controls")).toHaveCSS("pointer-events", "auto");

  await emit("dock:pointer", { session: 81, sequence: 3, x: 1, y: 1, inside: true });
  await emit("dock:pointer", { session: 80, sequence: 5, x: 1, y: 1, inside: true });
  await expect(second).toHaveClass(/is-hovered/);
  let calls = await getCallRecords(page);
  expect(calls.filter(([name]) => name === "SelectDockWindow")).toEqual([
    ["SelectDockWindow", 81, 2],
  ]);
  expect(calls.some(([name]) => name === "FocusDockWindow" || name === "PerformDockAction")).toBe(
    false,
  );

  await emit("dock:pointer", { session: 81, sequence: 5, x: -1, y: -1, inside: false });
  await expect(second).not.toHaveClass(/is-hovered/);
  calls = await getCallRecords(page);
  expect(calls.some(([name]) => name === "FocusDockWindow" || name === "PerformDockAction")).toBe(
    false,
  );
});

test("Dock card spacing changes real grid geometry while rows stay bounded", async ({ page }) => {
  await installFakeWails(page);
  await page.goto("/#dock");
  const base = showState() as any;
  const entries = Array.from({ length: 5 }, (_, index) => ({
    ...base.entries[0],
    windowId: index + 1,
    appId: 10,
    title: `Document ${index + 1}`,
  }));
  const state = {
    session: 82,
    revision: 1,
    open: true,
    item: {
      appId: 10,
      bundleId: "qa.app",
      path: "/QA.app",
      title: "QA",
      bounds: { x: 0, y: 0, w: 40, h: 40 },
      screenId: 1,
      edge: "bottom",
      kind: "app",
    },
    entries,
    selectedWindowId: 1,
    appearance: {
      ...base.appearance,
      thumbnailMaxPx: 150,
      maxColumns: 2,
      maxRows: 2,
      autoSize: false,
      showTitle: false,
    },
    cardSpacingPx: 0,
    emptyReason: "",
  };
  const emit = (name: string, data: unknown) =>
    page.evaluate(
      ([event, payload]) =>
        (window as any)._wails.dispatchWailsEvent({ name: event, data: payload }),
      [name, data] as const,
    );
  await emit("dock:show", state);
  const cards = page.locator("article[data-window-id]");
  const gap = async () => {
    const first = await cards.nth(0).boundingBox();
    const second = await cards.nth(1).boundingBox();
    return Math.round(second!.x - (first!.x + first!.width));
  };
  expect(await gap()).toBe(0);
  await expect(page.locator(".ot-dock-list-viewport")).toHaveCSS("max-height", "212px");
  await emit("dock:update", { ...state, revision: 2, cardSpacingPx: 24 });
  expect(await gap()).toBe(24);
  await expect(page.locator(".ot-dock-list-viewport")).toHaveCSS("max-height", "236px");
});

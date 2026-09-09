import { expect, test } from "@playwright/test";
import { getCallRecords, installFakeWails, showState } from "./support/fakeWails";

const previewState = (revision: number, title = "Automation preview") => ({
  open: true,
  session: 55,
  revision,
  title,
  entries: [
    {
      windowId: 102,
      appId: 10,
      appName: "Fixture",
      bundleId: "test.fixture",
      title: "Exact document",
      spaceId: 1,
      minimized: false,
      hidden: false,
      fullscreen: false,
    },
  ],
  selectedWindowId: 102,
  appearance: { ...(showState() as any).appearance, showWindowControls: true },
  cardSpacingPx: 7,
  emptyReason: "",
  error: "",
});

const emit = (page: import("@playwright/test").Page, name: string, data: unknown) =>
  page.evaluate(
    ([eventName, payload]) =>
      (window as any)._wails.dispatchWailsEvent({ name: eventName, data: payload }),
    [name, data] as const,
  );

test("automation preview admits exact scoped frames and actions in native header geometry", async ({
  page,
}) => {
  await installFakeWails(page);
  await page.addInitScript((state) => {
    (window as any).__automationPreviewState = state;
  }, previewState(2));
  await page.goto("/#automation/55");
  await expect(page.getByText("Automation preview")).toBeVisible();

  const panel = page.locator(".ot-dock-panel");
  const header = page.locator(".ot-dock-native-titlebar");
  const close = page.getByRole("button", { name: "Close preview" });
  const [panelBox, headerBox, closeBox] = await Promise.all([
    panel.boundingBox(),
    header.boundingBox(),
    close.boundingBox(),
  ]);
  expect(headerBox?.height).toBe(32);
  expect(closeBox?.width).toBe(48);
  expect(closeBox?.height).toBe(32);
  expect(Math.abs((headerBox?.y ?? 0) - (panelBox?.y ?? 0))).toBeLessThanOrEqual(1);
  expect(
    Math.abs((closeBox?.x ?? 0) + 48 - ((panelBox?.x ?? 0) + (panelBox?.width ?? 0))),
  ).toBeLessThanOrEqual(1);

  await emit(page, "automation-preview:frames", {
    session: 55,
    revision: 2,
    sequence: 1,
    frames: {
      "102": "data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw==",
      "999": "foreign",
    },
  });
  await expect(panel.locator(".ot-dock-image img")).toHaveCount(1);
  await page.getByRole("button", { name: "Focus Exact document" }).click();
  await page.getByLabel("Fullscreen window").click();
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual(["PerformAutomationPreviewAction", 55, 2, "focus", 102, false]);
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual(["PerformAutomationPreviewAction", 55, 2, "fullscreen", 102, true]);
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual([
      "SetAutomationPreviewSize",
      55,
      2,
      Math.ceil(panelBox?.width ?? 0),
      Math.ceil(panelBox?.height ?? 0),
    ]);

  await emit(page, "automation-preview:update", previewState(3, "New revision"));
  await expect(panel.locator(".ot-dock-image img")).toHaveCount(0);
  await emit(page, "automation-preview:frames", {
    session: 55,
    revision: 2,
    sequence: 99,
    frames: { "102": "stale" },
  });
  await expect(panel.locator(".ot-dock-image img")).toHaveCount(0);
  await emit(page, "automation-preview:hide", { session: 55, revision: 4 });
  await emit(page, "automation-preview:update", previewState(5, "Late resurrection"));
  await expect(page.getByText("Late resurrection")).toHaveCount(0);
});

test("automation preview renders an exact-revision RPC refusal", async ({ page }) => {
  await installFakeWails(page);
  await page.addInitScript((state) => {
    (window as any).__automationPreviewState = state;
    (window as any).__automationPreviewError = "Exact automation action refused";
  }, previewState(7));
  await page.goto("/#automation/55");
  await page.locator("article[data-window-id='102']").hover();
  await page.getByLabel("Close window").click();
  await expect(page.getByRole("alert")).toHaveText("Exact automation action refused");
});

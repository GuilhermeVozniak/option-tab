import { expect, test } from "@playwright/test";
import { emitShow, getCallRecords, installFakeWails, showState } from "./support/fakeWails";

const image = (label: string, color: string) =>
  `data:image/svg+xml,${encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" width="640" height="400"><rect width="100%" height="100%" fill="${color}"/><text x="32" y="64" fill="white" font-size="30">${label}</text></svg>`)}`;

const app = (id: number, name: string, count = 1, presence = "present") => ({
  appId: id,
  appName: name,
  bundleId: `qa.${id}`,
  hidden: false,
  windowCount: count,
  windowPresence: presence,
});
const entry = (id: number, appId: number, title: string) => ({
  windowId: id,
  appId,
  appName: `App ${appId}`,
  bundleId: `qa.${appId}`,
  title,
  minimized: false,
  hidden: false,
  fullscreen: false,
});

test.beforeEach(async ({ page }) => {
  await installFakeWails(page);
  await page.goto("/");
});

test("horizontal app mode receives streamed gallery and selected preview frames", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1080, height: 720 });
  await emitShow(
    page,
    showState({
      mode: "apps",
      apps: [app(1, "Writer"), app(2, "Browser"), app(3, "Terminal")],
      selectedWindowId: 11,
      appearance: { ...(showState() as any).appearance, previewSelected: true, previewFade: true },
      entries: [entry(11, 1, "Draft"), entry(12, 1, "Research")],
    }),
  );
  await page.evaluate(
    ([thumb, preview]) => {
      const w = window as any;
      w._wails.dispatchWailsEvent({
        name: "switcher:thumbnails",
        data: { "11": thumb, "12": thumb },
      });
      w._wails.dispatchWailsEvent({ name: "switcher:preview", data: { "11": preview } });
    },
    [image("thumbnail", "#2563eb"), image("selected preview", "#7c3aed")],
  );
  await expect(page.locator(".ot-app-gallery img")).toHaveCount(2);
  await expect(page.getByLabel("Selected window preview").locator("img")).toHaveAttribute(
    "src",
    /selected%20preview/,
  );
  await page.screenshot({
    path: "../../../.superpowers/sdd/2026-09-06-app-groups-and-dock-previews/qa-app-horizontal.png",
  });
});

test("vertical light mode stays contained with many apps and windows", async ({ page }) => {
  await page.setViewportSize({ width: 720, height: 600 });
  const apps = Array.from({ length: 10 }, (_, i) => app(i + 1, `Application ${i + 1}`));
  const entries = Array.from({ length: 8 }, (_, i) =>
    entry(100 + i, 1, `Document ${i + 1} with a readable title`),
  );
  await emitShow(
    page,
    showState({
      mode: "apps",
      apps,
      entries,
      selectedWindowId: 100,
      appearance: {
        ...(showState() as any).appearance,
        theme: "light",
        layoutDirection: "vertical",
        maxRows: 2,
        maxColumns: 3,
        thumbnailMaxPx: 180,
        compactThreshold: 99,
      },
    }),
  );
  const panel = page.locator(".ot-app-panel");
  await expect(panel).toBeVisible();
  expect((await panel.boundingBox())?.width).toBeLessThanOrEqual(720);
  await page.screenshot({
    path: "../../../.superpowers/sdd/2026-09-06-app-groups-and-dock-previews/qa-app-vertical-light.png",
  });
});

test("single-window and windowless states remain clear", async ({ page }) => {
  await page.setViewportSize({ width: 760, height: 500 });
  await emitShow(
    page,
    showState({
      mode: "apps",
      apps: [app(1, "Writer")],
      entries: [entry(11, 1, "Only document")],
      selectedWindowId: 11,
    }),
  );
  await expect(page.getByRole("button", { name: "Focus Only document" })).toBeVisible();
  await page.screenshot({
    path: "../../../.superpowers/sdd/2026-09-06-app-groups-and-dock-previews/qa-app-single.png",
  });
  await page.evaluate(
    (state) => {
      const w = window as any;
      w.__state = state;
      w._wails.dispatchWailsEvent({ name: "switcher:update", data: state });
    },
    showState({
      mode: "apps",
      apps: [app(5, "Windowless Utility", 0, "none")],
      entries: [],
      selected: 0,
      selectedWindowId: 0,
    }),
  );
  await expect(page.getByText("No open windows")).toBeVisible();
  await expect(page.getByRole("button", { name: "Open Windowless Utility" })).toBeVisible();
  await page.screenshot({
    path: "../../../.superpowers/sdd/2026-09-06-app-groups-and-dock-previews/qa-app-windowless.png",
  });
});

test("middle click is action-only and streamed frames respect each visual style", async ({
  page,
}) => {
  const base = showState() as any;
  const current = showState({
    mode: "apps",
    apps: [app(1, "Writer")],
    entries: [{ ...entry(11, 1, "Draft"), icon: image("icon", "#2563eb") }],
    selectedWindowId: 11,
    middleClickAction: "close",
    appearance: { ...base.appearance, style: "thumbnails", compactThreshold: 99 },
  });
  await emitShow(page, current);
  await page.evaluate(
    (thumbnail) => {
      (window as any)._wails.dispatchWailsEvent({
        name: "switcher:thumbnails",
        data: { "11": thumbnail },
      });
    },
    image("frame", "#7c3aed"),
  );
  await expect(page.locator(".ot-app-card-media > img")).toHaveAttribute("src", /frame/);
  await page.getByRole("button", { name: "Focus Draft" }).click({ button: "middle" });
  const calls = await getCallRecords(page);
  expect(calls.filter(([name]) => name === "PerformAction")).toHaveLength(1);
  expect(calls.some(([name]) => name === "ConfirmWindow")).toBe(false);

  for (const style of ["appIcons", "titles"] as const) {
    await page.evaluate(
      ({ state, nextStyle }) => {
        (window as any)._wails.dispatchWailsEvent({
          name: "switcher:update",
          data: {
            ...state,
            appearance: { ...(state as any).appearance, style: nextStyle },
          },
        });
      },
      { state: current, nextStyle: style },
    );
    await expect(page.locator(`.ot-app-switcher.style-${style}`)).toBeVisible();
    await expect(page.locator('img[src*="frame"]')).toHaveCount(0);
  }
  await expect(page.getByText("Draft")).toBeVisible();
});

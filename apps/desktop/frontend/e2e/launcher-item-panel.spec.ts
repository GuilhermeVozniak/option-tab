import { expect, test } from "@playwright/test";
import { getCallRecords, installFakeWails } from "./support/fakeWails";

const child = (revision: number, view: "list" | "grid") => ({
  session: 81,
  revision,
  parentEpoch: 7,
  parentSession: 12,
  displayUUID: "main",
  profileID: "default",
  itemID: "documents",
  kind: "folder",
  title: "Documents and archived project material",
  open: true,
  bounds: { x: 0, y: 0, w: 240, h: 300 },
  folder: {
    folderIdentity: "launcher-child:81",
    status: "ready",
    reason: "",
    view,
    entries: [
      {
        id: "opaque-entry",
        name: "A very long quarterly planning document name that remains readable.txt",
        kind: "file",
        size: 2048,
        modifiedAtMs: 1,
        hidden: false,
      },
    ],
    sort: { field: "name", direction: "asc", foldersFirst: true },
    partial: false,
    revision,
  },
  error: "",
});

test("launcher folder child switches list/grid without cross-axis document overflow", async ({
  page,
}) => {
  await installFakeWails(page);
  await page.addInitScript(
    (state) => ((window as any).__launcherItemPanelState = state),
    child(1, "list"),
  );
  await page.goto("/#/launcher-item/81");
  await page.waitForFunction(
    () => typeof (window as any)._wails?.dispatchWailsEvent === "function",
  );
  await page.evaluate(
    (state) => {
      (window as any)._wails.dispatchWailsEvent({ name: "launcher-item:update", data: state });
    },
    child(1, "list"),
  );
  await expect(page.getByRole("button", { name: /Open A very long quarterly/ })).toBeVisible();
  await page.evaluate(
    (state) => {
      (window as any)._wails.dispatchWailsEvent({ name: "launcher-item:update", data: state });
    },
    child(2, "grid"),
  );
  await expect(page.locator(".ot-folder-list")).toHaveClass(/is-grid/);
  const geometry = await page.evaluate(() => ({
    documentWidth: document.documentElement.scrollWidth,
    viewportWidth: document.documentElement.clientWidth,
    panelWidth: document.querySelector(".ot-launcher-child")?.scrollWidth ?? 0,
  }));
  expect(geometry.documentWidth).toBeLessThanOrEqual(geometry.viewportWidth);
  expect(geometry.panelWidth).toBeLessThanOrEqual(geometry.viewportWidth);
  await page.getByRole("button", { name: /Open A very long quarterly/ }).click();
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual(["OpenLauncherFolderEntry", 81, 2, "opaque-entry"]);
});

test("launcher window child admits current frames and dispatches exact actions", async ({
  page,
}) => {
  const windows = {
    ...child(1, "list"),
    kind: "windows",
    title: "Editor windows",
    folder: undefined,
    windows: {
      open: true,
      session: 81,
      revision: 1,
      title: "Editor windows",
      entries: [
        {
          windowId: 44,
          appId: 9,
          appName: "Editor",
          bundleId: "com.example.editor",
          title: "Plan",
          spaceId: 1,
          minimized: false,
          hidden: false,
          fullscreen: false,
        },
      ],
      selectedWindowId: 44,
      appearance: {
        style: "thumbnails",
        theme: "dark",
        sizePreset: "medium",
        maxRows: 4,
        maxColumns: 6,
        thumbnailMaxPx: 280,
        iconSizePx: 32,
        titleMaxWidthPx: 240,
        fontSizePx: 13,
        accentColor: "#3b82f6",
        backgroundOpacity: 0.85,
        blur: false,
        cornerRadiusPx: 12,
        showAppBadge: true,
        showTitle: true,
        showWindowControls: true,
        autoSize: true,
        apparitionDelayMs: 0,
        fadeOutAnimation: false,
        showStatusIcons: true,
        showSpaceNumbers: true,
        titleTruncation: "end",
        previewSelected: false,
        previewFade: false,
      },
      cardSpacingPx: 7,
      emptyReason: "",
      frames: {},
      frameSequence: 0,
    },
  };
  await installFakeWails(page);
  await page.addInitScript((state) => ((window as any).__launcherItemPanelState = state), windows);
  await page.goto("/#/launcher-item/81");
  await expect(page.getByText("Plan")).toBeVisible();
  await page.evaluate(() => {
    (window as any)._wails.dispatchWailsEvent({
      name: "launcher-item:frames",
      data: {
        session: 81,
        revision: 1,
        sequence: 2,
        frames: { "44": "data:image/png;base64,NEW" },
      },
    });
  });
  await expect(page.locator(".ot-dock-image img")).toHaveAttribute("src", /NEW$/);
  await page.getByRole("button", { name: "Hide app" }).click();
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual(["PerformLauncherWindowAction", 81, 1, "hide", 44, false]);
  await page.getByRole("button", { name: "Focus Plan" }).click();
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual(["PerformLauncherWindowAction", 81, 1, "focus", 44, false]);
});

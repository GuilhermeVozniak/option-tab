import { expect, test } from "@playwright/test";
import { getCallRecords, installFakeWails, showState } from "./support/fakeWails";

const folderState = (revision: number, status = "permissionRequired") => {
  const base = showState() as any;
  return {
    open: true,
    session: 41,
    revision,
    contentKind: "folder",
    item: {
      appId: 0,
      bundleId: "",
      path: "/tmp/Folder Pop Fixture",
      title: "Folder Pop Fixture",
      bounds: { x: 0, y: 0, w: 48, h: 48 },
      screenId: 1,
      edge: "bottom",
      kind: "folder",
    },
    entries: [],
    selectedWindowId: 0,
    appearance: base.appearance,
    emptyReason: "",
    folder: {
      status,
      reason: "",
      folderIdentity: "file:///tmp/Folder%20Pop%20Fixture",
      entries:
        status === "ready"
          ? [
              {
                id: "opaque-file",
                name: "Notes.txt",
                kind: "file",
                size: 14,
                modifiedAtMs: 1,
                hidden: false,
              },
              {
                id: "opaque-folder",
                name: "Projects",
                kind: "folder",
                size: 0,
                modifiedAtMs: 2,
                hidden: false,
              },
            ]
          : [],
      sort: { field: "name", direction: "asc", foldersFirst: true },
      partial: false,
      revision,
    },
  };
};

const emitDock = (page: import("@playwright/test").Page, name: string, data: unknown) =>
  page.evaluate(
    ([eventName, payload]) =>
      (window as any)._wails.dispatchWailsEvent({ name: eventName, data: payload }),
    [name, data] as const,
  );

test("Folder Pop prompts only on explicit action and keeps exact scoped item operations", async ({
  page,
}) => {
  await installFakeWails(page);
  await page.goto("/#dock");
  await page.waitForFunction(
    () => typeof (window as any)._wails?.dispatchWailsEvent === "function",
  );
  await emitDock(page, "dock:show", folderState(1));
  await expect(page.getByText("Folder access is required")).toBeVisible();
  await expect
    .poll(() => getCallRecords(page))
    .not.toContainEqual(["RequestDockFolderAccess", 41, 1]);
  await page.getByRole("button", { name: "Allow access" }).click();
  await expect.poll(() => getCallRecords(page)).toContainEqual(["RequestDockFolderAccess", 41, 1]);

  await emitDock(page, "dock:update", folderState(2, "ready"));
  await expect(page.getByRole("button", { name: "Open Notes.txt" })).toBeVisible();
  await page.getByLabel("Sort folder contents by").selectOption("size");
  await page.getByRole("button", { name: "Open Projects" }).click();
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual(["SetDockFolderSort", 41, 2, "size", "asc", true]);
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual(["OpenDockFolderEntry", 41, 2, "opaque-folder"]);
  expect(await getCallRecords(page)).not.toContainEqual(
    expect.arrayContaining(["SetDockPreviewRegions", 41]),
  );

  await emitDock(page, "dock:update", {
    ...folderState(1, "missing"),
    item: { ...folderState(1).item, title: "Stale folder" },
  });
  await expect(page.getByText("Folder Pop Fixture")).toBeVisible();
  await expect(page.getByText("Stale folder")).toHaveCount(0);

  await emitDock(page, "dock:update", folderState(3, "missing"));
  await expect(page.getByText("Folder is no longer available")).toBeVisible();
  await emitDock(page, "dock:update", folderState(4, "revoked"));
  await expect(page.getByText("Folder access expired")).toBeVisible();
});

test("Folder Pop setting remains independent from Dock window previews", async ({ page }) => {
  await installFakeWails(page);
  await page.goto("/#settings");
  await page.getByRole("tab", { name: "Dock" }).click();
  const previews = page.getByLabel("Enable Dock previews");
  const folders = page.getByLabel("Enable Folder Pop");
  await expect(previews).not.toBeChecked();
  await folders.check();
  await expect(folders).toBeChecked();
  await expect(previews).not.toBeChecked();
});

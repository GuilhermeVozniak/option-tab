import { readFile } from "node:fs/promises";
import { expect, test } from "@playwright/test";
import { defaultSettings } from "../src/lib/types";
import { getCallRecords, installFakeWails } from "./support/fakeWails";

test("downloads a sanitized profile and explicitly imports the reviewed bytes", async ({
  page,
}) => {
  await installFakeWails(page);
  const document = JSON.stringify({
    format: "option-tab.launcher-profile",
    version: 1,
    profile: {
      name: "Travel",
      items: [{ kind: "folder", referenceID: "selection-safe", iconID: "" }],
      widgets: [{ packageID: "org.optiontab.clock", enabled: false, grants: [] }],
    },
  });
  const imported = {
    ...defaultSettings,
    replacementDock: {
      ...defaultSettings.replacementDock,
      profiles: [
        ...defaultSettings.replacementDock.profiles,
        {
          ...defaultSettings.replacementDock.profiles[0],
          id: "profile-imported",
          name: "Travel",
          widgets: defaultSettings.replacementDock.profiles[0].widgets.map((widget) => ({
            ...widget,
            enabled: false,
            grants: [],
          })),
        },
      ],
    },
  };
  await page.addInitScript(
    ({ exportDocument, settingsJSON }) => {
      const w = window as any;
      w.__launcherProfileExport = exportDocument;
      w.__launcherProfileImportReview = {
        digest: "digest-e2e",
        revision: "dock-e2e-1",
        name: "Travel",
        itemCount: 1,
        widgetCount: 1,
        notices: ["selectionsRequireRepair", "widgetsDisabled", "iconsNotIncluded"],
      };
      w.__launcherProfileImportResult = { profileID: "profile-imported", settingsJSON };
    },
    { exportDocument: document, settingsJSON: JSON.stringify(imported) },
  );
  await page.goto("/#/settings");
  await page.getByRole("tab", { name: "Dock" }).click();

  const downloadPromise = page.waitForEvent("download");
  await page.getByRole("button", { name: "Export profile" }).click();
  const download = await downloadPromise;
  expect(download.suggestedFilename()).toBe("option-tab-launcher-profile.json");
  expect(JSON.parse(await readFile((await download.path())!, "utf8"))).toEqual(
    JSON.parse(document),
  );

  await page.getByLabel("Import profile file").setInputFiles({
    name: "travel.json",
    mimeType: "application/json",
    buffer: Buffer.from(document),
  });
  await expect(page.getByRole("region", { name: "Import review" })).toContainText("Travel");
  await expect(
    page.getByText("Files, folders and apps must be selected again on this Mac."),
  ).toBeVisible();
  expect(
    (await getCallRecords(page)).filter(([name]) => name === "ImportLauncherProfile"),
  ).toHaveLength(0);
  await page.getByRole("button", { name: "Import reviewed profile" }).click();
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual(["ImportLauncherProfile", document, "digest-e2e", "dock-e2e-1"]);
  await expect(page.getByLabel("Profile", { exact: true })).toHaveValue("profile-imported");
  await expect(page.getByRole("checkbox", { name: "Enable replacement Dock" })).not.toBeChecked();
});

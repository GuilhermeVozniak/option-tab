import { expect, test } from "@playwright/test";
import { getCallRecords, installFakeWails } from "./support/fakeWails";

const diagnosticsCalls = (calls: unknown[][]) =>
  calls.filter(([name]) => String(name).includes("Diagnostics"));

test("diagnostics stays inert until review and saves the exact reviewed token", async ({
  page,
}) => {
  await installFakeWails(page);
  await page.goto("/#settings");
  await page.getByRole("tab", { name: "About" }).click();
  await page.locator("summary", { hasText: "Diagnostics" }).click();
  expect(diagnosticsCalls(await getCallRecords(page))).toEqual([]);

  await page.getByRole("button", { name: "Review diagnostics" }).click();
  await expect(page.getByLabel("Diagnostics report preview")).toHaveText(
    '{"schemaVersion":1,"recording":false}',
  );
  await page.evaluate(() => {
    (window as any).__diagnosticsReview = {
      token: "diagnostics-review-2",
      json: '{"schemaVersion":2}',
      expiresAt: "2026-09-07T12:05:00Z",
      recording: false,
      dropped: 4,
    };
    (window as any).__diagnosticsSaveResult = { status: "cancelled" };
  });
  await expect(page.getByLabel("Diagnostics report preview")).toHaveText(
    '{"schemaVersion":1,"recording":false}',
  );
  await page.getByRole("button", { name: "Save report…" }).click();
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual(["SaveDiagnosticsReport", "diagnostics-review-1"]);
  await expect(page.getByRole("status")).toHaveCount(0);
  expect(diagnosticsCalls(await getCallRecords(page))).not.toContainEqual([
    "StartDiagnosticsRecording",
  ]);
});

test("diagnostics refreshes after explicit recording changes and maps stable save errors", async ({
  page,
}) => {
  await installFakeWails(page);
  await page.goto("/#settings");
  await page.getByRole("tab", { name: "About" }).click();
  await page.locator("summary", { hasText: "Diagnostics" }).click();
  await page.getByRole("button", { name: "Review diagnostics" }).click();
  await expect(page.getByRole("button", { name: "Start recording" })).toBeVisible();
  await page.evaluate(() => {
    (window as any).__diagnosticsReview = {
      token: "recording-review",
      json: '{"recording":true}',
      expiresAt: "2026-09-07T12:05:00Z",
      recording: true,
      dropped: 1,
    };
  });
  await page.getByRole("button", { name: "Start recording" }).click();
  await expect(page.getByRole("button", { name: "Stop recording" })).toBeVisible();
  await expect
    .poll(async () => diagnosticsCalls(await getCallRecords(page)))
    .toEqual([["GetDiagnosticsReview"], ["StartDiagnosticsRecording"], ["GetDiagnosticsReview"]]);
  await page.evaluate(() => {
    (window as any).__diagnosticsError = { SaveDiagnosticsReport: "destinationExists" };
  });
  await page.getByRole("button", { name: "Save report…" }).click();
  await expect(page.getByRole("alert")).toContainText(
    "That filename already exists. Choose a new name.",
  );
});

import { expect, test } from "@playwright/test";
import { getCallRecords, installFakeWails, showState } from "./support/fakeWails";

const appearance = (showState() as any).appearance;
const item = {
  appId: 88,
  bundleId: "com.apple.Music",
  path: "/System/Applications/Music.app",
  title: "Music",
  bounds: { x: 0, y: 0, w: 48, h: 48 },
  screenId: 1,
  edge: "bottom",
  kind: "app",
};
const entry = {
  windowId: 501,
  appId: 88,
  appName: "Music",
  bundleId: "com.apple.Music",
  title: "Library",
  minimized: false,
  hidden: false,
  fullscreen: false,
};
const windows = (session: number, revision: number) => ({
  open: true,
  session,
  revision,
  contentKind: "windows",
  contentOptions: ["windows", "media"],
  item,
  entries: [entry],
  selectedWindowId: 501,
  appearance,
  cardSpacingPx: 7,
  emptyReason: "",
});
const media = {
  session: 71,
  revision: 3,
  interactionEpoch: 1,
  open: true,
  pinned: false,
  pinnable: true,
  provider: "music",
  scope: {
    provider: "music",
    process: { pid: 88, launchID: "fixture" },
    generation: 1,
    trackEpoch: 1,
    trackID: "track",
  },
  sample: {
    provider: "music",
    process: { pid: 88, launchID: "fixture" },
    generation: 1,
    sequence: 1,
    trackEpoch: 1,
    track: { id: "track", title: "Current track", artist: "Fixture", album: "", durationMS: 1000 },
    playback: "paused",
    positionMS: 0,
    observedAt: "",
    status: "ready",
    reason: "",
    capabilities: { play: true, pause: true, previous: true, next: true, seek: true },
    artworkToken: "",
  },
  appearance,
  artwork: { status: "missing", reason: "", image: "" },
  lyrics: { documentID: "", status: "missing", reason: "", cues: [], offsetMS: 0 },
  positionMS: 0,
  activeCue: -1,
  error: "",
};
const mediaDock = {
  ...windows(42, 1),
  contentKind: "media",
  entries: [],
  selectedWindowId: 0,
  media,
};

const emit = (page: import("@playwright/test").Page, name: string, data: unknown) =>
  page.evaluate(
    ([eventName, payload]) =>
      (window as any)._wails.dispatchWailsEvent({ name: eventName, data: payload }),
    [name, data] as const,
  );

test("same-hover selector changes only from a fresh backend session and rejects old media events", async ({
  page,
}) => {
  await installFakeWails(page);
  await page.goto("/#dock");
  await page.waitForFunction(
    () => typeof (window as any)._wails?.dispatchWailsEvent === "function",
  );
  await emit(page, "dock:show", windows(41, 8));
  await expect(page.getByRole("button", { name: "Windows" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await page.getByRole("button", { name: "Media" }).click();
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual(["SelectDockContent", 41, 8, "media"]);
  await expect(page.getByText("Library")).toBeVisible();

  await emit(page, "dock:show", mediaDock);
  await expect(page.getByText("Current track")).toBeVisible();
  await emit(page, "media:hide", { session: 71, revision: 4 });
  await expect(page.locator(".ot-dock-panel")).toBeVisible();
  await emit(page, "dock:show", windows(43, 1));
  await emit(page, "media:update", {
    ...media,
    revision: 5,
    sample: { ...media.sample, track: { ...media.sample.track, title: "Late media" } },
  });
  await emit(page, "dock:frames", {
    session: 42,
    frames: { "501": "data:image/png;base64,stale" },
  });
  await expect(page.getByText("Library")).toBeVisible();
  await expect(page.getByText("Late media")).toHaveCount(0);
  await expect(page.locator('img[src="data:image/png;base64,stale"]')).toHaveCount(0);
});

test("same-hover selector shows an exact-revision refusal", async ({ page }) => {
  await installFakeWails(page);
  await page.addInitScript(() => {
    (window as any).__dockContentError = "Content switch was refused";
  });
  await page.goto("/#dock");
  await page.waitForFunction(
    () => typeof (window as any)._wails?.dispatchWailsEvent === "function",
  );
  await emit(page, "dock:show", windows(51, 12));
  await page.getByRole("button", { name: "Media" }).click();
  await expect(page.getByRole("alert")).toHaveText("Content switch was refused");
});

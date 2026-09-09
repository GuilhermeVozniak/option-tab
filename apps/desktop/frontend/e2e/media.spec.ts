import path from "node:path";
import { expect, test } from "@playwright/test";
import { getCallRecords, installFakeWails, showState } from "./support/fakeWails";

function mediaState(overrides: Record<string, unknown> = {}) {
  const appearance = (showState() as any).appearance;
  return {
    session: 71,
    revision: 3,
    interactionEpoch: 1,
    open: true,
    pinned: false,
    pinnable: true,
    provider: "music",
    scope: {
      provider: "music",
      process: { pid: 88, launchID: "launch-a" },
      generation: 2,
      trackEpoch: 4,
      trackID: "track-a",
    },
    sample: {
      provider: "music",
      process: { pid: 88, launchID: "launch-a" },
      generation: 2,
      sequence: 1,
      trackEpoch: 4,
      track: {
        id: "track-a",
        title: "Test Song",
        artist: "Example",
        album: "Album",
        durationMS: 90000,
      },
      playback: "paused",
      positionMS: 1000,
      observedAt: "2026-09-07T00:00:00Z",
      status: "ready",
      reason: "",
      capabilities: { play: true, pause: true, previous: true, next: true, seek: true },
      artworkToken: "",
    },
    appearance,
    artwork: { status: "missing", reason: "", image: "" },
    lyrics: {
      documentID: "doc-a",
      status: "ready",
      reason: "",
      cues: Array.from({ length: 60 }, (_, i) => ({
        atMs: i * 1000,
        text: i === 12 ? "x".repeat(1000) : `Original line ${i}`,
      })),
      offsetMS: 0,
    },
    positionMS: 1000,
    activeCue: 1,
    error: "",
    ...overrides,
  };
}

async function dispatch(page: any, name: string, data: unknown) {
  await page.waitForFunction(
    () => typeof (window as any)._wails?.dispatchWailsEvent === "function",
  );
  await page.evaluate(
    ([n, d]) => (window as any)._wails.dispatchWailsEvent({ name: n, data: d }),
    [name, data],
  );
}

test("hover media uses its own revision for controls and programmatic lyric following", async ({
  page,
}) => {
  await installFakeWails(page);
  await page.goto("/#dock");
  const media = mediaState();
  const dock = {
    open: true,
    session: 41,
    revision: 8,
    contentKind: "media",
    item: {
      appId: 0,
      bundleId: "",
      path: "",
      title: "Music",
      bounds: { x: 0, y: 0, w: 40, h: 40 },
      screenId: 1,
      edge: "bottom",
      kind: "media",
    },
    entries: [],
    selectedWindowId: 0,
    appearance: media.appearance,
    emptyReason: "",
    media,
  };
  await dispatch(page, "dock:show", dock);
  await page.getByRole("button", { name: "Play" }).click();
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual(["PerformMediaAction", 71, 3, "play", 0]);

  const updated = mediaState({ revision: 4, error: "Command refused" });
  await dispatch(page, "media:update", updated);
  await expect(page.getByRole("alert")).toContainText("Command refused");
  await page.getByRole("button", { name: "Play" }).click();
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual(["PerformMediaAction", 71, 4, "play", 0]);

  await dispatch(page, "media:progress", {
    session: 71,
    revision: 4,
    sequence: 1,
    positionMS: 40000,
    activeCue: 40,
  });
  await expect(page.getByText("Original line 40")).toBeVisible();
  await expect(page.getByRole("button", { name: "Follow current lyric" })).toHaveCount(0);
  await dispatch(page, "media:progress", {
    session: 71,
    revision: 4,
    sequence: 2,
    positionMS: 55000,
    activeCue: 55,
  });
  await expect(page.getByText("Original line 55")).toBeVisible();
  await expect(page.locator(".ot-media-lyrics")).toHaveJSProperty(
    "scrollWidth",
    await page.locator(".ot-media-lyrics").evaluate((e) => e.clientWidth),
  );
  await page.screenshot({
    path: path.resolve(
      process.cwd(),
      "../../../.superpowers/sdd/2026-09-06-dock-media/evidence/media-dark-hover.png",
    ),
  });
});

test("pinned route reports the whole host and keeps the native header exclusion", async ({
  page,
}) => {
  await installFakeWails(page);
  await page.addInitScript(
    (state) => {
      (window as any).__mediaState = state;
    },
    mediaState({ session: 88, pinned: true, pinnable: false }),
  );
  await page.goto("/#media/88");
  const panel = page.locator(".ot-dock-panel");
  await expect(panel).toBeVisible();
  const [box, header, close] = await Promise.all([
    panel.boundingBox(),
    page.locator(".ot-media-header").boundingBox(),
    page.getByRole("button", { name: "Close media panel" }).boundingBox(),
  ]);
  expect(box && header && close).toBeTruthy();
  expect(Math.abs(header!.y - box!.y)).toBeLessThanOrEqual(2);
  expect(header!.height).toBe(32);
  expect(Math.abs(close!.x + close!.width - (box!.x + box!.width))).toBeLessThanOrEqual(2);
  expect(close!.width).toBe(48);
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual(["SetMediaPanelSize", 88, 3, Math.ceil(box!.width), Math.ceil(box!.height)]);
  await page.evaluate(() => {
    (window as any).__mediaActionError = "Pinned command refused";
  });
  await page.getByRole("button", { name: "Play" }).click();
  await expect(page.getByRole("alert")).toContainText("Pinned command refused");
});

test("media panels inherit light and dark Dock text colors without narrow clipping", async ({
  page,
}) => {
  await installFakeWails(page);
  await page.setViewportSize({ width: 440, height: 620 });
  await page.addInitScript(
    (state) => {
      (window as any).__mediaState = state;
    },
    mediaState({
      session: 91,
      pinned: true,
      pinnable: false,
      appearance: { ...(showState() as any).appearance, theme: "light" },
    }),
  );
  await page.goto("/#media/91");
  const media = page.locator(".ot-media-panel");
  await expect(media).toHaveCSS("color", "rgb(22, 33, 60)");
  await expect(media).toHaveJSProperty("scrollWidth", await media.evaluate((el) => el.clientWidth));
  const evidence = path.resolve(
    process.cwd(),
    "../../../.superpowers/sdd/2026-09-06-dock-media/evidence",
  );
  await page.screenshot({ path: path.join(evidence, "media-light-pin-narrow.png") });

  const dark = mediaState({
    session: 91,
    revision: 4,
    pinned: true,
    pinnable: false,
    appearance: { ...(showState() as any).appearance, theme: "dark" },
  });
  await dispatch(page, "media:update", dark);
  await expect(media).toHaveCSS("color", "rgb(245, 247, 255)");
  await page.screenshot({ path: path.join(evidence, "media-dark-pin-narrow.png") });
});

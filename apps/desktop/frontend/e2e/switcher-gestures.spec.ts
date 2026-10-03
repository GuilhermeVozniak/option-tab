import { expect, type Page, test } from "@playwright/test";
import { emitShow, getCallRecords, installFakeWails, showState } from "./support/fakeWails";

type Region = {
  windowId: number;
  appId: number;
  bounds: { x: number; y: number; w: number; h: number };
};
const entries = Array.from({ length: 18 }, (_, index) => ({
  windowId: index + 1,
  appId: 31,
  appName: "Editor",
  bundleId: "test.editor",
  title: `Document ${index + 1}`,
  minimized: false,
  hidden: false,
  fullscreen: false,
}));
const apps = [
  {
    appId: 31,
    appName: "Editor",
    bundleId: "test.editor",
    windowCount: entries.length,
    hidden: false,
  },
];
async function latest(page: Page): Promise<unknown[] | undefined> {
  return (await getCallRecords(page))
    .filter((call) => call[0] === "SetSwitcherGestureRegions")
    .at(-1);
}
async function published(page: Page): Promise<Region[]> {
  return ((await latest(page))?.[4] ?? []) as Region[];
}

test.beforeEach(async ({ page }) => {
  await installFakeWails(page);
  await page.setViewportSize({ width: 760, height: 540 });
  await page.goto("/");
});

test("a reopened switcher uses the newly saved language through the native settings binding", async ({
  page,
}) => {
  for (const [session, language, expected] of [
    [7, "en", "This action is not supported for this window."],
    [8, "es", "Esta acción no es compatible con esta ventana."],
  ] as const) {
    await page.evaluate(
      ({ session, language }) => {
        const w = window as any;
        w._wails.dispatchWailsEvent({
          name: "switcher:hide",
          data: { session: session - 1, revision: 2 },
        });
        const settings = JSON.parse(w.__settingsJSON);
        settings.behavior.language = language;
        w.__settingsJSON = JSON.stringify(settings);
      },
      { session, language },
    );
    await emitShow(page, showState({ session, revision: 1, entries: entries.slice(0, 1) }));
    await page.evaluate(
      (session) =>
        (window as any)._wails.dispatchWailsEvent({
          name: "switcher:gestureError",
          data: { session, revision: 1, message: "requested AX action is unsupported" },
        }),
      session,
    );
    await expect(page.getByRole("alert")).toContainText(expected);
    await expect(
      page.getByRole("button", {
        name: language === "es" ? "Descartar error de la acción" : "Dismiss action error",
      }),
    ).toBeVisible();
  }
});

for (const mode of ["windows", "apps"] as const) {
  test(`${mode} does not turn DOM wheel or primary drags into native swipe actions`, async ({
    page,
  }) => {
    await emitShow(
      page,
      showState({
        session: 7,
        revision: 1,
        mode,
        entries: entries.slice(0, 1),
        apps,
        swipeUpAction: "minimize",
        swipeDownAction: "close",
        mouseHover: false,
      }),
    );
    await expect.poll(async () => (await published(page)).length).toBeGreaterThan(0);
    const card = page.locator("[data-switcher-gesture-window]").first();
    await card.dispatchEvent("wheel", { deltaY: -150, deltaMode: 0, bubbles: true });
    await card.dispatchEvent("wheel", { deltaY: 150, deltaMode: 1, bubbles: true });
    await card.dispatchEvent("mousedown", { button: 0, clientX: 200, clientY: 250, bubbles: true });
    await card.dispatchEvent("mousemove", {
      buttons: 1,
      clientX: 200,
      clientY: 100,
      bubbles: true,
    });
    await card.dispatchEvent("mouseup", { button: 0, clientX: 200, clientY: 100, bubbles: true });
    // Flush the ordinary geometry RPC path before checking absent actions.
    await page.setViewportSize({ width: 780, height: 560 });
    await expect
      .poll(
        async () =>
          (await getCallRecords(page)).filter((call) => call[0] === "SetSwitcherGestureRegions")
            .length,
      )
      .toBeGreaterThan(1);
    expect(
      (await getCallRecords(page)).filter((call) =>
        ["PerformAction", "CloseSelected", "MinimizeSelected"].includes(String(call[0])),
      ),
    ).toEqual([]);
  });

  for (const style of ["thumbnails", "appIcons", "titles"] as const) {
    test(`${mode} ${style} owns rendered clipped card areas and retires them on hide`, async ({
      page,
    }) => {
      await emitShow(
        page,
        showState({
          session: 7,
          revision: 1,
          mode,
          style,
          entries,
          apps,
          mouseHover: false,
          appearance: {
            ...(showState().appearance as object),
            style,
            maxRows: 2,
            maxColumns: 2,
            thumbnailMaxPx: 180,
            autoSize: false,
            fadeOutAnimation: true,
          },
        }),
      );
      await expect.poll(async () => (await published(page)).length).toBeGreaterThan(0);
      const before = await published(page);
      const visible = new Set(before.map((region) => region.windowId));
      expect(visible.size).toBeLessThan(entries.length);
      // Independently check the rendered hit target at points inside every
      // native-owned rectangle, including its edges. Controls and other UI
      // must never be mistaken for a card even when the gallery is clipped.
      const violations = await page.evaluate((regions) => {
        const problems: string[] = [];
        for (const { windowId, appId, bounds: b } of regions) {
          for (const x of [b.x + 0.5, b.x + b.w / 2, b.x + b.w - 0.5]) {
            for (const y of [b.y + 0.5, b.y + b.h / 2, b.y + b.h - 0.5]) {
              const target = document.elementFromPoint(x, y);
              const card = target?.closest<HTMLElement>("[data-switcher-gesture-window]");
              // Rounded visual corners belong to the card's bounding box;
              // exact corner samples can land on its parent padding.
              if (!card && (x !== b.x + b.w / 2 || y !== b.y + b.h / 2)) continue;
              if (
                Number(card?.dataset.switcherGestureWindow) !== windowId ||
                Number(card?.dataset.switcherGestureApp) !== appId ||
                target?.closest(
                  "[data-switcher-gesture-exclude], .ot-bulk-actions, .ot-app-rail, .ot-app-toolbar",
                )
              ) {
                problems.push(`${windowId} at ${x},${y}: ${target?.tagName}.${target?.className}`);
              }
            }
          }
        }
        for (const control of document.querySelectorAll<HTMLElement>(
          "[data-switcher-gesture-exclude], .ot-bulk-actions, .ot-app-rail, .ot-app-toolbar",
        )) {
          const c = control.getBoundingClientRect();
          for (const { bounds: b } of regions) {
            if (
              c.width > 0 &&
              c.height > 0 &&
              b.x < c.right &&
              b.x + b.w > c.left &&
              b.y < c.bottom &&
              b.y + b.h > c.top
            ) {
              problems.push(`intersects ${control.className}`);
            }
          }
        }
        return problems;
      }, before);
      expect(violations).toEqual([]);
      const scroll = page.locator(mode === "apps" ? ".ot-app-gallery" : ".ot-panel");
      await scroll.evaluate((element) => {
        element.scrollTop = element.scrollHeight;
      });
      await expect
        .poll(async () => JSON.stringify(await published(page)))
        .not.toBe(JSON.stringify(before));
      expect(
        (await published(page)).every((region) => region.windowId > 0 && region.appId === 31),
      ).toBe(true);
      await page.evaluate(() =>
        (window as any)._wails.dispatchWailsEvent({
          name: "switcher:hide",
          data: { session: 7, revision: 2 },
        }),
      );
      await expect.poll(async () => (await published(page)).length).toBe(0);
      await expect(page.locator(".ot-overlay, .ot-app-switcher")).toHaveCount(0);
    });
  }
}

test("entrance animation and delayed apparition never publish active geometry", async ({
  page,
}) => {
  await page.addStyleTag({ content: ".ot-panel { animation-duration: 10s !important; }" });
  await emitShow(page, showState({ session: 7, revision: 1, entries }));
  await expect.poll(async () => (await latest(page))?.slice(1, 3)).toEqual([7, 1]);
  expect(await published(page)).toEqual([]);
  await page.locator(".ot-panel").evaluate((panel) => {
    for (const animation of panel.getAnimations()) animation.finish();
  });
  await expect.poll(async () => (await published(page)).length).toBeGreaterThan(0);
  await page.evaluate(
    (state) => {
      const w = window as any;
      w._wails.dispatchWailsEvent({ name: "switcher:hide", data: { session: 7, revision: 2 } });
      w.__state = state;
      w._wails.dispatchWailsEvent({ name: "switcher:show", data: state });
    },
    showState({
      session: 8,
      revision: 1,
      entries,
      appearance: { ...(showState().appearance as object), apparitionDelayMs: 600 },
    }),
  );
  await expect.poll(async () => (await latest(page))?.slice(1, 3)).toEqual([8, 1]);
  expect(await published(page)).toEqual([]);
  await expect(page.locator(".ot-panel")).toHaveCount(0);
});

for (const available of [false, true]) {
  test(`native availability ${available} controls saved swipe selectors independently of mode`, async ({
    page,
  }) => {
    await page.addInitScript((value) => {
      (window as any).__switcherGestureCapabilities = { available: value };
    }, available);
    await page.goto(`/?nativeGestures=${available}#settings`);
    await page.getByRole("tab", { name: "Shortcuts", exact: true }).click();
    const up = page.getByLabel("Swipe up action");
    const down = page.getByLabel("Swipe down action");
    if (available) {
      await expect(up).toBeEnabled();
      await expect(down).toBeEnabled();
      for (const action of ["none", "close", "minimize", "fullscreen", "hide", "quit"]) {
        await expect(up.locator(`option[value="${action}"]`)).toHaveCount(1);
      }
    } else {
      await expect(up).toBeDisabled();
      await expect(down).toBeDisabled();
    }
    await page.getByLabel("Switcher settings mode").selectOption("apps");
    await expect(up).toHaveValue("none");
    if (available) await expect(up).toBeEnabled();
    else await expect(up).toBeDisabled();
  });
}

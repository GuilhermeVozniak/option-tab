import { expect, type Locator, type Page, test } from "@playwright/test";
import { defaultSettings, type VisualStyle } from "../src/lib/types";
import { emitShow, getCallRecords, installFakeWails, showState } from "./support/fakeWails";

const styles = ["thumbnails", "appIcons", "titles"] as const;

// Advance only the hover delay; no real multi-second sleeps are needed.
async function pauseHoverClock(page: Page) {
  const time = new Date("2026-10-04T12:00:00Z");
  await page.clock.install({ time });
  await page.clock.pauseAt(new Date(time.getTime() + 1000));
}

async function expectDelayedTooltip(page: Page, action: Locator, label: string) {
  await action.hover();
  await page.clock.runFor(2000);
  await expect(page.getByRole("tooltip")).toHaveCount(0);
  await page.clock.runFor(500);
  const tooltip = page.getByRole("tooltip", { name: label, exact: true });
  await expect(tooltip).toBeVisible();
  await expect(action).not.toHaveAttribute("title");
  await expect(action).toHaveAttribute(
    "aria-describedby",
    (await tooltip.getAttribute("id")) ?? "",
  );
  return tooltip;
}

// The macOS panel does not become key. Exercise the native input event contract
// directly, including the held chord modifier, rather than browser Tab focus.
async function nativeTab(page: Page, modifier: "command" | "option", shift = false) {
  await page.evaluate(
    ({ modifier, shift }) => {
      const w = window as any;
      w._wails.dispatchWailsEvent({
        name: "switcher:key",
        data: {
          session: w.__state.session,
          key: "Tab",
          code: "Tab",
          shift,
          ctrl: false,
          alt: modifier === "option",
          meta: modifier === "command",
        },
      });
    },
    { modifier, shift },
  );
}

async function openWindows(
  page: Page,
  style: VisualStyle,
  shortcutId = 1,
  controls = defaultSettings.appearance.showWindowControls,
) {
  await emitShow(
    page,
    showState({
      session: 60,
      revision: 1,
      mode: "windows",
      shortcutId,
      style,
      selected: 0,
      mouseHover: false,
      appearance: {
        ...defaultSettings.appearance,
        style,
        fadeOutAnimation: false,
        showWindowControls: controls,
      },
    }),
  );
}

test.beforeEach(async ({ page }) => {
  await installFakeWails(page);
  await page.goto("/");
  // Keep the pointer away from every card: hover must not be required to see
  // controls, including on windows that keyboard navigation has not selected.
  await page.mouse.move(0, 0);
});

test("preview tooltips wait 2.5 seconds and cancel when the pointer leaves", async ({ page }) => {
  await openWindows(page, "thumbnails");
  await pauseHoverClock(page);
  const first = page.getByRole("button", { name: "New window — Editor", exact: true });
  const next = page.getByRole("button", { name: "Force quit — Editor", exact: true });
  await first.hover();
  await page.clock.runFor(2000);
  await page.mouse.move(0, 0);
  await page.clock.runFor(1000);
  await expect(page.getByRole("tooltip")).toHaveCount(0);
  await expectDelayedTooltip(page, first, "New window — Editor");
  // Even immediately after another tooltip, each new button gets the full delay.
  await expectDelayedTooltip(page, next, "Force quit — Editor");
  await page.mouse.move(0, 0);
  await page.clock.runFor(300);
  await expect(page.getByRole("tooltip")).toHaveCount(0);
  expect((await getCallRecords(page)).filter(([kind]) => kind === "PerformAction")).toEqual([]);
});

for (const style of styles) {
  test(`${style}: every window exposes its controls before hover or click`, async ({ page }) => {
    await openWindows(page, style);
    const windows = page.getByRole("option");
    await expect(windows).toHaveCount(4);
    for (let index = 0; index < 4; index++) {
      const card = windows.nth(index);
      for (const label of [
        "Close window",
        index === 2 ? "Restore window" : "Minimize window",
        "Fullscreen window",
        "Hide app",
        "Quit app",
      ]) {
        const action = card.getByRole("button", { name: label, exact: true });
        await expect(action).toBeVisible();
        await expect(action).toBeEnabled();
        // Each visual style keeps the controls inside its card and off titles.
        const geometry = await action.evaluate((button) => {
          const control = button.getBoundingClientRect();
          const card = button.closest('[role="option"]');
          const parent = card?.getBoundingClientRect();
          const title = card?.querySelector(".ot-titlebar,.ot-meta")?.getBoundingClientRect();
          return {
            contained:
              parent !== undefined &&
              control.left >= parent.left &&
              control.top >= parent.top &&
              control.right <= parent.right + 1 &&
              control.bottom <= parent.bottom + 1,
            coversTitle:
              title !== undefined &&
              control.left < title.right &&
              control.right > title.left &&
              control.top < title.bottom &&
              control.bottom > title.top,
          };
        });
        expect(geometry.contained, `${label} should fit inside ${style} card ${index}`).toBe(true);
        expect(geometry.coversTitle, `${label} should not cover the window title`).toBe(false);
      }
    }
  });

  for (const [modifier, shortcutId] of [
    ["command", 1],
    ["option", 2],
  ] as const) {
    test(`${style}: repeated ${modifier}-Tab advances, wraps, and reverses without clicking`, async ({
      page,
    }) => {
      await openWindows(page, style, shortcutId);
      const windows = page.getByRole("option");
      for (const index of [1, 2, 3, 0]) {
        await nativeTab(page, modifier);
        await expect(windows.nth(index)).toHaveAttribute("aria-selected", "true");
        await expect(page.locator('[role="option"][aria-selected="true"]')).toHaveCount(1);
      }
      await nativeTab(page, modifier, true);
      await expect(windows.nth(3)).toHaveAttribute("aria-selected", "true");
      await expect(page.getByRole("dialog", { name: "Window switcher" })).toBeVisible();
      expect(
        (await getCallRecords(page)).filter(([kind]) =>
          ["Confirm", "ConfirmWindow", "ConfirmApp", "Cancel"].includes(String(kind)),
        ),
      ).toEqual([]);
    });
  }

  test(`${style}: control clicks act on that window without confirming or dismissing`, async ({
    page,
  }) => {
    await openWindows(page, style);
    await pauseHoverClock(page);
    const second = page.getByRole("option").nth(1);
    const actions = [
      ["Close window", "close", 2, 2],
      ["Minimize window", "minimize", 2, 2],
      ["Fullscreen window", "fullscreen", 2, 2],
      ["Hide app", "hide", 0, 2],
      ["Quit app", "quit", 0, 2],
    ] as const;
    for (const [label, kind, windowId, appId] of actions) {
      const action = second.getByRole("button", { name: label, exact: true });
      await expectDelayedTooltip(page, action, label);
      await action.click();
      await page.clock.runFor(300);
      await expect(page.getByRole("tooltip")).toHaveCount(0);
      await expect
        .poll(() => getCallRecords(page))
        .toContainEqual(["PerformAction", kind, windowId, appId]);
      await expect(page.getByRole("dialog", { name: "Window switcher" })).toBeVisible();
      await expect(page.getByRole("option").first()).toHaveAttribute("aria-selected", "true");
    }
    expect((await getCallRecords(page)).filter(([kind]) => kind === "PerformAction")).toHaveLength(
      actions.length,
    );
    expect(
      (await getCallRecords(page)).filter(([kind]) =>
        ["Confirm", "ConfirmWindow", "ConfirmApp", "Cancel"].includes(String(kind)),
      ),
    ).toEqual([]);
  });

  test(`${style}: icon toolbar keeps named tooltips and follows the selected app`, async ({
    page,
  }) => {
    await openWindows(page, style);
    await nativeTab(page, "command");
    await expect(page.getByRole("option").nth(1)).toHaveAttribute("aria-selected", "true");
    await pauseHoverClock(page);
    for (const [text, kind] of [
      ["New window", "newWindow"],
      ["Force quit", "forceQuit"],
      ["Close all windows", "closeAll"],
      ["Minimize all windows", "minimizeAll"],
    ] as const) {
      const label = `${text} — Browser`;
      const action = page.getByRole("button", { name: label, exact: true });
      await expect(action).toBeVisible();
      await expectDelayedTooltip(page, action, label);
      await expect(action.locator("svg")).toHaveCount(1);
      await expect(action).toHaveText("");
      await action.click();
      await page.clock.runFor(300);
      await expect(page.getByRole("tooltip")).toHaveCount(0);
      await expect.poll(() => getCallRecords(page)).toContainEqual(["PerformAction", kind, 2, 2]);
      await expect(page.getByRole("dialog", { name: "Window switcher" })).toBeVisible();
    }
    expect(
      (await getCallRecords(page)).filter(([kind]) =>
        ["Confirm", "ConfirmWindow", "ConfirmApp", "Cancel"].includes(String(kind)),
      ),
    ).toEqual([]);
  });

  test(`${style}: disabling window controls hides the toolbar and card actions`, async ({
    page,
  }) => {
    await openWindows(page, style, 1, false);
    await expect(page.locator(".ot-controls,.ot-bulk-actions")).toHaveCount(0);
    await page.getByRole("option").nth(1).click();
    await expect.poll(() => getCallRecords(page)).toContainEqual(["ConfirmWindow", 2]);
  });
}

test("compact controls remain clickable without covering titles, status, or app badges", async ({
  page,
}) => {
  const presets = [
    {
      name: "96px thumbnails with hidden titles",
      style: "thumbnails",
      thumbnailMaxPx: 96,
      showTitle: false,
      fontSizePx: 24,
    },
    {
      name: "96px thumbnails with large titles",
      style: "thumbnails",
      thumbnailMaxPx: 96,
      showTitle: true,
      fontSizePx: 24,
    },
    {
      name: "app icons with the minimum title width",
      style: "appIcons",
      titleMaxWidthPx: 60,
    },
    {
      name: "title rows with a large font",
      style: "titles",
      fontSizePx: 24,
    },
  ] as const;
  const base = showState();
  const thumbnail = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="320" height="200"><rect width="320" height="200" fill="#243654"/></svg>')}`;
  const entries = (base.entries as Array<Record<string, unknown>>).map((entry) => ({
    ...entry,
    thumbnail,
  }));
  for (const [index, { name, ...appearance }] of presets.entries()) {
    await test.step(name, async () => {
      await emitShow(
        page,
        showState({
          session: 70 + index,
          revision: 1,
          mode: "windows",
          style: appearance.style,
          entries,
          mouseHover: false,
          appearance: { ...defaultSettings.appearance, ...appearance, fadeOutAnimation: false },
        }),
      );
      // Finish the real entrance animation before measuring compact geometry.
      await page.evaluate(() => {
        for (const animation of document.getAnimations()) animation.finish();
      });
      const controls = page.locator(".ot-controls button");
      await expect(controls).toHaveCount(20);
      const problems = await controls.evaluateAll((buttons) => {
        const failures: string[] = [];
        const intersects = (a: DOMRect, b: DOMRect) =>
          a.left < b.right && a.right > b.left && a.top < b.bottom && a.bottom > b.top;
        for (const button of buttons) {
          const label = button.getAttribute("aria-label");
          const card = button.closest('[role="option"]');
          if (!card) continue;
          const rect = button.getBoundingClientRect();
          const bounds =
            button.closest(".ot-thumb")?.getBoundingClientRect() ?? card.getBoundingClientRect();
          if (
            rect.width <= 0 ||
            rect.height <= 0 ||
            rect.left < bounds.left - 1 ||
            rect.right > bounds.right + 1 ||
            rect.top < bounds.top - 1 ||
            rect.bottom > bounds.bottom + 1
          )
            failures.push(`${label} is clipped or outside its preview`);
          if (
            !button.contains(
              document.elementFromPoint(rect.x + rect.width / 2, rect.y + rect.height / 2),
            )
          ) {
            failures.push(`${label} cannot receive a click at its center`);
          }
          for (const element of card.querySelectorAll(
            ".ot-status-icon,.ot-space-badge,.ot-thumb-icon-badge,.ot-titlebar,.ot-meta",
          )) {
            if (intersects(rect, element.getBoundingClientRect())) {
              failures.push(
                `${label} covers ${element.getAttribute("aria-label") ?? element.className}`,
              );
            }
          }
        }
        return failures;
      });
      expect.soft(problems, name).toEqual([]);
    });
  }
});

test("a full thumbnail grid fits the panel with controls and window metadata", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 760 });
  const base = showState();
  const original = (base.entries as Array<Record<string, unknown>>)[0];
  const entries = Array.from({ length: 24 }, (_, index) => ({
    ...original,
    windowId: index + 100,
    appId: index + 100,
    title: `Window ${index + 1}`,
    minimized: true,
    spaceId: index % 2 === 0 ? 1 : 2,
  }));
  await emitShow(
    page,
    showState({
      session: 90,
      revision: 1,
      mode: "windows",
      entries,
      mouseHover: false,
      appearance: {
        ...defaultSettings.appearance,
        style: "thumbnails",
        maxColumns: 6,
        maxRows: 4,
        thumbnailMaxPx: 280,
        autoSize: true,
        showTitle: true,
        previewSelected: false,
        fadeOutAnimation: false,
      },
    }),
  );
  await expect(page.getByRole("option")).toHaveCount(24);
  await page.evaluate(() => {
    for (const animation of document.getAnimations()) animation.finish();
  });
  const frame = await page.locator(".ot-thumb").first().boundingBox();
  // This viewport has enough room to fit without reaching the 96px floor.
  expect(frame?.width).toBeGreaterThan(96);
  const panel = await page.locator(".ot-panel").evaluate((element) => ({
    scrollHeight: element.scrollHeight,
    clientHeight: element.clientHeight,
    scrollWidth: element.scrollWidth,
    clientWidth: element.clientWidth,
  }));
  expect(panel.scrollHeight).toBeLessThanOrEqual(panel.clientHeight);
  expect(panel.scrollWidth).toBeLessThanOrEqual(panel.clientWidth);
});

for (const mode of ["windows", "apps"] as const) {
  test(`${mode}: restore tooltip follows minimized state and inherits the light theme`, async ({
    page,
  }) => {
    await page.setViewportSize({ width: 760, height: 540 });
    const base = showState();
    await emitShow(
      page,
      showState({
        session: 100,
        revision: 1,
        mode,
        apps: [
          { appId: 3, appName: "Terminal", bundleId: "com.ex.term", hidden: false, windowCount: 1 },
        ],
        entries: (base.entries as Array<{ windowId: number }>).filter(
          (entry) => entry.windowId === 3,
        ),
        selectedWindowId: 3,
        mouseHover: false,
        appearance: { ...(base.appearance as object), theme: "light" },
      }),
    );
    await expect(page.getByRole("dialog")).toBeVisible();
    await pauseHoverClock(page);
    const action = page.getByRole("button", { name: "Restore window", exact: true });
    const tooltip = await expectDelayedTooltip(page, action, "Restore window");
    const geometry = await tooltip.evaluate((element) => {
      const bounds = element.getBoundingClientRect();
      const dialog = document.querySelector('[role="dialog"]')!;
      return {
        foreground: getComputedStyle(element).getPropertyValue("--foreground").trim(),
        expectedForeground: getComputedStyle(dialog).getPropertyValue("--foreground").trim(),
        inViewport:
          bounds.left >= 0 &&
          bounds.top >= 0 &&
          bounds.right <= innerWidth &&
          bounds.bottom <= innerHeight,
      };
    });
    expect(geometry.foreground).toBe(geometry.expectedForeground);
    expect(geometry.inViewport).toBe(true);
    await action.click();
    await page.clock.runFor(300);
    await expect(page.getByRole("tooltip")).toHaveCount(0);
    await expect
      .poll(() => getCallRecords(page))
      .toContainEqual(["PerformAction", "minimize", 3, 3]);
    expect((await getCallRecords(page)).filter(([kind]) => kind === "PerformAction")).toHaveLength(
      1,
    );
    expect(
      (await getCallRecords(page)).filter(([kind]) =>
        ["Confirm", "ConfirmWindow", "ConfirmApp", "Cancel"].includes(String(kind)),
      ),
    ).toEqual([]);
  });
}

test("app switcher toolbar and window icons explain actions without changing their click targets", async ({
  page,
}) => {
  const base = showState();
  await emitShow(
    page,
    showState({
      session: 101,
      revision: 1,
      mode: "apps",
      apps: [
        { appId: 1, appName: "Editor", bundleId: "com.ex.editor", hidden: false, windowCount: 1 },
      ],
      entries: (base.entries as Array<{ windowId: number }>).filter(
        (entry) => entry.windowId === 1,
      ),
      selectedWindowId: 1,
      mouseHover: false,
    }),
  );
  await expect(page.getByRole("dialog", { name: "Application switcher" })).toBeVisible();
  await pauseHoverClock(page);
  const targets = [
    [".ot-app-toolbar", "New window", "newWindow", 0],
    [".ot-app-toolbar", "Hide app", "hide", 0],
    [".ot-app-toolbar", "Quit app", "quit", 0],
    [".ot-app-window-actions", "Close window", "close", 1],
    [".ot-app-window-actions", "Minimize window", "minimize", 1],
    [".ot-app-window-actions", "Fullscreen window", "fullscreen", 1],
  ] as const;
  for (const [scope, label, kind, windowId] of targets) {
    const action = page.locator(scope).getByRole("button", { name: label, exact: true });
    await expectDelayedTooltip(page, action, label);
    await action.click();
    await page.clock.runFor(300);
    await expect(page.getByRole("tooltip")).toHaveCount(0);
    await expect
      .poll(() => getCallRecords(page))
      .toContainEqual(["PerformAction", kind, windowId, 1]);
    await expect(page.getByRole("dialog", { name: "Application switcher" })).toBeVisible();
  }
  expect((await getCallRecords(page)).filter(([kind]) => kind === "PerformAction")).toHaveLength(
    targets.length,
  );
  expect(
    (await getCallRecords(page)).filter(([kind]) =>
      ["Confirm", "ConfirmWindow", "ConfirmApp", "Cancel"].includes(String(kind)),
    ),
  ).toEqual([]);
  const open = page
    .locator(".ot-app-toolbar")
    .getByRole("button", { name: "Open Editor", exact: true });
  await expectDelayedTooltip(page, open, "Open Editor");
  await open.click();
  await expect.poll(() => getCallRecords(page)).toContainEqual(["ConfirmApp", 1]);
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(page.getByRole("tooltip")).toHaveCount(0);
});

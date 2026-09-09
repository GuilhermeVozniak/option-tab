import { expect, test } from "@playwright/test";
import {
  emitShow,
  getCallRecords,
  getCalls,
  installFakeWails,
  showState,
} from "./support/fakeWails";

// ---- Static rendering via the built-in #demo route (no backend needed) ----
test.describe("overlay — visual styles (demo route)", () => {
  for (const [hash, style] of [
    ["#demo", "thumbnails"],
    ["#demo:appIcons", "appIcons"],
    ["#demo:titles", "titles"],
  ] as const) {
    test(`renders the ${style} style with every demo window`, async ({ page }) => {
      await page.goto(`/${hash}`);
      const overlay = page.locator(".ot-overlay");
      await expect(overlay).toBeVisible();
      await expect(overlay).toHaveAttribute("data-style", style);
      await expect(page.getByRole("option")).toHaveCount(5);
    });
  }

  test("shows the selected-window preview under the grid", async ({ page }) => {
    await page.goto("/#demo");
    const preview = page.locator(".ot-preview-img");
    await expect(preview).toBeVisible();
    // The preview belongs to the selected entry, inside the panel.
    await expect(page.locator(".ot-panel .ot-preview")).toHaveCount(1);
  });

  test("marks the selected entry and renders status icons", async ({ page }) => {
    await page.goto("/#demo");
    await expect(page.locator(".ot-entry.ot-selected")).toHaveCount(1);
    // demo includes a minimized window and one on another Space
    await expect(page.locator(".ot-status").first()).toBeVisible();
  });

  test("reveals close/minimize/fullscreen/hide/quit controls on hover", async ({ page }) => {
    await page.goto("/#demo:titles");
    await page.getByRole("option").first().hover();
    await expect(page.getByLabel("Close window").first()).toBeVisible();
    await expect(page.getByLabel("Minimize window").first()).toBeVisible();
    await expect(page.getByLabel("Fullscreen window").first()).toBeVisible();
    await expect(page.getByLabel("Hide app").first()).toBeVisible();
    await expect(page.getByLabel("Quit app").first()).toBeVisible();
  });
});

// ---- Interactive behavior via an injected fake Wails runtime ----
test.describe("overlay — interactive", () => {
  test.beforeEach(async ({ page }) => {
    await installFakeWails(page);
    await page.goto("/");
  });

  test("Tab advances and Shift+Tab reverses the selection", async ({ page }) => {
    await emitShow(page, showState({ selected: 0 }));
    const options = page.getByRole("option");
    await expect(options.nth(0)).toHaveAttribute("aria-selected", "true");
    await page.keyboard.press("Tab");
    await expect(options.nth(1)).toHaveAttribute("aria-selected", "true");
    await page.keyboard.press("Shift+Tab");
    await expect(options.nth(0)).toHaveAttribute("aria-selected", "true");
  });

  test("app mode keeps duplicate names and confirms the clicked preview exactly", async ({
    page,
  }) => {
    await emitShow(
      page,
      showState({
        mode: "apps",
        apps: [
          {
            appId: 10,
            appName: "Example",
            bundleId: "a",
            hidden: false,
            windowCount: 2,
            windowPresence: "present",
          },
          {
            appId: 20,
            appName: "Example",
            bundleId: "b",
            hidden: false,
            windowCount: 0,
            windowPresence: "none",
          },
        ],
        selected: 0,
        selectedWindowId: 1,
        entries: [
          { ...(showState() as any).entries[0], appId: 10, title: "Document A" },
          { ...(showState() as any).entries[1], appId: 10, windowId: 102, title: "Document B" },
        ],
      }),
    );
    await expect(page.getByRole("option", { name: /Example/ })).toHaveCount(2);
    await page.getByRole("button", { name: "Focus Document B" }).click();
    await expect.poll(() => getCallRecords(page)).toContainEqual(["ConfirmWindow", 102]);
  });

  test("rejects reordered switcher state, frames, and closed-session resurrection", async ({
    page,
  }) => {
    const base = showState() as any;
    const state = (session: number, revision: number, title: string) => ({
      ...base,
      session,
      revision,
      entries: [{ ...base.entries[0], title }],
    });
    await emitShow(page, state(40, 2, "Initial"));
    await page.evaluate(
      ([newest, oldShow, oldSession]) => {
        const w = window as any;
        w._wails.dispatchWailsEvent({ name: "switcher:update", data: newest });
        w._wails.dispatchWailsEvent({ name: "switcher:show", data: oldShow });
        w._wails.dispatchWailsEvent({ name: "switcher:update", data: oldSession });
        w._wails.dispatchWailsEvent({
          name: "switcher:thumbnails",
          data: { session: 39, frames: { "1": "data:old" } },
        });
        w._wails.dispatchWailsEvent({
          name: "switcher:thumbnails",
          data: { session: 40, frames: { "1": "data:current" } },
        });
      },
      [state(40, 4, "Newest"), state(40, 3, "Old show"), state(39, 99, "Old session")],
    );
    await expect(page.getByText("Newest")).toBeVisible();
    await expect(page.locator(".ot-thumb-img")).toHaveAttribute("src", "data:current");
    await page.evaluate(
      (late) => {
        const w = window as any;
        w._wails.dispatchWailsEvent({ name: "switcher:hide", data: { session: 40, revision: 5 } });
        w._wails.dispatchWailsEvent({ name: "switcher:update", data: late });
        w._wails.dispatchWailsEvent({ name: "switcher:show", data: { ...late, revision: 7 } });
      },
      state(40, 6, "Late"),
    );
    await expect(page.locator(".ot-overlay,.ot-app-switcher")).toHaveCount(0);
  });

  test("rejects native keys from an older presentation session", async ({ page }) => {
    await emitShow(page, showState({ session: 50, revision: 1, selected: 0 }));
    const options = page.getByRole("option");
    const emitKey = (session: number) =>
      page.evaluate((keySession) => {
        const w = window as any;
        w._wails.dispatchWailsEvent({
          name: "switcher:key",
          data: {
            session: keySession,
            key: "Tab",
            code: "Tab",
            shift: false,
            ctrl: false,
            alt: false,
            meta: false,
          },
        });
      }, session);
    await emitKey(49);
    await expect(options.nth(0)).toHaveAttribute("aria-selected", "true");
    await emitKey(50);
    await expect(options.nth(1)).toHaveAttribute("aria-selected", "true");
  });

  test("labels additional actions and their target app for people", async ({ page }) => {
    await emitShow(page, showState({ selected: 0 }));
    await expect(page.getByText("New window — Editor", { exact: true })).toBeVisible();
    await expect(page.getByText("Force quit — Editor", { exact: true })).toBeVisible();
    await expect(page.getByText("Close all windows — Editor", { exact: true })).toBeVisible();
    await expect(page.getByText("Minimize all windows — Editor", { exact: true })).toBeVisible();
  });

  test("arrow keys navigate", async ({ page }) => {
    await emitShow(page, showState({ selected: 0 }));
    const options = page.getByRole("option");
    await expect(options.nth(0)).toHaveAttribute("aria-selected", "true");
    await page.keyboard.press("ArrowRight");
    await expect(options.nth(1)).toHaveAttribute("aria-selected", "true");
    await page.keyboard.press("ArrowLeft");
    await expect(options.nth(0)).toHaveAttribute("aria-selected", "true");
  });

  test("compact titles mode uses vertical arrow navigation", async ({ page }) => {
    const base = showState() as any;
    await emitShow(
      page,
      showState({
        appearance: { ...base.appearance, layoutDirection: "horizontal", compactThreshold: 4 },
      }),
    );
    await expect(page.locator('.ot-overlay[data-style="titles"]')).toBeVisible();
    await page.keyboard.press("ArrowDown");
    await expect(page.getByRole("option").nth(1)).toHaveAttribute("aria-selected", "true");
    await page.keyboard.press("ArrowUp");
    await expect(page.getByRole("option").nth(0)).toHaveAttribute("aria-selected", "true");
  });

  test("vim keys navigate only when enabled", async ({ page }) => {
    await emitShow(page, showState({ selected: 0, vimKeys: true }));
    const options = page.getByRole("option");
    await expect(options.nth(0)).toHaveAttribute("aria-selected", "true");
    await page.keyboard.press("j");
    await expect(options.nth(1)).toHaveAttribute("aria-selected", "true");
    await page.keyboard.press("k");
    await expect(options.nth(0)).toHaveAttribute("aria-selected", "true");
  });

  test("type-to-search updates the query line", async ({ page }) => {
    await emitShow(page, showState({}));
    await expect(page.locator(".ot-overlay")).toBeVisible();
    await page.keyboard.press("g");
    await expect(page.locator(".ot-search")).toContainText("g");
  });

  test("Escape cancels and dismisses the switcher", async ({ page }) => {
    await emitShow(page, showState({}));
    await expect(page.locator(".ot-overlay")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page.locator(".ot-overlay")).toHaveCount(0);
    expect(await getCalls(page)).toContain("Cancel");
  });

  test("clicking an unselected entry confirms its window when hover selection is disabled", async ({
    page,
  }) => {
    await emitShow(page, showState({ selected: 0, mouseHover: false }));
    await page.getByRole("option").nth(1).click();
    expect(await getCallRecords(page)).toContainEqual(["ConfirmWindow", 2]);
  });

  test("acts on the clicked window identity when selection differs", async ({ page }) => {
    await emitShow(page, showState({ selected: 0, mouseHover: false }));
    const second = page.getByRole("option").nth(1);
    await second.hover();
    await second.getByLabel("Close window").click();
    await expect.poll(() => getCallRecords(page)).toContainEqual(["PerformAction", "close", 2, 2]);
    await expect(page.getByRole("option").nth(0)).toHaveAttribute("aria-selected", "true");
  });

  test("shows a native action error without dismissing the open overlay", async ({ page }) => {
    await page.evaluate(() => {
      (window as any).__actionError = "Accessibility denied";
    });
    await emitShow(page, showState({}));
    await page.getByRole("option").first().hover();
    await page.getByLabel("Close window").first().click();
    await expect(page.getByRole("alert")).toContainText("Accessibility denied");
    await expect(page.locator(".ot-overlay")).toBeVisible();
  });

  test("shows partial bulk-action failures returned by native code", async ({ page }) => {
    await page.evaluate(() => {
      (window as any).__actionResult = {
        succeeded: 2,
        failures: [{ windowId: 4, error: "Notes refused to close" }],
      };
    });
    await emitShow(page, showState({}));
    await page.getByRole("button", { name: "closeAll" }).click();
    await expect(page.getByRole("alert")).toContainText("Notes refused to close");
    await expect
      .poll(() => getCallRecords(page))
      .toContainEqual(["PerformAction", "closeAll", 1, 1]);
  });

  test("forwards a custom physical action key", async ({ page }) => {
    await emitShow(page, showState({ actionBindings: { KeyX: "minimize" } }));
    await page.keyboard.press("Alt+x");
    await expect
      .poll(() => getCallRecords(page))
      .toContainEqual(["PerformAction", "minimize", 1, 1]);
  });

  test("consumes the click synthesized after a pointer swipe", async ({ page }) => {
    await emitShow(page, showState({ swipeUpAction: "minimize", mouseHover: false }));
    const box = await page.getByRole("option").first().boundingBox();
    if (!box) throw new Error("entry has no bounding box");
    await page.mouse.move(box.x + 40, box.y + Math.min(130, box.height - 10));
    await page.mouse.down();
    await page.mouse.move(box.x + 40, box.y + 10, { steps: 5 });
    await page.mouse.up();
    await expect
      .poll(() => getCallRecords(page))
      .toContainEqual(["PerformAction", "minimize", 1, 1]);
    expect((await getCallRecords(page)).filter((call) => call[0] === "ConfirmWindow")).toHaveLength(
      0,
    );
  });

  test("accumulates small wheel deltas and fires once for uninterrupted momentum", async ({
    page,
  }) => {
    await emitShow(page, showState({ swipeUpAction: "minimize" }));
    const entry = page.getByRole("option").first();
    for (let i = 0; i < 12; i++) await entry.dispatchEvent("wheel", { deltaY: -10 });
    await expect
      .poll(() => getCallRecords(page))
      .toContainEqual(["PerformAction", "minimize", 1, 1]);
    for (let i = 0; i < 8; i++) {
      await entry.dispatchEvent("wheel", { deltaY: -60 });
      await page.waitForTimeout(60);
    }
    expect((await getCallRecords(page)).filter((call) => call[1] === "minimize")).toHaveLength(1);
  });

  test("retains the preview image node across live frames and remounts on selection", async ({
    page,
  }) => {
    const base = showState() as any;
    const entries = base.entries.map((entry: any, index: number) => ({
      ...entry,
      preview: `data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg'><text>${index}</text></svg>`,
    }));
    await emitShow(
      page,
      showState({
        entries,
        appearance: { ...base.appearance, previewSelected: true, previewFade: true },
      }),
    );
    const preview = page.locator(".ot-preview-img");
    await preview.evaluate((img) => {
      img.setAttribute("data-node-marker", "retained");
    });
    await page.evaluate(() => {
      const w = window as any;
      w.__state.entries[0].preview += "#next-frame";
      w.__state = { ...w.__state, entries: [...w.__state.entries] };
      w._wails.dispatchWailsEvent({ name: "switcher:update", data: w.__state });
    });
    await expect(preview).toHaveAttribute("data-node-marker", "retained");
    await page.evaluate(() => {
      const w = window as any;
      w.__state = { ...w.__state, selected: 1 };
      w._wails.dispatchWailsEvent({ name: "switcher:update", data: w.__state });
    });
    await expect(preview).not.toHaveAttribute("data-node-marker", "retained");
  });

  test("hover controls call the matching window/app actions", async ({ page }) => {
    await emitShow(page, showState({}));
    const first = page.getByRole("option").first();
    const actions: [string, string, number, number][] = [
      ["Close window", "close", 1, 1],
      ["Minimize window", "minimize", 1, 1],
      ["Fullscreen window", "fullscreen", 1, 1],
      ["Hide app", "hide", 0, 1],
      ["Quit app", "quit", 0, 1],
    ];
    for (const [label] of actions) {
      await first.hover();
      await page.getByLabel(label).first().click();
    }
    await expect
      .poll(() => getCallRecords(page))
      .toEqual(
        expect.arrayContaining(
          actions.map(([, kind, windowId, appId]) => ["PerformAction", kind, windowId, appId]),
        ),
      );
  });

  test("blur knob toggles the frosted-glass class", async ({ page }) => {
    await emitShow(page, showState({ appearance: { ...showState().appearance, blur: false } }));
    await expect(page.locator(".ot-overlay.ot-no-blur")).toHaveCount(1);
  });
});

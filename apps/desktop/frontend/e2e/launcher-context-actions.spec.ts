import { expect, test } from "@playwright/test";
import type { LauncherPresentation } from "../src/lib/types";
import { getCallRecords, installFakeWails } from "./support/fakeWails";

for (const edge of ["bottom", "top", "left", "right"] as const) {
  test(`${edge} launcher reveals the last application's context actions without manual scrolling`, async ({
    page,
  }) => {
    const vertical = edge === "left" || edge === "right";
    const viewport = vertical ? { width: 65, height: 392 } : { width: 392, height: 65 };
    await page.setViewportSize(viewport);
    await installFakeWails(page);
    const presentation: LauncherPresentation = {
      epoch: 7,
      displayUUID: "display-fixture",
      session: 41,
      revision: 2,
      visible: true,
      reason: "",
      profileID: "default",
      bounds: { x: 0, y: 0, w: viewport.width, h: viewport.height },
      iconPx: 40,
      edge,
      layout: "floating",
      appearance: {
        theme: "dark",
        material: "solid",
        tint: "#172033",
        opacity: 0.76,
        borderOpacity: 0.16,
        cornerRadiusPx: 18,
        itemSpacingPx: 6,
        showLabels: false,
      },
      items: Array.from({ length: 8 }, (_, index) => ({
        id: `item-${index}`,
        name: `Application ${index}`,
        icon: "",
        kind: "app",
        running: true,
      })),
      widgets: [],
    };
    await page.addInitScript((state) => {
      (window as any).__launcherState = state;
      (window as any).__launcherInteractionState = {
        epoch: state.epoch,
        displayUUID: state.displayUUID,
        session: state.session,
        presentationRevision: state.revision,
        admission: 1,
        sequence: 1,
        visible: true,
        selectedItemID: "",
        keyboardMode: false,
        gestureAvailable: true,
        pinchAvailable: false,
        swipeAvailable: false,
        letterInputAvailable: false,
        hapticsAvailable: false,
        configured: {
          enabled: false,
          preciseScroll: false,
          pinch: false,
          swipe: false,
          primaryAction: "next",
          towardAction: "showPreview",
          pinchAction: "showPreview",
          haptics: false,
          letterNavigation: false,
          enterActivates: false,
        },
        reason: "",
      };
    }, presentation);
    await page.goto("/#/launcher/41");
    await page
      .getByRole("button", { name: "Application 7", exact: true })
      .click({ button: "right" });

    const actions = page.getByLabel("Application actions: Application 7", { exact: true });
    await expect(actions).toBeAttached();
    // Inspect real clipping and hit targets before Playwright can scroll an
    // action into view as part of its automatic click preparation.
    const readGeometry = () =>
      actions.evaluate((menu) => {
        const strip = menu.closest(".ot-launcher-strip")!.getBoundingClientRect();
        return [...menu.querySelectorAll("button")].map((button) => {
          const box = button.getBoundingClientRect();
          const x = box.left + box.width / 2;
          const y = box.top + box.height / 2;
          return {
            label: button.textContent,
            inside:
              box.left >= Math.max(0, strip.left) &&
              box.right <= Math.min(innerWidth, strip.right) &&
              box.top >= Math.max(0, strip.top) &&
              box.bottom <= Math.min(innerHeight, strip.bottom),
            hit: button.contains(document.elementFromPoint(x, y)),
            x,
            y,
          };
        });
      });
    const geometry = await readGeometry();
    expect(geometry).toHaveLength(3);
    for (const button of geometry) {
      expect(button, JSON.stringify(button)).toMatchObject({
        inside: true,
        hit: true,
      });
    }
    await page.mouse.click(geometry[0].x, geometry[0].y);
    await expect(actions).toHaveCount(0);
    await expect
      .poll(() => getCallRecords(page))
      .toContainEqual(["ShowLauncherItemPanel", 7, "display-fixture", 41, 2, "item-7"]);
    await page.getByRole("button", { name: "Application 7", exact: true }).press("Shift+F10");
    await expect(actions).toBeAttached();
    expect((await readGeometry()).every((button) => button.inside && button.hit)).toBe(true);
    await page.keyboard.press("Escape");
    await expect(actions).toHaveCount(0);
  });
}

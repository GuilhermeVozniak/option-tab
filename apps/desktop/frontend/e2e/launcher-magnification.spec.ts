import { expect, test } from "@playwright/test";
import { getCallRecords, installFakeWails } from "./support/fakeWails";

for (const edge of ["bottom", "top", "left", "right"]) {
  test(`magnification respects ${edge} envelope and stable group hitboxes`, async ({ page }) => {
    const vertical = edge === "left" || edge === "right";
    await page.setViewportSize(vertical ? { width: 88, height: 500 } : { width: 500, height: 88 });
    await installFakeWails(page);
    const state = {
      epoch: 7,
      displayUUID: "main",
      session: 41,
      revision: 1,
      visible: true,
      reason: "",
      profileID: "default",
      bounds: { x: 0, y: 0, w: vertical ? 88 : 500, h: vertical ? 500 : 88 },
      iconPx: 40,
      edge,
      layout: "floating",
      appearance: {
        theme: "dark",
        material: "solid",
        tint: "#172033",
        opacity: 0.8,
        borderOpacity: 0.2,
        cornerRadiusPx: 12,
        itemSpacingPx: 6,
        showLabels: true,
      },
      magnification: { enabled: true, scale: 2, reach: 1, primaryInset: 72, crossInset: 24 },
      items: [
        { id: "a", name: "Alpha", icon: "", kind: "app" },
        {
          id: "g",
          name: "Group",
          icon: "",
          kind: "group",
          members: [
            { id: "b", name: "Beta", icon: "", kind: "app" },
            { id: "c", name: "Gamma", icon: "", kind: "app" },
          ],
        },
        { id: "d", name: "Delta", icon: "", kind: "app" },
      ],
      widgets: [],
    };
    await page.addInitScript((value) => {
      (window as any).__launcherState = value;
    }, state);
    await page.goto("/#/launcher/41");
    await page.getByRole("button", { name: "Group", exact: true }).click();
    const beta = page.getByRole("button", { name: "Beta", exact: true });
    const before = await beta.boundingBox();
    await beta.hover();
    await expect
      .poll(async () =>
        beta.locator(".ot-launcher-visual").evaluate((n) => getComputedStyle(n).transform),
      )
      .not.toBe("matrix(1, 0, 0, 1, 0, 0)");
    await page.waitForTimeout(650);
    expect(await beta.boundingBox()).toEqual(before);
    const geometry = await page.evaluate(() => {
      const shell = document.querySelector(".ot-launcher-shell")!.getBoundingClientRect();
      return {
        shell: { left: shell.left, top: shell.top, right: shell.right, bottom: shell.bottom },
        visuals: Array.from(document.querySelectorAll(".ot-launcher-visual")).map((n) => {
          const r = n.getBoundingClientRect();
          return { left: r.left, top: r.top, right: r.right, bottom: r.bottom };
        }),
      };
    });
    for (const r of geometry.visuals) {
      expect(r.left).toBeGreaterThanOrEqual(geometry.shell.left);
      expect(r.right).toBeLessThanOrEqual(geometry.shell.right);
      expect(r.top).toBeGreaterThanOrEqual(geometry.shell.top);
      expect(r.bottom).toBeLessThanOrEqual(geometry.shell.bottom);
    }
    expect(
      (await getCallRecords(page)).filter((c) => c[0] === "ActivateLauncherItem"),
    ).toHaveLength(0);
    await page.emulateMedia({ reducedMotion: "reduce" });
    await expect
      .poll(async () =>
        beta.locator(".ot-launcher-visual").evaluate((n) => getComputedStyle(n).transform),
      )
      .toBe("matrix(1, 0, 0, 1, 0, 0)");
  });
}

test("a partly scrolled icon retains baseline clipping under hover", async ({ page }) => {
  await page.setViewportSize({ width: 300, height: 88 });
  await installFakeWails(page);
  const state = {
    epoch: 7,
    displayUUID: "main",
    session: 41,
    revision: 1,
    visible: true,
    reason: "",
    profileID: "default",
    bounds: { x: 0, y: 0, w: 300, h: 88 },
    iconPx: 40,
    edge: "bottom",
    layout: "floating",
    appearance: {
      theme: "dark",
      material: "solid",
      tint: "#172033",
      opacity: 0.8,
      borderOpacity: 0.2,
      cornerRadiusPx: 12,
      itemSpacingPx: 6,
      showLabels: true,
    },
    magnification: { enabled: true, scale: 2, reach: 1, primaryInset: 72, crossInset: 24 },
    items: Array.from({ length: 15 }, (_, i) => ({
      id: `a${i}`,
      name: `App ${i}`,
      icon: "",
      kind: "app",
    })),
    widgets: [],
  };
  await page.addInitScript((value) => {
    (window as any).__launcherState = value;
  }, state);
  await page.goto("/#/launcher/41");
  await expect(page.getByRole("button", { name: "App 0", exact: true })).toBeAttached();
  await page.locator(".ot-launcher-strip").evaluate((n) => {
    n.scrollLeft = 90;
  });
  const first = page.getByRole("button", { name: "App 0", exact: true });
  const box = await first.boundingBox();
  expect(box!.x).toBeLessThan(0);
  expect(box!.x + box!.width).toBeGreaterThan(4);
  await page.mouse.move(8, 44);
  await page.waitForTimeout(650);
  await expect
    .poll(() => first.locator(".ot-launcher-visual").evaluate((n) => getComputedStyle(n).transform))
    .toBe("matrix(1, 0, 0, 1, 0, 0)");
  expect(await first.locator(".ot-launcher-visual").boundingBox()).toEqual(
    await first.boundingBox(),
  );
  expect((await getCallRecords(page)).filter((c) => c[0] === "ActivateLauncherItem")).toHaveLength(
    0,
  );
});

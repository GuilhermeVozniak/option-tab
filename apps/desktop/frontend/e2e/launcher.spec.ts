import { expect, test } from "@playwright/test";
import { getCallRecords, installFakeWails } from "./support/fakeWails";

const presentation = (revision: number, name: string, visible = true) => ({
  epoch: 7,
  displayUUID: "display-main",
  session: 41,
  revision,
  visible,
  reason: "",
  profileID: "default",
  bounds: { x: 20, y: 700, w: 220, h: 64 },
  iconPx: 40,
  edge: "bottom",
  layout: "floating",
  appearance: {
    theme: "dark",
    material: "solid",
    tint: "#172033",
    opacity: 0.76,
    borderOpacity: 0.16,
    cornerRadiusPx: 18,
    itemSpacingPx: 6,
    showLabels: true,
  },
  items: visible ? [{ id: `opaque-${name}`, name, icon: "" }] : [],
  widgets: [],
});

test("keyboard navigation is explicitly admitted and selects without activating", async ({
  page,
}) => {
  await page.setViewportSize({ width: 520, height: 80 });
  await installFakeWails(page);
  const launcher = {
    ...presentation(2, ""),
    items: [
      {
        id: "group",
        name: "Work",
        icon: "",
        kind: "group",
        members: [{ id: "notes", name: "Notes", icon: "", kind: "app" }],
      },
      { id: "mail", name: "Mail", icon: "", kind: "app" },
    ],
  };
  const configured = {
    enabled: true,
    preciseScroll: true,
    pinch: false,
    swipe: false,
    primaryAction: "next",
    towardAction: "showPreview",
    pinchAction: "showPreview",
    haptics: true,
    letterNavigation: true,
    enterActivates: false,
  };
  const interaction = {
    epoch: 7,
    displayUUID: "display-main",
    session: 41,
    presentationRevision: 2,
    admission: 5,
    sequence: 1,
    visible: true,
    selectedItemID: "notes",
    keyboardMode: false,
    gestureAvailable: true,
    pinchAvailable: false,
    swipeAvailable: false,
    letterInputAvailable: true,
    hapticsAvailable: true,
    configured,
    reason: "",
  };
  await page.addInitScript(
    ([state, input]) => {
      (window as any).__launcherState = state;
      (window as any).__launcherInteractionState = input;
    },
    [launcher, interaction],
  );
  await page.goto("/#/launcher/41");
  await expect(page.getByRole("button", { name: "Notes" })).toHaveAttribute("aria-current", "true");
  await page.getByRole("button", { name: "Keyboard navigation" }).click();
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual(["SetLauncherKeyboardMode", 7, "display-main", 41, 2, 5, true]);
  await page.evaluate(
    (state) =>
      (window as any)._wails.dispatchWailsEvent({
        name: "launcher:interaction",
        data: { ...state, admission: 6, sequence: 1, keyboardMode: true },
      }),
    interaction,
  );
  await page.getByRole("textbox", { name: "Type a letter" }).fill("m");
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual(["CommitLauncherLetter", 7, "display-main", 41, 2, 6, 2, "m", 0, false]);
  expect((await getCallRecords(page)).some(([name]) => name === "ActivateLauncherSelection")).toBe(
    false,
  );
});

for (const edge of ["bottom", "top", "left", "right"] as const) {
  test(`one-item ${edge} launcher keeps its item usable beside keyboard mode`, async ({ page }) => {
    const vertical = edge === "left" || edge === "right";
    await page.setViewportSize(vertical ? { width: 32, height: 76 } : { width: 76, height: 32 });
    await installFakeWails(page);
    const state = {
      ...presentation(2, "Mail"),
      edge,
      iconPx: 24,
      bounds: { x: 0, y: 0, w: vertical ? 32 : 76, h: vertical ? 76 : 32 },
    };
    await page.addInitScript((launcher) => {
      (window as any).__launcherState = launcher;
      (window as any).__launcherInteractionState = {
        epoch: launcher.epoch,
        displayUUID: launcher.displayUUID,
        session: launcher.session,
        presentationRevision: launcher.revision,
        admission: 2,
        sequence: 1,
        visible: true,
        selectedItemID: "opaque-Mail",
        keyboardMode: true,
        gestureAvailable: true,
        pinchAvailable: false,
        swipeAvailable: false,
        letterInputAvailable: true,
        hapticsAvailable: false,
        configured: {
          enabled: true,
          preciseScroll: true,
          pinch: false,
          swipe: false,
          primaryAction: "next",
          towardAction: "showPreview",
          pinchAction: "showPreview",
          haptics: false,
          letterNavigation: true,
          enterActivates: false,
        },
        reason: "",
      };
    }, state);
    await page.goto("/#/launcher/41");
    const item = page.getByRole("button", { name: "Mail" });
    await expect(item).toBeVisible();
    await expect(page.getByRole("textbox", { name: "Type a letter" })).toBeVisible();
    const geometry = await item.evaluate((node) => {
      const item = node.getBoundingClientRect();
      const shell = node.closest("main")!.getBoundingClientRect();
      return {
        width: item.width,
        height: item.height,
        inside:
          item.left >= shell.left &&
          item.right <= shell.right &&
          item.top >= shell.top &&
          item.bottom <= shell.bottom,
      };
    });
    expect(geometry).toEqual({ width: 24, height: 24, inside: true });
  });
}

test("pinned groups retain choices across clock updates and relaunch the exact current item", async ({
  page,
}) => {
  await page.setViewportSize({ width: 720, height: 80 });
  await installFakeWails(page);
  const state = {
    ...presentation(2, ""),
    items: [
      { id: "pin:gap", name: "", icon: "", kind: "spacer", status: "ready" },
      { id: "pin:line", name: "", icon: "", kind: "separator", status: "ready" },
      {
        id: "pin:tools",
        name: "Tools",
        icon: "",
        kind: "group",
        status: "ready",
        members: [
          {
            id: "pin:editor",
            name: "Editor",
            icon: "",
            kind: "app",
            status: "ready",
            running: true,
            referenceRevision: 9,
          },
          { id: "pin:missing", name: "Missing", icon: "", kind: "app", status: "needsSelection" },
        ],
      },
    ],
  };
  await page.addInitScript((value) => {
    (window as any).__launcherState = value;
  }, state);
  await page.goto("/#/launcher/41");
  await page.getByRole("button", { name: "Tools", exact: true }).click();
  await expect(page.getByRole("button", { name: "Missing", exact: true })).toBeDisabled();
  expect(
    (await getCallRecords(page)).filter(([name]) => name === "ActivateLauncherItem"),
  ).toHaveLength(0);
  await page.evaluate(
    (value) => (window as any)._wails.dispatchWailsEvent({ name: "launcher:state", data: value }),
    { ...state, revision: 3 },
  );
  const editor = page.getByRole("button", { name: "Editor", exact: true });
  await expect(editor).toBeVisible();
  await editor.click({ button: "right" });
  await page.getByRole("button", { name: "Relaunch", exact: true }).click();
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual(["RelaunchLauncherItem", 7, "display-main", 41, 3, "pin:editor"]);
  await editor.click();
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual(["ActivateLauncherItem", 7, "display-main", 41, 3, "pin:editor"]);
  const cross = await page.getByLabel("Option Tab launcher").evaluate((node) => ({
    height: node.getBoundingClientRect().height,
    documentHeight: document.documentElement.scrollHeight,
  }));
  expect(cross.height).toBeLessThanOrEqual(80);
  expect(cross.documentHeight).toBeLessThanOrEqual(80);
});

for (const edge of ["bottom", "top", "left", "right"] as const) {
  test(`launcher fits 64px icons and clock at minimum ${edge} cross-axis`, async ({ page }) => {
    const vertical = edge === "left" || edge === "right";
    await page.setViewportSize(vertical ? { width: 72, height: 220 } : { width: 220, height: 72 });
    await page.emulateMedia({ colorScheme: "light" });
    await installFakeWails(page);
    const many = {
      ...presentation(2, "One"),
      edge,
      iconPx: 64,
      appearance: { ...presentation(2, "One").appearance, theme: "system" },
      items: Array.from({ length: 12 }, (_, index) => ({
        id: `item-${index}`,
        name: `Application ${index}`,
        icon: "",
      })),
      widgets: [
        {
          id: "clock",
          packageID: "org.optiontab.clock",
          digest: "clock",
          status: "ready",
          root: { kind: "row", children: [{ kind: "text", text: "09:41" }] },
        },
      ],
    };
    many.items[0] = {
      id: "group",
      name: "Work",
      icon: "",
      kind: "group",
      members: [{ id: "selected-member", name: "Selected", icon: "", kind: "app" }],
    } as any;
    await page.addInitScript((state) => {
      (window as any).__launcherState = state;
      (window as any).__launcherInteractionState = {
        epoch: state.epoch,
        displayUUID: state.displayUUID,
        session: state.session,
        presentationRevision: state.revision,
        admission: 2,
        sequence: 1,
        visible: true,
        selectedItemID: "selected-member",
        keyboardMode: true,
        gestureAvailable: true,
        pinchAvailable: false,
        swipeAvailable: false,
        letterInputAvailable: true,
        hapticsAvailable: false,
        configured: {
          enabled: true,
          preciseScroll: true,
          pinch: false,
          swipe: false,
          primaryAction: "next",
          towardAction: "showPreview",
          pinchAction: "showPreview",
          haptics: false,
          letterNavigation: true,
          enterActivates: false,
        },
        reason: "",
      };
      (window as any).__launcherWidgets = {
        epoch: state.epoch,
        displayUUID: state.displayUUID,
        session: state.session,
        profileID: state.profileID,
        revision: 1,
        visible: true,
        slots: [
          {
            id: "clock",
            name: { en: "Clock" },
            members: [{ id: "clock", name: { en: "Clock" } }],
            selectedID: "clock",
            status: "ready",
            state: {
              lease: {
                controllerEpoch: state.epoch,
                displayUUID: state.displayUUID,
                session: state.session,
                profileID: state.profileID,
                instanceID: "clock",
                digest: "clock",
                admissionEpoch: 1,
                revision: 1,
              },
              status: "ready",
              root: { key: "time", kind: "text", status: "ready", text: "09:41" },
            },
          },
        ],
      };
    }, many);
    await page.goto("/#/launcher/41");
    await expect(page.getByRole("button", { name: "Selected" })).toHaveAttribute(
      "aria-current",
      "true",
    );
    await expect(page.getByRole("textbox", { name: "Type a letter" })).toBeVisible();
    const geometry = await page.getByLabel("Option Tab launcher").evaluate((node, isVertical) => {
      const shell = node.getBoundingClientRect();
      const strip = node.querySelector("ul") as HTMLElement;
      const childrenInside = [...node.querySelectorAll("button, aside")].every((child) => {
        const rect = child.getBoundingClientRect();
        return isVertical
          ? rect.left >= shell.left && rect.right <= shell.right
          : rect.top >= shell.top && rect.bottom <= shell.bottom;
      });
      return {
        cross: isVertical ? shell.width : shell.height,
        documentCross: isVertical
          ? document.documentElement.scrollWidth
          : document.documentElement.scrollHeight,
        primaryOverflow: isVertical
          ? strip.scrollHeight > strip.clientHeight
          : strip.scrollWidth > strip.clientWidth,
        childrenInside,
        color: getComputedStyle(node).color,
      };
    }, vertical);
    expect(geometry.cross).toBeLessThanOrEqual(72);
    expect(geometry.documentCross).toBeLessThanOrEqual(72);
    expect(geometry.childrenInside).toBe(true);
    expect(geometry.primaryOverflow).toBe(true);
    expect(geometry.color).toBe("rgb(22, 33, 60)");
  });
}

test("launcher activates the exact rendered scope and tombstones retirement", async ({ page }) => {
  await installFakeWails(page);
  await page.addInitScript(
    (state) => {
      (window as any).__launcherState = state;
    },
    presentation(2, "Windowless Helper"),
  );
  await page.goto("/#/launcher/41");
  await page.getByRole("button", { name: "Windowless Helper" }).click();
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual(["ActivateLauncherItem", 7, "display-main", 41, 2, "opaque-Windowless Helper"]);
  await page.evaluate(
    (state) => (window as any)._wails.dispatchWailsEvent({ name: "launcher:state", data: state }),
    presentation(3, "", false),
  );
  await expect(page.getByRole("button")).toHaveCount(0);
  await page.evaluate(
    (state) => (window as any)._wails.dispatchWailsEvent({ name: "launcher:state", data: state }),
    presentation(4, "Revived"),
  );
  await expect(page.getByText("Revived")).toHaveCount(0);
});

test("launcher renders a stack and performs an explicit audio chooser action", async ({ page }) => {
  await installFakeWails(page);
  await page.addInitScript(
    (state) => {
      (window as any).__launcherState = state;
      (window as any).__launcherWidgets = {
        epoch: 7,
        displayUUID: "display-main",
        session: 41,
        profileID: "default",
        revision: 6,
        visible: true,
        slots: [
          {
            id: "stack:status",
            stackID: "status",
            name: { en: "Status" },
            members: [
              { id: "audio", name: { en: "Audio" } },
              { id: "network", name: { en: "Network" } },
            ],
            selectedID: "audio",
            status: "ready",
            state: {
              lease: {
                controllerEpoch: 7,
                displayUUID: "display-main",
                session: 41,
                profileID: "default",
                instanceID: "audio",
                digest: "audio-owned",
                admissionEpoch: 8,
                revision: 6,
              },
              status: "ready",
              root: {
                key: "output",
                kind: "button",
                status: "ready",
                text: "Choose output",
                actionToken: "audio-authority",
              },
            },
          },
        ],
      };
      (window as any).__widgetActionOptions = {
        options: [{ token: "display-output", label: "Studio Display" }],
      };
    },
    presentation(2, "Music"),
  );
  await page.goto("/#/launcher/41");
  await page.getByRole("button", { name: "Network" }).click();
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual([
      "SelectLauncherWidget",
      7,
      "display-main",
      41,
      "default",
      "status",
      "network",
    ]);
  await page.getByRole("button", { name: "Choose output" }).click();
  await page.getByRole("button", { name: "Studio Display" }).click();
  await expect
    .poll(async () => (await getCallRecords(page)).map((call) => call[0]))
    .toContain("PerformWidgetAction");
});

test("settings keeps launcher opt-in separate and offers native Dock recovery", async ({
  page,
}) => {
  await installFakeWails(page);
  await page.goto("/#/settings");
  await page.getByRole("tab", { name: "Dock" }).click();
  await expect(page.getByRole("checkbox", { name: "Enable replacement Dock" })).not.toBeChecked();
  await expect(page.getByRole("checkbox", { name: "Enable Clock" })).not.toBeChecked();
  await page.getByRole("checkbox", { name: "Enable Clock" }).click();
  await expect(
    page.getByRole("checkbox", { name: "Required · Read local time" }),
  ).not.toBeChecked();
  await page.getByRole("checkbox", { name: "Required · Read local time" }).click();
  await expect
    .poll(async () =>
      (await getCallRecords(page)).some(
        (call) =>
          call[0] === "SaveSettingsAtRevision" &&
          String(call[1]).includes('"grants":["clock.read"]') &&
          String(call[1]).includes(
            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
          ),
      ),
    )
    .toBe(true);
  await page.getByRole("button", { name: "Review local package…" }).click();
  await expect(page.getByText("status.otwidget")).toBeVisible();
  await expect(page.getByText("Not independently verified")).toBeVisible();
  await page.getByRole("button", { name: "Install reviewed package" }).click();
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual(["InstallReviewedWidget", "review-e2e"]);
  const settingsReads = (await getCallRecords(page)).filter(
    (call) => call[0] === "GetSettingsState",
  ).length;
  await page.getByRole("button", { name: "Use native Dock" }).click();
  await expect.poll(() => getCallRecords(page)).toContainEqual(["UseNativeDock"]);
  await expect
    .poll(
      async () =>
        (await getCallRecords(page)).filter((call) => call[0] === "GetSettingsState").length,
    )
    .toBeGreaterThan(settingsReads);
});

test("folder fan-out preserves explicit root open across parent clock ticks", async ({ page }) => {
  await installFakeWails(page);
  const state = {
    ...presentation(2, ""),
    items: [{ id: "pin:docs", name: "Documents", icon: "", kind: "folder", status: "ready" }],
  };
  await page.addInitScript((value) => {
    (window as any).__launcherState = value;
  }, state);
  await page.goto("/#/launcher/41");
  await page.getByRole("button", { name: "Documents", exact: true }).click();
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual(["ShowLauncherItemPanel", 7, "display-main", 41, 2, "pin:docs"]);
  await page.getByRole("button", { name: "Documents", exact: true }).click({ button: "right" });
  await page.evaluate(
    (value) => (window as any)._wails.dispatchWailsEvent({ name: "launcher:state", data: value }),
    { ...state, revision: 3 },
  );
  await page.getByRole("button", { name: "Open folder", exact: true }).click();
  await expect
    .poll(() => getCallRecords(page))
    .toContainEqual(["ActivateLauncherItem", 7, "display-main", 41, 3, "pin:docs"]);
});

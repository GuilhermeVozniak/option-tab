import type { Page } from "@playwright/test";
import { defaultSettings } from "../../src/lib/types";

// A complete Appearance matching emptyState defaults, tuned for deterministic
// e2e: dark theme, no apparition delay, animations off so nothing races.
const APPEARANCE = {
  style: "thumbnails",
  theme: "dark",
  sizePreset: "medium",
  maxRows: 4,
  maxColumns: 6,
  thumbnailMaxPx: 280,
  iconSizePx: 32,
  titleMaxWidthPx: 240,
  fontSizePx: 13,
  accentColor: "#3b82f6",
  backgroundOpacity: 0.85,
  blur: true,
  cornerRadiusPx: 12,
  showAppBadge: true,
  showTitle: true,
  showWindowControls: true,
  autoSize: true,
  apparitionDelayMs: 0,
  fadeOutAnimation: false,
  showStatusIcons: true,
  showSpaceNumbers: true,
  titleTruncation: "end",
  previewSelected: false,
  previewFade: false,
};

// Four synthetic windows: two on Space 1, one minimized, one on Space 2 (so the
// status icons, Space badges and other-Space marker all have something to show).
const ENTRIES = [
  {
    windowId: 1,
    appId: 1,
    appName: "Editor",
    bundleId: "com.ex.editor",
    title: "main.go",
    spaceId: 1,
    minimized: false,
    hidden: false,
    fullscreen: false,
  },
  {
    windowId: 2,
    appId: 2,
    appName: "Browser",
    bundleId: "com.ex.browser",
    title: "GitHub — option-tab",
    spaceId: 1,
    minimized: false,
    hidden: false,
    fullscreen: false,
  },
  {
    windowId: 3,
    appId: 3,
    appName: "Terminal",
    bundleId: "com.ex.term",
    title: "zsh",
    spaceId: 1,
    minimized: true,
    hidden: false,
    fullscreen: false,
  },
  {
    windowId: 4,
    appId: 4,
    appName: "Notes",
    bundleId: "com.ex.notes",
    title: "Parity",
    spaceId: 2,
    minimized: false,
    hidden: false,
    fullscreen: false,
  },
];

export type ShowState = Record<string, unknown>;

// showState builds a full SwitcherState payload for a switcher:show event.
export function showState(overrides: ShowState = {}): ShowState {
  return {
    open: true,
    style: "thumbnails",
    appearance: { ...APPEARANCE },
    placement: "cursorScreen",
    entries: ENTRIES.map((e) => ({ ...e })),
    selected: 0,
    search: "",
    shortcutId: 1,
    vimKeys: false,
    arrowKeys: true,
    mouseHover: true,
    activeSpaceId: 1,
    ...overrides,
  };
}

// Bound-method ids from the generated bindings (bindings/option-tab/app.js).
// They are deterministic hashes of the Go service method names, so they only
// change when a method is renamed — regenerate mentally via that file.
const METHOD = {
  Advance: 3974603045,
  Reverse: 2687580445,
  Select: 149583269,
  SetSearch: 1454005219,
  Confirm: 3228319335,
  ConfirmWindow: 3813603255,
  ConfirmApp: 1759019970,
  SelectApp: 523317996,
  SelectAppWindow: 76282064,
  Cancel: 2191755235,
  CloseSelected: 2868361114,
  MinimizeSelected: 1240045732,
  FullscreenSelected: 529416953,
  QuitSelectedApp: 3876391122,
  HideSelectedApp: 3942268823,
  GetVersion: 1049863377,
  GetSwitcherMaterialStatus: 3075636663,
  SetSwitcherMaterialRect: 724815411,
  InstallUpdate: 2443992793,
  PerformAction: 280563800,
  GetDockState: 1033939333,
  GetDockMaterialStatus: 167017963,
  SelectDockWindow: 1507952022,
  SelectDockContent: 2963493651,
  FocusDockWindow: 1587026944,
  PerformDockAction: 1943493959,
  SetDockPanelSize: 1080314307,
  SetDockPreviewRegions: 3579515437,
  BeginDockPreviewDrag: 3337788529,
  CancelDockPreviewDrag: 418174098,
  SetDockFolderSort: 3148788948,
  RequestDockFolderAccess: 1355609963,
  CancelDockFolderAccess: 3062959714,
  OpenDockFolderEntry: 1164782910,
  GetDockMonitorLockDisplays: 961307694,
  GetDockMonitorLockState: 3014544674,
  PlaceDockOnSelectedMonitor: 1589667255,
  CancelDockPlacement: 2631637315,
  GetMediaState: 3763583330,
  GetMediaPermissions: 3673411287,
  PerformMediaAction: 2082824200,
  PinMediaPanel: 900489102,
  CloseMediaPanel: 231734221,
  SetMediaPanelSize: 143498724,
  ImportMediaLyrics: 4054143088,
  CancelMediaLyricsImport: 3785335492,
  ReloadMediaLyrics: 3208690520,
  RemoveMediaLyrics: 2634844375,
  SetMediaLyricsOffset: 3423128546,
  ConnectMediaProvider: 2624774846,
  GetAutomationPreviewState: 473218209,
  SelectAutomationPreview: 2322291942,
  PerformAutomationPreviewAction: 513435453,
  SetAutomationPreviewSize: 3307079785,
  CloseAutomationPreview: 43965240,
  GetDiagnosticsReview: 3103844513,
  StartDiagnosticsRecording: 2282172420,
  StopDiagnosticsRecording: 76990490,
  ClearDiagnostics: 2969743170,
  SaveDiagnosticsReport: 1276106854,
  ActivateLauncherItem: 529143415,
  RelaunchLauncherItem: 3869413604,
  GetLauncherItemSettings: 1300445933,
  GetLauncherItemStatus: 117720418,
  GetLauncherState: 1942731528,
  GetLauncherItemPanelState: 2821689331,
  ShowLauncherItemPanel: 3216568691,
  CloseLauncherItemPanel: 1453209586,
  SetLauncherItemPanelSize: 3203216027,
  SetLauncherFolderSort: 419936015,
  SetLauncherFolderView: 4073294914,
  OpenLauncherFolderEntry: 2663677683,
  SelectLauncherWindow: 3962714865,
  PerformLauncherWindowAction: 2955927072,
  GetLauncherStatus: 3297738801,
  GetLauncherAppChoices: 1561877478,
  GetLauncherProfileExport: 2209096222,
  ImportLauncherProfile: 216581033,
  PreviewLauncherProfileImport: 3977947893,
  GetLauncherWidgets: 1662395444,
  GetWidgetActionOptions: 1913639053,
  GetWidgetAsset: 1591270545,
  GetWidgetCatalog: 1508342138,
  PerformWidgetAction: 4243588732,
  SelectLauncherWidget: 3729375001,
  CancelWidgetPackageReview: 1933127773,
  GetWidgetPackageStatus: 2166434855,
  InstallReviewedWidget: 4223715965,
  RemoveWidgetPackage: 2737711739,
  ReviewLocalWidgetPackage: 460648480,
  UseNativeDock: 421143656,
  GetSettingsState: 620781575,
  SaveSettingsAtRevision: 1896415605,
  MutateLauncherItems: 3805544533,
  GetLauncherInteractionState: 59755958,
  GetLauncherInteractionCapabilities: 447462271,
  SetLauncherKeyboardMode: 2605124801,
  CommitLauncherLetter: 3590058620,
  ActivateLauncherSelection: 4123151142,
  SetLauncherReorderTarget: 1107160523,
  GetLauncherBadges: 3953438223,
  SaveSettings: 1949631069,
} as const;

const METHOD_NAME = new Map<number, string>(Object.entries(METHOD).map(([name, id]) => [id, name]));

// installFakeWails emulates the Wails v3 backend in a plain browser:
//
//   - RPC: the generated bindings POST {object, method, args} to
//     /wails/runtime; a route handler answers navigation/action calls, driving
//     a shared fake controller state and re-emitting switcher:update.
//   - Events: page-side dispatch goes through window._wails.dispatchWailsEvent,
//     which the real @wailsio/runtime installs on import.
//   - Keyboard: the production overlay takes keys from native-tap "switcher:key"
//     events, so a DOM keydown listener re-emits them in that exact shape (the
//     tap only forwards while the switcher is open, hence the __state.open gate).
//
// Window/app action calls are recorded on window.__calls for assertions.
export async function installFakeWails(page: Page): Promise<void> {
  await page.addInitScript((initialSettingsJSON) => {
    const w = window as unknown as {
      __calls: unknown[][];
      __state: Record<string, unknown> | null;
      __actionResult?: { succeeded: number; failures: { windowId: number; error: string }[] };
      __actionError?: string;
      __dockState?: Record<string, unknown> | null;
      __dockMaterialStatus?: Record<string, unknown>;
      __switcherMaterialStatus?: Record<string, unknown>;
      __dockContentError?: string;
      __dockLockState?: Record<string, unknown>;
      __dockLockDisplays?: unknown[];
      __dockPlacementResult?: Record<string, unknown>;
      __mediaState?: Record<string, unknown> | null;
      __mediaPermissions?: Record<string, unknown>;
      __mediaPinSession?: number;
      __mediaActionError?: string;
      __automationPreviewState?: Record<string, unknown> | null;
      __launcherProfileExport?: string;
      __launcherProfileImportReview?: Record<string, unknown>;
      __launcherProfileImportResult?: Record<string, unknown>;
      __automationPreviewError?: string;
      __diagnosticsReview?: Record<string, unknown>;
      __diagnosticsSaveResult?: Record<string, unknown>;
      __diagnosticsError?: Record<string, string>;
      __launcherState?: Record<string, unknown> | null;
      __launcherInteractionState?: Record<string, unknown> | null;
      __launcherStatus?: Record<string, unknown>;
      __launcherAppChoices?: Array<{ name: string; bundleID: string }>;
      __launcherWidgets?: Record<string, unknown>;
      __widgetCatalog?: unknown[];
      __widgetActionOptions?: Record<string, unknown>;
      __widgetPackageStatus?: Record<string, unknown>;
      __widgetPackageReview?: Record<string, unknown>;
      __settingsRevision?: number;
      __settingsJSON?: string;
      _wails?: { dispatchWailsEvent?: (ev: { name: string; data: unknown }) => void };
    };
    w.__calls = [];
    w.__settingsRevision = 1;
    w.__settingsJSON = initialSettingsJSON;
    w.__state = null;
    w.__actionResult = undefined;
    w.__actionError = undefined;
    w.__dockState = null;
    w.__dockMaterialStatus = { session: 0, revision: 0, state: "unavailable" };
    w.__switcherMaterialStatus = { session: 0, revision: 0, state: "unavailable" };
    w.__mediaState = null;
    w.__mediaPermissions = {};
    w.__automationPreviewState = null;
    w.__launcherState = null;
    w.__launcherInteractionState = null;
    w.__launcherWidgets = { visible: false, revision: 0, slots: [] };
    w.__widgetCatalog = [
      {
        packageID: "org.optiontab.clock",
        digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
        version: "1.0.0",
        name: { en: "Clock", "pt-BR": "Relógio", es: "Reloj" },
        description: { en: "Local time", "pt-BR": "Hora local", es: "Hora local" },
        requiredCapabilities: ["clock.read"],
        optionalCapabilities: [],
        settings: [],
        builtin: true,
      },
    ];
    w.__widgetActionOptions = { options: [] };
    w.__widgetPackageStatus = { available: true, busy: false, reason: "" };
    w.__widgetPackageReview = {
      token: "review-e2e",
      sourceName: "status.otwidget",
      expiresAt: "2026-09-07T18:00:00Z",
      alreadyInstalled: false,
      package: {
        packageID: "org.example.status",
        digest: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
        version: "1.0.0",
        name: { en: "Status card" },
        description: { en: "Shows local status" },
        requiredCapabilities: ["network.status.read"],
        optionalCapabilities: [],
        settings: [],
        builtin: false,
      },
    };
    w.__launcherStatus = {
      epoch: 0,
      revision: 0,
      enabled: false,
      status: "disabled",
      reason: "",
      recoveryLatched: false,
      displays: [],
      clockPackageID: "org.optiontab.clock",
      clockDigest: "sha256:c2504147560285311f61886cae7a1f1396781443c9db52a85a04fbe669b2ded7d",
    };
    w.__launcherAppChoices = [
      { name: "Editor", bundleID: "com.example.editor" },
      { name: "Editor", bundleID: "org.example.editor" },
    ];
    w.__diagnosticsReview = {
      token: "diagnostics-review-1",
      json: '{"schemaVersion":1,"recording":false}',
      expiresAt: "2026-09-07T12:00:00Z",
      recording: false,
      dropped: 0,
    };
    w.__diagnosticsSaveResult = { status: "saved" };
    w.__diagnosticsError = {};
    w.__dockLockState = {
      session: 0,
      revision: 0,
      generation: 0,
      sequence: 0,
      status: "disabled",
      reason: "",
      targetUUID: "",
      actualUUID: "",
      edge: "",
      displays: [],
    };
    w.__dockLockDisplays = [];
    w.__dockPlacementResult = {
      requestId: 1,
      status: "protected",
      reason: "",
      actualUUID: "",
      verified: true,
      cursorRestored: true,
    };
    window.addEventListener("keydown", (e) => {
      if (!w.__state?.open) return; // the native tap only forwards while open
      w._wails?.dispatchWailsEvent?.({
        name: "switcher:key",
        data: {
          session: (w.__state.session as number | undefined) ?? 0,
          key: e.key,
          code: e.code,
          shift: e.shiftKey,
          ctrl: e.ctrlKey,
          alt: e.altKey,
          meta: e.metaKey,
        },
      });
    });
  }, JSON.stringify(defaultSettings));

  await page.route("**/wails/runtime", async (route) => {
    // Binding calls arrive as {object: CallBinding, method: 0, args: {"call-id",
    // methodID, args: [...]}} — the numeric methodID selects the Go method.
    const body = route.request().postDataJSON() as {
      args?: { methodID?: number; args?: unknown[] };
    };
    const name = METHOD_NAME.get(body.args?.methodID ?? 0);
    const args = body.args?.args ?? [];
    const json = (result: unknown) =>
      route.fulfill({ contentType: "application/json", body: JSON.stringify(result) });

    const evaluate = (fn: (arg: unknown) => void, arg?: unknown) => page.evaluate(fn, arg);

    switch (name) {
      case "GetLauncherBadges": {
        await evaluate((n) => (window as any).__calls.push([n]), name);
        return json(await page.evaluate(() => (window as any).__launcherBadges ?? { entries: [] }));
      }
      case "GetVersion":
        return json("0.0.0-e2e");
      case "GetSettingsState":
        return json(
          await page.evaluate(() => {
            const w = window as any;
            w.__calls.push(["GetSettingsState"]);
            return { revision: w.__settingsRevision, json: w.__settingsJSON };
          }),
        );
      case "GetDockState":
        return json(await page.evaluate(() => (window as any).__dockState));
      case "GetDockMaterialStatus":
        return json(await page.evaluate(() => (window as any).__dockMaterialStatus));
      case "GetSwitcherMaterialStatus":
        return json(await page.evaluate(() => (window as any).__switcherMaterialStatus));
      case "GetDockMonitorLockState":
        return json(await page.evaluate(() => (window as any).__dockLockState));
      case "GetDockMonitorLockDisplays":
        return json(await page.evaluate(() => (window as any).__dockLockDisplays));
      case "GetMediaState":
        return json(await page.evaluate(() => (window as any).__mediaState));
      case "GetMediaPermissions":
        return json(await page.evaluate(() => (window as any).__mediaPermissions));
      case "GetAutomationPreviewState":
        return json(await page.evaluate(() => (window as any).__automationPreviewState));
      case "GetDiagnosticsReview": {
        await evaluate((n) => (window as any).__calls.push([n]), name);
        return json(await page.evaluate(() => (window as any).__diagnosticsReview));
      }
      case "GetLauncherState":
        return json(await page.evaluate(() => (window as any).__launcherState));
      case "GetLauncherInteractionState":
        return json(await page.evaluate(() => (window as any).__launcherInteractionState));
      case "GetLauncherInteractionCapabilities":
        return json({
          gestureAvailable: true,
          pinchAvailable: false,
          swipeAvailable: false,
          letterInputAvailable: false,
          hapticsAvailable: true,
          reason: "deliveryUnverified",
        });
      case "GetLauncherItemPanelState":
        return json(await page.evaluate(() => (window as any).__launcherItemPanelState));
      case "GetLauncherItemSettings":
        return json({
          profileID: args[0],
          revision: "fixture-empty",
          items: [],
          references: [],
          iconIDs: [],
        });
      case "GetLauncherItemStatus":
        return json({ available: true, busy: false, reason: "" });
      case "GetLauncherStatus":
        return json(await page.evaluate(() => (window as any).__launcherStatus));
      case "GetLauncherAppChoices":
        return json(await page.evaluate(() => (window as any).__launcherAppChoices));
      case "GetLauncherProfileExport":
        await evaluate(([n, a]) => (window as any).__calls.push([n, ...a]), [name, args]);
        return json(await page.evaluate(() => (window as any).__launcherProfileExport));
      case "PreviewLauncherProfileImport":
        await evaluate(([n, a]) => (window as any).__calls.push([n, ...a]), [name, args]);
        return json(await page.evaluate(() => (window as any).__launcherProfileImportReview));
      case "ImportLauncherProfile":
        await evaluate(([n, a]) => (window as any).__calls.push([n, ...a]), [name, args]);
        return json(
          await page.evaluate(() => {
            const w = window as any;
            const result = w.__launcherProfileImportResult;
            if (result?.settingsJSON) {
              w.__settingsJSON = result.settingsJSON;
              w.__settingsRevision++;
            }
            return result;
          }),
        );
      case "GetLauncherWidgets":
        return json(await page.evaluate(() => (window as any).__launcherWidgets));
      case "GetWidgetCatalog":
        return json(await page.evaluate(() => (window as any).__widgetCatalog));
      case "GetWidgetPackageStatus":
        return json(await page.evaluate(() => (window as any).__widgetPackageStatus));
      case "ReviewLocalWidgetPackage":
        await evaluate(([n, a]) => (window as any).__calls.push([n, ...a]), [name, args]);
        return json(await page.evaluate(() => (window as any).__widgetPackageReview));
      case "InstallReviewedWidget": {
        await evaluate(([n, a]) => (window as any).__calls.push([n, ...a]), [name, args]);
        return json(await page.evaluate(() => (window as any).__widgetPackageReview.package));
      }
      case "GetWidgetActionOptions":
        await evaluate(([n, a]) => (window as any).__calls.push([n, ...a]), [name, args]);
        return json(await page.evaluate(() => (window as any).__widgetActionOptions));
      case "GetWidgetAsset":
        await evaluate(([n, a]) => (window as any).__calls.push([n, ...a]), [name, args]);
        return json("");
      case "ActivateLauncherItem":
      case "RelaunchLauncherItem":
      case "ShowLauncherItemPanel":
      case "CloseLauncherItemPanel":
      case "SetLauncherItemPanelSize":
      case "SetLauncherFolderSort":
      case "SetLauncherFolderView":
      case "OpenLauncherFolderEntry":
      case "SelectLauncherWindow":
      case "PerformLauncherWindowAction":
      case "PerformWidgetAction":
      case "MutateLauncherItems":
      case "SetLauncherKeyboardMode":
      case "CommitLauncherLetter":
      case "ActivateLauncherSelection":
      case "SetLauncherReorderTarget":
      case "SelectLauncherWidget":
      case "CancelWidgetPackageReview":
      case "RemoveWidgetPackage":
      case "UseNativeDock": {
        await evaluate(([n, a]) => (window as any).__calls.push([n, ...a]), [name, args]);
        return json(null);
      }
      case "PinMediaPanel": {
        const pinned = await page.evaluate(
          ([n, a]) => {
            const w = window as any;
            w.__calls.push([n, ...a]);
            return w.__mediaPinSession ?? a[0];
          },
          [name, args],
        );
        return json(pinned);
      }
      case "PlaceDockOnSelectedMonitor": {
        const result = await page.evaluate(
          ([n, a]) => {
            const w = window as any;
            w.__calls.push([n, ...a]);
            return w.__dockPlacementResult;
          },
          [name, args],
        );
        return json(result);
      }
      case "SaveSettingsAtRevision": {
        const result = await page.evaluate(
          ([n, a]) => {
            const w = window as any;
            w.__calls.push([n, ...a]);
            const expected = Number(a[1]);
            const revision = Number(w.__settingsRevision ?? 1);
            if (expected !== revision) return { error: "settings: staleRevision" };
            w.__settingsRevision = revision + 1;
            w.__settingsJSON = a[0];
            return { value: { revision: w.__settingsRevision, json: w.__settingsJSON } };
          },
          [name, args],
        );
        if (result.error) return route.fulfill({ status: 500, body: result.error });
        return json(result.value);
      }
      case "CancelDockPlacement":
      case "SaveSettings": {
        await evaluate(([n, a]) => (window as any).__calls.push([n, ...a]), [name, args]);
        return json(null);
      }
      case "InstallUpdate":
        // No real install in e2e: acknowledge and stay put.
        await evaluate((n) => {
          (window as any).__calls.push([n]);
        }, name);
        return json(null);
      case "Advance":
      case "Reverse":
      case "Select":
      case "SetSearch": {
        await evaluate(
          ([n, a]) => {
            const w = window as any;
            const st = w.__state;
            if (!st) return;
            const clamp = (i: number, len: number) => ((i % len) + len) % len;
            const count = st.mode === "apps" ? st.apps.length : st.entries.length;
            if (n === "Advance") st.selected = clamp(st.selected + 1, count);
            else if (n === "Reverse") st.selected = clamp(st.selected - 1, count);
            else if (n === "Select") st.selected = a;
            else st.search = a;
            if ((st.session ?? 0) > 0) st.revision = (st.revision ?? 0) + 1;
            w.__state = { ...st };
            w._wails?.dispatchWailsEvent?.({ name: "switcher:update", data: w.__state });
          },
          [name, args[0]],
        );
        return json(null);
      }
      case "Confirm":
      case "ConfirmWindow":
      case "ConfirmApp":
      case "Cancel": {
        await evaluate(
          ([n, a]) => {
            const w = window as any;
            w.__calls.push(a === undefined ? [n] : [n, a]);
            w.__state = { ...w.__state, open: false };
            const session = w.__state?.session ?? 0;
            const revision = session > 0 ? (w.__state?.revision ?? 0) + 1 : 0;
            w._wails?.dispatchWailsEvent?.({
              name: "switcher:hide",
              data: session > 0 ? { session, revision } : null,
            });
          },
          [name, args[0]],
        );
        return json(null);
      }
      case "CloseSelected":
      case "MinimizeSelected":
      case "FullscreenSelected":
      case "QuitSelectedApp":
      case "HideSelectedApp": {
        await evaluate((n) => {
          (window as any).__calls.push([n]);
        }, name);
        return json(null);
      }
      case "SetSwitcherMaterialRect":
      case "SelectApp":
      case "SelectAppWindow":
      case "SelectDockWindow":
      case "SelectDockContent":
      case "SetDockPanelSize":
      case "SetDockPreviewRegions":
      case "BeginDockPreviewDrag":
      case "CancelDockPreviewDrag":
      case "SetDockFolderSort":
      case "RequestDockFolderAccess":
      case "CancelDockFolderAccess":
      case "OpenDockFolderEntry": {
        await evaluate(([n, a]) => (window as any).__calls.push([n, ...a]), [name, args]);
        if (
          name === "SelectDockContent" &&
          (await page.evaluate(() => (window as any).__dockContentError))
        )
          return route.fulfill({
            status: 500,
            body: await page.evaluate(() => (window as any).__dockContentError),
          });
        return json(null);
      }
      case "PerformMediaAction":
      case "CloseMediaPanel":
      case "SetMediaPanelSize":
      case "ImportMediaLyrics":
      case "CancelMediaLyricsImport":
      case "ReloadMediaLyrics":
      case "RemoveMediaLyrics":
      case "SetMediaLyricsOffset":
      case "ConnectMediaProvider": {
        await evaluate(([n, a]) => (window as any).__calls.push([n, ...a]), [name, args]);
        if (
          name === "PerformMediaAction" &&
          (await page.evaluate(() => (window as any).__mediaActionError))
        )
          return route.fulfill({
            status: 500,
            body: await page.evaluate(() => (window as any).__mediaActionError),
          });
        return json(name === "ConnectMediaProvider" ? { status: "ready", reason: "" } : null);
      }
      case "SelectAutomationPreview":
      case "SetAutomationPreviewSize":
      case "CloseAutomationPreview":
      case "PerformAutomationPreviewAction": {
        await evaluate(([n, a]) => (window as any).__calls.push([n, ...a]), [name, args]);
        if (
          name === "PerformAutomationPreviewAction" &&
          (await page.evaluate(() => (window as any).__automationPreviewError))
        )
          return route.fulfill({
            status: 500,
            body: await page.evaluate(() => (window as any).__automationPreviewError),
          });
        return json(null);
      }
      case "StartDiagnosticsRecording":
      case "StopDiagnosticsRecording":
      case "ClearDiagnostics":
      case "SaveDiagnosticsReport": {
        await evaluate(([n, a]) => (window as any).__calls.push([n, ...a]), [name, args]);
        const error = await page.evaluate((n) => (window as any).__diagnosticsError?.[n], name);
        if (error) return route.fulfill({ status: 500, body: error });
        if (name === "SaveDiagnosticsReport")
          return json(await page.evaluate(() => (window as any).__diagnosticsSaveResult));
        return json(null);
      }
      case "FocusDockWindow":
      case "PerformDockAction": {
        const outcome = await page.evaluate(
          ([n, a]) => {
            const w = window as any;
            w.__calls.push([n, ...a]);
            if (w.__actionError)
              return { succeeded: 0, failures: [{ windowId: a[2] ?? 0, error: w.__actionError }] };
            return w.__actionResult ?? { succeeded: 1, failures: [] };
          },
          [name, args],
        );
        return json(outcome);
      }
      case "PerformAction": {
        const outcome = await page.evaluate(
          ([n, actionArgs]) => {
            const w = window as any;
            w.__calls.push([n, ...actionArgs]);
            if (w.__actionError) {
              return {
                succeeded: 0,
                failures: [{ windowId: actionArgs[1] ?? 0, error: w.__actionError }],
              };
            }
            return w.__actionResult ?? { succeeded: 1, failures: [] };
          },
          [name, args],
        );
        return json(outcome);
      }
      default:
        // Anything unmapped behaves as "no backend": the bridge degrades to its
        // browser defaults (empty settings, no permissions UI, ...).
        return route.fulfill({ status: 500, body: "unbound method" });
    }
  });
}

// emitShow pushes a switcher:show with the given state (also seeding the fake
// controller state). The runtime's listener registry is module-private, so a
// lost race shows up as the overlay never appearing — retry a few times.
export async function emitShow(page: Page, state: ShowState): Promise<void> {
  await page.waitForFunction(() => {
    const w = window as any;
    return typeof w._wails?.dispatchWailsEvent === "function";
  });
  for (let attempt = 0; attempt < 10; attempt++) {
    await page.evaluate((s) => {
      const w = window as any;
      w.__state = s;
      w._wails.dispatchWailsEvent({ name: "switcher:show", data: s });
    }, state);
    try {
      await page
        .locator(".ot-overlay,.ot-app-switcher")
        .waitFor({ state: "visible", timeout: 500 });
      return;
    } catch {
      // subscription not ready yet; dispatch again
    }
  }
  throw new Error("switcher:show never rendered the overlay");
}

// getCalls returns the recorded bound-method names, in order.
export async function getCalls(page: Page): Promise<string[]> {
  return page.evaluate(() => ((window as any).__calls || []).map((c: unknown[]) => c[0] as string));
}

// getCallRecords returns method names with their arguments for assertions
// where the target ID is part of the behavior under test.
export async function getCallRecords(page: Page): Promise<unknown[][]> {
  return page.evaluate(() => (window as any).__calls || []);
}

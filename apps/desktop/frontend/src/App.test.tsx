import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

// Mock the Wails v3 seams: the generated App service bindings and the
// @wailsio/runtime event bus, with a handler registry so tests can fire Go-side
// events (switcher:show, prefs:tab, ...) exactly like the runtime does.
const eventHandlers = new Map<string, (ev: { data: unknown }) => void>();

vi.mock("@wailsio/runtime", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@wailsio/runtime")>();
  return {
    ...actual,
    Events: {
      On: vi.fn((name: string, cb: (ev: { data: unknown }) => void) => {
        eventHandlers.set(name, cb);
        return () => eventHandlers.delete(name);
      }),
    },
  };
});

vi.mock("../bindings/option-tab/app.js", () => ({
  GetLauncherItemPanelState: vi.fn().mockResolvedValue(null),
  ShowLauncherItemPanel: vi.fn().mockResolvedValue(undefined),
  CloseLauncherItemPanel: vi.fn().mockResolvedValue(undefined),
  SetLauncherItemPanelSize: vi.fn().mockResolvedValue(undefined),
  SetLauncherFolderSort: vi.fn().mockResolvedValue(undefined),
  SetLauncherFolderView: vi.fn().mockResolvedValue(undefined),
  OpenLauncherFolderEntry: vi.fn().mockResolvedValue(undefined),
  SelectLauncherWindow: vi.fn().mockResolvedValue(undefined),
  PerformLauncherWindowAction: vi.fn().mockResolvedValue(undefined),
  PerformAction: vi.fn().mockResolvedValue({ succeeded: 1, failures: [] }),
  Advance: vi.fn().mockResolvedValue(undefined),
  Reverse: vi.fn().mockResolvedValue(undefined),
  Confirm: vi.fn().mockResolvedValue(undefined),
  ConfirmWindow: vi.fn().mockResolvedValue(undefined),
  SelectApp: vi.fn().mockResolvedValue(undefined),
  SelectAppWindow: vi.fn().mockResolvedValue(undefined),
  ConfirmApp: vi.fn().mockResolvedValue(undefined),
  GetSwitcherMaterialStatus: vi
    .fn()
    .mockResolvedValue({ session: 0, revision: 0, state: "unavailable" }),
  SetSwitcherMaterialRect: vi.fn().mockResolvedValue(undefined),
  GetDockState: vi.fn().mockResolvedValue(null),
  GetDockMaterialStatus: vi
    .fn()
    .mockResolvedValue({ session: 0, revision: 0, state: "unavailable" }),
  SelectDockWindow: vi.fn().mockResolvedValue(undefined),
  SelectDockContent: vi.fn().mockResolvedValue(undefined),
  FocusDockWindow: vi.fn().mockResolvedValue({ succeeded: 1, failures: [] }),
  PerformDockAction: vi.fn().mockResolvedValue({ succeeded: 1, failures: [] }),
  SetDockPanelSize: vi.fn().mockResolvedValue(undefined),
  SetDockPreviewRegions: vi.fn().mockResolvedValue(undefined),
  BeginDockPreviewDrag: vi.fn().mockResolvedValue(undefined),
  CancelDockPreviewDrag: vi.fn().mockResolvedValue(undefined),
  SetDockFolderSort: vi.fn().mockResolvedValue(undefined),
  RequestDockFolderAccess: vi.fn().mockResolvedValue(undefined),
  CancelDockFolderAccess: vi.fn().mockResolvedValue(undefined),
  OpenDockFolderEntry: vi.fn().mockResolvedValue(undefined),
  GetDockMonitorLockState: vi
    .fn()
    .mockResolvedValue({ session: 0, revision: 0, sequence: 0, status: "disabled", displays: [] }),
  GetDockMonitorLockDisplays: vi.fn().mockResolvedValue([]),
  PlaceDockOnSelectedMonitor: vi
    .fn()
    .mockResolvedValue({ status: "protected", reason: "", verified: true }),
  CancelDockPlacement: vi.fn().mockResolvedValue(undefined),
  GetMediaState: vi.fn().mockResolvedValue(null),
  PerformMediaAction: vi.fn().mockResolvedValue(undefined),
  PinMediaPanel: vi.fn().mockResolvedValue(0),
  CloseMediaPanel: vi.fn().mockResolvedValue(undefined),
  SetMediaPanelSize: vi.fn().mockResolvedValue(undefined),
  ImportMediaLyrics: vi.fn().mockResolvedValue(undefined),
  CancelMediaLyricsImport: vi.fn().mockResolvedValue(undefined),
  ReloadMediaLyrics: vi.fn().mockResolvedValue(undefined),
  RemoveMediaLyrics: vi.fn().mockResolvedValue(undefined),
  SetMediaLyricsOffset: vi.fn().mockResolvedValue(undefined),
  ConnectMediaProvider: vi.fn().mockResolvedValue({ status: "ready", reason: "" }),
  GetMediaPermissions: vi.fn().mockResolvedValue({}),
  Cancel: vi.fn().mockResolvedValue(undefined),
  Select: vi.fn().mockResolvedValue(undefined),
  SetSearch: vi.fn().mockResolvedValue(undefined),
  CloseSelected: vi.fn().mockResolvedValue(undefined),
  MinimizeSelected: vi.fn().mockResolvedValue(undefined),
  FullscreenSelected: vi.fn().mockResolvedValue(undefined),
  QuitSelectedApp: vi.fn().mockResolvedValue(undefined),
  HideSelectedApp: vi.fn().mockResolvedValue(undefined),
  GetSettings: vi.fn().mockResolvedValue("{}"),
  SaveSettings: vi.fn().mockResolvedValue(undefined),
  GetSettingsState: vi.fn().mockResolvedValue({ revision: 0, json: "{}" }),
  SaveSettingsAtRevision: vi.fn().mockResolvedValue({ revision: 0, json: "{}" }),
  GetPermissions: vi.fn().mockResolvedValue("{}"),
  GetVersion: vi.fn().mockResolvedValue("1.2.3"),
  InstallUpdate: vi.fn().mockResolvedValue(undefined),
  GetCrashReport: vi.fn().mockResolvedValue(""),
  GetAutomationPreviewState: vi.fn().mockResolvedValue(null),
  SelectAutomationPreview: vi.fn().mockResolvedValue(undefined),
  PerformAutomationPreviewAction: vi.fn().mockResolvedValue(undefined),
  SetAutomationPreviewSize: vi.fn().mockResolvedValue(undefined),
  CloseAutomationPreview: vi.fn().mockResolvedValue(undefined),
  GetLauncherState: vi.fn().mockResolvedValue(null),
  GetLauncherInteractionCapabilities: vi.fn().mockResolvedValue({
    gestureAvailable: true,
    pinchAvailable: false,
    swipeAvailable: false,
    letterInputAvailable: false,
    hapticsAvailable: true,
    reason: "deliveryUnverified",
  }),
  GetLauncherStatus: vi.fn().mockResolvedValue({
    epoch: 0,
    revision: 0,
    enabled: false,
    status: "disabled",
    reason: "",
    recoveryLatched: false,
    displays: [],
    clockPackageID: "org.optiontab.clock",
    clockDigest: "digest",
  }),
  GetWidgetCatalog: vi.fn().mockResolvedValue([]),
  GetLauncherWidgets: vi.fn().mockResolvedValue({ visible: false, slots: [] }),
  GetWidgetActionOptions: vi.fn().mockResolvedValue({ options: [] }),
  PerformWidgetAction: vi.fn().mockResolvedValue(undefined),
  GetWidgetAsset: vi.fn().mockResolvedValue(""),
  SelectLauncherWidget: vi.fn().mockResolvedValue(undefined),
  GetWidgetPackageStatus: vi.fn().mockResolvedValue({ available: false, busy: false, reason: "" }),
  ReviewLocalWidgetPackage: vi.fn().mockResolvedValue({}),
  InstallReviewedWidget: vi.fn().mockResolvedValue({}),
  CancelWidgetPackageReview: vi.fn().mockResolvedValue(undefined),
  RemoveWidgetPackage: vi.fn().mockResolvedValue(undefined),
  GetLauncherAppChoices: vi.fn().mockResolvedValue([]),
  GetLauncherProfileExport: vi.fn().mockResolvedValue("{}"),
  SaveLauncherProfileExport: vi.fn().mockResolvedValue({ status: "saved" }),
  SaveSettingsExport: vi.fn().mockResolvedValue({ status: "saved" }),
  PreviewLauncherProfileImport: vi.fn().mockResolvedValue({}),
  ImportLauncherProfile: vi.fn().mockResolvedValue({}),
  GetLauncherItemSettings: vi.fn().mockResolvedValue({
    profileID: "default",
    revision: "r1",
    items: [],
    references: [],
    iconIDs: [],
  }),
  SetLauncherItems: vi.fn().mockResolvedValue({
    profileID: "default",
    revision: "r2",
    items: [],
    references: [],
    iconIDs: [],
  }),
  GetLauncherItemStatus: vi.fn().mockResolvedValue({ available: false, busy: false, reason: "" }),
  ChooseLauncherItemReference: vi.fn().mockResolvedValue({}),
  RelinkLauncherItemReference: vi.fn().mockResolvedValue({}),
  CancelLauncherItemSelection: vi.fn().mockResolvedValue(undefined),
  ChooseLauncherItemIcon: vi.fn().mockResolvedValue({}),
  RemoveUnusedLauncherReference: vi.fn().mockResolvedValue(undefined),
  RemoveUnusedLauncherIcon: vi.fn().mockResolvedValue(undefined),
  GetLauncherItemIcon: vi.fn().mockResolvedValue({}),
  RelaunchLauncherItem: vi.fn().mockResolvedValue(undefined),
  ActivateLauncherItem: vi.fn().mockResolvedValue(undefined),
  UseNativeDock: vi.fn().mockResolvedValue(undefined),
}));

import * as AppService from "../bindings/option-tab/app.js";
import App from "./App";
import { resetBackendProbeForTests } from "./lib/bridge";
import type { Entry, SwitcherState } from "./lib/types";
import { defaultSettings, emptyState } from "./lib/types";

const mocked = vi.mocked(AppService);

function appEntry(windowId: number, title: string): Entry {
  return {
    windowId,
    appId: windowId,
    title,
    appName: title,
    bundleId: "",
    minimized: false,
    hidden: false,
    fullscreen: false,
  };
}

function openSwitcherState(overrides: Partial<SwitcherState> = {}): SwitcherState {
  return { ...emptyState, open: true, selected: 0, ...overrides };
}

function dockMedia(session: number, revision: number, title: string) {
  return {
    session,
    revision,
    open: true,
    pinned: false,
    pinnable: true,
    provider: "music",
    scope: {
      provider: "music",
      process: { pid: 3, launchID: "x" },
      generation: 1,
      trackEpoch: revision,
      trackID: title,
    },
    sample: {
      provider: "music",
      process: { pid: 3, launchID: "x" },
      generation: 1,
      sequence: 1,
      trackEpoch: revision,
      track: { id: title, title, artist: "Artist", album: "Album", durationMS: 1000 },
      playback: "paused",
      positionMS: 10,
      observedAt: "",
      status: "ready",
      reason: "",
      capabilities: { play: true, pause: true, previous: true, next: true, seek: true },
      artworkToken: "",
    },
    appearance: emptyState.appearance,
    artwork: { status: "missing", reason: "", image: "" },
    lyrics: { documentID: "", status: "missing", reason: "", cues: [], offsetMS: 0 },
    positionMS: 10,
    activeCue: -1,
    error: "",
  };
}

function dockMediaState(revision: number, media: ReturnType<typeof dockMedia>) {
  return {
    session: 40,
    revision,
    open: true,
    contentKind: "media",
    item: {
      appId: 0,
      bundleId: "",
      path: "",
      title: "Media",
      bounds: { x: 0, y: 0, w: 40, h: 40 },
      screenId: 1,
      edge: "bottom",
      kind: "media",
    },
    entries: [],
    selectedWindowId: 0,
    appearance: emptyState.appearance,
    emptyReason: "",
    media,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  eventHandlers.clear();
  resetBackendProbeForTests();
  window.location.hash = "";
  mocked.GetSettingsState.mockResolvedValue({
    revision: 1,
    json: JSON.stringify({
      ...defaultSettings,
      behavior: { ...defaultSettings.behavior, onboarded: true },
    }),
  } as never);
  mocked.SaveSettingsAtRevision.mockImplementation(
    (json: string, revision: number) => Promise.resolve({ revision: revision + 1, json }) as never,
  );
});

describe("App", () => {
  it.each([
    false,
    true,
  ])("recovers a failed permission snapshot without replacing a newer pending event (%s)", async (newerEvent) => {
    window.location.hash = "#settings";
    mocked.GetSettingsState.mockResolvedValueOnce({
      revision: 1,
      json: JSON.stringify({
        ...defaultSettings,
        behavior: { ...defaultSettings.behavior, onboarded: true },
        dock: {
          ...defaultSettings.dock,
          media: { ...defaultSettings.dock.media, enabled: true, musicEnabled: true },
        },
      }),
    } as never);
    let fail!: (error: Error) => void;
    mocked.GetMediaPermissions.mockReturnValueOnce(
      new Promise((_resolve, reject) => {
        fail = reject;
      }) as never,
    );
    render(<App />);
    await act(async () => {});
    fireEvent.click(screen.getByRole("tab", { name: "Dock" }));
    const dock = within(screen.getByRole("region", { name: "Dock" }));
    expect(dock.getByRole("button", { name: "Connect" })).toBeDisabled();
    if (newerEvent)
      act(() =>
        eventHandlers.get("media:permission")?.({
          data: { provider: "music", status: "connecting", reason: "" },
        }),
      );
    await act(async () => fail(new Error("permission snapshot unavailable")));
    if (newerEvent) expect(dock.getByRole("button", { name: "Connecting…" })).toBeDisabled();
    else {
      expect(dock.getByText("Media unavailable")).toBeVisible();
      const connect = dock.getByRole("button", { name: "Connect" });
      expect(connect).toBeEnabled();
      fireEvent.click(connect);
      expect(await dock.findByText("Connected")).toBeVisible();
    }
  });

  it("keeps media Connect pending until the original native request drains", async () => {
    window.location.hash = "#settings";
    const enabled = {
      ...defaultSettings,
      behavior: { ...defaultSettings.behavior, onboarded: true },
      dock: {
        ...defaultSettings.dock,
        media: {
          ...defaultSettings.dock.media,
          enabled: true,
          musicEnabled: true,
          spotifyEnabled: true,
        },
      },
    };
    mocked.GetSettingsState.mockResolvedValueOnce({
      revision: 1,
      json: JSON.stringify(enabled),
    } as never);
    let finish!: (value: { status: string; reason: string }) => void;
    mocked.ConnectMediaProvider.mockReturnValueOnce(
      new Promise((resolve) => {
        finish = resolve;
      }) as never,
    );
    render(<App />);
    await act(async () => {});
    fireEvent.click(screen.getByRole("tab", { name: "Dock" }));
    const dock = within(screen.getByRole("region", { name: "Dock" }));
    fireEvent.click(dock.getAllByRole("button", { name: "Connect" })[0]);
    const pending = dock.getByRole("button", { name: "Connecting…" });
    expect(pending).toBeDisabled();
    fireEvent.click(pending);
    expect(mocked.ConnectMediaProvider).toHaveBeenCalledTimes(1);
    expect(dock.getByRole("button", { name: "Connect" })).toBeEnabled();
    fireEvent.click(dock.getByLabelText("Enable media controls"));
    await act(async () => {});
    fireEvent.click(dock.getByLabelText("Enable media controls"));
    await act(async () => {});
    expect(dock.getByRole("button", { name: "Connecting…" })).toBeDisabled();
    // Backend cancellation drains without accepting the obsolete ready result.
    act(() =>
      eventHandlers.get("media:permission")?.({
        data: { provider: "music", status: "permissionRequired", reason: "" },
      }),
    );
    expect(dock.getByRole("button", { name: "Connecting…" })).toBeDisabled();
    await act(async () => finish({ status: "ready", reason: "" }));
    expect(dock.queryByText("Connected")).not.toBeInTheDocument();
    expect(dock.getAllByRole("button", { name: "Connect" })[0]).toBeEnabled();
  });

  it("recovers the pending provider when preferences reopen and ignores a retired response", async () => {
    window.location.hash = "#settings";
    mocked.GetSettingsState.mockResolvedValue({
      revision: 1,
      json: JSON.stringify({
        ...defaultSettings,
        behavior: { ...defaultSettings.behavior, onboarded: true },
        dock: {
          ...defaultSettings.dock,
          media: { ...defaultSettings.dock.media, enabled: true, musicEnabled: true },
        },
      }),
    } as never);
    let finish!: (value: { status: string; reason: string }) => void;
    mocked.ConnectMediaProvider.mockReturnValueOnce(
      new Promise((resolve) => {
        finish = resolve;
      }) as never,
    );
    const initial = render(<App />);
    await act(async () => {});
    fireEvent.click(screen.getByRole("tab", { name: "Dock" }));
    fireEvent.click(screen.getByRole("button", { name: "Connect" }));
    initial.unmount();
    mocked.GetMediaPermissions.mockResolvedValueOnce({
      music: { status: "connecting", reason: "" },
      spotify: { status: "permissionRequired", reason: "" },
    } as never);
    render(<App />);
    await act(async () => {});
    fireEvent.click(screen.getByRole("tab", { name: "Dock" }));
    expect(screen.getByRole("button", { name: "Connecting…" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Connecting…" }));
    expect(mocked.ConnectMediaProvider).toHaveBeenCalledTimes(1);
    act(() =>
      eventHandlers.get("media:permission")?.({
        data: { provider: "music", status: "denied", reason: "Automation access is denied" },
      }),
    );
    await act(async () => finish({ status: "ready", reason: "" }));
    expect(screen.getByText("Automation access is denied")).toBeVisible();
    expect(screen.queryByText("Connected")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Connect" })).toBeEnabled();
  });

  it("keeps Connect disabled until the pending permission snapshot is known", async () => {
    window.location.hash = "#settings";
    mocked.GetSettingsState.mockResolvedValueOnce({
      revision: 1,
      json: JSON.stringify({
        ...defaultSettings,
        behavior: { ...defaultSettings.behavior, onboarded: true },
        dock: {
          ...defaultSettings.dock,
          media: { ...defaultSettings.dock.media, enabled: true, musicEnabled: true },
        },
      }),
    } as never);
    let finish!: (value: Record<string, { status: string; reason: string }>) => void;
    mocked.GetMediaPermissions.mockReturnValueOnce(
      new Promise((resolve) => {
        finish = resolve;
      }) as never,
    );
    render(<App />);
    await act(async () => {});
    fireEvent.click(screen.getByRole("tab", { name: "Dock" }));
    expect(screen.getByRole("button", { name: "Connect" })).toBeDisabled();
    act(() =>
      eventHandlers.get("media:permission")?.({
        data: { provider: "music", status: "connecting", reason: "" },
      }),
    );
    await act(async () =>
      finish({
        music: { status: "permissionRequired", reason: "" },
        spotify: { status: "permissionRequired", reason: "" },
      }),
    );
    expect(screen.getByRole("button", { name: "Connecting…" })).toBeDisabled();
    expect(mocked.ConnectMediaProvider).not.toHaveBeenCalled();
  });

  it.each([
    ["pt-BR", "media provider command is busy", "Outro comando de mídia está em andamento"],
    ["es", "context deadline exceeded", "Se ha agotado el tiempo de espera de la operación"],
  ])("localizes a rejected native media Connect error in %s", async (language, message, expected) => {
    window.location.hash = "#settings";
    mocked.GetSettingsState.mockResolvedValueOnce({
      revision: 1,
      json: JSON.stringify({
        ...defaultSettings,
        behavior: { ...defaultSettings.behavior, onboarded: true, language },
        dock: {
          ...defaultSettings.dock,
          media: { ...defaultSettings.dock.media, enabled: true, musicEnabled: true },
        },
      }),
    } as never);
    const nativeError = new Error(message);
    nativeError.name = "RuntimeError";
    mocked.ConnectMediaProvider.mockRejectedValueOnce(nativeError);
    render(<App />);
    await act(async () => {});
    fireEvent.click(screen.getByRole("tab", { name: "Dock" }));
    fireEvent.click(screen.getByRole("button", { name: "Conectar" }));
    await waitFor(() => expect(mocked.ConnectMediaProvider).toHaveBeenCalledWith("music"));
    expect(await screen.findByText(expected)).toBeVisible();
    expect(screen.queryByText(`RuntimeError: ${message}`)).not.toBeInTheDocument();
  });

  it("admits scoped automation preview updates, frames, controls, and terminal hide", async () => {
    const preview = {
      open: true,
      session: 55,
      revision: 2,
      title: "Automation preview",
      entries: [{ ...appEntry(102, "Document"), thumbnail: "" }],
      selectedWindowId: 102,
      appearance: { ...emptyState.appearance, showWindowControls: true },
      cardSpacingPx: 7,
      emptyReason: "",
      error: "",
      frames: { "102": "data:image/png;base64,snapshot", "999": "foreign" },
      frameSequence: 1,
    };
    mocked.GetAutomationPreviewState.mockResolvedValueOnce(preview as never);
    window.location.hash = "#automation/55";
    render(<App />);
    expect(await screen.findByText("Automation preview")).toBeInTheDocument();
    expect(document.querySelector(".ot-dock-native-titlebar")).not.toBeNull();
    expect(screen.getByRole("button", { name: "Close preview" })).toHaveClass(
      "ot-dock-native-close",
    );
    expect(document.querySelector('img[src="data:image/png;base64,snapshot"]')).not.toBeNull();
    expect(document.querySelector('img[src="foreign"]')).toBeNull();
    act(() =>
      eventHandlers.get("automation-preview:frames")?.({
        data: {
          session: 55,
          revision: 2,
          sequence: 2,
          frames: {
            "102": "data:image/png;base64,current",
            "999": "data:image/png;base64,foreign",
          },
        },
      }),
    );
    expect(document.querySelector('img[src="data:image/png;base64,current"]')).not.toBeNull();
    expect(document.querySelector('img[src="data:image/png;base64,foreign"]')).toBeNull();
    act(() =>
      eventHandlers.get("automation-preview:update")?.({
        data: { ...preview, revision: 3, title: "Current revision" },
      }),
    );
    expect(document.querySelector('img[src="data:image/png;base64,current"]')).toBeNull();
    act(() =>
      eventHandlers.get("automation-preview:frames")?.({
        data: { session: 55, revision: 2, sequence: 99, frames: { "102": "old" } },
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Focus Document" }));
    fireEvent.click(screen.getByLabelText("Fullscreen window"));
    expect(mocked.PerformAutomationPreviewAction).toHaveBeenCalledWith(55, 3, "focus", 102, false);
    expect(mocked.PerformAutomationPreviewAction).toHaveBeenCalledWith(
      55,
      3,
      "fullscreen",
      102,
      true,
    );
    expect(screen.queryByText("New window")).toBeNull();
    act(() =>
      eventHandlers.get("automation-preview:hide")?.({ data: { session: 55, revision: 4 } }),
    );
    act(() =>
      eventHandlers.get("automation-preview:update")?.({
        data: { ...preview, revision: 5, title: "Late resurrection" },
      }),
    );
    expect(screen.queryByText("Late resurrection")).toBeNull();
  });

  it("shows only current-revision automation action failures", async () => {
    const preview = {
      open: true,
      session: 56,
      revision: 1,
      title: "Failure preview",
      entries: [appEntry(103, "Failure document")],
      selectedWindowId: 103,
      appearance: { ...emptyState.appearance, showWindowControls: true },
      cardSpacingPx: 7,
      emptyReason: "",
      error: "",
    };
    mocked.GetAutomationPreviewState.mockResolvedValueOnce(preview as never);
    mocked.PerformAutomationPreviewAction.mockRejectedValueOnce(new Error("Exact action refused"));
    window.location.hash = "#automation/56";
    render(<App />);
    fireEvent.click(await screen.findByLabelText("Close window"));
    expect(await screen.findByRole("alert")).toHaveTextContent("Exact action refused");
  });

  it("resets hover progress admission when embedded media scope changes", () => {
    window.location.hash = "#dock";
    render(<App />);
    act(() =>
      eventHandlers.get("dock:show")?.({ data: dockMediaState(1, dockMedia(70, 2, "First")) }),
    );
    act(() =>
      eventHandlers.get("media:progress")?.({
        data: { session: 70, revision: 2, sequence: 20, positionMS: 800, activeCue: -1 },
      }),
    );
    act(() =>
      eventHandlers.get("dock:update")?.({
        data: dockMediaState(2, dockMedia(70, 3, "Replacement")),
      }),
    );
    act(() =>
      eventHandlers.get("media:progress")?.({
        data: { session: 70, revision: 3, sequence: 1, positionMS: 500, activeCue: -1 },
      }),
    );
    expect(screen.getByRole("slider", { name: "Playback position" })).toHaveValue("500");
  });

  it("tombstones hidden hover media against delayed Dock updates", () => {
    window.location.hash = "#dock";
    render(<App />);
    const retired = dockMedia(71, 4, "Retired");
    act(() => eventHandlers.get("dock:show")?.({ data: dockMediaState(1, retired) }));
    act(() => eventHandlers.get("media:hide")?.({ data: { session: 71, revision: 5 } }));
    act(() => eventHandlers.get("dock:update")?.({ data: dockMediaState(2, retired) }));
    expect(screen.queryByText("Retired")).toBeNull();
    act(() =>
      eventHandlers.get("dock:update")?.({ data: dockMediaState(3, dockMedia(72, 1, "New")) }),
    );
    expect(screen.getByText("New")).toBeInTheDocument();
  });

  it("switches Dock content only from fresh outer events and ignores reordered media events", async () => {
    window.location.hash = "#dock";
    render(<App />);
    const oldMedia = dockMedia(73, 2, "Media track");
    act(() =>
      eventHandlers.get("dock:show")?.({
        data: { ...dockMediaState(8, oldMedia), contentOptions: ["windows", "media"] },
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Windows" }));
    expect(mocked.SelectDockContent).toHaveBeenCalledWith(40, 8, "windows");
    expect(screen.getByText("Media track")).toBeInTheDocument();

    const windowState = {
      ...dockMediaState(1, oldMedia),
      session: 41,
      revision: 1,
      contentKind: "windows",
      contentOptions: ["windows", "media"],
      media: undefined,
      entries: [appEntry(104, "Provider window")],
      selectedWindowId: 104,
    };
    act(() => eventHandlers.get("dock:show")?.({ data: windowState }));
    act(() => eventHandlers.get("media:hide")?.({ data: { session: 73, revision: 3 } }));
    act(() =>
      eventHandlers.get("media:update")?.({
        data: {
          ...oldMedia,
          revision: 4,
          sample: { ...oldMedia.sample, track: { ...oldMedia.sample.track, title: "Late media" } },
        },
      }),
    );
    act(() =>
      eventHandlers.get("dock:frames")?.({
        data: { session: 40, frames: { "104": "data:image/png;base64,stale" } },
      }),
    );
    expect(screen.getByText("Provider window")).toBeInTheDocument();
    expect(screen.queryByText("Late media")).toBeNull();
    expect(document.querySelector('img[src="data:image/png;base64,stale"]')).toBeNull();
  });

  it("does not render a selector refusal after a replacement Dock session", async () => {
    let reject!: (error: Error) => void;
    mocked.SelectDockContent.mockReturnValueOnce(
      new Promise<void>((_, fail) => {
        reject = fail;
      }) as never,
    );
    window.location.hash = "#dock";
    render(<App />);
    const mediaState = {
      ...dockMediaState(8, dockMedia(74, 2, "Original")),
      contentOptions: ["windows", "media"],
    };
    act(() => eventHandlers.get("dock:show")?.({ data: mediaState }));
    fireEvent.click(screen.getByRole("button", { name: "Windows" }));
    act(() =>
      eventHandlers.get("dock:show")?.({
        data: {
          ...mediaState,
          session: 41,
          revision: 1,
          contentKind: "windows",
          media: undefined,
          entries: [appEntry(105, "Replacement window")],
          selectedWindowId: 105,
        },
      }),
    );
    await act(async () => reject(new Error("Late selector refusal")));
    expect(screen.getByText("Replacement window")).toBeInTheDocument();
    expect(screen.queryByText("Late selector refusal")).toBeNull();
  });

  it("admits only current pinned media revisions and progress sequences", async () => {
    const mediaState = {
      session: 44,
      revision: 2,
      open: true,
      pinned: true,
      pinnable: false,
      provider: "music",
      scope: {
        provider: "music",
        process: { pid: 3, launchID: "x" },
        generation: 1,
        trackEpoch: 1,
        trackID: "A",
      },
      sample: {
        provider: "music",
        process: { pid: 3, launchID: "x" },
        generation: 1,
        sequence: 1,
        trackEpoch: 1,
        track: {
          id: "A",
          title: "Current track",
          artist: "Artist",
          album: "Album",
          durationMS: 1000,
        },
        playback: "paused",
        positionMS: 10,
        observedAt: "",
        status: "ready",
        reason: "",
        capabilities: { play: true, pause: true, previous: true, next: true, seek: true },
        artworkToken: "",
      },
      appearance: emptyState.appearance,
      artwork: { status: "missing", reason: "", image: "" },
      lyrics: {
        documentID: "",
        status: "missing",
        reason: "No synchronized lyrics",
        cues: [],
        offsetMS: 0,
      },
      positionMS: 10,
      activeCue: -1,
      error: "",
    };
    (mocked as any).GetMediaState.mockResolvedValueOnce(mediaState);
    window.location.hash = "#media/44";
    render(<App />);
    expect(await screen.findByText("Current track")).toBeInTheDocument();
    act(() =>
      eventHandlers.get("media:progress")?.({
        data: { session: 44, revision: 2, sequence: 3, positionMS: 800, activeCue: -1 },
      }),
    );
    act(() =>
      eventHandlers.get("media:progress")?.({
        data: { session: 44, revision: 2, sequence: 2, positionMS: 100, activeCue: -1 },
      }),
    );
    act(() => eventHandlers.get("media:hide")?.({ data: { session: 44, revision: 3 } }));
    act(() =>
      eventHandlers.get("media:update")?.({
        data: {
          ...mediaState,
          revision: 4,
          sample: {
            ...mediaState.sample,
            track: { ...mediaState.sample.track, title: "Late track" },
          },
        },
      }),
    );
    expect(screen.queryByText("Late track")).toBeNull();
  });
  it("renders the overlay route (closed) by default", () => {
    const { container } = render(<App />);
    // Overlay renders nothing while closed.
    expect(container.firstChild).toBeNull();
  });

  it("renders the settings route at #settings", () => {
    window.location.hash = "#settings";
    render(<App />);
    expect(screen.getByText(/Preferences/)).toBeInTheDocument();
  });

  it("renders app mode and confirms an explicit app/window target", async () => {
    render(<App />);
    act(() =>
      eventHandlers.get("switcher:show")?.({
        data: openSwitcherState({
          mode: "apps",
          apps: [
            {
              appId: 10,
              appName: "Editor",
              bundleId: "a",
              hidden: false,
              windowCount: 1,
              windowPresence: "present",
            },
          ],
          entries: [{ ...appEntry(101, "Document"), appId: 10, bundleId: "a" }],
          selectedWindowId: 101,
        }),
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Focus Document" }));
    await waitFor(() => expect((AppService as any).SelectAppWindow).toHaveBeenCalledWith(101));
    expect(mocked.ConfirmWindow).toHaveBeenCalledWith(101);
  });

  it("admits only monotonic switcher state, current frames, and rejects closed-session resurrection", () => {
    render(<App />);
    const state = (session: number, revision: number, title: string) =>
      openSwitcherState({
        session,
        revision,
        entries: [appEntry(101, title)],
      });
    act(() => {
      eventHandlers.get("switcher:update")?.({ data: state(7, 3, "Newest") });
      eventHandlers.get("switcher:show")?.({ data: state(7, 2, "Old show") });
      eventHandlers.get("switcher:update")?.({ data: state(6, 20, "Old session") });
      eventHandlers.get("switcher:thumbnails")?.({
        data: { session: 6, frames: { "101": "data:old" } },
      });
      eventHandlers.get("switcher:thumbnails")?.({
        data: { session: 7, frames: { "101": "data:current" } },
      });
    });
    expect(screen.getByText("Newest")).toBeInTheDocument();
    expect(document.querySelector(".ot-thumb-img")).toHaveAttribute("src", "data:current");
    act(() => {
      eventHandlers.get("switcher:hide")?.({ data: { session: 6, revision: 21 } });
    });
    expect(screen.getByText("Newest")).toBeInTheDocument();
    act(() => {
      eventHandlers.get("switcher:hide")?.({ data: { session: 7, revision: 4 } });
      eventHandlers.get("switcher:update")?.({ data: state(7, 5, "Late update") });
      eventHandlers.get("switcher:show")?.({ data: state(7, 6, "Late show") });
      eventHandlers.get("switcher:preview")?.({
        data: { session: 7, frames: { "101": "data:late" } },
      });
    });
    expect(screen.queryByText(/Late/)).toBeNull();
    expect(screen.getByRole("dialog")).toHaveClass("ot-closing");
  });

  it("rejects unscoped legacy frames and hides after a real switcher session", () => {
    render(<App />);
    act(() =>
      eventHandlers.get("switcher:show")?.({
        data: openSwitcherState({ session: 8, revision: 1, entries: [appEntry(102, "Scoped")] }),
      }),
    );
    act(() => {
      eventHandlers.get("switcher:thumbnails")?.({ data: { "102": "data:legacy" } });
      eventHandlers.get("switcher:hide")?.({ data: null });
    });
    expect(screen.getByText("Scoped")).toBeInTheDocument();
    expect(screen.queryByRole("img")).toBeNull();
  });

  it("treats a real hide as scoped before any show arrives", () => {
    render(<App />);
    act(() => {
      eventHandlers.get("switcher:hide")?.({ data: { session: 11, revision: 2 } });
      eventHandlers.get("switcher:show")?.({
        data: openSwitcherState({ entries: [appEntry(1, "Legacy resurrection")] }),
      });
      eventHandlers.get("switcher:thumbnails")?.({ data: { "1": "data:legacy" } });
    });
    expect(screen.queryByText("Legacy resurrection")).toBeNull();
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("does not let an old native key control a newer app presentation", () => {
    mocked.Advance.mockClear();
    render(<App />);
    act(() =>
      eventHandlers.get("switcher:show")?.({
        data: openSwitcherState({
          session: 15,
          revision: 1,
          mode: "apps",
          apps: [{ appId: 10, appName: "App", bundleId: "a", hidden: false, windowCount: 0 }],
          entries: [],
          selectedWindowId: 0,
        }),
      }),
    );
    act(() => {
      eventHandlers.get("switcher:key")?.({
        data: {
          session: 14,
          key: "Tab",
          code: "Tab",
          shift: false,
          ctrl: false,
          alt: false,
          meta: false,
        },
      });
      eventHandlers.get("switcher:key")?.({
        data: {
          session: 15,
          key: "Tab",
          code: "Tab",
          shift: false,
          ctrl: false,
          alt: false,
          meta: false,
        },
      });
    });
    expect(mocked.Advance).toHaveBeenCalledTimes(1);
  });

  it("rejects stale Dock snapshots, frames, hides, and errors", async () => {
    let resolveSnapshot: (value: never) => void = () => {};
    (AppService as any).GetDockState.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveSnapshot = resolve;
      }) as never,
    );
    window.location.hash = "#dock";
    const appearance = { ...emptyState.appearance, showWindowControls: true };
    render(<App />);
    const dockState = (session: number, title: string, revision = 0) => ({
      session,
      revision,
      item: {
        appId: 10,
        bundleId: "a",
        path: "/A.app",
        title: "A",
        bounds: { x: 0, y: 0, w: 40, h: 40 },
        screenId: 1,
        edge: "bottom",
        kind: "app",
      },
      entries: [{ ...appEntry(102, title), appId: 10 }],
      selectedWindowId: 102,
      appearance,
      emptyReason: "",
    });
    act(() => eventHandlers.get("dock:show")?.({ data: dockState(5, "Current") }));
    expect(screen.getByRole("button", { name: "Focus Current" })).toBeInTheDocument();
    await act(async () => resolveSnapshot(dockState(3, "Old") as never));
    act(() => {
      eventHandlers.get("dock:frames")?.({ data: { session: 3, frames: { "102": "old-frame" } } });
      eventHandlers.get("dock:error")?.({ data: { session: 3, message: "old error" } });
      eventHandlers.get("dock:hide")?.({ data: { session: 3 } });
    });
    expect(screen.getByRole("button", { name: "Focus Current" })).toBeInTheDocument();
    expect(screen.queryByText("old error")).toBeNull();
    act(() => {
      eventHandlers.get("dock:hide")?.({ data: { session: 5 } });
      eventHandlers.get("dock:update")?.({ data: dockState(5, "Retired") });
    });
    expect(screen.queryByRole("button", { name: "Focus Retired" })).toBeNull();
    act(() => {
      eventHandlers.get("dock:show")?.({ data: dockState(5, "Late show", 4) });
      eventHandlers.get("dock:update")?.({ data: dockState(5, "Late update", 4) });
    });
    expect(screen.queryByText(/Late/)).toBeNull();
  });

  it("rejects out-of-order revisions within the active Dock session", () => {
    window.location.hash = "#dock";
    render(<App />);
    const base = {
      session: 8,
      item: {
        appId: 10,
        bundleId: "a",
        path: "/A.app",
        title: "New",
        bounds: { x: 0, y: 0, w: 40, h: 40 },
        screenId: 1,
        edge: "bottom",
        kind: "app",
      },
      entries: [],
      selectedWindowId: 0,
      appearance: emptyState.appearance,
      emptyReason: "unavailable",
    };
    act(() => {
      eventHandlers.get("dock:show")?.({ data: { ...base, revision: 2 } });
      eventHandlers.get("dock:update")?.({
        data: { ...base, revision: 1, item: { ...base.item, title: "Old" } },
      });
    });
    expect(screen.getByText("New")).toBeInTheDocument();
    expect(screen.queryByText("Old")).toBeNull();
  });

  it("renders folder content with outer scope and no window-region traffic", async () => {
    mocked.SetDockPreviewRegions.mockClear();
    window.location.hash = "#dock";
    render(<App />);
    act(() =>
      eventHandlers.get("dock:show")?.({
        data: {
          session: 21,
          revision: 8,
          open: true,
          contentKind: "folder",
          item: {
            appId: 0,
            bundleId: "",
            path: "/tmp/Folder",
            title: "Folder",
            bounds: { x: 0, y: 0, w: 48, h: 48 },
            screenId: 1,
            edge: "bottom",
            kind: "folder",
          },
          entries: [],
          selectedWindowId: 0,
          appearance: emptyState.appearance,
          emptyReason: "",
          folder: {
            status: "ready",
            reason: "",
            folderIdentity: "file:///tmp/Folder",
            entries: [
              {
                id: "opaque-7",
                name: "Notes.txt",
                kind: "file",
                size: 12,
                modifiedAtMs: 1,
                hidden: false,
              },
            ],
            sort: { field: "name", direction: "asc", foldersFirst: true },
            partial: false,
            revision: 99,
          },
        },
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Open Notes.txt" }));
    fireEvent.change(screen.getByLabelText("Sort folder contents by"), {
      target: { value: "size" },
    });
    await waitFor(() =>
      expect((AppService as any).OpenDockFolderEntry).toHaveBeenCalledWith(21, 8, "opaque-7"),
    );
    expect((AppService as any).SetDockFolderSort).toHaveBeenCalledWith(21, 8, "size", "asc", true);
    expect(mocked.SetDockPreviewRegions).not.toHaveBeenCalled();
    expect(screen.queryByLabelText("Close window")).toBeNull();
  });

  it("uses an admitted Dock error revision for the next folder retry", async () => {
    window.location.hash = "#dock";
    render(<App />);
    act(() =>
      eventHandlers.get("dock:show")?.({
        data: {
          session: 22,
          revision: 1,
          open: true,
          contentKind: "folder",
          item: {
            appId: 0,
            bundleId: "",
            path: "/tmp/Folder",
            title: "Folder",
            bounds: { x: 0, y: 0, w: 48, h: 48 },
            screenId: 1,
            edge: "bottom",
            kind: "folder",
          },
          entries: [],
          selectedWindowId: 0,
          appearance: emptyState.appearance,
          emptyReason: "",
          folder: {
            status: "unavailable",
            reason: "",
            folderIdentity: "file:///tmp/Folder",
            entries: [],
            sort: { field: "name", direction: "asc", foldersFirst: true },
            partial: false,
            revision: 1,
          },
        },
      }),
    );
    act(() =>
      eventHandlers.get("dock:error")?.({
        data: { session: 22, revision: 2, message: "Temporary failure" },
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() =>
      expect((AppService as any).SetDockFolderSort).toHaveBeenCalledWith(
        22,
        2,
        "name",
        "asc",
        true,
      ),
    );
  });

  it("admits only current-session monotonic Dock pointer packets and clears outside", () => {
    window.location.hash = "#dock";
    render(<App />);
    const base = {
      session: 18,
      revision: 1,
      item: {
        appId: 10,
        bundleId: "a",
        path: "/A.app",
        title: "A",
        bounds: { x: 0, y: 0, w: 40, h: 40 },
        screenId: 1,
        edge: "bottom",
        kind: "app",
      },
      entries: [
        { ...appEntry(102, "First"), appId: 10 },
        { ...appEntry(103, "Second"), appId: 10 },
      ],
      selectedWindowId: 102,
      appearance: emptyState.appearance,
      emptyReason: "",
    };
    let hit: Element | null = null;
    Object.defineProperty(document, "elementFromPoint", {
      configurable: true,
      value: () => hit ?? document.querySelector("article[data-window-id='103']"),
    });
    act(() => {
      eventHandlers.get("dock:pointer")?.({
        data: { session: 17, sequence: 99, x: 1, y: 1, inside: true },
      });
      eventHandlers.get("dock:pointer")?.({
        data: { session: 18, sequence: 2, x: 10, y: 10, inside: true },
      });
    });
    act(() => eventHandlers.get("dock:show")?.({ data: base }));
    const first = document.querySelector("article[data-window-id='102']")!;
    const second = document.querySelector("article[data-window-id='103']")!;
    expect(second).toHaveClass("is-hovered");
    expect(AppService.SelectDockWindow).toHaveBeenCalledWith(18, 103);
    hit = first;
    act(() => {
      eventHandlers.get("dock:pointer")?.({
        data: { session: 18, sequence: 1, x: 1, y: 1, inside: true },
      });
      eventHandlers.get("dock:pointer")?.({
        data: { session: 17, sequence: 3, x: 1, y: 1, inside: true },
      });
      eventHandlers.get("dock:update")?.({
        data: {
          ...base,
          revision: 2,
          pointer: { session: 18, sequence: 1, x: 1, y: 1, inside: true },
        },
      });
    });
    expect(second).toHaveClass("is-hovered");
    expect(first).not.toHaveClass("is-hovered");
    hit = null;
    act(() =>
      eventHandlers.get("dock:pointer")?.({
        data: { session: 18, sequence: 3, x: -1, y: -1, inside: false },
      }),
    );
    expect(second).not.toHaveClass("is-hovered");
    expect(AppService.FocusDockWindow).not.toHaveBeenCalled();
    expect(AppService.PerformDockAction).not.toHaveBeenCalled();
    Reflect.deleteProperty(document, "elementFromPoint");
  });

  it("publishes visible Dock preview regions through the current session bridge", async () => {
    window.location.hash = "#dock";
    const rect = {
      x: 10,
      y: 10,
      left: 10,
      top: 10,
      right: 110,
      bottom: 70,
      width: 100,
      height: 60,
      toJSON: () => ({}),
    } as DOMRect;
    const bounds = vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue(rect);
    (AppService as any).SetDockPreviewRegions.mockClear();
    render(<App />);
    act(() =>
      eventHandlers.get("dock:show")?.({
        data: {
          session: 21,
          revision: 1,
          open: true,
          item: {
            appId: 10,
            bundleId: "a",
            path: "/A.app",
            title: "A",
            bounds: { x: 0, y: 0, w: 40, h: 40 },
            screenId: 1,
            edge: "bottom",
            kind: "app",
          },
          entries: [{ ...appEntry(101, "Region target"), appId: 10 }],
          selectedWindowId: 101,
          appearance: emptyState.appearance,
          emptyReason: "",
        },
      }),
    );
    await waitFor(() =>
      expect((AppService as any).SetDockPreviewRegions).toHaveBeenCalledWith(21, 1, [
        expect.objectContaining({
          windowId: 101,
          appId: 10,
          bounds: { x: 10, y: 10, w: 100, h: 60 },
        }),
      ]),
    );
    bounds.mockRestore();
  });

  it("uses a Dock error revision as a state high-water mark", () => {
    window.location.hash = "#dock";
    render(<App />);
    const base = {
      session: 9,
      revision: 2,
      item: {
        appId: 10,
        bundleId: "a",
        path: "/A.app",
        title: "Current",
        bounds: { x: 0, y: 0, w: 40, h: 40 },
        screenId: 1,
        edge: "bottom",
        kind: "app",
      },
      entries: [],
      selectedWindowId: 0,
      appearance: emptyState.appearance,
      emptyReason: "unavailable",
    };
    act(() => {
      eventHandlers.get("dock:show")?.({ data: base });
      eventHandlers.get("dock:error")?.({
        data: { session: 9, revision: 4, message: "Current refusal" },
      });
      eventHandlers.get("dock:update")?.({
        data: { ...base, revision: 3, item: { ...base.item, title: "Stale" } },
      });
    });
    expect(screen.getByRole("alert")).toHaveTextContent("Current refusal");
    expect(screen.queryByText("Stale")).toBeNull();
  });

  it("uses a hidden catch-up snapshot as a same-session tombstone", async () => {
    let resolveSnapshot: (value: never) => void = () => {};
    (AppService as any).GetDockState.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveSnapshot = resolve;
      }),
    );
    window.location.hash = "#dock";
    render(<App />);
    const state = {
      session: 12,
      revision: 2,
      open: true,
      item: {
        appId: 10,
        bundleId: "a",
        path: "/A.app",
        title: "Shown",
        bounds: { x: 0, y: 0, w: 40, h: 40 },
        screenId: 1,
        edge: "bottom",
        kind: "app",
      },
      entries: [],
      selectedWindowId: 0,
      appearance: emptyState.appearance,
      emptyReason: "unavailable",
    };
    act(() => eventHandlers.get("dock:show")?.({ data: state }));
    await act(async () => resolveSnapshot({ ...state, revision: 3, open: false } as never));
    act(() => eventHandlers.get("dock:show")?.({ data: state }));
    expect(screen.queryByText("Shown")).toBeNull();
  });

  it("acts on the clicked window without changing selection", async () => {
    render(<App />);
    act(() => {
      eventHandlers.get("switcher:show")?.({
        data: openSwitcherState({
          entries: [appEntry(11, "Editor"), appEntry(22, "Browser"), appEntry(33, "Terminal")],
        }),
      });
    });
    fireEvent.click(screen.getAllByLabelText("Close window")[2]);
    act(() => {
      eventHandlers.get("switcher:update")?.({
        data: openSwitcherState({
          selected: 1,
          entries: [appEntry(11, "Editor"), appEntry(22, "Browser"), appEntry(33, "Terminal")],
        }),
      });
    });
    await waitFor(() => expect(mocked.PerformAction).toHaveBeenCalledWith("close", 33, 33));
    expect(mocked.Select).not.toHaveBeenCalled();
    expect(mocked.CloseSelected).not.toHaveBeenCalled();
  });

  it("shows a failed native action instead of silently discarding it", async () => {
    mocked.PerformAction.mockRejectedValueOnce(new Error("Window no longer exists"));
    render(<App />);
    act(() => {
      eventHandlers.get("switcher:show")?.({
        data: openSwitcherState({
          entries: [appEntry(11, "Editor")],
        }),
      });
    });
    fireEvent.click(screen.getByLabelText("Close window"));
    expect(await screen.findByRole("alert")).toHaveTextContent("Window no longer exists");
  });

  it("shows partial bulk-action failures", async () => {
    mocked.PerformAction.mockResolvedValueOnce({
      succeeded: 2,
      failures: [{ windowId: 11, error: "Window refused to close" }],
    } as never);
    render(<App />);
    act(() => {
      eventHandlers.get("switcher:show")?.({
        data: openSwitcherState({ entries: [appEntry(11, "Editor")] }),
      });
    });
    fireEvent.click(screen.getByLabelText("Close window"));
    expect(await screen.findByRole("alert")).toHaveTextContent("Window refused to close");
    expect(screen.getByRole("alert")).toHaveTextContent("2 action requests accepted");
  });

  it("does not show a late action failure in a reopened switcher", async () => {
    let rejectAction: (error: Error) => void = () => {};
    mocked.PerformAction.mockReturnValueOnce(
      new Promise((_, reject) => {
        rejectAction = reject;
      }) as never,
    );
    render(<App />);
    const state = openSwitcherState({ entries: [appEntry(11, "Editor")] });
    act(() => {
      eventHandlers.get("switcher:show")?.({ data: state });
    });
    fireEvent.click(screen.getByLabelText("Close window"));
    act(() => {
      eventHandlers.get("switcher:hide")?.({ data: null });
      eventHandlers.get("switcher:show")?.({ data: state });
    });
    await act(async () => {
      rejectAction(new Error("Old session failure"));
    });
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("confirms a clicked entry atomically by window id", async () => {
    render(<App />);
    await waitFor(() => expect(mocked.GetVersion).toHaveBeenCalled());

    act(() => {
      eventHandlers.get("switcher:show")?.({
        data: openSwitcherState({
          mouseHover: false,
          entries: [appEntry(11, "Editor"), appEntry(22, "Browser")],
        }),
      });
    });

    fireEvent.click(screen.getByText("Browser").closest('[role="option"]') as HTMLElement);
    expect(mocked.ConfirmWindow).toHaveBeenCalledWith(22);
    expect(mocked.Confirm).not.toHaveBeenCalled();
  });

  it("merges streamed thumbnails and previews by windowId and resets them on show", () => {
    const { container } = render(<App />);

    const previewState = () =>
      openSwitcherState({
        entries: [appEntry(1, "Editor")],
        appearance: { ...emptyState.appearance, previewSelected: true },
      });

    act(() => {
      eventHandlers.get("switcher:show")?.({ data: previewState() });
    });
    act(() => {
      eventHandlers.get("switcher:thumbnails")?.({ data: { "1": "data:thumb" } });
    });
    act(() => {
      eventHandlers.get("switcher:preview")?.({ data: { "1": "data:prev" } });
    });

    // The high-resolution preview wins for the selected-window preview...
    const previewImg = screen
      .getByLabelText("Selected window preview")
      .querySelector("img") as HTMLImageElement;
    expect(previewImg.src).toContain("data:prev");
    // ...while the entry cell keeps the merged thumbnail.
    const thumbImg = container.querySelector(".ot-thumb-img") as HTMLImageElement;
    expect(thumbImg).not.toBeNull();
    expect(thumbImg.src).toContain("data:thumb");

    // A new session drops the previous captures.
    act(() => {
      eventHandlers.get("switcher:show")?.({ data: previewState() });
    });
    expect(screen.queryByLabelText("Selected window preview")).toBeNull();
    expect(container.querySelector(".ot-thumb-img")).toBeNull();
  });

  it("renders the demo route with a style override and thumbnails fallback", () => {
    window.location.hash = "#demo:appIcons";
    const first = render(<App />);
    expect(first.container.querySelector(".ot-demo-backdrop")).not.toBeNull();
    expect(first.container.querySelector('[data-style="appIcons"]')).not.toBeNull();
    first.unmount();

    window.location.hash = "#demo";
    const second = render(<App />);
    expect(second.container.querySelector(".ot-demo-backdrop")).not.toBeNull();
    expect(second.container.querySelector('[data-style="thumbnails"]')).not.toBeNull();
  });

  it("deep-links a preferences tab via prefs:tab in the settings window", () => {
    window.location.hash = "#settings";
    render(<App />);

    const tabs = screen.getByRole("tablist");
    expect(within(tabs).getByRole("tab", { name: "General" })).toHaveAttribute(
      "aria-selected",
      "true",
    );

    act(() => {
      eventHandlers.get("prefs:tab")?.({ data: "About" });
    });
    expect(within(tabs).getByRole("tab", { name: "About" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("disables retained preferences while loading and admits a monotonic canonical event", async () => {
    window.location.hash = "#settings";
    render(<App />);
    fireEvent.click(await screen.findByRole("tab", { name: "Dock" }));
    const name = screen.getByLabelText("Profile name");
    act(() => eventHandlers.get("prefs:settings-loading")?.({ data: { generation: 1 } }));
    expect(name).toBeDisabled();
    const refreshed = {
      ...defaultSettings,
      behavior: { ...defaultSettings.behavior, onboarded: true },
      replacementDock: {
        ...defaultSettings.replacementDock,
        profiles: [{ ...defaultSettings.replacementDock.profiles[0], name: "Runtime pins" }],
      },
    };
    act(() =>
      eventHandlers.get("prefs:settings")?.({
        data: { generation: 1, revision: 4, json: JSON.stringify(refreshed) },
      }),
    );
    expect(name).toBeEnabled();
    expect(name).toHaveValue("Runtime pins");
    act(() => eventHandlers.get("prefs:settings-loading")?.({ data: { generation: 1 } }));
    expect(name).toBeEnabled();
    fireEvent.change(name, { target: { value: "Newer local edit" } });
    await waitFor(() => expect(name).toHaveValue("Newer local edit"));
    act(() =>
      eventHandlers.get("prefs:settings")?.({
        data: { generation: 1, revision: 3, json: JSON.stringify(defaultSettings) },
      }),
    );
    expect(name).toHaveValue("Newer local edit");
  });

  it("keeps preferences disabled until the initial revisioned snapshot arrives", async () => {
    window.location.hash = "#settings";
    let resolve!: (value: unknown) => void;
    mocked.GetSettingsState.mockReturnValueOnce(new Promise((done) => (resolve = done)) as never);
    render(<App />);
    const startAtLogin = screen.getByLabelText("Start at login");
    expect(startAtLogin).toBeDisabled();
    await act(async () => {
      resolve({
        revision: 3,
        json: JSON.stringify({
          ...defaultSettings,
          behavior: { ...defaultSettings.behavior, onboarded: true },
        }),
      });
    });
    expect(startAtLogin).toBeEnabled();
  });

  it("enables local settings edits after confirming there is no Wails backend", async () => {
    window.location.hash = "#settings";
    mocked.GetSettingsState.mockRejectedValueOnce(new Error("no backend"));
    mocked.GetVersion.mockResolvedValueOnce("<!doctype html>");
    render(<App />);
    const startAtLogin = screen.getByLabelText("Start at login");
    await waitFor(() => expect(startAtLogin).toBeEnabled());
    fireEvent.click(startAtLogin);
    expect(startAtLogin).toBeChecked();
    expect(mocked.SaveSettingsAtRevision).not.toHaveBeenCalled();
  });

  it("retires queued drafts after a CAS conflict and reloads canonical settings", async () => {
    window.location.hash = "#settings";
    const initial = {
      ...defaultSettings,
      behavior: { ...defaultSettings.behavior, onboarded: true },
    };
    const canonical = {
      ...initial,
      replacementDock: {
        ...initial.replacementDock,
        profiles: [{ ...initial.replacementDock.profiles[0], name: "Runtime winner" }],
      },
    };
    mocked.GetSettingsState.mockResolvedValueOnce({
      revision: 8,
      json: JSON.stringify(initial),
    } as never).mockResolvedValueOnce({ revision: 9, json: JSON.stringify(canonical) } as never);
    let reject!: (error: Error) => void;
    mocked.SaveSettingsAtRevision.mockReturnValueOnce(
      new Promise((_, fail) => (reject = fail)) as never,
    );
    render(<App />);
    fireEvent.click(await screen.findByRole("tab", { name: "Dock" }));
    const name = screen.getByLabelText("Profile name");
    fireEvent.change(name, { target: { value: "First stale draft" } });
    fireEvent.blur(name);
    fireEvent.change(name, { target: { value: "Second stale draft" } });
    fireEvent.blur(name);
    await waitFor(() => expect(mocked.SaveSettingsAtRevision).toHaveBeenCalledTimes(1));
    expect(mocked.SaveSettingsAtRevision.mock.calls[0][1]).toBe(8);
    await act(async () => {
      reject(new Error("settings: staleRevision"));
      await Promise.resolve();
    });
    await waitFor(() => expect(name).toHaveValue("Runtime winner"));
    expect(mocked.SaveSettingsAtRevision).toHaveBeenCalledTimes(1);
  });

  it("does not let a delayed conflict reload overwrite a newer preferences event", async () => {
    window.location.hash = "#settings";
    const initial = {
      ...defaultSettings,
      behavior: { ...defaultSettings.behavior, onboarded: true },
    };
    let finishReload!: (value: unknown) => void;
    mocked.GetSettingsState.mockResolvedValueOnce({
      revision: 4,
      json: JSON.stringify(initial),
    } as never).mockReturnValueOnce(new Promise((done) => (finishReload = done)) as never);
    mocked.SaveSettingsAtRevision.mockRejectedValueOnce(new Error("settings: staleRevision"));
    render(<App />);
    fireEvent.click(await screen.findByRole("tab", { name: "Dock" }));
    const loadsBefore = mocked.GetSettingsState.mock.calls.length;
    fireEvent.change(screen.getByLabelText("Profile name"), { target: { value: "Old draft" } });
    fireEvent.blur(screen.getByLabelText("Profile name"));
    await waitFor(() => expect(mocked.GetSettingsState).toHaveBeenCalledTimes(loadsBefore + 1));
    const savesDuringRecovery = mocked.SaveSettingsAtRevision.mock.calls.length;
    expect(screen.getByLabelText("Profile name")).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Profile name"), {
      target: { value: "Must not overwrite" },
    });
    fireEvent.blur(screen.getByLabelText("Profile name"));
    expect(mocked.SaveSettingsAtRevision).toHaveBeenCalledTimes(savesDuringRecovery);
    const winner = {
      ...initial,
      replacementDock: {
        ...initial.replacementDock,
        profiles: [{ ...initial.replacementDock.profiles[0], name: "Event winner" }],
      },
    };
    act(() =>
      eventHandlers.get("prefs:settings")?.({
        data: { generation: 2, revision: 8, json: JSON.stringify(winner) },
      }),
    );
    await act(async () => {
      finishReload({
        revision: 7,
        json: JSON.stringify({
          ...winner,
          replacementDock: {
            ...winner.replacementDock,
            profiles: [{ ...winner.replacementDock.profiles[0], name: "Late reload" }],
          },
        }),
      });
    });
    expect(screen.getByLabelText("Profile name")).toHaveValue("Event winner");
  });

  it("unsubscribes revisioned preferences events on unmount", async () => {
    window.location.hash = "#settings";
    const view = render(<App />);
    await screen.findByRole("tab", { name: "Dock" });
    expect(eventHandlers.has("prefs:settings")).toBe(true);
    view.unmount();
    expect(eventHandlers.has("prefs:settings")).toBe(false);
    expect(eventHandlers.has("prefs:settings-loading")).toBe(false);
  });

  it.each([
    "settings",
    "profile",
  ])("queues %s export behind the profile name save", async (kind) => {
    window.location.hash = "#settings";
    let finishSave!: (value: { revision: number; json: string }) => void;
    mocked.SaveSettingsAtRevision.mockReturnValueOnce(
      new Promise((resolve) => (finishSave = resolve)) as never,
    );
    render(<App />);
    await act(async () => {});
    fireEvent.click(screen.getByRole("tab", { name: "Dock" }));
    fireEvent.change(screen.getByLabelText("Profile name"), { target: { value: "Exported name" } });
    fireEvent.blur(screen.getByLabelText("Profile name"));
    if (kind === "settings") fireEvent.click(screen.getByRole("tab", { name: "General" }));
    fireEvent.click(
      screen.getByRole("button", {
        name: kind === "settings" ? "Export settings" : "Export profile",
      }),
    );
    await waitFor(() => expect(mocked.SaveSettingsAtRevision).toHaveBeenCalledTimes(1));
    const save = kind === "settings" ? mocked.SaveSettingsExport : mocked.SaveLauncherProfileExport;
    expect(save).not.toHaveBeenCalled();
    const document = mocked.SaveSettingsAtRevision.mock.calls[0][0];
    expect(JSON.parse(document).replacementDock.profiles[0].name).toBe("Exported name");
    await act(async () => finishSave({ revision: 2, json: document }));
    await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
    expect(
      await screen.findByText(kind === "settings" ? "Settings exported." : "Profile exported."),
    ).toBeVisible();
  });

  it.each([
    "settings",
    "profile",
  ])("retires queued %s export when the preceding settings save fails", async (kind) => {
    window.location.hash = "#settings";
    let failSave!: (error: Error) => void;
    mocked.SaveSettingsAtRevision.mockReturnValueOnce(
      new Promise((_resolve, reject) => (failSave = reject)) as never,
    );
    render(<App />);
    await act(async () => {});
    fireEvent.click(screen.getByRole("tab", { name: "Dock" }));
    fireEvent.change(screen.getByLabelText("Profile name"), { target: { value: "Unsaved name" } });
    fireEvent.blur(screen.getByLabelText("Profile name"));
    if (kind === "settings") fireEvent.click(screen.getByRole("tab", { name: "General" }));
    fireEvent.click(
      screen.getByRole("button", {
        name: kind === "settings" ? "Export settings" : "Export profile",
      }),
    );
    await waitFor(() => expect(mocked.SaveSettingsAtRevision).toHaveBeenCalledTimes(1));
    await act(async () => failSave(new Error("settings: stale revision")));
    expect(await screen.findByText("The file could not be exported.")).toBeVisible();
    expect(
      kind === "settings" ? mocked.SaveSettingsExport : mocked.SaveLauncherProfileExport,
    ).not.toHaveBeenCalled();
  });

  it.each([
    "settings",
    "profile",
  ])("retires queued %s export after a newer preferences snapshot", async (kind) => {
    window.location.hash = "#settings";
    let finishSave!: (value: { revision: number; json: string }) => void;
    mocked.SaveSettingsAtRevision.mockReturnValueOnce(
      new Promise((resolve) => (finishSave = resolve)) as never,
    );
    render(<App />);
    await act(async () => {});
    fireEvent.click(screen.getByRole("tab", { name: "Dock" }));
    fireEvent.change(screen.getByLabelText("Profile name"), { target: { value: "Retired name" } });
    fireEvent.blur(screen.getByLabelText("Profile name"));
    if (kind === "settings") fireEvent.click(screen.getByRole("tab", { name: "General" }));
    fireEvent.click(
      screen.getByRole("button", {
        name: kind === "settings" ? "Export settings" : "Export profile",
      }),
    );
    await waitFor(() => expect(mocked.SaveSettingsAtRevision).toHaveBeenCalledTimes(1));
    const canonical = {
      ...defaultSettings,
      behavior: { ...defaultSettings.behavior, onboarded: true },
    };
    await act(async () => {
      eventHandlers.get("prefs:settings")?.({
        data: { generation: 2, revision: 3, json: JSON.stringify(canonical) },
      });
      finishSave({ revision: 2, json: mocked.SaveSettingsAtRevision.mock.calls[0][0] });
    });
    expect(await screen.findByText("The file could not be exported.")).toBeVisible();
    expect(
      kind === "settings" ? mocked.SaveSettingsExport : mocked.SaveLauncherProfileExport,
    ).not.toHaveBeenCalled();
  });

  it("queues profile import after a pending settings save and recovers from canonical result", async () => {
    window.location.hash = "#settings";
    const initial = {
      ...defaultSettings,
      behavior: { ...defaultSettings.behavior, onboarded: true },
    };
    const imported = {
      ...initial,
      replacementDock: {
        ...initial.replacementDock,
        profiles: [
          ...initial.replacementDock.profiles,
          { ...initial.replacementDock.profiles[0], id: "profile-imported", name: "Travel" },
        ],
      },
    };
    mocked.GetSettingsState.mockResolvedValueOnce({
      revision: 1,
      json: JSON.stringify(initial),
    } as never).mockResolvedValueOnce({ revision: 0, json: "{}" } as never);
    let releaseSave!: (value: { revision: number; json: string }) => void;
    mocked.SaveSettingsAtRevision.mockReturnValueOnce(
      new Promise((resolve) => (releaseSave = resolve)) as never,
    );
    mocked.PreviewLauncherProfileImport.mockResolvedValueOnce({
      digest: "digest-1",
      revision: "dock-r1",
      name: "Travel",
      itemCount: 1,
      widgetCount: 0,
      notices: ["selectionsRequireRepair"],
    } as never);
    mocked.ImportLauncherProfile.mockResolvedValueOnce({
      profileID: "profile-imported",
      settingsJSON: JSON.stringify(imported),
    } as never);

    render(<App />);
    fireEvent.click(await screen.findByRole("tab", { name: "Dock" }));
    fireEvent.change(screen.getByLabelText("Profile name"), { target: { value: "Changed" } });
    fireEvent.blur(screen.getByLabelText("Profile name"));
    const file = new File(["{}"], "profile.json", { type: "application/json" });
    fireEvent.change(screen.getByLabelText("Import profile file"), { target: { files: [file] } });
    fireEvent.click(await screen.findByRole("button", { name: "Import reviewed profile" }));
    expect(mocked.ImportLauncherProfile).not.toHaveBeenCalled();
    await act(async () => {
      releaseSave({ revision: 2, json: JSON.stringify(initial) });
      await Promise.resolve();
    });
    await waitFor(() => expect(mocked.ImportLauncherProfile).toHaveBeenCalledTimes(1));
    await waitFor(() =>
      expect(screen.getByLabelText("Profile", { exact: true })).toHaveValue("profile-imported"),
    );
    expect(screen.getByRole("button", { name: "Reload settings" })).toBeVisible();
  });

  it("does not let a retired native mutation re-block a newer preferences snapshot", async () => {
    window.location.hash = "#settings";
    const initial = {
      ...defaultSettings,
      behavior: { ...defaultSettings.behavior, onboarded: true },
    };
    mocked.GetSettingsState.mockResolvedValueOnce({
      revision: 1,
      json: JSON.stringify(initial),
    } as never);
    mocked.PreviewLauncherProfileImport.mockResolvedValueOnce({
      digest: "digest-old",
      revision: "dock-r1",
      name: "Imported",
      itemCount: 0,
      widgetCount: 0,
      notices: [],
    } as never);
    let finishImport!: (value: unknown) => void;
    mocked.ImportLauncherProfile.mockReturnValueOnce(
      new Promise((resolve) => (finishImport = resolve)) as never,
    );
    render(<App />);
    fireEvent.click(await screen.findByRole("tab", { name: "Dock" }));
    const file = new File(["{}"], "profile.json", { type: "application/json" });
    fireEvent.change(screen.getByLabelText("Import profile file"), { target: { files: [file] } });
    fireEvent.click(await screen.findByRole("button", { name: "Import reviewed profile" }));
    await waitFor(() => expect(mocked.ImportLauncherProfile).toHaveBeenCalledTimes(1));

    const winner = {
      ...initial,
      replacementDock: {
        ...initial.replacementDock,
        profiles: [{ ...initial.replacementDock.profiles[0], name: "New authority" }],
      },
    };
    act(() => eventHandlers.get("prefs:settings-loading")?.({ data: { generation: 3 } }));
    act(() =>
      eventHandlers.get("prefs:settings")?.({
        data: { generation: 3, revision: 6, json: JSON.stringify(winner) },
      }),
    );
    expect(screen.getByLabelText("Profile name")).toHaveValue("New authority");

    await act(async () => {
      finishImport({ profileID: "old-import", settingsJSON: JSON.stringify(initial) });
    });
    expect(screen.getByLabelText("Profile name")).toHaveValue("New authority");
    await waitFor(() => expect(screen.getByLabelText("Profile name")).toBeEnabled());
  });

  it("ignores delayed monitor-lock snapshot failures after runtime recovery", async () => {
    window.location.hash = "#settings";
    let rejectState!: (error: Error) => void;
    let rejectDisplays!: (error: Error) => void;
    (AppService as any).GetDockMonitorLockState.mockReturnValueOnce(
      new Promise((_, reject) => {
        rejectState = reject;
      }),
    );
    (AppService as any).GetDockMonitorLockDisplays.mockReturnValueOnce(
      new Promise((_, reject) => {
        rejectDisplays = reject;
      }),
    );
    mocked.GetSettings.mockResolvedValueOnce(
      JSON.stringify({
        ...defaultSettings,
        behavior: { ...defaultSettings.behavior, onboarded: true },
      }),
    );
    render(<App />);
    act(() =>
      eventHandlers.get("dock:monitor-lock")?.({
        data: {
          session: 1,
          revision: 1,
          generation: 2,
          sequence: 1,
          observedAtMs: 0,
          status: "protected",
          reason: "",
          targetUUID: "",
          actualUUID: "main",
          edge: "bottom",
          displays: [],
        },
      }),
    );
    await act(async () => {
      rejectState(new Error("late state failure"));
      rejectDisplays(new Error("late inventory failure"));
      await Promise.resolve();
    });
    fireEvent.click(await screen.findByRole("tab", { name: "Dock" }));
    expect(screen.getByText(/Status:/)).toHaveTextContent("Protected");
    expect(screen.queryByText(/late .* failure/)).toBeNull();
  });

  it("admits only current monotonic switcher material status", async () => {
    window.location.hash = "";
    mocked.GetSwitcherMaterialStatus.mockResolvedValueOnce({
      session: 21,
      revision: 1,
      state: "solid",
      reason: "missingReporter",
    });
    const { container } = render(<App />);
    act(() =>
      eventHandlers.get("switcher:show")?.({
        data: openSwitcherState({ session: 21, revision: 7 }),
      }),
    );
    expect(container.querySelector(".ot-panel")).toHaveClass("ot-solid-material");
    act(() => {
      eventHandlers.get("switcher:material")?.({
        data: { session: 21, revision: 4, state: "system" },
      });
      eventHandlers.get("switcher:material")?.({
        data: { session: 21, revision: 3, state: "solid" },
      });
      eventHandlers.get("switcher:material")?.({
        data: { session: 20, revision: 99, state: "solid" },
      });
    });
    expect(container.querySelector(".ot-panel")).toHaveClass("ot-native-material");
    act(() =>
      eventHandlers.get("switcher:show")?.({
        data: openSwitcherState({ session: 22, revision: 1 }),
      }),
    );
    expect(container.querySelector(".ot-panel")).toHaveClass("ot-solid-material");
  });

  it("keeps Dock material status scoped to a live windows presentation", () => {
    window.location.hash = "#dock";
    const { container } = render(<App />);
    const state = {
      session: 30,
      revision: 2,
      open: true,
      contentKind: "windows",
      item: {
        appId: 4,
        bundleId: "app",
        path: "/App.app",
        title: "App",
        bounds: { x: 0, y: 0, w: 40, h: 40 },
        screenId: 1,
        edge: "bottom",
        kind: "app",
      },
      entries: [{ ...appEntry(8, "Window"), appId: 4 }],
      selectedWindowId: 8,
      appearance: { ...emptyState.appearance, blur: true },
      emptyReason: "",
    };
    act(() => eventHandlers.get("dock:show")?.({ data: state }));
    act(() =>
      eventHandlers.get("dock:material")?.({
        data: { session: 30, revision: 2, state: "system" },
      }),
    );
    expect(container.querySelector(".ot-dock-panel")).toHaveClass("ot-native-material");
    act(() => eventHandlers.get("dock:update")?.({ data: { ...state, contentKind: "folder" } }));
    expect(container.querySelector(".ot-dock-panel")).toHaveClass("ot-solid-material");
    act(() => eventHandlers.get("dock:hide")?.({ data: { session: 30, revision: 3 } }));
    act(() => eventHandlers.get("dock:show")?.({ data: { ...state, session: 31, revision: 1 } }));
    expect(container.querySelector(".ot-dock-panel")).toHaveClass("ot-solid-material");
  });
});

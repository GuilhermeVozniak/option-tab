import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";

function openLauncherSection(name: "Widgets" | "Items" | "Focus rules") {
  fireEvent.click(
    within(screen.getByRole("navigation", { name: "Launcher sections" })).getByRole("button", {
      name,
    }),
  );
}

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
  Advance: vi.fn().mockResolvedValue(undefined),
  Reverse: vi.fn().mockResolvedValue(undefined),
  Confirm: vi.fn().mockResolvedValue(undefined),
  ConfirmWindow: vi.fn().mockResolvedValue(undefined),
  SelectApp: vi.fn().mockResolvedValue(undefined),
  SelectAppWindow: vi.fn().mockResolvedValue(undefined),
  ConfirmApp: vi.fn().mockResolvedValue(undefined),
  GetDockState: vi.fn().mockResolvedValue(null),
  GetDockMonitorLockState: vi.fn().mockResolvedValue({
    session: 0,
    revision: 0,
    generation: 0,
    sequence: 0,
    observedAtMs: 0,
    status: "disabled",
    reason: "",
    targetUUID: "",
    actualUUID: "",
    edge: "",
    displays: [],
    placementAvailable: false,
  }),
  GetDockMonitorLockDisplays: vi.fn().mockResolvedValue([]),
  SelectDockWindow: vi.fn().mockResolvedValue(undefined),
  FocusDockWindow: vi.fn().mockResolvedValue({ succeeded: 1, failures: [] }),
  PerformDockAction: vi.fn().mockResolvedValue({ succeeded: 1, failures: [] }),
  SetDockPanelSize: vi.fn().mockResolvedValue(undefined),
  Cancel: vi.fn().mockResolvedValue(undefined),
  Select: vi.fn().mockResolvedValue(undefined),
  SetSearch: vi.fn().mockResolvedValue(undefined),
  CloseSelected: vi.fn().mockResolvedValue(undefined),
  MinimizeSelected: vi.fn().mockResolvedValue(undefined),
  FullscreenSelected: vi.fn().mockResolvedValue(undefined),
  QuitSelectedApp: vi.fn().mockResolvedValue(undefined),
  HideSelectedApp: vi.fn().mockResolvedValue(undefined),
  SaveSettings: vi.fn().mockResolvedValue(undefined),
  GetSettings: vi.fn().mockResolvedValue("{}"),
  GetSettingsState: vi.fn().mockResolvedValue({ revision: 0, json: "{}" }),
  SaveSettingsAtRevision: vi.fn().mockResolvedValue({ revision: 0, json: "{}" }),
  GetPermissions: vi.fn().mockResolvedValue("{}"),
  GetVersion: vi.fn().mockResolvedValue("1.2.3"),
  InstallUpdate: vi.fn().mockResolvedValue(undefined),
  GetCrashReport: vi.fn().mockResolvedValue(""),
  GetMediaPermissions: vi.fn().mockResolvedValue({}),
  ConnectMediaProvider: vi.fn().mockResolvedValue({ status: "ready", reason: "" }),
  GetLauncherState: vi.fn().mockResolvedValue(null),
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
  GetLauncherInteractionCapabilities: vi.fn().mockResolvedValue({
    gestureAvailable: false,
    pinchAvailable: false,
    swipeAvailable: false,
    letterInputAvailable: false,
    hapticsAvailable: false,
    reason: "unavailable",
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
import { defaultSettings } from "./lib/types";
import type { WidgetCatalogDescriptor } from "./lib/widget-types";

const mocked = vi.mocked(AppService);

function canonicalBlacklistBackend() {
  const initial = structuredClone(defaultSettings);
  initial.behavior.onboarded = true;
  let stored = { revision: 1, json: JSON.stringify(initial) };
  mocked.GetSettingsState.mockReset().mockImplementation(() => Promise.resolve(stored) as never);
  mocked.SaveSettingsAtRevision.mockReset().mockImplementation((json, revision) => {
    const next = JSON.parse(json) as typeof initial;
    // config.Normalize drops empty matchers before returning canonical settings.
    next.filters.appBlacklist = next.filters.appBlacklist.filter((entry) => entry.match !== "");
    stored = { revision: revision + 1, json: JSON.stringify(next) };
    return Promise.resolve(stored) as never;
  });
  return () => stored;
}

it("keeps a new blacklist draft until a valid entry survives canonical save and reopen", async () => {
  window.location.hash = "#settings";
  const stored = canonicalBlacklistBackend();
  const first = render(<App />);
  await act(async () => {});
  fireEvent.click(screen.getByRole("tab", { name: "Excluded apps" }));
  fireEvent.click(screen.getByText("+ Add app"));
  await act(async () => {});
  expect(screen.getByLabelText("Blacklist entry 1")).toBeVisible();
  fireEvent.change(screen.getByLabelText("Blacklist entry 1"), {
    target: { value: " com.example.Acceptance " },
  });
  fireEvent.change(screen.getByLabelText("Blacklist hide 1"), {
    target: { value: "whenNoWindow" },
  });
  fireEvent.click(screen.getByLabelText("Blacklist ignore shortcuts 1"));
  expect(JSON.parse(stored().json).filters.appBlacklist).toEqual([]);
  fireEvent.click(screen.getByRole("button", { name: "Save app" }));
  await waitFor(() =>
    expect(JSON.parse(stored().json).filters.appBlacklist).toEqual([
      { match: "com.example.Acceptance", hide: "whenNoWindow", ignoreShortcuts: true },
    ]),
  );
  first.unmount();
  render(<App />);
  await act(async () => {});
  fireEvent.click(screen.getByRole("tab", { name: "Excluded apps" }));
  expect(screen.getByLabelText("Blacklist entry 1")).toHaveValue("com.example.Acceptance");
  expect(screen.getByLabelText("Blacklist hide 1")).toHaveValue("whenNoWindow");
  expect(screen.getByLabelText("Blacklist ignore shortcuts 1")).toBeChecked();
});

it("preserves blacklist drafts across unrelated canonical saves and cancels them on preferences refresh", async () => {
  window.location.hash = "#settings";
  const stored = canonicalBlacklistBackend();
  render(<App />);
  await act(async () => {});
  fireEvent.click(screen.getByRole("tab", { name: "Excluded apps" }));
  fireEvent.click(screen.getByText("+ Add app"));
  fireEvent.change(screen.getByLabelText("Blacklist entry 1"), {
    target: { value: "Unfinished app" },
  });
  fireEvent.click(screen.getByRole("tab", { name: "General" }));
  fireEvent.click(screen.getByLabelText("Capture windows in the background"));
  await waitFor(() => expect(stored().revision).toBe(2));
  fireEvent.click(screen.getByRole("tab", { name: "Excluded apps" }));
  expect(screen.getByLabelText("Blacklist entry 1")).toHaveValue("Unfinished app");
  expect(JSON.parse(stored().json).filters.appBlacklist).toEqual([]);
  act(() => eventHandlers.get("prefs:settings")?.({ data: { ...stored(), generation: 1 } }));
  expect(screen.queryByLabelText("Blacklist entry 1")).toBeNull();
  expect(JSON.parse(stored().json).filters.appBlacklist).toEqual([]);
});

it("edits and deletes persisted blacklist entries without saving an empty matcher", async () => {
  window.location.hash = "#settings";
  const stored = canonicalBlacklistBackend();
  const seeded = JSON.parse(stored().json) as typeof defaultSettings;
  seeded.filters.appBlacklist = [{ match: "Old app", hide: "always", ignoreShortcuts: false }];
  await mocked.SaveSettingsAtRevision(JSON.stringify(seeded), 1);
  render(<App />);
  await act(async () => {});
  fireEvent.click(screen.getByRole("tab", { name: "Excluded apps" }));
  const input = screen.getByLabelText("Blacklist entry 1");
  fireEvent.change(input, { target: { value: "" } });
  await act(async () => {});
  expect(screen.getByLabelText("Blacklist entry 1")).toBeInTheDocument();
  fireEvent.blur(input);
  expect(input).toHaveValue("Old app");
  fireEvent.change(input, { target: { value: "Updated app" } });
  fireEvent.blur(input);
  await waitFor(() =>
    expect(JSON.parse(stored().json).filters.appBlacklist[0].match).toBe("Updated app"),
  );
  fireEvent.click(screen.getByLabelText("Remove blacklist entry 1"));
  await waitFor(() => expect(JSON.parse(stored().json).filters.appBlacklist).toEqual([]));
  expect(screen.queryByLabelText("Blacklist entry 1")).toBeNull();
});

it("discards a failed blacklist save in favor of canonical settings", async () => {
  window.location.hash = "#settings";
  const stored = canonicalBlacklistBackend();
  render(<App />);
  await act(async () => {});
  fireEvent.click(screen.getByRole("tab", { name: "Excluded apps" }));
  fireEvent.click(screen.getByText("+ Add app"));
  fireEvent.change(screen.getByLabelText("Blacklist entry 1"), {
    target: { value: "Unsaved app" },
  });
  mocked.SaveSettingsAtRevision.mockRejectedValueOnce(new Error("disk full"));
  fireEvent.click(screen.getByRole("button", { name: "Save app" }));
  await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("disk full"));
  expect(screen.queryByLabelText("Blacklist entry 1")).toBeNull();
  expect(JSON.parse(stored().json).filters.appBlacklist).toEqual([]);
  fireEvent.click(screen.getByText("+ Add app"));
  fireEvent.change(screen.getByLabelText("Blacklist entry 1"), {
    target: { value: "Saved retry" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Save app" }));
  await waitFor(() =>
    expect(JSON.parse(stored().json).filters.appBlacklist[0].match).toBe("Saved retry"),
  );
});

it("does not announce an import whose response lost preferences authority", async () => {
  window.location.hash = "#settings";
  const stored = canonicalBlacklistBackend();
  let finish!: (value: { revision: number; json: string }) => void;
  mocked.SaveSettingsAtRevision.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }) as never,
  );
  const { container } = render(<App />);
  await act(async () => {});
  const file = new File([stored().json], "settings.json");
  Object.defineProperty(file, "text", { value: () => Promise.resolve(stored().json) });
  fireEvent.change(container.querySelector('input[type="file"]')!, { target: { files: [file] } });
  await waitFor(() => expect(mocked.SaveSettingsAtRevision).toHaveBeenCalledTimes(1));
  act(() =>
    eventHandlers.get("prefs:settings")?.({ data: { ...stored(), revision: 5, generation: 1 } }),
  );
  await act(async () => finish({ ...stored(), revision: 2 }));
  expect(screen.queryByText("Settings imported.")).toBeNull();
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Settings changed before this import could be saved.",
  );
});

it("discards an unsaved blacklist draft when settings are imported", async () => {
  window.location.hash = "#settings";
  const stored = canonicalBlacklistBackend();
  const { container } = render(<App />);
  await act(async () => {});
  fireEvent.click(screen.getByRole("tab", { name: "Excluded apps" }));
  fireEvent.click(screen.getByText("+ Add app"));
  fireEvent.change(screen.getByLabelText("Blacklist entry 1"), { target: { value: "Old draft" } });
  fireEvent.click(screen.getByRole("tab", { name: "General" }));
  const file = new File([stored().json], "settings.json");
  Object.defineProperty(file, "text", { value: () => Promise.resolve(stored().json) });
  fireEvent.change(container.querySelector('input[type="file"]')!, { target: { files: [file] } });
  await screen.findByText("Settings imported.");
  fireEvent.click(screen.getByRole("tab", { name: "Excluded apps" }));
  expect(screen.queryByLabelText("Blacklist entry 1")).toBeNull();
  expect(JSON.parse(stored().json).filters.appBlacklist).toEqual([]);
});

it.each([
  "installation",
  "failed removal",
] as const)("reloads disabled widget grants after package %s changes canonical settings", async (operation) => {
  window.location.hash = "#settings";
  const pkg: WidgetCatalogDescriptor = {
    packageID: "org.example.status",
    digest: "f".repeat(64),
    version: "1.0.0",
    name: { en: "Status card" },
    description: { en: "Local status" },
    requiredCapabilities: ["network.status.read"],
    optionalCapabilities: [],
    settings: [],
    builtin: false,
  };
  const initial = structuredClone(defaultSettings);
  initial.behavior.onboarded = true;
  initial.replacementDock.profiles[0].widgets = [
    {
      id: "status",
      packageID: pkg.packageID,
      digest: pkg.digest,
      enabled: true,
      grants: ["network.status.read"],
    },
  ];
  let stored = { revision: 1, json: JSON.stringify(initial) };
  let installed = operation === "failed removal";
  mocked.GetSettingsState.mockReset().mockImplementation(() => Promise.resolve(stored) as never);
  mocked.GetWidgetCatalog.mockImplementation(
    () => Promise.resolve(installed ? [pkg] : []) as never,
  );
  mocked.GetWidgetPackageStatus.mockResolvedValue({ available: true, busy: false, reason: "" });
  mocked.ReviewLocalWidgetPackage.mockResolvedValue({
    token: "review",
    sourceName: "status.otwidget",
    package: pkg,
    expiresAt: "2030-01-01T00:00:00Z",
    alreadyInstalled: false,
  } as never);
  const revoke = () => {
    const next = structuredClone(initial);
    next.replacementDock.profiles[0].widgets[0].enabled = false;
    next.replacementDock.profiles[0].widgets[0].grants = [];
    stored = { revision: 2, json: JSON.stringify(next) };
  };
  mocked.InstallReviewedWidget.mockReset().mockImplementation(() => {
    revoke();
    installed = true;
    return Promise.resolve(pkg) as never;
  });
  mocked.RemoveWidgetPackage.mockReset().mockImplementation(() => {
    revoke();
    return Promise.reject(new Error("package removal failed")) as never;
  });
  render(<App />);
  await act(async () => {});
  fireEvent.click(screen.getByRole("tab", { name: "Dock" }));
  openLauncherSection("Widgets");
  expect(
    screen.getByLabelText(installed ? "Enable Status card" : "Enable org.example.status"),
  ).toBeChecked();
  if (operation === "installation") {
    fireEvent.click(screen.getByRole("button", { name: "Review local package…" }));
    await screen.findByText("status.otwidget");
    fireEvent.click(screen.getByRole("button", { name: "Install reviewed package" }));
  } else fireEvent.click(screen.getByRole("button", { name: "Remove package" }));
  await waitFor(() => expect(screen.getByLabelText("Enable Status card")).not.toBeChecked());
  expect(screen.getByLabelText("Required · Network status")).not.toBeChecked();
  if (operation === "failed removal") {
    expect(screen.getByRole("alert")).toHaveTextContent(
      "The widget package operation could not be completed. Try again.",
    );
    expect(screen.queryByText("package removal failed")).toBeNull();
  }
});

beforeEach(() => {
  eventHandlers.clear();
  resetBackendProbeForTests();
  window.location.hash = "";
  mocked.GetMediaPermissions.mockClear();
  mocked.ConnectMediaProvider.mockClear();
  mocked.GetLauncherAppChoices.mockReset();
  mocked.GetLauncherAppChoices.mockResolvedValue([]);
  mocked.GetWidgetCatalog.mockReset();
  mocked.GetWidgetCatalog.mockResolvedValue([]);
  mocked.GetWidgetPackageStatus.mockReset();
  mocked.GetWidgetPackageStatus.mockResolvedValue({ available: false, busy: false, reason: "" });
  mocked.GetLauncherItemStatus.mockResolvedValue({ available: false, busy: false, reason: "" });
  mocked.GetLauncherItemSettings.mockResolvedValue({
    profileID: "default",
    revision: "r1",
    items: [],
    references: [],
    iconIDs: [],
  });
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

it("loads exact launcher app choices only for the settings editor", async () => {
  window.location.hash = "#settings";
  mocked.GetLauncherAppChoices.mockResolvedValueOnce([
    { name: "Editor", bundleID: "com.example.editor" },
    { name: "Editor", bundleID: "org.example.editor" },
  ]);
  mocked.GetSettingsState.mockResolvedValueOnce({
    revision: 1,
    json: JSON.stringify({
      ...defaultSettings,
      behavior: { ...defaultSettings.behavior, onboarded: true },
    }),
  } as never);
  render(<App />);
  await act(async () => {});
  fireEvent.click(screen.getByRole("tab", { name: "Dock" }));
  openLauncherSection("Focus rules");
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Add focus rule" })));
  expect(mocked.GetLauncherAppChoices).toHaveBeenCalledTimes(1);
  expect(mocked.GetWidgetCatalog).toHaveBeenCalledTimes(1);
  expect(mocked.GetWidgetPackageStatus).toHaveBeenCalledTimes(1);
  expect(screen.getByLabelText("Running app")).toHaveTextContent("Editor — org.example.editor");
});

it("shows persistence errors without unmounting preferences", async () => {
  window.location.hash = "#settings";
  mocked.GetSettingsState.mockResolvedValueOnce({
    revision: 1,
    json: JSON.stringify({
      ...defaultSettings,
      behavior: { ...defaultSettings.behavior, onboarded: true },
    }),
  } as never);
  mocked.SaveSettingsAtRevision.mockRejectedValueOnce(new Error("disk full"));
  render(<App />);
  await act(async () => {});
  expect(mocked.GetMediaPermissions).toHaveBeenCalledTimes(1);
  expect(mocked.ConnectMediaProvider).not.toHaveBeenCalled();
  fireEvent.click(screen.getByLabelText("Start at login"));
  await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("disk full"));
  expect(screen.getByRole("tablist", { name: "Settings sections" })).toBeVisible();
  expect(screen.getByRole("tabpanel", { name: "General" })).toBeVisible();
});

it("keeps preferences usable after an invalid import and loads canonical partial settings", async () => {
  window.location.hash = "#settings";
  const initial = {
    ...defaultSettings,
    behavior: { ...defaultSettings.behavior, onboarded: true },
  };
  mocked.GetSettingsState.mockResolvedValueOnce({
    revision: 1,
    json: JSON.stringify(initial),
  } as never);
  const { container } = render(<App />);
  await act(async () => {});
  const input = container.querySelector('input[type="file"]') as HTMLInputElement;
  const upload = (text: string) => {
    const file = new File([text], "settings.json");
    Object.defineProperty(file, "text", { value: () => Promise.resolve(text) });
    fireEvent.change(input, { target: { files: [file] } });
  };
  upload("null");
  await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("JSON object"));
  expect(screen.getByLabelText("Start at login")).toBeInTheDocument();
  const canonical = { ...initial, behavior: { ...initial.behavior, startAtLogin: true } };
  mocked.SaveSettingsAtRevision.mockResolvedValueOnce({
    revision: 2,
    json: JSON.stringify(canonical),
  } as never);
  const importedDocument = '{\n  "version": 1,\n  "behavior": {"startAtLogin": true}\n}';
  upload(importedDocument);
  await waitFor(() => expect(screen.getByLabelText("Start at login")).toBeChecked());
  expect(mocked.SaveSettingsAtRevision).toHaveBeenLastCalledWith(importedDocument, 1);
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
});

it("serializes saves without letting older completions reset newer optimistic edits", async () => {
  window.location.hash = "#settings";
  mocked.GetSettingsState.mockResolvedValueOnce({
    revision: 1,
    json: JSON.stringify({
      ...defaultSettings,
      behavior: { ...defaultSettings.behavior, onboarded: true },
    }),
  } as never);
  let finish: (value: { revision: number; json: string }) => void = () => {};
  let finishSecond: (value: { revision: number; json: string }) => void = () => {};
  const first = new Promise<{ revision: number; json: string }>((resolve) => {
    finish = resolve;
  });
  mocked.SaveSettingsAtRevision.mockClear();
  mocked.SaveSettingsAtRevision.mockImplementationOnce(
    () => first as ReturnType<typeof AppService.SaveSettingsAtRevision>,
  ).mockImplementationOnce(
    () =>
      new Promise((resolve) => (finishSecond = resolve)) as ReturnType<
        typeof AppService.SaveSettingsAtRevision
      >,
  );
  render(<App />);
  await act(async () => {});
  fireEvent.click(screen.getByLabelText("Start at login"));
  fireEvent.click(screen.getByLabelText("Start at login"));
  await waitFor(() => expect(mocked.SaveSettingsAtRevision).toHaveBeenCalledTimes(1));
  await act(async () => {
    finish({ revision: 2, json: mocked.SaveSettingsAtRevision.mock.calls[0][0] });
  });
  await waitFor(() => expect(mocked.SaveSettingsAtRevision).toHaveBeenCalledTimes(2));
  expect(screen.getByLabelText("Start at login")).not.toBeChecked();
  fireEvent.click(screen.getByLabelText("Start at login"));
  expect(screen.getByLabelText("Start at login")).toBeChecked();
  await act(async () => {
    finishSecond({ revision: 3, json: mocked.SaveSettingsAtRevision.mock.calls[1][0] });
  });
  await waitFor(() => expect(mocked.SaveSettingsAtRevision).toHaveBeenCalledTimes(3));
  expect(JSON.parse(mocked.SaveSettingsAtRevision.mock.calls[1][0]).behavior.startAtLogin).toBe(
    false,
  );
  expect(mocked.SaveSettingsAtRevision.mock.calls[1][1]).toBe(2);
  expect(JSON.parse(mocked.SaveSettingsAtRevision.mock.calls[2][0]).behavior.startAtLogin).toBe(
    true,
  );
  expect(mocked.SaveSettingsAtRevision.mock.calls[2][1]).toBe(3);
});

it("queues launcher item CAS saves behind settings writes and blocks stale fallback writes", async () => {
  mocked.SaveSettingsAtRevision.mockReset();
  mocked.SetLauncherItems.mockClear();
  window.location.hash = "#settings";
  const initial = {
    ...defaultSettings,
    behavior: { ...defaultSettings.behavior, onboarded: true },
  };
  mocked.GetSettingsState.mockResolvedValueOnce({
    revision: 1,
    json: JSON.stringify(initial),
  } as never);
  mocked.GetLauncherItemStatus.mockResolvedValue({ available: true, busy: false, reason: "" });
  let finishSave: (value: { revision: number; json: string }) => void = () => {};
  mocked.SaveSettingsAtRevision.mockImplementationOnce(
    () =>
      new Promise((resolve) => (finishSave = resolve)) as ReturnType<
        typeof AppService.SaveSettingsAtRevision
      >,
  );
  mocked.SetLauncherItems.mockResolvedValueOnce({
    profileID: "default",
    revision: "r2",
    items: [
      {
        id: "spacer-1",
        kind: "spacer",
        label: "",
        referenceID: "",
        url: "",
        iconID: "",
        members: [],
        folderView: "",
      },
    ],
    references: [],
    iconIDs: [],
  });
  render(<App />);
  await act(async () => {});
  fireEvent.click(screen.getByLabelText("Start at login"));
  fireEvent.click(screen.getByRole("tab", { name: "Dock" }));
  openLauncherSection("Items");
  fireEvent.click(await screen.findByRole("button", { name: "Add spacer" }));
  fireEvent.click(screen.getByRole("button", { name: "Save launcher items" }));
  expect(mocked.SetLauncherItems).not.toHaveBeenCalled();
  mocked.GetSettingsState.mockRejectedValueOnce(new Error("reload failed"));
  await act(async () =>
    finishSave({ revision: 2, json: mocked.SaveSettingsAtRevision.mock.calls[0][0] }),
  );
  await waitFor(() => expect(mocked.SetLauncherItems).toHaveBeenCalledTimes(1));
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "Reload settings" })).toBeVisible(),
  );
  expect(mocked.SaveSettingsAtRevision).toHaveBeenCalledTimes(1);
});

it("retires a queued native settings mutation when preferences unmount", async () => {
  window.location.hash = "#settings";
  mocked.GetLauncherItemStatus.mockResolvedValue({ available: true, busy: false, reason: "" });
  let finishSave!: (value: { revision: number; json: string }) => void;
  mocked.SaveSettingsAtRevision.mockImplementationOnce(
    () =>
      new Promise((resolve) => (finishSave = resolve)) as ReturnType<
        typeof AppService.SaveSettingsAtRevision
      >,
  );
  const view = render(<App />);
  await act(async () => {});
  const savesBefore = mocked.SaveSettingsAtRevision.mock.calls.length;
  const mutationsBefore = mocked.SetLauncherItems.mock.calls.length;
  fireEvent.click(screen.getByLabelText("Start at login"));
  fireEvent.click(screen.getByRole("tab", { name: "Dock" }));
  openLauncherSection("Items");
  fireEvent.click(await screen.findByRole("button", { name: "Add spacer" }));
  fireEvent.click(screen.getByRole("button", { name: "Save launcher items" }));
  await waitFor(() => expect(mocked.SaveSettingsAtRevision).toHaveBeenCalledTimes(savesBefore + 1));
  view.unmount();
  await act(async () => {
    finishSave({ revision: 2, json: mocked.SaveSettingsAtRevision.mock.calls[savesBefore][0] });
  });
  expect(mocked.SetLauncherItems).toHaveBeenCalledTimes(mutationsBefore);
});

it("publishes a successful import even if the next queued import fails", async () => {
  window.location.hash = "#settings";
  const initial = {
    ...defaultSettings,
    behavior: { ...defaultSettings.behavior, onboarded: true },
  };
  mocked.GetSettingsState.mockResolvedValueOnce({
    revision: 1,
    json: JSON.stringify(initial),
  } as never);
  let finish: (value: { revision: number; json: string }) => void = () => {};
  const first = new Promise<{ revision: number; json: string }>((resolve) => {
    finish = resolve;
  });
  mocked.SaveSettingsAtRevision.mockImplementationOnce(
    () => first as ReturnType<typeof AppService.SaveSettingsAtRevision>,
  );
  const { container } = render(<App />);
  await act(async () => {});
  const input = container.querySelector('input[type="file"]') as HTMLInputElement;
  const upload = (text: string) => {
    const file = new File([text], "settings.json");
    Object.defineProperty(file, "text", { value: () => Promise.resolve(text) });
    fireEvent.change(input, { target: { files: [file] } });
  };
  const canonical = { ...initial, behavior: { ...initial.behavior, startAtLogin: true } };
  await act(async () => {
    upload('{"behavior":{"startAtLogin":true}}');
  });
  await act(async () => {
    upload("null");
  });
  await act(async () => {
    finish({ revision: 2, json: JSON.stringify(canonical) });
  });
  await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("JSON object"));
  expect(screen.getByLabelText("Start at login")).toBeChecked();
});

// A backend that behaves like Go for the Preferences round trip: it rejects a
// stale revision, drops chordless shortcuts (config.Normalize) and echoes the
// canonical document. `external` is a save Go made itself (tray Pause, native
// Dock recovery, launcher mutations), which retires the renderer's revision.
function canonicalSettingsBackend() {
  const initial = structuredClone(defaultSettings);
  initial.behavior.onboarded = true;
  let stored = { revision: 1, json: JSON.stringify(initial) };
  mocked.GetSettingsState.mockReset().mockImplementation(() => Promise.resolve(stored) as never);
  mocked.SaveSettingsAtRevision.mockReset().mockImplementation((json, revision) => {
    if (revision !== stored.revision)
      return Promise.reject(new Error("settings: stale revision")) as never;
    const next = JSON.parse(json) as typeof initial;
    next.shortcuts = next.shortcuts.filter((shortcut) => shortcut.chord !== "");
    stored = { revision: revision + 1, json: JSON.stringify(next) };
    return Promise.resolve(stored) as never;
  });
  return {
    stored: () => stored,
    settings: () => JSON.parse(stored.json) as typeof initial,
    external: (mutate: (settings: typeof initial) => void) => {
      const next = JSON.parse(stored.json) as typeof initial;
      mutate(next);
      stored = { revision: stored.revision + 1, json: JSON.stringify(next) };
      return stored;
    },
  };
}

function openPreferences(generation: number, state: { revision: number; json: string }) {
  act(() => eventHandlers.get("prefs:settings-loading")?.({ data: { generation } }));
  act(() => eventHandlers.get("prefs:settings")?.({ data: { ...state, generation } }));
}

it("keeps a new shortcut local until its chord is recorded, then saves it through the canonical echo", async () => {
  window.location.hash = "#settings";
  const backend = canonicalSettingsBackend();
  render(<App />);
  await act(async () => {});
  fireEvent.click(screen.getByRole("tab", { name: "Shortcuts" }));
  fireEvent.click(screen.getByText("+ Add shortcut"));
  await act(async () => {});
  // Persisting the chordless row let the canonical echo delete it again.
  expect(mocked.SaveSettingsAtRevision).not.toHaveBeenCalled();
  expect(screen.getByLabelText("Shortcut 3 chord")).toHaveValue("");
  expect(screen.getByText("+ Add shortcut").closest("button")).toBeDisabled();
  fireEvent.change(screen.getByLabelText("Shortcut 3 mode"), { target: { value: "apps" } });
  fireEvent.change(screen.getByLabelText("Shortcut 3 scope"), { target: { value: "activeApp" } });
  await act(async () => {});
  expect(mocked.SaveSettingsAtRevision).not.toHaveBeenCalled();
  expect(screen.getByLabelText("Shortcut 3 mode")).toHaveValue("apps");

  // An unrelated canonical save must not cancel the unfinished row.
  fireEvent.click(screen.getByLabelText("Shortcut 1 enabled"));
  await waitFor(() => expect(backend.stored().revision).toBe(2));
  expect(backend.settings().shortcuts.map((shortcut) => shortcut.id)).toEqual([1, 2]);
  expect(screen.getByLabelText("Shortcut 3 chord")).toHaveValue("");

  fireEvent.keyDown(screen.getByLabelText("Shortcut 3 chord"), {
    code: "KeyK",
    ctrlKey: true,
    altKey: true,
  });
  await waitFor(() =>
    expect(backend.settings().shortcuts).toContainEqual({
      id: 3,
      chord: "control+option+k",
      enabled: true,
      scope: { appScope: "activeApp" },
      mode: "apps",
    }),
  );
  await act(async () => {});
  expect(screen.getByLabelText("Shortcut 3 chord")).toHaveValue("control+option+k");
  expect(screen.getByLabelText("Shortcut 3 mode")).toHaveValue("apps");
  expect(screen.getByText("+ Add shortcut").closest("button")).toBeEnabled();
  expect(screen.queryByRole("alert")).toBeNull();
});

it("cancels an unrecorded shortcut without saving and on a preferences refresh", async () => {
  window.location.hash = "#settings";
  const backend = canonicalSettingsBackend();
  render(<App />);
  await act(async () => {});
  fireEvent.click(screen.getByRole("tab", { name: "Shortcuts" }));
  fireEvent.click(screen.getByText("+ Add shortcut"));
  fireEvent.click(screen.getByLabelText("Remove shortcut 3"));
  expect(screen.queryByLabelText("Shortcut 3 chord")).toBeNull();
  fireEvent.click(screen.getByText("+ Add shortcut"));
  expect(screen.getByLabelText("Shortcut 3 chord")).toBeInTheDocument();
  openPreferences(1, backend.stored());
  await act(async () => {});
  expect(screen.queryByLabelText("Shortcut 3 chord")).toBeNull();
  expect(mocked.SaveSettingsAtRevision).not.toHaveBeenCalled();
  expect(backend.settings().shortcuts.map((shortcut) => shortcut.id)).toEqual([1, 2]);
});

it("navigates on every preferences deep link, including a repeated one", async () => {
  window.location.hash = "#settings";
  const scrollIntoView = vi.fn();
  Element.prototype.scrollIntoView = scrollIntoView;
  render(<App />);
  await act(async () => {});
  const tab = (name: string) => screen.getByRole("tab", { name });
  act(() => eventHandlers.get("prefs:tab")?.({ data: "About" }));
  expect(tab("About")).toHaveAttribute("aria-selected", "true");
  fireEvent.click(tab("General"));
  expect(tab("General")).toHaveAttribute("aria-selected", "true");
  act(() => eventHandlers.get("prefs:tab")?.({ data: "About" }));
  expect(tab("About")).toHaveAttribute("aria-selected", "true");

  act(() => eventHandlers.get("prefs:tab")?.({ data: "General#updates" }));
  expect(tab("General")).toHaveAttribute("aria-selected", "true");
  expect(scrollIntoView).toHaveBeenCalledTimes(1);
  fireEvent.click(tab("Dock"));
  act(() => eventHandlers.get("prefs:tab")?.({ data: "General#updates" }));
  expect(tab("General")).toHaveAttribute("aria-selected", "true");
  expect(scrollIntoView).toHaveBeenCalledTimes(2);
});

it("accepts an edit after settings were saved outside open Preferences", async () => {
  window.location.hash = "#settings";
  const backend = canonicalSettingsBackend();
  render(<App />);
  await act(async () => {});
  openPreferences(1, backend.stored());
  // Tray → Pause: Go pushes the snapshot under the current refresh generation.
  const paused = backend.external((settings) => {
    settings.behavior.paused = true;
  });
  act(() => eventHandlers.get("prefs:settings")?.({ data: { ...paused, generation: 1 } }));
  fireEvent.click(screen.getByLabelText("Start at login"));
  await waitFor(() => expect(backend.stored().revision).toBe(3));
  expect(mocked.SaveSettingsAtRevision).toHaveBeenLastCalledWith(expect.any(String), 2);
  expect(backend.settings().behavior).toMatchObject({ paused: true, startAtLogin: true });
  expect(screen.getByLabelText("Start at login")).toBeChecked();
  expect(screen.queryByRole("alert")).toBeNull();
});

it("gives an edit in flight to the newer snapshot of an outside save without a stale error", async () => {
  window.location.hash = "#settings";
  const backend = canonicalSettingsBackend();
  render(<App />);
  await act(async () => {});
  openPreferences(1, backend.stored());
  // Go commits the tray toggle first, so the renderer's save at revision 1
  // loses; the pushed snapshot reaches the window before that rejection.
  let reject!: (error: Error) => void;
  mocked.SaveSettingsAtRevision.mockImplementationOnce(
    () =>
      new Promise((_, fail) => {
        reject = fail;
      }) as never,
  );
  fireEvent.click(screen.getByLabelText("Start at login"));
  await waitFor(() => expect(mocked.SaveSettingsAtRevision).toHaveBeenCalledTimes(1));
  const paused = backend.external((settings) => {
    settings.behavior.paused = true;
  });
  act(() => eventHandlers.get("prefs:settings")?.({ data: { ...paused, generation: 1 } }));
  await act(async () => reject(new Error("settings: stale revision")));
  expect(screen.queryByRole("alert")).toBeNull();
  expect(screen.getByLabelText("Start at login")).not.toBeChecked();
  expect(screen.getByLabelText("Start at login")).toBeEnabled();
  fireEvent.click(screen.getByLabelText("Start at login"));
  await waitFor(() => expect(backend.stored().revision).toBe(3));
  expect(backend.settings().behavior).toMatchObject({ paused: true, startAtLogin: true });
  expect(screen.queryByRole("alert")).toBeNull();
});

it("does not let a snapshot Preferences already hold retire an edit in flight", async () => {
  window.location.hash = "#settings";
  const backend = canonicalSettingsBackend();
  render(<App />);
  await act(async () => {});
  openPreferences(1, backend.stored());
  const known = backend.stored();
  const save = mocked.SaveSettingsAtRevision.getMockImplementation()!;
  let finish!: () => void;
  mocked.SaveSettingsAtRevision.mockImplementationOnce((json, revision) => {
    const result = save(json, revision);
    return new Promise((resolve) => {
      finish = () => resolve(result);
    }) as never;
  });
  fireEvent.click(screen.getByLabelText("Start at login"));
  await waitFor(() => expect(mocked.SaveSettingsAtRevision).toHaveBeenCalledTimes(1));
  // A delayed push of the revision the renderer already reloaded carries no news.
  act(() => eventHandlers.get("prefs:settings")?.({ data: { ...known, generation: 1 } }));
  expect(screen.getByLabelText("Start at login")).toBeChecked();
  await act(async () => finish());
  expect(screen.getByLabelText("Start at login")).toBeChecked();
  fireEvent.click(screen.getByLabelText("Capture windows in the background"));
  await waitFor(() => expect(backend.stored().revision).toBe(3));
  expect(mocked.SaveSettingsAtRevision).toHaveBeenLastCalledWith(expect.any(String), 2);
  expect(backend.settings().behavior).toMatchObject({
    startAtLogin: true,
    captureInBackground: true,
  });
  expect(screen.queryByRole("alert")).toBeNull();
});

it("reloads launcher app choices whenever Preferences are opened", async () => {
  window.location.hash = "#settings";
  const backend = canonicalSettingsBackend();
  // The preferences webview is created hidden: Go answers [] until it is open.
  let open = false;
  mocked.GetLauncherAppChoices.mockImplementation(
    () =>
      Promise.resolve(open ? [{ name: "Editor", bundleID: "com.example.editor" }] : []) as never,
  );
  render(<App />);
  await act(async () => {});
  expect(mocked.GetLauncherAppChoices).toHaveBeenCalledTimes(1);
  open = true;
  openPreferences(1, backend.stored());
  await waitFor(() => expect(mocked.GetLauncherAppChoices).toHaveBeenCalledTimes(2));
  await act(async () => {});
  fireEvent.click(screen.getByRole("tab", { name: "Dock" }));
  openLauncherSection("Focus rules");
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "Add focus rule" })));
  expect(screen.getByLabelText("Running app")).toHaveTextContent("Editor — com.example.editor");
  // A snapshot pushed for an outside save is not another open.
  const paused = backend.external((settings) => {
    settings.behavior.paused = true;
  });
  act(() => eventHandlers.get("prefs:settings")?.({ data: { ...paused, generation: 1 } }));
  await act(async () => {});
  expect(mocked.GetLauncherAppChoices).toHaveBeenCalledTimes(2);
  openPreferences(2, backend.stored());
  await waitFor(() => expect(mocked.GetLauncherAppChoices).toHaveBeenCalledTimes(3));
});

it("stays out of the onboarding wizard after resetting to defaults", async () => {
  window.location.hash = "#settings";
  const backend = canonicalSettingsBackend();
  render(<App />);
  await act(async () => {});
  fireEvent.click(screen.getByLabelText("Capture windows in the background"));
  await waitFor(() => expect(backend.stored().revision).toBe(2));
  fireEvent.click(screen.getByLabelText("Reset to defaults"));
  await waitFor(() => expect(backend.stored().revision).toBe(3));
  await act(async () => {});
  expect(backend.settings().behavior).toMatchObject({
    onboarded: true,
    captureInBackground: false,
  });
  expect(screen.queryByText("Welcome to Option Tab")).toBeNull();
  expect(screen.getByRole("tab", { name: "General" })).toHaveAttribute("aria-selected", "true");
});

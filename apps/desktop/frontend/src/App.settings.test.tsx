import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";

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
  Advance: vi.fn().mockResolvedValue(undefined),
  Reverse: vi.fn().mockResolvedValue(undefined),
  Confirm: vi.fn().mockResolvedValue(undefined),
  ConfirmWindow: vi.fn().mockResolvedValue(undefined),
  SelectApp: vi.fn().mockResolvedValue(undefined),
  SelectAppWindow: vi.fn().mockResolvedValue(undefined),
  ConfirmApp: vi.fn().mockResolvedValue(undefined),
  GetDockState: vi.fn().mockResolvedValue(null),
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

const mocked = vi.mocked(AppService);

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
});

it("loads exact launcher app choices only for the settings editor", async () => {
  window.location.hash = "#settings";
  mocked.GetLauncherAppChoices.mockResolvedValueOnce([
    { name: "Editor", bundleID: "com.example.editor" },
    { name: "Editor", bundleID: "org.example.editor" },
  ]);
  mocked.GetSettings.mockResolvedValueOnce(
    JSON.stringify({
      ...defaultSettings,
      behavior: { ...defaultSettings.behavior, onboarded: true },
    }),
  );
  render(<App />);
  await act(async () => {});
  fireEvent.click(screen.getByRole("tab", { name: "Dock" }));
  fireEvent.click(screen.getByRole("button", { name: "Add focus rule" }));
  expect(mocked.GetLauncherAppChoices).toHaveBeenCalledTimes(1);
  expect(mocked.GetWidgetCatalog).toHaveBeenCalledTimes(1);
  expect(mocked.GetWidgetPackageStatus).toHaveBeenCalledTimes(1);
  expect(screen.getByLabelText("Running app")).toHaveTextContent("Editor — org.example.editor");
});

it("shows persistence errors without unmounting preferences", async () => {
  window.location.hash = "#settings";
  mocked.GetSettings.mockResolvedValueOnce(
    JSON.stringify({
      ...defaultSettings,
      behavior: { ...defaultSettings.behavior, onboarded: true },
    }),
  );
  mocked.SaveSettings.mockRejectedValueOnce(new Error("disk full"));
  render(<App />);
  await act(async () => {});
  expect(mocked.GetMediaPermissions).toHaveBeenCalledTimes(1);
  expect(mocked.ConnectMediaProvider).not.toHaveBeenCalled();
  fireEvent.click(screen.getByLabelText("Start at login"));
  await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("disk full"));
  expect(screen.getByText(/Preferences/)).toBeInTheDocument();
});

it("keeps preferences usable after an invalid import and loads canonical partial settings", async () => {
  window.location.hash = "#settings";
  const initial = {
    ...defaultSettings,
    behavior: { ...defaultSettings.behavior, onboarded: true },
  };
  mocked.GetSettings.mockResolvedValueOnce(JSON.stringify(initial));
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
  mocked.GetSettings.mockResolvedValueOnce(JSON.stringify(canonical));
  upload('{"version":1,"behavior":{"startAtLogin":true}}');
  await waitFor(() => expect(screen.getByLabelText("Start at login")).toBeChecked());
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
});

it("serializes rapid preference saves", async () => {
  window.location.hash = "#settings";
  mocked.GetSettings.mockResolvedValueOnce(
    JSON.stringify({
      ...defaultSettings,
      behavior: { ...defaultSettings.behavior, onboarded: true },
    }),
  );
  let finish: () => void = () => {};
  const first = new Promise<void>((resolve) => {
    finish = resolve;
  });
  mocked.SaveSettings.mockClear();
  mocked.SaveSettings.mockImplementationOnce(
    () => first as ReturnType<typeof AppService.SaveSettings>,
  );
  render(<App />);
  await act(async () => {});
  fireEvent.click(screen.getByLabelText("Start at login"));
  fireEvent.click(screen.getByLabelText("Start at login"));
  await waitFor(() => expect(mocked.SaveSettings).toHaveBeenCalledTimes(1));
  await act(async () => {
    finish();
  });
  await waitFor(() => expect(mocked.SaveSettings).toHaveBeenCalledTimes(2));
  expect(JSON.parse(mocked.SaveSettings.mock.calls[1][0]).behavior.startAtLogin).toBe(false);
});

it("queues launcher item CAS saves behind settings writes and reloads the canonical settings", async () => {
  mocked.SaveSettings.mockReset();
  mocked.SaveSettings.mockResolvedValue(undefined);
  mocked.SetLauncherItems.mockClear();
  window.location.hash = "#settings";
  const initial = {
    ...defaultSettings,
    behavior: { ...defaultSettings.behavior, onboarded: true },
  };
  mocked.GetSettings.mockResolvedValueOnce(JSON.stringify(initial));
  mocked.GetLauncherItemStatus.mockResolvedValue({ available: true, busy: false, reason: "" });
  let finishSave: () => void = () => {};
  mocked.SaveSettings.mockImplementationOnce(
    () =>
      new Promise<void>((resolve) => (finishSave = resolve)) as ReturnType<
        typeof AppService.SaveSettings
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
  fireEvent.click(await screen.findByRole("button", { name: "Add spacer" }));
  fireEvent.click(screen.getByRole("button", { name: "Save launcher items" }));
  expect(mocked.SetLauncherItems).not.toHaveBeenCalled();
  mocked.GetSettings.mockRejectedValueOnce(new Error("reload failed"));
  await act(async () => finishSave());
  await waitFor(() => expect(mocked.SetLauncherItems).toHaveBeenCalledTimes(1));
  await waitFor(() => expect(mocked.GetSettings.mock.calls.length).toBeGreaterThanOrEqual(2));
  expect(mocked.SaveSettings).toHaveBeenCalledTimes(1);
  fireEvent.click(screen.getByRole("tab", { name: "General" }));
  fireEvent.click(screen.getByLabelText("Start at login"));
  await waitFor(() => expect(mocked.SaveSettings).toHaveBeenCalledTimes(2));
  expect(
    JSON.parse(mocked.SaveSettings.mock.calls[1][0]).replacementDock.profiles[0].items,
  ).toEqual([expect.objectContaining({ id: "spacer-1" })]);
});

it("publishes a successful import even if the next queued import fails", async () => {
  window.location.hash = "#settings";
  const initial = {
    ...defaultSettings,
    behavior: { ...defaultSettings.behavior, onboarded: true },
  };
  mocked.GetSettings.mockResolvedValueOnce(JSON.stringify(initial));
  let finish: () => void = () => {};
  const first = new Promise<void>((resolve) => {
    finish = resolve;
  });
  mocked.SaveSettings.mockImplementationOnce(
    () => first as ReturnType<typeof AppService.SaveSettings>,
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
  mocked.GetSettings.mockResolvedValueOnce(JSON.stringify(canonical));
  await act(async () => {
    upload('{"behavior":{"startAtLogin":true}}');
  });
  await act(async () => {
    upload("null");
  });
  await act(async () => {
    finish();
  });
  await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("JSON object"));
  expect(screen.getByLabelText("Start at login")).toBeChecked();
});

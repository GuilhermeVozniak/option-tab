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
  fireEvent.click(screen.getByRole("button", { name: "Add focus rule" }));
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
  expect(screen.getByText(/Preferences/)).toBeInTheDocument();
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

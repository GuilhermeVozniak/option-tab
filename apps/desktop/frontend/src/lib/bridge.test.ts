import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// Mock the Wails v3 seams: the generated App service bindings and the
// @wailsio/runtime event bus. The events mock keeps a handler registry so tests
// can dispatch payloads exactly like window._wails.dispatchWailsEvent does.
const eventHandlers = new Map<string, (ev: { data: unknown }) => void>();
const eventOffs: Array<ReturnType<typeof vi.fn>> = [];

vi.mock("@wailsio/runtime", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@wailsio/runtime")>();
  return {
    ...actual,
    Events: {
      On: vi.fn((name: string, cb: (ev: { data: unknown }) => void) => {
        eventHandlers.set(name, cb);
        const off = vi.fn(() => eventHandlers.delete(name));
        eventOffs.push(off);
        return off;
      }),
    },
  };
});

vi.mock("../../bindings/option-tab/app.js", () => ({
  Advance: vi.fn().mockResolvedValue(undefined),
  Reverse: vi.fn().mockResolvedValue(undefined),
  Confirm: vi.fn().mockResolvedValue(undefined),
  Cancel: vi.fn().mockResolvedValue(undefined),
  Select: vi.fn().mockResolvedValue(undefined),
  SetSearch: vi.fn().mockResolvedValue(undefined),
  CloseSelected: vi.fn().mockResolvedValue(undefined),
  MinimizeSelected: vi.fn().mockResolvedValue(undefined),
  FullscreenSelected: vi.fn().mockResolvedValue(undefined),
  QuitSelectedApp: vi.fn().mockResolvedValue(undefined),
  HideSelectedApp: vi.fn().mockResolvedValue(undefined),
  TogglePause: vi.fn().mockResolvedValue(undefined),
  SetPaused: vi.fn().mockResolvedValue(undefined),
  IsPaused: vi.fn().mockResolvedValue(false),
  OpenPreferences: vi.fn().mockResolvedValue(undefined),
  ClosePreferences: vi.fn().mockResolvedValue(undefined),
  GetSettings: vi.fn().mockResolvedValue("{}"),
  SaveSettings: vi.fn().mockResolvedValue(undefined),
  GetPermissions: vi.fn().mockResolvedValue("{}"),
  RequestAccessibility: vi.fn().mockResolvedValue(undefined),
  RequestScreenRecording: vi.fn().mockResolvedValue(undefined),
  OpenPermissionSettings: vi.fn().mockResolvedValue(undefined),
  GetVersion: vi.fn().mockResolvedValue("1.2.3"),
  OpenURL: vi.fn().mockResolvedValue(undefined),
  CheckForUpdates: vi.fn().mockResolvedValue(undefined),
  InstallUpdate: vi.fn().mockResolvedValue(undefined),
  GetCrashReport: vi.fn().mockResolvedValue(""),
  DismissCrashReport: vi.fn().mockResolvedValue(undefined),
  ReportCrash: vi.fn().mockResolvedValue(undefined),
  CaptureShortcut: vi.fn().mockResolvedValue("option+tab"),
  CancelShortcutCapture: vi.fn().mockResolvedValue(undefined),
}));

import * as AppService from "../../bindings/option-tab/app.js";
import {
  crashReports,
  hasBackend,
  loadCrashReport,
  loadPermissions,
  loadSettings,
  loadVersion,
  onPrefsTab,
  onSwitcherEvent,
  onSwitcherKey,
  onUpdateAvailable,
  onUpdateChecked,
  onUpdateProgress,
  permissions,
  resetBackendProbeForTests,
  saveSettings,
  switcher,
  system,
} from "./bridge";
import type { Settings } from "./types";

const mocked = vi.mocked(AppService);

beforeEach(() => {
  eventHandlers.clear();
  eventOffs.length = 0;
  resetBackendProbeForTests();
});

afterEach(() => {
  vi.clearAllMocks();
});

describe("hasBackend", () => {
  it("resolves true when the backend answers", async () => {
    await expect(hasBackend()).resolves.toBe(true);
  });

  it("resolves false when the backend call fails", async () => {
    mocked.GetVersion.mockRejectedValueOnce(new Error("no backend"));
    await expect(hasBackend()).resolves.toBe(false);
  });
});

describe("switcher bindings", () => {
  it("routes each action to its exact Go method with args", async () => {
    await switcher.advance();
    await switcher.select(2);
    await switcher.setSearch("hi");
    await switcher.reverse();
    await switcher.confirm();
    await switcher.cancel();
    await switcher.closeSelected();
    await switcher.minimizeSelected();
    await switcher.fullscreenSelected();
    await switcher.quitSelectedApp();
    await switcher.hideSelectedApp();

    expect(mocked.Advance).toHaveBeenCalledTimes(1);
    expect(mocked.Select).toHaveBeenCalledWith(2);
    expect(mocked.SetSearch).toHaveBeenCalledWith("hi");
    expect(mocked.Reverse).toHaveBeenCalledTimes(1);
    expect(mocked.Confirm).toHaveBeenCalledTimes(1);
    expect(mocked.Cancel).toHaveBeenCalledTimes(1);
    expect(mocked.CloseSelected).toHaveBeenCalledTimes(1);
    expect(mocked.MinimizeSelected).toHaveBeenCalledTimes(1);
    expect(mocked.FullscreenSelected).toHaveBeenCalledTimes(1);
    expect(mocked.QuitSelectedApp).toHaveBeenCalledTimes(1);
    expect(mocked.HideSelectedApp).toHaveBeenCalledTimes(1);
  });

  it("swallows rejections instead of surfacing unhandled errors", async () => {
    mocked.Advance.mockRejectedValueOnce(new Error("no backend"));
    await expect(switcher.advance()).resolves.toBeUndefined();
  });
});

describe("onSwitcherEvent", () => {
  it("dispatches show/update/hide and thumbnail/preview payloads", () => {
    const onShow = vi.fn();
    const onUpdate = vi.fn();
    const onHide = vi.fn();
    const onThumbnails = vi.fn();
    const onPreview = vi.fn();
    const unsubscribe = onSwitcherEvent({ onShow, onUpdate, onHide, onThumbnails, onPreview });

    eventHandlers.get("switcher:show")?.({ data: { open: true } });
    eventHandlers.get("switcher:update")?.({ data: { selected: 1 } });
    eventHandlers.get("switcher:hide")?.({ data: null });
    eventHandlers.get("switcher:thumbnails")?.({ data: { "42": "data:t" } });
    eventHandlers.get("switcher:preview")?.({ data: { "42": "data:p" } });

    expect(onShow).toHaveBeenCalledWith({ open: true });
    expect(onUpdate).toHaveBeenCalledWith({ selected: 1 });
    expect(onHide).toHaveBeenCalled();
    expect(onThumbnails).toHaveBeenCalledWith({ "42": "data:t" });
    expect(onPreview).toHaveBeenCalledWith({ "42": "data:p" });

    unsubscribe();
    for (const off of eventOffs) {
      expect(off).toHaveBeenCalledTimes(1);
    }
  });
});

describe("onSwitcherKey", () => {
  it("dispatches forwarded key payloads", () => {
    const cb = vi.fn();
    onSwitcherKey(cb);
    const payload = { key: "w", code: "KeyW", shift: false, ctrl: false, alt: true, meta: false };
    eventHandlers.get("switcher:key")?.({ data: payload });
    expect(cb).toHaveBeenCalledWith(payload);
  });
});

describe("onPrefsTab", () => {
  it("dispatches the deep-linked tab", () => {
    const cb = vi.fn();
    onPrefsTab(cb);
    eventHandlers.get("prefs:tab")?.({ data: "About" });
    expect(cb).toHaveBeenCalledWith("About");
  });
});

describe("onUpdateAvailable", () => {
  it("dispatches the payload", () => {
    const cb = vi.fn();
    onUpdateAvailable(cb);
    eventHandlers.get("update:available")?.({ data: { version: "1.2.3", url: "https://x" } });
    expect(cb).toHaveBeenCalledWith({ version: "1.2.3", url: "https://x" });
  });
});

describe("onUpdateProgress", () => {
  it("dispatches the stage payload", () => {
    const cb = vi.fn();
    onUpdateProgress(cb);
    eventHandlers.get("update:progress")?.({ data: { stage: "downloading" } });
    expect(cb).toHaveBeenCalledWith({ stage: "downloading" });
  });
});

describe("onUpdateChecked", () => {
  it("dispatches the check outcome", () => {
    const cb = vi.fn();
    onUpdateChecked(cb);
    eventHandlers.get("update:checked")?.({ data: { latest: "v1.2.3", available: false } });
    expect(cb).toHaveBeenCalledWith({ latest: "v1.2.3", available: false });
  });
});

describe("system bindings", () => {
  it("calls the bound Go methods", async () => {
    await system.togglePause();
    await system.setPaused(true);
    await system.closePreferences();
    await system.installUpdate();

    expect(mocked.TogglePause).toHaveBeenCalled();
    expect(mocked.SetPaused).toHaveBeenCalledWith(true);
    expect(mocked.ClosePreferences).toHaveBeenCalled();
    expect(mocked.InstallUpdate).toHaveBeenCalled();
  });
});

describe("captureShortcut", () => {
  it("resolves the captured chord when the backend is present", async () => {
    await expect(system.captureShortcut()).resolves.toBe("option+tab");
  });

  it("resolves null when there is no backend", async () => {
    mocked.GetVersion.mockRejectedValueOnce(new Error("no backend"));
    await expect(system.captureShortcut()).resolves.toBeNull();
  });

  it("resolves empty string when the binding rejects", async () => {
    mocked.CaptureShortcut.mockRejectedValueOnce(new Error("cancelled"));
    await expect(system.captureShortcut()).resolves.toBe("");
  });
});

describe("loadVersion", () => {
  it("resolves the version from the binding", async () => {
    await expect(loadVersion()).resolves.toBe("1.2.3");
  });

  it("resolves null when the binding rejects", async () => {
    mocked.GetVersion.mockRejectedValueOnce(new Error("boom"));
    await expect(loadVersion()).resolves.toBeNull();
  });
});

describe("system.openURL", () => {
  it("routes through the Go binding when the backend is present", async () => {
    const open = vi
      .spyOn(globalThis as unknown as { open: typeof window.open }, "open")
      .mockImplementation(() => null);

    await system.openURL("https://example.com");

    expect(mocked.OpenURL).toHaveBeenCalledWith("https://example.com");
    expect(open).not.toHaveBeenCalled();
    open.mockRestore();
  });

  it("falls back to window.open without a backend", async () => {
    mocked.GetVersion.mockRejectedValueOnce(new Error("no backend"));
    const open = vi
      .spyOn(globalThis as unknown as { open: typeof window.open }, "open")
      .mockImplementation(() => null);

    await system.openURL("https://example.com");

    expect(open).toHaveBeenCalledWith("https://example.com", "_blank", "noopener");
    open.mockRestore();
  });
});

describe("crash report bridge", () => {
  it("loadCrashReport resolves null for an empty report", async () => {
    await expect(loadCrashReport()).resolves.toBeNull();
  });

  it("loadCrashReport returns a non-empty log verbatim", async () => {
    const log = "panic: boom\n\ngoroutine 1 [running]:";
    mocked.GetCrashReport.mockResolvedValueOnce(log);
    await expect(loadCrashReport()).resolves.toBe(log);
  });

  it("report and dismiss invoke their Go bindings", async () => {
    await crashReports.report();
    await crashReports.dismiss();

    expect(mocked.ReportCrash).toHaveBeenCalledTimes(1);
    expect(mocked.DismissCrashReport).toHaveBeenCalledTimes(1);
  });
});

describe("loadSettings", () => {
  it("coerces a null appBlacklist from Go into an empty array", async () => {
    mocked.GetSettings.mockResolvedValueOnce(
      '{"version":2,"filters":{"showMinimized":"show","appBlacklist":null}}',
    );

    const s = await loadSettings();
    expect(s?.filters.appBlacklist).toEqual([]);
  });

  it("resolves null when Go returns malformed JSON", async () => {
    mocked.GetSettings.mockResolvedValueOnce("not-json");
    await expect(loadSettings()).resolves.toBeNull();
  });
});

describe("saveSettings", () => {
  it("passes exactly the JSON-serialized settings to Go", async () => {
    const settings = { version: 2, filters: { appBlacklist: ["Finder"] } } as unknown as Settings;

    await saveSettings(settings);

    expect(mocked.SaveSettings).toHaveBeenCalledWith(JSON.stringify(settings));
  });
});

describe("permissions bindings", () => {
  it("loads and parses the permission state", async () => {
    mocked.GetPermissions.mockResolvedValueOnce(
      '{"accessibility":"granted","screenRecording":"denied"}',
    );
    await expect(loadPermissions()).resolves.toEqual({
      accessibility: "granted",
      screenRecording: "denied",
    });
  });

  it("calls request and open-settings bindings", async () => {
    await permissions.request("screenRecording");
    await permissions.openSettings("accessibility");

    expect(mocked.RequestScreenRecording).toHaveBeenCalled();
    expect(mocked.OpenPermissionSettings).toHaveBeenCalledWith("accessibility");
  });

  it("request('accessibility') calls only the accessibility binding", async () => {
    await permissions.request("accessibility");

    expect(mocked.RequestAccessibility).toHaveBeenCalledTimes(1);
    expect(mocked.RequestScreenRecording).not.toHaveBeenCalled();
  });

  it("loadPermissions resolves null on a parse error", async () => {
    mocked.GetPermissions.mockResolvedValueOnce("not-json");
    await expect(loadPermissions()).resolves.toBeNull();
  });
});

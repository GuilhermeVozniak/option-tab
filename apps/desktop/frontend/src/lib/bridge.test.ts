import { afterEach, describe, expect, it, vi } from "vitest";
import {
  crashReports,
  loadCrashReport,
  loadPermissions,
  loadSettings,
  loadVersion,
  onSwitcherEvent,
  onUpdateAvailable,
  permissions,
  saveSettings,
  switcher,
  system,
} from "./bridge";
import type { Settings } from "./types";

afterEach(() => {
  (globalThis as any).go = undefined;
  (globalThis as any).runtime = undefined;
  vi.restoreAllMocks();
});

describe("switcher bindings", () => {
  it("calls the bound Go methods when present", async () => {
    const Advance = vi.fn().mockResolvedValue(undefined);
    const Select = vi.fn().mockResolvedValue(undefined);
    const SetSearch = vi.fn().mockResolvedValue(undefined);
    (globalThis as any).go = { main: { App: { Advance, Select, SetSearch } } };

    await switcher.advance();
    await switcher.select(2);
    await switcher.setSearch("hi");

    expect(Advance).toHaveBeenCalled();
    expect(Select).toHaveBeenCalledWith(2);
    expect(SetSearch).toHaveBeenCalledWith("hi");
  });

  it("no-ops safely when Wails bindings are absent", async () => {
    await expect(switcher.advance()).resolves.toBeUndefined();
    await expect(switcher.confirm()).resolves.toBeUndefined();
  });

  it("routes each remaining action to its exact Go method", async () => {
    const Reverse = vi.fn().mockResolvedValue(undefined);
    const Confirm = vi.fn().mockResolvedValue(undefined);
    const Cancel = vi.fn().mockResolvedValue(undefined);
    const CloseSelected = vi.fn().mockResolvedValue(undefined);
    const MinimizeSelected = vi.fn().mockResolvedValue(undefined);
    const FullscreenSelected = vi.fn().mockResolvedValue(undefined);
    const QuitSelectedApp = vi.fn().mockResolvedValue(undefined);
    const HideSelectedApp = vi.fn().mockResolvedValue(undefined);
    (globalThis as any).go = {
      main: {
        App: {
          Reverse,
          Confirm,
          Cancel,
          CloseSelected,
          MinimizeSelected,
          FullscreenSelected,
          QuitSelectedApp,
          HideSelectedApp,
        },
      },
    };

    await switcher.reverse();
    expect(Reverse).toHaveBeenCalledTimes(1);
    await switcher.confirm();
    expect(Confirm).toHaveBeenCalledTimes(1);
    await switcher.cancel();
    expect(Cancel).toHaveBeenCalledTimes(1);
    await switcher.closeSelected();
    expect(CloseSelected).toHaveBeenCalledTimes(1);
    await switcher.minimizeSelected();
    expect(MinimizeSelected).toHaveBeenCalledTimes(1);
    await switcher.fullscreenSelected();
    expect(FullscreenSelected).toHaveBeenCalledTimes(1);
    await switcher.quitSelectedApp();
    expect(QuitSelectedApp).toHaveBeenCalledTimes(1);
    await switcher.hideSelectedApp();
    expect(HideSelectedApp).toHaveBeenCalledTimes(1);
  });
});

describe("onSwitcherEvent", () => {
  it("subscribes to Wails runtime events and dispatches payloads", () => {
    const handlers: Record<string, (data: unknown) => void> = {};
    const EventsOn = vi.fn((name: string, cb: (data: unknown) => void) => {
      handlers[name] = cb;
      return () => delete handlers[name];
    });
    (globalThis as any).runtime = { EventsOn };

    const onShow = vi.fn();
    const onUpdate = vi.fn();
    const onHide = vi.fn();
    onSwitcherEvent({ onShow, onUpdate, onHide });

    handlers["switcher:show"]?.({ open: true });
    handlers["switcher:update"]?.({ selected: 1 });
    handlers["switcher:hide"]?.(null);

    expect(onShow).toHaveBeenCalledWith({ open: true });
    expect(onUpdate).toHaveBeenCalledWith({ selected: 1 });
    expect(onHide).toHaveBeenCalled();
  });

  it("does not throw when the runtime is absent", () => {
    expect(() =>
      onSwitcherEvent({ onShow: vi.fn(), onUpdate: vi.fn(), onHide: vi.fn() }),
    ).not.toThrow();
  });

  it("dispatches preferences open/close events", () => {
    const handlers: Record<string, (data: unknown) => void> = {};
    const EventsOn = vi.fn((name: string, cb: (data: unknown) => void) => {
      handlers[name] = cb;
      return () => delete handlers[name];
    });
    (globalThis as any).runtime = { EventsOn };

    const onPrefsOpen = vi.fn();
    const onPrefsClose = vi.fn();
    onSwitcherEvent({
      onShow: vi.fn(),
      onUpdate: vi.fn(),
      onHide: vi.fn(),
      onPrefsOpen,
      onPrefsClose,
    });

    handlers["prefs:open"]?.(null);
    handlers["prefs:close"]?.(null);

    expect(onPrefsOpen).toHaveBeenCalled();
    expect(onPrefsClose).toHaveBeenCalled();
  });

  it("dispatches thumbnail/preview payloads and unsubscribes every event", () => {
    const handlers: Record<string, (data: unknown) => void> = {};
    const offFns: Array<ReturnType<typeof vi.fn>> = [];
    const EventsOn = vi.fn((name: string, cb: (data: unknown) => void) => {
      handlers[name] = cb;
      const off = vi.fn();
      offFns.push(off);
      return off;
    });
    (globalThis as any).runtime = { EventsOn };

    const onThumbnails = vi.fn();
    const onPreview = vi.fn();
    const unsubscribe = onSwitcherEvent({
      onShow: vi.fn(),
      onUpdate: vi.fn(),
      onHide: vi.fn(),
      onThumbnails,
      onPreview,
    });

    handlers["switcher:thumbnails"]?.({ "42": "data:t" });
    handlers["switcher:preview"]?.({ "42": "data:p" });

    expect(onThumbnails).toHaveBeenCalledWith({ "42": "data:t" });
    expect(onPreview).toHaveBeenCalledWith({ "42": "data:p" });

    unsubscribe();
    expect(offFns.length).toBeGreaterThan(0);
    for (const off of offFns) {
      expect(off).toHaveBeenCalledTimes(1);
    }
  });
});

describe("onUpdateAvailable", () => {
  it("subscribes to update:available and dispatches the payload", () => {
    const handlers: Record<string, (data: unknown) => void> = {};
    const EventsOn = vi.fn((name: string, cb: (data: unknown) => void) => {
      handlers[name] = cb;
      return () => delete handlers[name];
    });
    (globalThis as any).runtime = { EventsOn };

    const cb = vi.fn();
    onUpdateAvailable(cb);

    handlers["update:available"]?.({ version: "1.2.3", url: "https://x" });
    expect(cb).toHaveBeenCalledWith({ version: "1.2.3", url: "https://x" });
  });

  it("returns a callable no-op unsubscribe without a runtime", () => {
    let unsubscribe: () => void = () => {};
    expect(() => {
      unsubscribe = onUpdateAvailable(vi.fn());
    }).not.toThrow();
    expect(() => unsubscribe()).not.toThrow();
  });
});

describe("system bindings", () => {
  it("calls the bound Go methods when present", async () => {
    const TogglePause = vi.fn().mockResolvedValue(undefined);
    const SetPaused = vi.fn().mockResolvedValue(undefined);
    const ClosePreferences = vi.fn().mockResolvedValue(undefined);
    (globalThis as any).go = { main: { App: { TogglePause, SetPaused, ClosePreferences } } };

    await system.togglePause();
    await system.setPaused(true);
    await system.closePreferences();

    expect(TogglePause).toHaveBeenCalled();
    expect(SetPaused).toHaveBeenCalledWith(true);
    expect(ClosePreferences).toHaveBeenCalled();
  });

  it("no-ops safely when bindings are absent", async () => {
    await expect(system.togglePause()).resolves.toBeUndefined();
    await expect(system.openPreferences()).resolves.toBeUndefined();
  });
});

describe("captureShortcut", () => {
  it("resolves null when the binding is absent", async () => {
    await expect(system.captureShortcut()).resolves.toBeNull();
  });

  it("resolves the captured chord from the binding", async () => {
    const CaptureShortcut = vi.fn().mockResolvedValue("option+tab");
    (globalThis as any).go = { main: { App: { CaptureShortcut } } };
    await expect(system.captureShortcut()).resolves.toBe("option+tab");
  });

  it("resolves empty string when the binding rejects", async () => {
    const CaptureShortcut = vi.fn().mockRejectedValue(new Error("cancelled"));
    (globalThis as any).go = { main: { App: { CaptureShortcut } } };
    await expect(system.captureShortcut()).resolves.toBe("");
  });
});

describe("loadVersion", () => {
  it("resolves the version from the binding", async () => {
    const GetVersion = vi.fn().mockResolvedValue("1.2.3");
    (globalThis as any).go = { main: { App: { GetVersion } } };
    await expect(loadVersion()).resolves.toBe("1.2.3");
  });

  it("resolves null when the binding is absent", async () => {
    await expect(loadVersion()).resolves.toBeNull();
  });

  it("resolves null when the binding rejects", async () => {
    const GetVersion = vi.fn().mockRejectedValue(new Error("boom"));
    (globalThis as any).go = { main: { App: { GetVersion } } };
    await expect(loadVersion()).resolves.toBeNull();
  });
});

describe("system.openURL", () => {
  it("falls back to window.open without Wails", async () => {
    const open = vi
      .spyOn(globalThis as unknown as { open: typeof window.open }, "open")
      .mockImplementation(() => null);

    await system.openURL("https://example.com");

    expect(open).toHaveBeenCalledWith("https://example.com", "_blank", "noopener");
  });

  it("routes through the Go binding and skips window.open", async () => {
    const open = vi
      .spyOn(globalThis as unknown as { open: typeof window.open }, "open")
      .mockImplementation(() => null);
    const OpenURL = vi.fn().mockResolvedValue(undefined);
    (globalThis as any).go = { main: { App: { OpenURL } } };

    await system.openURL("https://example.com");

    expect(OpenURL).toHaveBeenCalledWith("https://example.com");
    expect(open).not.toHaveBeenCalled();
  });
});

describe("crash report bridge", () => {
  it("loadCrashReport resolves null for an empty report", async () => {
    const GetCrashReport = vi.fn().mockResolvedValue("");
    (globalThis as any).go = { main: { App: { GetCrashReport } } };
    await expect(loadCrashReport()).resolves.toBeNull();
  });

  it("loadCrashReport returns a non-empty log verbatim", async () => {
    const log = "panic: boom\n\ngoroutine 1 [running]:";
    const GetCrashReport = vi.fn().mockResolvedValue(log);
    (globalThis as any).go = { main: { App: { GetCrashReport } } };
    await expect(loadCrashReport()).resolves.toBe(log);
  });

  it("report and dismiss invoke their Go bindings", async () => {
    const ReportCrash = vi.fn().mockResolvedValue(undefined);
    const DismissCrashReport = vi.fn().mockResolvedValue(undefined);
    (globalThis as any).go = { main: { App: { ReportCrash, DismissCrashReport } } };

    await crashReports.report();
    await crashReports.dismiss();

    expect(ReportCrash).toHaveBeenCalledTimes(1);
    expect(DismissCrashReport).toHaveBeenCalledTimes(1);
  });
});

describe("loadSettings", () => {
  it("coerces a null appBlacklist from Go into an empty array", async () => {
    const GetSettings = vi
      .fn()
      .mockResolvedValue('{"version":2,"filters":{"showMinimized":"show","appBlacklist":null}}');
    (globalThis as any).go = { main: { App: { GetSettings } } };

    const s = await loadSettings();
    expect(s?.filters.appBlacklist).toEqual([]);
  });

  it("resolves null when the binding is absent", async () => {
    await expect(loadSettings()).resolves.toBeNull();
  });

  it("resolves null when Go returns malformed JSON", async () => {
    const GetSettings = vi.fn().mockResolvedValue("not-json");
    (globalThis as any).go = { main: { App: { GetSettings } } };
    await expect(loadSettings()).resolves.toBeNull();
  });
});

describe("saveSettings", () => {
  it("passes exactly the JSON-serialized settings to Go", async () => {
    const SaveSettings = vi.fn().mockResolvedValue(undefined);
    (globalThis as any).go = { main: { App: { SaveSettings } } };
    const settings = { version: 2, filters: { appBlacklist: ["Finder"] } } as unknown as Settings;

    await saveSettings(settings);

    expect(SaveSettings).toHaveBeenCalledWith(JSON.stringify(settings));
  });

  it("no-ops without the binding", async () => {
    await expect(saveSettings({} as Settings)).resolves.toBeUndefined();
  });
});

describe("permissions bindings", () => {
  it("loads and parses the permission state", async () => {
    const GetPermissions = vi
      .fn()
      .mockResolvedValue('{"accessibility":"granted","screenRecording":"denied"}');
    (globalThis as any).go = { main: { App: { GetPermissions } } };
    await expect(loadPermissions()).resolves.toEqual({
      accessibility: "granted",
      screenRecording: "denied",
    });
  });

  it("returns null when the binding is absent", async () => {
    await expect(loadPermissions()).resolves.toBeNull();
  });

  it("calls request and open-settings bindings", async () => {
    const RequestScreenRecording = vi.fn().mockResolvedValue(undefined);
    const OpenPermissionSettings = vi.fn().mockResolvedValue(undefined);
    (globalThis as any).go = { main: { App: { RequestScreenRecording, OpenPermissionSettings } } };

    await permissions.request("screenRecording");
    await permissions.openSettings("accessibility");

    expect(RequestScreenRecording).toHaveBeenCalled();
    expect(OpenPermissionSettings).toHaveBeenCalledWith("accessibility");
  });

  it("request('accessibility') calls only the accessibility binding", async () => {
    const RequestAccessibility = vi.fn().mockResolvedValue(undefined);
    const RequestScreenRecording = vi.fn().mockResolvedValue(undefined);
    (globalThis as any).go = { main: { App: { RequestAccessibility, RequestScreenRecording } } };

    await permissions.request("accessibility");

    expect(RequestAccessibility).toHaveBeenCalledTimes(1);
    expect(RequestScreenRecording).not.toHaveBeenCalled();
  });

  it("loadPermissions resolves null on a parse error", async () => {
    const GetPermissions = vi.fn().mockResolvedValue("not-json");
    (globalThis as any).go = { main: { App: { GetPermissions } } };
    await expect(loadPermissions()).resolves.toBeNull();
  });
});

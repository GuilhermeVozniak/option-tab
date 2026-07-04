import { afterEach, describe, expect, it, vi } from "vitest";
import {
  loadPermissions,
  loadSettings,
  onSwitcherEvent,
  permissions,
  switcher,
  system,
} from "./bridge";

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

describe("loadSettings", () => {
  it("coerces a null appBlacklist from Go into an empty array", async () => {
    const GetSettings = vi
      .fn()
      .mockResolvedValue('{"version":2,"filters":{"showMinimized":"show","appBlacklist":null}}');
    (globalThis as any).go = { main: { App: { GetSettings } } };

    const s = await loadSettings();
    expect(s?.filters.appBlacklist).toEqual([]);
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
});

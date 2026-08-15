import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

// Mock the Wails v3 seams (generated bindings + @wailsio/runtime events) the
// same way bridge.test.ts does; each test then tailors the mocked bindings.
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

vi.mock("../../bindings/option-tab/app.js", () => ({
  GetPermissions: vi.fn().mockResolvedValue("{}"),
  RequestAccessibility: vi.fn().mockResolvedValue(undefined),
  RequestScreenRecording: vi.fn().mockResolvedValue(undefined),
  OpenPermissionSettings: vi.fn().mockResolvedValue(undefined),
  GetVersion: vi.fn().mockResolvedValue("1.2.3"),
  OpenURL: vi.fn().mockResolvedValue(undefined),
  CheckForUpdates: vi.fn().mockResolvedValue(undefined),
  InstallUpdate: vi.fn().mockResolvedValue(undefined),
  GetCrashReport: vi.fn().mockResolvedValue(""),
  ReportCrash: vi.fn().mockResolvedValue(undefined),
  DismissCrashReport: vi.fn().mockResolvedValue(undefined),
}));

import * as AppService from "../../bindings/option-tab/app.js";
import { resetBackendProbeForTests } from "../lib/bridge";
import { useAbout, useCrash, usePermissions } from "./useBridge";

const mocked = vi.mocked(AppService);

beforeEach(() => {
  eventHandlers.clear();
  resetBackendProbeForTests();
  vi.clearAllMocks();
  vi.useRealTimers();
});

describe("usePermissions", () => {
  it("returns undefined without Wails (no Permissions UI)", async () => {
    mocked.GetPermissions.mockRejectedValueOnce(new Error("no backend"));
    const { result } = renderHook(() => usePermissions());
    // The initial render is undefined; give the (null-resolving) load a tick
    // and confirm it stays undefined so the section is never rendered.
    await act(async () => {});
    expect(result.current).toBeUndefined();
  });

  it("loads permission state, polls for changes, and delegates actions", async () => {
    vi.useFakeTimers();
    mocked.GetPermissions.mockResolvedValueOnce(
      JSON.stringify({ accessibility: "granted", screenRecording: "granted" }),
    ).mockResolvedValue(JSON.stringify({ accessibility: "denied", screenRecording: "granted" }));

    const { result } = renderHook(() => usePermissions());

    // Initial async load resolves the granted/granted state.
    await act(async () => {
      await Promise.resolve();
    });
    expect(result.current?.state).toEqual({
      accessibility: "granted",
      screenRecording: "granted",
    });

    // The 2s poll picks up a grant revoked in System Settings.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000);
    });
    expect(result.current?.state.accessibility).toBe("denied");

    // Actions route to the matching bound method.
    result.current?.onRequest("accessibility");
    expect(mocked.RequestAccessibility).toHaveBeenCalledTimes(1);
    result.current?.onRequest("screenRecording");
    expect(mocked.RequestScreenRecording).toHaveBeenCalledTimes(1);
    result.current?.onOpenSettings("accessibility");
    expect(mocked.OpenPermissionSettings).toHaveBeenCalledWith("accessibility");
  });
});

describe("useAbout", () => {
  it("falls back to the dev version and opens URLs via the browser without Wails", async () => {
    // No backend: the version probe and the backend probe both fail.
    mocked.GetVersion.mockRejectedValue(new Error("no backend"));
    const openSpy = vi.spyOn(globalThis, "open").mockReturnValue(null);
    const { result } = renderHook(() => useAbout());
    await act(async () => {});
    expect(result.current.version).toBe("dev");
    expect(result.current.update).toBeUndefined();

    // onOpenURL with no backend falls back to window.open.
    await act(async () => {
      await result.current.onOpenURL("https://example.com");
    });
    expect(openSpy).toHaveBeenCalledWith("https://example.com", "_blank", "noopener");
    openSpy.mockRestore();
  });

  it("reports the Go version, surfaces an update event, and routes actions", async () => {
    mocked.GetVersion.mockResolvedValue("9.9.9");

    const { result } = renderHook(() => useAbout());

    await waitFor(() => expect(result.current.version).toBe("9.9.9"));

    // A background update:available event flows into the hook.
    act(() => {
      eventHandlers.get("update:available")?.({
        data: { version: "9.9.9", url: "https://releases/9.9.9" },
      });
    });
    expect(result.current.update).toEqual({
      version: "9.9.9",
      url: "https://releases/9.9.9",
    });

    await act(async () => {
      await result.current.onOpenURL("https://option-tab.dev");
    });
    expect(mocked.OpenURL).toHaveBeenCalledWith("https://option-tab.dev");
    result.current.onCheckUpdates();
    expect(mocked.CheckForUpdates).toHaveBeenCalledTimes(1);

    // A background update:progress event flows into the hook; the install
    // action routes to the bound Go method.
    act(() => {
      eventHandlers.get("update:progress")?.({ data: { stage: "installing" } });
    });
    expect(result.current.progress).toEqual({ stage: "installing" });
    result.current.onInstallUpdate?.();
    expect(mocked.InstallUpdate).toHaveBeenCalledTimes(1);

    // Every check's outcome (even "no update") flows into the hook, so the
    // About tab can answer a manual check instead of staying silent.
    act(() => {
      eventHandlers.get("update:checked")?.({ data: { latest: "v9.9.9", available: false } });
    });
    expect(result.current.checked).toEqual({ latest: "v9.9.9", available: false });
  });
});

describe("useCrash", () => {
  it("stays undefined when there is nothing to report", async () => {
    const { result } = renderHook(() => useCrash());
    await act(async () => {
      await Promise.resolve();
    });
    expect(result.current).toBeUndefined();
  });

  it("surfaces a crash summary and reports / dismisses it", async () => {
    mocked.GetCrashReport.mockResolvedValue("panic: boom\ngoroutine 1 [running]:");

    const { result } = renderHook(() => useCrash());
    await waitFor(() => expect(result.current?.summary).toBe("panic: boom"));

    result.current?.onReport();
    expect(mocked.ReportCrash).toHaveBeenCalledTimes(1);

    // Dismiss clears the banner and tells Go to discard the pending report.
    act(() => {
      result.current?.onDismiss();
    });
    expect(mocked.DismissCrashReport).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(result.current).toBeUndefined());
  });
});

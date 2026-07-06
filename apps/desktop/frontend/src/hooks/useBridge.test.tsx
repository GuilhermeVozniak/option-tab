import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useAbout, useCrash, usePermissions } from "./useBridge";

// These hooks read the Wails globals lazily through lib/bridge, so each test
// installs (or omits) globalThis.go / globalThis.runtime the same way
// bridge.test.ts does, then renders the hook and flushes the bridge promises.
afterEach(() => {
  (globalThis as any).go = undefined;
  (globalThis as any).runtime = undefined;
  vi.restoreAllMocks();
  vi.useRealTimers();
});

function setApp(methods: Record<string, unknown>) {
  (globalThis as any).go = { main: { App: methods } };
}

describe("usePermissions", () => {
  it("returns undefined without Wails (no Permissions UI)", async () => {
    const { result } = renderHook(() => usePermissions());
    // The initial render is undefined; give the (null-resolving) load a tick
    // and confirm it stays undefined so the section is never rendered.
    await act(async () => {});
    expect(result.current).toBeUndefined();
  });

  it("loads permission state, polls for changes, and delegates actions", async () => {
    vi.useFakeTimers();
    const GetPermissions = vi
      .fn()
      .mockResolvedValueOnce(
        JSON.stringify({ accessibility: "granted", screenRecording: "granted" }),
      )
      .mockResolvedValue(JSON.stringify({ accessibility: "denied", screenRecording: "granted" }));
    const RequestAccessibility = vi.fn().mockResolvedValue(undefined);
    const RequestScreenRecording = vi.fn().mockResolvedValue(undefined);
    const OpenPermissionSettings = vi.fn().mockResolvedValue(undefined);
    setApp({
      GetPermissions,
      RequestAccessibility,
      RequestScreenRecording,
      OpenPermissionSettings,
    });

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
    expect(RequestAccessibility).toHaveBeenCalledTimes(1);
    result.current?.onRequest("screenRecording");
    expect(RequestScreenRecording).toHaveBeenCalledTimes(1);
    result.current?.onOpenSettings("accessibility");
    expect(OpenPermissionSettings).toHaveBeenCalledWith("accessibility");
  });
});

describe("useAbout", () => {
  it("falls back to the dev version and opens URLs via the browser without Wails", async () => {
    const openSpy = vi.spyOn(globalThis, "open").mockReturnValue(null);
    const { result } = renderHook(() => useAbout());
    await act(async () => {});
    expect(result.current.version).toBe("dev");
    expect(result.current.update).toBeUndefined();

    // onOpenURL with no OpenURL binding falls back to window.open.
    result.current.onOpenURL("https://example.com");
    expect(openSpy).toHaveBeenCalledWith("https://example.com", "_blank", "noopener");
  });

  it("reports the Go version, surfaces an update event, and routes actions", async () => {
    const GetVersion = vi.fn().mockResolvedValue("9.9.9");
    const OpenURL = vi.fn().mockResolvedValue(undefined);
    const CheckForUpdates = vi.fn().mockResolvedValue(undefined);
    setApp({ GetVersion, OpenURL, CheckForUpdates });
    let updateCb: ((d: unknown) => void) | undefined;
    (globalThis as any).runtime = {
      EventsOn: (event: string, cb: (d: unknown) => void) => {
        if (event === "update:available") updateCb = cb;
        return () => {};
      },
    };

    const { result } = renderHook(() => useAbout());

    await waitFor(() => expect(result.current.version).toBe("9.9.9"));

    // A background update:available event flows into the hook.
    act(() => {
      updateCb?.({ version: "9.9.9", url: "https://releases/9.9.9" });
    });
    expect(result.current.update).toEqual({
      version: "9.9.9",
      url: "https://releases/9.9.9",
    });

    result.current.onOpenURL("https://option-tab.dev");
    expect(OpenURL).toHaveBeenCalledWith("https://option-tab.dev");
    result.current.onCheckUpdates();
    expect(CheckForUpdates).toHaveBeenCalledTimes(1);
  });
});

describe("useCrash", () => {
  it("stays undefined when there is nothing to report", async () => {
    const GetCrashReport = vi.fn().mockResolvedValue("");
    setApp({ GetCrashReport });
    const { result } = renderHook(() => useCrash());
    await act(async () => {
      await Promise.resolve();
    });
    expect(result.current).toBeUndefined();
  });

  it("surfaces a crash summary and reports / dismisses it", async () => {
    const GetCrashReport = vi.fn().mockResolvedValue("panic: boom\ngoroutine 1 [running]:");
    const ReportCrash = vi.fn().mockResolvedValue(undefined);
    const DismissCrashReport = vi.fn().mockResolvedValue(undefined);
    setApp({ GetCrashReport, ReportCrash, DismissCrashReport });

    const { result } = renderHook(() => useCrash());
    await waitFor(() => expect(result.current?.summary).toBe("panic: boom"));

    result.current?.onReport();
    expect(ReportCrash).toHaveBeenCalledTimes(1);

    // Dismiss clears the banner and tells Go to discard the pending report.
    act(() => {
      result.current?.onDismiss();
    });
    expect(DismissCrashReport).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(result.current).toBeUndefined());
  });
});

import { act, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const events = new Map<string, (event: { data: unknown }) => void>();
const native = vi.hoisted(() => ({
  capabilities: vi.fn().mockResolvedValue({ available: true }),
  settings: vi.fn().mockResolvedValue("{}"),
  regions: vi.fn().mockResolvedValue(undefined),
}));
vi.mock("@wailsio/runtime", async (original) => ({
  ...(await original<typeof import("@wailsio/runtime")>()),
  Events: {
    On: (name: string, handler: (event: { data: unknown }) => void) => {
      events.set(name, handler);
      return () => events.delete(name);
    },
  },
}));
vi.mock("../bindings/option-tab/app.js", async (original) => ({
  ...(await original<typeof import("../bindings/option-tab/app.js")>()),
  GetVersion: vi.fn().mockResolvedValue("0.4.8"),
  GetSettingsState: vi.fn().mockResolvedValue({ revision: 1, json: "{}" }),
  GetSettings: native.settings,
  GetSwitcherMaterialStatus: vi
    .fn()
    .mockResolvedValue({ session: 0, revision: 0, state: "unavailable" }),
  SetSwitcherMaterialRect: vi.fn().mockResolvedValue(undefined),
  GetSwitcherGestureCapabilities: native.capabilities,
  SetSwitcherGestureRegions: native.regions,
}));

import App from "./App";
import { switcher } from "./lib/bridge";
import { emptyState } from "./lib/types";

function emit(name: string, data: unknown) {
  act(() => events.get(name)?.({ data }));
}
const state = (session: number, revision = 1) => ({
  ...emptyState,
  open: true,
  session,
  revision,
  entries: [
    {
      windowId: 10,
      appId: 20,
      title: "Test",
      appName: "Editor",
      bundleId: "test.editor",
      minimized: false,
      hidden: false,
      fullscreen: false,
    },
  ],
});
beforeEach(() => {
  window.location.hash = "";
  events.clear();
  vi.clearAllMocks();
  native.settings.mockReset().mockResolvedValue("{}");
});
describe("native switcher gesture route", () => {
  it("exposes only a positively advertised native capability", async () => {
    await expect(switcher.gestureCapabilities()).resolves.toEqual({ available: true });
    native.capabilities.mockResolvedValueOnce("<html>browser fallback</html>");
    await expect(switcher.gestureCapabilities()).resolves.toEqual({ available: false });
    native.capabilities.mockRejectedValueOnce(new Error("no native port"));
    await expect(switcher.gestureCapabilities()).resolves.toEqual({ available: false });
  });
  it("displays only current-presentation native action errors, including repeated failures, and retires them on hide", async () => {
    render(<App />);
    emit("switcher:show", state(7, 2));
    emit("switcher:gestureError", { session: 6, revision: 99, message: "Old session" });
    expect(screen.queryByRole("alert")).toBeNull();
    emit("switcher:gestureError", {
      session: 7,
      revision: 2,
      message: "requested AX action is unsupported",
    });
    expect(screen.getByRole("alert")).toHaveTextContent(
      "This action is not supported for this window.",
    );
    emit("switcher:gestureError", { session: 7, revision: 1, message: "Old action" });
    expect(screen.getByRole("alert")).toHaveTextContent(
      "This action is not supported for this window.",
    );
    emit("switcher:gestureError", {
      session: 7,
      revision: 2,
      message: "accessibility permission is required",
    });
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Accessibility permission is required for this action.",
    );
    emit("switcher:update", state(7, 3));
    emit("switcher:gestureError", { session: 7, revision: 2, message: "Retired gallery" });
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Accessibility permission is required for this action.",
    );
    emit("switcher:gestureError", { session: 7, revision: 4, message: "Future gallery" });
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Accessibility permission is required for this action.",
    );
    emit("switcher:hide", { session: 7, revision: 4 });
    emit("switcher:gestureError", { session: 7, revision: 4, message: "After hide" });
    expect(screen.queryByRole("alert")).toBeNull();
    emit("switcher:show", state(8));
    expect(screen.queryByRole("alert")).toBeNull();
    emit("switcher:gestureError", { session: 7, revision: 100, message: "Delayed old action" });
    expect(screen.queryByRole("alert")).toBeNull();
    emit("switcher:gestureError", { session: 8, revision: 1, message: "Current action" });
    expect(screen.getByRole("alert")).toHaveTextContent(
      "The window action could not be completed. Try again.",
    );
    await waitFor(() => expect(native.regions).toHaveBeenCalled());
  });
  it.each([
    [
      "en",
      "Accessibility permission is required for this action.",
      "This action is not supported for this window.",
      "The window action could not be completed. Try again.",
    ],
    [
      "pt-BR",
      "A permissão de Acessibilidade é necessária para esta ação.",
      "Esta ação não é compatível com esta janela.",
      "Não foi possível concluir a ação na janela. Tente novamente.",
    ],
    [
      "es",
      "Se necesita permiso de Accesibilidad para esta acción.",
      "Esta acción no es compatible con esta ventana.",
      "No se pudo completar la acción en la ventana. Inténtalo de nuevo.",
    ],
  ])("localizes native failure categories in %s without exposing internal errors", async (language, permission, unsupported, generic) => {
    native.settings.mockResolvedValue(JSON.stringify({ behavior: { language } }));
    render(<App />);
    emit("switcher:show", state(7));
    for (const [message, expected] of [
      ["accessibility permission is required", permission],
      ["requested AX action is unsupported", unsupported],
      ["internal failure at /private/user/window-title", generic],
    ]) {
      emit("switcher:gestureError", { session: 7, revision: 1, message });
      await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent(expected));
      expect(screen.getByRole("alert")).not.toHaveTextContent(message);
    }
  });

  it("retranslates a gesture failure when delayed language settings arrive", async () => {
    let resolveSettings: (value: string) => void = () => {};
    native.settings.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveSettings = resolve;
        }),
    );
    render(<App />);
    emit("switcher:show", state(7));
    emit("switcher:gestureError", {
      session: 7,
      revision: 1,
      message: "requested AX action is unsupported",
    });
    await waitFor(() => expect(native.settings).toHaveBeenCalled());
    expect(screen.getByRole("alert")).toHaveTextContent(
      "This action is not supported for this window.",
    );
    await act(async () => resolveSettings(JSON.stringify({ behavior: { language: "pt-BR" } })));
    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent(
        "Esta ação não é compatível com esta janela.",
      ),
    );
  });

  it("keeps geometry sequence increasing across renderer changes and ignores stale RPC replies", async () => {
    let rejectOld: (reason: unknown) => void = () => {};
    native.regions.mockImplementationOnce(
      () =>
        new Promise((_, reject) => {
          rejectOld = reject;
        }),
    );
    const { unmount } = render(<App />);
    emit("switcher:show", state(7));
    emit("switcher:update", {
      ...state(7, 2),
      mode: "apps",
      apps: [{ appId: 20, appName: "Editor", bundleId: "test.editor", windowCount: 1 }],
    });
    emit("switcher:hide", { session: 7, revision: 3 });
    emit("switcher:show", state(8));
    await act(async () => rejectOld(new Error("old region request")));
    expect(screen.queryByRole("alert")).toBeNull();
    const old = native.regions.mock.calls.filter((call) => call[0] === 7);
    expect(old.length).toBeGreaterThanOrEqual(3);
    expect(old.map((call) => call[2])).toEqual(old.map((_, index) => index + 1));
    expect(native.regions.mock.calls.at(-1)).toEqual([8, 1, 1, []]);
    unmount();
    expect(native.regions.mock.calls.at(-1)).toEqual([8, 1, 2, []]);
  });
});

import { act, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import App from "./App";

afterEach(() => {
  window.location.hash = "";
  // biome-ignore lint/suspicious/noExplicitAny: test cleanup of injected globals
  (globalThis as any).runtime = undefined;
  // biome-ignore lint/suspicious/noExplicitAny: test cleanup of injected globals
  (globalThis as any).go = undefined;
  vi.restoreAllMocks();
});

describe("App", () => {
  it("renders the overlay route (closed) by default", () => {
    window.location.hash = "";
    const { container } = render(<App />);
    // Overlay renders nothing while closed.
    expect(container.firstChild).toBeNull();
  });

  it("renders the settings route at #settings", () => {
    window.location.hash = "#settings";
    render(<App />);
    expect(screen.getByText(/Preferences/)).toBeInTheDocument();
  });

  it("opens the preferences panel on the prefs:open event", () => {
    const handlers: Record<string, (data: unknown) => void> = {};
    // biome-ignore lint/suspicious/noExplicitAny: injecting the Wails global
    (globalThis as any).runtime = {
      EventsOn: (name: string, cb: (data: unknown) => void) => {
        handlers[name] = cb;
        return () => delete handlers[name];
      },
    };
    window.location.hash = "";
    render(<App />);
    // No preferences dialog until the menubar fires the event.
    expect(screen.queryByRole("dialog", { name: "Preferences" })).toBeNull();

    act(() => {
      handlers["prefs:open"]?.(null);
    });
    expect(screen.getByRole("dialog", { name: "Preferences" })).toBeInTheDocument();
  });
});

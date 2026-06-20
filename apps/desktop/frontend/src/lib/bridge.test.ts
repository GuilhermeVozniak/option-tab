import { afterEach, describe, expect, it, vi } from "vitest";
import { onSwitcherEvent, switcher } from "./bridge";

afterEach(() => {
  // biome-ignore lint/suspicious/noExplicitAny: test cleanup of injected globals
  (globalThis as any).go = undefined;
  // biome-ignore lint/suspicious/noExplicitAny: test cleanup of injected globals
  (globalThis as any).runtime = undefined;
  vi.restoreAllMocks();
});

describe("switcher bindings", () => {
  it("calls the bound Go methods when present", async () => {
    const Advance = vi.fn().mockResolvedValue(undefined);
    const Select = vi.fn().mockResolvedValue(undefined);
    const SetSearch = vi.fn().mockResolvedValue(undefined);
    // biome-ignore lint/suspicious/noExplicitAny: injecting the Wails global
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
    // biome-ignore lint/suspicious/noExplicitAny: injecting the Wails global
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
});

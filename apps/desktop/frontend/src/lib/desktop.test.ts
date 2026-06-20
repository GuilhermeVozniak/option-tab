import { afterEach, describe, expect, it, vi } from "vitest";
import { greet } from "./desktop";

afterEach(() => {
  // biome-ignore lint/performance/noDelete: test cleanup of the injected global
  delete (globalThis as { go?: unknown }).go;
});

describe("greet", () => {
  it("delegates to the Wails App.Greet binding", async () => {
    const Greet = vi.fn().mockResolvedValue("Hello, Gui!");
    (globalThis as { go?: unknown }).go = { main: { App: { Greet } } };

    await expect(greet("Gui")).resolves.toBe("Hello, Gui!");
    expect(Greet).toHaveBeenCalledWith("Gui");
  });

  it("throws a helpful error when the Wails runtime is absent", async () => {
    await expect(greet("Gui")).rejects.toThrow(/wails/i);
  });
});
